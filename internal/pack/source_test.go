package pack

import (
	"strings"
	"testing"
)

func TestResolveEmbeddedNames(t *testing.T) {
	for _, ref := range []string{"", "default"} {
		p, err := Resolve(ref)
		if err != nil || p.Name != "default" {
			t.Fatalf("Resolve(%q) = %v, %v", ref, p, err)
		}
	}
	p, err := Resolve("template")
	if err != nil || p.Name != "template" {
		t.Fatalf("Resolve(template) = %v, %v", p, err)
	}
}

func TestResolveDir(t *testing.T) {
	p, err := Resolve("testdata/goodpack")
	if err != nil || p.Name != "goodpack" {
		t.Fatalf("Resolve(dir) = %v, %v", p, err)
	}
}

func TestResolveMissingDir(t *testing.T) {
	_, err := Resolve("testdata/nope")
	if err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("err = %v", err)
	}
}
