package memconn_test

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"

	"github.com/akutz/memconn"
)

// ExampleHTTP illustrates an HTTP server and client that communicate
// over an unbuffered, in-memory connection.
func Example_hTTP() {

	// Create a new, named listener using the in-memory, unbuffered
	// network "memu" and address "MyNamedNetwork".
	lis, err := memconn.Listen("memu", "MyNamedNetwork")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// Create a new HTTP mux and register a handler with it that responds
	// to requests with the text "Hello, world.".
	mux := http.NewServeMux()
	mux.Handle("/", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, "Hello, world.")
		}))

	// Start an HTTP server using the HTTP mux.
	go func() {
		if err := http.Serve(lis, mux); err != http.ErrServerClosed {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}()

	// Create a new HTTP client that delegates its dialing to memconn.
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(
				ctx context.Context, _, _ string) (net.Conn, error) {
				return memconn.DialContext(ctx, "memu", "MyNamedNetwork")
			},
		},
	}

	// Get the root resource and copy its response to os.Stdout. Please
	// note that the URL must contain a host name, even if it's ignored.
	rep, err := client.Get("http://host/")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer rep.Body.Close()
	if _, err := io.Copy(os.Stdout, rep.Body); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// Output: Hello, world.
}
