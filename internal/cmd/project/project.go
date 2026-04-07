package project

import (
	"cmp"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmd/param"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	tcerrors "github.com/JetBrains/teamcity-cli/internal/errors"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
)

func NewCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project",
		Short: "Manage projects",
		Long:  `List and view TeamCity projects.`,
		Args:  cobra.NoArgs,
		RunE:  cmdutil.SubcommandRequired,
	}

	cmd.AddCommand(newProjectListCmd(f))
	cmd.AddCommand(newProjectViewCmd(f))
	cmd.AddCommand(newProjectTreeCmd(f))
	cmd.AddCommand(newProjectTokenCmd(f))
	cmd.AddCommand(newProjectSettingsCmd(f))
	cmd.AddCommand(newCloudCmd(f))
	cmd.AddCommand(newVcsCmd(f))
	cmd.AddCommand(param.NewCmd(f, "project", param.ProjectParamAPI))

	return cmd
}

type projectListOptions struct {
	parent string
	cmdutil.ListFlags
}

func newProjectListCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &projectListOptions{}

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List projects",
		Long:    `List all TeamCity projects.`,
		Aliases: []string{"ls"},
		Example: `  teamcity project list
  teamcity project list --parent Falcon
  teamcity project list --json
  teamcity project list --json=id,name,webUrl
  teamcity project list --plain
  teamcity project list --plain --no-header`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.RunList(f, cmd, &opts.ListFlags, &api.ProjectFields, opts.fetch)
		},
	}

	cmd.Flags().StringVarP(&opts.parent, "parent", "p", "", "Filter by parent project ID")
	cmdutil.AddListFlags(cmd, &opts.ListFlags, 100)

	return cmd
}

func (opts *projectListOptions) fetch(client api.ClientInterface, fields []string) (*cmdutil.ListResult, error) {
	projects, err := client.GetProjects(api.ProjectsOptions{
		Parent: opts.parent,
		Limit:  opts.Limit,
		Fields: fields,
	})
	if err != nil {
		return nil, err
	}

	headers := []string{"ID", "NAME", "PARENT"}
	var rows [][]string

	for _, p := range projects.Projects {
		parent := "-"
		if p.ParentProjectID != "" {
			parent = p.ParentProjectID
		}

		rows = append(rows, []string{
			p.ID,
			p.Name,
			parent,
		})
	}

	return &cmdutil.ListResult{
		JSON:     projects,
		Table:    cmdutil.ListTable{Headers: headers, Rows: rows, FlexCols: []int{0, 1, 2}},
		EmptyMsg: "No projects found",
	}, nil
}

func newProjectViewCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &cmdutil.ViewOptions{}
	cmd := &cobra.Command{
		Use:     "view <project-id>",
		Short:   "View project details",
		Long:    `View details of a TeamCity project.`,
		Aliases: []string{"show"},
		Args:    cobra.ExactArgs(1),
		Example: `  teamcity project view Falcon
  teamcity project view Falcon --web`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProjectView(f, args[0], opts)
		},
	}
	cmdutil.AddViewFlags(cmd, opts)
	return cmd
}

func runProjectView(f *cmdutil.Factory, projectID string, opts *cmdutil.ViewOptions) error {
	client, err := f.Client()
	if err != nil {
		return err
	}

	project, err := client.GetProject(projectID)
	if err != nil {
		return err
	}

	if opts.Web {
		return browser.OpenURL(project.WebURL)
	}

	if opts.JSON {
		return f.Printer.PrintJSON(project)
	}

	f.Printer.PrintViewHeader(project.Name, project.WebURL, func() {
		f.Printer.PrintField("ID", project.ID)
		if project.ParentProjectID != "" {
			f.Printer.PrintField("Parent", project.ParentProjectID)
		}
		if project.Description != "" {
			f.Printer.PrintField("Description", project.Description)
		}
	})

	return nil
}

func newProjectTokenCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "token",
		Short: "Manage secure tokens",
		Long: `Manage secure tokens for versioned settings.

Secure tokens allow you to store sensitive values (passwords, API keys, etc.)
in TeamCity's credentials storage. The scrambled token can be safely committed
to version control and used in configuration files as credentialsJSON:<token>.

See: https://www.jetbrains.com/help/teamcity/storing-project-settings-in-version-control.html#Managing+Tokens`,
		Args: cobra.NoArgs,
		RunE: cmdutil.SubcommandRequired,
	}

	cmd.AddCommand(newProjectTokenPutCmd(f))
	cmd.AddCommand(newProjectTokenGetCmd(f))

	return cmd
}

type projectTokenPutOptions struct {
	stdin bool
}

func newProjectTokenPutCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &projectTokenPutOptions{}

	cmd := &cobra.Command{
		Use:   "put <project-id> [value]",
		Short: "Store a secret and get a secure token",
		Long: `Store a sensitive value and get a secure token reference.

The returned token can be used in versioned settings configuration files
as credentialsJSON:<token>. The actual value is stored securely in TeamCity
and is not committed to version control.

Requires EDIT_PROJECT permission (Project Administrator role).`,
		Example: `  # Store a secret interactively (prompts for value)
  teamcity project token put Falcon

  # Store a secret from a value
  teamcity project token put Falcon "my-secret-password"

  # Store a secret from stdin (useful for piping)
  echo -n "my-secret" | teamcity project token put Falcon --stdin

  # Use the token in versioned settings
  # password: credentialsJSON:<returned-token>`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			var value string
			if len(args) > 1 {
				value = args[1]
			}
			return runProjectTokenPut(f, args[0], value, opts)
		},
	}

	cmd.Flags().BoolVar(&opts.stdin, "stdin", false, "Read value from stdin")

	return cmd
}

func runProjectTokenPut(f *cmdutil.Factory, projectID, value string, opts *projectTokenPutOptions) error {
	client, err := f.Client()
	if err != nil {
		return err
	}

	if opts.stdin {
		data, err := io.ReadAll(f.IOStreams.In)
		if err != nil {
			return fmt.Errorf("failed to read from stdin: %w", err)
		}
		value = strings.TrimSuffix(string(data), "\n")
	}

	if value == "" && !f.IsInteractive() {
		return tcerrors.WithSuggestion(
			"value is required",
			"Provide value as argument or use --stdin",
		)
	}

	if value == "" {
		prompt := &survey.Password{
			Message: "Enter secure value to scramble:",
		}
		if err := survey.AskOne(prompt, &value); err != nil {
			return fmt.Errorf("failed to read value: %w", err)
		}
	}

	if value == "" {
		return fmt.Errorf("value cannot be empty")
	}

	token, err := client.CreateSecureToken(projectID, value)
	if err != nil {
		return fmt.Errorf("failed to create secure token: %w", err)
	}

	_, _ = fmt.Fprintln(f.Printer.Out, token)

	if strings.HasPrefix(token, "credentialsJSON:") {
		_, _ = fmt.Fprintln(f.Printer.ErrOut, "")
		_, _ = fmt.Fprintln(f.Printer.ErrOut, output.Faint("Use in versioned settings as: "+token))
	}

	return nil
}

func newProjectTokenGetCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <project-id> <token>",
		Short: "Get the value of a secure token",
		Long: `Retrieve the original value for a secure token.

This operation requires CHANGE_SERVER_SETTINGS permission,
which is only available to System Administrators.`,
		Example: `  teamcity project token get Falcon "credentialsJSON:abc123..."
  teamcity project token get Falcon "abc123..."`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProjectTokenGet(f, args[0], args[1])
		},
	}

	return cmd
}

func runProjectTokenGet(f *cmdutil.Factory, projectID, token string) error {
	client, err := f.Client()
	if err != nil {
		return err
	}

	token = strings.TrimPrefix(token, "credentialsJSON:")

	value, err := client.GetSecureValue(projectID, token)
	if err != nil {
		return fmt.Errorf("failed to get secure value: %w", err)
	}

	_, _ = fmt.Fprintln(f.Printer.Out, value)
	return nil
}

