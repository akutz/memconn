package memconn

import (
	"context"
	"errors"
	"net"
)

// listener implements the net.Listener interface.
type listener struct {
	addr Addr
	rcvr chan net.Conn
	done chan struct{}
}

func (l listener) dial(
	ctx context.Context,
	network string,
	laddr, raddr Addr) (net.Conn, error) {

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
	//     closing closes the remoteConn immediately.
	localConn, remoteConn := &Conn{
		Conn:       local,
		localAddr:  laddr,
		remoteAddr: raddr,
	}, &Conn{
		Conn:       remote,
		localAddr:  raddr,
		remoteAddr: laddr,
	}

	// Start a goroutine that closes the remote side of the connection
	// as soon as the listener's done channel is no longer blocked.
	go func() {
		<-l.done
		remoteConn.Close()
	}()

	// Announce a new connection by placing the new remoteConn
	// onto the rcvr channel. An Accept call from this listener will
	// remove the remoteConn from the channel. However, if that does
	// not occur by the time the context times out / is cancelled, then
	// an error is returned.
	select {
	case l.rcvr <- remoteConn:
		return localConn, nil
	case <-ctx.Done():
		localConn.Close()
		remoteConn.Close()
		return nil, &net.OpError{
			Addr:   raddr,
			Source: laddr,
			Net:    network,
			Op:     "dial",
			Err:    ctx.Err(),
		}
	}
}

// Accept implements the net.Listener Accept method.
func (l listener) Accept() (net.Conn, error) {
	// Loop until a new connection is received from the
	// rcvr channel or until the listener is closed.
	for {
		select {
		case remoteConn := <-l.rcvr:
			return remoteConn, nil
		case <-l.done:
			return nil, &net.OpError{
				Addr:   l.addr,
				Source: l.addr,
				Net:    l.addr.Network(),
				Err:    errors.New("listener closed"),
			}
		}
	}
}

// Close implements the net.Listener Close method.
func (l listener) Close() error {
	if !isClosedChan(l.done) {
		close(l.done)
	}
	return nil
}

// Addr implements the net.Listener Addr method.
func (l listener) Addr() net.Addr {
	return l.addr
}
