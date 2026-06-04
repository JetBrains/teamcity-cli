// Package test implements the `teamcity test` noun for cross-build test operations.
package test

import (
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/spf13/cobra"
)

func NewCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Inspect and manage tests across builds",
		Long: `Query and manage tests as entities with their own lifecycle, across builds.

Where 'teamcity run tests' answers "what broke in this build?", these commands
answer "is this test reliable?" — list currently failing/muted/investigated
tests in a project or job, inspect a test's pass/fail history, and manage mutes
and investigations.

See: https://www.jetbrains.com/help/teamcity/investigating-and-muting-build-failures.html`,
		Args: cobra.NoArgs,
		RunE: cmdutil.SubcommandRequired,
	}

	cmd.AddCommand(newListCmd(f))

	return cmd
}
