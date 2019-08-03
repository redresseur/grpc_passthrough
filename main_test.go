package main

import (
	"code.speakin.mobi/identify/remote_desktop_multi/passthrough.git/common"
	"code.speakin.mobi/identify/remote_desktop_multi/passthrough.git/impl"
	"code.speakin.mobi/identify/remote_desktop_multi/passthrough.git/impl/event"
	eimpl "code.speakin.mobi/identify/remote_desktop_multi/passthrough.git/impl/event/impl"
	"code.speakin.mobi/identify/remote_desktop_multi/passthrough.git/pass_through"
	ptimpl "code.speakin.mobi/identify/remote_desktop_multi/passthrough.git/pass_through/impl"
	"code.speakin.mobi/identify/remote_desktop_multi/passthrough.git/proto/bridge"
	"code.speakin.mobi/identify/remote_desktop_multi/passthrough.git/proto/hello"
	"code.speakin.mobi/identify/remote_desktop_multi/passthrough.git/sdk/client"
	"code.speakin.mobi/identify/remote_desktop_multi/passthrough.git/sdk/server"
	"context"
	"fmt"
	"google.golang.org/grpc"
	"net"
	"sync"
	"testing"
)

var(
	once = sync.Once{}
	ptAddr = net.TCPAddr{
		//IP: net.IPv4(127, 0, 0, 1),
		IP: net.IPv4(127, 0, 0, 1),
		Port: 9082,
	}

	bridgeAddr = net.TCPAddr{
		IP: net.IPv4(127, 0, 0, 1),
		Port: 9081,
	}

	ptsrv pass_through.PassThroughSrv
	bridgeSrv bridge.BridgeServer

	serverName = "speakin.mobi:8081"

	csync = make(chan struct {})
)

func initTest()  {
	// 启动透传服务
	em := event.NewManager()
	em.RegistryService(bridge.EventRequest_SERVICE_EVENT, eimpl.NewTcpServiceEventService())
	em.RegistryService(bridge.EventRequest_CONNECT_EVENT, eimpl.NewTcpConnEventService())
	ptsrv = ptimpl.NewTcpPassThroughSrv(&ptAddr, em)
	bridgeSrv = impl.New(em, ptsrv)
	go ptsrv.Listener()


	// 启动桥接服务
	go func() {
		srv := grpc.NewServer()
		bridge.RegisterBridgeServer(srv, bridgeSrv)

		l, err := net.Listen(bridgeAddr.Network(), bridgeAddr.String())
		if err != nil{
			panic(fmt.Sprintf("Create bridge server's listener: %v", err))
		}

		srv.Serve(l)
	}()


	// 启动 grpc hello 服务
	go func() {
		client, err := grpc.Dial(fmt.Sprintf("%s", bridgeAddr.String()), grpc.WithInsecure())
		if err != nil{
			panic(fmt.Sprintf("Dial bridge server's listener: %v", err))
		}

		// 注册服务
		tcpListener := server.NewTcpListener(&ptAddr, context.Background())
		bridgeClient := bridge.NewBridgeClient(client)
		helloSrv, err := server.RegistryService(bridgeClient, serverName, tcpListener)
		if err != nil{
			panic(fmt.Sprintf("RegistryService [%s] listener: %v", serverName, err))
		}

		csync <- struct{}{}

		// 启动 服务
		grpcSrv := grpc.NewServer()
		hello.RegisterGreeterServer(grpcSrv, &server.GreaterSrv{})

		if err := server.RunService(bridgeClient, helloSrv, grpcSrv, context.Background()); err != nil{
			panic(fmt.Sprintf("RunService [%v]", err))
		}
	}()
}

// 以下用于测试整个工程的功能 和 性能

// 测试单个链接
func TestCollection(t *testing.T)  {
	once.Do(initTest2)

	<- csync
	// 启动 grpc  hello客户端
	client_, err := grpc.Dial(fmt.Sprintf("%s", bridgeAddr.String()),
		grpc.WithBlock(), grpc.WithInsecure())

	if err != nil{
		t.Fatalf("Dial bridge server's listener: %v", err)
	}

	bridgeClient := bridge.NewBridgeClient(client_)
	grpcClient, err := client.Connect(bridgeClient, serverName)
	if err != nil{
		t.Fatalf("Connect to hello server %v", err)
	}

	helloClient := hello.NewGreeterClient(grpcClient)

	for i:=0 ; i < 10000; i++{
		ctx, _ := context.WithTimeout(context.Background(), common.WRITE_TIMEOUT)
		//ctx := context.Background()
		rsp, err :=helloClient.SayHello(ctx, &hello.HelloRequest{
			Name:"ok, 阿拉嗦",
		})

		if err != nil{
			t.Fatalf("SayHello %d %v", i, err)
		}else {
			t.Logf("Hello Reply %d %s", i, rsp.Message)
		}

		//time.Sleep(20 * time.Millisecond)
	}

}

