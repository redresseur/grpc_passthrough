package main

import (
	"code.speakin.mobi/identify/remote_desktop_multi/passthrough.git/impl"
	"code.speakin.mobi/identify/remote_desktop_multi/passthrough.git/impl/event"
	"code.speakin.mobi/identify/remote_desktop_multi/passthrough.git/pass_through"
	"code.speakin.mobi/identify/remote_desktop_multi/passthrough.git/proto/bridge"
	//"code.speakin.mobi/identify/remote_desktop_multi/passthrough.git/proto/hello"
	//"code.speakin.mobi/identify/remote_desktop_multi/passthrough.git/sdk/server"
	ptimpl "code.speakin.mobi/identify/remote_desktop_multi/passthrough.git/pass_through/impl"
	eimpl "code.speakin.mobi/identify/remote_desktop_multi/passthrough.git/impl/event/impl"
	//"context"

	"fmt"
	"google.golang.org/grpc"
	"net"
)



func main()  {
	var(
		ptAddr = net.TCPAddr{
			IP: net.IPv4(192, 168, 1, 160),
			Port: 9082,
		}

		bridgeAddr = net.TCPAddr{
			IP: net.IPv4(192, 168, 1, 160),
			Port: 9081,
		}

		ptsrv pass_through.PassThroughSrv
		bridgeSrv bridge.BridgeServer

		//serverName = "speakin.mobi:8081"

		// csync = make(chan struct {})
	)

	// 启动透传服务
	em := event.NewManager()
	em.RegistryService(bridge.EventRequest_SERVICE_EVENT, eimpl.NewTcpServiceEventService())
	em.RegistryService(bridge.EventRequest_CONNECT_EVENT, eimpl.NewTcpConnEventService())
	ptsrv = ptimpl.NewTcpPassThroughSrv(&ptAddr, em)
	bridgeSrv = impl.New(em, ptsrv)
	go ptsrv.Listener()


	// 启动桥接服务
	srv := grpc.NewServer()
	bridge.RegisterBridgeServer(srv, bridgeSrv)

	l, err := net.Listen(bridgeAddr.Network(), bridgeAddr.String())
	if err != nil{
		panic(fmt.Sprintf("Create bridge server's listener: %v", err))
	}

	srv.Serve(l)
}
