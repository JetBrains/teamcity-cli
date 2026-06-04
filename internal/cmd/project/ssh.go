package project

import (
	"cmp"
	"fmt"
	"os"
	"strings"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/completion"
	"github.com/JetBrains/teamcity-cli/internal/config"
	"github.com/spf13/cobra"
)

func newSSHCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ssh",
		Short: "Manage SSH keys",
		Long: `List, upload, generate, and delete SSH keys in a project.

SSH keys uploaded to a project can be used by VCS roots and build
steps to authenticate with remote services without exposing private
keys in configuration or source control.

See: https://www.jetbrains.com/help/teamcity/ssh-keys-management.html`,
		Args: cobra.NoArgs,
		RunE: cmdutil.SubcommandRequired,
	}

	cmd.AddCommand(newSSHListCmd(f))
	cmd.AddCommand(newSSHUploadCmd(f))
	cmd.AddCommand(newSSHGenerateCmd(f))
	cmd.AddCommand(newSSHDeleteCmd(f))

	return cmd
}

type sshListOptions struct {
	project string
	cmdutil.ListFlags
	cmdutil.ViewOptions
}

func newSSHListCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &sshListOptions{}

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List SSH keys",
		Aliases: []string{"ls"},
		Example: `  teamcity project ssh list
  teamcity project ssh list --project MyProject
  teamcity project ssh list --project MyProject --json
  teamcity project ssh list --plain
  teamcity project ssh list --project MyProject --web`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.Web {
				if err := cmdutil.ValidateLimit(opts.Limit); err != nil {
					return err
				}
			}
			path := "/admin/editProject.html?projectId=" + cmp.Or(opts.project, "_Root") + "&tab=ssh-manager"
			if done, err := opts.EmitListWebURL(f.Printer, config.ResolveServerURL(), path); done {
				return err
			}
			return cmdutil.RunList(f, cmd, &opts.ListFlags, &api.SSHKeyFields, opts.fetch)
		},
	}

	cmd.Flags().StringVarP(&opts.project, "project", "p", "", "Project ID (default: _Root)")
	cmdutil.AddListFlags(cmd, &opts.ListFlags, 100)
	cmdutil.AddWebFlags(cmd, &opts.ViewOptions)

	_ = cmd.RegisterFlagCompletionFunc("project", completion.LinkedProjects())

	return cmd
}

func (opts *sshListOptions) fetch(client api.ClientInterface, fields []string) (*cmdutil.ListResult, error) {
	keys, err := client.GetSSHKeys(cmp.Or(opts.project, "_Root"))
	if err != nil {
		return nil, err
	}

	items := keys.SSHKey
	truncated := opts.Limit > 0 && opts.Limit < len(items)
	if truncated {
		items = items[:opts.Limit]
	}

	headers := []string{"NAME", "ENCRYPTED", "PUBLIC KEY"}
	var rows [][]string
	for _, k := range items {
		encrypted := "no"
		if k.Encrypted {
			encrypted = "yes"
		}
		rows = append(rows, []string{k.Name, encrypted, k.PublicKey})
	}

	return &cmdutil.ListResult{
		JSON:      filterJSONList(items, fields, sshKeyToMap),
		Table:     cmdutil.ListTable{Headers: headers, Rows: rows, FlexCols: []int{0, 2}},
		EmptyMsg:  "No SSH keys found",
		Truncated: truncated,
	}, nil
}

func sshKeyToMap(k api.SSHKey) map[string]any {
	return map[string]any{
		"name":      k.Name,
		"encrypted": k.Encrypted,
		"publicKey": k.PublicKey,
	}
}

type sshUploadOptions struct {
	project string
	name    string
}

func newSSHUploadCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &sshUploadOptions{}

	cmd := &cobra.Command{
		Use:   "upload <file>",
		Short: "Upload an SSH private key",
		Example: `  teamcity project ssh upload ~/.ssh/id_ed25519 --project MyProject
  teamcity project ssh upload key.pem --name my-deploy-key --project MyProject`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSSHUpload(f, args[0], opts)
		},
	}

	cmd.Flags().StringVarP(&opts.project, "project", "p", "", "Project ID")
	cmd.Flags().StringVar(&opts.name, "name", "", "Key name (default: filename)")

	_ = cmd.RegisterFlagCompletionFunc("project", completion.LinkedProjects())

	return cmd
}

