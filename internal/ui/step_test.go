package ui

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
)

func nonInteractive(t *testing.T) {
	t.Helper()
	old := Interactive
	Interactive = func() bool { return false }
	t.Cleanup(func() { Interactive = old })
}

func TestStepPlainSuccess(t *testing.T) {
	nonInteractive(t)
	var buf bytes.Buffer

	err := Step(&buf, "terraform apply", func(func(string)) error { return nil })

	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "… terraform apply") {
		t.Fatalf("missing start line:\n%s", out)
	}
	if !strings.Contains(out, GlyphOK+" terraform apply") || !strings.Contains(out, "0s") {
		t.Fatalf("missing final row:\n%s", out)
	}
}

func TestStepPlainFailure(t *testing.T) {
	nonInteractive(t)
	var buf bytes.Buffer
	boom := errors.New("bang")

	err := Step(&buf, "stage", func(func(string)) error { return boom })

	if !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(buf.String(), GlyphFail+" stage") ||
		!strings.Contains(buf.String(), "bang") {
		t.Fatalf("missing fail row:\n%s", buf.String())
	}
}

func TestStepTitleUpdateWins(t *testing.T) {
	nonInteractive(t)
	var buf bytes.Buffer

	_ = Step(&buf, "validating", func(update func(string)) error {
		update("validating (attempt 2/4)")
		return nil
	})

	if !strings.Contains(buf.String(), "validating (attempt 2/4)") {
		t.Fatalf("updated title not rendered:\n%s", buf.String())
	}
}

func TestStepInteractiveAnimates(t *testing.T) {
	old := Interactive
	Interactive = func() bool { return true }
	t.Cleanup(func() { Interactive = old })
	var buf bytes.Buffer

	err := Step(&buf, "spin", func(func(string)) error {
		time.Sleep(300 * time.Millisecond)
		return nil
	})

	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "\r") {
		t.Fatalf("no carriage-return frames:\n%q", buf.String())
	}
	if !strings.Contains(buf.String(), GlyphOK+" spin") {
		t.Fatalf("missing final row:\n%q", buf.String())
	}
}

func TestStepRowsIndented(t *testing.T) {
	old := Interactive
	Interactive = func() bool { return false }
	t.Cleanup(func() { Interactive = old })

	var buf bytes.Buffer
	if err := Step(&buf, "stage", func(func(string)) error { return nil }); err != nil {
		t.Fatal(err)
	}

	for i, line := range strings.Split(strings.TrimRight(buf.String(), "\n"), "\n") {
		if !strings.HasPrefix(line, "  ") {
			t.Fatalf("step line %d not indented: %q", i, line)
		}
	}
}
