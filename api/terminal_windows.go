//go:build windows

package api

import (
	"os"
	"time"

	"github.com/JetBrains/teamcity-cli/internal/output"
)

// resizeSignal returns a channel that receives when the terminal is resized.
// On Windows, there's no SIGWINCH, so we poll for size changes every 500ms.
func resizeSignal() (chan os.Signal, func()) {
	ch := make(chan os.Signal, 1)
	done := make(chan struct{})

	go func() {
		cols, rows := output.TerminalSize()
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				newCols, newRows := output.TerminalSize()
				if newCols != cols || newRows != rows {
					cols, rows = newCols, newRows
					select {
					case ch <- nil: // signal resize
					default:
					}
				}
			}
		}
	}()

	return ch, func() { close(done) }
}
