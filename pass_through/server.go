package pass_through

import (
	"code.speakin.mobi/identify/remote_desktop_multi/passthrough.git/proto/pass_through"
	"net"
)

/*
	introduction: 这是一个用于流量实时透传的package
	author: wangzhipeng@speakin.mobi.com
	date: 2019/06/21
*/


type PassThroughSrv interface {
	// 启动监听服务
	Listener() error

	// 为地址绑定唯一id
	//Bind(id string, addr net.Addr)error
	Accept(connection *pass_through.PassThroughConnection) error

	// 桥接
	Addr() net.Addr
}

