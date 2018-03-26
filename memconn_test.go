package memconn_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"os"
	"strings"
	"testing"

	"github.com/akutz/memconn"
)

const parallelTests = 100

// TestMemConn validates that multiple MemConn connections do not
// interfere with one another and validats that the response data
// matches the expected result.
func TestMemConn(t *testing.T) {
	lis := serve(t, memconn.Listen, "memu", t.Name(), true)
	testNetConnParallel(t, lis, memconn.Dial)
}

func TestTCP(t *testing.T) {
	lis := serve(t, net.Listen, "tcp", "127.0.0.1:", true)
	testNetConnParallel(t, lis, net.Dial)
}

func TestUnix(t *testing.T) {
	// Get a temp file name to use for the socket file.
	f, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatalf("error creating temp sock file: %v", err)
	}
	fileName := f.Name()
	f.Close()
	os.RemoveAll(fileName)
	sockFile := fmt.Sprintf("%s.sock", fileName)

	// Ensure the socket file is cleaned up.
	defer os.RemoveAll(sockFile)

	lis := serve(t, net.Listen, "unix", sockFile, true)

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
	testNetConnParallel(t, lis, dial)
}

func testNetConnParallel(
	t *testing.T,
	lis net.Listener,
	dial func(string, string) (net.Conn, error)) {

	defer lis.Close()
	network, addr := lis.Addr().Network(), lis.Addr().String()
	t.Run("Parallel", func(t *testing.T) {
		for i := 0; i < parallelTests; i++ {
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
	writeBack bool) net.Listener {

	lis, err := listen(network, addr)
	if err != nil {
		logger.Fatalf("error serving %s:%s: %v", network, addr, err)
	}

	logger.Logf("serving %s:%s",
		lis.Addr().Network(),
		lis.Addr().String())

	go func() {
		for {
			c, err := lis.Accept()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				buf := make([]byte, dataLen)
				n, err := c.Read(buf)
				if err != nil {
					logger.Fatal(err)
				}
				if n != dataLen {
					logger.Fatalf("read != %d bytes: %d", dataLen, n)
				}
				if writeBack {
					n, err := c.Write(buf)
					if err != nil {
						logger.Fatal(err)
					}
					if n != dataLen {
						logger.Fatalf("write != %d bytes: %d", dataLen, n)
					}
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
	rbuf := make([]byte, dataLen)
	if n, err := client.Read(rbuf); err != nil {
		logger.Fatal(err)
	} else if n != dataLen {
		logger.Fatalf("read != %d bytes: %d", dataLen, n)
	} else if !bytes.Equal(rbuf, wbuf) {
		logger.Fatalf("read != write: rbuf=%v, wbuf=%v", rbuf, wbuf)
	}
}
