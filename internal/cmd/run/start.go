package run

import (
	"fmt"
	"strings"
	"time"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	tcerrors "github.com/JetBrains/teamcity-cli/internal/errors"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/pkg/browser"
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
	cmd.Flags().BoolVar(&w.watch, "watch", false, "Watch run until it completes")
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

func printQueuedRun(p *output.Printer, build *api.Build, context string) {
	ref := fmt.Sprintf("%d  #%s", build.ID, build.Number)
	if build.Number == "" {
		ref = fmt.Sprintf("%d", build.ID)
	}
	p.Success("Queued run %s for %s", ref, context)
}

func afterQueue(f *cmdutil.Factory, build *api.Build, web bool, wf *watchFlags) error {
	if web {
		_ = browser.OpenURL(build.WebURL)
	}
	if wf.watch {
		_, _ = fmt.Fprintln(f.Printer.Out)
		return doRunWatch(f, fmt.Sprintf("%d", build.ID), wf.watchOpts(true, false))
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
		Use:   "start <job-id>",
		Short: "Start a new run",
		Args:  cobra.ExactArgs(1),
		Example: `  teamcity run start Falcon_Build
  teamcity run start Falcon_Build --branch feature/test
  teamcity run start Falcon_Build -P version=1.0 -S build.number=123 -E CI=true
  teamcity run start Falcon_Build --comment "Release build" --tag release --tag v1.0
  teamcity run start Falcon_Build --clean --rebuild-deps --top
  teamcity run start Falcon_Build --local-changes # personal build with uncommitted Git changes
  teamcity run start Falcon_Build --local-changes changes.patch  # from file
  teamcity run start Falcon_Build --revision abc123def --branch main
  teamcity run start Falcon_Build --dry-run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRunStart(f, args[0], opts)
		},
	}

	cmd.Flags().StringVarP(&opts.branch, "branch", "b", "", "Branch to build")
	cmd.Flags().StringVar(&opts.revision, "revision", "", "Pin build to a specific Git commit SHA")
	cmd.Flags().StringToStringVarP(&opts.params, "param", "P", nil, "Build parameters (key=value)")
	cmd.Flags().StringToStringVarP(&opts.systemProps, "system", "S", nil, "System properties (key=value)")
	cmd.Flags().StringToStringVarP(&opts.envVars, "env", "E", nil, "Environment variables (key=value)")
	cmd.Flags().StringVarP(&opts.comment, "comment", "m", "", "Run comment")
	cmd.Flags().StringSliceVarP(&opts.tags, "tag", "t", nil, "Run tags (can be repeated)")
	cmd.Flags().BoolVar(&opts.personal, "personal", false, "Run as personal build")
	localChangesFlag := cmd.Flags().VarPF(&localChangesValue{val: &opts.localChanges}, "local-changes", "l", "Include local changes (git, -, or path; default: git)")
	localChangesFlag.NoOptDefVal = "git"
	cmd.Flags().BoolVar(&opts.noPush, "no-push", false, "Skip auto-push of branch to remote")
	cmd.Flags().BoolVar(&opts.cleanSources, "clean", false, "Clean sources before run")
	cmd.Flags().BoolVar(&opts.rebuildDeps, "rebuild-deps", false, "Rebuild all dependencies")
	cmd.Flags().BoolVar(&opts.rebuildFailedDeps, "rebuild-failed-deps", false, "Rebuild failed/incomplete dependencies")
	cmd.Flags().BoolVar(&opts.queueAtTop, "top", false, "Add to top of queue")
	cmd.Flags().IntVar(&opts.agent, "agent", 0, "Run on specific agent (by ID)")
	opts.addToCmd(cmd)
	cmd.Flags().BoolVarP(&opts.web, "web", "w", false, "Open run in browser")
	cmd.Flags().BoolVarP(&opts.dryRun, "dry-run", "n", false, "Show what would be triggered without running")
	cmd.Flags().BoolVar(&opts.json, "json", false, "Output as JSON (for scripting)")

	return cmd
}

func runRunStart(f *cmdutil.Factory, jobID string, opts *runStartOptions) error {
	p := f.Printer
	opts.resolve()
	if opts.dryRun {
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
		if opts.queueAtTop {
			_, _ = fmt.Fprintln(p.Out, "  Queue at top: yes")
		}
		if opts.agent > 0 {
			_, _ = fmt.Fprintf(p.Out, "  Agent ID: %d\n", opts.agent)
		}
		return nil
	}

	if opts.localChanges != "" && opts.branch == "" {
		if !isGitRepo() {
			return tcerrors.WithSuggestion(
				"not a git repository",
				"Run this command from within a git repository, or specify --branch explicitly",
			)
		}
		branch, err := getCurrentBranch()
		if err != nil {
			return err
		}
		opts.branch = branch
		p.Info("Using current branch: %s", branch)
	}

	if opts.localChanges != "" && !opts.noPush {
		if !branchExistsOnRemote(opts.branch) {
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
		description := opts.comment
		if description == "" {
			description = "Personal build with local changes"
		}

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
	})
	if err != nil {
		return err
	}

	if opts.json {
		if opts.watch {
			return doRunWatch(f, fmt.Sprintf("%d", build.ID), opts.watchOpts(false, true))
		}
		return p.PrintJSON(build)
	}

	printQueuedRun(p, build, jobID)

	if opts.branch != "" {
		p.Info("  Branch: %s", opts.branch)
	}
	if opts.comment != "" {
		p.Info("  Comment: %s", opts.comment)
	}
	if len(opts.tags) > 0 {
		p.Info("  Tags: %s", strings.Join(opts.tags, ", "))
	}
	p.Info("  URL: %s", build.WebURL)
	if opts.agent > 0 {
		_, _ = fmt.Fprintf(p.Out, "  %s teamcity agent term %d\n", output.Faint("Agent terminal:"), opts.agent)
	} else {
		_, _ = fmt.Fprintf(p.Out, "  %s teamcity agent term <agent-id>\n", output.Faint("Agent terminal:"))
	}

	return afterQueue(f, build, opts.web, &opts.watchFlags)
}
