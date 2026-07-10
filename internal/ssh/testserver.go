package ssh

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"net"
	"testing"

	cryptossh "golang.org/x/crypto/ssh"
)

// StartTestServer runs a minimal ssh server answering exec requests with "Hello world".
// Returns its address and the client keypair (PEM private, authorized-keys public).
func StartTestServer(t *testing.T) (addr, privPEM, pub string) {
	t.Helper()
	_, clientPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	pemBlock, err := cryptossh.MarshalPrivateKey(clientPriv, "")
	if err != nil {
		t.Fatal(err)
	}
	privPEM = string(pem.EncodeToMemory(pemBlock))
	signer, err := cryptossh.NewSignerFromKey(clientPriv)
	if err != nil {
		t.Fatal(err)
	}
	pub = string(cryptossh.MarshalAuthorizedKey(signer.PublicKey()))

	_, hostPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	hostSigner, err := cryptossh.NewSignerFromKey(hostPriv)
	if err != nil {
		t.Fatal(err)
	}

	conf := &cryptossh.ServerConfig{
		PublicKeyCallback: func(_ cryptossh.ConnMetadata,
			key cryptossh.PublicKey) (*cryptossh.Permissions, error) {
			return &cryptossh.Permissions{}, nil
		},
	}
	conf.AddHostKey(hostSigner)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				sconn, chans, reqs, err := cryptossh.NewServerConn(conn, conf)
				if err != nil {
					return
				}
				defer sconn.Close()
				go cryptossh.DiscardRequests(reqs)
				for newCh := range chans {
					ch, chReqs, err := newCh.Accept()
					if err != nil {
						continue
					}
					go func() {
						for req := range chReqs {
							if req.Type == "exec" {
								req.Reply(true, nil)
								ch.Write([]byte("Hello world\n"))
								ch.SendRequest("exit-status", false,
									[]byte{0, 0, 0, 0})
								ch.Close()
							} else {
								req.Reply(false, nil)
							}
						}
					}()
				}
			}()
		}
	}()
	return ln.Addr().String(), privPEM, pub
}
