package memconn_test

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/akutz/memconn"
)

func TestTLS_Memu(t *testing.T) {
	testTLS(t, "memu", "localhost", true)
}

func TestTLS_Memu_NoTLS(t *testing.T) {
	testTLS(t, "memu", "localhost", false)
}

func TestTLS_Memb(t *testing.T) {
	testTLS(t, "memb", "localhost", true)
}

func TestTLS_Memb_NoTLS(t *testing.T) {
	testTLS(t, "memb", "localhost", false)
}

func TestTLS_TCP(t *testing.T) {
	testTLS(t, "tcp", "localhost:9669", true)
}

func TestTLS_TCP_NoTLS(t *testing.T) {
	testTLS(t, "tcp", "localhost:9669", false)
}

func TestTLS_UNIX(t *testing.T) {
	os.RemoveAll(".tls.sock")
	testTLS(t, "unix", ".tls.sock", true)
}

func TestTLS_UNIX_NoTLS(t *testing.T) {
	os.RemoveAll(".tls.sock")
	testTLS(t, "unix", ".tls.sock", false)
}

var opCounter uint64

func testTLS(t *testing.T, network, address string, useTLS bool) {

	var (
		lis         net.Listener
		rootCAs     *x509.CertPool
		hostKeyPair tls.Certificate
	)

	if useTLS {
		// Create the cert pool for the CA.
		rootCAs = x509.NewCertPool()
		if !rootCAs.AppendCertsFromPEM([]byte(caCrt)) {
			t.Fatal("error appending root ca")
		}

		// Load the host certificate and key.
		var err error
		hostKeyPair, err = tls.X509KeyPair([]byte(hostCrt), []byte(hostKey))
		if err != nil {
			t.Fatalf("error loading host certs: %v", err)
		}
	}

	// Start listening.
	lis, err := memconn.Listen(network, address)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := lis.Close(); err != nil {
			t.Fatalf("error closing listener: %v", err)
		}
	}()

	var (
		wgUp             sync.WaitGroup
		wgDown           sync.WaitGroup
		remoteConnClosed = make(chan struct{})
	)

	defer func() {
		<-remoteConnClosed
	}()

	wgUp.Add(2)
	wgDown.Add(2)

	go func() {
		conn, err := lis.Accept()
		if err != nil {
			t.Fatal(err)
		}
		if debug {
			conn = &connLogger{
				Conn:      conn,
				isRemote:  true,
				opCounter: &opCounter,
			}
		}

		drain := func() {}

		if useTLS {
			if network == "memu" {
				noTLS := conn
				drain = func() {
					io.Copy(ioutil.Discard, noTLS)
				}
			}
			conn = tls.Server(conn, &tls.Config{
				Certificates: []tls.Certificate{hostKeyPair},
				RootCAs:      rootCAs,
			})
		}

		defer func() {
			go drain()
			conn.Close()
			close(remoteConnClosed)
		}()

		buf := &bytes.Buffer{}
		if _, err := io.CopyN(buf, conn, 8); err != nil {
			t.Fatal(err)
		}
		wgUp.Done()
		wgUp.Wait()
		if _, err := io.CopyN(conn, buf, 8); err != nil {
			t.Fatal(err)
		}
		wgDown.Done()
		wgDown.Wait()
	}()

	// Dial the server using TLS.
	client, err := memconn.Dial(network, address)
	if err != nil {
		t.Fatalf("error dialing server: %v", err)
	}
	if debug {
		client = &connLogger{Conn: client, opCounter: &opCounter}
	}

	drain := func() {}

	if useTLS {
		if network == "memu" {
			noTLS := client
			drain = func() {
				io.Copy(ioutil.Discard, noTLS)
			}
		}
		client = tls.Client(client, &tls.Config{
			RootCAs:    rootCAs,
			ServerName: "localhost",
		})
	}

	defer func() {
		go drain()
		client.Close()
	}()

	data := []byte{0, 1, 2, 3, 4, 5, 6, 7}
	wbuf := bytes.NewBuffer(data)
	if _, err := io.CopyN(client, wbuf, 8); err != nil {
		t.Fatal(err)
	}
	wgUp.Done()
	wgUp.Wait()
	rbuf := &bytes.Buffer{}
	if _, err := io.CopyN(rbuf, client, 8); err != nil {
		t.Fatal(err)
	}

	if data2 := rbuf.Bytes(); !bytes.Equal(data, data2) {
		t.Fatalf("wbuf != rbuf: wbuf=%v, rbuf=%v", data, data2)
	}
	wgDown.Done()
	wgDown.Wait()
}