func initTest2()  {
	go func(){
		bclient, err := grpc.Dial(fmt.Sprintf("%s", bridgeAddr.String()),
			grpc.WithInsecure(),
			grpc.WithBlock())
		if err != nil {
			panic(fmt.Sprintf("Dial bridge server's listener: %v", err))
		}

		// 注册服务
		tcpListener := server.NewTcpListener(&ptAddr, context.Background())
		bridgeClient := bridge.NewBridgeClient(bclient)
		helloSrv, err := server.RegistryService(bridgeClient, serverName, tcpListener)
		if err != nil {
			panic(fmt.Sprintf("RegistryService [%s] listener: %v", serverName, err))
		}

		csync <- struct{}{}

		// 启动 服务
		grpcSrv := grpc.NewServer()
		hello.RegisterGreeterServer(grpcSrv, &server.GreaterSrv{})

		if err := server.RunService(bridgeClient, helloSrv, grpcSrv, context.Background()); err != nil {
			panic(fmt.Sprintf("RunService [%v]", err))
		}

	}()
}

// 测试多个链接
func TestCollections(t *testing.T)  {
	once.Do(initTest2)

	<- csync

	connNums := 10
	over := make(chan bool, connNums)
	for i := 0; i < connNums ; i ++{
		go func() {
			// 启动 grpc  hello客户端
			defer func() {
				over <- true
			}()

			client_, err := grpc.Dial(fmt.Sprintf("%s", bridgeAddr.String()),
				grpc.WithBlock(),
				grpc.WithInsecure())

			if err != nil{
				t.Fatalf("Dial bridge server's listener: %v", err)
			}

			bridgeClient := bridge.NewBridgeClient(client_)
			grpcClient, err := client.Connect(bridgeClient, serverName)
			if err != nil{
				t.Fatalf("Connect to hello server %v", err)
			}

			helloClient := hello.NewGreeterClient(grpcClient)

			for i:=0 ; i < 1000; i++{
				ctx, _ := context.WithTimeout(context.Background(), common.WRITE_TIMEOUT)
				rsp, err :=helloClient.SayHello(ctx, &hello.HelloRequest{
					Name:"ok, 阿拉嗦",
				})

				if err != nil{
					t.Fatalf("SayHello [%d] %v", i,  err)
				}else {
					t.Logf("Hello Reply [%d] %s", i,  rsp.Message)
				}
				//time.Sleep(20 * time.Millisecond)
			}
		}()
	}

	for i := 0; i < connNums; i++{
		<- over
	}

}


func BenchmarkCollection(b *testing.B)  {
	once.Do(initTest2)
	<- csync
	// 启动 grpc  hello客户端
	client_, err := grpc.Dial(fmt.Sprintf("%s", bridgeAddr.String()), grpc.WithInsecure())
	if err != nil{
		b.Fatalf("Dial bridge server's listener: %v", err)
	}else {
		defer  client_.Close()
	}

	bridgeClient := bridge.NewBridgeClient(client_)
	grpcClient, err := client.Connect(bridgeClient, serverName)
	if err != nil{
		b.Fatalf("Connect to hello server %v", err)
	}

	helloClient := hello.NewGreeterClient(grpcClient)

	for i := 0; i < b.N; i++ {
		rsp, err :=helloClient.SayHello(context.Background(), &hello.HelloRequest{
			Name:"ok, 阿拉嗦",
		})

		if err != nil{
			b.Fatalf("SayHello %v", err)
		}else {
			b.Logf("Hello Reply %s", rsp.Message)
		}
	}

}

func TestIpv4(t *testing.T){
	conn, err := net.ListenIP("ip:tcp", &net.IPAddr{IP: net.IPv4(127,0,0,1)})
	if err != nil{
		t.Fatalf("ListenIP %v", err)
	}
	go func() {
		net.DialTCP("tcp", nil, &net.TCPAddr{
			IP: net.IPv4(127, 0, 0, 1),
			Port: 9081,
		})
	}()
	for  {
		data := make([]byte, common.MTU)
		if readLen, err := conn.Read(data); err != nil{
			t.Logf("ip read <%d, %v>", readLen, data[:readLen])
		}
	}
}