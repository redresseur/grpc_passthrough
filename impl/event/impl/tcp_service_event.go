package impl

import (
	"code.speakin.mobi/identify/remote_desktop_multi/passthrough.git/impl/event"
	"code.speakin.mobi/identify/remote_desktop_multi/passthrough.git/proto/bridge"
	"github.com/golang/protobuf/proto"
	"github.com/google/uuid"
	"github.com/op/go-logging"
	"sync"
)

var logger *logging.Logger

func init()  {
	logger = logging.MustGetLogger("event/impl")
}

type TcpServiceEventService struct {
	hookers sync.Map
}

func NewTcpServiceEventService() *TcpServiceEventService {
	return &TcpServiceEventService{
		hookers: sync.Map{},
	}
}

func (t *TcpServiceEventService) RegistryEvent(req *bridge.EventRequest,
	c event.CatchFunc)(string, error) {
	eventID := uuid.New().String()
	t.hookers.Store(eventID, &hooker{
		c: c,
		req: req,
	})

	return eventID, nil
}

func (t *TcpServiceEventService)UnRegistryEvent(eventID string) error {
	t.hookers.Delete(eventID)
	return nil
}

func (t *TcpServiceEventService)CatchEvent(e *bridge.Event) (error) {
	ss := &bridge.ServiceStatus{}
	if err := proto.Unmarshal(e.Payload, ss); err != nil{
		return err
	}

	catchFuncs := []event.CatchFunc{}
	q := func(key, value interface{}) bool {
		h := value.(*hooker)
		if string(h.req.Payload) == string(ss.Payload) {
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