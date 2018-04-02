package memconn

// Addr represents the address of an in-memory endpoint.
type Addr struct {
	// Buffered indicates whether or not the endpoint is
	// buffered.
	Buffered bool

	// Name is the name of the endpoint.
	Name string
}

// Network returns the address's network type.
func (a Addr) Network() string {
	if !a.Buffered {
		return networkMemu
	}
	return networkMemb
}

// String returns the address's name.
func (a Addr) String() string {
	return a.Name
}
