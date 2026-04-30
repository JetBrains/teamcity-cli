package link

import (
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
	"github.com/dustin/go-humanize/english"
)

// errPickerHandled signals the picker finalized (or deliberately skipped) the write so the outer cmd's flag-driven path is bypassed.
var errPickerHandled = errors.New("picker handled write")

// pickerInputs is what the form fills in for the outer cmd to persist.
type pickerInputs struct {
	server  string
	project string
	job     string
	jobs    []string
}

// serverResult is one server's discovery outcome.
type serverResult struct {
	url       string
	discovery *discovery
	err       error
}

// findHits runs discovery shared by --auto and the interactive picker; returns servers that produced ≥1 project match.
func findHits(f *cmdutil.Factory, serverOverride string, cfg *link.Config, scopePath string) ([]serverResult, []string, error) {
	candidates := candidateServers(serverOverride)
	if len(candidates) == 0 {
		return nil, nil, errors.New("no TeamCity server configured\n  Run 'teamcity auth login' first")
	}

	printHeader(f, candidates, cfg, scopePath, serverOverride != "")

	remotes := git.RemoteURLs()
	if len(candidates) == 1 {
		f.Printer.Progress("Discovering projects on %s... ", output.Cyan(candidates[0]))
	} else {
		f.Printer.Progress("Discovering projects across %d %s... ", len(candidates), english.PluralWord(len(candidates), "server", ""))
	}
	results := discoverAcrossServers(f, candidates, remotes)

	var hits []serverResult
	for _, r := range results {
		if r.discovery != nil && len(r.discovery.Projects) > 0 {
			hits = append(hits, r)
		}
	}
	if len(hits) == 0 {
		return nil, remotes, noMatchHint(remotes, results)
	}
	return hits, remotes, nil
}

// runPicker drives interactive `teamcity link`; mutates outputs on Change, returns errPickerHandled on Keep/Clear (already finalized).
func runPicker(f *cmdutil.Factory, serverOverride string, cfg *link.Config, scopePath, tomlPath string, server, project, job *string, jobs *[]string) error {
	hits, _, err := findHits(f, serverOverride, cfg, scopePath)
	if err != nil {
		return err
	}

	hitByURL := map[string]*discovery{}
	for _, h := range hits {
		hitByURL[h.url] = h.discovery
	}

	inputs := pickerInputs{server: hits[0].url}
	resetProjectAndJob(&inputs, hitByURL)

	existing := lookupScope(cfg, inputs.server, scopePath)
	action := "change"
	if existing != nil {
		action = ""
		if pm := findProject(hitByURL[inputs.server], existing.Project); pm != nil {
			inputs.project = existing.Project
			if slices.ContainsFunc(pm.Jobs, func(j jobOption) bool { return j.ID == existing.Job }) {
				inputs.job = existing.Job
			}
		}
	}

	groups := buildGroups(hits, hitByURL, cfg, scopePath, &action, &inputs)
	if len(groups) > 0 {
		if err := cmdutil.RunForm(groups...); err != nil {
			return err
		}
	}

	// Honor Keep/Clear only when the finally-picked server has a binding; a stale action from an earlier server falls through to a Change write.
	if lookupScope(cfg, inputs.server, scopePath) != nil {
		switch action {
		case "keep":
			f.Printer.Success("Kept existing binding")
			return errPickerHandled
		case "clear":
			clearScope(cfg, inputs.server, scopePath)
			if err := link.Save(tomlPath, cfg); err != nil {
				return fmt.Errorf("write %s: %w", tomlPath, err)
			}
			f.Printer.Success("Cleared binding for %s", output.Cyan(scopeLabel(scopePath)))
			f.Printer.Info("  Wrote: %s", tomlPath)
			return errPickerHandled
		}
	}

	*server = inputs.server
	*project = inputs.project
	*job = inputs.job
	*jobs = inputs.jobs
	return nil
}

// autoResolution is what runAuto computes from discovery hits before writing.
type autoResolution struct {
	server  string
	project string
	job     string
	jobs    []string
}

