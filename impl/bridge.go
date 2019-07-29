package impl

import (
	"code.speakin.mobi/identify/remote_desktop_multi/passthrough.git/impl/event"
	"code.speakin.mobi/identify/remote_desktop_multi/passthrough.git/pass_through"
	"code.speakin.mobi/identify/remote_desktop_multi/passthrough.git/pass_through/impl"
	"code.speakin.mobi/identify/remote_desktop_multi/passthrough.git/proto/bridge"
	ppass_through "code.speakin.mobi/identify/remote_desktop_multi/passthrough.git/proto/pass_through"
	"github.com/op/go-logging"

	"context"
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/pborman/uuid"
	"github.com/redresseur/utils/utime"
	"io"
	"net/http"
	"sync"
	"time"
)

var logger *logging.Logger

func init()  {
	logger = logging.MustGetLogger("impl/bridge")
}

type ServerEndPoint struct {
	*bridge.RegisterRequest
	notify chan *bridge.ConnectNotify
	createTimeTamp *timestamp.Timestamp
}

type Bridge struct {
	// 记录所有注册的服务
	serviceList sync.Map

	//
	// commitConn func(task *pass_through.BridgeConnectionTask) error
	eventManger *event.Manager

	passThroughSrv pass_through.PassThroughSrv
}

func New( manager *event.Manager,
	srv pass_through.PassThroughSrv) *Bridge{
	return &Bridge{
		passThroughSrv: srv,
		eventManger:manager,
		serviceList: sync.Map{},
	}
}

func (b *Bridge)RegisterEvent(req *bridge.EventRequest,
	srv bridge.Bridge_RegisterEventServer)(err error){

	c := func (e *bridge.Event) error{
		return srv.Send(e)
	}

	switch req.Type {
	case bridge.EventRequest_SERVICE_EVENT:
		if eventId, err := b.eventManger.Service(req.Type).RegistryEvent(req, c); err != nil{
			return err
		}else {
			defer b.eventManger.Service(req.Type).UnRegistryEvent(eventId)
		}
	case bridge.EventRequest_CONNECT_EVENT:
		if eventId, err := b.eventManger.Service(req.Type).RegistryEvent(req, c); err != nil{
			return err
		}else {
			defer  b.eventManger.Service(req.Type).UnRegistryEvent(eventId)
		}
	default:
		err = fmt.Errorf("the type [%v] is not supported", req.Type)
		return
	}

	<-srv.Context().Done()
	return nil
}

func (b *Bridge)Query(ctx context.Context, request *bridge.QueryRequest) (*bridge.Respond, error){
	switch request.Type {
	case bridge.QueryRequest_LIST_SERVICES:
		res := b.queryAllServiceName()
		payload, _ := proto.Marshal(res)
		return Success(payload), nil
	case bridge.QueryRequest_SERVICE_SEQ:
		seq, s := b.queryWithServiceName(string(request.Condition));
		if  s == nil{
			return Errorf(http.StatusNotFound,
				"the service [%s] has been not existed", string(request.Condition) ), nil
		}else {
			// 返回当前服务的序列号
			return Success([]byte(seq)), nil
		}
	default:
		return Errorf(http.StatusBadRequest,
			"the query type [%d] is not supported", request.Type), nil
	}

	return nil, nil
}

func (b *Bridge)queryAllServiceName()(*bridge.QueryResult){
	res := &bridge.QueryResult{}
	q := func(key, value interface{}) bool {
		if s, ok := value.(*ServerEndPoint); ok {
			res.Data = append(res.Data, []byte(s.ServiceName))
		}
		return true
	}

	b.serviceList.Range(q)
	return res
}

func (b *Bridge)queryWithSeqNum(seqNum string)(res *ServerEndPoint){
	if v, ok := b.serviceList.Load(seqNum); ok{
		res = v.(*ServerEndPoint)
	}

	return
}

func (b *Bridge)queryWithServiceName(name string)(seq string, res *ServerEndPoint) {
	q := func(key, value interface{}) bool {
		if sp, ok := value.(*ServerEndPoint); ok {
			if sp.ServiceName == name{
				seq = key.(string)
				res = value.(*ServerEndPoint)
				return false
			}
		}

		return true
	}

	b.serviceList.Range(q)
	return
}

func Success(payload []byte)*bridge.Respond{
	return &bridge.Respond{
		StatusCode: http.StatusOK,
		Payload:payload,
	}
}

func Errorf(errCode int32, format string, args ...interface{} ) *bridge.Respond {
	return &bridge.Respond{
		StatusCode: errCode,
		ErrMsg: fmt.Sprintf(format, args),
	}
}

