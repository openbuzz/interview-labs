package main

import (
	"crypto/tls"
	"net"
	"net/http"
)

func runHealthcheck(addr string, useTLS bool) int {
	_, port, err := net.SplitHostPort(addr)
	if err != nil || port == "" {
		port = "8080"
	}

	scheme := "http"
	client := http.DefaultClient
	if useTLS {
		scheme = "https"
		client = &http.Client{Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // localhost self-signed probe only
		}}
	}

	resp, err := client.Get(scheme + "://127.0.0.1:" + port + "/healthz")
	if err != nil {
		return 1
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 1
	}

	return 0
}
