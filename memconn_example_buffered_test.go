package memconn_test

import (
	"io"
	"os"

	"github.com/akutz/memconn"
)

// ExampleBuffered illustrates a server and client that
// communicate over a buffered, in-memory connection.
func Example_buffered() {
	// Announce a new listener named "localhost" on MemConn's
	// buffered network, "memb".
	lis, _ := memconn.Listen("memb", "localhost")

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

	// Dial the buffered, in-memory network named "localhost".
	conn, _ := memconn.Dial("memb", "localhost")

	// Ensure the connection is closed.
	defer conn.Close()

	// Write the data to the server.
	go conn.Write([]byte("Hello, world."))

	// Read the data from the server.
	io.CopyN(os.Stdout, conn, 13)

	// Output: Hello, world.
}
