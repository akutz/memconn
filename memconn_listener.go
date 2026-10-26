package memconn

import (
	"context"
	"errors"
	"net"
	"sync"
)

// Listener implements the net.Listener interface.
type Listener struct {
	addr Addr
	once sync.Once
	rcvr chan *Conn
	done chan struct{}
	rmvd chan struct{}
}

func (l *Listener) dial(
	ctx context.Context,
	network string,
	laddr, raddr Addr) (*Conn, error) {

	local, remote := makeNewConns(network, laddr, raddr)

	// TODO Figure out if this logic is valid.
	//
	// Start a goroutine that closes the remote side of the connection
	// as soon as the listener's done channel is no longer blocked.
	//go func() {
	//	<-l.done
	//	remoteConn.Close()
	//}()

	// A nil context means no caller deadline, but listener shutdown must
	// still interrupt a dial waiting for space in the accept queue.
	if ctx == nil {
		ctx = context.Background()
	}

	var err error
	select {
	case l.rcvr <- remote:
		return local, nil
	case <-ctx.Done():
		err = ctx.Err()
	case <-l.done:
		err = errors.New("listener closed")
	}

	local.Close()
	remote.Close()
	return nil, &net.OpError{
		Addr:   raddr,
		Source: laddr,
		Net:    network,
		Op:     "dial",
		Err:    err,
	}
}

// Accept implements the net.Listener Accept method.
func (l *Listener) Accept() (net.Conn, error) {
	return l.AcceptMemConn()
}

// AcceptMemConn implements the net.Listener Accept method logic and
// returns a *memconn.Conn object.
func (l *Listener) AcceptMemConn() (*Conn, error) {
	select {
	case remoteConn, ok := <-l.rcvr:
		if ok {
			return remoteConn, nil
		}
		return nil, &net.OpError{
			Addr:   l.addr,
			Source: l.addr,
			Net:    l.addr.Network(),
			Err:    errors.New("listener closed"),
		}
	case <-l.done:
		return nil, &net.OpError{
			Addr:   l.addr,
			Source: l.addr,
			Net:    l.addr.Network(),
			Err:    errors.New("listener closed"),
		}
	}
}

// Close implements the net.Listener Close method.
func (l *Listener) Close() error {
	l.once.Do(func() {
		close(l.done)
		<-l.rmvd
	})
	return nil
}

// Addr implements the net.Listener Addr method.
func (l *Listener) Addr() net.Addr {
	return l.addr
}
