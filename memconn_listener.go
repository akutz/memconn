package memconn

import (
	"fmt"
	"net"
)

type listener struct {
	addr  Addr
	cnxns chan net.Conn
	close func(Addr) error
}

func (l listener) dial(
	network string, laddr, raddr net.Addr) (net.Conn, error) {

	// Get two, connected net.Conn objects.
	local, remote := Pipe()

	// Wrap the connections with pipeWrapper so that calls to
	// LocalAddr() and RemoteAddr() return the correct address
	// information.
	localWrapper, remoteWrapper := &pipeWrapper{
		Conn:       local,
		localAddr:  laddr,
		remoteAddr: raddr,
	}, &pipeWrapper{
		Conn:       remote,
		localAddr:  raddr,
		remoteAddr: laddr,
	}

	// Announce a new connection.
	go func() {
		l.cnxns <- remoteWrapper
	}()

	return localWrapper, nil
}

func (l listener) Accept() (net.Conn, error) {
	for serverWrapper := range l.cnxns {
		return serverWrapper, nil
	}
	return nil, fmt.Errorf("listener closed: network=%s, addr=%s",
		l.addr.Network(), l.addr.Name)
}

func (l listener) Close() error {
	return l.close(l.addr)
}

func (l listener) Addr() net.Addr {
	return l.addr
}
