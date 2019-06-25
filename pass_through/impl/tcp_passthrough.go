package impl

import (
	"code.speakin.mobi/identify/remote_desktop_multi/passthrough.git/common"
	"code.speakin.mobi/identify/remote_desktop_multi/passthrough.git/impl/event"
	"code.speakin.mobi/identify/remote_desktop_multi/passthrough.git/proto/bridge"
	ppass_through "code.speakin.mobi/identify/remote_desktop_multi/passthrough.git/proto/pass_through"
	"encoding/json"
	"github.com/golang/protobuf/proto"
	"github.com/op/go-logging"
	"log"
	"sync/atomic"
	"time"

	//"code.speakin.mobi/identify/remote_desktop_multi/passthrough.git/bridge"
	"errors"
	"fmt"
	"net"
	"sync"
)

var logger *logging.Logger

func init()  {
	logger = logging.MustGetLogger("tcp/bridge")
}


type TcpPassThroughSrv struct {
	srvAddr *net.TCPAddr
	connWaitingSets sync.Map // 处于等待使用状态的连接
	passThroughConns sync.Map // 处于执行中的任务
	connRunningSets sync.Map // 正在使用的连接
	eventManage *event.Manager
}

func NewTcpPassThroughSrv(srvAddr *net.TCPAddr, manager *event.Manager) *TcpPassThroughSrv {
	return &TcpPassThroughSrv{
		srvAddr: srvAddr,
		connWaitingSets: sync.Map{},
		connRunningSets: sync.Map{},
		passThroughConns:sync.Map{},
		eventManage:manager,
	}
}

func (t *TcpPassThroughSrv)Addr()net.Addr{
	return t.srvAddr
}

// TODO: 增加TLS选项
func (t *TcpPassThroughSrv)TlsEnable()bool{
	return false
}

func (t *TcpPassThroughSrv) Listener() error {
	// Listen on TCP port 2000 on all available unicast and
	// anycast IP addresses of the local system.
	l, err := net.Listen(t.srvAddr.Network(), t.srvAddr.String())
	if err != nil {
		return err
	}
	defer l.Close()
	for {
		// Wait for a connection.
		conn, err := l.Accept()
		if err != nil {
			log.Fatal(err)
		}

		//t.automSetID(conn)
		go t.verifyConn(conn)
	}

	return nil
}

func (t *TcpPassThroughSrv) Accept(connection *ppass_through.PassThroughConnection) (err error){
	if _, ok := t.passThroughConns.Load(connection.GrpcClientId); ok {
		err = fmt.Errorf("the client id [%s] has been existed", connection.GrpcClientId)
		logger.Error(err.Error())
		return
	}

	if _, ok := t.passThroughConns.Load(connection.GrpcSrvId); ok {
		err = fmt.Errorf("the server id [%s] has been existed", connection.GrpcSrvId)
		return
	}

	t.passThroughConns.Store(connection.GrpcClientId, connection)
	t.passThroughConns.Store(connection.GrpcSrvId, connection)
	return nil
}


func (t *TcpPassThroughSrv)removeTask(conn *ppass_through.PassThroughConnection)  {
	t.passThroughConns.Delete(conn.GrpcClientId)
	t.passThroughConns.Delete(conn.GrpcSrvId)
}

func (t *TcpPassThroughSrv)verifyConn(conn net.Conn)(err error) {
	data := make([]byte, common.MTU)
	readLength, err := conn.Read(data)
	if  err != nil{
		logger.Errorf("read data: %v", err)
		return
	}

	bc := bridge.Connection{}
	//if err = proto.Unmarshal(data[:readLength], &bc); err != nil{
	tlv, err := common.Unmarshal(data[:readLength])
	if err != nil{
		logger.Errorf("unmarshal data by tlv length %v, data: %v", readLength, err)
	}

	if err = json.Unmarshal(tlv.Value(), &bc); err != nil{
		logger.Errorf("unmarshal data by json length %v, error: %v", tlv.Len(), err)
		return
	}else {
		logger.Infof("unmarshal data length %v, data: %s", tlv.Len(), string(tlv.Value()))
	}

	// 校验是否合法
	v, ok := t.passThroughConns.Load(bc.Header.ConnectionId)
	if  !ok {
		err = fmt.Errorf("illagel connection [%s]", bc.Header.ConnectionId)
		logger.Error(err)
		return
	}

	// TODO: check time

	// 校验类型是否正确
	ptConn := v.(*ppass_through.PassThroughConnection)
	switch bc.Header.Type {
	case bridge.ConnectHeader_CLIENT:
		if ptConn.GrpcClientId != bc.Header.ConnectionId{
			err = fmt.Errorf("the type of connection [%s]  is not illagel, should be client",
				bc.Header.ConnectionId)
			logger.Error(err.Error())
			return
		}
	case bridge.ConnectHeader_SERVER:
		if ptConn.GrpcSrvId != bc.Header.ConnectionId{
			err = fmt.Errorf("the type of connection [%s]  is not illagel, should be server",
				bc.Header.ConnectionId)
			logger.Error(err)
			return
		}
	default:
		err = fmt.Errorf("the type of connection [%s]  is not illagel, it is not supported",
			bc.Header.ConnectionId)
		logger.Error(err)
		return
	}

	// 绑定连接
	if err = t.bind(bc.Header.ConnectionId, conn); err != nil{
		logger.Errorf("bind [%s] with the connection: %v", bc.Header.ConnectionId ,err.Error())
		return
	}

	// 启动桥接
	if err = t.Bridge(ptConn); err != nil{
		logger.Errorf("Bridge %s to %s : %v", ptConn.GrpcClientId, ptConn.GrpcSrvId ,err.Error())
		return
	}else {
		// 桥接成功，记录一个监听事件
		logger.Debugf("Bridge %s to %s success", ptConn.GrpcClientId, ptConn.GrpcSrvId)
		atomic.StoreInt32(&ptConn.Status, int32(ppass_through.PassThroughConnection_TASK_RUNNING))

		data, _ := proto.Marshal(ptConn)
		t.eventManage.Service(bridge.EventRequest_CONNECT_EVENT).CatchEvent(&bridge.Event{
			Type: bridge.EventRequest_CONNECT_EVENT,
			Payload: []byte(data),
		})
	}

	logger.Infof("Success Bridge %s to %s", ptConn.GrpcClientId, ptConn.GrpcSrvId)
	return nil
}

