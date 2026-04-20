package project

import (
	"cmp"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/spf13/cobra"
)

func newConnectionCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "connection",
		Short: "Manage project connections",
		Long:  `List OAuth and other connections configured in a project.`,
		Args:  cobra.NoArgs,
		RunE:  cmdutil.SubcommandRequired,
	}

	cmd.AddCommand(newConnectionListCmd(f))

	return cmd
}

type connectionListOptions struct {
	project string
	cmdutil.ListFlags
}

func newConnectionListCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &connectionListOptions{}

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List project connections",
		Aliases: []string{"ls"},
		Example: `  teamcity project connection list
  teamcity project connection list --project MyProject
  teamcity project connection list --json
  teamcity project connection list --plain`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.RunList(f, cmd, &opts.ListFlags, &api.ConnectionFields, opts.fetch)
		},
	}

	cmd.Flags().StringVarP(&opts.project, "project", "p", "", "Project ID (default: _Root)")
	cmdutil.AddListFlags(cmd, &opts.ListFlags, 100)

	return cmd
}

func (opts *connectionListOptions) fetch(client api.ClientInterface, fields []string) (*cmdutil.ListResult, error) {
	projectID := cmp.Or(opts.project, "_Root")
	features, err := client.GetProjectConnections(projectID)
	if err != nil {
		return nil, err
	}

	items := features.ProjectFeature
	if opts.Limit > 0 && opts.Limit < len(items) {
		items = items[:opts.Limit]
	}

	headers := []string{"ID", "NAME", "TYPE"}
	var rows [][]string
	for _, feat := range items {
		name, providerType := connectionDisplayInfo(feat)
		rows = append(rows, []string{feat.ID, name, providerType})
	}

	return &cmdutil.ListResult{
		JSON:      filterJSONList(items, fields, connectionToMap),
		Table:     cmdutil.ListTable{Headers: headers, Rows: rows, FlexCols: []int{0, 1, 2}},
		EmptyMsg:  "No connections found",
		EmptyHint: output.HintNoConnections,
	}, nil
}

func connectionToMap(feat api.ProjectFeature) map[string]any {
	m := map[string]any{
		"id":   feat.ID,
		"type": feat.Type,
	}
	if feat.Properties != nil {
		m["properties"] = feat.Properties
	}
	return m
}

func filterJSONList[T any](items []T, fields []string, toMap func(T) map[string]any) any {
	var result []map[string]any
	for _, item := range items {
		full := toMap(item)
		filtered := make(map[string]any, len(fields))
		for _, f := range fields {
			if v, ok := full[f]; ok {
				filtered[f] = v
			}
		}
		result = append(result, filtered)
	}
	return result
}

func connectionDisplayInfo(feat api.ProjectFeature) (name, providerType string) {
	if feat.Properties == nil {
		return feat.ID, feat.Type
	}
	for _, p := range feat.Properties.Property {
		switch p.Name {
		case "displayName":
			name = p.Value
		case "providerType":
			providerType = p.Value
		}
	}
	if name == "" {
		name = feat.ID
	}
	if providerType == "" {
		providerType = feat.Type
	}
	return name, providerType
}

// connectionOptions fetches connections for the vcs create wizard select prompt
func connectionOptions(client api.ClientInterface, projectID string) (ids, labels []string, err error) {
	features, err := client.GetProjectConnections(projectID)
	if err != nil {
		return nil, nil, err
	}
	for _, feat := range features.ProjectFeature {
		name, ptype := connectionDisplayInfo(feat)
		ids = append(ids, feat.ID)
		labels = append(labels, feat.ID+" — "+name+" ("+ptype+")")
	}
	return ids, labels, nil
}
