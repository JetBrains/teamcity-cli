package project

import (
	"cmp"
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/config"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
)

func newVcsCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vcs",
		Short: "Manage VCS roots",
		Long:  `List, view, create, and delete VCS roots in a project.`,
		Args:  cobra.NoArgs,
		RunE:  cmdutil.SubcommandRequired,
	}

	cmd.AddCommand(newVcsListCmd(f))
	cmd.AddCommand(newVcsViewCmd(f))
	cmd.AddCommand(newVcsCreateCmd(f))
	cmd.AddCommand(newVcsDeleteCmd(f))

	return cmd
}

type vcsListOptions struct {
	project string
	cmdutil.ListFlags
}

func newVcsListCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &vcsListOptions{}

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List VCS roots",
		Long:    `List VCS roots visible to a project, including inherited from parent projects.`,
		Aliases: []string{"ls"},
		Example: `  teamcity project vcs list
  teamcity project vcs list --project MyProject
  teamcity project vcs list --project MyProject --json
  teamcity project vcs list --plain`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.RunList(f, cmd, &opts.ListFlags, &api.VcsRootFields, opts.fetch)
		},
	}

	cmd.Flags().StringVarP(&opts.project, "project", "p", "", "Project ID (default: _Root)")
	cmdutil.AddListFlags(cmd, &opts.ListFlags, 100)

	return cmd
}

func (opts *vcsListOptions) fetch(client api.ClientInterface, fields []string) (*cmdutil.ListResult, error) {
	project := cmp.Or(opts.project, "_Root")
	roots, err := client.GetVcsRoots(api.VcsRootsOptions{
		Project: project,
		Limit:   opts.Limit,
		Fields:  fields,
	})
	if err != nil {
		return nil, err
	}

	headers := []string{"ID", "NAME", "TYPE", "PROJECT"}
	var rows [][]string

	for _, r := range roots.VcsRoot {
		projectID := ""
		if r.Project != nil {
			projectID = r.Project.ID
		}

		rows = append(rows, []string{
			r.ID,
			r.Name,
			vcsTypeName(r.VcsName),
			projectID,
		})
	}

	return &cmdutil.ListResult{
		JSON:     roots,
		Table:    cmdutil.ListTable{Headers: headers, Rows: rows, FlexCols: []int{0, 1, 2, 3}},
		EmptyMsg: "No VCS roots found",
	}, nil
}

func newVcsViewCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &cmdutil.ViewOptions{}

	cmd := &cobra.Command{
		Use:     "view <vcs-root-id>",
		Short:   "View VCS root details",
		Aliases: []string{"show"},
		Args:    cobra.ExactArgs(1),
		Example: `  teamcity project vcs view MyProject_GitHubRepo
  teamcity project vcs view MyProject_GitHubRepo --json
  teamcity project vcs view MyProject_GitHubRepo --web`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runVcsView(f, args[0], opts)
		},
	}

	cmdutil.AddViewFlags(cmd, opts)

	return cmd
}

var vcsTypeNames = map[string]string{
	"jetbrains.git": "Git",
	"perforce":      "Perforce Helix Core",
	"svn":           "Subversion",
	"mercurial":     "Mercurial",
	"tfs":           "Team Foundation Version Control",
}

func vcsTypeName(vcsName string) string {
	if name, ok := vcsTypeNames[vcsName]; ok {
		return name
	}
	return vcsName
}

var vcsPropertyLabels = map[string]string{
	"url":                    "URL",
	"branch":                 "Branch",
	"teamcity:branchSpec":    "Branch Spec",
	"authMethod":             "Auth Method",
	"username":               "Username",
	"secure:password":        "Password",
	"secure:passphrase":      "Passphrase",
	"submoduleCheckout":      "Submodule Checkout",
	"agentCleanPolicy":       "Agent Clean Policy",
	"agentCleanFilesPolicy":  "Agent Clean Files Policy",
	"ignoreKnownHosts":       "Ignore Known Hosts",
	"useAlternates":          "Use Alternates",
	"usernameStyle":          "Username Style",
	"reportTagRevisions":     "Report Tag Revisions",
	"pipelines.connectionId": "Connection ID",
	"tokenId":               "Token ID",
}

