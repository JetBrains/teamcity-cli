package run

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
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
	cmd.Flags().IntVarP(&w.interval, "interval", "i", 3, "Refresh interval in seconds when watching")
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

func printQueuedRun(build *api.Build, context string) {
	ref := fmt.Sprintf("%d  #%s", build.ID, build.Number)
	if build.Number == "" {
		ref = fmt.Sprintf("%d", build.ID)
	}
	output.Success("Queued run %s for %s", ref, context)
}

func afterQueue(f *cmdutil.Factory, build *api.Build, web bool, wf *watchFlags) error {
	if web {
		_ = browser.OpenURL(build.WebURL)
	}
	if wf.watch {
		fmt.Println()
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
	opts.resolve()
	if opts.dryRun {
		fmt.Printf("%s Would trigger run for %s\n", output.Faint("[dry-run]"), output.Cyan(jobID))
		if opts.branch != "" {
			fmt.Printf("  Branch: %s\n", opts.branch)
		}
		if opts.revision != "" {
			fmt.Printf("  Revision: %s\n", opts.revision)
		}
		if len(opts.params) > 0 {
			fmt.Println("  Parameters:")
			for k, v := range opts.params {
				fmt.Printf("    %s=%s\n", k, v)
			}
		}
		if len(opts.systemProps) > 0 {
			fmt.Println("  System properties:")
			for k, v := range opts.systemProps {
				fmt.Printf("    %s=%s\n", k, v)
			}
		}
		if len(opts.envVars) > 0 {
			fmt.Println("  Environment variables:")
			for k, v := range opts.envVars {
				fmt.Printf("    %s=%s\n", k, v)
			}
		}
		if opts.comment != "" {
			fmt.Printf("  Comment: %s\n", opts.comment)
		}
		if len(opts.tags) > 0 {
			fmt.Printf("  Tags: %s\n", strings.Join(opts.tags, ", "))
		}
		if opts.personal || opts.localChanges != "" {
			fmt.Println("  Personal build: yes")
		}
		if opts.localChanges != "" {
			fmt.Printf("  Local changes: %s\n", opts.localChanges)
		}
		if opts.cleanSources {
			fmt.Println("  Clean sources: yes")
		}
		if opts.rebuildDeps {
			fmt.Println("  Rebuild dependencies: yes")
		}
		if opts.queueAtTop {
			fmt.Println("  Queue at top: yes")
		}
		if opts.agent > 0 {
			fmt.Printf("  Agent ID: %d\n", opts.agent)
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
		output.Info("Using current branch: %s", branch)
	}

	if opts.localChanges != "" && !opts.noPush {
		if !branchExistsOnRemote(opts.branch) {
			output.Info("Pushing branch to remote...")
			if err := pushBranch(opts.branch); err != nil {
				return err
			}
			output.Success("Branch pushed to remote")
		}
	}

	client, err := f.Client()
	if err != nil {
		return err
	}

	var personalChangeID string
	if opts.localChanges != "" {
		patch, err := loadLocalChanges(opts.localChanges)
		if err != nil {
			return err
		}

		output.Info("Uploading local changes...")
		description := opts.comment
		if description == "" {
			description = "Personal build with local changes"
		}

		changeID, err := client.UploadDiffChanges(patch, description)
		if err != nil {
			return fmt.Errorf("failed to upload changes: %w", err)
		}
		personalChangeID = changeID
		output.Success("Uploaded changes (ID: %s)", changeID)

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
		return output.PrintJSON(build)
	}

	printQueuedRun(build, jobID)

	if opts.branch != "" {
		output.Info("  Branch: %s", opts.branch)
	}
	if opts.comment != "" {
		output.Info("  Comment: %s", opts.comment)
	}
	if len(opts.tags) > 0 {
		output.Info("  Tags: %s", strings.Join(opts.tags, ", "))
	}
	output.Info("  URL: %s", build.WebURL)
	if opts.agent > 0 {
		fmt.Printf("  %s teamcity agent term %d\n", output.Faint("Agent terminal:"), opts.agent)
	} else {
		fmt.Printf("  %s teamcity agent term <agent-id>\n", output.Faint("Agent terminal:"))
	}

	return afterQueue(f, build, opts.web, &opts.watchFlags)
}

type runRestartOptions struct {
	watchFlags
	web bool
}

func newRunRestartCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &runRestartOptions{}

	cmd := &cobra.Command{
		Use:   "restart <run-id>",
		Short: "Restart a run",
		Long:  `Restart a run with the same configuration.`,
		Args:  cobra.ExactArgs(1),
		Example: `  teamcity run restart 12345
  teamcity run restart 12345 --watch`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRunRestart(f, args[0], opts)
		},
	}

	opts.addToCmd(cmd)
	cmd.Flags().BoolVarP(&opts.web, "web", "w", false, "Open run in browser")

	return cmd
}

