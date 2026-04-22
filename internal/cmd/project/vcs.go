package project

import (
	"cmp"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/config"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/charmbracelet/huh"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
)

func newVcsCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vcs",
		Short: "Manage VCS roots",
		Long: `List, view, create, test, and delete VCS roots in a project.

A VCS root defines how TeamCity connects to a version control
repository (Git, Mercurial, Perforce, SVN, ...) so that jobs can
check out sources and react to changes.

See: https://www.jetbrains.com/help/teamcity/vcs-root.html`,
		Args: cobra.NoArgs,
		RunE: cmdutil.SubcommandRequired,
	}

	cmd.AddCommand(newVcsListCmd(f))
	cmd.AddCommand(newVcsViewCmd(f))
	cmd.AddCommand(newVcsCreateCmd(f))
	cmd.AddCommand(newVcsTestCmd(f))
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
	"tokenId":                "Token ID",
	"teamcitySshKey":         "SSH Key",
	"privateKeyPath":         "Private Key Path",
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

const (
	authPassword  = "password"
	authSSHKey    = "ssh-key"
	authSSHAgent  = "ssh-agent"
	authSSHFile   = "ssh-file"
	authToken     = "token"
	authAnonymous = "anonymous"
)

var authMethodLabels = []string{
	"Password / Personal Access Token",
	"SSH Key (uploaded to TeamCity)",
	"SSH Key (default on build agent)",
	"SSH Key (custom path on agent)",
	"Access Token (via project connection)",
	"Anonymous",
}

var authMethodValues = []string{
	authPassword,
	authSSHKey,
	authSSHAgent,
	authSSHFile,
	authToken,
	authAnonymous,
}

type vcsCreateOptions struct {
	project      string
	repoURL      string
	name         string
	branch       string
	branchSpec   string
	auth         string
	username     string
	password     string
	stdin        bool
	sshKeyName   string
	keyPath      string
	passphrase   string
	connectionID string
	noTest       bool
}

func newVcsCreateCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &vcsCreateOptions{}

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a VCS root",
		Long: `Create a new Git VCS root in a project.

In interactive mode, guides you through URL, name, and authentication setup.
Tests the connection before creating unless --no-test is specified.`,
		Example: `  # Interactive wizard
  teamcity project vcs create

  # Non-interactive with password/PAT
  teamcity project vcs create --url https://github.com/org/repo.git --auth password --username oauth2 --password ghp_xxx

  # Non-interactive with SSH key uploaded to TeamCity
  teamcity project vcs create --url git@github.com:org/repo.git --auth ssh-key --ssh-key-name my-key

  # Non-interactive with anonymous access
  teamcity project vcs create --url https://github.com/org/repo.git --auth anonymous

  # Skip connection test
  teamcity project vcs create --url https://github.com/org/repo.git --auth anonymous --no-test`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runVcsCreate(f, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.project, "project", "p", "", "Project ID (default: _Root)")
	cmd.Flags().StringVar(&opts.repoURL, "url", "", "Repository URL")
	cmd.Flags().StringVar(&opts.name, "name", "", "Display name (auto-generated from URL if omitted)")
	cmd.Flags().StringVar(&opts.branch, "branch", "refs/heads/main", "Default branch")
	cmd.Flags().StringVar(&opts.branchSpec, "branch-spec", "", "Branch specification")
	cmd.Flags().StringVar(&opts.auth, "auth", "", "Auth method: password|ssh-key|ssh-agent|ssh-file|token|anonymous")
	cmd.Flags().StringVar(&opts.username, "username", "", "Username")
	cmd.Flags().StringVar(&opts.password, "password", "", "Password or personal access token")
	cmd.Flags().BoolVar(&opts.stdin, "stdin", false, "Read password from stdin")
	cmd.Flags().StringVar(&opts.sshKeyName, "ssh-key-name", "", "Name of SSH key uploaded to TeamCity")
	cmd.Flags().StringVar(&opts.keyPath, "key-path", "", "Path to SSH key file on agent")
	cmd.Flags().StringVar(&opts.passphrase, "passphrase", "", "SSH key passphrase")
	cmd.Flags().StringVar(&opts.connectionID, "connection-id", "", "OAuth connection ID")
	cmd.Flags().BoolVar(&opts.noTest, "no-test", false, "Skip connection test before creating")

	return cmd
}

