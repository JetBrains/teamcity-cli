package job

import (
	"fmt"

	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/completion"
	"github.com/spf13/cobra"
)

type jobStateAction struct {
	use    string
	short  string
	long   string
	verb   string
	paused bool
}

var jobStateActions = map[string]jobStateAction{
	"pause":  {"pause", "Pause a job", "Pause a job to prevent new runs from being triggered.", "Paused", true},
	"resume": {"resume", "Resume a paused job", "Resume a paused job to allow new runs.", "Resumed", false},
}

func newJobStateCmd(f *cmdutil.Factory, a jobStateAction) *cobra.Command {
	return &cobra.Command{
		Use:               a.use + " [job-id]",
		Short:             a.short,
		Long:              a.long + " With no argument, uses the linked default job from teamcity.toml.",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completion.LinkedJobs(),
		Example: fmt.Sprintf(`  teamcity job %s Falcon_Build
  teamcity job %s                # uses linked default job (see 'teamcity link')`, a.use, a.use),
		RunE: func(cmd *cobra.Command, args []string) error {
			jobID, _, err := cmdutil.ResolveOwnerID("job", args, 0, f.ResolveDefaultJob)
			if err != nil {
				return err
			}
			client, err := f.Client()
			if err != nil {
				return err
			}
			if err := client.SetBuildTypePaused(jobID, a.paused); err != nil {
				return fmt.Errorf("failed to %s job: %w", a.use, err)
			}
			f.Printer.Success("%s job %s", a.verb, jobID)
			return nil
		},
	}
}

func newJobPauseCmd(f *cmdutil.Factory) *cobra.Command {
	return newJobStateCmd(f, jobStateActions["pause"])
}
func newJobResumeCmd(f *cmdutil.Factory) *cobra.Command {
	return newJobStateCmd(f, jobStateActions["resume"])
}
