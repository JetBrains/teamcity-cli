package skill

import (
	"fmt"

	teamcitycli "github.com/JetBrains/teamcity-cli"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/spf13/cobra"
	"github.com/tiulpin/instill"
)

const defaultSkill = "teamcity-cli"

func NewCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skill",
		Short: "Manage AI coding agent skills",
		Long:  "Install, update, or remove TeamCity skills for AI coding agents (Claude Code, Cursor, etc.).",
		Args:  cobra.NoArgs,
		RunE:  cmdutil.SubcommandRequired,
	}

	cmd.AddCommand(newSkillListCmd(f))
	cmd.AddCommand(newSkillInstallCmd(f))
	cmd.AddCommand(newSkillUpdateCmd(f))
	cmd.AddCommand(newSkillRemoveCmd(f))

	return cmd
}

type skillOptions struct {
	agents  []string
	all     bool
	project bool
}

func newSkillListCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List available skills bundled with this release",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			skills := instill.ListSkills(teamcitycli.SkillsFS)
			if len(skills) == 0 {
				f.Printer.Info("No skills bundled")
				return nil
			}
			rows := make([][]string, len(skills))
			for i, s := range skills {
				def := ""
				if s.Name == defaultSkill {
					def = "(default)"
				}
				rows[i] = []string{s.Name, s.Version, s.Description, def}
			}
			f.Printer.PrintTable([]string{"Name", "Version", "Description", ""}, rows)
			return nil
		},
	}
}

func newSkillInstallCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &skillOptions{}

	cmd := &cobra.Command{
		Use:   "install [skill-name...]",
		Short: "Install skills for AI coding agents",
		Long: `Install TeamCity skills so AI coding agents can use teamcity commands.

When no skill name is given, installs the default skill (teamcity-cli).
Use --all to install every bundled skill.
Installs globally by default. Use --project to install to the current project only.
Auto-detects installed agents when --agent is not specified.`,
		Example: `  teamcity skill install
  teamcity skill install teamcity-cli
  teamcity skill install --all
  teamcity skill install --agent claude-code --agent cursor
  teamcity skill install --project`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSkillInstall(f.Printer, opts, args, false)
		},
	}

	addSkillFlags(cmd, opts)
	return cmd
}

func newSkillUpdateCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &skillOptions{}

	cmd := &cobra.Command{
		Use:   "update [skill-name...]",
		Short: "Update skills for AI coding agents",
		Long: `Update TeamCity skills to the latest version bundled with this release.

When no skill name is given, updates the default skill (teamcity-cli).
Use --all to update every bundled skill.
Skips if the installed version already matches.
Auto-detects installed agents when --agent is not specified.`,
		Example: `  teamcity skill update
  teamcity skill update --all
  teamcity skill update --agent claude-code
  teamcity skill update --project`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSkillInstall(f.Printer, opts, args, true)
		},
	}

	addSkillFlags(cmd, opts)
	return cmd
}

func addSkillFlags(cmd *cobra.Command, opts *skillOptions) {
	cmd.Flags().StringSliceVarP(&opts.agents, "agent", "a", nil, "Target agent(s); auto-detects if omitted")
	cmd.Flags().BoolVar(&opts.all, "all", false, "Install/update all bundled skills")
	cmd.Flags().BoolVar(&opts.project, "project", false, "Install to current project instead of globally")
}

func runSkillInstall(p *output.Printer, opts *skillOptions, args []string, checkVersion bool) error {
	agents, err := resolveSkillAgents(opts.agents, !opts.project)
	if err != nil {
		return err
	}

	names, err := resolveSkillNames(opts.all, args)
	if err != nil {
		return err
	}

	versions := bundledVersions()
	for _, name := range names {
		if _, ok := versions[name]; !ok {
			return fmt.Errorf("unknown skill %q; run 'teamcity skill list' to see available skills", name)
		}
	}

	instillOpts := instill.Options{
		Agents:     agents,
		Skills:     names,
		ProjectDir: ".",
		Global:     !opts.project,
	}

	if checkVersion {
		allUpToDate := true
		for _, name := range names {
			bundled := versions[name]
			if bundled == "" {
				allUpToDate = false
				break
			}
			installed, err := instill.InstalledVersion(name, instillOpts)
			if err != nil {
				return err
			}
			if installed != bundled {
				allUpToDate = false
				break
			}
		}
		if allUpToDate {
			for _, name := range names {
				p.Success("Skill %s already up to date (%s)", name, versions[name])
			}
			return nil
		}
	}

	results, err := instill.Install(teamcitycli.SkillsFS, instillOpts)
	if err != nil {
		return err
	}

	for _, r := range results {
		bundled := versions[r.Skill]
		switch {
		case !r.Existed:
			p.Success("Installed %s for %s (%s)", r.Skill, r.Agent, bundled)
		case r.PriorVersion != "" && r.PriorVersion != bundled:
			p.Success("Updated %s for %s (%s → %s)", r.Skill, r.Agent, r.PriorVersion, bundled)
		case r.PriorVersion == "":
			p.Success("Updated %s for %s (unversioned → %s)", r.Skill, r.Agent, bundled)
		default:
			p.Success("Reinstalled %s for %s (%s)", r.Skill, r.Agent, bundled)
		}
	}
	return nil
}

func newSkillRemoveCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &skillOptions{}

	cmd := &cobra.Command{
		Use:   "remove [skill-name...]",
		Short: "Remove skills from AI coding agents",
		Long: `Remove TeamCity skills from AI coding agents.

When no skill name is given, removes the default skill (teamcity-cli).
Use --all to remove every bundled skill.`,
		Example: `  teamcity skill remove
  teamcity skill remove --all
  teamcity skill remove --agent claude-code
  teamcity skill remove --project`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSkillRemove(f.Printer, opts, args)
		},
	}

	addSkillFlags(cmd, opts)
	return cmd
}

func runSkillRemove(p *output.Printer, opts *skillOptions, args []string) error {
	agents, err := resolveSkillAgents(opts.agents, !opts.project)
	if err != nil {
		return err
	}

	names, err := resolveSkillNames(opts.all, args)
	if err != nil {
		return err
	}

	versions := bundledVersions()
	for _, name := range names {
		if _, ok := versions[name]; !ok {
			return fmt.Errorf("unknown skill %q; run 'teamcity skill list' to see available skills", name)
		}

		results, err := instill.Remove(name, instill.Options{
			Agents:     agents,
			ProjectDir: ".",
			Global:     !opts.project,
		})
		if err != nil {
			return err
		}

		for _, r := range results {
			if r.Existed {
				p.Success("Removed %s from %s", name, r.Agent)
			} else {
				p.Info("Skill %s not installed for %s, nothing to remove", name, r.Agent)
			}
		}
	}
	return nil
}

func resolveSkillNames(all bool, args []string) ([]string, error) {
	if all && len(args) > 0 {
		return nil, fmt.Errorf("cannot specify both --all and skill names")
	}
	if all {
		skills := instill.ListSkills(teamcitycli.SkillsFS)
		names := make([]string, len(skills))
		for i, s := range skills {
			names[i] = s.Name
		}
		return names, nil
	}
	if len(args) == 0 {
		return []string{defaultSkill}, nil
	}
	return args, nil
}

func bundledVersions() map[string]string {
	skills := instill.ListSkills(teamcitycli.SkillsFS)
	m := make(map[string]string, len(skills))
	for _, s := range skills {
		m[s.Name] = s.Version
	}
	return m
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