// resolveAuto picks a unique binding from hits; ambiguity yields a typed error the caller renders to the user.
// activeServer, if set and present in hits, is preferred when multiple servers match — that's the user's working server.
func resolveAuto(hits []serverResult, activeServer string) (autoResolution, error) {
	if len(hits) > 1 {
		if activeServer != "" {
			for _, h := range hits {
				if h.url == activeServer {
					hits = []serverResult{h}
					break
				}
			}
		}
	}
	if len(hits) > 1 {
		urls := make([]string, len(hits))
		for i, h := range hits {
			urls[i] = h.url
		}
		slices.Sort(urls)
		return autoResolution{}, fmt.Errorf("multiple servers match this repo: %s\n  Pass --server <url> --auto to disambiguate", strings.Join(urls, ", "))
	}
	h := hits[0]
	if len(h.discovery.Projects) > 1 {
		ids := make([]string, len(h.discovery.Projects))
		for i, p := range h.discovery.Projects {
			ids[i] = p.ProjectID
		}
		slices.Sort(ids)
		return autoResolution{}, fmt.Errorf("multiple projects match on %s: %s\n  Drop --auto and pass --project <id> --job <id>", h.url, strings.Join(ids, ", "))
	}

	pm := h.discovery.Projects[0]
	res := autoResolution{server: h.url, project: pm.ProjectID}
	if len(pm.Jobs) == 1 {
		res.job = pm.Jobs[0].ID
	}
	for _, j := range allJobsOnServer(h.discovery) {
		if j.ID != res.job {
			res.jobs = append(res.jobs, j.ID)
		}
	}
	return res, nil
}

// runAuto runs discovery and writes the resolved binding without prompting; the same outputs the picker would persist on Change.
func runAuto(f *cmdutil.Factory, serverOverride string, cfg *link.Config, scopePath string, server, project, job *string, jobs *[]string) error {
	hits, _, err := findHits(f, serverOverride, cfg, scopePath)
	if err != nil {
		return err
	}
	res, err := resolveAuto(hits, config.NormalizeURL(config.GetServerURL()))
	if err != nil {
		return err
	}
	*server = res.server
	*project = res.project
	*job = res.job
	*jobs = res.jobs
	if res.job == "" && len(allJobsOnServer(hits[0].discovery)) > 1 {
		f.Printer.Info("  No single default job in %s; pass --job <id> to set one", output.Cyan(res.project))
	}
	return nil
}

// candidateServers returns server URLs to probe: --server override is a single result, otherwise active server first then any others with saved credentials.
func candidateServers(serverOverride string) []string {
	if serverOverride != "" {
		return []string{config.NormalizeURL(serverOverride)}
	}
	seen := map[string]bool{}
	var out []string
	add := func(u string) {
		u = config.NormalizeURL(u)
		if u == "" || seen[u] {
			return
		}
		seen[u] = true
		out = append(out, u)
	}
	add(config.GetServerURL())
	for url := range config.Get().Servers {
		add(url)
	}
	return out
}

// pickerClient returns a client for serverURL — f.Client() for the active server, a fresh client from saved creds otherwise.
func pickerClient(f *cmdutil.Factory, serverURL string) (api.ClientInterface, error) {
	if config.NormalizeURL(serverURL) == config.NormalizeURL(config.GetServerURL()) {
		return f.Client()
	}
	token, _, _ := config.GetTokenForServer(serverURL)
	if token == "" {
		return nil, fmt.Errorf("no saved credentials for %s — run 'teamcity auth login -s %s'", serverURL, serverURL)
	}
	return api.NewClient(serverURL, token, api.WithVersion(version.String())).WithContext(f.Context()), nil
}

// discoverAcrossServers runs discovery in parallel and returns results in the same order as urls.
func discoverAcrossServers(f *cmdutil.Factory, urls, remotes []string) []serverResult {
	results := make([]serverResult, len(urls))
	var wg sync.WaitGroup
	for i, url := range urls {
		wg.Go(func() {
			results[i].url = url
			c, err := pickerClient(f, url)
			if err != nil {
				results[i].err = err
				return
			}
			c.SetCommandName("link")
			d, err := discoverProjects(c, remotes)
			results[i].discovery = d
			results[i].err = err
		})
	}
	wg.Wait()
	return results
}