func runVcsCreate(f *cmdutil.Factory, opts *vcsCreateOptions) error {
	client, err := f.Client()
	if err != nil {
		return err
	}

	projectID := cmp.Or(opts.project, "_Root")
	interactive := f.IsInteractive()

	repoURL := opts.repoURL
	if repoURL == "" {
		if !interactive {
			return api.RequiredFlag("url")
		}
		if err := cmdutil.PromptString(f.Printer, "Repository URL", "", &repoURL); err != nil {
			return err
		}
	}

	name := opts.name
	if name == "" {
		name = vcsNameFromURL(repoURL)
	}

	authMethod := opts.auth
	if authMethod == "" {
		if !interactive {
			authMethod = inferAuthFromURL(repoURL)
		} else {
			authMethod = inferAuthFromURL(repoURL)
			options := make([]huh.Option[string], len(authMethodValues))
			for i, v := range authMethodValues {
				options[i] = huh.NewOption(authMethodLabels[i], v)
			}
			if err := cmdutil.Select(f.Printer, "Authentication method", options, &authMethod); err != nil {
				return err
			}
		}
	}

	props, testReq, err := resolveAuth(f, client, projectID, authMethod, opts, interactive)
	if err != nil {
		return err
	}

	props = append(props,
		api.Property{Name: "url", Value: repoURL},
		api.Property{Name: "branch", Value: opts.branch},
	)
	if opts.branchSpec != "" {
		props = append(props, api.Property{Name: "teamcity:branchSpec", Value: opts.branchSpec})
	}

	testReq.URL = repoURL
	testReq.VcsName = "jetbrains.git"

	if !opts.noTest && client.SupportsFeature("vcs_test_connection") {
		if err := runConnectionTest(f, client, testReq, projectID); err != nil {
			return err
		}
	}

	root := api.VcsRoot{
		Name:    name,
		VcsName: "jetbrains.git",
		Project: &api.Project{ID: projectID},
		Properties: &api.PropertyList{
			Property: props,
		},
	}

	created, err := client.CreateVcsRoot(root)
	if err != nil {
		return fmt.Errorf("failed to create VCS root: %w", err)
	}

	f.Printer.Success("Created VCS root %q (%s) in project %s", created.Name, created.ID, projectID)
	return nil
}

