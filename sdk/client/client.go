package client

import (
	"code.speakin.mobi/identify/remote_desktop_multi/passthrough.git/common"
	"code.speakin.mobi/identify/remote_desktop_multi/passthrough.git/proto/bridge"
	"code.speakin.mobi/identify/remote_desktop_multi/passthrough.git/sdk/server"
	"context"
	"errors"
	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc"
	"net"
	"net/http"
)

func Connect(client bridge.BridgeClient, serverName string)(*grpc.ClientConn, error)  {
	rsp, err := client.Query(context.Background(), &bridge.QueryRequest{
		Type: bridge.QueryRequest_SERVICE_SEQ,
		Condition: []byte(serverName),
	} )

	if err != nil{
		return nil, err
	}else if rsp.StatusCode != http.StatusOK {
		return nil, errors.New(rsp.ErrMsg)
	}

	// TODO: 添加TOKEN

	rsp, err = client.Connect(context.Background(), &bridge.ConnectRequest{
		ServiceSequence: string(rsp.Payload),
	})

	if err != nil{
		return nil, err
	} else if rsp.StatusCode != http.StatusOK {
		return nil, errors.New(rsp.ErrMsg)
	}

	cn := &bridge.ConnectNotify{}
	if err := proto.Unmarshal(rsp.Payload , cn); err != nil{
		return nil, err
	}

	dialer := func(ctx context.Context, name string) (net.Conn, error){
		// 注册事件
		connEvent, err := client.RegisterEvent(ctx, &bridge.EventRequest{
			Type: bridge.EventRequest_CONNECT_EVENT,
			Payload: []byte(cn.ConnectionId), // 将connection id 放入payload 中， 作为事件的过滤条件
		})

		if err != nil{
			return nil, err
		}else {
			defer connEvent.CloseSend()
		}

		conn , err := server.Connect(cn, bridge.ConnectHeader_CLIENT)
		if err != nil{
			return nil, err
		}

		// 等待链接事件
		_, err = connEvent.Recv()
		if err != nil{
			return nil, err
		}

		return conn, nil
	}

	ctx, _ := context.WithTimeout(context.Background(), common.DIAL_TIEMOUT)
	// TODO: 开启tls
	return grpc.DialContext(ctx, serverName, grpc.WithContextDialer(dialer), grpc.WithInsecure(), )
}