func (b *Bridge)RegisterService(ctx context.Context,
	req *bridge.RegisterRequest) (*bridge.Respond, error)  {
	seq := uuid.NewUUID().String()

	if _, sp := b.queryWithServiceName(req.ServiceName); nil != sp{
		return Errorf(http.StatusBadRequest,
			"this service %s has been registered",
			req.ServiceName), nil
	}

	srvEndPoint := &ServerEndPoint{
		RegisterRequest:req,
		notify: make(chan *bridge.ConnectNotify, 0),
	}
	b.serviceList.Store(seq, srvEndPoint)

	// 记录一个服务创建的事件
	data, _ := proto.Marshal(&bridge.ServiceStatus{
		StatusCode: bridge.ServiceStatus_CREATE,
		Payload: []byte(req.ServiceName),
	})
	b.eventManger.Service(bridge.EventRequest_SERVICE_EVENT).CatchEvent(&bridge.Event{
		Type: bridge.EventRequest_SERVICE_EVENT,
		Payload: data,
	})

	return Success([]byte(seq)), nil
}

func (b *Bridge)Listener(req *bridge.ListenerRequest, srv bridge.Bridge_ListenerServer) error{
	srvEndPoint := b.queryWithSeqNum(req.ServiceSequence)
	if srvEndPoint == nil{
		return fmt.Errorf("the seqnum %s is invaild", req.ServiceSequence)
	}

	// 记录一个服务监听的事件
	data, _ := proto.Marshal(&bridge.ServiceStatus{
		StatusCode: bridge.ServiceStatus_ON,
		Payload: []byte(req.ServiceSequence),
	})
	b.eventManger.Service(bridge.EventRequest_SERVICE_EVENT).CatchEvent(&bridge.Event{
		Type: bridge.EventRequest_SERVICE_EVENT,
		Payload: data,
	})

	defer func() {
		// 记录一个服务监听的事件
		data, _ := proto.Marshal(&bridge.ServiceStatus{
			StatusCode: bridge.ServiceStatus_OFF,
			Payload: []byte(req.ServiceSequence),
		})
		b.eventManger.Service(bridge.EventRequest_SERVICE_EVENT).CatchEvent(&bridge.Event{
			Type: bridge.EventRequest_SERVICE_EVENT,
			Payload: data,
		})

		b.serviceList.Delete(req.ServiceSequence)
	}()

	for  {
		select {
		case <-srv.Context().Done():
			logger.Infof("the service [%s] listener over", srvEndPoint.ServiceName)
			return nil
		case c , ok := <-srvEndPoint.notify:
			if !ok {
				return io.ErrUnexpectedEOF
			}
			if err := srv.Send(c); err != nil{
				return err
			}
		}
	}

	return nil
}

func (b *Bridge) Connect(ctx context.Context, req *bridge.ConnectRequest) (*bridge.Respond, error){
	srvEndPoint := b.queryWithSeqNum(req.ServiceSequence)
	if srvEndPoint == nil{
		return Errorf(http.StatusBadRequest,
			"the sequence_num [%d] was invalid",
			req.ServiceSequence) ,nil
	}

	// 生成一对约定的 连接ID
	// 服务端tcp 连接时携带 srvConnID
	// 客户端tcp 连接时携带 clientConnID
	srvConnID := uuid.NewUUID().String()

	clientConnID := uuid.NewUUID().String()
	logger.Infof("sid %s, cid %s", srvConnID, clientConnID)

	b.passThroughSrv.Accept(&ppass_through.PassThroughConnection{
		GrpcSrvId:    srvConnID,
		GrpcClientId: clientConnID,
		CreateTime:   utime.CreateUtcTimestamp(),
		Status:       int32(ppass_through.PassThroughConnection_TASK_WAITING),
	})

	addr := b.passThroughSrv.Addr()
	tlsEnable := false
	if pttSrv, ok := b.passThroughSrv.(*impl.TcpPassThroughSrv); ok{
		tlsEnable = pttSrv.TlsEnable()
	}
	srvEndPoint.notify <- &bridge.ConnectNotify{
		ConnectionId: srvConnID,
		PassThroughAddr: addr.String(),
		PassThroughType: addr.Network(),
		TlsEnable: 	tlsEnable,
	}

	// 返回 用于 grpc 连接的 id
	payload, _ := proto.Marshal(&bridge.ConnectNotify{
		ConnectionId: clientConnID,
		PassThroughAddr: addr.String(),
		PassThroughType: addr.Network(),
		TlsEnable: tlsEnable,
	})

	return Success(payload), nil
}

func (b *Bridge)PingPong(ctx context.Context, ping *bridge.Ping) (*bridge.Pong, error)  {
	// 根据延迟时间判断当前的网络状态
	// 必须全部为utc时间
	pingTime := time.Unix(ping.Timestamp.Seconds, int64(ping.Timestamp.Nanos))
	nowTime := time.Now()

	// 判断时间戳是否有效
	if !nowTime.After(pingTime) {
		return nil, fmt.Errorf("the timestamp of <id: seq> [%s : %d] was invalid",
			ping.Id, ping.SequenceNum)
	}

	return &bridge.Pong{
		Timestamp: utime.GenerateTimestamp(nowTime),
		SequenceNum: ping.SequenceNum,
		Id: ping.Id,
	}, nil
}


