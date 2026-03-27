package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// Printer writes formatted output to configurable writers.
// Commands should use a Printer instead of the package-level
// functions to enable proper testing and parallel execution.
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
