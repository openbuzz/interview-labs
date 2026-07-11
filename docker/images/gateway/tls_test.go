package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeTestCert generates a self-signed cert for 127.0.0.1 into dir,
// returning the cert and key file paths.
func writeTestCert(t *testing.T, dir string) (certPath, keyPath string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "127.0.0.1"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}

	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}

	certPath = filepath.Join(dir, "cert.pem")
	keyPath = filepath.Join(dir, "key.pem")
	certOut, _ := os.Create(certPath)
	pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: der})
	certOut.Close()

	keyDER, _ := x509.MarshalECPrivateKey(key)
	keyOut, _ := os.Create(keyPath)
	pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	keyOut.Close()

	return certPath, keyPath
}

func TestServeTLS(t *testing.T) {
	dir := t.TempDir()
	certPath, keyPath := writeTestCert(t, dir)

	t.Setenv("GATEWAY_PASSWORD", "pw")
	t.Setenv("GATEWAY_TLS_CERT", certPath)
	t.Setenv("GATEWAY_TLS_KEY", keyPath)

	cfg, err := loadConfig(nil)
	if err != nil {
		t.Fatal(err)
	}
	srv := newServer(cfg, newLogger("error", "text"))

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	httpSrv := &http.Server{Handler: srv.handler()}
	go httpSrv.ServeTLS(ln, certPath, keyPath)
	defer httpSrv.Close()

	addr := ln.Addr().String()
	client := &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}}

	// https /healthz returns 200
	resp, err := client.Get("https://" + addr + "/healthz")
	if err != nil {
		t.Fatalf("https healthz: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("https healthz status = %d", resp.StatusCode)
	}
	resp.Body.Close()

	// plain http to the TLS listener must fail (Go returns 400, not a transport error)
	plainResp, plainErr := http.Get("http://" + addr + "/healthz")
	if plainErr == nil {
		defer plainResp.Body.Close()
		if plainResp.StatusCode == http.StatusOK {
			t.Fatalf("plain http to TLS listener should not return 200")
		}
	}

	// runHealthcheck probes https successfully
	if code := runHealthcheck(addr, true); code != 0 {
		t.Fatalf("runHealthcheck over TLS = %d, want 0", code)
	}
}
