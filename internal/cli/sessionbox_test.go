package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/openbuzz/interview-labs/internal/session"
)

// newBoxSession stores one session under a temp XDG root for box tests.
func newBoxSession(t *testing.T, status, ip string) *session.Session {
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

func TestSessionBoxFields(t *testing.T) {
	s := newBoxSession(t, session.StatusReady, "203.0.113.9")

	box := sessionBox(s)

	for _, want := range []string{
		"Session " + s.Meta.Slug, "status", "ready", "provider", "digitalocean",
		"ip", "203.0.113.9", "os", "ubuntu-26-04-x64", "ssh user", "root",
		"region", "fra1", "size", "s-2vcpu-2gb", "created", "╭",
	} {
		if !strings.Contains(box, want) {
			t.Fatalf("box missing %q:\n%s", want, box)
		}
	}
}

func TestSessionBoxEmptyIP(t *testing.T) {
	s := newBoxSession(t, session.StatusFailed, "")
	if box := sessionBox(s); strings.Contains(box, "ip") {
		t.Fatalf("empty IP must omit the row:\n%s", box)
	}
}

func TestPrintHandoverReady(t *testing.T) {
	s := newBoxSession(t, session.StatusReady, "203.0.113.9")
	var buf bytes.Buffer

	printHandover(&buf, s)

	out := buf.String()
	for _, want := range []string{
		"Session " + s.Meta.Slug, "root@203.0.113.9",
		"interview ssh " + s.Meta.Slug, "interview destroy " + s.Meta.Slug,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("handover missing %q:\n%s", want, out)
		}
	}
}

func TestPrintHandoverFailedOmitsSSH(t *testing.T) {
	s := newBoxSession(t, session.StatusFailed, "")
	var buf bytes.Buffer

	printHandover(&buf, s)

	out := buf.String()
	if strings.Contains(out, "interview ssh") {
		t.Fatalf("failed session offers ssh:\n%s", out)
	}
	if !strings.Contains(out, "interview destroy "+s.Meta.Slug) {
		t.Fatalf("failed session missing destroy hint:\n%s", out)
	}
}

func TestSessionBoxDNSAndAIRows(t *testing.T) {
	s := newBoxSession(t, session.StatusReady, "203.0.113.9")
	s.Meta.Roles["ai"] = "openrouter"
	s.Meta.AIKeyHash, s.Meta.AICapUSD = "hash-1", 12.5
	if err := s.SetFQDN("calm-otter.example.test"); err != nil {
		t.Fatal(err)
	}

	box := sessionBox(s)

	for _, want := range []string{
		"dns", "calm-otter.example.test", "ai", "openrouter (cap $12.5)",
	} {
		if !strings.Contains(box, want) {
			t.Fatalf("box missing %q:\n%s", want, box)
		}
	}
}

func TestSessionBoxOmitsRowsWithoutAIAndDNS(t *testing.T) {
	s := newBoxSession(t, session.StatusReady, "203.0.113.9")

	box := sessionBox(s)

	if strings.Contains(box, "dns") || strings.Contains(box, "cap $") {
		t.Fatalf("plain session shows dns/ai rows:\n%s", box)
	}
}

func TestSessionBoxNoAIRowWhenMintNeverRan(t *testing.T) {
	s := newBoxSession(t, session.StatusFailed, "")
	s.Meta.Roles["ai"] = "openrouter" // role recorded, mint never reached

	if box := sessionBox(s); strings.Contains(box, "cap $") {
		t.Fatalf("unminted session shows ai row:\n%s", box)
	}
}
