package run

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/config"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
)

var runListConfigCurrentUserFn = config.GetCurrentUser
var runListAPICurrentUserFn = func(client api.ClientInterface) (*api.User, error) { return client.GetCurrentUser() }
var runListIsGitRepoFn = isGitRepo
var runListCurrentBranchFn = getCurrentBranch
var runListHeadRevisionFn = getHeadRevision // used in tests

type runListOptions struct {
	job        string
	branch     string
	status     string
	user       string
	revision   string
	favorites  bool
	project    string
	limit      int
	since      string
	until      string
	jsonFields string
	plain      bool
	noHeader   bool
	web        bool
}

func newRunListCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &runListOptions{}

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List recent runs",
		Example: `  teamcity run list
  teamcity run list --favorites
  teamcity run list --user @me --limit 1
  teamcity run list --job Falcon_Build
  teamcity run list --status failure --limit 10
  teamcity run list --project Falcon --branch main
  teamcity run list --branch @this
  teamcity run list --revision abc1234
  teamcity run list --revision @head --job Falcon_Build
  teamcity run list --since 24h
  teamcity run list --json
  teamcity run list --json=id,status,webUrl
  teamcity run list --plain | grep failure`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRunList(f, cmd, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.job, "job", "j", "", "Filter by job ID")
	cmd.Flags().StringVarP(&opts.branch, "branch", "b", "", "Filter by branch name (or '@this' for current git branch)")
	cmd.Flags().StringVar(&opts.status, "status", "", "Filter by status (success, failure, running, queued, error, unknown)")
	cmd.Flags().StringVarP(&opts.user, "user", "u", "", "Filter by user who triggered")
	cmd.Flags().StringVar(&opts.revision, "revision", "", "Filter by VCS revision/commit SHA (or '@head' for current HEAD)")
	cmd.Flags().BoolVar(&opts.favorites, "favorites", false, "Show favorites for the current user")
	cmd.Flags().StringVarP(&opts.project, "project", "p", "", "Filter by project ID")
	cmd.Flags().IntVarP(&opts.limit, "limit", "n", 30, "Maximum number of items")
	cmd.Flags().StringVar(&opts.since, "since", "", "Finished after this time (e.g., 24h, 2026-01-21)")
	cmd.Flags().StringVar(&opts.until, "until", "", "Finished before this time (e.g., 12h, 2026-01-22)")
	cmdutil.AddJSONFieldsFlag(cmd, &opts.jsonFields)
	cmd.Flags().BoolVar(&opts.plain, "plain", false, "Output in plain text format for scripting")
	cmd.Flags().BoolVar(&opts.noHeader, "no-header", false, "Omit header row (use with --plain)")
	cmd.Flags().BoolVarP(&opts.web, "web", "w", false, "Open in browser")

	cmd.MarkFlagsMutuallyExclusive("json", "plain")

	return cmd
}