func resolveAuth(f *cmdutil.Factory, client api.ClientInterface, projectID, authMethod string, opts *vcsCreateOptions, interactive bool) ([]api.Property, api.TestConnectionRequest, error) {
	var props []api.Property
	var testReq api.TestConnectionRequest

	switch authMethod {
	case authPassword:
		username := cmp.Or(opts.username, "oauth2")
		password := opts.password

		if interactive {
			if err := cmdutil.PromptString(f.Printer, "Username", "", &username); err != nil {
				return nil, testReq, err
			}
			if password == "" && !opts.stdin {
				if err := cmdutil.PromptSecret("Password / Token", &password); err != nil {
					return nil, testReq, err
				}
			}
		}
		if password == "" && opts.stdin {
			data, err := readStdin(f)
			if err != nil {
				return nil, testReq, err
			}
			password = data
		}
		if password == "" {
			return nil, testReq, api.Validation(
				"password is required for password auth",
				"Use --password, --stdin, or run interactively",
			)
		}

		props = append(props,
			api.Property{Name: "authMethod", Value: "PASSWORD"},
			api.Property{Name: "username", Value: username},
			api.Property{Name: "secure:password", Value: password},
		)
		testReq.Username = username
		testReq.Password = password

	case authSSHKey:
		keyName := opts.sshKeyName
		if keyName == "" {
			if !interactive {
				return nil, testReq, api.RequiredFlag("ssh-key-name")
			}
			names, err := sshKeyNames(client, projectID)
			if err != nil {
				return nil, testReq, fmt.Errorf("failed to list SSH keys: %w", err)
			}
			if len(names) == 0 {
				return nil, testReq, fmt.Errorf("no SSH keys uploaded to project %s — upload one with: teamcity project ssh upload", projectID)
			}
			options := make([]huh.Option[string], len(names))
			for i, n := range names {
				options[i] = huh.NewOption(n, n)
			}
			if err := cmdutil.Select(f.Printer, "SSH key", options, &keyName); err != nil {
				return nil, testReq, err
			}
		}
		props = append(props,
			api.Property{Name: "authMethod", Value: "TEAMCITY_SSH_KEY"},
			api.Property{Name: "teamcitySshKey", Value: keyName},
			api.Property{Name: "username", Value: "git"},
		)
		testReq.SSHKey = &api.SSHKeyRef{Name: keyName}
		testReq.IsPrivate = true

	case authSSHAgent:
		props = append(props,
			api.Property{Name: "authMethod", Value: "PRIVATE_KEY_DEFAULT"},
			api.Property{Name: "username", Value: "git"},
		)
		testReq.IsPrivate = true

	case authSSHFile:
		keyPath := opts.keyPath
		if keyPath == "" {
			if !interactive {
				return nil, testReq, api.RequiredFlag("key-path")
			}
			if err := cmdutil.PromptString(f.Printer, "Path to SSH key on build agent", "", &keyPath); err != nil {
				return nil, testReq, err
			}
		}
		props = append(props,
			api.Property{Name: "authMethod", Value: "PRIVATE_KEY_FILE"},
			api.Property{Name: "privateKeyPath", Value: keyPath},
			api.Property{Name: "username", Value: "git"},
		)
		if opts.passphrase != "" {
			props = append(props, api.Property{Name: "secure:passphrase", Value: opts.passphrase})
		}
		testReq.IsPrivate = true

	case authToken:
		connID := opts.connectionID
		if connID == "" {
			if !interactive {
				return nil, testReq, api.RequiredFlag("connection-id")
			}
			ids, labels, err := connectionOptions(client, projectID)
			if err != nil {
				return nil, testReq, fmt.Errorf("failed to list connections: %w", err)
			}
			if len(ids) == 0 {
				return nil, testReq, fmt.Errorf("no connections found in project %s", projectID)
			}
			options := make([]huh.Option[string], len(ids))
			for i, id := range ids {
				options[i] = huh.NewOption(labels[i], id)
			}
			if err := cmdutil.Select(f.Printer, "Connection", options, &connID); err != nil {
				return nil, testReq, err
			}
		}
		props = append(props,
			api.Property{Name: "authMethod", Value: "PASSWORD"},
			api.Property{Name: "pipelines.connectionId", Value: connID},
		)
		testReq.ConnectionID = connID

	case authAnonymous:
		props = append(props,
			api.Property{Name: "authMethod", Value: "ANONYMOUS"},
		)

	default:
		return nil, testReq, fmt.Errorf("unknown auth method: %s", authMethod)
	}

	return props, testReq, nil
}

func readStdin(f *cmdutil.Factory) (string, error) {
	data, err := io.ReadAll(f.IOStreams.In)
	if err != nil {
		return "", fmt.Errorf("failed to read from stdin: %w", err)
	}
	return strings.TrimRight(string(data), "\r\n"), nil
}

func vcsNameFromURL(repoURL string) string {
	if strings.Contains(repoURL, "@") && !strings.Contains(repoURL, "://") {
		if _, after, ok := strings.Cut(repoURL, ":"); ok {
			return strings.TrimSuffix(after, ".git")
		}
	}

	if parsed, err := url.Parse(repoURL); err == nil && parsed.Host != "" {
		path := strings.TrimPrefix(parsed.Path, "/")
		return strings.TrimSuffix(path, ".git")
	}

	return repoURL
}

func isSSHURL(repoURL string) bool {
	if strings.HasPrefix(repoURL, "ssh://") {
		return true
	}
	return strings.Contains(repoURL, "@") && !strings.Contains(repoURL, "://")
}

