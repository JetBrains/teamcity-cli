// Package link implements `teamcity link`: upsert a [[server]] entry (or a
// per-path scope inside one) in teamcity.toml.
package link

import (
	"cmp"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/config"
	tcerrors "github.com/JetBrains/teamcity-cli/internal/errors"
	"github.com/JetBrains/teamcity-cli/internal/link"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/spf13/cobra"
)

func NewCmd(f *cmdutil.Factory) *cobra.Command {
	var server, project, job, scope string
	var jobs []string
	var scopeSet bool

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
			scopeSet = cmd.Flags().Changed("scope")
			serverURL := cmp.Or(server, config.GetServerURL())
			if serverURL == "" {
				return tcerrors.WithSuggestion(
					"--server is required when no active TeamCity server is configured",
					"Pass --server <url> or run 'teamcity auth login' first",
				)
			}
			if project == "" && job == "" && len(jobs) == 0 {
				return tcerrors.WithSuggestion(
					"at least one of --project, --job, or --jobs is required",
					"Pass --project <id> (and optionally --job <id> or --jobs A,B,C)",
				)
			}

			path, err := writePath()
			if err != nil {
				return err
			}
			scopePath, err := resolveScopePath(scope, scopeSet, path)
			if err != nil {
				return err
			}

			cfg := loadOrEmpty(path)
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

	return cmd
}

func loadOrEmpty(path string) *link.Config {
	if c, err := link.Load(path); err == nil {
		return c
	}
	return &link.Config{}
}

// resolveScopePath returns the path key for the upsert: explicit --scope wins; otherwise cwd-rel-to-toml-dir.
func resolveScopePath(override string, overrideSet bool, tomlPath string) (string, error) {
	if overrideSet {
		return strings.Trim(override, "/"), nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return link.RelPath(filepath.Dir(tomlPath), cwd), nil
}

// writePath chooses where teamcity.toml goes: existing file wins, else git root, else cwd.
func writePath() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	if path, ok := link.Find(cwd); ok {
		return path, nil
	}
	if root, ok := gitRoot(); ok {
		return filepath.Join(root, link.FileName), nil
	}
	return filepath.Join(cwd, link.FileName), nil
}

func gitRoot() (string, bool) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(out)), true
}
