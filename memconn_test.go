package memconn_test

import (
	"bytes"
	"fmt"
	"io"
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
//         $ go test -race -run TestMemuRace
func TestMemuRace(t *testing.T) {
	for i := 0; i < 1000; i++ {
		p := &memconn.Provider{}
		addr := strconv.Itoa(i)
		go func(p *memconn.Provider, addr string) {
			if c, err := p.Listen("memu", addr); err == nil {
				go c.Accept()
			}
		}(p, addr)
		go func(p *memconn.Provider, addr string) {
			if c, err := p.Dial("memu", addr); err == nil {
				c.Close()
			}
		}(p, addr)
	}
}

func TestMembRace(t *testing.T) {
	for i := 0; i < 1000; i++ {
		p := &memconn.Provider{}
		addr := strconv.Itoa(i)
		go func(p *memconn.Provider, addr string) {
			if c, err := p.Listen("memb", addr); err == nil {
				go c.Accept()
			}
		}(p, addr)
		go func(p *memconn.Provider, addr string) {
			if c, err := p.Dial("memb", addr); err == nil {
				c.Close()
			}
		}(p, addr)
	}
}

// TestMemuNoDeadline validates that the memu connection properly implements
// the net.Conn interface's deadline semantics.
func TestMemuNoDeadline(t *testing.T) {
	testMemConnDeadline(t, "memu", 0, 0)
}

func TestMembNoDeadline(t *testing.T) {
	testMemConnDeadline(t, "memb", 0, 0)
}

// TestMemuDeadline validates that the memu connection properly implements
// the net.Conn interface's deadline semantics.
func TestMemuDeadline(t *testing.T) {
	testMemConnDeadline(
		t, "memu", time.Duration(3)*time.Second, time.Duration(3)*time.Second)
}

func TestMembDeadline(t *testing.T) {
	testMemConnDeadline(
		t, "memb", time.Duration(3)*time.Second, time.Duration(3)*time.Second)
}

// TestMemuWriteDeadline validates that the memu connection properly implements
// the net.Conn interface's write deadline semantics.
func TestMemuWriteDeadline(t *testing.T) {
	testMemConnDeadline(t, "memu", time.Duration(3)*time.Second, 0)
}

func TestMembWriteDeadline(t *testing.T) {
	testMemConnDeadline(t, "memb", time.Duration(3)*time.Second, 0)
}

// TestMemuReadDeadline validates that the memu connection properly implements
// the net.Conn interface's read deadline semantics.
func TestMemuReadDeadline(t *testing.T) {
	testMemConnDeadline(t, "memu", 0, time.Duration(3)*time.Second)
}

func TestMembReadDeadline(t *testing.T) {
	testMemConnDeadline(t, "memb", 0, time.Duration(3)*time.Second)
}

func testMemConnDeadline(
	t *testing.T, network string, write, read time.Duration) {

	var (
		serverReadDeadline  time.Duration
		serverWriteDeadline time.Duration
	)
	if write > 0 {
		serverReadDeadline = time.Duration(1) * time.Minute
	}
	if read > 0 {
		serverWriteDeadline = time.Duration(1) * time.Minute
	}
	lis := serve(
		t, memconn.Listen, "memu", t.Name(),
		serverReadDeadline, serverWriteDeadline, read > 0)

	client, err := memconn.Dial(lis.Addr().Network(), lis.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	// Set the deadline(s)
	if read == write && read > 0 {
		client.SetDeadline(time.Now().Add(read))
	} else {
		now := time.Now()
		if read > 0 {
			client.SetReadDeadline(now.Add(read))
		}
		if write > 0 {
			client.SetWriteDeadline(now.Add(write))
		}
	}

	// Write data to the server. If an error occurs check to see
	// if a write deadline was specified. It would have been small of
	// enough a window to force a timeout error. If the error is not
	// ErrTimeout then fail the write test.
	if _, e := client.Write(fixedData); e == nil && write > 0 {
		t.Fatal("write timeout should have occurred")
	} else if e != nil && write > 0 {
		if opErr, ok := e.(*net.OpError); !ok || !opErr.Timeout() {
			t.Fatalf("write timeout should have occurred: %v", e)
		}
	} else if e != nil {
		t.Fatalf("%[1]T %+[1]v", e)
	}

	// Only perform the read test if a read deadline was set.
	// Read data from the server. If an error occurs check to see
	// if a read deadline was specified. It would have been small of
	// enough a window to force a timeout error. If the error is not
	// ErrTimeout then fail the read test.
	if read > 0 {
		buf := make([]byte, dataLen)
		if _, e := client.Read(buf); e == nil {
			t.Fatal("read timeout should have occurred")
		} else if opErr, ok := e.(*net.OpError); !ok || !opErr.Timeout() {
			t.Fatalf("read timeout should have occurred: %v", e)
		}
	}
}

const parallelTests = 100

func TestMemb(t *testing.T) {
	lis := serve(t, memconn.Listen, "memb", t.Name(), 0, 0, true)
	testNetConnParallel(t, lis, memconn.Dial)
}

func TestMemu(t *testing.T) {
	lis := serve(t, memconn.Listen, "memu", t.Name(), 0, 0, true)
	testNetConnParallel(t, lis, memconn.Dial)
}

func TestTCP(t *testing.T) {
	lis := serve(t, net.Listen, "tcp", "127.0.0.1:", 0, 0, true)
	testNetConnParallel(t, lis, net.Dial)
}

func TestUNIX(t *testing.T) {
	sockFile := getTempSockFile(t)
	defer os.RemoveAll(sockFile)
	lis := serve(t, net.Listen, "unix", sockFile, 0, 0, true)
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
				if err != nil && err != io.ErrClosedPipe && err != io.EOF {
					logger.Fatal(err)
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
