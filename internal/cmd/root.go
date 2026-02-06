package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/fatih/color"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

var (
	Version = "dev"

	NoColor bool
	Quiet   bool
	Verbose bool
	NoInput bool
)

var rootCmd = &cobra.Command{
	Use:   "tc",
	Short: "TeamCity CLI",
	Long: "TeamCity CLI v" + Version + `

A command-line interface for interacting with TeamCity CI/CD server.

tc provides a complete experience for managing
TeamCity runs, jobs, projects and more from the command line.

Documentation:  https://jb.gg/tc/docs
Report issues:  https://jb.gg/tc/issues`,
	Version: Version,
	Run: func(cmd *cobra.Command, args []string) {
		output.PrintLogo()
		fmt.Println()
		fmt.Println("TeamCity CLI " + output.Faint("v"+Version) + " - " + output.Faint("https://jb.gg/tc/docs"))
		fmt.Println()
		fmt.Println("Usage: tc <command> [flags]")
		fmt.Println()
		fmt.Println("Common commands:")
		fmt.Println("  auth login              Authenticate with TeamCity")
		fmt.Println("  run list                List recent runs")
		fmt.Println("  run trigger <job>       Trigger a new run")
		fmt.Println("  run view <id>           View run details")
		fmt.Println("  job list                List jobs")
		fmt.Println()
		fmt.Println(output.Faint("Run 'tc --help' for full command list, or 'tc <command> --help' for details"))
	},
}

func init() {
	rootCmd.SetVersionTemplate("tc version {{.Version}}\n")
	rootCmd.SuggestionsMinimumDistance = 2

	rootCmd.PersistentFlags().BoolVar(&NoColor, "no-color", false, "Disable colored output")
	rootCmd.PersistentFlags().BoolVarP(&Quiet, "quiet", "q", false, "Suppress non-essential output")
	rootCmd.PersistentFlags().BoolVar(&Verbose, "verbose", false, "Show detailed output including debug info")
	rootCmd.PersistentFlags().BoolVar(&NoInput, "no-input", false, "Disable interactive prompts")

	rootCmd.MarkFlagsMutuallyExclusive("quiet", "verbose")

	cobra.OnInitialize(initColorSettings)

	rootCmd.AddCommand(newAuthCmd())
	rootCmd.AddCommand(newProjectCmd())
	rootCmd.AddCommand(newJobCmd())
	rootCmd.AddCommand(newRunCmd())
	rootCmd.AddCommand(newQueueCmd())
	rootCmd.AddCommand(newAgentCmd())
	rootCmd.AddCommand(newPoolCmd())
	rootCmd.AddCommand(newAPICmd())
}

func initColorSettings() {
	output.Quiet = Quiet
	output.Verbose = Verbose

	if os.Getenv("NO_COLOR") != "" ||
		os.Getenv("TERM") == "dumb" ||
		NoColor ||
		!isatty.IsTerminal(os.Stdout.Fd()) {
		color.NoColor = true
	}
}

func Execute() error {
	rootCmd.SilenceErrors = true
	err := rootCmd.Execute()
	if err != nil {
		var exitErr *ExitError
		if !errors.As(err, &exitErr) {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
	}
	return err
}

// subcommandRequired is a RunE function for parent commands that require a subcommand.
// It returns an error when no valid subcommand is provided.
func subcommandRequired(cmd *cobra.Command, args []string) error {
	return fmt.Errorf("requires a subcommand\n\nRun '%s --help' for available commands", cmd.CommandPath())
}

// RootCommand is an alias for cobra.Command for external access
type RootCommand = cobra.Command

// GetRootCmd returns the root command for testing
func GetRootCmd() *RootCommand {
	return rootCmd
}

// NewRootCmd creates a fresh root command instance for testing.
// This ensures tests don't share flag state from previous test runs.
func NewRootCmd() *RootCommand {
	cmd := &cobra.Command{
		Use:     "tc",
		Short:   "TeamCity CLI",
		Version: Version,
	}

	cmd.PersistentFlags().BoolVar(&NoColor, "no-color", false, "Disable colored output")
	cmd.PersistentFlags().BoolVarP(&Quiet, "quiet", "q", false, "Suppress non-essential output")
	cmd.PersistentFlags().BoolVar(&Verbose, "verbose", false, "Show detailed output including debug info")
	cmd.PersistentFlags().BoolVar(&NoInput, "no-input", false, "Disable interactive prompts")

	cmd.AddCommand(newAuthCmd())
	cmd.AddCommand(newProjectCmd())
	cmd.AddCommand(newJobCmd())
	cmd.AddCommand(newRunCmd())
	cmd.AddCommand(newQueueCmd())
	cmd.AddCommand(newAgentCmd())
	cmd.AddCommand(newPoolCmd())
	cmd.AddCommand(newAPICmd())

	return cmd
}
