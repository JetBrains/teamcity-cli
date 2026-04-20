package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// Printer writes formatted output respecting Quiet/Verbose flags.
type Printer struct {
	Out     io.Writer
	ErrOut  io.Writer
	Quiet   bool
	Verbose bool
}

// DefaultPrinter returns a Printer that writes to os.Stdout/os.Stderr.
func DefaultPrinter() *Printer {
	return &Printer{
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	}
}

func (p *Printer) Success(format string, args ...any) {
	if !p.Quiet {
		_, _ = fmt.Fprintf(p.Out, "%s %s\n", Green("✓"), fmt.Sprintf(format, args...))
	}
}

func (p *Printer) Info(format string, args ...any) {
	if !p.Quiet {
		_, _ = fmt.Fprintf(p.Out, format+"\n", args...)
	}
}

// Empty prints an empty-state message with an optional next-step tip.
func (p *Printer) Empty(message, tip string) {
	if p.Quiet {
		return
	}
	_, _ = fmt.Fprintln(p.Out, message)
	if tip != "" {
		_, _ = fmt.Fprintf(p.Out, "\n%s\n", FormatTip(tip))
	}
}

// Tip prints a "Tip: <text>" line for next-step guidance on non-error events.
func (p *Printer) Tip(format string, args ...any) {
	if p.Quiet {
		return
	}
	_, _ = fmt.Fprintln(p.Out, FormatTip(fmt.Sprintf(format, args...)))
}

func (p *Printer) Infof(format string, args ...any) {
	if !p.Quiet {
		_, _ = fmt.Fprintf(p.Out, format, args...)
	}
}

func (p *Printer) Warn(format string, args ...any) {
	if !p.Quiet {
		_, _ = fmt.Fprintf(p.ErrOut, "%s %s\n", Yellow("!"), fmt.Sprintf(format, args...))
	}
}

func (p *Printer) Debug(format string, args ...any) {
	if p.Verbose {
		_, _ = fmt.Fprintf(p.ErrOut, "%s %s\n", Faint("[debug]"), fmt.Sprintf(format, args...))
	}
}

func (p *Printer) PrintJSON(data any) error {
	encoder := json.NewEncoder(p.Out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

func (p *Printer) PrintField(label, value string) {
	_, _ = fmt.Fprintf(p.Out, "%s: %s\n", label, value)
}

func (p *Printer) PrintViewHeader(title, webURL string, details func()) {
	_, _ = fmt.Fprintf(p.Out, "%s\n", Cyan(title))
	details()
	_, _ = fmt.Fprintf(p.Out, "\n%s %s\n", Faint("View in browser:"), Green(webURL))
}

func (p *Printer) PrintTable(headers []string, rows [][]string) {
	_, _ = fmt.Fprintln(p.Out, renderTable(headers, rows))
}

func (p *Printer) PrintPlainTable(headers []string, rows [][]string, noHeader bool) {
	_, _ = fmt.Fprint(p.Out, renderPlainTable(headers, rows, noHeader))
}

func (p *Printer) PrintTree(root TreeNode) {
	_, _ = fmt.Fprintln(p.Out, root.Label)
	p.printTreeNodes(root.Children, "")
}

func (p *Printer) printTreeNodes(nodes []TreeNode, prefix string) {
	for i, n := range nodes {
		conn, next := "├── ", "│   "
		if i == len(nodes)-1 {
			conn, next = "└── ", "    "
		}
		_, _ = fmt.Fprintf(p.Out, "%s%s%s\n", prefix, conn, n.Label)
		p.printTreeNodes(n.Children, prefix+next)
	}
}
