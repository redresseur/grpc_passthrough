package server

import (
	"code.speakin.mobi/identify/remote_desktop_multi/passthrough.git/proto/hello"
	"context"
	"io"
	"net"
)

type GreaterSrv struct {

}

// Sends a greeting
func (g* GreaterSrv)SayHello(ctx context.Context, req *hello.HelloRequest) (*hello.HelloReply, error){
	return &hello.HelloReply{Message: "recv from " + req.Name}, nil
}
// Sends another greeting
func (g *GreaterSrv)SayHelloAgain(context.Context, *hello.HelloRequest) (*hello.HelloReply, error){
	return nil, nil
}


// A Listener is a generic network listener for stream-oriented protocols.
//
// Multiple goroutines may invoke methods on a Listener simultaneously.
type PTcpListener struct {
	c 	 chan net.Conn
	addr net.Addr

	ctx 	  context.Context
	ctxCancel context.CancelFunc
}

// Any blocked Accept operations will be unblocked and return errors.
func (t *PTcpListener)Close() error{
	t.ctxCancel()
	return nil
}

// Accept waits for and returns the next connection to the listener.
func (t *PTcpListener)Accept()(net.Conn, error)  {
	for {
		select {
		case conn, ok := <- t.c:
			if !ok{
				return nil, io.ErrUnexpectedEOF
			}
			if conn != nil{
				return conn, nil
			}
		case <-t.ctx.Done():
			return nil, io.EOF
		default:
			t.c <- nil
		}
	}
}
// Addr returns the listener's network address.
func (t  *PTcpListener)Addr() net.Addr {
	return t.Addr()
}

// Deliver a tcp conn to the acceptor.
func (t *PTcpListener)Deliver(conn net.Conn){
	t.c <- conn
	return
}

func NewTcpListener(addr net.Addr, ctx context.Context) *PTcpListener {
	res := &PTcpListener{
		addr:addr,
		c: make(chan net.Conn, 1),
	}

	res.ctx, res.ctxCancel = context.WithCancel(ctx)
	return res
}

