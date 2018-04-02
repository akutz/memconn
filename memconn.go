package memconn

import (
	"fmt"
	"net"
)

const (
	networkMemb = "memb"
	networkMemu = "memu"

	// addrLocalhost is a reserved address name. It is used when a
	// Listen variant omits the local address or a Dial variant omits
	// the remote address.
	addrLocalhost = "localhost"
)

var provider = Provider{}

// Listen begins listening at addr for the specified network.
// Known networks are "memu" (memconn unbuffered).
// If addr is already in use then an error is returned.
func Listen(network, addr string) (net.Listener, error) {
	return provider.Listen(network, addr)
}

// ListenMem begins listening at laddr.
// Known networks are "memu" (memconn unbuffered).
// If laddr is nil then ListenMem listens on "localhost" on the
// "memu" (unbuffered) network.
func ListenMem(network string, laddr *Addr) (net.Listener, error) {
	return provider.ListenMem(network, laddr)
}

// Dial dials a named connection.
// Known networks are "memu" (memconn unbuffered).
func Dial(network, addr string) (net.Conn, error) {
	return provider.Dial(network, addr)
}

// DialMem dials a named connection.
// Known networks are "memu" (memconn unbuffered).
// If laddr is nil then a new, unique local address is generated
// using a UUID.
// If raddr is nil then the named, unbuffered endpoint "localhost"
// is used.
func DialMem(network string, laddr, raddr *Addr) (net.Conn, error) {
	return provider.DialMem(network, laddr, raddr)
}

func errUnknownNetwork(network string) error {
	return fmt.Errorf("unknown network: %s", network)
}
