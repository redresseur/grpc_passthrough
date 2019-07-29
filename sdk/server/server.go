package server

import (
	"code.speakin.mobi/identify/remote_desktop_multi/passthrough.git/common"
	"code.speakin.mobi/identify/remote_desktop_multi/passthrough.git/proto/bridge"
	"context"
	"encoding/json"
	"errors"
	"github.com/op/go-logging"
	"google.golang.org/grpc"
	"io"
	"net"
	"net/http"
	"time"
)

var logger *logging.Logger

func init() {
	logger = logging.MustGetLogger("sdk/server")
}

/*
	function: 此模块的作用是用于grpc透传模式的服务端使用
*/

type Server struct {
	name string
	seq  string
	l    net.Listener
}

//注册服务
// 默认超时TTL
func RegistryService(c bridge.BridgeClient, name string, l net.Listener) (*Server, error) {
	ctx, _ := context.WithTimeout(context.Background(), common.TTL)

	// TODO: 增加Token
	req := &bridge.RegisterRequest{
		ServiceName: name,
		Type:        bridge.RegisterRequest_PRIVATE,
	}

	rsp, err := c.RegisterService(ctx, req)
	if err != nil {
		return nil, err
	}

	if rsp.StatusCode != http.StatusOK {
		return nil, errors.New(rsp.ErrMsg)
	}

	return &Server{
		name: name,
		seq:  string(rsp.Payload),
		l:    l,
	}, nil
}

func Connect(notify *bridge.ConnectNotify, pointType bridge.ConnectHeader_PointType) (*net.TCPConn, error) {
	logger.Infof("the connection id [%s], type [%d] ", notify.ConnectionId, pointType)
	// TODO: 增加TLS选项
	preVerify := &bridge.Connection{
		Header: &bridge.ConnectHeader{
			ConnectionId: notify.ConnectionId,
			Type:         pointType,
		},
	}

	// data, err := proto.Marshal(preVerify)
	data, err := json.Marshal(preVerify)
	if err != nil {
		return nil, err
	}

	data = common.Marshal(common.JSON, data)
	conn, err := net.DialTimeout(notify.PassThroughType, notify.PassThroughAddr, common.DIAL_TIEMOUT)
	if err != nil {
		return nil, err
	}

	conn.SetDeadline(time.Now().Add(common.TTL))
	if written, err := conn.Write(data); err != nil {
		conn.Close()
		return nil, err
	} else if written != len(data) {
		return nil, errors.New("the write data length was not unique")
	} else {
		defer conn.SetDeadline(time.Time{})
		logger.Debugf("write to data len %v", written)
	}

	// 开启心跳
	if tc, ok := conn.(*net.TCPConn); ok {
		tc.SetKeepAlivePeriod(common.KEEP_ALIVE_DURATION)
		tc.SetKeepAlive(true)
	}

	return conn.(*net.TCPConn), nil
}

func RunService(c bridge.BridgeClient, server *Server,
	grpcServer *grpc.Server, ctx context.Context) error {
	ctx, _ = context.WithCancel(ctx)

	// TODO: 增加TOKEN
	req := &bridge.ListenerRequest{
		ServiceSequence: server.seq,
	}

	// 启动一个接收桥接服务的链接通知的协程
	if lc, err := c.Listener(ctx, req); err != nil {
		return err
	} else {
		go func() {
			defer lc.CloseSend()
			for {
				cn, err := lc.Recv()
				if err != nil {
					if err == io.EOF {
						logger.Debugf("the listener connection [%s] shutdown by server",
							server.name)
						return
					}

					if ctxErr := lc.Context().Err(); ctxErr != nil {
						logger.Errorf("the listener connection [%s] context: [%v]",
							server.name, ctxErr)
						return
					}

					logger.Errorf("the listener connection [%s] exception: [%v]",
						server.name, err)
					return
				}

				if connNew, err := Connect(cn, bridge.ConnectHeader_SERVER); err != nil {
					logger.Errorf("make a new connection to <%s, %s>: %v",
						cn.PassThroughType, cn.PassThroughAddr, err)
				} else {
					server.l.(*PTcpListener).Deliver(connNew)
				}
			}
		}()
	}
	err := grpcServer.Serve(server.l)
	return err
}
