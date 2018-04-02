package memconn

import "net"

type pipeWrapper struct {
	net.Conn
	localAddr  net.Addr
	remoteAddr net.Addr
}

func (p pipeWrapper) LocalAddr() net.Addr {
	return p.localAddr
}

func (p pipeWrapper) RemoteAddr() net.Addr {
	return p.remoteAddr
}
