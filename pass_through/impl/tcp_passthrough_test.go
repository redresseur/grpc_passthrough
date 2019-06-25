package impl

import (
	"code.speakin.mobi/identify/remote_desktop_multi/passthrough.git/proto/pass_through"
	"code.speakin.mobi/identify/remote_desktop_multi/passthrough.git/pass_through"
	"github.com/golang/protobuf/proto"
	"github.com/google/uuid"

	//"code.speakin.mobi/identify/remote_desktop_multi/passthrough.git/service"
	"fmt"
	"golang.org/x/net/http2"
	"io"
	"net"
	"sync"
	"testing"
)

var (
	addr  = net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 9082 }
	srv *TcpPassThroughSrv
	once = sync.Once{}
)

func initPttTest()  {
	srv = NewTcpPassThroughSrv(&addr)

}

func TestTcpPassThroughSrv_Listener(t *testing.T) {
	once.Do(initPttTest)
	go func() {
		srv.Listener()
	}()

	commit := srv.CommitTaskFunc()

	srcID := uuid.New().String()
	dstID := uuid.New().String()
	commit(&pass_through.BridgeConnectionTask{
		GrpcClientID: srcID,
		GrpcSrvID: dstID,
	})

	srcConn, err := net.DialTCP(addr.Network(), nil, &addr)
	if err !=nil{
		panic(err)
	}else {
		c := pass_through.BridgeConnection{
			Header: &pass_through.BridgeConnectHeader{
				ConnectionId: srcID,
				Type: pass_through.BridgeConnectHeader_CLIENT,
			},
		}
		data, _ := proto.Marshal(&c)
		srcConn.Write(data)
	}

	dstConn, err := net.DialTCP(addr.Network(), nil, &addr)
	if err !=nil{
		panic(err)
	}else {
		c := pass_through.BridgeConnection{
			Header: &pass_through.BridgeConnectHeader{
				ConnectionId: dstID,
				Type: pass_through.BridgeConnectHeader_SERVER,
			},
		}
		data, _ := proto.Marshal(&c)
		dstConn.Write(data)
	}

	go srcConn.Write([]byte(http2.ClientPreface))
	data := make([]byte, len(http2.ClientPreface))
	dstConn.Read(data)
	t.Logf("read the data: %s ", string(data))

	go dstConn.Write([]byte(http2.ClientPreface))
	data = make([]byte, len(http2.ClientPreface))
	if _, err := io.ReadFull(srcConn, data); err != nil{
		t.Fatalf("read the data error %v", err)
	}else {
		t.Logf("read the data: %s ", string(data))
	}
}


func TestTcpPassThroughSrv_Echo(t *testing.T) {
	once.Do(initPttTest)
	go func() {
		srv.Listener()
	}()

	var srcID string
	srcConn, err := net.DialTCP(addr.Network(), nil, &addr)
	if err !=nil{
		panic(err)
	}else {
		id := make([]byte, 64)
		if len, err := srcConn.Read(id); err != nil{
			panic(err)
		}else {
			srcID = string(id[:len])
		}
	}

	go srv.Bridge(&pass_through.BridgeConnectionTask{
		GrpcClientID: srcID,
		GrpcSrvID: srcID,
	})

	go func() {
		srcConn.Write([]byte("test"))
	}()

	data := make([]byte, len("test"))
	srcConn.Read(data)
	t.Logf("read the data: %s ", string(data))
}

func TestTcpPassThroughSrv_Common(t *testing.T) {
	// Listen on TCP port 2000 on all available unicast and
	// anycast IP addresses of the local system.
	l, err := net.Listen(addr.Network(), addr.String())
	if err != nil {
		t.Fatal(err)
	}
	//defer l.Close()

	go func() {
		srcConn, err := net.DialTCP(addr.Network(), nil, &addr)
		if err !=nil{
			panic(err)
		}

		go func() {
			srcConn.Write([]byte("test"))
		}()

		data := make([]byte, 64)
		srcConn.Read(data)
		fmt.Printf("read the data: %s ", string(data))
		data = make([]byte, len("test"))
		srcConn.Read(data)
		fmt.Printf("read the data: %s ", string(data))
	}()

	for {
		// Wait for a connection.
		conn, err := l.Accept()
		if err != nil {
			t.Fatal(err)
		}
		// Handle the connection in a new goroutine.
		// The loop then returns to accepting, so that
		// multiple connections may be served concurrently.
		go func(c net.Conn) {
			// Echo all incoming data.
			c.Write([]byte("test"))
			io.Copy(c, c)
			// Shut down the connection.
			//c.Close()
		}(conn)
	}
}
