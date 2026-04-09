package run

import (
	"strconv"
	"sync"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/spf13/cobra"
)

type RunTreeNode struct {
	ID           int           `json:"id"`
	Number       string        `json:"number,omitempty"`
	Name         string        `json:"name"`
	BuildTypeID  string        `json:"buildTypeId"`
	Status       string        `json:"status,omitempty"`
	State        string        `json:"state,omitempty"`
	Dependencies []RunTreeNode `json:"dependencies"`
	circular     bool
}

func (n RunTreeNode) toDisplayNode() output.TreeNode {
	label := output.StatusIcon(n.Status, n.State) + " " + output.Cyan(n.Name) + " " + output.Faint(strconv.Itoa(n.ID))
	if n.circular {
		return output.TreeNode{Label: label + " " + output.Yellow("(circular)")}
	}
	children := make([]output.TreeNode, len(n.Dependencies))
	for i, dep := range n.Dependencies {
		children[i] = dep.toDisplayNode()
	}
	return output.TreeNode{Label: label, Children: children}
}

func newRunTreeCmd(f *cmdutil.Factory) *cobra.Command {
	var depth int
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "tree <run-id>",
		Short: "Display snapshot dependency tree for a run",
		Example: `  teamcity run tree 12345
  teamcity run tree 12345 --depth 2
  teamcity run tree 12345 --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRunTree(f, args[0], depth, jsonOut)
		},
	}

	cmd.Flags().IntVarP(&depth, "depth", "d", 0, "Limit tree depth (0 = unlimited)")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON")

	return cmd
}

func runRunTree(f *cmdutil.Factory, runID string, depth int, jsonOut bool) error {
	client, err := f.Client()
	if err != nil {
		return err
	}

	build, err := client.GetBuild(runID)
	if err != nil {
		return err
	}

	if depth > 0 {
		depth++
	}

	node, err := buildRunTree(client, *build, depth, map[string]bool{strconv.Itoa(build.ID): true})
	if err != nil {
		return err
	}

	if jsonOut {
		return f.Printer.PrintJSON(node)
	}
	f.Printer.PrintTree(node.toDisplayNode())
	return nil
}

func buildRunTree(client api.ClientInterface, b api.Build, depth int, path map[string]bool) (RunTreeNode, error) {
	name := b.BuildTypeID
	if b.BuildType != nil && b.BuildType.Name != "" {
		name = b.BuildType.Name
	}
	node := RunTreeNode{
		ID:           b.ID,
		Number:       b.Number,
		Name:         name,
		BuildTypeID:  b.BuildTypeID,
		Status:       b.Status,
		State:        b.State,
		Dependencies: []RunTreeNode{},
	}
	if depth == 1 {
		return node, nil
	}

	deps, err := client.GetBuildSnapshotDependencies(strconv.Itoa(b.ID))
	if err != nil {
		return node, err
	}

	next := max(depth-1, 0)
	type result struct {
		idx  int
		node RunTreeNode
		err  error
	}
	results := make([]result, len(deps.Builds))
	var wg sync.WaitGroup
	for i, dep := range deps.Builds {
		sid := strconv.Itoa(dep.ID)
		if path[sid] {
			name := dep.BuildTypeID
			if dep.BuildType != nil && dep.BuildType.Name != "" {
				name = dep.BuildType.Name
			}
			results[i] = result{idx: i, node: RunTreeNode{
				ID:           dep.ID,
				Number:       dep.Number,
				Name:         name,
				BuildTypeID:  dep.BuildTypeID,
				Dependencies: []RunTreeNode{},
				circular:     true,
			}}
			continue
		}
		childPath := make(map[string]bool, len(path)+1)
		for k, v := range path {
			childPath[k] = v
		}
		childPath[sid] = true
		wg.Add(1)
		go func(i int, dep api.Build, childPath map[string]bool) {
			defer wg.Done()
			child, err := buildRunTree(client, dep, next, childPath)
			results[i] = result{idx: i, node: child, err: err}
		}(i, dep, childPath)
	}
	wg.Wait()

	for _, r := range results {
		if r.err != nil {
			return node, r.err
		}
		node.Dependencies = append(node.Dependencies, r.node)
	}
	return node, nil
}
