package update

import (
	"fmt"
	"time"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/JetBrains/teamcity-cli/internal/update"
	"github.com/JetBrains/teamcity-cli/internal/version"
	"github.com/spf13/cobra"
)

type updateOptions struct {
	check bool
	yes   bool
	json  bool
}

type updateStatus struct {
	CurrentVersion  string `json:"current_version"`
	LatestVersion   string `json:"latest_version"`
	UpdateAvailable bool   `json:"update_available"`
	InstallMethod   string `json:"install_method"`
	ReleaseURL      string `json:"release_url"`
}

var latestRelease = update.LatestRelease
var detectInstallMethod = update.DetectInstallMethod
var upgrade = update.Upgrade

func NewCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &updateOptions{}
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Check for and install CLI updates",
		Long: `Check for a newer TeamCity CLI release and install it after confirmation.

Homebrew, Scoop, Winget, Chocolatey, and npm installs update through their
package manager. Unmanaged macOS/Linux binaries are updated in place after
checksum and version verification. Other methods show manual instructions.

Use --yes to install without prompting. Non-interactive invocations require
--yes to install. --check and --json only report status and never install.`,
		Args: cobra.NoArgs,
		Example: `  teamcity update
  teamcity update --yes
  teamcity update --check
  teamcity update --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpdate(f, opts)
		},
	}
	cmd.Flags().BoolVar(&opts.check, "check", false, "Check for updates without installing")
	cmd.Flags().BoolVarP(&opts.yes, "yes", "y", false, "Install without prompting")
	cmd.Flags().BoolVar(&opts.json, "json", false, "Output update status as JSON without installing")
	cmd.MarkFlagsMutuallyExclusive("yes", "check")
	cmd.MarkFlagsMutuallyExclusive("yes", "json")
	return cmd
}

func runUpdate(f *cmdutil.Factory, opts *updateOptions) error {
	p := f.Printer

	if !opts.json {
		p.Info("Checking for updates...")
	}

	release, err := latestRelease(f.Context())
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	update.SaveState(&update.State{
		LastCheckedAt: time.Now(),
		LatestVersion: release.Version,
		LatestURL:     release.URL,
	})

	method := detectInstallMethod()
	available := update.IsNewer(version.Version, release.Version)
	if opts.json {
		return p.PrintJSON(updateStatus{version.Version, release.Version, available, method.String(), release.URL})
	}
	if !available {
		p.Success("Already up to date (v%s)", version.Version)
		return nil
	}

	_, _ = fmt.Fprintf(p.Out, "%s "+output.Sym().Arrow+" %s (%s)\n",
		output.Faint("v"+version.Version),
		output.Green("v"+release.Version),
		method,
	)
	if opts.check {
		return nil
	}
	if !method.CanUpgrade() {
		p.Warn("Automatic updates are unavailable for %s. %s", method, method.UpdateCommand())
		return nil
	}
	if !opts.yes {
		if !f.IsInteractive() || !output.IsTerminal() {
			return api.Validation("confirmation required to install the update", "Run 'teamcity update --yes' to install or 'teamcity update --check' to only check")
		}
		var confirmed bool
		if err := cmdutil.Confirm("Install this update?", &confirmed); err != nil {
			return err
		}
		if !confirmed {
			p.Info("Update canceled")
			return nil
		}
	}
	output.StopSpinner()
	if err := upgrade(f.Context(), method, release.Version, f.IOStreams.Out, f.IOStreams.ErrOut); err != nil {
		return fmt.Errorf("update failed: %w", err)
	}
	p.Success("Update completed. Run 'teamcity --version' to check the installed version.")

	return nil
}