func newProjectTreeCmd(f *cmdutil.Factory) *cobra.Command {
	var noJobs bool
	var depth int

	cmd := &cobra.Command{
		Use:   "tree [project-id]",
		Short: "Display project hierarchy as a tree",
		Example: `  teamcity project tree
  teamcity project tree MyProject
  teamcity project tree --no-jobs
  teamcity project tree --depth 2`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rootID := "_Root"
			if len(args) > 0 {
				rootID = args[0]
			}
			return runProjectTree(f, rootID, noJobs, depth)
		},
	}

	cmd.Flags().BoolVar(&noJobs, "no-jobs", false, "Hide build configurations")
	cmd.Flags().IntVarP(&depth, "depth", "d", 0, "Limit tree depth (0 = unlimited)")

	return cmd
}

func runProjectTree(f *cmdutil.Factory, rootID string, noJobs bool, depth int) error {
	client, err := f.Client()
	if err != nil {
		return err
	}

	projects, err := client.GetProjects(api.ProjectsOptions{Limit: 10000})
	if err != nil {
		return err
	}

	known := map[string]*api.Project{}
	children := map[string][]api.Project{}
	for i := range projects.Projects {
		p := &projects.Projects[i]
		known[p.ID] = p
		if p.ParentProjectID != "" {
			children[p.ParentProjectID] = append(children[p.ParentProjectID], *p)
		}
	}

	root := known[rootID]
	if root == nil {
		root, err = client.GetProject(rootID)
		if err != nil {
			return fmt.Errorf("project %q not found", rootID)
		}
		known[root.ID] = root
	}

	var jobsByProject map[string][]api.BuildType
	if !noJobs {
		buildTypes, err := client.GetBuildTypes(api.BuildTypesOptions{Limit: 10000})
		if err != nil {
			return err
		}
		jobsByProject = map[string][]api.BuildType{}
		for _, bt := range buildTypes.BuildTypes {
			jobsByProject[bt.ProjectID] = append(jobsByProject[bt.ProjectID], bt)
		}
		resolveHiddenProjects(client, known, children, jobsByProject)
	}

	if depth > 0 {
		depth++
	}
	f.Printer.PrintTree(buildProjectTree(children, jobsByProject, rootID, root.Name, depth))
	return nil
}

func buildProjectTree(children map[string][]api.Project, jobs map[string][]api.BuildType, id, name string, depth int) output.TreeNode {
	node := output.TreeNode{Label: output.Cyan(name) + " " + output.Faint(id)}
	if depth == 1 {
		return node
	}
	next := max(depth-1, 0)
	slices.SortFunc(children[id], func(a, b api.Project) int { return cmp.Compare(a.Name, b.Name) })
	for _, p := range children[id] {
		node.Children = append(node.Children, buildProjectTree(children, jobs, p.ID, p.Name, next))
	}
	slices.SortFunc(jobs[id], func(a, b api.BuildType) int { return cmp.Compare(a.Name, b.Name) })
	for _, j := range jobs[id] {
		node.Children = append(node.Children, output.TreeNode{Label: output.Faint(j.Name) + " " + output.Faint(j.ID)})
	}
	return node
}

func resolveHiddenProjects(client api.ClientInterface, known map[string]*api.Project, children map[string][]api.Project, jobsByProject map[string][]api.BuildType) {
	var queue []string
	for pid := range jobsByProject {
		if _, ok := known[pid]; !ok {
			queue = append(queue, pid)
			known[pid] = nil
		}
	}
	for i := 0; i < len(queue); i++ {
		p, err := client.GetProject(queue[i])
		if err != nil {
			continue
		}
		known[p.ID] = p
		children[p.ParentProjectID] = append(children[p.ParentProjectID], *p)
		if _, ok := known[p.ParentProjectID]; p.ParentProjectID != "" && !ok {
			queue = append(queue, p.ParentProjectID)
			known[p.ParentProjectID] = nil
		}
	}
}

func newProjectSettingsCmd(f *cmdutil.Factory) *cobra.Command {
	return newSettingsCmd(f)
}
