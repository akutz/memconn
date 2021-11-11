package memconn

import (
	"fmt"
	"io"
	"math"
	"net"
	"strconv"
	"sync/atomic"
	"time"
)

// Conn is an in-memory implementation of Golang's "net.Conn" interface.
type Conn struct {
	pipe

	laddr Addr
	raddr Addr

	// buf contains information about the connection's buffer state if
	// the connection is buffered. Otherwise this field is nil.
	buf *bufConn

	isRemote bool
}

type bufConn struct {
	// tx is used for buffering writes.
	tx bufTx
}

type bufTx struct {
	// errs is the error channel returned by the Errs() function and
	// used to report erros that occur as a result of buffered write
	// operations.
	errs chan error

	// lock is a channel used to disallow concurrent, buffered writes.
	lock chan struct{}

	// Please see the SetWriteBuffer function for more information.
	limit int64

	// Please see the SetWriteBufferLimit function for more information.
	size int64

	// pending is the number of pending, buffered bytes
	pending int64

	// sigPrev is a channel that is signalled when buffered data
	// has been written to the underlying data stream.
	sigPrev chan struct{}

	// sigFree is a channel that is signalled when the buffer has
	// free space.
	sigFree chan struct{}
}

func loadInt(addr *int64) int {
	i := atomic.LoadInt64(addr)
	if strconv.IntSize > 32 {
		return int(i)
	}
	if i <= math.MaxInt32 {
		return int(i)
	}
	panic(fmt.Errorf("loadInt: i=%d > max(int32)", i))
}

func storeInt(addr *int64, i int) {
	atomic.StoreInt64(addr, int64(i))
}

func addInt(addr *int64, delta int) int {
	i := atomic.AddInt64(addr, int64(delta))
	if strconv.IntSize > 32 {
		return int(i)
	}
	if i <= math.MaxInt32 {
		return int(i)
	}
	panic(fmt.Errorf("addInt: i=%d > max(int32)", i))
}

func makeNewConns(network string, laddr, raddr Addr) (*Conn, *Conn) {
	// This code is duplicated from the Pipe() function from the file
	// "memconn_pipe.go". The reason for the duplication is to optimize
	// the performance by removing the need to wrap the *pipe values as
	// interface{} objects out of the Pipe() function and assert them
	// back as *pipe* objects in this function.
	cb1 := make(chan []byte)
	cb2 := make(chan []byte)
	cn1 := make(chan int)
	cn2 := make(chan int)
	done1 := make(chan struct{})
	done2 := make(chan struct{})

	// Wrap the pipes with Conn to support:
	//
	//   * The correct address information for the functions LocalAddr()
	//     and RemoteAddr() return the
	//   * Errors returns from the internal pipe are checked and
	//     have their internal OpError addr information replaced with
	//     the correct address information.
	//   * A channel can be setup to cause the event of the Listener
	//     closing closes the remoteConn immediately.
	//   * Buffered writes
	local := &Conn{
		pipe: pipe{
			rdRx: cb1, rdTx: cn1,
			wrTx: cb2, wrRx: cn2,
			localDone: done1, remoteDone: done2,
			readDeadline:  makePipeDeadline(),
			writeDeadline: makePipeDeadline(),
		},
		laddr: laddr,
		raddr: raddr,
	}
	remote := &Conn{
		pipe: pipe{
			rdRx: cb2, rdTx: cn2,
			wrTx: cb1, wrRx: cn1,
			localDone: done2, remoteDone: done1,
			readDeadline:  makePipeDeadline(),
			writeDeadline: makePipeDeadline(),
		},
		laddr:    raddr,
		raddr:    laddr,
		isRemote: true,
	}

	if laddr.Buffered() {
		local.buf = &bufConn{
			tx: bufTx{
				errs:    make(chan error),
				lock:    make(chan struct{}, 1),
				sigFree: make(chan struct{}, 1),
				sigPrev: make(chan struct{}),
			},
		}
		local.buf.tx.sigFree <- struct{}{}
		close(local.buf.tx.sigPrev)
	}

	if raddr.Buffered() {
		remote.buf = &bufConn{
			tx: bufTx{
				errs:    make(chan error),
				lock:    make(chan struct{}, 1),
				sigFree: make(chan struct{}, 1),
				sigPrev: make(chan struct{}),
			},
		}
		remote.buf.tx.sigFree <- struct{}{}
		close(remote.buf.tx.sigPrev)
	}

	return local, remote
}

// LocalBuffered returns a flag indicating whether or not the local side
// of the connection is buffered.
func (c *Conn) LocalBuffered() bool {
	return c.laddr.Buffered()
}

// RemoteBuffered returns a flag indicating whether or not the remote side
// of the connection is buffered.
func (c *Conn) RemoteBuffered() bool {
	return c.raddr.Buffered()
}

// SetWriteBuffer sets the number of bytes used to transmit buffered
// Writes.
//
// Please note that setting the write buffer size has no effect on
// unbuffered connections.
func (c *Conn) SetWriteBuffer(bytes int) {
	if c.buf != nil {
		atomic.StoreInt64(&c.buf.tx.size, int64(bytes))
	}
}

