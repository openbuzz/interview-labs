// Package ssh dials sessions with per-session pinned host keys.
package ssh

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	cryptossh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// Client is an established session connection.
type Client struct {
	c *cryptossh.Client
}

// Close closes the connection.
func (c *Client) Close() error { return c.c.Close() }

// watchCtx closes sess when ctx cancels; call the returned stop once the
// command finishes.
func watchCtx(ctx context.Context, sess *cryptossh.Session) (stop func()) {
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			sess.Close()
		case <-done:
		}
	}()
	return func() { close(done) }
}

// Run executes command and returns its combined output.
func (c *Client) Run(ctx context.Context, command string) (string, error) {
	return c.RunIn(ctx, nil, command)
}

// RunIn executes command with stdin streamed from r (nil for none) and
// returns the combined output.
func (c *Client) RunIn(ctx context.Context, r io.Reader, command string) (string, error) {
	sess, err := c.c.NewSession()
	if err != nil {
		return "", err
	}
	defer sess.Close()
	if r != nil {
		sess.Stdin = r
	}

	stop := watchCtx(ctx, sess)
	out, err := sess.CombinedOutput(command)
	stop()
	return string(out), err
}

// syncWriter serializes writes so stdout and stderr can share one
// destination without racing — they copy from independent goroutines.
type syncWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (s *syncWriter) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.w.Write(p)
}

// RunStream executes command, streaming combined output to w as it arrives
// — launch uses it for the long image build.
func (c *Client) RunStream(ctx context.Context, w io.Writer, command string) error {
	sess, err := c.c.NewSession()
	if err != nil {
		return err
	}
	defer sess.Close()
	sw := &syncWriter{w: w}
	sess.Stdout = sw
	sess.Stderr = sw

	stop := watchCtx(ctx, sess)
	err = sess.Run(command)
	stop()
	return err
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

// clientConfig assembles auth and host-key pinning for one session.
func clientConfig(user, keyPath, knownHostsPath string) (*cryptossh.ClientConfig, error) {
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

	return &cryptossh.ClientConfig{
		User:            user,
		Auth:            []cryptossh.AuthMethod{cryptossh.PublicKeys(signer)},
		HostKeyCallback: callback,
		Timeout:         10 * time.Second,
	}, nil
}

// Dial connects with 3s retry until ctx expires. A host-key mismatch fails fast.
func Dial(ctx context.Context, addr, user, keyPath, knownHostsPath string) (*Client, error) {
	conf, err := clientConfig(user, keyPath, knownHostsPath)
	if err != nil {
		return nil, err
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
