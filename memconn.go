package memconn

import (
	"errors"
	"net"
)

// ErrServerClosed is returned by the Listen method after a call to Close.
var ErrServerClosed = errors.New("memconn: Server closed")

// Listen creates a new, named MemConn and stores it in the default
// MemConn provider.
// Known networks are "memu" (memconn unbuffered).
// If a MemConn with the specified address already exists then an error is
// returned.
func Listen(network, addr string) (net.Listener, error) {
	return provider.Listen(network, addr)
}

// Dial dials a named MemConn in the default MemConn provider and returns the
// net.Conn object if the connection is successful.
// Known networks are "memu" (memconn unbuffered).
func Dial(network, addr string) (net.Conn, error) {
	return provider.Dial(network, addr)
}

// Drain removes all of the channels from the channel pool in the default
// MemConn provider.
func Drain() {
	provider.Drain()
}

// Prime initializes the default MemConn provider with the specified
// number of channels.
func Prime(count int) {
	for i := 0; i < count; i++ {
		provider.chanPool.Put(make(chan interface{}, 1))
	}
}
