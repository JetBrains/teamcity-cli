// Package link implements `teamcity link`: upsert a [[server]] entry (or a
// per-path scope inside one) in teamcity.toml.
package link

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
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

With no flags and a terminal, runs an interactive picker that matches the repo's
git remote against TeamCity VCS roots, lets you pick a default job, and optionally
attach additional jobs under the same scope.

Resolution cascade (highest to lowest):
  --flag → TEAMCITY_* env → matching [[server]] entry, deepest matching path scope`,
		Example: `  # Interactive: picks project from git remote, prompts for a default job
  teamcity link

  # Explicit: no prompts
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
			serverOverride := ""
			if cmd.Flags().Changed("server") {
				serverOverride = config.NormalizeURL(server)
			}
			// Fallback for the non-interactive path: if no --server was set we still need
			// *some* URL to write into teamcity.toml; take the active one.
			serverURL := serverOverride
			if serverURL == "" {
				serverURL = config.NormalizeURL(config.GetServerURL())
			}
			if serverURL == "" {
				return api.Validation(
					"--server is required when no active TeamCity server is configured",
					"Pass --server <url> or run 'teamcity auth login' first",
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

			noFields := project == "" && job == "" && len(jobs) == 0
			if noFields && f.IsInteractive() {
				inputs := &pickerInputs{}
				err := runPicker(f, serverOverride, path, scopePath, inputs)
				if errors.Is(err, errPickerHandled) {
					return nil
				}
				if err != nil {
					return err
				}
				serverURL, project, job, jobs = inputs.server, inputs.project, inputs.job, inputs.jobs
			} else if noFields {
				return api.Validation(
					"at least one of --project, --job, or --jobs is required",
					"Pass --project <id> (and optionally --job <id> or --jobs A,B,C), or run 'teamcity link' in a terminal for the interactive picker",
				)
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
	if root, ok := git.RepoRoot(cwd); ok {
		return filepath.Join(root, link.FileName), nil
	}
	return filepath.Join(cwd, link.FileName), nil
}
