package session

import (
	"regexp"
	"testing"
)

func TestNewSlugFormat(t *testing.T) {
	slug := newSlug(func(string) bool { return false })
	if !regexp.MustCompile(`^[a-z]+-[a-z]+$`).MatchString(slug) {
		t.Fatalf("slug %q, want two lowercase words", slug)
	}
}

func TestNewSlugRetriesOnCollision(t *testing.T) {
	calls := 0
	_ = newSlug(func(string) bool {
		calls++
		return calls == 1 // first candidate collides
	})

	if calls < 2 {
		t.Fatalf("exists called %d times, want at least 2", calls)
	}
}
