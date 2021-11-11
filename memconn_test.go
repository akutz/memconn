package memconn_test

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"os"
	"testing"
	"time"

	"github.com/akutz/memconn"
)

var args struct {
	clients int
}

func TestMain(m *testing.M) {
	// Seed the random package so the values returned by rand.Int63n
	// are not predetermined.
	rand.Seed(time.Now().UnixNano())

	flag.IntVar(
		&args.clients,
		"clients", 1,
		"The number of clients accessing a server during a test execution.")

	flag.Parse()

	// Discard all log statements unless -test.v is present.
	if !testing.Verbose() {
		log.SetOutput(ioutil.Discard)
	}

	os.Exit(m.Run())
}

func TestBuffered(t *testing.T) {
	testParallel(t, "memb", false)
}

func TestUnbuffered(t *testing.T) {
	testParallel(t, "memu", false)
}

func TestBufferedTLS(t *testing.T) {
	testParallel(t, "memb", true)
}

func TestUnbufferedTLS(t *testing.T) {
	testParallel(t, "memu", true)
}

func testParallel(t *testing.T, network string, useTLS bool) {
	var (
		rootCAs     *x509.CertPool
		hostKeyPair tls.Certificate
	)
	if useTLS {
		// Create the cert pool for the CA.
		rootCAs = x509.NewCertPool()
		rootCAs.AppendCertsFromPEM([]byte(caCrt))

		// Load the host certificate and key.
		var err error
		hostKeyPair, err = tls.X509KeyPair([]byte(hostCrt), []byte(hostKey))
		if err != nil {
			t.Fatalf("error loading host key pair: %v", err)
		}
	}

	// Announce a new listener named "localhost" the specified network.
	lis, err := memconn.Listen(network, "localhost")
	if err != nil {
		t.Fatalf("error announcing new listener: network=%s address=%s: %v",
			network, "localhost", err)
	}

	// Ensure the listener is closed.
	defer lis.Close()

	// Start a goroutine that will wait for a client to dial the
	// listener and then echo back any data sent to the remote
	// connection.
	go func() {
		for {
			conn, err := lis.Accept()
			if err != nil {
				e, ok := err.(*net.OpError)
				if ok && e.Err != nil && e.Err.Error() == "listener closed" {
					return
				}
				t.Fatalf("error accepting cnxn: %v", err)
			}

			go func(conn net.Conn) {

				// Set the transmit buffer to 64KiB
				conn.(*memconn.Conn).SetWriteBuffer(64 * 1024)

				// Set the limit for the number of bytes buffered to 10MiB.
				conn.(*memconn.Conn).SetWriteBufferLimit(10 * 1024 * 1024)

				if useTLS {
					// Wrap the new connection inside of a TLS server.
					conn = tls.Server(conn, &tls.Config{
						Certificates: []tls.Certificate{hostKeyPair},
						RootCAs:      rootCAs,
					})
				}

				// Ensure the connection is closed.
				defer func() {
					if network == "memu" {
						go io.Copy(ioutil.Discard, conn)
					}
					conn.Close()
				}()

				// Echo any data received from the connection.
				if _, err := io.Copy(conn, conn); err != nil {
					t.Fatalf("error echoing data: %v", err)
				}
			}(conn)
		}
	}()

	t.Run("Client", func(t *testing.T) {
		for i := 0; i < args.clients; i++ {
			t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
				t.Parallel()

				// Dial the network named "localhost".
				conn, err := memconn.Dial(network, "localhost")
				if err != nil {
					t.Fatalf("error dialing: network=%s address=%s: %v",
						network, "localhost", err)
				}

				// Set the transmit buffer to 64KiB
				conn.(*memconn.Conn).SetWriteBuffer(64 * 1024)

				// Set the limit for the number of bytes buffered to 10MiB.
				conn.(*memconn.Conn).SetWriteBufferLimit(10 * 1024 * 1024)

				if useTLS {
					// Wrap the connection in TLS. It's necessary to set the
					// "ServerName" field in the TLS configuration in order
					// to match one of the host certificate's Subject Alternate
					// Name values.
					conn = tls.Client(conn, &tls.Config{
						RootCAs:    rootCAs,
						ServerName: "localhost",
					})
				}

				// Ensure the connection is closed.
				defer func() {
					if network == "memu" {
						go io.Copy(ioutil.Discard, conn)
					}
					conn.Close()
				}()

				// Get the number of bytes to write then read from
				// the connection. The value is between 4MiB-8MiB.
				n := rand.Int63n(4194304) + 4194304

				// Create a buffer n bytes in length.
				out := make([]byte, n)

				// Fill the buffer with random data.
				rand.Read(out)

				// Write the data to the connection.
				if network == "memu" {
					// If the network is unbuffered then the data must
					// be written on a separate goroutine so the write
					// isn't blocked.
					go func() {
						_, err := io.Copy(conn, bytes.NewReader(out))
						if err != nil {
							t.Fatalf("error writing data: %v", err)
						}
					}()
				} else {
					_, err := io.Copy(conn, bytes.NewReader(out))
					if err != nil {
						t.Fatalf("error writing data: %v", err)
					}
				}

				// The remote side of the connection should echo the
				// same data back to this side of the connection.
				//
				// Create a new buffer with the same size as the
				// original and read the echoed data.
				in := &bytes.Buffer{}
				inn, err := io.CopyN(in, conn, n)
				if err != nil {
					t.Fatalf("error reading data: %v", err)
				}

				// Verify the copied and echoed data are equal.
				if !bytes.Equal(out, in.Bytes()) {
					t.Fatalf("echo failed: len(out)=%d len(in)=%d", n, inn)
				}
			})
		}
	})
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
