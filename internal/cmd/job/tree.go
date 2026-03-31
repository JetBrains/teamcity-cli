package job

import (
	"fmt"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/spf13/cobra"
)

func newJobTreeCmd(f *cmdutil.Factory) *cobra.Command {
	var depth int
	var only string

	cmd := &cobra.Command{
		Use:   "tree <job-id>",
		Short: "Display snapshot dependency tree",
		Example: `  teamcity job tree MyProject_Build
  teamcity job tree Falcon_Deploy --depth 2
  teamcity job tree MyProject_Build --only dependents
  teamcity job tree MyProject_Build --only dependencies`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runJobTree(f, args[0], depth, only)
		},
	}

	cmd.Flags().IntVarP(&depth, "depth", "d", 0, "Limit tree depth (0 = unlimited)")
	cmd.Flags().StringVar(&only, "only", "", "Show only 'dependents' or 'dependencies'")
	cmdutil.AnnotateEnum(cmd, "only", []string{"dependents", "dependencies"})

	return cmd
}

func runJobTree(f *cmdutil.Factory, jobID string, depth int, only string) error {
	if only != "" && only != "dependents" && only != "dependencies" {
		return fmt.Errorf("--only must be 'dependents' or 'dependencies'")
	}

	client, err := f.Client()
	if err != nil {
		return err
	}

	bt, err := client.GetBuildType(jobID)
	if err != nil {
		return err
	}

	if depth > 0 {
		depth++
	}

	p := f.Printer
	if only != "" {
		tree := buildJobTree(client, jobID, bt.Name, depth, only == "dependents", map[string]bool{jobID: true})
		p.PrintTree(tree)
		return nil
	}

	up := buildJobTree(client, jobID, bt.Name, depth, true, map[string]bool{jobID: true})
	down := buildJobTree(client, jobID, bt.Name, depth, false, map[string]bool{jobID: true})

	section := func(label string, children []output.TreeNode) output.TreeNode {
		l := output.Faint(label)
		if len(children) == 0 {
			l += output.Faint(": none")
		}
		return output.TreeNode{Label: l, Children: children}
	}

	p.PrintTree(output.TreeNode{
		Label: output.Cyan(bt.Name),
		Children: []output.TreeNode{
			section("▲ Dependents", up.Children),
			section("▼ Dependencies", down.Children),
		},
	})
	return nil
}

func buildJobTree(client api.ClientInterface, jobID, name string, depth int, reverse bool, visited map[string]bool) output.TreeNode {
	node := output.TreeNode{Label: output.Cyan(name)}
	if depth == 1 {
		return node
	}

	children, err := jobTreeChildren(client, jobID, reverse)
	if err != nil {
		return node
	}

	next := max(depth-1, 0)
	for _, bt := range children {
		label := output.Cyan(bt.Name) + " " + output.Faint(bt.ID)
		if visited[bt.ID] {
			node.Children = append(node.Children, output.TreeNode{Label: label + " " + output.Yellow("(circular)")})
			continue
		}
		visited[bt.ID] = true
		child := buildJobTree(client, bt.ID, bt.Name, next, reverse, visited)
		child.Label = label
		node.Children = append(node.Children, child)
	}
	return node
}

func jobTreeChildren(client api.ClientInterface, jobID string, reverse bool) ([]api.BuildType, error) {
	if reverse {
		list, err := client.GetDependentBuildTypes(jobID)
		if err != nil {
			return nil, err
		}
		return list.BuildTypes, nil
	}
	deps, err := client.GetSnapshotDependencies(jobID)
	if err != nil {
		return nil, err
	}
	var result []api.BuildType
	for _, dep := range deps.SnapshotDependency {
		if dep.SourceBuildType != nil {
			result = append(result, *dep.SourceBuildType)
		}
	}
	return result, nil
}
