package memconn_test

import (
	"io"
	"os"

	"github.com/akutz/memconn"
)

// ExampleUnbuffered illustrates a server and client that
// communicate over an unbuffered, in-memory connection.
func Example_unbuffered() {
	// Announce a new listener named "localhost" on MemConn's
	// unbuffered network, "memu".
	lis, _ := memconn.Listen("memu", "localhost")

	// Ensure the listener is closed.
	defer lis.Close()

	// Start a goroutine that will wait for a client to dial the
	// listener and then echo back any data sent to the remote
	// connection.
	go func() {
		conn, _ := lis.Accept()

		// If no errors occur then make sure the connection is closed.
		defer conn.Close()

		// Echo the data back to the client.
		io.Copy(conn, conn)
	}()

	// Dial the unbuffered, in-memory network named "localhost".
	conn, _ := memconn.Dial("memu", "localhost")

	// Ensure the connection is closed.
	defer conn.Close()

	// Write the data to the server.
	go conn.Write([]byte("Hello, world."))

	// Read the data from the server.
	io.CopyN(os.Stdout, conn, 13)

	// Output: Hello, world.
}
