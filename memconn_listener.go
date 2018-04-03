package memconn

import (
	"fmt"
	"net"
	"sync"
)

type listener struct {
	addr    Addr
	rcvr    chan net.Conn
	onClose func()

	// activeCnxns is a map of connections that have been received
	// on the rcvr channel. The map is used to close all open
	// connections when the listener's Close function is called.
	// The map's key is the client address's name.
	activeCnxns    map[string]func() error
	activeCnxnsRWL sync.RWMutex
}

type errAddrUnavailable struct{}

func (e errAddrUnavailable) Error() string {
	return "addr unavailable"
}

func (l *listener) dial(
	network string, laddr, raddr Addr) (net.Conn, error) {

	l.activeCnxnsRWL.Lock()
	defer l.activeCnxnsRWL.Unlock()

	// Ensure the laddr has a unique Name.
	if _, ok := l.activeCnxns[laddr.Name]; ok {
		return nil, &net.OpError{
			Source: laddr,
			Addr:   raddr,
			Net:    network,
			Op:     "dial",
			Err:    errAddrUnavailable{},
		}
	}

	// Get two, connected net.Conn objects.
	local, remote := Pipe()

	// Wrap the connections with pipeWrapper so:
	//
	//   * Calls to LocalAddr() and RemoteAddr() return the
	//     correct address information
	//   * Errors returns from the internal pipe are checked and
	//     have their internal OpError addr information replaced with
	//     the correct address information.
	//   * The remote pipe wrapper is able to remove itself from
	//     the list of active connections when the listener's Close
	//     function is called.
	localWrapper, remoteWrapper := &pipeWrapper{
		Conn:       local,
		localAddr:  laddr,
		remoteAddr: raddr,
	}, &pipeWrapper{
		Conn:       remote,
		localAddr:  raddr,
		remoteAddr: laddr,
		onClose: func() {
			l.activeCnxnsRWL.Lock()
			defer l.activeCnxnsRWL.Unlock()
			delete(l.activeCnxns, laddr.Name)
		},
	}

	// Record the connection as active. Please note that
	// the close function recorded is the underlying
	// Conn.Close, not the wrapper's Close. This is because
	// the wrapper's Close function invokes the onClose
	// callback, and that would result in a deadlock due
	// to both l.onPipeClose and l.closeActiveCnxns both
	// trying to obtain a lock on l.activeCnxnsRWL.
	l.activeCnxns[laddr.Name] = remoteWrapper.Conn.Close

	// Announce a new connection.
	go func() {
		l.rcvr <- remoteWrapper
	}()

	return localWrapper, nil
}

func (l *listener) Accept() (net.Conn, error) {
	for serverWrapper := range l.rcvr {
		return serverWrapper, nil
	}
	return nil, fmt.Errorf("listener closed: network=%s, addr=%s",
		l.addr.Network(), l.addr.Name)
}

func (l *listener) Close() error {
	// Close all active connections immediately.
	go func() {
		l.activeCnxnsRWL.Lock()
		defer l.activeCnxnsRWL.Unlock()
		for name, closeNow := range l.activeCnxns {
			go closeNow()
			delete(l.activeCnxns, name)
		}
	}()
	l.onClose()
	return nil
}

func (l *listener) Addr() net.Addr {
	return l.addr
}
