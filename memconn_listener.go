package memconn

import (
	"net"
)

type listener struct {
	addr    Addr
	rcvr    chan net.Conn
	onClose func()
	done    chan struct{}
}

func (l *listener) dial(
	network string, laddr, raddr Addr) (net.Conn, error) {

	// Get two, connected net.Conn objects.
	local, remote := Pipe()

	// Wrap the connections with pipeWrapper so:
	//
	//   * Calls to LocalAddr() and RemoteAddr() return the
	//     correct address information
	//   * Errors returns from the internal pipe are checked and
	//     have their internal OpError addr information replaced with
	//     the correct address information.
	//   * A channel can be setup to cause the event of the Listener
	//     closing closes the remoteWrapper immediately.
	localWrapper, remoteWrapper := &pipeWrapper{
		Conn:       local,
		localAddr:  laddr,
		remoteAddr: raddr,
	}, &pipeWrapper{
		Conn:       remote,
		localAddr:  raddr,
		remoteAddr: laddr,
	}

	// Start a goroutine that closes the remote side of the connection
	// as soon as the listener's done channel is no longer blocked.
	go func() {
		<-l.done
		remoteWrapper.Close()
	}()

	// Announce a new connection.
	go func() {
		l.rcvr <- remoteWrapper
	}()

	return localWrapper, nil
}

type errListenerClosed struct{}

func (e errListenerClosed) Error() string {
	return "closed"
}

func (l *listener) Accept() (net.Conn, error) {
	if !isClosedChan(l.done) {
		for serverWrapper := range l.rcvr {
			return serverWrapper, nil
		}
		// Notify pending remote endpoints that the listener is closed
		// and that all pending operations should be unblocked.
		close(l.done)
	}
	return nil, &net.OpError{
		Addr:   l.addr,
		Source: l.addr,
		Net:    l.addr.Network(),
		Err:    errListenerClosed{},
	}
}

func (l *listener) Close() error {
	l.onClose()
	return nil
}

func (l *listener) Addr() net.Addr {
	return l.addr
}
