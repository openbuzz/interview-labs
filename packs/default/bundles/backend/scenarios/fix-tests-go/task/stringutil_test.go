package stringutil

import "testing"

func TestTruncate(t *testing.T) {
	// Should not truncate when text fits exactly.
	got := Truncate("hello", 5)
	if got != "hello" {
		t.Errorf("Truncate(\"hello\", 5) = %q, want %q", got, "hello")
	}

	// Should truncate long text with "..." suffix.
	got = Truncate("hello world", 8)
	if got != "hello..." {
		t.Errorf("Truncate(\"hello world\", 8) = %q, want %q", got, "hello...")
	}
}

func TestCapitalize(t *testing.T) {
	got := Capitalize("hello world")
	if got != "Hello World" {
		t.Errorf("Capitalize(\"hello world\") = %q, want %q", got, "Hello World")
	}
}

func TestWordCount(t *testing.T) {
	got := WordCount("the quick brown fox")
	if got != 4 {
		t.Errorf("WordCount(\"the quick brown fox\") = %d, want 4", got)
	}
}

func TestIsPalindrome(t *testing.T) {
	if !IsPalindrome("Race Car") {
		t.Error("IsPalindrome(\"Race Car\") = false, want true")
	}
	if IsPalindrome("hello") {
		t.Error("IsPalindrome(\"hello\") = true, want false")
	}
}

func TestSnakeToTitle(t *testing.T) {
	got := SnakeToTitle("hello_world")
	if got != "Hello World" {
		t.Errorf("SnakeToTitle(\"hello_world\") = %q, want %q", got, "Hello World")
	}
}
