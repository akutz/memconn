package memconn

import (
	"context"
	"net"
)

const (
	// networkMemb is a buffered network connection
	// TODO
	networkMemb = "memb"

	// networkMemu is an unbuffered network connection
	networkMemu = "memu"

	// addrLocalhost is a reserved address name. It is used when a
	// Listen variant omits the local address or a Dial variant omits
	// the remote address.
	addrLocalhost = "localhost"
)

// provider is the package's default provider instance. All of the
// package-level functions interact with this object.
var provider Provider

// Listen begins listening at addr for the specified network.
//
// Known networks are "memu" (memconn unbuffered).
//
// When the specified address is already in use on the specified
// network an error is returned.
//
// When the provided network is unknown the operation defers to
// net.Dial.
func Listen(network, address string) (net.Listener, error) {
	return provider.Listen(network, address)
}

// ListenMem begins listening at laddr.
//
// Known networks are "memu" (memconn unbuffered).
//
// If laddr is nil then ListenMem listens on "localhost" on the
// specified network.
func ListenMem(network string, laddr *Addr) (net.Listener, error) {
	return provider.ListenMem(network, laddr)
}

// Dial dials a named connection.
//
// Known networks are "memu" (memconn unbuffered).
//
// When the provided network is unknown the operation defers to
// net.Dial.
func Dial(network, address string) (net.Conn, error) {
	return provider.Dial(network, address)
}

// DialWithContext dials a named connection using a
// Go context to provide timeout behavior.
//
// Please see Dial for more information.
func DialWithContext(
	ctx context.Context,
	network, addr string) (net.Conn, error) {

	return provider.DialWithContext(ctx, network, addr)
}

// DialMem dials a named connection.
//
// Known networks are "memu" (memconn unbuffered).
//
// If laddr is nil then a new address is generated using
// time.Now().UnixNano(). Please note that client addresses are
// not required to be unique.
//
// If raddr is nil then the "localhost" endpoint is used on the
// specified network.
func DialMem(network string, laddr, raddr *Addr) (net.Conn, error) {
	return provider.DialMem(network, laddr, raddr)
}

// DialMemWithContext dials a named connection using a
// Go context to provide timeout behavior.
//
// Please see DialMem for more information.
func DialMemWithContext(
	ctx context.Context,
	network string,
	laddr, raddr *Addr) (net.Conn, error) {

	return provider.DialMemWithContext(ctx, network, laddr, raddr)
}
