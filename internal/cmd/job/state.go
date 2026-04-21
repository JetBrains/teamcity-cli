package job

import (
	"fmt"

	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
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
		Use:     a.use + " <job-id>",
		Short:   a.short,
		Long:    a.long,
		Args:    cobra.ExactArgs(1),
		Example: fmt.Sprintf("  teamcity job %s Falcon_Build", a.use),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.Client()
			if err != nil {
				return err
			}
			if err := client.SetBuildTypePaused(args[0], a.paused); err != nil {
				return fmt.Errorf("failed to %s job: %w", a.use, err)
			}
			f.Printer.Success("%s job %s", a.verb, args[0])
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
