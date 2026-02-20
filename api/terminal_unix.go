//go:build unix

package api

import (
	"os"
	"os/signal"
	"syscall"
)

// resizeSignal returns a channel that receives signals when the terminal is resized.
func resizeSignal() (chan os.Signal, func()) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGWINCH)
	return sigChan, func() { signal.Stop(sigChan) }
}
