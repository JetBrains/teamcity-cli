package output

import (
	"fmt"
	"os"

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

// Quiet suppresses non-essential output when true
var Quiet bool

// Verbose enables debug output when true
var Verbose bool

// Success prints a success message with green checkmark (respects --quiet)
func Success(format string, args ...any) {
	if !Quiet {
		fmt.Printf("%s %s\n", Green("✓"), fmt.Sprintf(format, args...))
	}
}

// Info prints an informational message (respects --quiet)
func Info(format string, args ...any) {
	if !Quiet {
		fmt.Printf(format+"\n", args...)
	}
}

// Infof prints formatted info without newline (respects --quiet)
func Infof(format string, args ...any) {
	if !Quiet {
		fmt.Printf(format, args...)
	}
}

// Warn prints a warning to stderr (respects --quiet)
func Warn(format string, args ...any) {
	if !Quiet {
		fmt.Fprintf(os.Stderr, "%s %s\n", Yellow("!"), fmt.Sprintf(format, args...))
	}
}

// Debug prints debug info when verbose mode is enabled
func Debug(format string, args ...any) {
	if Verbose {
		fmt.Fprintf(os.Stderr, "%s %s\n", Faint("[debug]"), fmt.Sprintf(format, args...))
	}
}

// PrintField prints a labeled field (e.g. "ID: value").
func PrintField(label, value string) {
	fmt.Printf("%s: %s\n", label, value)
}

// PrintViewHeader prints a view command header with a title, detail block, and browser link.
func PrintViewHeader(title, webURL string, details func()) {
	fmt.Printf("%s\n", Cyan(title))
	details()
	fmt.Printf("\n%s %s\n", Faint("View in browser:"), Green(webURL))
}