// SetWriteBufferLimit sets the number of bytes that may be queued for
// Writes. Once this limit has been reached, the Write function will
// block callers until the difference between the write buffer limit
// and the number of pending bytes is greater or equal to the size of
// the specified write buffer size.
//
// The default value is zero, which means no limit is defined.
//
// Please note that setting the write buffer limit has no effect on
// unbuffered connections.
func (c *Conn) SetWriteBufferLimit(bytes int) {
	if c.buf != nil {
		atomic.StoreInt64(&c.buf.tx.limit, int64(bytes))
	}
}

// LocalAddr implements the net.Conn LocalAddr method.
func (c *Conn) LocalAddr() net.Addr {
	return c.laddr
}

// RemoteAddr implements the net.Conn RemoteAddr method.
func (c *Conn) RemoteAddr() net.Addr {
	return c.raddr
}

// WriteErrs returns a channel that receives errors that occur during
// buffered Writes.
//
// This function will always return nil for unbuffered connections.
func (c *Conn) WriteErrs() <-chan error {
	return c.buf.tx.errs
}

// SetReadDeadline implements the net.Conn SetReadDeadline method.
func (c *Conn) SetReadDeadline(t time.Time) error {
	if err := c.pipe.SetReadDeadline(t); err != nil {
		if e, ok := err.(*net.OpError); ok {
			e.Addr = c.laddr
			e.Source = c.laddr
			return e
		}
		return &net.OpError{
			Op:     "setReadDeadline",
			Addr:   c.laddr,
			Source: c.laddr,
			Net:    c.laddr.network,
			Err:    err,
		}
	}
	return nil
}

// SetWriteDeadline implements the net.Conn SetWriteDeadline method.
func (c *Conn) SetWriteDeadline(t time.Time) error {
	if err := c.pipe.SetWriteDeadline(t); err != nil {
		if e, ok := err.(*net.OpError); ok {
			e.Addr = c.laddr
			e.Source = c.laddr
			return e
		}
		return &net.OpError{
			Op:     "setWriteDeadline",
			Addr:   c.laddr,
			Source: c.laddr,
			Net:    c.laddr.network,
			Err:    err,
		}
	}
	return nil
}

// Read implements the net.Conn Read method.
func (c *Conn) Read(b []byte) (int, error) {
	n, err := c.pipe.Read(b)
	if err != nil {
		if err == io.EOF {
			return n, err
		}
		if e, ok := err.(*net.OpError); ok {
			e.Addr = c.raddr
			e.Source = c.laddr
			return n, e
		}
		return n, &net.OpError{
			Op:     "read",
			Addr:   c.raddr,
			Source: c.laddr,
			Net:    c.raddr.network,
			Err:    err,
		}
	}
	return n, nil
}

// Write implements the net.Conn Write method.
func (c *Conn) Write(p []byte) (int, error) {
	if c.buf != nil {
		return c.writeAsync(p)
	}
	return c.writeSync(p)
}

func (c *Conn) writeSync(p []byte) (int, error) {
	n, err := c.pipe.Write(p)
	if err != nil {
		if e, ok := err.(*net.OpError); ok {
			e.Addr = c.raddr
			e.Source = c.laddr
			return n, e
		}
		return n, &net.OpError{
			Op:     "write",
			Addr:   c.raddr,
			Source: c.laddr,
			Net:    c.raddr.network,
			Err:    err,
		}
	}
	return n, nil
}

