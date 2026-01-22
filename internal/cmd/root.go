package cmd

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
	"github.com/tiulpin/teamcity-cli/internal/output"
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
	Long: `A command-line interface for interacting with TeamCity CI/CD server.

tc provides a complete experience for managing
TeamCity runs, jobs, projects and more from the command line.

Documentation: https://github.com/tiulpin/teamcity-cli
Report issues:  https://github.com/tiulpin/teamcity-cli/issues`,
	Version: Version,
	Run: func(cmd *cobra.Command, args []string) {
		logo := `
████████╗ ██████╗
╚══██╔══╝██╔════╝
   ██║   ██║
   ██║   ██║
   ██║   ╚██████╗
   ╚═╝    ╚═════╝`
		fmt.Println(output.Cyan(logo))
		fmt.Println()
		fmt.Println("TeamCity CLI - " + output.Faint("https://github.com/tiulpin/teamcity-cli"))
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

	cobra.OnInitialize(initColorSettings)

	rootCmd.AddCommand(newAuthCmd())
	rootCmd.AddCommand(newProjectCmd())
	rootCmd.AddCommand(newJobCmd())
	rootCmd.AddCommand(newRunCmd())
	rootCmd.AddCommand(newQueueCmd())
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
	return rootCmd.Execute()
}

// RootCommand is an alias for cobra.Command for external access
type RootCommand = cobra.Command

// GetRootCmd returns the root command for testing
func GetRootCmd() *RootCommand {
	return rootCmd
}
