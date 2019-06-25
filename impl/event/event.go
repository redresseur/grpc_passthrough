package event

import (
	"code.speakin.mobi/identify/remote_desktop_multi/passthrough.git/proto/bridge"
)

// 用来接收事件
type CatchFunc func (*bridge.Event) error

type Service interface {
	// 注册一个事件处理的hooker
	RegistryEvent(req *bridge.EventRequest,c CatchFunc) (string, error)

	// 注销监听
	UnRegistryEvent(eventID string) error

	// 处理事件
	CatchEvent( event *bridge.Event) error
}

type EmptyService struct {

}

func (*EmptyService) RegistryEvent(req *bridge.EventRequest, c CatchFunc) (string, error) {
	//panic("implement me")
	return "", nil
}

func (*EmptyService) UnRegistryEvent(eventID string) error {
	//panic("implement me")
	return nil
}

func (*EmptyService) CatchEvent(event *bridge.Event) error {
	//panic("implement me")
	return nil
}

type Manager struct {
	ss map[bridge.EventRequest_EventType]Service
}

func (m *Manager)RegistryService(eventType bridge.EventRequest_EventType, s Service){
	m.ss[eventType] = s
}

func (m *Manager)Service( eventType bridge.EventRequest_EventType) Service {
	s, ok := m.ss[eventType]
	if ok{
		return s
	}

	return &EmptyService{}
}

func NewManager()*Manager {
	return &Manager{
		ss: map[bridge.EventRequest_EventType]Service{},
	}
}