// buildGroups assembles the multi-server form. Returns nil only when there's literally nothing to ask.
func buildGroups(hits []serverResult, hitByURL map[string]*discovery, cfg *link.Config, scopePath string, action *string, in *pickerInputs) []*huh.Group {
	var groups []*huh.Group

	if len(hits) > 1 {
		opts := make([]huh.Option[string], len(hits))
		for i, h := range hits {
			n := len(h.discovery.Projects)
			label := h.url + " " + output.Faint(fmt.Sprintf("(%d %s)", n, english.PluralWord(n, "project", "")))
			opts[i] = huh.NewOption(label, h.url)
		}
		groups = append(groups, huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select TeamCity server").
				Options(opts...).
				Value(&in.server),
		))
	}

	groups = append(groups,
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Existing binding for "+scopeLabel(scopePath)).
				DescriptionFunc(func() string {
					if e := lookupScope(cfg, in.server, scopePath); e != nil {
						return existingDescription(*e)
					}
					return ""
				}, &in.server).
				Options(
					huh.NewOption("Change", "change"),
					huh.NewOption("Keep", "keep"),
					huh.NewOption("Clear", "clear"),
				).
				Value(action),
		).WithHideFunc(func() bool {
			return lookupScope(cfg, in.server, scopePath) == nil
		}),
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select project").
				OptionsFunc(func() []huh.Option[string] {
					d := hitByURL[in.server]
					if d == nil {
						return nil
					}
					if findProject(d, in.project) == nil {
						resetProjectAndJob(in, hitByURL)
					}
					out := make([]huh.Option[string], len(d.Projects))
					for i, p := range d.Projects {
						out[i] = huh.NewOption(p.ProjectName+" "+output.Faint("("+p.ProjectID+")"), p.ProjectID)
					}
					return out
				}, &in.server).
				Value(&in.project),
		).WithHideFunc(func() bool {
			if keptOrCleared(cfg, in.server, scopePath, *action) {
				return true
			}
			d := hitByURL[in.server]
			return d == nil || len(d.Projects) <= 1
		}),
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select default job").
				OptionsFunc(func() []huh.Option[string] {
					jobs := jobsForProject(hitByURL[in.server], in.project)
					if len(jobs) == 0 {
						return []huh.Option[string]{huh.NewOption(output.Faint("(no jobs in this project)"), "")}
					}
					out := make([]huh.Option[string], 0, len(jobs)+1)
					out = append(out, huh.NewOption(output.Faint("(skip — set later)"), ""))
					for _, j := range jobs {
						out = append(out, huh.NewOption(j.Label, j.ID))
					}
					return out
				}, []any{&in.server, &in.project}).
				Value(&in.job),
		).WithHideFunc(func() bool { return keptOrCleared(cfg, in.server, scopePath, *action) }),
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Select additional jobs").
				Description("Surfaced in TAB completion alongside the default · saved to teamcity.toml").
				OptionsFunc(func() []huh.Option[string] {
					all := allJobsOnServer(hitByURL[in.server])
					out := make([]huh.Option[string], 0, len(all))
					for _, j := range all {
						if j.ID == in.job {
							continue
						}
						out = append(out, huh.NewOption(j.Label, j.ID))
					}
					return out
				}, []any{&in.server, &in.job}).
				Value(&in.jobs),
		).WithHideFunc(func() bool {
			if keptOrCleared(cfg, in.server, scopePath, *action) {
				return true
			}
			return len(allJobsOnServer(hitByURL[in.server])) <= 1
		}),
	)

	return groups
}

// keptOrCleared hides project/job groups only when there is an existing binding the user chose to keep or clear; with no binding the action prompt never shows, so the picks are always required.
func keptOrCleared(cfg *link.Config, serverURL, scopePath, action string) bool {
	if lookupScope(cfg, serverURL, scopePath) == nil {
		return false
	}
	return action != "change"
}

// findProject returns the matching project (by ID) on a server, or nil.
func findProject(d *discovery, projectID string) *projectMatch {
	if d == nil || projectID == "" {
		return nil
	}
	for i := range d.Projects {
		if d.Projects[i].ProjectID == projectID {
			return &d.Projects[i]
		}
	}
	return nil
}

