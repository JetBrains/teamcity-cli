package run

import (
	"context"
	"fmt"
	"maps"
	"strconv"
	"strings"
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
	StatusText   string        `json:"statusText,omitempty"`
	State        string        `json:"state,omitempty"`
	Dependencies []RunTreeNode `json:"dependencies"`
	circular     bool
}

func (n RunTreeNode) toDisplayNode() output.TreeNode {
	label := output.StatusIcon(n.Status, n.State, n.StatusText) + " " + output.Cyan(n.Name) + " " + output.Faint(strconv.Itoa(n.ID))
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
		Use:   "tree <id>",
		Short: "Display snapshot dependency tree",
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

	build, err := client.GetBuild(context.Background(), runID)
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

	pipelineRun, _ := client.GetBuildPipelineRun(strconv.Itoa(build.ID))
	if pipelineRun != nil && pipelineRun.Pipeline != nil && pipelineRun.Pipeline.Name != "" {
		printPipelineTree(f.Printer, build, pipelineRun, node)
		return nil
	}

	f.Printer.PrintTree(node.toDisplayNode())
	return nil
}

func printPipelineTree(p *output.Printer, build *api.Build, pr *api.PipelineRun, node RunTreeNode) {
	icon := output.StatusIcon(build.Status, build.State, build.StatusText)
	header := fmt.Sprintf("%s %s ⬡  %d  #%s", icon, output.Cyan(pr.Pipeline.Name), build.ID, build.Number)
	if build.BranchName != "" {
		header += "  · " + build.BranchName
	}
	_, _ = fmt.Fprintln(p.Out, header)

	summary := buildStatusSummary(node.Dependencies)
	if summary != "" {
		_, _ = fmt.Fprintf(p.Out, "  %s\n", summary)
	}

	_, _ = fmt.Fprintln(p.Out)

	maxNameLen := 0
	for _, dep := range node.Dependencies {
		if len(dep.Name) > maxNameLen {
			maxNameLen = len(dep.Name)
		}
	}

	for _, dep := range node.Dependencies {
		icon := output.StatusIcon(dep.Status, dep.State, dep.StatusText)
		padded := fmt.Sprintf("%-*s", maxNameLen+2, dep.Name)
		_, _ = fmt.Fprintf(p.Out, "  %s %s %s\n", icon, padded, output.Faint(strconv.Itoa(dep.ID)))
	}
}

func buildStatusSummary(deps []RunTreeNode) string {
	var failed, passed, running, queued int
	for _, d := range deps {
		switch {
		case d.State == "running":
			running++
		case d.State == "queued":
			queued++
		case strings.EqualFold(d.Status, "SUCCESS"):
			passed++
		case strings.EqualFold(d.Status, "FAILURE") || strings.EqualFold(d.Status, "ERROR"):
			failed++
		}
	}

	var parts []string
	if failed > 0 {
		parts = append(parts, output.Red(fmt.Sprintf("%d failed", failed)))
	}
	if passed > 0 {
		parts = append(parts, output.Green(fmt.Sprintf("%d passed", passed)))
	}
	if running > 0 {
		parts = append(parts, output.Yellow(fmt.Sprintf("%d running", running)))
	}
	if queued > 0 {
		parts = append(parts, output.Faint(fmt.Sprintf("%d queued", queued)))
	}
	return strings.Join(parts, " · ")
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
		StatusText:   b.StatusText,
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
		maps.Copy(childPath, path)
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