func TestTLS_HTTP_Memu(t *testing.T) {
	testTLS_HTTP(t, "memu", "localhost")
}

func TestTLS_HTTP_Memb(t *testing.T) {
	testTLS_HTTP(t, "memb", "localhost")
}

func testTLS_HTTP(t *testing.T, network, address string) {
	var (
		rootCAs     *x509.CertPool
		hostKeyPair tls.Certificate
	)

	// Create the cert pool for the CA.
	rootCAs = x509.NewCertPool()
	if !rootCAs.AppendCertsFromPEM([]byte(caCrt)) {
		t.Fatal("error appending root ca")
	}

	// Load the host certificate and key.
	hostKeyPair, err := tls.X509KeyPair([]byte(hostCrt), []byte(hostKey))
	if err != nil {
		t.Fatalf("error loading host certs: %v", err)
	}

	// Create a new listener.
	lis, err := memconn.Listen(network, address)
	if err != nil {
		t.Fatal(err)
	}

	// Wrap the listener in a TLS listener.
	lis = tls.NewListener(lis, &tls.Config{
		Certificates: []tls.Certificate{hostKeyPair},
		RootCAs:      rootCAs,
	})

	// Create a new HTTP mux and register a handler with it that responds
	// to requests with the text "Hello, world.".
	mux := http.NewServeMux()
	mux.Handle("/", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, "Hello, world.")
		}))

	// Create a new HTTP server and specify its TLS config using the
	// already loaded key/pair and root CA pool.
	server := &http.Server{Handler: mux}

	go func() {
		if err := server.Serve(lis); err != http.ErrServerClosed {
			t.Fatalf("http.Serve failed: %v", err)
		}
	}()

	// Create a new HTTP client that delegates its dialing to memconn.
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(
				ctx context.Context, _, _ string) (net.Conn, error) {

				// Dial the server, but don't return it. It must be wrapped
				// in TLS.
				conn, err := memconn.DialContext(ctx, network, address)
				if err != nil {
					return nil, err
				}

				// Return a TLS-wrapped version of the dialed connection.
				return tls.Client(conn, &tls.Config{
					RootCAs:    rootCAs,
					ServerName: "localhost",
				}), nil
			},
		},
	}

	// Get the root resource and copy its response to os.Stdout. Please
	// note that the URL must contain a host name, even if it's ignored.
	rep, err := client.Get("http://localhost/")
	if err != nil {
		t.Fatal(err)
	}
	defer rep.Body.Close()

	buf, err := ioutil.ReadAll(rep.Body)
	if err != nil {
		t.Fatal(err)
	}

	sz := string(buf)
	if sz != "Hello, world." {
		t.Fatalf("invalid response: exp='%s' act='%s'", "Hello, world.", sz)
	}

	if err := server.Shutdown(context.Background()); err != nil {
		t.Fatalf("http.Shutdown failed: %v", err)
	}
}

