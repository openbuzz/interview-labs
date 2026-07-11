package provider

import (
	"context"
	"fmt"
	"io"
	"time"
)

// RetryDelays are the sleeps between credential-validation attempts: fresh
// AWS IAM keys can take seconds to propagate. Exported so tests can shrink it.
var RetryDelays = []time.Duration{time.Second, 2 * time.Second, 4 * time.Second}

// sleep is a seam: context-aware delay.
var sleep = func(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}

// Retry runs fn up to len(RetryDelays)+1 times. onAttempt (optional) fires
// before each try with (attempt, total). Returns nil on the first success,
// the last error otherwise; a cancelled context stops the retrying early.
func Retry(ctx context.Context, onAttempt func(attempt, total int),
	fn func(context.Context) error) error {
	total := len(RetryDelays) + 1

	var err error
	for i := 0; i < total; i++ {
		if onAttempt != nil {
			onAttempt(i+1, total)
		}
		if err = fn(ctx); err == nil {
			return nil
		}
		if i == len(RetryDelays) {
			break
		}
		if sleepErr := sleep(ctx, RetryDelays[i]); sleepErr != nil {
			return err
		}
	}
	return err
}

// TestCredentials reports a credential validation run on out: banner, spinner
// step with attempt retitling, resolved status row. step renders the spinner
// (ui.Step in production; injected so provider doesn't import ui).
func TestCredentials(ctx context.Context, out io.Writer,
	step func(io.Writer, string, func(update func(string)) error) error,
	fn func(context.Context) error) error {
	fmt.Fprintln(out, "Testing credentials…")

	return step(out, "validating credentials", func(update func(string)) error {
		return Retry(ctx, func(attempt, total int) {
			if attempt > 1 {
				update(fmt.Sprintf("validating credentials (attempt %d/%d)",
					attempt, total))
			}
		}, fn)
	})
}
