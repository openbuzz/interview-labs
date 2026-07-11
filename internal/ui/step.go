package ui

import (
	"fmt"
	"io"
	"sync"
	"time"
)

var stepFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Step runs fn as one reported step. Interactive terminals get an animated
// single line that resolves into a status row with elapsed time;
// non-interactive writers get a plain start line and the same final row.
// fn may retitle the live line through update (e.g. retry counters).
func Step(w io.Writer, title string, fn func(update func(string)) error) error {
	start := time.Now()
	var mu sync.Mutex
	current := title
	update := func(t string) {
		mu.Lock()
		current = t
		mu.Unlock()
	}
	read := func() string {
		mu.Lock()
		defer mu.Unlock()
		return current
	}

	if !Interactive() {
		fmt.Fprintln(w, Faint.Render("… "+title))
		err := fn(update)
		finishStep(w, read(), start, err)
		return err
	}

	done := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		tick := time.NewTicker(120 * time.Millisecond)
		defer tick.Stop()
		for i := 0; ; i++ {
			select {
			case <-done:
				return
			case <-tick.C:
				fmt.Fprintf(w, "\r\x1b[2K%s %s (%s)",
					Accent.Render(stepFrames[i%len(stepFrames)]), read(),
					stepElapsed(start))
			}
		}
	}()

	err := fn(update)
	close(done)
	<-stopped
	fmt.Fprint(w, "\r\x1b[2K")
	finishStep(w, read(), start, err)
	return err
}

// finishStep prints the resolved row for a step.
func finishStep(w io.Writer, title string, start time.Time, err error) {
	if err != nil {
		fmt.Fprintln(w, RowFail(title, err.Error()))
		return
	}
	fmt.Fprintln(w, RowOK(title, stepElapsed(start)))
}

// stepElapsed renders a compact duration: 0s, 42s, 1m12s.
func stepElapsed(start time.Time) string {
	d := time.Since(start).Truncate(time.Second)
	if d >= time.Minute {
		return fmt.Sprintf("%dm%02ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%ds", int(d.Seconds()))
}
