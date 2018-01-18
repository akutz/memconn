package memconn_test

import (
	"context"
	"net"
	"net/http"
	"os"
	"testing"

	"github.com/akutz/memconn"
)

func BenchmarkMemConn(b *testing.B) {
	addr := b.Name()
	lis, err := memconn.Listen(addr)
	if err != nil {
		b.Fatal(err)
	}
	defer lis.Close()
	benchmarkNetConnParallel(b, lis, memconn.DialHTTP(addr))
}

func BenchmarkTCP(b *testing.B) {
	lis, err := net.Listen("tcp", "127.0.0.1:")
	if err != nil {
		b.Fatalf("failed to listen on 127.0.0.1: %v", err)
	}
	benchmarkTCPOrUnix(b, lis)
}

const sockFile = ".memconn.sock"

func BenchmarkUnix(b *testing.B) {
	defer os.RemoveAll(sockFile)
	lis, err := net.Listen("unix", sockFile)
	if err != nil {
		b.Fatalf("failed to listen on %s: %v", sockFile, err)
	}
	benchmarkTCPOrUnix(b, lis)
}

func benchmarkTCPOrUnix(b *testing.B, lis net.Listener) {
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(
				context.Context, string, string) (net.Conn, error) {
				return net.Dial(lis.Addr().Network(), lis.Addr().String())
			},
			// If the error "can't assign requested address" occurs when
			// running the benchmark then try reducing the following value
			// or increasing the number of open file descriptors allowed
			// on this host.
			MaxIdleConnsPerHost: 100,
		},
	}
	benchmarkNetConnParallel(b, lis, client)
}

func benchmarkNetConnParallel(
	b *testing.B,
	lis net.Listener,
	client *http.Client) {

	// Create and launch an HTTP server.
	server := goHTTPServer(lis, b.Fatalf)

	// Make sure the server is shutdown.
	defer server.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if err := postHTTPRequest(client); err != nil {
				b.Fatal(err)
			}
		}
	})
}
