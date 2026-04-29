package run

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/completion"
	"github.com/JetBrains/teamcity-cli/internal/git"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/spf13/cobra"
)

// watchFlags holds the shared watch-related flags used by run start, restart, and watch.
type watchFlags struct {
	watch    bool
	interval int
	timeout  time.Duration
}

// addToCmd registers the shared watch flags on a cobra command.
func (w *watchFlags) addToCmd(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&w.watch, "watch", false, "Watch until completion")
	cmd.Flags().IntVarP(&w.interval, "interval", "i", 5, "Refresh interval in seconds when watching")
	cmd.Flags().DurationVar(&w.timeout, "timeout", 0, "Timeout when watching (e.g., 30m, 1h); implies --watch")
}

// resolve ensures timeout implies watch and returns the runWatchOptions.
func (w *watchFlags) resolve() {
	if w.timeout > 0 {
		w.watch = true
	}
}

// watchOpts builds runWatchOptions from the shared flags with additional overrides.
func (w *watchFlags) watchOpts(logs, json bool) *runWatchOptions {
	return &runWatchOptions{
		interval: w.interval,
		timeout:  w.timeout,
		logs:     logs,
		json:     json,
	}
}

type reuseDep struct {
	id    int
	build *api.Build
	err   error
}

func fetchReuseDeps(ctx context.Context, client api.ClientInterface, ids []int) []reuseDep {
	out := make([]reuseDep, len(ids))
	var wg sync.WaitGroup
	for i, id := range ids {
		wg.Go(func() {
			b, err := client.GetBuild(ctx, strconv.Itoa(id))
			out[i] = reuseDep{id: id, build: b, err: err}
		})
	}
	wg.Wait()
	return out
}

func printReuseDeps(p *output.Printer, deps []reuseDep) {
	if len(deps) == 0 {
		return
	}
	_, _ = fmt.Fprintln(p.Out, "  Snapshot dependencies:")
	idW := 0
	for _, d := range deps {
		idW = max(idW, len(strconv.Itoa(d.id)))
	}
	for _, d := range deps {
		icon, summary := reuseDepRow(d)
		_, _ = fmt.Fprintf(p.Out, "    %s %-*d  %s\n", icon, idW, d.id, summary)
	}
}

func reuseDepRow(d reuseDep) (icon, summary string) {
	if d.build == nil {
		if _, ok := errors.AsType[*api.NotFoundError](d.err); ok || d.err == nil {
			return output.Faint("?"), output.Red("(not found)")
		}
		return output.Faint("?"), output.Yellow(fmt.Sprintf("(lookup failed: %v)", d.err))
	}
	b := d.build
	var btName string
	if b.BuildType != nil {
		btName = b.BuildType.Name
	}
	parts := make([]string, 0, 3)
	if b.Number != "" {
		parts = append(parts, output.Cyan("#"+b.Number))
	}
	parts = append(parts, cmp.Or(btName, b.BuildTypeID))
	if (b.State != "" && b.State != "finished") || (b.Status != "" && !strings.EqualFold(b.Status, "SUCCESS")) {
		parts = append(parts, output.StatusText(b.Status, b.State, b.StatusText))
	}
	return output.StatusIcon(b.Status, b.State, b.StatusText), strings.Join(parts, "  ")
}

func printQueuedRun(p *output.Printer, build *api.Build, context string) {
	ref := fmt.Sprintf("%d  #%s", build.ID, build.Number)
	if build.Number == "" {
		ref = strconv.Itoa(build.ID)
	}
	p.Success("Queued run %s for %s", ref, context)
}

func afterQueue(f *cmdutil.Factory, build *api.Build, web bool, wf *watchFlags) error {
	if web {
		cmdutil.OpenURLOrWarn(f.Printer, build.WebURL)
	}
	if wf.watch {
		_, _ = fmt.Fprintln(f.Printer.Out)
		return doRunWatch(f, strconv.Itoa(build.ID), wf.watchOpts(true, false))
	}
	return nil
}

