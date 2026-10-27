package memconn_test

import (
	"testing"

	"github.com/akutz/memconn"
)

func TestErrsBufferedAndUnbuffered(t *testing.T) {
	for _, listenNetwork := range []string{"memu", "memb"} {
		for _, dialNetwork := range []string{"memu", "memb"} {
			t.Run(listenNetwork+"/"+dialNetwork, func(t *testing.T) {
				var provider memconn.Provider
				listener, err := provider.ListenMem(listenNetwork, nil)
				if err != nil {
					t.Fatal(err)
				}
				defer listener.Close()
				client, err := provider.DialMem(dialNetwork, nil, nil)
				if err != nil {
					t.Fatal(err)
				}
				defer client.Close()
				server, err := listener.AcceptMemConn()
				if err != nil {
					t.Fatal(err)
				}
				defer server.Close()
				for _, conn := range []*memconn.Conn{client, server} {
					if gotNil, wantNil := conn.Errs() == nil, !conn.LocalBuffered(); gotNil != wantNil {
						t.Errorf("buffered=%v: nil Errs channel=%v, want %v", conn.LocalBuffered(), gotNil, wantNil)
					}
				}
			})
		}
	}
}