func runVcsView(f *cmdutil.Factory, id string, opts *cmdutil.ViewOptions) error {
	client, err := f.Client()
	if err != nil {
		return err
	}

	root, err := client.GetVcsRoot(id)
	if err != nil {
		return err
	}

	if opts.Web {
		webURL := vcsRootEditURL(root.ID)
		return browser.OpenURL(webURL)
	}

	if opts.JSON {
		return f.Printer.PrintJSON(root)
	}

	webURL := vcsRootEditURL(root.ID)
	f.Printer.PrintViewHeader(root.Name, webURL, func() {
		f.Printer.PrintField("ID", root.ID)
		f.Printer.PrintField("Type", vcsTypeName(root.VcsName))
		if root.Project != nil {
			f.Printer.PrintField("Project", root.Project.ID)
		}
		if root.Properties != nil {
			for _, p := range root.Properties.Property {
				label := vcsPropertyLabel(p.Name)
				value := p.Value
				if strings.HasPrefix(p.Name, "secure:") {
					value = "********"
				}
				f.Printer.PrintField(label, value)
			}
		}
	})

	return nil
}

func vcsPropertyLabel(name string) string {
	if label, ok := vcsPropertyLabels[name]; ok {
		return label
	}
	return name
}

func vcsRootEditURL(id string) string {
	return fmt.Sprintf("%s/admin/editVcsRoot.html?vcsRootId=%s", config.GetServerURL(), id)
}

func newVcsCreateCmd(f *cmdutil.Factory) *cobra.Command {
	var project string

	cmd := &cobra.Command{
		Use:     "create",
		Short:   "Open TeamCity to add a VCS root",
		Long: `Opens the TeamCity UI in a browser to create a new VCS root in the specified project.

VCS root creation involves complex authentication configuration (OAuth connections,
SSH keys, tokens) that the TeamCity UI handles well. A full CLI-based create flow
may be added in a future release.`,
		Example: `  teamcity project vcs create
  teamcity project vcs create --project MyProject`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runVcsCreate(f, project)
		},
	}

	cmd.Flags().StringVarP(&project, "project", "p", "", "Project ID (default: _Root)")

	return cmd
}

func runVcsCreate(f *cmdutil.Factory, projectID string) error {
	projectID = cmp.Or(projectID, "_Root")
	serverURL := config.GetServerURL()
	if serverURL == "" {
		return fmt.Errorf("no server URL configured")
	}
	createURL := fmt.Sprintf("%s/admin/editVcsRoot.html?action=addVcsRoot&editingScope=editProject:%s", serverURL, projectID)

	f.Printer.Info("Opening TeamCity to add a VCS root to %s...", projectID)
	_, _ = fmt.Fprintf(f.Printer.Out, "  → %s\n", createURL)

	return browser.OpenURL(createURL)
}

type vcsDeleteOptions struct {
	force bool
}

func newVcsDeleteCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &vcsDeleteOptions{}

	cmd := &cobra.Command{
		Use:     "delete <vcs-root-id>",
		Short:   "Delete a VCS root",
		Aliases: []string{"rm"},
		Args:    cobra.ExactArgs(1),
		Example: `  teamcity project vcs delete MyProject_GitHubRepo
  teamcity project vcs delete MyProject_GitHubRepo --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runVcsDelete(f, args[0], opts)
		},
	}

	cmd.Flags().BoolVarP(&opts.force, "force", "f", false, "Skip confirmation prompt")

	return cmd
}

func runVcsDelete(f *cmdutil.Factory, id string, opts *vcsDeleteOptions) error {
	client, err := f.Client()
	if err != nil {
		return err
	}

	if !opts.force && f.IsInteractive() {
		var confirm bool
		prompt := &survey.Confirm{
			Message: fmt.Sprintf("Delete VCS root %q?", id),
			Default: false,
		}
		if err := survey.AskOne(prompt, &confirm); err != nil {
			return err
		}
		if !confirm {
			f.Printer.Info("Canceled")
			return nil
		}
	}

	if err := client.DeleteVcsRoot(id); err != nil {
		return fmt.Errorf("failed to delete VCS root: %w", err)
	}

	f.Printer.Success("Deleted VCS root %s", id)
	return nil
}