type runStartOptions struct {
	branch            string
	revision          string
	params            map[string]string
	systemProps       map[string]string
	envVars           map[string]string
	comment           string
	personal          bool
	localChanges      string
	noPush            bool
	cleanSources      bool
	rebuildDeps       bool
	rebuildFailedDeps bool
	queueAtTop        bool
	agent             int
	tags              []string
	reuseDeps         []int
	watchFlags
	web    bool
	dryRun bool
	json   bool
}

func newRunStartCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &runStartOptions{
		params:      make(map[string]string),
		systemProps: make(map[string]string),
		envVars:     make(map[string]string),
	}

	cmd := &cobra.Command{
		Use:               "start [job-id]",
		Short:             "Start a new run",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completion.LinkedJobs(),
		Example: `  teamcity run start Falcon_Build
  teamcity run start                              # uses linked default (see 'teamcity link')
  teamcity run start Falcon_Build --branch feature/test
  teamcity run start Falcon_Build --branch @this
  teamcity run start Falcon_Build -P version=1.0 -S build.number=123 -E CI=true
  teamcity run start Falcon_Build --comment "Release build" --tag release --tag v1.0
  teamcity run start Falcon_Build --clean --rebuild-deps --top
  teamcity run start Falcon_Build --reuse-deps 6946,6917  # reuse existing as snapshot dependencies
  teamcity run start Falcon_Build --local-changes # personal build with uncommitted Git changes
  teamcity run start Falcon_Build --local-changes changes.patch  # from file
  teamcity run start Falcon_Build --revision abc123def --branch main
  teamcity run start Falcon_Build --revision @head --branch @this
  teamcity run start Falcon_Build --dry-run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			explicit := ""
			if len(args) > 0 {
				explicit = args[0]
			}
			jobID := f.ResolveDefaultJob(explicit)
			if jobID == "" {
				return api.Validation(
					"job id is required",
					"Pass <job-id> or run 'teamcity link' to bind a default job to this repository",
				)
			}
			return runRunStart(f, jobID, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.branch, "branch", "b", "", "Branch to build (or '@this' for current git branch)")
	cmd.Flags().StringVar(&opts.revision, "revision", "", "Pin to a specific Git commit SHA (or '@head' for current HEAD)")
	cmd.Flags().StringToStringVarP(&opts.params, "param", "P", nil, "Parameters (key=value)")
	cmd.Flags().StringToStringVarP(&opts.systemProps, "system", "S", nil, "System properties (key=value)")
	cmd.Flags().StringToStringVarP(&opts.envVars, "env", "E", nil, "Environment variables (key=value)")
	cmd.Flags().StringVarP(&opts.comment, "comment", "m", "", "Comment to attach")
	cmd.Flags().StringSliceVarP(&opts.tags, "tag", "t", nil, "Tags (can be repeated)")
	cmd.Flags().BoolVar(&opts.personal, "personal", false, "Personal build")
	localChangesFlag := cmd.Flags().VarPF(&localChangesValue{val: &opts.localChanges}, "local-changes", "l", "Include local changes (git, -, or path; default: git)")
	localChangesFlag.NoOptDefVal = "git"
	cmd.Flags().BoolVar(&opts.noPush, "no-push", false, "Skip auto-push of branch to remote")
	cmd.Flags().BoolVar(&opts.cleanSources, "clean", false, "Clean sources before start")
	cmd.Flags().BoolVar(&opts.rebuildDeps, "rebuild-deps", false, "Rebuild all dependencies")
	cmd.Flags().BoolVar(&opts.rebuildFailedDeps, "rebuild-failed-deps", false, "Rebuild failed/incomplete dependencies")
	cmd.Flags().IntSliceVar(&opts.reuseDeps, "reuse-deps", nil, "Reuse existing as snapshot dependencies (IDs, comma-separated or repeated)")
	cmd.Flags().BoolVar(&opts.queueAtTop, "top", false, "Add to top of queue")
	cmd.Flags().IntVar(&opts.agent, "agent", 0, "Use specific agent (by ID)")
	opts.addToCmd(cmd)
	cmd.Flags().BoolVarP(&opts.web, "web", "w", false, "Open in browser")
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "Preview without triggering")
	cmd.Flags().BoolVar(&opts.json, "json", false, "Output as JSON")

	_ = cmd.RegisterFlagCompletionFunc("branch", completion.GitBranches())
	_ = cmd.RegisterFlagCompletionFunc("revision", completion.AtHead())
	_ = cmd.RegisterFlagCompletionFunc("local-changes", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return []string{"git", "-"}, cobra.ShellCompDirectiveDefault
	})

	return cmd
}

func runRunStart(f *cmdutil.Factory, jobID string, opts *runStartOptions) error {
	p := f.Printer
	opts.resolve()
	branch, err := resolveBranchFlag(opts.branch)
	if err != nil {
		return err
	}
	opts.branch = branch
	revision, err := resolveRevisionFlag(opts.revision)
	if err != nil {
		return err
	}
	opts.revision = revision
	if opts.dryRun {
		client, err := f.Client()
		if err != nil {
			return err
		}
		if !client.BuildTypeExists(jobID) {
			return api.Validation(
				fmt.Sprintf("job %q not found", jobID),
				"Check the job ID with: teamcity job list",
			)
		}
		_, _ = fmt.Fprintf(p.Out, "%s Would trigger run for %s\n", output.Faint("[dry-run]"), output.Cyan(jobID))
		if opts.branch != "" {
			_, _ = fmt.Fprintf(p.Out, "  Branch: %s\n", opts.branch)
		}
		if opts.revision != "" {
			_, _ = fmt.Fprintf(p.Out, "  Revision: %s\n", opts.revision)
		}
		if len(opts.params) > 0 {
			_, _ = fmt.Fprintln(p.Out, "  Parameters:")
			for k, v := range opts.params {
				_, _ = fmt.Fprintf(p.Out, "    %s=%s\n", k, v)
			}
		}
		if len(opts.systemProps) > 0 {
			_, _ = fmt.Fprintln(p.Out, "  System properties:")
			for k, v := range opts.systemProps {
				_, _ = fmt.Fprintf(p.Out, "    %s=%s\n", k, v)
			}
		}
		if len(opts.envVars) > 0 {
			_, _ = fmt.Fprintln(p.Out, "  Environment variables:")
			for k, v := range opts.envVars {
				_, _ = fmt.Fprintf(p.Out, "    %s=%s\n", k, v)
			}
		}
		if opts.comment != "" {
			_, _ = fmt.Fprintf(p.Out, "  Comment: %s\n", opts.comment)
		}
		if len(opts.tags) > 0 {
			_, _ = fmt.Fprintf(p.Out, "  Tags: %s\n", strings.Join(opts.tags, ", "))
		}
		if opts.personal || opts.localChanges != "" {
			_, _ = fmt.Fprintln(p.Out, "  Personal build: yes")
		}
		if opts.localChanges != "" {
			_, _ = fmt.Fprintf(p.Out, "  Local changes: %s\n", opts.localChanges)
		}
		if opts.cleanSources {
			_, _ = fmt.Fprintln(p.Out, "  Clean sources: yes")
		}
		if opts.rebuildDeps {
			_, _ = fmt.Fprintln(p.Out, "  Rebuild dependencies: yes")
		}
		if len(opts.reuseDeps) > 0 {
			printReuseDeps(p, fetchReuseDeps(f.Context(), client, opts.reuseDeps))
		}
		if opts.queueAtTop {
			_, _ = fmt.Fprintln(p.Out, "  Queue at top: yes")
		}
		if opts.agent > 0 {
			_, _ = fmt.Fprintf(p.Out, "  Agent ID: %d\n", opts.agent)
		}
		return nil
	}

	if opts.localChanges != "" && opts.branch == "" {
		if !isGitRepoFn() {
			return api.Validation(
				"not a git repository",
				"Run this command from within a git repository, or specify --branch explicitly",
			)
		}
		branch, err := currentBranchFn()
		if err != nil {
			return err
		}
		opts.branch = branch
		p.Info("Using current branch: %s", branch)
	}

	if opts.localChanges != "" && !opts.noPush {
		if !git.BranchExistsOnRemote(opts.branch) {
			p.Info("Pushing branch to remote...")
			if err := pushBranch(opts.branch); err != nil {
				return err
			}
			p.Success("Branch pushed to remote")
		}
	}

	client, err := f.Client()
	if err != nil {
		return err
	}

	var personalChangeID string
	if opts.localChanges != "" {
		patch, err := loadLocalChanges(opts.localChanges, f.IOStreams.In)
		if err != nil {
			return err
		}

		p.Info("Uploading local changes...")
		description := cmp.Or(opts.comment, "Personal build with local changes")

		changeID, err := client.UploadDiffChanges(patch, description)
		if err != nil {
			return fmt.Errorf("failed to upload changes: %w", err)
		}
		personalChangeID = changeID
		p.Success("Uploaded changes (ID: %s)", changeID)

		opts.personal = true
	}

	build, err := client.RunBuild(jobID, api.RunBuildOptions{
		Branch:                    opts.branch,
		Params:                    opts.params,
		SystemProps:               opts.systemProps,
		EnvVars:                   opts.envVars,
		Comment:                   opts.comment,
		Personal:                  opts.personal,
		CleanSources:              opts.cleanSources,
		RebuildDependencies:       opts.rebuildDeps,
		RebuildFailedDependencies: opts.rebuildFailedDeps,
		QueueAtTop:                opts.queueAtTop,
		AgentID:                   opts.agent,
		Tags:                      opts.tags,
		PersonalChangeID:          personalChangeID,
		Revision:                  opts.revision,
		SnapshotDependencies:      opts.reuseDeps,
	})
	if err != nil {
		return err
	}

	if opts.json {
		if opts.watch {
			return doRunWatch(f, strconv.Itoa(build.ID), opts.watchOpts(false, true))
		}
		return p.PrintJSON(build)
	}

	reused := build.State == "finished"
	if reused {
		ref := strconv.Itoa(build.ID)
		if build.Number != "" {
			ref = fmt.Sprintf("%d  #%s", build.ID, build.Number)
		}
		p.Info("Reused existing #%s for %s (optimization)", ref, jobID)
	} else {
		printQueuedRun(p, build, jobID)
	}

	if opts.branch != "" {
		p.Info("  Branch: %s", opts.branch)
	}
	if opts.comment != "" {
		p.Info("  Comment: %s", opts.comment)
	}
	if len(opts.tags) > 0 {
		p.Info("  Tags: %s", strings.Join(opts.tags, ", "))
	}
	if len(opts.reuseDeps) > 0 {
		printReuseDeps(p, fetchReuseDeps(f.Context(), client, opts.reuseDeps))
	}
	p.Info("  URL: %s", build.WebURL)
	if opts.agent > 0 {
		_, _ = fmt.Fprintf(p.Out, "  %s teamcity agent term %d\n", output.Faint("Agent terminal:"), opts.agent)
	}
	if build.WaitReason != "" {
		p.Info("  Wait reason: %s", build.WaitReason)
	}
	if !reused && !opts.watch {
		_, _ = fmt.Fprintf(p.Out, "  %s teamcity run log -f %d\n", output.Faint("Follow logs:"), build.ID)
	}

	if reused {
		if opts.web {
			cmdutil.OpenURLOrWarn(f.Printer, build.WebURL)
		}
		return nil
	}
	return afterQueue(f, build, opts.web, &opts.watchFlags)
}
