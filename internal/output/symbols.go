package output

import (
	"os"
	"runtime"
)

// Status symbols used throughout the CLI output.
// On Windows with legacy console (cmd.exe, PowerShell without Windows Terminal),
// Unicode glyphs like ✓/✗ may render as □ because console fonts (Consolas,
// Lucida Console) lack those code-points. Windows Terminal sets the WT_SESSION
// environment variable and ships with Cascadia Code which has full Unicode
// support, so we only fall back to ASCII on legacy consoles.
var (
	// Success is rendered after a completed check (green).
	Success = "✓"
	// Failure is rendered after a failed check (red).
	Failure = "✗"
	// Arrow is rendered as a directional hint (yellow).
	Arrow = "→"
	// ArrowLeft is rendered as a left-directional hint.
	ArrowLeft = "←"
	// RunningIcon indicates a running build (yellow).
	RunningIcon = "●"
	// QueuedIcon indicates a queued build (faint).
	QueuedIcon = "◦"
	// CanceledIcon indicates a canceled build (faint).
	CanceledIcon = "⊘"
	// DefaultIcon is the fallback status icon (faint).
	DefaultIcon = "○"
	// Bullet is used for list items.
	Bullet = "•"
)

func init() {
	if runtime.GOOS == "windows" && os.Getenv("WT_SESSION") == "" {
		Success = "[ok]"
		Failure = "[x]"
		Arrow = "->"
		ArrowLeft = "<-"
		RunningIcon = "*"
		QueuedIcon = "o"
		CanceledIcon = "/"
		DefaultIcon = "-"
		Bullet = "*"
	}
}