func runRunRestart(f *cmdutil.Factory, runID string, opts *runRestartOptions) error {
	opts.resolve()

	client, err := f.Client()
	if err != nil {
		return err
	}

	originalBuild, err := client.GetBuild(runID)
	if err != nil {
		return fmt.Errorf("failed to get run: %w", err)
	}

	newBuild, err := client.RunBuild(originalBuild.BuildTypeID, api.RunBuildOptions{
		Branch: originalBuild.BranchName,
	})
	if err != nil {
		return fmt.Errorf("failed to trigger run: %w", err)
	}

	printQueuedRun(newBuild, fmt.Sprintf("%s (restart of %d)", originalBuild.BuildTypeID, originalBuild.ID))
	fmt.Printf("  Job: %s\n", originalBuild.BuildTypeID)
	if originalBuild.BranchName != "" {
		fmt.Printf("  Branch: %s\n", originalBuild.BranchName)
	}
	output.Info("  URL: %s", newBuild.WebURL)

	return afterQueue(f, newBuild, opts.web, &opts.watchFlags)
}

type localChangesValue struct {
	val *string
}

func (v *localChangesValue) String() string {
	if v.val == nil {
		return ""
	}
	return *v.val
}

func (v *localChangesValue) Set(s string) error {
	*v.val = s
	return nil
}

func (v *localChangesValue) Type() string {
	return "string"
}

func loadLocalChanges(source string) ([]byte, error) {
	switch source {
	case "git":
		if !isGitRepo() {
			return nil, tcerrors.WithSuggestion(
				"not a git repository",
				"Run this command from within a git repository, or use --local-changes <path> to specify a diff file",
			)
		}
		patch, err := getGitDiff()
		if err != nil {
			return nil, err
		}
		if len(patch) == 0 {
			return nil, tcerrors.WithSuggestion(
				"no uncommitted changes found",
				"Make some changes to your files before running a personal build, or use --local-changes <path> to specify a diff file",
			)
		}
		return patch, nil
	case "-":
		patch, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("failed to read from stdin: %w", err)
		}
		if len(patch) == 0 {
			return nil, tcerrors.WithSuggestion(
				"no changes provided via stdin",
				"Pipe a diff file to stdin, e.g.: git diff | teamcity run start Job --local-changes -",
			)
		}
		return patch, nil
	default:
		patch, err := os.ReadFile(source)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, tcerrors.WithSuggestion(
					fmt.Sprintf("diff file not found: %s", source),
					"Check the file path and try again",
				)
			}
			return nil, fmt.Errorf("failed to read diff file: %w", err)
		}
		if len(patch) == 0 {
			return nil, tcerrors.WithSuggestion(
				fmt.Sprintf("diff file is empty: %s", source),
				"Provide a non-empty diff file",
			)
		}
		return patch, nil
	}
}

func getGitDiff() ([]byte, error) {
	untrackedFiles, err := getUntrackedFiles()
	if err != nil {
		return nil, fmt.Errorf("failed to get untracked files: %w", err)
	}

	if len(untrackedFiles) > 0 {
		addArgs := append([]string{"add", "-N", "--"}, untrackedFiles...)
		addCmd := exec.Command("git", addArgs...)
		if err := addCmd.Run(); err != nil {
			output.Debug("Failed to stage untracked files: %v", err)
		} else {
			defer func() {
				resetArgs := append([]string{"reset", "HEAD", "--"}, untrackedFiles...)
				resetCmd := exec.Command("git", resetArgs...)
				_ = resetCmd.Run()
			}()
		}
	}

	cmd := exec.Command("git", "diff", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return nil, tcerrors.WithSuggestion(
			"failed to generate git diff",
			"Ensure you have at least one commit in your repository",
		)
	}
	return out, nil
}

type runCancelOptions struct {
	comment string
	force   bool
}

func newRunCancelCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &runCancelOptions{}

	cmd := &cobra.Command{
		Use:   "cancel <run-id>",
		Short: "Cancel a running build",
		Long:  `Cancel a running or queued run.`,
		Args:  cobra.ExactArgs(1),
		Example: `  teamcity run cancel 12345
  teamcity run cancel 12345 --comment "Canceling for hotfix"
  teamcity run cancel 12345 --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRunCancel(f, args[0], opts)
		},
	}

	cmd.Flags().StringVar(&opts.comment, "comment", "", "Comment for cancellation")
	cmd.Flags().BoolVarP(&opts.force, "force", "f", false, "Skip confirmation prompt")

	return cmd
}

func runRunCancel(f *cmdutil.Factory, runID string, opts *runCancelOptions) error {
	client, err := f.Client()
	if err != nil {
		return err
	}

	needsConfirmation := !opts.force && opts.comment == "" && f.IsInteractive()

	if needsConfirmation {
		var confirm bool
		prompt := &survey.Confirm{
			Message: fmt.Sprintf("Cancel run #%s?", runID),
			Default: false,
		}
		if err := survey.AskOne(prompt, &confirm); err != nil {
			return err
		}
		if !confirm {
			output.Info("Canceled")
			return nil
		}
	}

	comment := opts.comment
	if comment == "" {
		comment = "Canceled via teamcity CLI"
	}

	if err := client.CancelBuild(runID, comment); err != nil {
		return err
	}

	output.Success("Canceled run #%s", runID)
	return nil
}
