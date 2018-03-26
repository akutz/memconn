package memconn_test

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/akutz/memconn"
)

func BenchmarkMemConn(b *testing.B) {
	addr := fmt.Sprintf("%d", time.Now().UnixNano())
	lis := serve(b, memconn.Listen, "memu", addr, false)
	benchmarkNetConnParallel(b, lis, memconn.Dial)
}

func BenchmarkTCP(b *testing.B) {
	lis := serve(b, net.Listen, "tcp", "127.0.0.1:", false)
	benchmarkNetConnParallel(b, lis, net.Dial)
}

func BenchmarkUnix(b *testing.B) {
	// Get a temp file name to use for the socket file.
	f, err := ioutil.TempFile("", "")
	if err != nil {
		b.Fatalf("error creating temp sock file: %v", err)
	}
	fileName := f.Name()
	f.Close()
	os.RemoveAll(fileName)
	sockFile := fmt.Sprintf("%s.sock", fileName)

	// Ensure the socket file is cleaned up.
	defer os.RemoveAll(sockFile)

	lis := serve(b, net.Listen, "unix", sockFile, false)

	// The UNIX dialer keeps attempting to connect if an ECONNREFUSED
	// error is encountered due to heavy use.
	dial := func(network, addr string) (net.Conn, error) {
		for {
			client, err := net.Dial(network, addr)

			// If there is no error then return the client
			if err == nil {
				return client, nil
			}

			// If the error is ECONNREFUSED then keep trying to connect.
			if strings.Contains(err.Error(), "connect: connection refused") {
				continue
			}

			return nil, err
		}
	}
	benchmarkNetConnParallel(b, lis, dial)
}

var fixedData = []byte{0, 1, 2, 3, 4, 5, 6, 7}

func benchmarkNetConnParallel(
	b *testing.B,
	lis net.Listener,
	dial func(string, string) (net.Conn, error)) {

	defer lis.Close()
	network, addr := lis.Addr().Network(), lis.Addr().String()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			writeBenchmarkData(b, network, addr, dial)
		}
	})
}

func writeBenchmarkData(
	logger hasLoggerFuncs,
	network, addr string,
	dial func(string, string) (net.Conn, error)) {

	client, err := dial(network, addr)
	if err != nil {
		logger.Fatal(err)
	}
	defer client.Close()

	if n, err := client.Write(fixedData); err != nil {
		logger.Fatal(err)
	} else if n != dataLen {
		logger.Fatalf("wrote != %d bytes: %d", dataLen, n)
	}
	if c, ok := client.(*net.TCPConn); ok {
		c.SetLinger(0)
	}
}
