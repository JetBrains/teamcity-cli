package job

import (
	"github.com/JetBrains/teamcity-cli/internal/cmd/param"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/spf13/cobra"
)

func NewCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "job",
		Aliases: []string{"buildtype"},
		Short:   "Manage jobs (build configurations)",
		Long: `List and manage TeamCity jobs (build configurations).

A job (referred to as a build configuration in the TeamCity UI) is the
recipe that turns source revisions into builds: VCS roots, build steps,
parameters, and triggers. Use these commands to browse jobs, inspect
their parameters and dependencies, and pause or resume them.

See: https://www.jetbrains.com/help/teamcity/creating-and-editing-build-configurations.html`,
		Args: cobra.NoArgs,
		RunE: cmdutil.SubcommandRequired,
	}

	cmd.AddCommand(newJobListCmd(f))
	cmd.AddCommand(newJobViewCmd(f))
	cmd.AddCommand(newJobTreeCmd(f))
	cmd.AddCommand(newJobPauseCmd(f))
	cmd.AddCommand(newJobResumeCmd(f))
	cmd.AddCommand(param.NewCmd(f, "job", param.JobParamAPI, f.ResolveDefaultJob))

	cmdutil.AliasAwareHelp(cmd, "", "")
	return cmd
}
