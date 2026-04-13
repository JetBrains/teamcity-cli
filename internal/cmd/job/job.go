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
		Long:    `List and manage TeamCity jobs (build configurations).`,
		Args:    cobra.NoArgs,
		RunE:    cmdutil.SubcommandRequired,
	}

	cmd.AddCommand(newJobListCmd(f))
	cmd.AddCommand(newJobViewCmd(f))
	cmd.AddCommand(newJobTreeCmd(f))
	cmd.AddCommand(newJobPauseCmd(f))
	cmd.AddCommand(newJobResumeCmd(f))
	cmd.AddCommand(param.NewCmd(f, "job", param.JobParamAPI))

	return cmd
}
