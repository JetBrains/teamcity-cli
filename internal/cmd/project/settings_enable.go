package project

import (
	"errors"
	"fmt"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/completion"
	"github.com/spf13/cobra"
)

func newProjectSettingsEnableCmd(f *cmdutil.Factory) *cobra.Command {
	var vcsRoot, format, settingsPath string
	var asJSON bool
	cmd := &cobra.Command{
		Use: "enable <project-id>", Short: "Enable versioned settings by importing from VCS",
		Long: `Enable versioned settings for a project that has no existing settings format.
Imports configuration from the selected VCS root and keeps UI editing enabled.
Does not overwrite the settings repository with the project's current configuration.
Existing versioned settings are never disabled automatically; use the TeamCity UI
for reconfiguration. Import completion and missing DSL context parameters can be
checked with project settings status.`,
		Example: "  teamcity project settings enable MyProject --vcs-root MyProject_Settings --format kotlin",
		Args:    cobra.ExactArgs(1), ValidArgsFunction: completion.LinkedProjects(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if format != "kotlin" && format != "xml" {
				return api.Validation("invalid settings format", "Use --format kotlin or --format xml")
			}
			if vcsRoot == "" {
				return api.RequiredFlag("vcs-root")
			}
			client, err := f.Client()
			if err != nil {
				return err
			}
			current, err := client.GetVersionedSettingsConfig(args[0])
			if err != nil {
				return fmt.Errorf("failed to get versioned settings configuration: %w", err)
			}
			if current.Format != "" || current.SynchronizationMode == "enabled" {
				return api.Validation("project already has versioned settings configured", "Reconfigure versioned settings in the TeamCity UI; existing settings were not disabled")
			}
			enabler, ok := client.(interface {
				EnableVersionedSettings(string, string, string, string) (*api.VersionedSettingsConfig, error)
			})
			if !ok {
				return errors.New("client does not support enabling versioned settings")
			}
			result, err := enabler.EnableVersionedSettings(args[0], vcsRoot, format, settingsPath)
			if err != nil {
				return fmt.Errorf("failed to enable versioned settings: %w", err)
			}
			if asJSON {
				return f.Printer.PrintJSON(result)
			}
			f.Printer.Success("Versioned settings import requested for %s", args[0])
			f.Printer.Tip("Check progress with teamcity project settings status %s", args[0])
			return nil
		},
	}
	cmd.Flags().StringVar(&vcsRoot, "vcs-root", "", "VCS root containing the project settings (required)")
	cmd.Flags().StringVar(&format, "format", "kotlin", "Settings format: kotlin or xml")
	cmd.Flags().StringVar(&settingsPath, "settings-path", ".teamcity", "Settings directory in the repository")
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output as JSON")
	return cmd
}
