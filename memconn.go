package memconn

import (
	"net"
	"net/http"
)

// MemConn is a memory-based network connection that can be provided to
// servers as a net.Listener which clients can then access via the same
type MemConn interface {
	net.Listener
	Dial() (net.Conn, error)
}

// Listen creates a new, named MemConn and stores it in the default
// MemConn provider.
// If a MemConn with the specified name already exists then an error is
// returned.
// The network type returned by MemConn.Addr().Network() is "memconn".
// The provided name is returned by MemConn.Addr().String().
func Listen(name string) (net.Listener, error) {
	return provider.Listen(name)
}

// Dial dials a named MemConn in the default MemConn provider and returns the
// net.Conn object if the connection is successful.
func Dial(name string) (net.Conn, error) {
	return provider.Dial(name)
}

// DialHTTP dials a named PipeConn in the default MemConn provider and
// returns an http.Client object if the connection is successful. The client's
// Transport field is set to a *http.Transport with a custom DialContext
// function that uses this package's Dial function to access the named
// pipe.
func DialHTTP(name string) *http.Client {
	return provider.DialHTTP(name)
}

// Drain removes all of the channels from the channel pool in the default
// MemConn provider.
func Drain() {
	provider.Drain()
}
