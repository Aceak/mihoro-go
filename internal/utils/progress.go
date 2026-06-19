package utils

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

const barWidth = 30

// NewProgressBar creates a writer that renders a progress bar to stderr.
// label is shown on the left (e.g. "  subscribe   ").
// Call Done() after the write completes.
func NewProgressBar(label string, total int64) *ProgressBar {
	b := &ProgressBar{
		label:      label,
		total:      total,
		start:      time.Now(),
		lastRedraw: time.Now(),
	}
	b.redraw()
	return b
}

type ProgressBar struct {
	label      string
	total      int64
	current    int64
	start      time.Time
	lastRedraw time.Time
	status     string // overrides the right-side display when set
}

func (b *ProgressBar) Write(p []byte) (int, error) {
	n := len(p)
	b.current += int64(n)
	if time.Since(b.lastRedraw) > 100*time.Millisecond {
		b.redraw()
		b.lastRedraw = time.Now()
	}
	return n, nil
}

func (b *ProgressBar) redraw() {
	var frac float64
	if b.total > 0 {
		frac = float64(b.current) / float64(b.total)
	}
	done := int(frac * barWidth)
	bar := strings.Repeat("=", done) + strings.Repeat(" ", barWidth-done)
	if done > 0 && done < barWidth {
		bar = bar[:done-1] + ">" + bar[done:]
	}

	right := ""
	if b.status != "" {
		right = b.status
	} else if b.total <= 0 {
		elapsed := time.Since(b.start).Truncate(time.Second)
		speed := float64(b.current) / time.Since(b.start).Seconds()
		right = fmt.Sprintf("%s / ??  %s  %s", FormatSize(float64(b.current)), FormatSize(speed)+"/s", elapsed)
	} else {
		elapsed := time.Since(b.start).Truncate(time.Second)
		speed := float64(b.current) / time.Since(b.start).Seconds()
		right = fmt.Sprintf("%s / %s  %s  %s",
			FormatSize(float64(b.current)), FormatSize(float64(b.total)), FormatSize(speed)+"/s", elapsed)
	}

	fmt.Fprintf(os.Stderr, "\r%s |%s| %s\033[K", b.label, bar, right)
}

// SetTotal updates the total size (called when Content-Length becomes known).
func (b *ProgressBar) SetTotal(n int64) {
	b.total = n
}

// SetStatus overrides the right-side display text (e.g. "retry 1/3").
func (b *ProgressBar) SetStatus(s string) {
	b.status = s
	b.redraw()
}

// Done clears the progress bar and prints the final "done" line.
func (b *ProgressBar) Done() {
	bar := strings.Repeat("=", barWidth)
	fmt.Fprintf(os.Stderr, "\r%s |%s| done\033[K\n", b.label, bar)
}

// Canceled keeps the bar as-is and prints "canceled" at the end.
func (b *ProgressBar) Canceled() {
	fmt.Fprintf(os.Stderr, "\r%s |%s| canceled\033[K\n", b.label, b.currentBar())
}

// Failed keeps the bar as-is and prints "failed" at the end (with newline).
func (b *ProgressBar) Failed() {
	fmt.Fprintf(os.Stderr, "\r%s |%s| failed\033[K\n", b.label, b.currentBar())
}

func (b *ProgressBar) currentBar() string {
	if b.total <= 0 {
		return strings.Repeat(" ", barWidth)
	}
	done := int(float64(b.current) / float64(b.total) * barWidth)
	if done > barWidth {
		done = barWidth
	}
	s := strings.Repeat("=", done)
	if done < barWidth {
		s += ">"
		done++
	}
	return s + strings.Repeat(" ", barWidth-done)
}

// FormatSize formats a bytes-per-second rate as a human-readable string.
func FormatSize(bytesPerSec float64) string {
	switch {
	case bytesPerSec >= 1<<30:
		return fmt.Sprintf("%.1f GiB", bytesPerSec/(1<<30))
	case bytesPerSec >= 1<<20:
		return fmt.Sprintf("%.1f MiB", bytesPerSec/(1<<20))
	case bytesPerSec >= 1<<10:
		return fmt.Sprintf("%.1f KiB", bytesPerSec/(1<<10))
	default:
		return fmt.Sprintf("%.0f B", bytesPerSec)
	}
}

// Ensure ProgressBar implements io.Writer.
var _ io.Writer = (*ProgressBar)(nil)