func runRunList(f *cmdutil.Factory, cmd *cobra.Command, opts *runListOptions) error {
	if err := cmdutil.ValidateLimit(opts.limit); err != nil {
		return err
	}
	jsonResult, showHelp, err := cmdutil.ParseJSONFields(cmd, opts.jsonFields, &api.BuildFields, f.Printer.Out)
	if err != nil {
		return err
	}
	if showHelp {
		return nil
	}

	client, err := f.Client()
	if err != nil {
		return err
	}

	request, err := resolveRunListRequest(client, opts, jsonResult.Fields)
	if err != nil {
		return err
	}

	if opts.web {
		url := config.GetServerURL() + request.webPath
		return browser.OpenURL(url)
	}

	runs, err := client.GetBuilds(context.Background(), request.builds)
	if err != nil {
		return err
	}

	if jsonResult.Enabled {
		return f.Printer.PrintJSON(runs)
	}

	if runs.Count == 0 {
		f.Printer.Empty(request.emptyMsg, request.emptyTip)
		return nil
	}

	var headers []string
	if opts.plain {
		headers = []string{"STATUS", "ID", "JOB", "BRANCH", "TRIGGERED_BY", "DURATION", "AGE"}
	} else {
		headers = []string{"STATUS", "RUN", "JOB", "BRANCH", "TRIGGERED BY", "DURATION", "AGE"}
	}
	var rows [][]string

	for _, r := range runs.Builds {
		var status, runRef string
		if opts.plain {
			status = output.PlainStatusText(r.Status, r.State, r.StatusText)
			runRef = fmt.Sprintf("%d", r.ID)
		} else {
			status = fmt.Sprintf("%s %s", output.StatusIcon(r.Status, r.State, r.StatusText), output.StatusText(r.Status, r.State, r.StatusText))
			runRef = fmt.Sprintf("%d  #%s", r.ID, r.Number)
		}

		triggeredBy := "-"
		if r.Triggered != nil && r.Triggered.User != nil {
			triggeredBy = r.Triggered.User.Name
		} else if r.Triggered != nil {
			triggeredBy = r.Triggered.Type
		}

		duration := "-"
		age := "-"

		if r.StartDate != "" {
			startTime, _ := api.ParseTeamCityTime(r.StartDate)
			if r.FinishDate != "" {
				finishTime, _ := api.ParseTeamCityTime(r.FinishDate)
				duration = output.FormatDuration(finishTime.Sub(startTime))
				age = output.RelativeTime(finishTime)
			} else {
				duration = output.FormatDuration(time.Since(startTime))
				age = "now"
			}
		} else if r.QueuedDate != "" {
			queuedTime, _ := api.ParseTeamCityTime(r.QueuedDate)
			age = output.RelativeTime(queuedTime)
		}

		branch := r.BranchName
		if branch == "" {
			branch = "-"
		}

		rows = append(rows, []string{
			status,
			runRef,
			r.BuildTypeID,
			branch,
			triggeredBy,
			duration,
			age,
		})
	}

	p := f.Printer
	if opts.plain {
		p.PrintPlainTable(headers, rows, opts.noHeader)
	} else {
		output.AutoSizeColumns(headers, rows, 2, 2, 3, 4)
		p.PrintTable(headers, rows)
	}
	return nil
}

type runListRequest struct {
	builds   api.BuildsOptions
	webPath  string
	emptyMsg string
	emptyTip string
}

func resolveRunListRequest(client api.ClientInterface, opts *runListOptions, fields []string) (*runListRequest, error) {
	user, err := resolveRunListUser(client, opts)
	if err != nil {
		return nil, err
	}

	branch, err := resolveRunListBranch(opts)
	if err != nil {
		return nil, err
	}

	statusFilter, stateFilter, err := resolveRunListStatus(opts.status)
	if err != nil {
		return nil, err
	}

	sinceDate, untilDate, err := resolveRunListDateRange(opts)
	if err != nil {
		return nil, err
	}

	revision, err := resolveRunListRevision(opts)
	if err != nil {
		return nil, err
	}

	return &runListRequest{
		builds: api.BuildsOptions{
			BuildTypeID: opts.job,
			Branch:      branch,
			Status:      statusFilter,
			State:       stateFilter,
			User:        user,
			Project:     opts.project,
			Revision:    revision,
			Favorites:   opts.favorites,
			Limit:       opts.limit,
			SinceDate:   sinceDate,
			UntilDate:   untilDate,
			Fields:      fields,
		},
		webPath:  resolveRunListWebPath(opts),
		emptyMsg: resolveRunListEmptyMessage(opts),
		emptyTip: resolveRunListEmptyTip(opts),
	}, nil
}

func resolveRunListUser(client api.ClientInterface, opts *runListOptions) (string, error) {
	if strings.EqualFold(opts.user, "@me") {
		return resolveCurrentAuthenticatedUser(client, "@me")
	}
	return opts.user, nil
}

func resolveCurrentAuthenticatedUser(client api.ClientInterface, source string) (string, error) {
	user := runListConfigCurrentUserFn()
	if user != "" {
		return user, nil
	}

	u, err := runListAPICurrentUserFn(client)
	if err != nil || u == nil || u.Username == "" {
		return "", fmt.Errorf("%s requires login (username not found in config)", source)
	}

	return u.Username, nil
}

func resolveRunListBranch(opts *runListOptions) (string, error) {
	if strings.EqualFold(opts.branch, "@this") {
		return resolveAutoBranch(true)
	}
	return opts.branch, nil
}

