package memconn_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/akutz/memconn"
)

// TestMemuRace is for verifying there is no race condition when
// calling Dial and Listen concurrently for the same address. The
// Provider.Dial function did not used to initialize the Provider.cnxns
// map since the function only queries it, and querying a nil map is
// allowed. However, Go detected a race condition when the field was
// both assigned and queried at the same time.
//
// Many thanks to @vburenin for spotting this!
//
// To reactive the race condition just remove the p.Once call at the
// top of provider.Dial and then execute:
//
//         $ go test -race -run TestMemuRace
func TestMemuRace(t *testing.T) {
	for i := 0; i < 1000; i++ {
		p := &memconn.Provider{}
		addr := strconv.Itoa(i)
		go func(p *memconn.Provider, addr string) {
			if c, _ := p.Listen("memu", addr); c != nil {
				go c.Accept()
			}
		}(p, addr)
		go func(p *memconn.Provider, addr string) {
			if c, _ := p.Dial("memu", addr); c != nil {
				c.Close()
			}
		}(p, addr)
	}
}

const parallelTests = 100

func TestMemu(t *testing.T) {
	lis := serve(t, memconn.Listen, "memu", t.Name(), true)
	testNetConnParallel(t, lis, memconn.Dial)
}

func TestTCP(t *testing.T) {
	lis := serve(t, net.Listen, "tcp", "127.0.0.1:", true)
	testNetConnParallel(t, lis, net.Dial)
}

func TestUNIX(t *testing.T) {
	sockFile := getTempSockFile(t)
	defer os.RemoveAll(sockFile)
	lis := serve(t, net.Listen, "unix", sockFile, true)
	testNetConnParallel(t, lis, dialUNIX)
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
