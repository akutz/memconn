package memconn

import (
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/google/uuid"
)

// Provider is used to track named MemConn objects.
type Provider struct {
	memu listenerCache
}

type listenerCache struct {
	sync.RWMutex
	cache map[string]*listener
}

// Listen begins listening at addr for the specified network.
// Known networks are "memu" (memconn unbuffered).
// If addr is already in use then an error is returned.
func (p *Provider) Listen(network, addr string) (net.Listener, error) {
	switch network {
	case networkMemu:
		return p.ListenMem(network, &Addr{Name: addr})
	default:
		return net.Listen(network, addr)
	}
}

// ListenMem begins listening at laddr.
// Known networks are "memu" (memconn unbuffered).
// If laddr is nil then ListenMem listens on "localhost" on the
// specified network.
func (p *Provider) ListenMem(
	network string, laddr *Addr) (net.Listener, error) {

	var (
		onClose   func()
		listeners map[string]*listener
	)

	switch network {
	case networkMemu:
		// Verify that network is compatible with laddr.
		if laddr.Buffered {
			return nil, errors.New("incompatible network & laddr")
		}

		// If laddr is not specified then set it to the reserved name
		// "localhost".
		if laddr == nil {
			laddr = &Addr{Name: addrLocalhost}
		}

		p.memu.Lock()
		defer p.memu.Unlock()

		if p.memu.cache == nil {
			p.memu.cache = map[string]*listener{}
		}

		listeners = p.memu.cache

		// Set up the close handler that removes this
		// listener from the cache when the listener is
		// closed.
		onClose = func() {
			p.memu.Lock()
			defer p.memu.Unlock()
			if c, ok := listeners[laddr.Name]; ok {
				delete(listeners, laddr.Name)
				close(c.rcvr)
			}
		}
	default:
		return nil, errUnknownNetwork(network)
	}

	if _, ok := listeners[laddr.Name]; ok {
		return nil, fmt.Errorf(
			"addr in use: network=%s, addr=%s", network, laddr.Name)
	}

	c := &listener{
		addr:        *laddr,
		rcvr:        make(chan net.Conn, 1),
		onClose:     onClose,
		activeCnxns: map[string]func() error{},
	}

	listeners[laddr.Name] = c
	return c, nil
}

// Dial dials a named connection.
// Known networks are "memu" (memconn unbuffered).
func (p *Provider) Dial(network, addr string) (net.Conn, error) {
	switch network {
	case networkMemu:
		return p.DialMem(network, nil, &Addr{Name: addr})
	default:
		return net.Dial(network, addr)
	}
}

// DialMem dials a named connection.
// Known networks are "memu" (memconn unbuffered).
// If laddr is nil then a new, unique local address is generated
// using a UUID.
// If raddr is nil then the "localhost" endpoint is used on the
// specified network.
func (p *Provider) DialMem(
	network string, laddr, raddr *Addr) (net.Conn, error) {

	var listeners map[string]*listener

	switch network {
	case networkMemu:
		// If laddr is not specified then create one using a UUID as
		// the client address name.
		if laddr == nil {
			laddr = &Addr{Name: uuid.New().String()}
		}
		// If raddr is not specified then set it to the reserved name
		// "localhost".
		if raddr == nil {
			raddr = &Addr{Name: addrLocalhost}
		}
		if laddr.Buffered {
			return nil, errors.New("incompatible network & laddr")
		}
		if raddr.Buffered {
			return nil, errors.New("incompatible network & raddr")
		}

		p.memu.RLock()
		defer p.memu.RUnlock()
		listeners = p.memu.cache
	default:
		return nil, errUnknownNetwork(network)
	}

	if c, ok := listeners[raddr.Name]; ok {
		return c.dial(network, *laddr, *raddr)
	}

	return nil, fmt.Errorf(
		"unknown raddr: network=%s, addr=%s", network, raddr.Name)
}
