package memconn_test

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"os"
	"strings"
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

const (
	errConnRefused   = "connect: connection refused"
	errConnTmpUnavai = "connect: resource temporarily unavailable"
)

var oneMillisecond = time.Duration(1) * time.Millisecond

func getTempSockFile(logger hasLoggerFuncs) string {
	// Get a temp file name to use for the socket file.
	f, err := ioutil.TempFile("", "")
	if err != nil {
		logger.Fatalf("error creating temp sock file: %v", err)
	}
	fileName := f.Name()
	f.Close()
	os.RemoveAll(fileName)
	return fmt.Sprintf("%s.sock", fileName)
}

// dialUNIX is a custom dialer that keeps trying to connect in case
// of ECONNREFUSED or EAGAIN
func dialUNIX(network, addr string) (net.Conn, error) {
	for {
		client, err := net.Dial(network, addr)

		// If there is no error then return the client
		if err == nil {
			return client, nil
		}

		msg := err.Error()

		// If the error is ECONNREFUSED then keep trying to connect.
		if strings.Contains(msg, errConnRefused) {
			time.Sleep(oneMillisecond)
			continue
		}

		// If the error is EAGAIN then keep trying to connect.
		if strings.Contains(msg, errConnTmpUnavai) {
			time.Sleep(oneMillisecond)
			continue
		}

		return nil, err
	}
}

func testNetConnParallel(
	t *testing.T,
	lis net.Listener,
	dial func(string, string) (net.Conn, error)) {

	defer lis.Close()
	network, addr := lis.Addr().Network(), lis.Addr().String()
	t.Run("Client", func(t *testing.T) {
		for i := 0; i < args.clients; i++ {
			t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
				t.Parallel()
				writeAndReadTestData(t, network, addr, dial)
			})
		}
	})
}

const dataLen = 8

type hasLoggerFuncs interface {
	Logf(string, ...interface{})
	Fatal(...interface{})
	Fatalf(string, ...interface{})
}

func serve(
	logger hasLoggerFuncs,
	listen func(string, string) (net.Listener, error),
	network, addr string,
	readDeadline, writeDeadline time.Duration,
	writeBack bool) net.Listener {

	lis, err := listen(network, addr)
	if err != nil {
		logger.Fatalf("error serving %s:%s: %v", network, addr, err)
	}

	if testing.Verbose() {
		logger.Logf("serving %s:%s",
			lis.Addr().Network(),
			lis.Addr().String())
	}

	go func() {
		for {
			c, err := lis.Accept()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				if readDeadline > 0 {
					time.Sleep(readDeadline)
				}
				buf := make([]byte, dataLen)
				_, err := c.Read(buf)
				if err != nil {
					fatal := true
					if e, ok := err.(*net.OpError); ok {
						if e.Err == io.EOF || e.Err == io.ErrClosedPipe {
							fatal = false
						}
					}
					if fatal {
						logger.Fatal(err)
					}
				}
				//if n != dataLen {
				//	logger.Fatalf("read != %d bytes: %d", dataLen, n)
				//}
				if writeBack {
					if writeDeadline > 0 {
						time.Sleep(writeDeadline)
					}
					_, err := c.Write(buf)
					if err != nil {
						logger.Fatal(err)
					}
					//if n != dataLen {
					//	logger.Fatalf("write != %d bytes: %d", dataLen, n)
					//}
				}
				if err := c.Close(); err != nil {
					logger.Fatal(err)
				}
			}(c)
		}
	}()

	return lis
}

func writeAndReadTestData(
	logger hasLoggerFuncs,
	network, addr string,
	dial func(string, string) (net.Conn, error)) {

	client, err := dial(network, addr)
	if err != nil {
		logger.Fatal(err)
	}
	defer client.Close()

	// Create the buffer to write to the server and fill it with random data.
	wbuf := make([]byte, dataLen)
	if n, err := rand.Read(wbuf); err != nil {
		logger.Fatal(err)
	} else if n != dataLen {
		logger.Fatalf("rand != %d bytes: %d", dataLen, n)
	}

	// Write the buffer to the server and assert that dataLen bytes were
	// successfully written.
	if n, err := client.Write(wbuf); err != nil {
		logger.Fatal(err)
	} else if n != dataLen {
		logger.Fatalf("wrote != %d bytes: %d", dataLen, n)
	}

	// Read the response and assert that it matches what was sent.
	rbuf := &bytes.Buffer{}
	if n, err := io.CopyN(rbuf, client, dataLen); err != nil {
		logger.Fatal(err)
	} else if n != dataLen {
		logger.Fatalf("read != %d bytes: %d", dataLen, n)
	} else if rbytes := rbuf.Bytes(); !bytes.Equal(rbytes, wbuf) {
		logger.Fatalf("read != write: rbuf=%v, wbuf=%v", rbytes, wbuf)
	}
}