func (c *Conn) writeAsync(p []byte) (int, error) {
	// Block until there are no other goroutines calling this function
	// or until the local side of the connection is closed.
	select {
	case c.buf.tx.lock <- struct{}{}:
		defer func() { <-c.buf.tx.lock }()
	case <-c.localDone:
		return 0, &net.OpError{
			Op:     "write",
			Addr:   c.raddr,
			Source: c.laddr,
			Net:    c.raddr.network,
			Err:    io.ErrClosedPipe,
		}
	}

	// Get the buffer limit and transmit buffer size.
	bufferLimit := loadInt(&c.buf.tx.limit)
	txBufSize := loadInt(&c.buf.tx.size)

	// The size of a transmit buffer should not exceed the number of
	// bytes allowed to be buffered.
	if bufferLimit > 0 && txBufSize > bufferLimit {
		return 0, &net.OpError{
			Op:     "write",
			Addr:   c.raddr,
			Source: c.laddr,
			Net:    c.raddr.network,
			Err:    fmt.Errorf("txbuf > txlimit"),
		}
	}

	var numBytesBuffered int

	// Golang's definition for an io.Writer states that:
	//
	//   > Write must return a non-nil error if it returns n < len(p).
	//
	// Therefore an attempt must be made to buffer all of the provided
	// payload in a single call. This function will block until there
	// is available buffer space.
	for len(p) > 0 {
		numBytesToBuffer := len(p)

		// If there is a defined limit on how many bytes may be
		// buffered then wait until there is free space available.
		//
		// Every time the number of buffered bytes decreases, the
		// channel c.buf.tx.rdRx is signalled. This signal indicates
		// that there should be some new, free space available to the
		// buffer but does not indicate the amount.
		if bufferLimit > 0 {
			numBytesFree := bufferLimit - loadInt(&c.buf.tx.pending)
			for numBytesFree == 0 {
				select {
				case <-c.buf.tx.sigFree:
					numBytesFree = bufferLimit - loadInt(&c.buf.tx.pending)
				case <-c.localDone:
					return numBytesBuffered, &net.OpError{
						Op:     "write",
						Addr:   c.raddr,
						Source: c.laddr,
						Net:    c.raddr.network,
						Err:    io.ErrClosedPipe,
					}
				}
			}

			// If the number of free bytes is less than len(p), set the
			// numBytesToBuffer equal to the free space.
			if numBytesFree < len(p) {
				numBytesToBuffer = numBytesFree
			}
		}

		// Prepare to buffer n bytes of the payload.
		buffer := make([]byte, numBytesToBuffer)
		copy(buffer, p[:numBytesToBuffer])
		numBytesBuffered = numBytesBuffered + numBytesToBuffer

		// Reslice p to account for the n bytes that will be buffered.
		p = p[numBytesToBuffer:]

		// Update the number of pending bytes to reflect the
		// number of bytes that have just been buffered.
		addInt(&c.buf.tx.pending, numBytesToBuffer)

		// sigDone will be closed when buffer is drained to the underlying
		// data stream.
		sigDone := make(chan struct{})

		// sigPrev is the sigDone from the previous call to this function
		// and is used to block the goroutine below until the goroutine
		// started by the previous call to this function has completed,
		sigPrev := c.buf.tx.sigPrev

		// Update the global sigPrev with the current sigDone so the
		// next call to this function starts a goroutine that waits on
		// the one below to complete.
		c.buf.tx.sigPrev = sigDone

		go func() {
			// When this goroutine completes mark sigDone as closed to
			// ensure the next goroutine is unblocked.
			defer close(sigDone)

			// Wait for the previous write to the underlying data stream
			// to complete. Exit immediately if the local connection has
			// been closed.
			select {
			case <-sigPrev:
			case <-c.localDone:
				go func() {
					c.buf.tx.errs <- &net.OpError{
						Op:     "write",
						Addr:   c.raddr,
						Source: c.laddr,
						Net:    c.raddr.network,
						Err:    io.ErrClosedPipe,
					}
				}()
				return
			}

			// Write the buffered data to the underlying pipe in packets
			// the size of the transmit buffer.
			for len(buffer) > 0 {

				// Stop immediately if the connection has been closed.
				if isClosedChan(c.localDone) {
					go func() {
						c.buf.tx.errs <- &net.OpError{
							Op:     "write",
							Addr:   c.raddr,
							Source: c.laddr,
							Net:    c.raddr.network,
							Err:    io.ErrClosedPipe,
						}
					}()
					return
				}

				// The number of bytes to write cannot exceed the size of
				// the transmit buffer.
				numBytesToWrite := len(buffer)
				if txBufSize > 0 && numBytesToWrite > txBufSize {
					numBytesToWrite = txBufSize
				}

				// Create a transmit buffer to write the specified
				// number of bytes to the underlying data stream.
				txBuf := buffer[:numBytesToWrite]

				// Trim the buffer to reflect the number of bytes
				// placed into the transmit buffer.
				buffer = buffer[numBytesToWrite:]

				// Write the entire transmit buffer to the underlying
				// data stream.
				for len(txBuf) > 0 {

					// Stop immediately if the connection has been closed.
					if isClosedChan(c.localDone) {
						go func() {
							c.buf.tx.errs <- &net.OpError{
								Op:     "write",
								Addr:   c.raddr,
								Source: c.laddr,
								Net:    c.raddr.network,
								Err:    io.ErrClosedPipe,
							}
						}()
						return
					}

					// Write the transmit buffer to the underlying data stream.
					n, err := c.writeSync(txBuf)

					if err != nil {
						go func() { c.buf.tx.errs <- err }()
					}

					// Trim the transmit buffer to reflect the number of
					// bytes written to the underlying data stream.
					txBuf = txBuf[n:]

					// Decrement the number of pending bytes.
					storeInt(&c.buf.tx.pending, -n)

					// Send a signal indicating that the buffer has some
					// amount of free space. If the signal cannot be sent
					// right away it is because another signal is already
					// on the channel. When this happens, return control
					// to the goroutine.
					select {
					case c.buf.tx.sigFree <- struct{}{}:
					case <-c.localDone:
						go func() {
							c.buf.tx.errs <- &net.OpError{
								Op:     "write",
								Addr:   c.raddr,
								Source: c.laddr,
								Net:    c.raddr.network,
								Err:    io.ErrClosedPipe,
							}
						}()
						return
					default:
					}
				}
			}
		}()
	}
	return numBytesBuffered, nil
}
