package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/dustin/go-humanize"
	"github.com/fatih/color"
	"github.com/mattn/go-isatty"
	"github.com/mattn/go-runewidth"
	"golang.org/x/term"
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
func Success(format string, args ...interface{}) {
	if !Quiet {
		fmt.Printf("%s %s\n", Green("✓"), fmt.Sprintf(format, args...))
	}
}

// Info prints an informational message (respects --quiet)
func Info(format string, args ...interface{}) {
	if !Quiet {
		fmt.Printf(format+"\n", args...)
	}
}

// Infof prints formatted info without newline (respects --quiet)
func Infof(format string, args ...interface{}) {
	if !Quiet {
		fmt.Printf(format, args...)
	}
}

// Warn prints a warning to stderr (respects --quiet)
func Warn(format string, args ...interface{}) {
	if !Quiet {
		fmt.Fprintf(os.Stderr, "%s %s\n", Yellow("!"), fmt.Sprintf(format, args...))
	}
}

// Debug prints debug info when verbose mode is enabled
func Debug(format string, args ...interface{}) {
	if Verbose {
		fmt.Fprintf(os.Stderr, "%s %s\n", Faint("[debug]"), fmt.Sprintf(format, args...))
	}
}

// IsTerminal returns true if stdout is a terminal
func IsTerminal() bool {
	return isatty.IsTerminal(os.Stdout.Fd())
}

// IsStdinTerminal returns true if stdin is a terminal
func IsStdinTerminal() bool {
	return isatty.IsTerminal(os.Stdin.Fd())
}

// TerminalSize returns terminal width and height (defaults: 80x24)
func TerminalSize() (int, int) {
	if !IsTerminal() {
		return 120, 40
	}
	width, height, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || width <= 0 {
		return 80, 24
	}
	return width, height
}

// TerminalWidth returns the terminal width, or 80 as default
func TerminalWidth() int {
	w, _ := TerminalSize()
	return w
}

// StatusIcon returns a colored status icon
func StatusIcon(status, state string) string {
	if state == "running" {
		return Yellow("●")
	}
	if state == "queued" {
		return Faint("◦")
	}

	switch strings.ToUpper(status) {
	case "SUCCESS":
		return Green("✓")
	case "FAILURE", "ERROR":
		return Red("✗")
	case "UNKNOWN":
		return Yellow("?")
	default:
		return Faint("○")
	}
}

// StatusText returns colored status text
func StatusText(status, state string) string {
	if state == "running" {
		return Yellow("Running")
	}
	if state == "queued" {
		return Faint("Queued")
	}

	switch strings.ToUpper(status) {
	case "SUCCESS":
		return Green("Success")
	case "FAILURE":
		return Red("Failed")
	case "ERROR":
		return Red("Error")
	default:
		return status
	}
}

var shortTimeMagnitudes = []humanize.RelTimeMagnitude{
	{D: time.Minute, Format: "now", DivBy: time.Second},
	{D: 2 * time.Minute, Format: "1m ago", DivBy: 1},
	{D: time.Hour, Format: "%dm ago", DivBy: time.Minute},
	{D: 2 * time.Hour, Format: "1h ago", DivBy: 1},
	{D: 24 * time.Hour, Format: "%dh ago", DivBy: time.Hour},
	{D: 2 * 24 * time.Hour, Format: "1d ago", DivBy: 1},
	{D: 7 * 24 * time.Hour, Format: "%dd ago", DivBy: 24 * time.Hour},
}

// RelativeTime formats a time as relative to now
func RelativeTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}

	now := time.Now()
	if now.Sub(t) < 0 {
		return "now"
	}

	if now.Sub(t) >= 7*24*time.Hour {
		return t.Format("Jan 02")
	}

	return humanize.CustomRelTime(t, now, "", "", shortTimeMagnitudes)
}

// FormatDuration formats a duration in human-readable form
func FormatDuration(d time.Duration) string {
	if d < 0 {
		return "-"
	}

	if d < time.Second {
		return "< 1s"
	}

	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}

	if d < time.Hour {
		mins := int(d.Minutes())
		secs := int(d.Seconds()) % 60
		return fmt.Sprintf("%dm %ds", mins, secs)
	}

	hours := int(d.Hours())
	mins := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh %dm", hours, mins)
}

