package impl

import (
	"code.speakin.mobi/identify/remote_desktop_multi/passthrough.git/impl/event"
	"code.speakin.mobi/identify/remote_desktop_multi/passthrough.git/proto/bridge"
	"code.speakin.mobi/identify/remote_desktop_multi/passthrough.git/proto/pass_through"
	"github.com/golang/protobuf/proto"
	"github.com/google/uuid"
	"sync"
)

type hooker struct {
	req *bridge.EventRequest
	c event.CatchFunc
}

type TcpConnEventService struct {
	hookers sync.Map
}

func NewTcpConnEventService() *TcpConnEventService {
	return &TcpConnEventService{
		hookers: sync.Map{},
	}
}

func (t *TcpConnEventService) RegistryEvent(req *bridge.EventRequest,
	c event.CatchFunc)(string, error) {
	eventID := uuid.New().String()
	t.hookers.Store(eventID, &hooker{
		c: c,
		req: req,
	})

	return eventID, nil
}

func (t *TcpConnEventService)UnRegistryEvent(eventID string) error {
	t.hookers.Delete(eventID)
	return nil
}

func (t *TcpConnEventService)CatchEvent(e *bridge.Event) error {
	ptConn := &pass_through.PassThroughConnection{}
	if err := proto.Unmarshal(e.Payload, ptConn); err != nil{
		return err
	}

	catchFuncs := []event.CatchFunc{}
	q := func(key, value interface{}) bool {
		h := value.(*hooker)
		if string(h.req.Payload) == ptConn.GrpcClientId ||
			string(h.req.Payload) == ptConn.GrpcSrvId{
			catchFuncs = append(catchFuncs, h.c)
		}
		return true
	}

	t.hookers.Range(q)

	cc := func(c event.CatchFunc, e *bridge.Event)(err error ){
		defer func() {
			if r := recover(); r != nil{
				if defErr, ok := r.(error); ok{
					err = defErr
				}
			}
		}()
		err = c(e)
		return
	}

	for _, c := range catchFuncs{
		if err := cc(c, e); err != nil{
			logger.Warningf("send service event: [%v]", err)
			continue
		}
	}


	return nil
}
