// Package completion exposes shell-completion helpers backed by static enums and on-disk config.
package completion

import (
	"maps"
	"os"
	"slices"

	teamcitycli "github.com/JetBrains/teamcity-cli"
	"github.com/JetBrains/teamcity-cli/internal/config"
	"github.com/JetBrains/teamcity-cli/internal/git"
	"github.com/JetBrains/teamcity-cli/internal/link"
	"github.com/spf13/cobra"
	"github.com/tiulpin/instill"
)

// CompFunc matches cobra's ValidArgsFunction and RegisterFlagCompletionFunc signature.
type CompFunc func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective)

// Fixed returns a CompFunc serving values with file completion suppressed.
func Fixed(values ...string) CompFunc {
	return cobra.FixedCompletions(values, cobra.ShellCompDirectiveNoFileComp)
}

// RunStatuses completes `run list --status`; mirrors resolveRunListStatus.
func RunStatuses() CompFunc {
	return Fixed("success", "failure", "running", "queued", "error", "unknown", "canceled")
}

// JobTreeOnly completes `job tree --only`.
func JobTreeOnly() CompFunc {
	return Fixed("dependents", "dependencies")
}

// HTTPMethods completes `api -X/--method`.
func HTTPMethods() CompFunc {
	return Fixed("GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS")
}

// SSHKeyTypes completes `project ssh generate --type`.
func SSHKeyTypes() CompFunc {
	return Fixed("ed25519", "rsa")
}

// VCSAuthMethods completes `project vcs create --auth`.
func VCSAuthMethods() CompFunc {
	return Fixed("password", "ssh-key", "ssh-agent", "ssh-file", "token", "anonymous")
}

// ConfigKeys completes `config get|set <key>` from config.ValidKeys.
func ConfigKeys() CompFunc {
	return func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return config.ValidKeys(), cobra.ShellCompDirectiveNoFileComp
	}
}

// ConfiguredServers completes server URLs from the local config file.
func ConfiguredServers() CompFunc {
	return func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return config.SortedServerURLs(config.Get()), cobra.ShellCompDirectiveNoFileComp
	}
}

// AliasNames completes user-defined alias names from local config.
func AliasNames() CompFunc {
	return func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return slices.Collect(maps.Keys(config.GetAllAliases())), cobra.ShellCompDirectiveNoFileComp
	}
}

// SkillNames completes skill names from the bundled skills FS.
func SkillNames() CompFunc {
	return func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		skills := instill.ListSkills(teamcitycli.SkillsFS)
		names := make([]string, len(skills))
		for i, s := range skills {
			names[i] = s.Name
		}
		return names, cobra.ShellCompDirectiveNoFileComp
	}
}

// SkillAgents completes `skill --agent` from instill.AgentNames.
func SkillAgents() CompFunc {
	return func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return instill.AgentNames(), cobra.ShellCompDirectiveNoFileComp
	}
}

// LinkedJobs completes job IDs from teamcity.toml in the cwd's ancestry.
func LinkedJobs() CompFunc {
	return func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		cfg := loadLinkConfig()
		if cfg == nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		var jobs []string
		for _, s := range cfg.Servers {
			if s.Job != "" {
				jobs = append(jobs, s.Job)
			}
			jobs = append(jobs, s.Jobs...)
			for _, p := range s.Paths {
				if p.Job != "" {
					jobs = append(jobs, p.Job)
				}
				jobs = append(jobs, p.Jobs...)
			}
		}
		return dedupe(jobs), cobra.ShellCompDirectiveNoFileComp
	}
}

// LinkedProjects completes project IDs from teamcity.toml; always includes _Root.
func LinkedProjects() CompFunc {
	return func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		out := []string{"_Root"}
		cfg := loadLinkConfig()
		if cfg != nil {
			for _, s := range cfg.Servers {
				if s.Project != "" {
					out = append(out, s.Project)
				}
				for _, p := range s.Paths {
					if p.Project != "" {
						out = append(out, p.Project)
					}
				}
			}
		}
		return dedupe(out), cobra.ShellCompDirectiveNoFileComp
	}
}

// LinkScopes completes `link --scope` from existing path scopes in teamcity.toml.
func LinkScopes() CompFunc {
	return func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		cfg := loadLinkConfig()
		if cfg == nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		var scopes []string
		for _, s := range cfg.Servers {
			scopes = slices.AppendSeq(scopes, maps.Keys(s.Paths))
		}
		return dedupe(scopes), cobra.ShellCompDirectiveNoFileComp
	}
}

// GitBranches completes branch flags from local refs/heads, prefixed with @this.
func GitBranches() CompFunc {
	return func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		out := []string{"@this"}
		out = append(out, git.LocalBranches()...)
		return out, cobra.ShellCompDirectiveNoFileComp
	}
}

// AtMe completes `run list --user` with the @me token.
func AtMe() CompFunc {
	return Fixed("@me")
}

// AtHead completes `--revision` with the @head token.
func AtHead() CompFunc {
	return Fixed("@head")
}

func loadLinkConfig() *link.Config {
	cwd, err := os.Getwd()
	if err != nil {
		return nil
	}
	path, ok := link.Find(cwd)
	if !ok {
		return nil
	}
	cfg, err := link.Load(path)
	if err != nil {
		return nil
	}
	return cfg
}

func dedupe(in []string) []string {
	out := slices.DeleteFunc(slices.Clone(in), func(s string) bool { return s == "" })
	slices.Sort(out)
	return slices.Compact(out)
}
