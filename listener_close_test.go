package memconn

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestListenerCloseUnblocksPendingDial(t *testing.T) {
	for _, network := range []string{"memu", "memb"} {
		for _, withContext := range []bool{false, true} {
			name := network + "/Dial"
			if withContext {
				name += "Context"
			}
			t.Run(name, func(t *testing.T) {
				var provider Provider
				listener, err := provider.ListenMem(network, &Addr{Name: "pending-dial"})
				if err != nil {
					t.Fatal(err)
				}
				defer listener.Close()
				defer func() {
					// Drain the queue even on a failing implementation so no dial goroutine leaks.
					select {
					case c := <-listener.rcvr:
						c.Close()
					default:
					}
				}()

				// Fill the pending accept queue without calling Accept.
				first, err := provider.Dial(network, "pending-dial")
				if err != nil {
					t.Fatal(err)
				}
				defer first.Close()

				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				result := make(chan error, 1)
				started := make(chan struct{})
				go func() {
					close(started)
					var conn *Conn
					var err error
					if withContext {
						conn, err = listener.dial(ctx, network, Addr{Name: "second", network: network}, listener.addr)
					} else {
						conn, err = listener.dial(nil, network, Addr{Name: "second", network: network}, listener.addr)
					}
					if conn != nil {
						conn.Close()
					}
					result <- err
				}()
				<-started
				// Use the captured listener, as a Dial that has already looked it up
				// does. This also covers a dial entering the queue after Close.
				if err := listener.Close(); err != nil {
					t.Fatal(err)
				}
				select {
				case err := <-result:
					if err == nil {
						t.Fatal("pending dial succeeded after listener closed")
					}
					if op, ok := err.(*net.OpError); !ok || op.Op != "dial" {
						t.Errorf("expected dial net.OpError, got %T: %v", err, err)
					}
				case <-time.After(time.Second):
					t.Fatal("listener Close did not unblock the pending dial")
				}
			})
		}
	}
}

// dialContext records when the dial has reached its queue wait, while it
// holds the provider's listener-cache read lock.
type dialContext struct {
	context.Context
	waiting chan struct{}
}

func (c dialContext) Done() <-chan struct{} {
	close(c.waiting)
	return c.Context.Done()
}

func TestProviderListenerCloseUnblocksPendingDial(t *testing.T) {
	var provider Provider
	listener, err := provider.Listen("memu", "close-pending")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	first, err := provider.Dial("memu", "close-pending")
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	waiting := make(chan struct{})
	dialDone := make(chan error, 1)
	go func() {
		conn, err := provider.DialContext(dialContext{ctx, waiting}, "memu", "close-pending")
		if err == nil {
			conn.Close()
		}
		dialDone <- err
	}()
	<-waiting

	closeDone := make(chan struct{})
	go func() {
		listener.Close()
		close(closeDone)
	}()
	select {
	case err := <-dialDone:
		if err == nil {
			t.Error("pending dial succeeded after listener shutdown")
		}
	case <-time.After(time.Second):
		t.Error("pending public DialContext was not interrupted")
		cancel() // Let a failing implementation release the cache read lock.
	}
	select {
	case <-closeDone:
	case <-time.After(time.Second):
		t.Fatal("listener Close is still waiting for the cache lock")
	}
}
