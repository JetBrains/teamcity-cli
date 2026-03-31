package update

import (
	"context"
	"fmt"
	"time"

	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/JetBrains/teamcity-cli/internal/update"
	"github.com/JetBrains/teamcity-cli/internal/version"
	"github.com/spf13/cobra"
)

func NewCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Check for CLI updates and show how to upgrade",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpdate(f)
		},
	}
}

func runUpdate(f *cmdutil.Factory) error {
	p := f.Printer

	p.Info("Checking for updates...")

	release, err := update.LatestRelease(context.Background())
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	update.SaveState(&update.State{
		LastCheckedAt: time.Now(),
		LatestVersion: release.Version,
		LatestURL:     release.URL,
	})

	if !update.IsNewer(version.Version, release.Version) {
		p.Success("Already up to date (v%s)", version.Version)
		return nil
	}

	method := update.DetectInstallMethod()
	_, _ = fmt.Fprintf(p.Out, "%s → %s: %s\n%s\n",
		output.Faint("v"+version.Version),
		output.Green("v"+release.Version),
		output.Bold(method.UpdateCommand()),
		output.Faint(release.URL),
	)

	return nil
}
