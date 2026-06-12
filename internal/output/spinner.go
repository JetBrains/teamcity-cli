package output

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

var spinnerFrames = []rune("⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏")

// spin is the process-wide activity spinner; one CLI run drives one terminal.
var spin spinner

type spinner struct {
	mu     sync.Mutex
	active bool
	stop   chan struct{}
	done   chan struct{}
}

// StartSpinner shows a "Loading..." indicator on stdout until StopSpinner or the first byte of output; no-op when quiet, non-terminal, dumb-terminal, or already running, and the first frame waits one 80ms tick so fast work shows nothing.
func StartSpinner(quiet bool) {
	if quiet || os.Getenv("TERM") == "dumb" || !IsTerminal() {
		return
	}
	spin.mu.Lock()
	defer spin.mu.Unlock()
	if spin.active {
		return
	}
	spin.active = true
	spin.stop = make(chan struct{})
	spin.done = make(chan struct{})
	stop, done := spin.stop, spin.done

	frames := spinnerFrames
	if ASCII {
		frames = []rune(`|/-\`)
	}
	go func() {
		defer close(done)
		t := time.NewTicker(80 * time.Millisecond)
		defer t.Stop()
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			case <-t.C:
				_, _ = fmt.Fprintf(os.Stdout, "\r%s Loading...", Faint(string(frames[i%len(frames)])))
			}
		}
	}()
}

// StopSpinner halts the spinner and clears its line; safe to call repeatedly and when nothing is running, so it can guard pagers and raw-terminal takeovers.
func StopSpinner() {
	spin.mu.Lock()
	if !spin.active {
		spin.mu.Unlock()
		return
	}
	spin.active = false
	close(spin.stop)
	done := spin.done
	spin.mu.Unlock()

	<-done
	ClearLine(os.Stdout)
}

// ClearLine returns the cursor to column 0 and erases the line, wiping a transient spinner or progress line before final output.
func ClearLine(w io.Writer) {
	_, _ = fmt.Fprint(w, "\r\033[K")
}

// ProgressWriter wraps an io.Writer and renders a determinate "name  size  NN%" bar to out as bytes flow through, throttled to ~10 fps; callers gate it on output.IsTerminal() and a known total.
type ProgressWriter struct {
	w          io.Writer
	out        io.Writer
	name       string
	size       string
	total      int64
	written    int64
	nameWidth  int
	lastUpdate time.Time
}

// NewProgressWriter copies to w while drawing progress for name/total to out; size is the human-readable total shown in the bar.
func NewProgressWriter(w, out io.Writer, name, size string, total int64, nameWidth int) *ProgressWriter {
	return &ProgressWriter{w: w, out: out, name: name, size: size, total: total, nameWidth: nameWidth}
}

func (p *ProgressWriter) Write(b []byte) (int, error) {
	n, err := p.w.Write(b)
	p.written += int64(n)

	now := time.Now()
	if now.Sub(p.lastUpdate) >= 100*time.Millisecond {
		p.lastUpdate = now
		pct := int(float64(p.written) / float64(p.total) * 100)
		_, _ = fmt.Fprintf(p.out, "\r%-*s  %10s  %3d%%", p.nameWidth, p.name, p.size, pct)
	}
	return n, err
}

// Clear erases the progress line.
func (p *ProgressWriter) Clear() { ClearLine(p.out) }
