// Package link implements `teamcity link`: upsert a [[server]] entry (or a
// per-path scope inside one) in teamcity.toml.
package link

import (
	"cmp"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/completion"
	"github.com/JetBrains/teamcity-cli/internal/config"
	"github.com/JetBrains/teamcity-cli/internal/git"
	"github.com/JetBrains/teamcity-cli/internal/link"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/spf13/cobra"
)

func NewCmd(f *cmdutil.Factory) *cobra.Command {
	var server, project, job, scope string
	var jobs []string

	cmd := &cobra.Command{
		Use:   "link",
		Short: "Bind this repository to a TeamCity project",
		Long: `Upsert a [[server]] entry in teamcity.toml binding this repo to a TeamCity
instance. Per-path scopes (monorepo) are upserted under [server.paths."<path>"].

Resolution cascade (highest to lowest):
  --flag → TEAMCITY_* env → matching [[server]] entry, deepest matching path scope`,
		Example: `  # Bind the repo (uses active server, top-level scope)
  teamcity link --project Acme_Backend --job Acme_Backend_Build

  # Add a second server's pipelines to the same teamcity.toml
  teamcity link --server https://nightly.example --project Acme_Nightly \
      --jobs Acme_Nightly_Release,Acme_Nightly_Eval

  # Path-scoped: cwd relative to teamcity.toml's dir is the implicit scope
  cd services/api && teamcity link --project Acme_API --job Acme_API_Build

  # Inspect or remove the file directly:
  cat teamcity.toml
  rm  teamcity.toml`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			serverURL := config.NormalizeURL(cmp.Or(server, config.GetServerURL()))
			if serverURL == "" {
				return api.Validation(
					"--server is required when no active TeamCity server is configured",
					"Pass --server <url> or run 'teamcity auth login' first",
				)
			}
			if project == "" && job == "" && len(jobs) == 0 {
				return api.Validation(
					"at least one of --project, --job, or --jobs is required",
					"Pass --project <id> (and optionally --job <id> or --jobs A,B,C)",
				)
			}

			path, err := writePath()
			if err != nil {
				return err
			}
			scopePath, err := resolveScopePath(scope, cmd.Flags().Changed("scope"), path)
			if err != nil {
				return err
			}

			cfg, err := loadOrEmpty(path)
			if err != nil {
				return fmt.Errorf("read %s: %w", path, err)
			}
			cfg.UpsertScope(serverURL, scopePath, link.PathScope{
				Project: project,
				Job:     job,
				Jobs:    jobs,
			})
			if err := link.Save(path, cfg); err != nil {
				return fmt.Errorf("write %s: %w", path, err)
			}

			label := scopePath
			if label == "" {
				label = "(top-level)"
			}
			f.Printer.Success("Linked %s — %s", output.Cyan(serverURL), label)
			if project != "" {
				f.Printer.Info("  Project: %s", project)
			}
			if job != "" {
				f.Printer.Info("  Default job: %s", job)
			}
			if len(jobs) > 0 {
				f.Printer.Info("  Jobs: %s", strings.Join(jobs, ", "))
			}
			f.Printer.Info("  Wrote: %s", path)
			return nil
		},
	}

	cmd.Flags().StringVar(&server, "server", "", "TeamCity server URL (default: active server)")
	cmd.Flags().StringVarP(&project, "project", "p", "", "TeamCity project ID for this scope")
	cmd.Flags().StringVarP(&job, "job", "j", "", "Default job/pipeline ID for this scope")
	cmd.Flags().StringSliceVar(&jobs, "jobs", nil, "Additional job/pipeline IDs (comma-separated or repeated)")
	cmd.Flags().StringVar(&scope, "scope", "", "Path scope inside the server entry (default: cwd relative to teamcity.toml's dir; pass --scope= for top-level)")

	_ = cmd.RegisterFlagCompletionFunc("server", completion.ConfiguredServers())
	_ = cmd.RegisterFlagCompletionFunc("project", completion.LinkedProjects())
	_ = cmd.RegisterFlagCompletionFunc("job", completion.LinkedJobs())
	_ = cmd.RegisterFlagCompletionFunc("jobs", completion.LinkedJobs())
	_ = cmd.RegisterFlagCompletionFunc("scope", completion.LinkScopes())

	return cmd
}

// loadOrEmpty returns the parsed config (empty if path doesn't exist); other errors propagate so we don't overwrite a malformed file.
func loadOrEmpty(path string) (*link.Config, error) {
	c, err := link.Load(path)
	if err == nil {
		return c, nil
	}
	if os.IsNotExist(err) {
		return &link.Config{}, nil
	}
	return nil, err
}

// resolveScopePath returns the path key for the upsert: explicit --scope wins; otherwise cwd-rel-to-toml-dir. Cleans dot-segments and rejects paths that escape the toml's directory so the stored key matches what Server.Resolve later looks up.
func resolveScopePath(override string, overrideSet bool, tomlPath string) (string, error) {
	if overrideSet {
		cleaned := filepath.ToSlash(filepath.Clean(override))
		if cleaned == "." {
			return "", nil
		}
		if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
			return "", api.Validation(
				fmt.Sprintf("--scope %q escapes teamcity.toml's directory", override),
				"Pass a path inside the toml's directory, or use --scope= for top-level",
			)
		}
		return strings.TrimPrefix(cleaned, "/"), nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return link.RelPath(filepath.Dir(tomlPath), cwd), nil
}

// writePath chooses where teamcity.toml goes: existing file wins, else git root, else cwd. Resolves symlinks so a symlinked entry into a repo still finds the real root.
func writePath() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	if path, ok := link.Find(cwd); ok {
		return path, nil
	}
	resolved := cwd
	if r, err := filepath.EvalSymlinks(cwd); err == nil {
		resolved = r
	}
	if root, ok := git.RepoRoot(resolved); ok {
		return filepath.Join(root, link.FileName), nil
	}
	return filepath.Join(cwd, link.FileName), nil
}