func inferAuthFromURL(repoURL string) string {
	if isSSHURL(repoURL) {
		return authSSHKey
	}
	return authAnonymous
}

func newVcsTestCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "test <vcs-root-id>",
		Short:   "Test a VCS root connection",
		Long:    `Test the connection for an existing VCS root.`,
		Args:    cobra.ExactArgs(1),
		Example: `  teamcity project vcs test MyProject_GitHubRepo`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runVcsTest(f, args[0])
		},
	}

	return cmd
}

func runVcsTest(f *cmdutil.Factory, id string) error {
	client, err := f.Client()
	if err != nil {
		return err
	}

	if !client.SupportsFeature("vcs_test_connection") {
		return errors.New("connection testing requires TeamCity 2024.12 or later")
	}

	root, err := client.GetVcsRoot(id)
	if err != nil {
		return err
	}

	projectID := "_Root"
	if root.Project != nil {
		projectID = root.Project.ID
	}

	req := buildTestRequestFromRoot(root)
	if err := runConnectionTest(f, client, req, projectID); err != nil {
		return err
	}
	f.Printer.Success("Connection to %q is working", root.Name)
	return nil
}

func runConnectionTest(f *cmdutil.Factory, client api.ClientInterface, req api.TestConnectionRequest, projectID string) error {
	_, _ = fmt.Fprint(f.Printer.ErrOut, "Testing connection... ")
	result, err := client.TestVcsConnection(req, projectID)
	if err != nil {
		_, _ = fmt.Fprintln(f.Printer.ErrOut, output.Red("✗"))
		return fmt.Errorf("connection test failed: %w", err)
	}
	if result.Status != "OK" {
		_, _ = fmt.Fprintln(f.Printer.ErrOut, output.Red("✗"))
		msg := "connection test failed"
		if len(result.Errors) > 0 {
			msg = result.Errors[0].Message
		}
		return fmt.Errorf("%s", msg)
	}
	_, _ = fmt.Fprintln(f.Printer.ErrOut, output.Green("✓"))
	return nil
}

func buildTestRequestFromRoot(root *api.VcsRoot) api.TestConnectionRequest {
	req := api.TestConnectionRequest{
		VcsName: root.VcsName,
	}

	if root.Properties == nil {
		return req
	}

	var authMethod string
	for _, p := range root.Properties.Property {
		switch p.Name {
		case "url":
			req.URL = p.Value
		case "authMethod":
			authMethod = p.Value
		case "username":
			req.Username = p.Value
		case "teamcitySshKey":
			req.SSHKey = &api.SSHKeyRef{Name: p.Value}
		case "pipelines.connectionId":
			req.ConnectionID = p.Value
		}
	}

	switch authMethod {
	case "TEAMCITY_SSH_KEY", "PRIVATE_KEY_DEFAULT", "PRIVATE_KEY_FILE":
		req.IsPrivate = true
	}

	return req
}

type vcsDeleteOptions struct {
	yes bool
}

func newVcsDeleteCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &vcsDeleteOptions{}

	cmd := &cobra.Command{
		Use:     "delete <vcs-root-id>",
		Short:   "Delete a VCS root",
		Aliases: []string{"rm"},
		Args:    cobra.ExactArgs(1),
		Example: `  teamcity project vcs delete MyProject_GitHubRepo
  teamcity project vcs delete MyProject_GitHubRepo --yes`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runVcsDelete(f, args[0], opts)
		},
	}

	cmd.Flags().BoolVarP(&opts.yes, "yes", "y", false, "Skip confirmation prompt")
	cmd.Flags().BoolVarP(&opts.yes, "force", "f", false, "")
	cmdutil.DeprecateFlag(cmd, "force", "yes", "v1.0.0")

	return cmd
}

func runVcsDelete(f *cmdutil.Factory, id string, opts *vcsDeleteOptions) error {
	client, err := f.Client()
	if err != nil {
		return err
	}

	if !opts.yes && f.IsInteractive() {
		var confirm bool
		if err := cmdutil.Confirm(fmt.Sprintf("Delete VCS root %q?", id), &confirm); err != nil {
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