const (
	caCrt = `-----BEGIN CERTIFICATE-----
MIIDgjCCAmoCCQChm2IAOP7tiTANBgkqhkiG9w0BAQsFADCBgjELMAkGA1UEBhMC
VVMxDjAMBgNVBAgMBVRleGFzMQ8wDQYDVQQHDAZBdXN0aW4xDzANBgNVBAoMBkdp
dEh1YjEOMAwGA1UECwwFYWt1dHoxEDAOBgNVBAMMB01lbUNvbm4xHzAdBgkqhkiG
9w0BCQEWEGFrdXR6QGdpdGh1Yi5jb20wHhcNMTgwNDI0MTQ0NTA4WhcNMTgwNTI0
MTQ0NTA4WjCBgjELMAkGA1UEBhMCVVMxDjAMBgNVBAgMBVRleGFzMQ8wDQYDVQQH
DAZBdXN0aW4xDzANBgNVBAoMBkdpdEh1YjEOMAwGA1UECwwFYWt1dHoxEDAOBgNV
BAMMB01lbUNvbm4xHzAdBgkqhkiG9w0BCQEWEGFrdXR6QGdpdGh1Yi5jb20wggEi
MA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQDDltHl9NfNDQoSqoOVe1zXiNjz
monSFCK3mOZXepFxJFrGZ+h8BuPZfW78HRFgCO7JH2iFHFEJg5ZJPlA4bMzH0Qqs
12dEmKEW1L9FPyTadtYv74RGdpPuIyC+PXGSUVEtrcCNaKdlGqoDl3zATWwprx2t
lXJUsuaSvlpsAwJmpkFMgVlkYCd3u3pbad8Fx7mFwP/3YsD0ksj/ffGQCMCIGBEG
bOxYRZAiurwlB4JkoJHMz5i5sUYDBqC0mRGHX0+W5LGQf79bfJFkmHwDhJJbeG2h
SzODVtO/60GV6QWJ/FR9ofv+PxCh0LUxJ42SfKMI8apciCYwgGWDRaNjYS/xAgMB
AAEwDQYJKoZIhvcNAQELBQADggEBAGcv4AoCNhboqX/Eiaa0hBZjw51jDP85dzHC
ZqY3eJQImRQGkEHIPHW7vEARrUDoL6HayLAfUBx2fZ4FAbVroH1SbDKPqeXFfxb+
Wp6DZOlEgmsfYyLBdBJNkF0wl3an09h9m0Lj0JAgKqeoyPWV2SfFw2zDOllgXFix
rPB8FHEVFK8nMKOY2XP0JnnWzbo4zcUW72ytDPquvFsN42dPmCNsdwPUXP2gFs1h
AMLe58rnDDT1dQpAO/n8kiyGGzQhktMIFtmuW/OzCMSC2xLAgrlN6tIVNAAzVnFK
590tG24h1m+iHeOKEB4NZDhWagzkM2OT9bs6gHx9FAK7+NzfgnM=
-----END CERTIFICATE-----`

	caKey = `-----BEGIN RSA PRIVATE KEY-----
MIIEpQIBAAKCAQEAw5bR5fTXzQ0KEqqDlXtc14jY85qJ0hQit5jmV3qRcSRaxmfo
fAbj2X1u/B0RYAjuyR9ohRxRCYOWST5QOGzMx9EKrNdnRJihFtS/RT8k2nbWL++E
RnaT7iMgvj1xklFRLa3AjWinZRqqA5d8wE1sKa8drZVyVLLmkr5abAMCZqZBTIFZ
ZGAnd7t6W2nfBce5hcD/92LA9JLI/33xkAjAiBgRBmzsWEWQIrq8JQeCZKCRzM+Y
ubFGAwagtJkRh19PluSxkH+/W3yRZJh8A4SSW3htoUszg1bTv+tBlekFifxUfaH7
/j8QodC1MSeNknyjCPGqXIgmMIBlg0WjY2Ev8QIDAQABAoIBAQCXfphx72W/k5v7
vUtSW00cPQkeFtMtfx8s6idwFqXU5v3Qs+clOgj+CuQOL02n/wNFkShaAgbawauE
a9mi+tLa6pXELsv0G+yaTIsiTbhz5pwcYP8pvOr0Bw1zjRAM7yNbqDt+zFLsQuzw
/0NHiDCBUPxB2YHHDRL/EqXjB6myazVSzWowggsDpHVvmpnz30QyyH7XDzf1sTHR
9lKJ4he1+KbBDuIaLHbaRK9uRHLnIv9vIBsNzv5CjsUNjmqJOTRZS9gJ5THrs0dk
2xKHOQ8ZlK+TeiueOwntP/imT1MJQOqqCbJXhWO7beLkfxQTYblZt7udGWI/41EO
RjEDZsaBAoGBAOcP4P7yItnvpA1cXfFqs5yvzK5UN2Z3mOA5PYP/1dOHUQ4316M6
89Q61yRlrhGIrFFZr7BmcIN7GbGpbmq17ZO/fG/fg65Wen8np5YoolDaIIbepqKs
iAf7Bv71kcAZVzxJPVGAgmt/ML6Wb2DAxph/eeZorgc9CIn2Z2Ug0sn5AoGBANiy
2BCgB6/DdkgmmEAzMZrF2mi8ukqJhImifr0P9W6H4aAA0I6NoyUz87jlciV3X8ny
AJlKHVOQXNATu4fTJnEZe0XJpkNtIgJNEQ5Zlv2KZuTVQNjku9ATEinXyQw/dE6/
GRZohEbN3ymO8FUm5Ap/3DfW+uT/uafhWsrQfdO5AoGBAKrrzRTqSquKIIGdpQRz
WL/8L115gK20pIqg7Qda1XKu81+gIUxmzH1etU0ARj5EKqvWuyay8GHiSsRoP/yB
7WdQy5z56y+oWt76l3Z1QnSqlksOIpfNJqc4oxkw0IsYc7ZtuwUyGcepA4bIQ0V/
9KhUC/lL0AgcttdPRXbCTAsJAoGACshTYfhkiYVjVFG/T6p8dGQV6xJA/sZ69tJE
Fio+HyLZwjloJz+693XvUarxFBYtiQHmr7n1XZwYUi45LZf/GK+Y568R+9bpU038
ZEdm8PS7C/XkhSZUhhT82WIoWdiqc+SkXe4TbuZ9jTbUlJgbzr3v+kNTNqPW3Bil
iOP47tkCgYEAtxj1keHjswWr/jT3XqPjXMsR0Tvk1nrjKB49+UK1bb4YyYy5yEKZ
K1d9HtagCk2H1NHIf66hP+8m6CZ8OV+rof3pLEYZWihHt/Ra2wTXiXxuT0d1weZ6
x3K9rt7km4A8HFG/nFxmQIedjGO3JiCep36p0I8nvSQZJlJ7tkTqxhY=
-----END RSA PRIVATE KEY-----`

	hostCrt = `-----BEGIN CERTIFICATE-----
MIIDvjCCAqagAwIBAgIBATANBgkqhkiG9w0BAQUFADCBgjELMAkGA1UEBhMCVVMx
DjAMBgNVBAgMBVRleGFzMQ8wDQYDVQQHDAZBdXN0aW4xDzANBgNVBAoMBkdpdEh1
YjEOMAwGA1UECwwFYWt1dHoxEDAOBgNVBAMMB01lbUNvbm4xHzAdBgkqhkiG9w0B
CQEWEGFrdXR6QGdpdGh1Yi5jb20wIBcNMTgwNDI0MTUwMzQ2WhgPMjExODAzMzEx
NTAzNDZaMFExCzAJBgNVBAYTAlVTMQ4wDAYDVQQIDAVUZXhhczEPMA0GA1UECgwG
R2l0SHViMSEwHwYDVQQDDBhtZW1jb25uLmFrdXR6LmdpdGh1Yi5jb20wggEiMA0G
CSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQDJVRieeIQCb4RpXiDL8cPs6QYxRzZX
fHxTFK8GOhr08fBbW+HJbHCVlbQa/iLwJyjUc98oBiWmdNyZBLJDmEPxcjFGdD8C
N6EArKto9R1NC1vEMdm3nMYzveQ0/4O9RqLxk12papCMZEomv0Wv4ZsqhWTSl+gQ
MLt5uI51AAbpNmzO2eUHGHX7TDb73dcAiH77p5mrqDnFR+vd1a6wJNeLIRoUBzeR
peKO+gTTgWgLfqYEk/eOL3ymA3IxPadWGleeQZl0el+2fau5Z+L0Rlo4Tbdj2Gb2
VG3r/GKBIB7ALW10JJloyi5sTn4cpPUJp8FX6RZAZ9gGcwJtdJ38hqXLAgMBAAGj
bTBrMAkGA1UdEwQCMAAwPwYDVR0RBDgwNoIEaG9zdIIHbWVtY29uboIJbG9jYWxo
b3N0ghoqLm1lbWNvbm4uYWt1dHouZ2l0aHViLmNvbTAdBgNVHQ4EFgQU1W1W6BhC
tL7bMLitcLEnkAVE0TUwDQYJKoZIhvcNAQEFBQADggEBAGnGW3xwU/8rHlYoQY9e
lQHq6MQaJrzdQOnFNEKbejmQ8birctWiT6zmich+Aqr2FYqFSz1OKdFJXtoCyzLv
qe2sAQlBh5Anqy6v3TYzOM4+yEH5IzYL+1VGHhbe6mZmzHUnentf9/va9htDeagT
DjoZPFPxF/u+TKyzapq4fdo7tBgKRZC61SnKjEq3vw3bLw+zQgzGnVb2aE3LFY34
qh8U5LoNtOu7JawryW8yT7d5W6UqKTN2BLCH0i2UrN9pkNTGwuHGJl33jH9Q6NuZ
zrU/OFhPR4fyk5N0PZ2MbMQkEC0JqxJUkoTs1aWYDiWTRLsk7tiWsNWKKPxhObWo
ZcU=
-----END CERTIFICATE-----`

	hostKey = `-----BEGIN PRIVATE KEY-----
MIIEvgIBADANBgkqhkiG9w0BAQEFAASCBKgwggSkAgEAAoIBAQDJVRieeIQCb4Rp
XiDL8cPs6QYxRzZXfHxTFK8GOhr08fBbW+HJbHCVlbQa/iLwJyjUc98oBiWmdNyZ
BLJDmEPxcjFGdD8CN6EArKto9R1NC1vEMdm3nMYzveQ0/4O9RqLxk12papCMZEom
v0Wv4ZsqhWTSl+gQMLt5uI51AAbpNmzO2eUHGHX7TDb73dcAiH77p5mrqDnFR+vd
1a6wJNeLIRoUBzeRpeKO+gTTgWgLfqYEk/eOL3ymA3IxPadWGleeQZl0el+2fau5
Z+L0Rlo4Tbdj2Gb2VG3r/GKBIB7ALW10JJloyi5sTn4cpPUJp8FX6RZAZ9gGcwJt
dJ38hqXLAgMBAAECggEBAK5otlQJqKoHexhgP18NSCICV6f2vb+aCoVaRKjLSzDo
KcSq2vTHqNwcfJJpl1CdS8SHwEiG0rTZRYSVSew+ipUtzDvxVegQ0run2TGqLUDh
1xQl7yodeKG4HWo/8xrThzJo69lohGHqO0ZHqhHMCcQTHJ1GlPT5kl7GnzoB1PrO
7qovfzFrvMgNat+NdqdHy0lAZt35cbshAK50TukYG+P85oow4Lt8+uth3+dh2pNE
tSDhPOfSGLU9kufImBdK6UVoTdxLQOWtdXf9tVtO2ehRuvlPm2virzGmyY2y8Gax
t+Tcw2XmvffVOATvcjRu44bXFAxhPCHAlTYhuepz0KkCgYEA4wmsPiZF0ff2fdzi
ttVhyrisvhIy3QCsxlchHUbmvpIv0KW26MJprMmLGa/wTIXJEa0PQX4sGNObgqGl
+K/Uq7uolooc4Y9PneVr/H/GaogtXLiursTNY6XluU0r6iq8aoqKJMT+vr1mN9Ez
FK3HaNGqgGqkRdVCVXzTwdOfSDcCgYEA4wP3z3vaezJjJt2fkNKcfruevTcA5DUz
nK3q0iXNl+tKJsgrvkLrbj9ElJocIom9PkgKjcXQiG9Z+F2g4/rrJ++YV8Mbk58g
jGBXMjWWLwJ9AMqUkgQ3/mZLD69+j2+9VxitIIJKcUyn5KJSOvU/Mlr4ur/pQavz
EjsoDd9sXQ0CgYEAuwWiz2dzqG0crb2hPH82GWpbUg9nusntiU0IyDc5qM5/eN6p
d79+kYlMfpKB3mdupJLsuESZSrI1rjw+nkcpZ3YkgC2xcNU+/pCYjd0rs2IODA1O
SEVx854bSLObc0BVCWaqOXPVbYZTh7Na4rPsSho825/9RlFQXV+AiHAtC60CgYAj
m/O7MApNWNIEvq7Q4Lh7iKKVu5MAOPgnk4BKBnQBaH7xJmT2KzkSygnP5XyUTlbI
9jPxmR3kyNKsCsO5/xnz4blbytcAiO1qF5KV5aHxLcq93QkA/nhqB1Gu3DBV/4kL
qGs/tjBHJWcQjgWoCeAn3e02HfRQwNAYA/98bZdp4QKBgDo6P0VWVFjVCkWgyFay
rUliOMnNbwVLh7E1HwRM9DN7j6YFVnkhP6uhGcDiZjQHCKjGVX/INOdDvm6ck55h
iKGJnYJ6BdQGeUd+42vE5md+f5jujt46oK36eMyT7j52NUp/7cWpOW6c16O1wchy
P+eS2Jh9T9rdxcFOY9Myr6zt
-----END PRIVATE KEY-----`
)

