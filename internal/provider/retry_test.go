package provider

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"
)

func fastRetries(t *testing.T) *[]time.Duration {
	t.Helper()
	oldDelays, oldSleep := RetryDelays, sleep
	slept := &[]time.Duration{}
	RetryDelays = []time.Duration{time.Second, 2 * time.Second, 4 * time.Second}
	sleep = func(ctx context.Context, d time.Duration) error {
		*slept = append(*slept, d)
		return ctx.Err()
	}
	t.Cleanup(func() { RetryDelays, sleep = oldDelays, oldSleep })
	return slept
}

func TestRetrySucceedsThirdAttempt(t *testing.T) {
	slept := fastRetries(t)
	var attempts []string
	calls := 0

	err := Retry(context.Background(), func(a, total int) {
		attempts = append(attempts, fmt.Sprintf("%d/%d", a, total))
	}, func(context.Context) error {
		calls++
		if calls < 3 {
			return errors.New("not yet")
		}
		return nil
	})

	if err != nil || calls != 3 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
	if want := []time.Duration{time.Second, 2 * time.Second}; len(*slept) != 2 ||
		(*slept)[0] != want[0] || (*slept)[1] != want[1] {
		t.Fatalf("slept = %v", *slept)
	}
	if strings.Join(attempts, " ") != "1/4 2/4 3/4" {
		t.Fatalf("attempts = %v", attempts)
	}
}

func TestRetryExhaustsAndReturnsLastError(t *testing.T) {
	slept := fastRetries(t)
	calls := 0

	err := Retry(context.Background(), nil, func(context.Context) error {
		calls++
		return fmt.Errorf("boom %d", calls)
	})

	if calls != 4 || err == nil || err.Error() != "boom 4" {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
	if len(*slept) != 3 {
		t.Fatalf("slept = %v", *slept)
	}
}

func TestRetryStopsOnCancelledContext(t *testing.T) {
	fastRetries(t)
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0

	err := Retry(ctx, nil, func(context.Context) error {
		calls++
		cancel() // cancelled while "sleeping" before the next attempt
		return errors.New("nope")
	})

	if calls != 1 || err == nil {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
}

func TestTestCredentialsReportsAttempts(t *testing.T) {
	fastRetries(t)
	var buf bytes.Buffer
	calls := 0

	// Deterministic step: print the title and every retitle, no spinner.
	step := func(w io.Writer, title string, fn func(update func(string)) error) error {
		fmt.Fprintln(w, title)
		return fn(func(s string) { fmt.Fprintln(w, s) })
	}

	err := TestCredentials(context.Background(), &buf, step, func(context.Context) error {
		calls++
		if calls < 2 {
			return errors.New("not active yet")
		}
		return nil
	})

	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "Testing credentials…") {
		t.Fatalf("missing banner:\n%s", out)
	}
	if !strings.Contains(out, "(attempt 2/4)") {
		t.Fatalf("missing attempt retitle:\n%s", out)
	}
}
