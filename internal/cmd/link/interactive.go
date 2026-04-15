package link

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/config"
	"github.com/JetBrains/teamcity-cli/internal/git"
	"github.com/JetBrains/teamcity-cli/internal/link"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/JetBrains/teamcity-cli/internal/version"
	"github.com/charmbracelet/huh"
)

// errPickerHandled — the picker wrote or cleared teamcity.toml itself; the caller must exit without re-writing.
var errPickerHandled = errors.New("picker handled")

type pickerInputs struct {
	server, project, job string
	jobs                 []string
}

type serverResult struct {
	url  string
	disc *discovery
	err  error
}

func runPicker(f *cmdutil.Factory, serverOverride, tomlPath, scopePath string, inputs *pickerInputs) error {
	p := f.Printer
	cfg, err := loadOrEmpty(tomlPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", tomlPath, err)
	}

	servers := candidateServers(serverOverride)
	if len(servers) == 0 {
		return api.Validation(
			"no authenticated TeamCity servers available",
			"Run 'teamcity auth login' first, or pass --server with --project/--job.",
		)
	}
	printHeader(p, servers, scopePath, cfg, serverOverride != "")

	results := discoverAcrossServers(f, servers, git.RemoteURLs("."))
	var hits []serverResult
	for _, r := range results {
		if r.err == nil && r.disc != nil && len(r.disc.Projects) > 0 {
			hits = append(hits, r)
		}
	}
	if len(hits) == 0 {
		return noMatchHint(servers, results)
	}
	return runForm(p, cfg, hits, tomlPath, scopePath, inputs)
}

// runForm is a single huh.Form spanning every choice (server → existing-action → project → job → additional) so shift+tab navigates back across all groups.
func runForm(p *output.Printer, cfg *link.Config, hits []serverResult, tomlPath, scopePath string, inputs *pickerInputs) error {
	server := preferredServer(hits)
	action := "change"
	var project, job string
	var jobs []string

	findHit := func() *serverResult {
		for i := range hits {
			if hits[i].url == server {
				return &hits[i]
			}
		}
		return &hits[0]
	}
	hasExisting := func() bool { return lookupScope(cfg, server, scopePath) != nil }
	allJobs := func() []api.BuildType { return findHit().disc.AllJobs }

	// Initial picks reflect the pre-focused server so the first forward pass has sensible defaults.
	h := findHit()
	project = h.disc.Projects[pickCwdAffinity(h.disc.Projects, scopePath)].ID
	if len(h.disc.AllJobs) > 0 {
		job = h.disc.AllJobs[0].ID
	}

	var groups []*huh.Group
	if len(hits) > 1 {
		groups = append(groups, huh.NewGroup(huh.NewSelect[string]().
			Title("Multiple servers match this repo — which one?").
			Options(serverOpts(hits)...).
			Value(&server)))
	}
	groups = append(groups,
		huh.NewGroup(huh.NewSelect[string]().
			TitleFunc(func() string {
				e := lookupScope(cfg, server, scopePath)
				if e == nil {
					return ""
				}
				summary := e.Project
				if e.Job != "" {
					summary += " → " + e.Job
				}
				return fmt.Sprintf("%s already linked: %s", server, summary)
			}, &server).
			Options(
				huh.NewOption("Keep as-is", "keep"),
				huh.NewOption("Change", "change"),
				huh.NewOption("Clear this scope", "clear"),
			).
			Value(&action)).
			WithHideFunc(func() bool { return !hasExisting() }),
		huh.NewGroup(huh.NewSelect[string]().
			Title("TeamCity project").
			OptionsFunc(func() []huh.Option[string] { return projectOpts(findHit().disc.Projects) }, &server).
			Value(&project)).
			WithHideFunc(func() bool { return action != "change" || len(findHit().disc.Projects) <= 1 }),
		huh.NewGroup(huh.NewSelect[string]().
			Title("Default job for 'teamcity run start'").
			OptionsFunc(func() []huh.Option[string] { return jobOpts(allJobs(), findHit().disc.Pipelines, "", true) }, &server).
			Value(&job)).
			WithHideFunc(func() bool { return action != "change" }),
		huh.NewGroup(huh.NewMultiSelect[string]().
			Title("Also link these jobs  (optional — space toggles, enter continues)").
			OptionsFunc(func() []huh.Option[string] { return jobOpts(allJobs(), findHit().disc.Pipelines, job, false) }, &job).
			Value(&jobs)).
			WithHideFunc(func() bool { return action != "change" || len(allJobs()) <= 1 }),
	)

	if err := cmdutil.RunForm(groups...); err != nil {
		return err
	}

	switch action {
	case "keep":
		p.Info("Kept existing binding.")
		return errPickerHandled
	case "clear":
		return clearScope(p, cfg, tomlPath, server, scopePath)
	}
	inputs.server, inputs.project, inputs.job, inputs.jobs = server, project, job, jobs
	return nil
}

