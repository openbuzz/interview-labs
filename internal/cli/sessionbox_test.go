package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/openbuzz/interview-labs/internal/session"
)

// newFactsSession stores one session under a temp XDG root for section tests.
func newFactsSession(t *testing.T, status, ip string) *session.Session {
	t.Helper()
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	s, err := session.New("fra1", "s-2vcpu-2gb", "ubuntu-26-04-x64", "root",
		map[string]string{"vm": "digitalocean"},
		session.TerraformInfo{Binary: "terraform", Version: "1.14.8"})
	if err != nil {
		t.Fatal(err)
	}
	if ip != "" {
		if err := s.SetIP(ip); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.SetStatus(status); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestSessionSectionFields(t *testing.T) {
	s := newFactsSession(t, session.StatusReady, "203.0.113.9")

	sec := sessionSection(s)

	for _, want := range []string{
		"SESSION", s.Meta.Slug, "ready", "provider", "digitalocean",
		"ip", "203.0.113.9", "os", "ubuntu-26-04-x64", "ssh user", "root",
		"region", "fra1", "size", "s-2vcpu-2gb", "created",
	} {
		if !strings.Contains(sec, want) {
			t.Fatalf("section missing %q:\n%s", want, sec)
		}
	}
	for _, forbid := range []string{"╭", "│", "status", "password", "url"} {
		if strings.Contains(sec, forbid) {
			t.Fatalf("section still contains %q:\n%s", forbid, sec)
		}
	}
}

func TestSessionSectionBundleRow(t *testing.T) {
	s := newFactsSession(t, session.StatusReady, "203.0.113.9")
	s.Meta.Pack, s.Meta.Bundle = "default", "devops"

	if sec := sessionSection(s); !strings.Contains(sec, "devops") {
		t.Fatalf("bundle row missing:\n%s", sec)
	}
}

func TestSessionSectionEmptyIP(t *testing.T) {
	s := newFactsSession(t, session.StatusFailed, "")
	if sec := sessionSection(s); strings.Contains(sec, "\n  ip ") {
		t.Fatalf("empty IP must omit the row:\n%s", sec)
	}
}

func TestPrintHandoverReadyZones(t *testing.T) {
	s := newFactsSession(t, session.StatusReady, "203.0.113.9")
	s.Meta.GatewayPassword = "9f3a1c778d02b4e6"
	if err := s.SetURL("http://203.0.113.9"); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer

	printHandover(&buf, s)

	out := buf.String()
	for _, want := range []string{
		"SESSION", s.Meta.Slug,
		"SEND TO CANDIDATE", "http://203.0.113.9", "password: 9f3a1c778d02b4e6",
		"SSH", "root@203.0.113.9", "─",
		"interview ssh " + s.Meta.Slug, "interview destroy " + s.Meta.Slug,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("handover missing %q:\n%s", want, out)
		}
	}

	// The copy-zone interiors must be bare: the URL line is exactly the URL.
	found := false
	for _, line := range strings.Split(out, "\n") {
		if line == "http://203.0.113.9" {
			found = true
		}
	}
	if !found {
		t.Fatalf("url line not verbatim/bare:\n%s", out)
	}
}

func TestPrintHandoverFailedOmitsZones(t *testing.T) {
	s := newFactsSession(t, session.StatusFailed, "")
	s.Meta.GatewayPassword = "9f3a1c778d02b4e6"
	var buf bytes.Buffer

	printHandover(&buf, s)

	out := buf.String()
	for _, forbid := range []string{"SEND TO CANDIDATE", "SSH", "interview ssh"} {
		if strings.Contains(out, forbid) {
			t.Fatalf("failed session shows %q:\n%s", forbid, out)
		}
	}
	if !strings.Contains(out, "interview destroy "+s.Meta.Slug) {
		t.Fatalf("failed session missing destroy hint:\n%s", out)
	}
}

func TestSessionSectionDNSAndAIRows(t *testing.T) {
	s := newFactsSession(t, session.StatusReady, "203.0.113.9")
	s.Meta.Roles["ai"] = "openrouter"
	s.Meta.AIKeyHash, s.Meta.AICapUSD = "hash-1", 12.5
	if err := s.SetFQDN("calm-otter.example.test"); err != nil {
		t.Fatal(err)
	}

	sec := sessionSection(s)

	for _, want := range []string{
		"dns", "calm-otter.example.test", "ai", "openrouter (cap $12.5)",
	} {
		if !strings.Contains(sec, want) {
			t.Fatalf("section missing %q:\n%s", want, sec)
		}
	}
}

func TestSessionSectionNoAIRowWhenMintNeverRan(t *testing.T) {
	s := newFactsSession(t, session.StatusFailed, "")
	s.Meta.Roles["ai"] = "openrouter" // role recorded, mint never reached

	if sec := sessionSection(s); strings.Contains(sec, "cap $") {
		t.Fatalf("unminted session shows ai row:\n%s", sec)
	}
}
