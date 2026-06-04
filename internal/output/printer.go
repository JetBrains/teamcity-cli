package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
)

// Printer writes formatted output respecting Quiet/Verbose flags.
type Printer struct {
	Out     io.Writer
	ErrOut  io.Writer
	Quiet   bool
	Verbose bool

	mu sync.Mutex
}

// DefaultPrinter returns a Printer writing to os.Stdout/os.Stderr, wrapped so the activity spinner clears on the first byte of output.
func DefaultPrinter() *Printer {
	return &Printer{
		Out:    stopWriter{os.Stdout},
		ErrOut: stopWriter{os.Stderr},
	}
}

// stopWriter halts the activity spinner before its first write; the spinner writes to the raw fd, not through this wrapper, so there is no feedback loop.
type stopWriter struct{ w io.Writer }

func (s stopWriter) Write(b []byte) (int, error) {
	StopSpinner()
	return s.w.Write(b)
}

// write atomically emits s to w, serializing concurrent calls across all Printer methods.
func (p *Printer) write(w io.Writer, s string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	_, _ = io.WriteString(w, s)
}

func (p *Printer) Success(format string, args ...any) {
	if p.Quiet {
		return
	}
	p.write(p.Out, fmt.Sprintf("%s %s\n", Green(Sym().Check), fmt.Sprintf(format, args...)))
}

func (p *Printer) Info(format string, args ...any) {
	if p.Quiet {
		return
	}
	p.write(p.Out, fmt.Sprintf(format+"\n", args...))
}

// Empty prints an empty-state message with an optional next-step tip.
func (p *Printer) Empty(message, tip string) {
	if p.Quiet {
		return
	}
	var buf bytes.Buffer
	fmt.Fprintln(&buf, message)
	if tip != "" {
		fmt.Fprintf(&buf, "\n%s\n", FormatTip(tip))
	}
	p.write(p.Out, buf.String())
}

// Tip prints a "Tip: <text>" line for next-step guidance on non-error events.
func (p *Printer) Tip(format string, args ...any) {
	if p.Quiet {
		return
	}
	p.write(p.Out, FormatTip(fmt.Sprintf(format, args...))+"\n")
}

// Progress writes an inline progress line to stdout (no newline). Suppressed by --quiet.
func (p *Printer) Progress(format string, args ...any) {
	if p.Quiet {
		return
	}
	p.write(p.Out, fmt.Sprintf(format, args...))
}

func (p *Printer) Warn(format string, args ...any) {
	if p.Quiet {
		return
	}
	p.write(p.ErrOut, fmt.Sprintf("%s %s\n", Yellow("!"), fmt.Sprintf(format, args...)))
}

func (p *Printer) Debug(format string, args ...any) {
	if !p.Verbose {
		return
	}
	p.write(p.ErrOut, fmt.Sprintf("%s %s\n", Faint("[debug]"), fmt.Sprintf(format, args...)))
}

func (p *Printer) PrintJSON(data any) error {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		return err
	}
	p.write(p.Out, buf.String())
	return nil
}

func (p *Printer) PrintField(label, value string) {
	p.write(p.Out, fmt.Sprintf("%s: %s\n", label, value))
}

func (p *Printer) PrintViewHeader(title, webURL string, details func()) {
	p.write(p.Out, Cyan(title)+"\n")
	details()
	p.write(p.Out, fmt.Sprintf("\n%s %s\n", Faint("View in browser:"), Green(webURL)))
}

func (p *Printer) PrintTable(headers []string, rows [][]string) {
	p.write(p.Out, renderTable(headers, rows)+"\n")
}

func (p *Printer) PrintPlainTable(headers []string, rows [][]string, noHeader bool) {
	p.write(p.Out, renderPlainTable(headers, rows, noHeader))
}

func (p *Printer) PrintTree(root TreeNode) {
	var buf bytes.Buffer
	fmt.Fprintln(&buf, root.Label)
	writeTreeNodes(&buf, root.Children, "")
	p.write(p.Out, buf.String())
}

func writeTreeNodes(w io.Writer, nodes []TreeNode, prefix string) {
	s := Sym()
	for i, n := range nodes {
		conn, next := s.TreeMid, s.TreePipe
		if i == len(nodes)-1 {
			conn, next = s.TreeEnd, s.TreeGap
		}
		_, _ = fmt.Fprintf(w, "%s%s%s\n", prefix, conn, n.Label)
		writeTreeNodes(w, n.Children, prefix+next)
	}
}
