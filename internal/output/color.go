package output

import (
	"regexp"

	"github.com/fatih/color"
)

var (
	Green  = color.New(color.FgGreen).SprintFunc()
	Red    = color.New(color.FgRed).SprintFunc()
	Yellow = color.New(color.FgYellow).SprintFunc()
	Cyan   = color.New(color.FgCyan).SprintFunc()
	Bold   = color.New(color.Bold).SprintFunc()
	Faint  = color.New(color.Faint).SprintFunc()
)

// TCAnsiRe matches real ESC sequences and TC's space-prefixed ANSI codes (` [33m` instead of `\x1b[33m`).
var TCAnsiRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]| \[[0-9;]*m`)

// RestoreAnsi converts TC's space-prefixed ANSI codes to real terminal escape sequences.
// When color.NoColor is true (non-TTY or --no-color), all ANSI sequences are stripped instead.
func RestoreAnsi(s string) string {
	if color.NoColor {
		return TCAnsiRe.ReplaceAllString(s, "")
	}
	return TCAnsiRe.ReplaceAllStringFunc(s, func(match string) string {
		if match[0] == '\x1b' {
			return match // already real ANSI
		}
		return "\x1b" + match[1:] // replace leading space with ESC
	})
}
