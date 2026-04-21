package project

import (
	"cmp"
	"fmt"
	"os"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
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
}

func newSSHListCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &sshListOptions{}

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List SSH keys",
		Aliases: []string{"ls"},
		Example: `  teamcity project ssh list
  teamcity project ssh list --project MyProject
  teamcity project ssh list --json
  teamcity project ssh list --plain`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.RunList(f, cmd, &opts.ListFlags, &api.SSHKeyFields, opts.fetch)
		},
	}

	cmd.Flags().StringVarP(&opts.project, "project", "p", "", "Project ID (default: _Root)")
	cmdutil.AddListFlags(cmd, &opts.ListFlags, 100)

	return cmd
}

func (opts *sshListOptions) fetch(client api.ClientInterface, fields []string) (*cmdutil.ListResult, error) {
	projectID := cmp.Or(opts.project, "_Root")
	keys, err := client.GetSSHKeys(projectID)
	if err != nil {
		return nil, err
	}

	items := keys.SSHKey
	if opts.Limit > 0 && opts.Limit < len(items) {
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
		JSON:     filterJSONList(items, fields, sshKeyToMap),
		Table:    cmdutil.ListTable{Headers: headers, Rows: rows, FlexCols: []int{0, 2}},
		EmptyMsg: "No SSH keys found",
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
		Example: `  teamcity project ssh upload ~/.ssh/id_ed25519
  teamcity project ssh upload key.pem --name my-deploy-key --project MyProject`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSSHUpload(f, args[0], opts)
		},
	}

	cmd.Flags().StringVarP(&opts.project, "project", "p", "", "Project ID (default: _Root)")
	cmd.Flags().StringVar(&opts.name, "name", "", "Key name (default: filename)")

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

	projectID := cmp.Or(opts.project, "_Root")
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
		Example: `  teamcity project ssh generate --name deploy-key
  teamcity project ssh generate --name deploy-key --type rsa --project MyProject`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSSHGenerate(f, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.project, "project", "p", "", "Project ID (default: _Root)")
	cmd.Flags().StringVar(&opts.name, "name", "", "Key name (required)")
	cmd.Flags().StringVar(&opts.keyType, "type", "ed25519", "Key type: ed25519 or rsa")

	return cmd
}

func runSSHGenerate(f *cmdutil.Factory, opts *sshGenerateOptions) error {
	client, err := f.Client()
	if err != nil {
		return err
	}

	projectID := cmp.Or(opts.project, "_Root")
	name := opts.name

	if name == "" {
		if !f.IsInteractive() {
			return api.RequiredFlag("name")
		}
		prompt := &survey.Input{Message: "Key name:"}
		if err := survey.AskOne(prompt, &name, survey.WithValidator(survey.Required)); err != nil {
			return err
		}
	}

	key, err := client.GenerateSSHKey(projectID, name, opts.keyType)
	if err != nil {
		return fmt.Errorf("failed to generate SSH key: %w", err)
	}

	f.Printer.Success("Generated SSH key %q in project %s", key.Name, projectID)
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
		Example: `  teamcity project ssh delete my-deploy-key
  teamcity project ssh delete my-deploy-key --yes`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSSHDelete(f, args[0], opts)
		},
	}

	cmd.Flags().StringVarP(&opts.project, "project", "p", "", "Project ID (default: _Root)")
	cmd.Flags().BoolVarP(&opts.yes, "yes", "y", false, "Skip confirmation prompt")
	cmd.Flags().BoolVarP(&opts.yes, "force", "f", false, "")
	cmdutil.DeprecateFlag(cmd, "force", "yes", "v1.0.0")

	return cmd
}

func runSSHDelete(f *cmdutil.Factory, name string, opts *sshDeleteOptions) error {
	client, err := f.Client()
	if err != nil {
		return err
	}

	projectID := cmp.Or(opts.project, "_Root")

	if !opts.yes && f.IsInteractive() {
		var confirm bool
		prompt := &survey.Confirm{
			Message: fmt.Sprintf("Delete SSH key %q from project %s?", name, projectID),
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