func runSSHUpload(f *cmdutil.Factory, filePath string, opts *sshUploadOptions) error {
	client, err := f.Client()
	if err != nil {
		return err
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read key file: %w", err)
	}

	projectID, err := resolveProject(f, opts.project, api.PermissionEditProject)
	if err != nil {
		return err
	}
	name := opts.name
	if name == "" {
		name = baseName(filePath)
	}

	if err := client.UploadSSHKey(projectID, name, data); err != nil {
		return fmt.Errorf("failed to upload SSH key: %w", err)
	}

	f.Printer.Success("Uploaded SSH key %q to project %s", name, projectID)
	return nil
}

func baseName(path string) string {
	i := strings.LastIndexAny(path, "/\\")
	if i >= 0 {
		return path[i+1:]
	}
	return path
}

type sshGenerateOptions struct {
	project string
	name    string
	keyType string
}

func newSSHGenerateCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &sshGenerateOptions{}

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate an SSH key pair",
		Long:  `Generate an SSH key pair in TeamCity and print the public key.`,
		Example: `  teamcity project ssh generate --name deploy-key --project MyProject
  teamcity project ssh generate --name deploy-key --type rsa --project MyProject`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSSHGenerate(f, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.project, "project", "p", "", "Project ID")
	cmd.Flags().StringVar(&opts.name, "name", "", "Key name (required)")
	cmd.Flags().StringVar(&opts.keyType, "type", "ed25519", "Key type: ed25519 or rsa")

	_ = cmd.RegisterFlagCompletionFunc("type", completion.SSHKeyTypes())
	_ = cmd.RegisterFlagCompletionFunc("project", completion.LinkedProjects())

	return cmd
}

func runSSHGenerate(f *cmdutil.Factory, opts *sshGenerateOptions) error {
	client, err := f.Client()
	if err != nil {
		return err
	}

	if f.IsInteractive() {
		if err := runInteractiveForm(f, &opts.project, api.PermissionEditProject, formField{title: "Key name", value: &opts.name}); err != nil {
			return err
		}
	}
	if opts.project == "" {
		return api.RequiredFlag("project")
	}
	if opts.name == "" {
		return api.RequiredFlag("name")
	}

	key, err := client.GenerateSSHKey(opts.project, opts.name, opts.keyType)
	if err != nil {
		return fmt.Errorf("failed to generate SSH key: %w", err)
	}

	f.Printer.Success("Generated SSH key %q in project %s", key.Name, opts.project)
	_, _ = fmt.Fprintln(f.Printer.Out)
	_, _ = fmt.Fprintln(f.Printer.Out, key.PublicKey)
	return nil
}

type sshDeleteOptions struct {
	project string
	yes     bool
}

func newSSHDeleteCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &sshDeleteOptions{}

	cmd := &cobra.Command{
		Use:     "delete <name>",
		Short:   "Delete an SSH key",
		Aliases: []string{"rm"},
		Args:    cobra.ExactArgs(1),
		Example: `  teamcity project ssh delete my-deploy-key --project MyProject
  teamcity project ssh delete my-deploy-key --project MyProject --yes`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSSHDelete(f, args[0], opts)
		},
	}

	cmd.Flags().StringVarP(&opts.project, "project", "p", "", "Project ID")
	cmd.Flags().BoolVarP(&opts.yes, "yes", "y", false, "Skip confirmation prompt")

	_ = cmd.RegisterFlagCompletionFunc("project", completion.LinkedProjects())

	return cmd
}

func runSSHDelete(f *cmdutil.Factory, name string, opts *sshDeleteOptions) error {
	client, err := f.Client()
	if err != nil {
		return err
	}

	projectID, err := resolveProject(f, opts.project, api.PermissionEditProject)
	if err != nil {
		return err
	}

	if !opts.yes && f.IsInteractive() {
		var confirm bool
		if err := cmdutil.Confirm(fmt.Sprintf("Delete SSH key %q from project %s?", name, projectID), &confirm); err != nil {
			return err
		}
		if !confirm {
			f.Printer.Info("Canceled")
			return nil
		}
	}

	if err := client.DeleteSSHKey(projectID, name); err != nil {
		return fmt.Errorf("failed to delete SSH key: %w", err)
	}

	f.Printer.Success("Deleted SSH key %q from project %s", name, projectID)
	return nil
}

// sshKeyNames fetches SSH key names for a project (used by vcs create wizard)
func sshKeyNames(client api.ClientInterface, projectID string) ([]string, error) {
	keys, err := client.GetSSHKeys(projectID)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, k := range keys.SSHKey {
		names = append(names, k.Name)
	}
	return names, nil
}
