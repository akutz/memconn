package memconn_test

import (
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"testing"

	"github.com/akutz/memconn"
	"github.com/golang/go/src/math/rand"
)

const parallelTests = 100

// TestMemConn validates that multiple MemConn connections do not
// interfere with one another and validats that the response data
// matches the expected result.
func TestMemConn(t *testing.T) {

	// addr is the name used to get a new memconn net.Conn with
	// memconn.Listen and then later to dial the same connection
	// with memconn.Dial (or memconn.DialHTTP)
	addr := t.Name()

	// Use memconn.Listen to create a new net.Listener used the HTTP
	// server below.
	lis, err := memconn.Listen(addr)
	if err != nil {
		t.Fatal(err)
	}
	defer lis.Close()

	// Create and launch an HTTP server.
	server := goHTTPServer(lis, t.Fatalf)

	// Make sure the server is shutdown.
	defer server.Close()

	t.Run("Parallel", func(t *testing.T) {
		for i := 0; i < parallelTests; i++ {
			t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
				t.Parallel()

				// Dial the HTTP server that is listening on the memconn
				// listener and post an HTTP request. The memconn.DialHTTP
				// function is just a shortcut that returns an *http.Client.
				if err := postHTTPRequest(memconn.DialHTTP(addr)); err != nil {
					t.Fatal(err)
				}
			})
		}
	})
}

func goHTTPServer(
	lis net.Listener,
	fatalf func(string, ...interface{})) *http.Server {

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if _, err := io.Copy(w, r.Body); err != nil {
			fatalf("write http response failed: %v", err)
		}
	})
	server := &http.Server{Handler: mux}
	go server.Serve(lis)
	return server
}

func postHTTPRequest(client *http.Client) error {
	// Generate a random integer to be the unique request
	// data posted to the HTTP server.
	reqData := fmt.Sprintf("%d", rand.Int())

	// Post the request data.
	rep, err := client.Post(
		"http://host/",
		"text/pain",
		strings.NewReader(reqData))
	if err != nil {
		return fmt.Errorf("http request failed: %v", err)
	}
	if rep.StatusCode != 200 {
		return fmt.Errorf("http status not okay: %d", rep.StatusCode)
	}
	defer rep.Body.Close()

	buf, err := ioutil.ReadAll(rep.Body)
	if err != nil {
		return fmt.Errorf("read http response failed: %v", err)
	}

	repData := string(buf)
	if repData != reqData {
		return fmt.Errorf("invalid response: reqData=%v, repData=%v",
			reqData, repData)
	}

	return nil
}
