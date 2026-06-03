package project

import (
	"cmp"
	"fmt"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/completion"
	"github.com/spf13/cobra"
)

type connectionViewOptions struct {
	project string
	cmdutil.ViewOptions
}

func newConnectionViewCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &connectionViewOptions{}

	cmd := &cobra.Command{
		Use:     "view <id>",
		Short:   "View a project connection",
		Aliases: []string{"show"},
		Long: `Show details of a single OAuth/connection feature by id.

The id is the one shown in the first column of 'connection list'.`,
		Example: `  teamcity project connection view PROJECT_EXT_1 -p Backend
  teamcity project connection view PROJECT_EXT_1 -p Backend --json
  teamcity project connection view PROJECT_EXT_1 -p Backend --web`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConnectionView(f, opts, args[0])
		},
	}

	cmd.Flags().StringVarP(&opts.project, "project", "p", "", "Project ID (default: _Root)")
	cmdutil.AddViewFlags(cmd, &opts.ViewOptions)

	_ = cmd.RegisterFlagCompletionFunc("project", completion.LinkedProjects())

	return cmd
}

func runConnectionView(f *cmdutil.Factory, opts *connectionViewOptions, id string) error {
	client, err := f.Client()
	if err != nil {
		return err
	}

	project := cmp.Or(opts.project, "_Root")

	features, err := client.GetProjectConnections(project)
	if err != nil {
		return err
	}

	var feat *api.ProjectFeature
	for i := range features.ProjectFeature {
		if features.ProjectFeature[i].ID == id {
			feat = &features.ProjectFeature[i]
			break
		}
	}
	if feat == nil {
		return api.Validation(
			fmt.Sprintf("connection %s not found in project %s", id, project),
			fmt.Sprintf("Run 'teamcity project connection list --project %s' to see available connections", project),
		)
	}

	url := projectConnectionsURL(client.ServerURL(), project)

	if done, err := opts.EmitWebURL(f.Printer, url); done {
		return err
	}

	if opts.JSON {
		return f.Printer.PrintJSON(connectionToMap(*feat))
	}

	name, providerType := connectionDisplayInfo(*feat)
	f.Printer.PrintViewHeader(name, url, func() {
		f.Printer.PrintField("ID", feat.ID)
		f.Printer.PrintField("Name", name)
		f.Printer.PrintField("Type", providerType)
		if feat.Properties != nil {
			for _, p := range feat.Properties.Property {
				f.Printer.PrintField(p.Name, maskSecure(p.Name, p.Value))
			}
		}
	})

	return nil
}