func preferredServer(hits []serverResult) string {
	active := config.GetServerURL()
	for _, h := range hits {
		if h.url == active {
			return h.url
		}
	}
	return hits[0].url
}

// candidateServers lists the TC servers we should probe: just the override if set, else every URL we have credentials for.
func candidateServers(override string) []string {
	if override != "" {
		return []string{override}
	}
	seen := map[string]bool{}
	var out []string
	add := func(u string) {
		if u != "" && !seen[u] {
			seen[u] = true
			out = append(out, u)
		}
	}
	add(config.GetServerURL())
	for u := range config.Get().Servers {
		add(u)
	}
	return out
}

func discoverAcrossServers(f *cmdutil.Factory, servers, remotes []string) []serverResult {
	results := make([]serverResult, len(servers))
	active := config.GetServerURL()
	var wg sync.WaitGroup
	for i, url := range servers {
		wg.Go(func() {
			client, err := pickerClient(f, url, url == active)
			if err != nil {
				results[i] = serverResult{url: url, err: err}
				return
			}
			disc, err := discoverProjects(client, remotes)
			results[i] = serverResult{url: url, disc: disc, err: err}
		})
	}
	wg.Wait()
	return results
}

func serverOpts(hits []serverResult) []huh.Option[string] {
	active := config.GetServerURL()
	opts := make([]huh.Option[string], len(hits))
	for i, h := range hits {
		label := h.url
		if h.url == active {
			label += "  " + output.Faint("(active)")
		}
		opts[i] = huh.NewOption(label, h.url)
	}
	return opts
}

func projectOpts(projects []projectMatch) []huh.Option[string] {
	out := make([]huh.Option[string], len(projects))
	for i, p := range projects {
		out[i] = huh.NewOption(fmt.Sprintf("%s  (%s)", cmp.Or(p.Name, p.ID), p.ID), p.ID)
	}
	return out
}

func jobOpts(jobs []api.BuildType, pipelines map[string]string, exclude string, withNoDefault bool) []huh.Option[string] {
	out := make([]huh.Option[string], 0, len(jobs)+1)
	for _, j := range jobs {
		if j.ID == exclude {
			continue
		}
		out = append(out, huh.NewOption(jobLabel(j, pipelines), j.ID))
	}
	if withNoDefault {
		out = append(out, huh.NewOption("— No default (project-only binding) —", ""))
	}
	return out
}

// jobLabel formats one job option, swapping "Pipeline Head" for the pipeline's real name + ⬡ marker and suppressing duplicate project paths.
func jobLabel(j api.BuildType, pipelines map[string]string) string {
	name := cmp.Or(j.Name, j.ID)
	_, isPipeline := pipelines[j.ID]
	if isPipeline {
		if pn := pipelines[j.ID]; pn != "" {
			name = pn
		}
	}
	project := cmp.Or(j.ProjectName, j.ProjectID)
	if project == name || strings.HasSuffix(project, " / "+name) {
		project = strings.TrimSuffix(project, " / "+name)
	}
	var label string
	switch {
	case project != "":
		label = fmt.Sprintf("%s / %s  (%s)", project, name, j.ID)
	default:
		label = fmt.Sprintf("%s  (%s)", name, j.ID)
	}
	if isPipeline {
		label += " " + output.Faint("⬡ pipeline")
	}
	return label
}