// allJobsOnServer returns every matched job across every project on the server, used by the cross-project additional-jobs picker.
func allJobsOnServer(d *discovery) []jobOption {
	if d == nil {
		return nil
	}
	var out []jobOption
	for _, p := range d.Projects {
		out = append(out, p.Jobs...)
	}
	return out
}

// jobsForProject returns the jobs for projectID, falling back to the first project when projectID isn't on the current server.
func jobsForProject(d *discovery, projectID string) []jobOption {
	if pm := findProject(d, projectID); pm != nil {
		return pm.Jobs
	}
	if d != nil && len(d.Projects) > 0 {
		return d.Projects[0].Jobs
	}
	return nil
}

// resetProjectAndJob seats project + job to the first match on in.server, since project IDs are server-scoped.
func resetProjectAndJob(in *pickerInputs, hitByURL map[string]*discovery) {
	d := hitByURL[in.server]
	if d == nil || len(d.Projects) == 0 {
		in.project, in.job = "", ""
		return
	}
	in.project = d.Projects[0].ProjectID
	in.job = ""
	if len(d.Projects[0].Jobs) > 0 {
		in.job = d.Projects[0].Jobs[0].ID
	}
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
	if srv.Paths == nil {
		return nil
	}
	if p, ok := srv.Paths[scopePath]; ok {
		return &p
	}
	return nil
}

func clearScope(cfg *link.Config, serverURL, scopePath string) {
	srv := cfg.Match(serverURL)
	if srv == nil {
		return
	}
	if scopePath == "" {
		srv.Project, srv.Job, srv.Jobs = "", "", nil
		return
	}
	delete(srv.Paths, scopePath)
}

func existingDescription(s link.PathScope) string {
	var parts []string
	if s.Project != "" {
		parts = append(parts, "project "+s.Project)
	}
	if s.Job != "" {
		parts = append(parts, "job "+s.Job)
	}
	if len(s.Jobs) > 0 {
		parts = append(parts, "jobs ["+strings.Join(s.Jobs, ", ")+"]")
	}
	return strings.Join(parts, ", ")
}

func scopeLabel(scopePath string) string {
	if scopePath == "" {
		return "the whole repo"
	}
	return scopePath + "/"
}

func printHeader(f *cmdutil.Factory, candidates []string, cfg *link.Config, scopePath string, fromOverride bool) {
	p := f.Printer
	_, _ = fmt.Fprintln(p.Out)
	p.PrintField("Linking", output.Cyan(scopeLabel(scopePath)))
	if scopePath != "" {
		p.Info("  %s", output.Faint("(pass --scope= to link the whole repo instead)"))
	}
	if len(candidates) == 1 {
		source := "active"
		if fromOverride {
			source = "from --server"
		}
		p.PrintField("Server", output.Cyan(candidates[0])+" "+output.Faint("("+source+")"))
	}
	if cfg != nil {
		var bound []string
		for _, s := range cfg.Servers {
			if s.Project != "" || s.Job != "" || len(s.Jobs) > 0 || len(s.Paths) > 0 {
				bound = append(bound, s.URL)
			}
		}
		slices.Sort(bound)
		if len(bound) > 0 {
			p.Info("  %s", output.Faint("Already bound in this repo: "+strings.Join(bound, ", ")))
		}
	}
	_, _ = fmt.Fprintln(p.Out)
}

func noMatchHint(remotes []string, results []serverResult) error {
	if len(remotes) == 0 {
		return errors.New("no git remotes found in this repository\n  Add one with 'git remote add origin <url>', or run 'teamcity link --project <id> --job <id>' explicitly")
	}
	var msg strings.Builder
	msg.WriteString("no TeamCity projects found whose VCS roots match this repo's remotes\n")
	for _, r := range results {
		switch {
		case r.err != nil:
			fmt.Fprintf(&msg, "  %s: %v\n", r.url, r.err)
		case r.discovery == nil || len(r.discovery.Projects) == 0:
			fmt.Fprintf(&msg, "  %s: no match\n", r.url)
		}
	}
	msg.WriteString("  Run 'teamcity project list' to find a project ID, then 'teamcity link --project <id> --job <id>'.")
	return errors.New(msg.String())
}
