package output

import (
	"fmt"
	"os"
	"regexp"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"golang.org/x/term"
)

// NoColor disables ANSI styling when true. Factory.InitOutput writes it from
// NO_COLOR / TEAMCITY_NO_COLOR / FORCE_COLOR / --no-color / TTY detection;
// tests flip it directly for deterministic golden output.
var NoColor bool

// ansiRenderer emits 16-color ANSI SGR sequences regardless of the detected
// terminal profile, so output bytes are stable across TTY / piped / CI contexts.
var ansiRenderer = lipgloss.NewRenderer(os.Stdout)

func init() {
	ansiRenderer.SetColorProfile(termenv.ANSI)
	NoColor = os.Getenv("NO_COLOR") != "" ||
		os.Getenv("TERM") == "dumb" ||
		!term.IsTerminal(int(os.Stdout.Fd()))
}

func wrap(s lipgloss.Style) func(a ...any) string {
	return func(a ...any) string {
		str := fmt.Sprint(a...)
		if NoColor {
			return str
		}
		return s.Render(str)
	}
}

var (
	Green  = wrap(ansiRenderer.NewStyle().Foreground(lipgloss.Color("2")))
	Red    = wrap(ansiRenderer.NewStyle().Foreground(lipgloss.Color("1")))
	Yellow = wrap(ansiRenderer.NewStyle().Foreground(lipgloss.Color("3")))
	Cyan   = wrap(ansiRenderer.NewStyle().Foreground(lipgloss.Color("6")))
	Bold   = wrap(ansiRenderer.NewStyle().Bold(true))
	Faint  = wrap(ansiRenderer.NewStyle().Faint(true))
)

// TCAnsiRe matches real ESC sequences and TC's space-prefixed ANSI codes (` [33m` instead of `\x1b[33m`).
var TCAnsiRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]| \[[0-9;]*m`)

// RestoreAnsi converts TC's space-prefixed ANSI codes to real terminal escape sequences.
// When NoColor is true (non-TTY or --no-color), all ANSI sequences are stripped instead.
func RestoreAnsi(s string) string {
	if NoColor {
		return TCAnsiRe.ReplaceAllString(s, "")
	}
	return TCAnsiRe.ReplaceAllStringFunc(s, func(match string) string {
		if match[0] == '\x1b' {
			return match
		}
		return "\x1b" + match[1:]
	})
}