func printHeader(p *output.Printer, servers []string, scopePath string, cfg *link.Config, explicit bool) {
	if scopePath == "" {
		_, _ = fmt.Fprintln(p.Out, "Linking: whole repo")
	} else {
		_, _ = fmt.Fprintf(p.Out, "Linking: the %s/ subdirectory only  %s\n",
			scopePath, output.Faint("(pass --scope= to link the whole repo)"))
	}
	switch {
	case explicit:
		_, _ = fmt.Fprintf(p.Out, "Server:  %s  %s\n", output.Cyan(servers[0]), output.Faint("(from --server)"))
	case len(servers) == 1:
		_, _ = fmt.Fprintf(p.Out, "Server:  %s  %s\n", output.Cyan(servers[0]), output.Faint("(active)"))
	default:
		_, _ = fmt.Fprintf(p.Out, "Server:  searching %d...\n", len(servers))
	}
	if cfg != nil && len(cfg.Servers) > 0 {
		_, _ = fmt.Fprintln(p.Out, "\nteamcity.toml already binds this repo to:")
		for i := range cfg.Servers {
			s := &cfg.Servers[i]
			summary := s.Project
			if s.Job != "" {
				summary += " → " + s.Job
			}
			_, _ = fmt.Fprintf(p.Out, "  %s  %s\n", output.Cyan(s.URL), summary)
		}
	}
	_, _ = fmt.Fprintln(p.Out)
}

func lookupScope(cfg *link.Config, serverURL, scopePath string) *link.PathScope {
	srv := cfg.Match(serverURL)
	if srv == nil {
		return nil
	}
	if scopePath == "" {
		if srv.Project == "" && srv.Job == "" && len(srv.Jobs) == 0 {
			return nil
		}
		return &link.PathScope{Project: srv.Project, Job: srv.Job, Jobs: srv.Jobs}
	}
	if ps, ok := srv.Paths[scopePath]; ok {
		return &ps
	}
	return nil
}

func clearScope(p *output.Printer, cfg *link.Config, tomlPath, serverURL, scopePath string) error {
	srv := cfg.Match(serverURL)
	if srv == nil {
		return errPickerHandled
	}
	if scopePath == "" {
		cfg.Servers = slices.DeleteFunc(cfg.Servers, func(s link.Server) bool { return s.URL == srv.URL })
	} else {
		delete(srv.Paths, scopePath)
	}
	if err := link.Save(tomlPath, cfg); err != nil {
		return fmt.Errorf("write %s: %w", tomlPath, err)
	}
	p.Success("Cleared binding from %s", tomlPath)
	return errPickerHandled
}

func pickerClient(f *cmdutil.Factory, serverURL string, isActive bool) (api.ClientInterface, error) {
	if isActive {
		return f.Client()
	}
	token, _, _ := config.GetTokenForServer(serverURL)
	if token == "" {
		return nil, api.Validation(
			"not authenticated to "+serverURL,
			"Run 'teamcity auth login -s "+serverURL+"' first, or pass --project and --job explicitly.",
		)
	}
	return api.NewClient(serverURL, token, api.WithVersion(version.String())).WithContext(f.Context()), nil
}

func noMatchHint(tried []string, results []serverResult) error {
	var hint strings.Builder
	if len(tried) > 1 {
		for _, r := range results {
			reason := "no match"
			if r.err != nil {
				reason = r.err.Error()
			}
			fmt.Fprintf(&hint, "  %s  (%s)\n", r.url, reason)
		}
		hint.WriteString("\nOr find IDs manually:\n")
	} else {
		hint.WriteString("Find IDs manually:\n")
	}
	hint.WriteString("    teamcity project list\n")
	hint.WriteString("    teamcity job list --project <id>\n")
	hint.WriteString("    teamcity link --project <id> --job <id>")
	return api.Validation(
		fmt.Sprintf("no TeamCity VCS roots on %d server(s) reference this repository", len(tried)),
		hint.String(),
	)
}