var (
	debug, _   = strconv.ParseBool(os.Getenv("MEMCONN_DEBUG"))
	verbose, _ = strconv.ParseBool(os.Getenv("MEMCONN_VERBOSE"))
)

type connLogger struct {
	net.Conn
	opCounter *uint64
	isRemote  bool
}

// Read implements the net.Conn Read method.
func (c *connLogger) Read(b []byte) (rn int, failed error) {
	if debug {
		c.logReadOrWrite("R1", 0, b)
	}
	n, err := c.Conn.Read(b)
	if debug {
		f := c.logReadOrWrite("R2", n, b)
		defer func() {
			f(failed)
		}()
	}
	return n, err
}

// Write implements the net.Conn Write method.
func (c *connLogger) Write(b []byte) (wn int, failed error) {
	if debug {
		c.logReadOrWrite("W1", 0, b)
	}
	n, err := c.Conn.Write(b)
	if debug {
		f := c.logReadOrWrite("W2", n, b)
		defer func() {
			f(failed)
		}()
	}
	return n, err
}

var logReadOrWriteHeaderOnce sync.Once

func (c *connLogger) logReadOrWrite(op string, n int, b []byte) func(error) {

	logReadOrWriteHeaderOnce.Do(func() {
		log("SRC RW ID  LEN_DAT HASH_DAT LEN_BUF HASH_BUF BUF\n")
	})

	rtype := "LOC"
	if c.isRemote {
		rtype = "REM"
	}

	var (
		hashDat string
		hashBuf string
	)
	{
		h := md5.New()
		h.Write(b)
		hashBuf = fmt.Sprintf("%x", h.Sum(nil))[0:7]
	}
	{
		h := md5.New()
		h.Write(b[:n])
		hashDat = fmt.Sprintf("%x", h.Sum(nil))[0:7]
	}

	var id uint64
	switch op {
	case "R1":
		id = atomic.AddUint64(c.opCounter, 1)
	case "R2":
		id = atomic.LoadUint64(c.opCounter)
	case "W1":
		id = atomic.AddUint64(c.opCounter, 1)
	case "W2":
		id = atomic.LoadUint64(c.opCounter)
	}

	patt := "%s %-2s %03d %-7d %-8s %-7d %-8s %s"

	const max = 8
	var szB string
	if verbose || (n > 0 && n <= max) {
		szB = fmt.Sprintf("%v", b)
	} else {
		szB = fmt.Sprintf("%v", b[:max])
		szB = szB[:len(szB)-1]
		szB = szB + " ...]"
	}

	log(patt+"\n", rtype, op, id, n, hashDat, len(b), hashBuf, szB)

	return func(err error) {
		if err != nil {
			log(patt+" err=%v\n",
				rtype, op, id, n, hashDat, len(b), hashBuf, szB, err)
		}
	}
}

func log(msg string, args ...interface{}) {
	if !debug {
		return
	}
	fmt.Fprintf(os.Stderr, msg, args...)
}
