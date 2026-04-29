package pipeline

import (
	"fmt"

	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/completion"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
)

func newPipelineViewCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &cmdutil.ViewOptions{}

	cmd := &cobra.Command{
		Use:               "view <pipeline-id>",
		Short:             "View pipeline details",
		Aliases:           []string{"show"},
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completion.LinkedJobs(),
		Example: `  teamcity pipeline view CLI_CiCd
  teamcity pipeline view CLI_CiCd --web
  teamcity pipeline view CLI_CiCd --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPipelineView(f, args[0], opts)
		},
	}

	cmdutil.AddViewFlags(cmd, opts)
	return cmd
}

func runPipelineView(f *cmdutil.Factory, id string, opts *cmdutil.ViewOptions) error {
	client, err := f.Client()
	if err != nil {
		return err
	}

	pipeline, err := client.GetPipeline(id)
	if err != nil {
		return err
	}

	if opts.Web {
		if pipeline.WebURL == "" {
			return fmt.Errorf("no web URL available for pipeline %s", id)
		}
		return browser.OpenURL(pipeline.WebURL)
	}

	if opts.JSON {
		return f.Printer.PrintJSON(pipeline)
	}

	p := f.Printer
	_, _ = fmt.Fprintf(p.Out, "%s\n", output.Cyan(pipeline.Name))
	p.PrintField("ID", pipeline.ID)

	if pipeline.ParentProject != nil {
		p.PrintField("Project", pipeline.ParentProject.Name+" ("+pipeline.ParentProject.ID+")")
	}

	if pipeline.HeadBuildType != nil {
		p.PrintField("Head Job", pipeline.HeadBuildType.ID)
	}

	if pipeline.Jobs != nil && len(pipeline.Jobs.Job) > 0 {
		maxIDLen := 0
		for _, j := range pipeline.Jobs.Job {
			if len(j.ID) > maxIDLen {
				maxIDLen = len(j.ID)
			}
		}
		_, _ = fmt.Fprintf(p.Out, "\n  %s (%d):\n", output.Cyan("Jobs"), len(pipeline.Jobs.Job))
		for _, j := range pipeline.Jobs.Job {
			padded := fmt.Sprintf("%-*s", maxIDLen+2, j.ID)
			_, _ = fmt.Fprintf(p.Out, "    %s %s\n", output.Faint(padded), j.Name)
		}
	}

	if pipeline.WebURL != "" {
		_, _ = fmt.Fprintf(p.Out, "\n%s %s\n", output.Faint("View in browser:"), output.Green(pipeline.WebURL))
	}

	return nil
}
