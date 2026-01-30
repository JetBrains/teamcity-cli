//go:build windows

package api

import "os"

// resizeSignal returns a channel that receives signals when the terminal is resized.
// On Windows, terminal resize is handled differently (not via signals), so this
// returns a channel that never receives.
func resizeSignal() (chan os.Signal, func()) {
	return make(chan os.Signal), func() {}
}
