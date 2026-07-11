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
	privPEM, pub = testClientKeypair(t)
	conf := testServerConfig(t)

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
			go serveConn(conn, conf)
		}
	}()
	return ln.Addr().String(), privPEM, pub
}

// testClientKeypair mints the client identity handed back to the test.
func testClientKeypair(t *testing.T) (privPEM, pub string) {
	t.Helper()
	_, clientPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	pemBlock, err := cryptossh.MarshalPrivateKey(clientPriv, "")
	if err != nil {
		t.Fatal(err)
	}
	signer, err := cryptossh.NewSignerFromKey(clientPriv)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(pemBlock)),
		string(cryptossh.MarshalAuthorizedKey(signer.PublicKey()))
}

// testServerConfig builds an accept-any-key server config with a fresh host key.
func testServerConfig(t *testing.T) *cryptossh.ServerConfig {
	t.Helper()
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
	return conf
}

// serveConn upgrades one TCP conn and answers its channels.
func serveConn(conn net.Conn, conf *cryptossh.ServerConfig) {
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
		go answerExec(ch, chReqs)
	}
}

// answerExec replies to exec with a greeting and a zero exit status.
func answerExec(ch cryptossh.Channel, reqs <-chan *cryptossh.Request) {
	for req := range reqs {
		if req.Type != "exec" {
			req.Reply(false, nil)
			continue
		}
		req.Reply(true, nil)
		ch.Write([]byte("Hello world\n"))
		ch.SendRequest("exit-status", false, []byte{0, 0, 0, 0})
		ch.Close()
	}
}
