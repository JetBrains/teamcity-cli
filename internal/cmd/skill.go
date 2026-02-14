package cmd

import (
	"fmt"

	teamcitycli "github.com/JetBrains/teamcity-cli"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/spf13/cobra"
	"github.com/tiulpin/instill"
)

func newSkillCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skill",
		Short: "Manage AI coding agent skills",
		Long:  "Install, update, or remove the teamcity-cli skill for AI coding agents (Claude Code, Cursor, etc.).",
		Args:  cobra.NoArgs,
		RunE:  subcommandRequired,
	}

	cmd.AddCommand(newSkillInstallCmd())
	cmd.AddCommand(newSkillUpdateCmd())
	cmd.AddCommand(newSkillRemoveCmd())

	return cmd
}

type skillOptions struct {
	agents  []string
	project bool
}

func newSkillInstallCmd() *cobra.Command {
	opts := &skillOptions{}

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install the teamcity-cli skill for AI coding agents",
		Long: `Install the teamcity-cli skill so AI coding agents can use tc commands.

Installs globally by default. Use --project to install to the current project only.
Auto-detects installed agents when --agent is not specified.`,
		Example: `  tc skill install
  tc skill install --agent claude-code --agent cursor
  tc skill install --project`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSkillInstall(opts, false)
		},
	}

	addSkillFlags(cmd, opts)
	return cmd
}

func newSkillUpdateCmd() *cobra.Command {
	opts := &skillOptions{}

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update the teamcity-cli skill for AI coding agents",
		Long: `Update the teamcity-cli skill to the latest version bundled with this tc release.

Skips if the installed version already matches.
Auto-detects installed agents when --agent is not specified.`,
		Example: `  tc skill update
  tc skill update --agent claude-code
  tc skill update --project`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSkillInstall(opts, true)
		},
	}

	addSkillFlags(cmd, opts)
	return cmd
}

func addSkillFlags(cmd *cobra.Command, opts *skillOptions) {
	cmd.Flags().StringSliceVarP(&opts.agents, "agent", "a", nil, "Target agent(s); auto-detects if omitted")
	cmd.Flags().BoolVar(&opts.project, "project", false, "Install to current project instead of globally")
}

func runSkillInstall(opts *skillOptions, checkVersion bool) error {
	agents, err := resolveSkillAgents(opts.agents, !opts.project)
	if err != nil {
		return err
	}

	instillOpts := instill.Options{
		Agents:     agents,
		ProjectDir: ".",
		Global:     !opts.project,
	}

	bundled := instill.SkillVersion(teamcitycli.SkillsFS)

	if checkVersion && bundled != "" {
		installed, err := instill.InstalledVersion("teamcity-cli", instillOpts)
		if err != nil {
			return err
		}
		if installed == bundled {
			output.Success("Already up to date (%s)", bundled)
			return nil
		}
	}

	results, err := instill.Install(teamcitycli.SkillsFS, instillOpts)
	if err != nil {
		return err
	}

	for _, r := range results {
		switch {
		case !r.Existed:
			output.Success("Installed for %s (%s)", r.Agent, bundled)
		case r.PriorVersion != "" && r.PriorVersion != bundled:
			output.Success("Updated for %s (%s → %s)", r.Agent, r.PriorVersion, bundled)
		case r.PriorVersion == "":
			output.Success("Updated for %s (unversioned → %s)", r.Agent, bundled)
		default:
			output.Success("Reinstalled for %s (%s)", r.Agent, bundled)
		}
	}
	return nil
}

func newSkillRemoveCmd() *cobra.Command {
	opts := &skillOptions{}

	cmd := &cobra.Command{
		Use:   "remove",
		Short: "Remove the teamcity-cli skill from AI coding agents",
		Example: `  tc skill remove
  tc skill remove --agent claude-code
  tc skill remove --project`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSkillRemove(opts)
		},
	}

	addSkillFlags(cmd, opts)
	return cmd
}

func runSkillRemove(opts *skillOptions) error {
	agents, err := resolveSkillAgents(opts.agents, !opts.project)
	if err != nil {
		return err
	}

	results, err := instill.Remove("teamcity-cli", instill.Options{
		Agents:     agents,
		ProjectDir: ".",
		Global:     !opts.project,
	})
	if err != nil {
		return err
	}

	for _, r := range results {
		if r.Existed {
			output.Success("Removed from %s", r.Agent)
		} else {
			output.Info("Not installed for %s, nothing to remove", r.Agent)
		}
	}
	return nil
}

func resolveSkillAgents(explicit []string, global bool) ([]string, error) {
	if len(explicit) > 0 {
		return explicit, nil
	}

	detected, err := instill.Detect(".", global)

	if err != nil {
		return nil, fmt.Errorf("detecting agents: %w", err)
	}
	if len(detected) == 0 {
		return nil, fmt.Errorf("no AI coding agents detected; use --agent to specify one (available: %s)",
			formatAgentList())
	}

	names := make([]string, len(detected))
	for i, a := range detected {
		names[i] = a.Name
	}
	return names, nil
}

func formatAgentList() string {
	names := instill.AgentNames()
	if len(names) > 5 {
		return fmt.Sprintf("%s, %s, %s, ... (%d total)",
			names[0], names[1], names[2], len(names))
	}
	return fmt.Sprintf("%v", names)
}
