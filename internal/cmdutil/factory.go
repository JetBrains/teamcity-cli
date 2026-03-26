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

	// IOStreams provides standard I/O handles. Override in tests to capture output.
	IOStreams *IOStreams

	// ClientFunc returns an API client. Override in tests to inject mocks.
	ClientFunc func() (api.ClientInterface, error)
}

// NewFactory creates a Factory with production defaults.
func NewFactory() *Factory {
	return &Factory{
		IOStreams: &IOStreams{
			In:     os.Stdin,
			Out:    os.Stdout,
			ErrOut: os.Stderr,
		},
		ClientFunc: defaultGetClient,
	}
}

// Client returns an API client using the configured ClientFunc.
func (f *Factory) Client() (api.ClientInterface, error) {
	return f.ClientFunc()
}

// InitOutput synchronizes the output package state from Factory flags.
// Called once after flags are parsed (in PersistentPreRun).
func (f *Factory) InitOutput() {
	output.Quiet = f.Quiet
	output.Verbose = f.Verbose

	if os.Getenv("NO_COLOR") != "" ||
		os.Getenv("TERM") == "dumb" ||
		f.NoColor ||
		!isatty.IsTerminal(os.Stdout.Fd()) {
		color.NoColor = true
	}
}

// IsInteractive returns true if the CLI can prompt the user.
func (f *Factory) IsInteractive() bool {
	return !f.NoInput && output.IsStdinTerminal()
}
