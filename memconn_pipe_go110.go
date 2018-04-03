// +build go1.10

package memconn

import (
	"net"
)

// Pipe creates a synchronous, in-memory, full duplex
// network connection; both ends implement the Conn interface.
// Reads on one end are matched with writes on the other,
// copying data directly between the two; there is no internal
// buffering.
func Pipe() (net.Conn, net.Conn) {
	return net.Pipe()
}
