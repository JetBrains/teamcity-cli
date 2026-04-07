package cmdutil

import (
	"io"
	"os"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/fatih/color"
	"github.com/mattn/go-isatty"
)

// IOStreams provides the standard streams for commands to read/write.
// Commands should use these instead of os.Stdin/os.Stdout/os.Stderr directly,
// enabling tests to capture output without redirecting globals.
type IOStreams struct {
	In     io.Reader
	Out    io.Writer
	ErrOut io.Writer
}

// Factory provides shared dependencies to all commands.
// Instead of reaching for package-level globals, commands receive a Factory
// and use its methods/fields to get clients, check flags, etc.
type Factory struct {
	// Global flags — set once by root command, read by subcommands.
	NoColor bool
	Quiet   bool
	Verbose bool
	NoInput bool

	// JSONOutput is set by commands that accept --json to signal that errors
	// should be emitted as structured JSON instead of human-readable text.
	JSONOutput bool

	// IOStreams provides standard I/O handles. Override in tests to capture output.
	IOStreams *IOStreams

	// Printer writes formatted output. Prefer this over the output package
	// free functions — it uses the Factory's IOStreams and respects Quiet/Verbose
	// without relying on package globals.
	Printer *output.Printer

	// ClientFunc returns an API client. Override in tests to inject mocks.
	ClientFunc func() (api.ClientInterface, error)

	// UpdateNotice is called after command execution to print update notices.
	UpdateNotice func()
}

// NewFactory creates a Factory with production defaults.
func NewFactory() *Factory {
	f := &Factory{
		IOStreams: &IOStreams{
			In:     os.Stdin,
			Out:    os.Stdout,
			ErrOut: os.Stderr,
		},
		Printer: output.DefaultPrinter(),
	}
	f.ClientFunc = f.defaultGetClient
	return f
}

// Client returns an API client using the configured ClientFunc.
func (f *Factory) Client() (api.ClientInterface, error) {
	return f.ClientFunc()
}

// InitOutput configures output settings from Factory flags.
// Called once after flags are parsed (in PersistentPreRun).
func (f *Factory) InitOutput() {
	noColor := os.Getenv("NO_COLOR") != "" ||
		os.Getenv("TEAMCITY_NO_COLOR") != "" ||
		os.Getenv("TERM") == "dumb" ||
		f.NoColor ||
		!isatty.IsTerminal(os.Stdout.Fd())

	if noColor {
		color.NoColor = true
	}

	f.Printer.Quiet = f.Quiet
	f.Printer.Verbose = f.Verbose
}

// IsInteractive returns true if the CLI can prompt the user.
func (f *Factory) IsInteractive() bool {
	return !f.NoInput && output.IsStdinTerminal()
}
