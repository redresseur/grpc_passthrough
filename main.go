package main

import (
	"code.speakin.mobi/identify/remote_desktop_multi/passthrough.git/impl"
	"code.speakin.mobi/identify/remote_desktop_multi/passthrough.git/impl/event"
	eimpl "code.speakin.mobi/identify/remote_desktop_multi/passthrough.git/impl/event/impl"
	"code.speakin.mobi/identify/remote_desktop_multi/passthrough.git/pass_through"
	ptimpl "code.speakin.mobi/identify/remote_desktop_multi/passthrough.git/pass_through/impl"
	"code.speakin.mobi/identify/remote_desktop_multi/passthrough.git/proto/bridge"
	"context"
	"fmt"
	"github.com/redresseur/utils"
	"github.com/redresseur/utils/ioutils"
	"google.golang.org/grpc"
	"net"
	"os"
	"runtime/pprof"
)


func main()  {
	var(
		ptAddr = net.TCPAddr{
			IP: net.IPv4(127, 0, 0, 1),
			Port: 9082,
		}

		bridgeAddr = net.TCPAddr{
			IP: net.IPv4(127, 0, 0, 1),
			Port: 9081,
		}

		ptsrv pass_through.PassThroughSrv
		bridgeSrv bridge.BridgeServer

		//serverName = "speakin.mobi:8081"

		// csync = make(chan struct {})
	)

	os.RemoveAll("cpu.profile")
	fs, err := ioutils.OpenFile("cpu.profile","")
	if err != nil{
		panic(err)
	}else {
		// defer fs.Close()
	}

	if err := pprof.StartCPUProfile(fs); err != nil{
		panic(err)
	}else {
		// defer
	}

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

	go srv.Serve(l)
	utils.WatchSignal(map[os.Signal]func(){
		os.Interrupt: func() {
			pprof.StopCPUProfile()
			fs.Close()
			os.Exit(1)
		},
		os.Kill : func() {
			pprof.StopCPUProfile()
			fs.Close()
			os.Exit(1)
		},
	}, context.Background())
}