// PrintTable prints a formatted table with proper Unicode/ANSI handling
func PrintTable(headers []string, rows [][]string) {
	noBorder := lipgloss.Border{}
	headerStyle := lipgloss.NewStyle().Faint(true)
	cellStyle := lipgloss.NewStyle()

	t := table.New().
		Headers(headers...).
		Rows(rows...).
		Border(noBorder).
		BorderColumn(false).
		BorderRow(false).
		BorderHeader(false).
		StyleFunc(func(row, col int) lipgloss.Style {
			// Last column doesn't need right padding
			padding := 2
			if col == len(headers)-1 {
				padding = 0
			}
			if row == table.HeaderRow {
				return headerStyle.PaddingRight(padding)
			}
			return cellStyle.PaddingRight(padding)
		})

	output := strings.TrimSpace(t.Render())
	fmt.Println(output)
}

// PrintJSON prints data as JSON
func PrintJSON(data interface{}) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// ColumnWidths calculates column widths for a table based on terminal width.
// margin is the space reserved for non-flexible content (padding, fixed columns).
// minFlex is the minimum flexible space to use.
// percentages defines how to divide the flexible space among columns.
// Returns the calculated widths for each column.
func ColumnWidths(margin, minFlex int, percentages ...int) []int {
	termWidth := TerminalWidth()
	flexSpace := termWidth - margin
	if flexSpace < minFlex {
		flexSpace = minFlex
	}

	widths := make([]int, len(percentages))
	for i, pct := range percentages {
		widths[i] = flexSpace * pct / 100
	}
	return widths
}

// Truncate truncates a string to maxLen display width, adding "..." if truncated
// Properly handles unicode and wide characters
func Truncate(s string, maxLen int) string {
	if runewidth.StringWidth(s) <= maxLen {
		return s
	}
	return runewidth.Truncate(s, maxLen, "...")
}

// PrintPlainTable prints tab-separated output for scripting (works with cut -f, awk)
func PrintPlainTable(headers []string, rows [][]string, noHeader bool) {
	colWidths := make([]int, len(headers))
	for i, h := range headers {
		colWidths[i] = runewidth.StringWidth(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(colWidths) {
				if w := runewidth.StringWidth(cell); w > colWidths[i] {
					colWidths[i] = w
				}
			}
		}
	}

	padRow := func(cells []string) string {
		padded := make([]string, len(cells))
		for i, cell := range cells {
			if i < len(colWidths) {
				padded[i] = runewidth.FillRight(cell, colWidths[i])
			} else {
				padded[i] = cell
			}
		}
		return strings.Join(padded, "\t")
	}

	if !noHeader {
		fmt.Println(padRow(headers))
	}

	for _, row := range rows {
		fmt.Println(padRow(row))
	}
}

// PlainStatusIcon returns a plain text status icon (for --plain output)
func PlainStatusIcon(status, state string) string {
	if state == "running" {
		return "*"
	}
	if state == "queued" {
		return "o"
	}

	switch strings.ToUpper(status) {
	case "SUCCESS":
		return "+"
	case "FAILURE", "ERROR":
		return "x"
	case "UNKNOWN":
		return "?"
	default:
		return "-"
	}
}

// PlainStatusText returns plain status text (for --plain output)
func PlainStatusText(status, state string) string {
	if state == "running" {
		return "running"
	}
	if state == "queued" {
		return "queued"
	}
	return strings.ToLower(status)
}

// WithPager pipes output through less if it exceeds terminal height
func WithPager(fn func(w io.Writer)) {
	var buf bytes.Buffer
	fn(&buf)

	_, height := TerminalSize()
	lineCount := bytes.Count(buf.Bytes(), []byte{'\n'})
	lessPath, err := exec.LookPath("less")

	if !IsTerminal() || err != nil || lineCount <= height-2 {
		os.Stdout.Write(buf.Bytes())
		return
	}

	pager := exec.Command(lessPath, "-FIRX", "--mouse", "--incsearch")
	pager.Stdin = &buf
	pager.Stdout = os.Stdout
	pager.Stderr = os.Stderr
	pager.Run()
}
