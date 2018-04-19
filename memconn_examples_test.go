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

// Example_http illustrates how to create an HTTP server and client
// using memconn.
func Example_http() {

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

// Example_http_mapped_network illustrates how to create an HTTP server
// and client with the "tcp" network mapped to "memu" to make creating
// an HTTP client easier.
func Example_http_mapped_network() {

	// MemConn's MapNetwork function enables mapping known network
	// types to different types. This is useful for when using MemConn
	// with Golang's "http" package since the "http.Client" builds
	// the network address from the provided resource URL. Please
	// see the next comment for more information.
	memconn.MapNetwork("tcp", "memu")

	// Create a new, named listener using the in-memory, unbuffered
	// network "memu".
	//
	// Please note that the listener's address is "host:80". While
	// MemConn names do not need to be unique and are free-form, this
	// name was selected due to the way the "http.Client" builds the
	// network address from the provided resource URL.
	//
	// For example, if the request is "GET http://host/" then the HTTP
	// client dials "network=tcp, address=host:80". In combination
	// with the above "MapNetwork" function this means creating
	// a MemConn listener with the address "host:80" requires no special
	// address translation is required for the when using
	// "memconn.DialContext" with the HTTP client transport.
	lis, err := memconn.Listen("memu", "host:80")
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
		Transport: &http.Transport{DialContext: memconn.DialContext},
	}

	// Get the root resource and copy its response to os.Stdout. The
	// absence of a port means the address sent to the HTTP client's
	// dialer will be "host:80".
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
