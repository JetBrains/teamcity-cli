package output

import "runtime"

// Status symbols used throughout the CLI output.
// On Windows (non-Terminal), Unicode glyphs like ✓/✗ may render as □ because
// legacy console fonts (Consolas, Lucida Console) lack those code-points.
// ASCII fallbacks keep the output readable everywhere.
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
	if runtime.GOOS == "windows" {
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
