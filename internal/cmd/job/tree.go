package job

import (
	"fmt"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/spf13/cobra"
)

type JobTreeNode struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	ProjectID    string        `json:"projectId"`
	Dependents   []JobTreeNode `json:"dependents,omitempty"`
	Dependencies []JobTreeNode `json:"dependencies,omitempty"`
	circular     bool
}

func (n JobTreeNode) toDisplayNode() output.TreeNode {
	label := output.Cyan(n.Name) + " " + output.Faint(n.ID)
	if n.circular {
		return output.TreeNode{Label: label + " " + output.Yellow("(circular)")}
	}
	kids := n.Dependents
	if len(n.Dependencies) > 0 {
		kids = n.Dependencies
	}
	children := make([]output.TreeNode, len(kids))
	for i, k := range kids {
		children[i] = k.toDisplayNode()
	}
	return output.TreeNode{Label: label, Children: children}
}

func newJobTreeCmd(f *cmdutil.Factory) *cobra.Command {
	var depth int
	var only string
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "tree <job-id>",
		Short: "Display snapshot dependency tree",
		Example: `  teamcity job tree MyProject_Build
  teamcity job tree Falcon_Deploy --depth 2
  teamcity job tree MyProject_Build --only dependents
  teamcity job tree MyProject_Build --only dependencies
  teamcity job tree MyProject_Build --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runJobTree(f, args[0], depth, only, jsonOut)
		},
	}

	cmd.Flags().IntVarP(&depth, "depth", "d", 0, "Limit tree depth (0 = unlimited)")
	cmd.Flags().StringVar(&only, "only", "", "Show only 'dependents' or 'dependencies'")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON")

	return cmd
}

func runJobTree(f *cmdutil.Factory, jobID string, depth int, only string, jsonOut bool) error {
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

	if jsonOut {
		node := JobTreeNode{ID: bt.ID, Name: bt.Name, ProjectID: bt.ProjectID}
		if only != "dependencies" {
			node.Dependents = buildJobTreeNodes(client, jobID, depth, true, map[string]bool{jobID: true})
		}
		if only != "dependents" {
			node.Dependencies = buildJobTreeNodes(client, jobID, depth, false, map[string]bool{jobID: true})
		}
		return f.Printer.PrintJSON(node)
	}

	p := f.Printer
	if only != "" {
		nodes := buildJobTreeNodes(client, jobID, depth, only == "dependents", map[string]bool{jobID: true})
		root := output.TreeNode{Label: output.Cyan(bt.Name)}
		for _, n := range nodes {
			root.Children = append(root.Children, n.toDisplayNode())
		}
		p.PrintTree(root)
		return nil
	}

	upNodes := buildJobTreeNodes(client, jobID, depth, true, map[string]bool{jobID: true})
	downNodes := buildJobTreeNodes(client, jobID, depth, false, map[string]bool{jobID: true})

	section := func(label string, nodes []JobTreeNode) output.TreeNode {
		l := output.Faint(label)
		if len(nodes) == 0 {
			l += output.Faint(": none")
		}
		sec := output.TreeNode{Label: l}
		for _, n := range nodes {
			sec.Children = append(sec.Children, n.toDisplayNode())
		}
		return sec
	}

	p.PrintTree(output.TreeNode{
		Label: output.Cyan(bt.Name),
		Children: []output.TreeNode{
			section("▲ Dependents", upNodes),
			section("▼ Dependencies", downNodes),
		},
	})
	return nil
}

func buildJobTreeNodes(client api.ClientInterface, jobID string, depth int, reverse bool, visited map[string]bool) []JobTreeNode {
	if depth == 1 {
		return nil
	}

	children, err := jobTreeChildren(client, jobID, reverse)
	if err != nil {
		return nil
	}

	next := max(depth-1, 0)
	var nodes []JobTreeNode
	for _, bt := range children {
		node := JobTreeNode{ID: bt.ID, Name: bt.Name, ProjectID: bt.ProjectID}
		if visited[bt.ID] {
			node.circular = true
			nodes = append(nodes, node)
			continue
		}
		visited[bt.ID] = true
		kids := buildJobTreeNodes(client, bt.ID, next, reverse, visited)
		if reverse {
			node.Dependents = kids
		} else {
			node.Dependencies = kids
		}
		nodes = append(nodes, node)
	}
	return nodes
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
