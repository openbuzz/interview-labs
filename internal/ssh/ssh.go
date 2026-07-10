// Package ssh dials sessions with per-session pinned host keys.
package ssh

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	cryptossh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// WriteKeyFiles writes key (0600) and key.pub (0644) into sshDir.
func WriteKeyFiles(sshDir, privateKeyPEM, publicKey string) error {
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(sshDir, "key"),
		[]byte(privateKeyPEM), 0o600); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(sshDir, "key.pub"), []byte(publicKey), 0o644)
}

// Client is an established session connection.
type Client struct {
	c *cryptossh.Client
}

// Close closes the connection.
func (c *Client) Close() error { return c.c.Close() }

// Run executes command and returns its combined output.
func (c *Client) Run(ctx context.Context, command string) (string, error) {
	sess, err := c.c.NewSession()
	if err != nil {
		return "", err
	}
	defer sess.Close()

	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			sess.Close()
		case <-done:
		}
	}()
	out, err := sess.CombinedOutput(command)
	close(done)
	return string(out), err
}

// hostKeyCallback pins on first contact, verifies strictly afterwards.
func hostKeyCallback(knownHostsPath string) (cryptossh.HostKeyCallback, error) {
	return func(hostname string, remote net.Addr, key cryptossh.PublicKey) error {
		if _, err := os.Stat(knownHostsPath); errors.Is(err, os.ErrNotExist) {
			line := knownhosts.Line([]string{hostname}, key)
			return os.WriteFile(knownHostsPath, []byte(line+"\n"), 0o600)
		}
		verify, err := knownhosts.New(knownHostsPath)
		if err != nil {
			return err
		}
		return verify(hostname, remote, key)
	}, nil
}

// Dial connects with 3s retry until ctx expires. A host-key mismatch fails fast.
func Dial(ctx context.Context, addr, user, keyPath, knownHostsPath string) (*Client, error) {
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}
	signer, err := cryptossh.ParsePrivateKey(keyData)
	if err != nil {
		return nil, err
	}
	callback, err := hostKeyCallback(knownHostsPath)
	if err != nil {
		return nil, err
	}

	conf := &cryptossh.ClientConfig{
		User:            user,
		Auth:            []cryptossh.AuthMethod{cryptossh.PublicKeys(signer)},
		HostKeyCallback: callback,
		Timeout:         10 * time.Second,
	}
	var lastErr error
	for {
		conn, err := cryptossh.Dial("tcp", addr, conf)
		if err == nil {
			return &Client{c: conn}, nil
		}
		lastErr = err
		var keyErr *knownhosts.KeyError
		mismatch := errors.As(err, &keyErr) && len(keyErr.Want) > 0
		if mismatch || strings.Contains(err.Error(), "key mismatch") {
			return nil, fmt.Errorf("host key changed for %s: %w", addr, err)
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("ssh not reachable at %s: %w (last: %v)",
				addr, ctx.Err(), lastErr)
		case <-time.After(3 * time.Second):
		}
	}
}

// Argv builds the host-ssh handover command line.
func Argv(keyPath, knownHostsPath, user, ip string) []string {
	return []string{"ssh", "-i", keyPath,
		"-o", "UserKnownHostsFile=" + knownHostsPath, user + "@" + ip}
}