func (t *TcpPassThroughSrv)removeRunningConn(id string){
	if c , ok := t.connRunningSets.Load(id); ok {
		c.(net.Conn).Close()
		t.connRunningSets.Delete(id)
	}
}

func (t *TcpPassThroughSrv)bind(id string, conn net.Conn)error{
	if _, ok := t.connWaitingSets.Load(id); ok {
		return errors.New("the id has been existed")
	}

	if _, ok := t.connRunningSets.Load(id); ok {
		return errors.New("the id has been existed")
	}

	t.connWaitingSets.Store(id, conn)

	// 开启心跳
	//if tc, ok := conn.(*net.TCPConn); ok {
	//	tc.SetKeepAlivePeriod(common.KEEP_ALIVE_DURATION)
	//	tc.SetKeepAlive(true)
	//}

	return nil
}

func (t *TcpPassThroughSrv)switchToRunning(id string) {
	if v, ok := t.connWaitingSets.Load(id); ok{
		t.connWaitingSets.Delete(id)
		t.connRunningSets.Store(id, v)
	}
}

func (t *TcpPassThroughSrv)Bridge(conn *ppass_through.PassThroughConnection) error{
	serverConnID := conn.GrpcSrvId
	clientConnID := conn.GrpcClientId
	grpcSrvConn, ok :=  t.connWaitingSets.Load(serverConnID)
	if !ok {
		return fmt.Errorf("the sid %s is not existed", serverConnID)
	}

	grpcClientConn, ok :=  t.connWaitingSets.Load(clientConnID)
	if !ok {
		return fmt.Errorf("the cid %s is not existed", clientConnID)
	}

	// 转移到正在执行的链接队列中
	t.switchToRunning(serverConnID)
	t.switchToRunning(clientConnID)

	// 服务端的连接和客户端的连接首发缓存都设为 0
	// 目的是为了提高速度
	srvTcpC := grpcSrvConn.(*net.TCPConn)

	if err := srvTcpC.SetWriteBuffer(0); err != nil{
		return err
	}

	if err := srvTcpC.SetReadBuffer(0); err != nil{
		return err
	}

	clientTcpC := grpcClientConn.(*net.TCPConn)
	if err := clientTcpC.SetWriteBuffer(0); err != nil{
		return err
	}

	if err := clientTcpC.SetReadBuffer(0); err != nil{
		return err
	}

	beRunning := make(chan struct{}, 2)
	defer close(beRunning)
	redirect := func(dstID, srcID string, dstC, srcC *net.TCPConn)error {
		defer func() {
			t.removeRunningConn(dstID)
			t.removeRunningConn(srcID)
			atomic.StoreInt32(&conn.Status, int32(ppass_through.PassThroughConnection_TASK_OVER))
			data, _ := proto.Marshal(conn)
			t.eventManage.Service(bridge.EventRequest_CONNECT_EVENT).CatchEvent(&bridge.Event{
				Type: bridge.EventRequest_CONNECT_EVENT,
				Payload: data,
			})
		}()

		beRunning <- struct{}{}
		
		writeSum := 0
		readSum := 0

		for {
			if err := srcC.SetReadDeadline(time.Now().Add(common.READ_TIMEOUT/2)); err != nil{
				return err
			}

			data := make([]byte, common.MTU)
			readLen, err := srcC.Read(data)
			if err != nil{
				// 超時就跳过
				if opErr, ok := err.(*net.OpError); ok{
					if opErr.Timeout(){
						logger.Infof("read from [%s] time out, continue", srcID)
						//dstC.Write([]byte{})
						continue
					}
				}
				logger.Errorf("read from to [%s] error: [%v]", srcID, err)
				return err
			}

			if err := dstC.SetWriteDeadline(time.Now().Add(common.WRITE_TIMEOUT/2)); err != nil{
				return err
			}

			writtenLen, err := dstC.Write(data[:readLen])
			if err != nil{
				// 超时就跳过，数据丢弃掉
				if opErr, ok := err.(*net.OpError); ok{
					if opErr.Timeout(){
						logger.Infof("write to [%s] time out, continue", dstID)
						//srcC.Write([]byte{})
						continue
					}
				}
				logger.Errorf("write to [%s] error: [%v]", dstID, err)
				return err
			}

			if readLen != writtenLen{
				logger.Warningf("from [%s] to [%s]： the write and read not unique",  srcID, dstID)
			}else {
				writeSum += writtenLen
				readSum += readLen
				logger.Debugf("from [%s] to [%s]： data length: [%d] data [%v], sum write data length [%d] and sum read data length [%d]",
					srcID, dstID, readLen, data[:readLen], writeSum, readSum)
			}

		}
		return nil
	}

	// 请求转发
	go redirect(serverConnID, clientConnID, srvTcpC, clientTcpC)

	//应答转发
	go redirect(clientConnID, serverConnID, clientTcpC, srvTcpC)

	<- beRunning
	<- beRunning

	return nil
}
