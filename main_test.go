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
	"time"
)

var(
	once = sync.Once{}
	ptAddr = net.TCPAddr{
		//IP: net.IPv4(127, 0, 0, 1),
		IP: net.IPv4(192, 168, 1, 160),
		Port: 9082,
	}

	bridgeAddr = net.TCPAddr{
		IP: net.IPv4(192, 168, 1, 160),
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
	client_, err := grpc.Dial(fmt.Sprintf("%s", bridgeAddr.String()), grpc.WithInsecure())
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
		rsp, err :=helloClient.SayHello(context.Background(), &hello.HelloRequest{
			Name:"ok, 阿拉嗦",
		})

		if err != nil{
			t.Fatalf("SayHello %v", err)
		}else {
			t.Logf("Hello Reply %s", rsp.Message)
		}

		time.Sleep(20 * time.Millisecond)
	}

}

func initTest2()  {
	go func(){
		bclient, err := grpc.Dial(fmt.Sprintf("%s", bridgeAddr.String()), grpc.WithInsecure())
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

	connNums := 1
	over := make(chan bool, connNums)
	for i := 0; i < connNums ; i ++{
		go func() {
			// 启动 grpc  hello客户端
			defer func() {
				over <- true
			}()

			client_, err := grpc.Dial(fmt.Sprintf("%s", bridgeAddr.String()), grpc.WithInsecure())
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
				rsp, err :=helloClient.SayHello(ctx, &hello.HelloRequest{
					Name:"ok, 阿拉嗦",
				})

				if err != nil{
					t.Fatalf("SayHello [%d] %v", i,  err)
				}else {
					t.Logf("Hello Reply [%d] %s", i,  rsp.Message)
				}
				time.Sleep(20 * time.Millisecond)
			}
		}()
	}

	for i := 0; i < connNums; i++{
		<- over
	}

}


func BenchmarkCollection(b *testing.B)  {
	once.Do(initTest)
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

	for i := 0; i < 1; i++ {
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

//
//func Benchmark_Main(b *testing.B)  {
//	once.Do(initTest)
//	srv := grpc.NewServer()
//	_proto.RegisterGreeterServer(srv ,&impl.GreaterSrv{})
//
//	pttl := impl.NewTcpListener(&addr, context.Background())
//
//	go srv.Serve(pttl)
//
//	var grpcClientID, grpcSrvID string
//	var grpcClient _proto.GreeterClient
//
//	// grpc 服务端连接透传服务器
//	if srvConn, err := net.DialTCP(addr.Network(), nil, &addr); err !=nil{
//		panic(err)
//	}else {
//		id := make([]byte, 64)
//		if len, err := srvConn.Read(id); err != nil{
//			panic(err)
//		}else {
//			grpcSrvID = string(id[:len])
//		}
//
//		pttl.Deliver(srvConn)
//	}
//
//	//time.Sleep(3*time.Second)
//	// grpc 客户端连接透传服务器
//	dialer := func(context.Context, string) (net.Conn, error){
//		conn, err := net.Dial(addr.Network(), addr.String())
//
//		if err !=nil{
//			return nil, err
//		}
//		id := make([]byte, 64)
//		if len, err := conn.Read(id); err != nil{
//			return nil, err
//		}else {
//			grpcClientID = string(id[:len])
//		}
//
//		if err := ptsrv.Bridge(grpcClientID, grpcSrvID); err != nil{
//			return nil, err
//		}
//		return  conn, nil
//	}
//	cc, err := grpc.Dial( `grpc://` + addr.String(), grpc.WithInsecure(), grpc.WithContextDialer(dialer))
//	if err != nil{
//		b.Fatalf("connect %v", err)
//	}
//	grpcClient = _proto.NewGreeterClient(cc)
//
//	// time.Sleep(1*time.Second)
//	for i := 0; i< b.N; i++{
//		_, err := grpcClient.SayHello(context.Background(),
//			&_proto.HelloRequest{Name: fmt.Sprintf("%s_%d", grpcClientID, i),})
//		if err != nil{
//			b.Fatalf("say hello %v", err)
//		}
//		//b.Log(resp.Message)
//	}
//
//}