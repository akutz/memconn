package memconn_test

import (
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/akutz/memconn"
)

func BenchmarkMemb(b *testing.B) {
	addr := fmt.Sprintf("%d", time.Now().UnixNano())
	lis := serve(b, memconn.Listen, "memb", addr, 0, 0, false)
	benchmarkNetConnParallel(b, lis, memconn.Dial)
}

func BenchmarkMembWithDeadline(b *testing.B) {
	laddr := fmt.Sprintf("%d", time.Now().UnixNano())
	lis := serve(b, memconn.Listen, "memb", laddr, 0, 0, false)
	benchmarkNetConnParallel(b, lis,
		func(network, addr string) (net.Conn, error) {
			client, err := memconn.Dial(network, addr)
			if err != nil {
				return nil, err
			}
			deadline := time.Now().Add(time.Duration(1) * time.Minute)
			client.SetDeadline(deadline)
			return client, nil
		})
}

func BenchmarkMemu(b *testing.B) {
	addr := fmt.Sprintf("%d", time.Now().UnixNano())
	lis := serve(b, memconn.Listen, "memu", addr, 0, 0, false)
	benchmarkNetConnParallel(b, lis, memconn.Dial)
}

func BenchmarkMemuWithDeadline(b *testing.B) {
	laddr := fmt.Sprintf("%d", time.Now().UnixNano())
	lis := serve(b, memconn.Listen, "memu", laddr, 0, 0, false)
	benchmarkNetConnParallel(b, lis,
		func(network, addr string) (net.Conn, error) {
			client, err := memconn.Dial(network, addr)
			if err != nil {
				return nil, err
			}
			deadline := time.Now().Add(time.Duration(1) * time.Minute)
			client.SetDeadline(deadline)
			return client, nil
		})
}

func BenchmarkTCP(b *testing.B) {
	lis := serve(b, net.Listen, "tcp", "127.0.0.1:", 0, 0, false)
	benchmarkNetConnParallel(b, lis, net.Dial)
}

func BenchmarkTCPWithDeadline(b *testing.B) {
	lis := serve(b, net.Listen, "tcp", "127.0.0.1:", 0, 0, false)
	benchmarkNetConnParallel(b, lis,
		func(network, addr string) (net.Conn, error) {
			client, err := net.Dial(network, addr)
			if err != nil {
				return nil, err
			}
			deadline := time.Now().Add(time.Duration(1) * time.Minute)
			client.SetDeadline(deadline)
			return client, nil
		})
}

func BenchmarkUNIX(b *testing.B) {
	sockFile := getTempSockFile(b)
	defer os.RemoveAll(sockFile)
	lis := serve(b, net.Listen, "unix", sockFile, 0, 0, false)
	benchmarkNetConnParallel(b, lis, dialUNIX)
}

func BenchmarkUNIXWithDeadline(b *testing.B) {
	sockFile := getTempSockFile(b)
	defer os.RemoveAll(sockFile)
	lis := serve(b, net.Listen, "unix", sockFile, 0, 0, false)
	benchmarkNetConnParallel(b, lis,
		func(network, addr string) (net.Conn, error) {
			client, err := dialUNIX(network, addr)
			if err != nil {
				return nil, err
			}
			deadline := time.Now().Add(time.Duration(1) * time.Minute)
			client.SetDeadline(deadline)
			return client, nil
		})
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
