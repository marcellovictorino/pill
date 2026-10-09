package cmd

import (
	"fmt"
	"time"

	"github.com/marcellovictorino/pill/internal/hf"
)

// downloadProgress draws a one-line progress indicator on stderr, but only for
// a person at a terminal. Agents and pipes get nothing, so their output stays
// clean and cheap.
func (a *App) downloadProgress(name string) hf.Progress {
	if !a.printer.TTY || !a.printer.Human {
		return nil
	}
	start := time.Now()
	return func(done, total int64) {
		pct := 0.0
		if total > 0 {
			pct = float64(done) / float64(total) * 100
		}
		rate := float64(done) / 1e6 / max(time.Since(start).Seconds(), 0.001)
		fmt.Fprintf(a.deps.Err, "\r  %s  %5.1f%%  %s / %s  %.0f MB/s   ", name, pct, humanBytes(done), humanBytes(total), rate)
		if done >= total && total > 0 {
			fmt.Fprintln(a.deps.Err)
		}
	}
}
