package memconn_test

import (
	"crypto/tls"
	"crypto/x509"
	"io"
	"os"

	"github.com/akutz/memconn"
)

// ExampleBufferedTLS illustrates a server and client that
// communicate over a buffered, in-memory connection using TLS.
func Example_bufferedTLS() {
	// Create the cert pool for the CA.
	rootCAs := x509.NewCertPool()
	rootCAs.AppendCertsFromPEM([]byte(tlsBufferedCACrt))

	// Load the host certificate and key.
	hostKeyPair, _ := tls.X509KeyPair(
		[]byte(tlsBufferedCrt), []byte(tlsBufferedKey))

	// Announce a new listener named "localhost" on MemConn's
	// buffered network, "memb".
	lis, _ := memconn.Listen("memb", "localhost")

	// Ensure the listener is closed.
	defer lis.Close()

	// Start a goroutine that will wait for a client to dial the
	// listener and then echo back any data sent to the remote
	// connection.
	go func() {
		conn, _ := lis.Accept()

		// Wrap the new connection inside of a TLS server.
		conn = tls.Server(conn, &tls.Config{
			Certificates: []tls.Certificate{hostKeyPair},
			RootCAs:      rootCAs,
		})

		// If no errors occur then make sure the connection is closed.
		defer conn.Close()

		// Echo the data back to the client.
		io.Copy(conn, conn)
	}()

	// Dial the buffered, in-memory network named "localhost".
	conn, _ := memconn.Dial("memb", "localhost")

	// Wrap the connection in TLS. It's necessary to set the "ServerName"
	// field in the TLS configuration in order to match one of the host
	// certificate's Subject Alternate Name values.
	conn = tls.Client(conn, &tls.Config{
		RootCAs:    rootCAs,
		ServerName: "localhost",
	})

	// Ensure the connection is closed.
	defer conn.Close()

	// Write the data to the server.
	go conn.Write([]byte("Hello, world."))

	// Read the data from the server.
	io.CopyN(os.Stdout, conn, 13)

	// Output: Hello, world.
}

const (
	tlsBufferedCrt = `-----BEGIN CERTIFICATE-----
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

	tlsBufferedKey = `-----BEGIN PRIVATE KEY-----
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

	tlsBufferedCACrt = `-----BEGIN CERTIFICATE-----
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
)