func resolveAutoBranch(required bool) (string, error) {
	if !runListIsGitRepoFn() {
		if required {
			return "", fmt.Errorf("--branch @this requires a git repository")
		}
		return "", nil
	}
	return runListCurrentBranchFn()
}

func resolveRunListStatus(status string) (statusFilter, stateFilter string, err error) {
	if status == "" {
		return "", "", nil
	}

	validValues := []string{"success", "failure", "running", "queued", "error", "unknown", "canceled"}
	v := strings.ToLower(status)
	if !slices.Contains(validValues, v) {
		return "", "", fmt.Errorf("invalid status %q, must be one of: %s", status, strings.Join(validValues, ", "))
	}

	switch v {
	case "running", "queued":
		return "", v, nil
	case "canceled":
		return "unknown", "finished", nil
	default:
		return v, "finished", nil
	}
}

func resolveRunListDateRange(opts *runListOptions) (sinceDate, untilDate string, err error) {
	if opts.since != "" {
		sinceDate, err = api.ParseUserDate(opts.since)
		if err != nil {
			return "", "", fmt.Errorf("invalid --since date: %w", err)
		}
	}
	if opts.until != "" {
		untilDate, err = api.ParseUserDate(opts.until)
		if err != nil {
			return "", "", fmt.Errorf("invalid --until date: %w", err)
		}
	}

	if sinceDate != "" && untilDate != "" {
		sinceTime, err1 := api.ParseTeamCityTime(sinceDate)
		untilTime, err2 := api.ParseTeamCityTime(untilDate)
		if err1 == nil && err2 == nil && sinceTime.After(untilTime) {
			return "", "", fmt.Errorf("--since (%s) is more recent than --until (%s), resulting in an empty range", opts.since, opts.until)
		}
	}

	return sinceDate, untilDate, nil
}

var runListResolveRevisionFn = resolveRevision

func resolveRunListRevision(opts *runListOptions) (string, error) {
	if strings.EqualFold(opts.revision, "@head") {
		if !runListIsGitRepoFn() {
			return "", fmt.Errorf("--revision @head requires a git repository")
		}
		return runListHeadRevisionFn()
	}
	if opts.revision != "" && len(opts.revision) < 40 && runListIsGitRepoFn() {
		return runListResolveRevisionFn(opts.revision)
	}
	return opts.revision, nil
}

func resolveRunListWebPath(opts *runListOptions) string {
	if opts.favorites {
		return "/favorite/builds"
	}
	return "/builds"
}

func resolveRunListEmptyMessage(opts *runListOptions) string {
	if opts.favorites {
		return "No favorite runs found"
	}
	return "No runs found"
}

func resolveRunListEmptyTip(opts *runListOptions) string {
	if opts.favorites {
		return output.TipNoFavoriteRuns
	}
	return output.TipNoRuns
}

func newRunViewCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &cmdutil.ViewOptions{}
	cmd := &cobra.Command{
		Use:     "view <id>",
		Aliases: []string{"show"},
		Short:   "View details",
		Args:    cobra.ExactArgs(1),
		Example: `  teamcity run view 12345
  teamcity run view 12345 --web
  teamcity run view 12345 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRunView(f, args[0], opts)
		},
	}
	cmdutil.AddViewFlags(cmd, opts)
	return cmd
}

func runRunView(f *cmdutil.Factory, runID string, opts *cmdutil.ViewOptions) error {
	p := f.Printer
	client, err := f.Client()
	if err != nil {
		return err
	}

	build, err := client.GetBuild(context.Background(), runID)
	if err != nil {
		return err
	}

	if opts.Web {
		return browser.OpenURL(build.WebURL)
	}

	if opts.JSON {
		reused, _ := client.GetBuildUsedByOtherBuilds(fmt.Sprintf("%d", build.ID))
		build.UsedByOtherBuilds = reused
		return p.PrintJSON(build)
	}

	reused, _ := client.GetBuildUsedByOtherBuilds(fmt.Sprintf("%d", build.ID))
	build.UsedByOtherBuilds = reused

	pipelineRun, _ := client.GetBuildPipelineRun(fmt.Sprintf("%d", build.ID))

	icon := output.StatusIcon(build.Status, build.State, build.StatusText)
	jobName := build.BuildTypeID
	if pipelineRun != nil && pipelineRun.Pipeline != nil && pipelineRun.Pipeline.Name != "" {
		jobName = pipelineRun.Pipeline.Name + " ⬡"
	} else if build.BuildType != nil {
		jobName = build.BuildType.Name
	}

	_, _ = fmt.Fprintf(p.Out, "%s %s %d  #%s", icon, output.Cyan(jobName), build.ID, build.Number)
	if build.BranchName != "" {
		_, _ = fmt.Fprintf(p.Out, " · %s", build.BranchName)
	}
	_, _ = fmt.Fprintln(p.Out)

	if build.Triggered != nil {
		triggeredBy := build.Triggered.Type
		if build.Triggered.User != nil {
			triggeredBy = build.Triggered.User.Name
		}
		_, _ = fmt.Fprintf(p.Out, "Triggered by %s", triggeredBy)

		if build.StartDate != "" {
			startTime, _ := api.ParseTeamCityTime(build.StartDate)
			_, _ = fmt.Fprintf(p.Out, " · %s", output.RelativeTime(startTime))

			if build.FinishDate != "" {
				finishTime, _ := api.ParseTeamCityTime(build.FinishDate)
				duration := finishTime.Sub(startTime)
				_, _ = fmt.Fprintf(p.Out, " · Took %s", output.FormatDuration(duration))
			}
		}
		_, _ = fmt.Fprintln(p.Out)
	}

	if build.UsedByOtherBuilds {
		_, _ = fmt.Fprintf(p.Out, "\n%s Results shared in build chain\n", output.Yellow("⟳"))
	}

	if build.StatusText != "" && build.StatusText != build.Status {
		_, _ = fmt.Fprintf(p.Out, "\nStatus: %s\n", build.StatusText)
	}

	if build.State == "queued" && build.WaitReason != "" {
		_, _ = fmt.Fprintf(p.Out, "\nWait reason: %s\n", output.Yellow(build.WaitReason))
	}

	if build.State == "running" && build.PercentageComplete > 0 {
		_, _ = fmt.Fprintf(p.Out, "\nProgress: %d%%\n", build.PercentageComplete)
	}

	if build.Agent != nil {
		_, _ = fmt.Fprintf(p.Out, "\nAgent: %s", output.Faint(build.Agent.Name))
		if build.State == "running" {
			_, _ = fmt.Fprintf(p.Out, "  %s teamcity agent term %d", output.Faint("·"), build.Agent.ID)
		}
		_, _ = fmt.Fprintln(p.Out)
	}

	if build.Pinned {
		_, _ = fmt.Fprintf(p.Out, "\n%s\n", output.Yellow("📌 Pinned"))
	}

	if build.Tags != nil && len(build.Tags.Tag) > 0 {
		var tagNames []string
		for _, t := range build.Tags.Tag {
			tagNames = append(tagNames, t.Name)
		}
		_, _ = fmt.Fprintf(p.Out, "\nTags: %s\n", strings.Join(tagNames, ", "))
	}

	if pipelineRun != nil && pipelineRun.Jobs != nil && len(pipelineRun.Jobs.Job) > 0 {
		maxIDLen := 0
		for _, j := range pipelineRun.Jobs.Job {
			if len(j.ID) > maxIDLen {
				maxIDLen = len(j.ID)
			}
		}
		_, _ = fmt.Fprintf(p.Out, "\n%s:\n", output.Cyan("Pipeline Jobs"))
		for _, j := range pipelineRun.Jobs.Job {
			padded := fmt.Sprintf("%-*s", maxIDLen+2, j.ID)
			buildInfo := ""
			if j.Build != nil && j.Build.ID > 0 {
				buildInfo = fmt.Sprintf(" (#%d)", j.Build.ID)
			}
			_, _ = fmt.Fprintf(p.Out, "  %s %s%s\n", output.Faint(padded), j.Name, output.Faint(buildInfo))
		}
	}

	_, _ = fmt.Fprintf(p.Out, "\n%s %s\n", output.Faint("View in browser:"), output.Green(build.WebURL))

	return nil
}
