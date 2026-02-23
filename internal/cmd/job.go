package cmd

import (
	"fmt"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
)

func newJobCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "job",
		Short: "Manage jobs (build configurations)",
		Long:  `List and manage TeamCity jobs (build configurations).`,
		Args:  cobra.NoArgs,
		RunE:  subcommandRequired,
	}

	cmd.AddCommand(newJobListCmd())
	cmd.AddCommand(newJobViewCmd())
	cmd.AddCommand(newJobTreeCmd())
	cmd.AddCommand(newJobPauseCmd())
	cmd.AddCommand(newJobResumeCmd())
	cmd.AddCommand(newParamCmd("job", jobParamAPI))

	return cmd
}

type jobListOptions struct {
	project    string
	limit      int
	jsonFields string
}

func newJobListCmd() *cobra.Command {
	opts := &jobListOptions{}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List jobs",
		Example: `  teamcity job list
  teamcity job list --project Falcon
  teamcity job list --json
  teamcity job list --json=id,name,webUrl`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runJobList(cmd, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.project, "project", "p", "", "Filter by project ID")
	cmd.Flags().IntVarP(&opts.limit, "limit", "n", 30, "Maximum number of jobs")
	AddJSONFieldsFlag(cmd, &opts.jsonFields)

	return cmd
}

func runJobList(cmd *cobra.Command, opts *jobListOptions) error {
	if err := validateLimit(opts.limit); err != nil {
		return err
	}
	jsonResult, showHelp, err := ParseJSONFields(cmd, opts.jsonFields, &api.BuildTypeFields)
	if err != nil {
		return err
	}
	if showHelp {
		return nil
	}

	client, err := getClient()
	if err != nil {
		return err
	}

	jobs, err := client.GetBuildTypes(api.BuildTypesOptions{
		Project: opts.project,
		Limit:   opts.limit,
		Fields:  jsonResult.Fields,
	})
	if err != nil {
		return err
	}

	if jsonResult.Enabled {
		return output.PrintJSON(jobs)
	}

	if jobs.Count == 0 {
		fmt.Println("No jobs found")
		return nil
	}

	headers := []string{"ID", "NAME", "PROJECT", "STATUS"}
	var rows [][]string

	for _, j := range jobs.BuildTypes {
		status := output.Green("Active")
		if j.Paused {
			status = output.Faint("Paused")
		}

		rows = append(rows, []string{
			j.ID,
			j.Name,
			j.ProjectName,
			status,
		})
	}

	output.AutoSizeColumns(headers, rows, 2, 0, 1, 2)
	output.PrintTable(headers, rows)
	return nil
}

func newJobViewCmd() *cobra.Command {
	opts := &viewOptions{}
	cmd := &cobra.Command{
		Use:   "view <job-id>",
		Short: "View job details",
		Args:  cobra.ExactArgs(1),
		Example: `  teamcity job view Falcon_Build
  teamcity job view Falcon_Build --web`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runJobView(args[0], opts)
		},
	}
	addViewFlags(cmd, opts)
	return cmd
}

func runJobView(jobID string, opts *viewOptions) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	buildType, err := client.GetBuildType(jobID)
	if err != nil {
		return err
	}

	if opts.web {
		return browser.OpenURL(buildType.WebURL)
	}

	if opts.json {
		return output.PrintJSON(buildType)
	}

	fmt.Printf("%s\n", output.Cyan(buildType.Name))
	fmt.Printf("ID: %s\n", buildType.ID)
	fmt.Printf("Project: %s (%s)\n", buildType.ProjectName, buildType.ProjectID)

	status := output.Green("Active")
	if buildType.Paused {
		status = output.Faint("Paused")
	}
	fmt.Printf("Status: %s\n", status)

	fmt.Printf("\n%s %s\n", output.Faint("View in browser:"), output.Green(buildType.WebURL))

	return nil
}

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

func newJobStateCmd(a jobStateAction) *cobra.Command {
	return &cobra.Command{
		Use:     fmt.Sprintf("%s <job-id>", a.use),
		Short:   a.short,
		Long:    a.long,
		Args:    cobra.ExactArgs(1),
		Example: fmt.Sprintf("  teamcity job %s Falcon_Build", a.use),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := getClient()
			if err != nil {
				return err
			}
			if err := client.SetBuildTypePaused(args[0], a.paused); err != nil {
				return fmt.Errorf("failed to %s job: %w", a.use, err)
			}
			output.Success("%s job %s", a.verb, args[0])
			return nil
		},
	}
}

func newJobPauseCmd() *cobra.Command  { return newJobStateCmd(jobStateActions["pause"]) }
func newJobResumeCmd() *cobra.Command { return newJobStateCmd(jobStateActions["resume"]) }

func newJobTreeCmd() *cobra.Command {
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
			return runJobTree(args[0], depth, only)
		},
	}

	cmd.Flags().IntVarP(&depth, "depth", "d", 0, "Limit tree depth (0 = unlimited)")
	cmd.Flags().StringVar(&only, "only", "", "Show only 'dependents' or 'dependencies'")

	return cmd
}

func runJobTree(jobID string, depth int, only string) error {
	if only != "" && only != "dependents" && only != "dependencies" {
		return fmt.Errorf("--only must be 'dependents' or 'dependencies'")
	}

	client, err := getClient()
	if err != nil {
		return err
	}

	bt, err := client.GetBuildType(jobID)
	if err != nil {
		return err
	}

	if only != "" {
		tree := buildJobTree(client, jobID, bt.Name, depth, only == "dependents", map[string]bool{jobID: true})
		output.PrintTree(tree)
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

	output.PrintTree(output.TreeNode{
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
