package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/JetBrains/teamcity-cli/internal/api"
	tcerrors "github.com/JetBrains/teamcity-cli/internal/errors"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
)

type runStartOptions struct {
	branch            string
	params            map[string]string
	systemProps       map[string]string
	envVars           map[string]string
	comment           string
	personal          bool
	localChanges      string // path to a diff file, "-" for stdin, or "git" to auto-generate
	noPush            bool   // skip auto-push of branch to remote
	cleanSources      bool
	rebuildDeps       bool
	rebuildFailedDeps bool
	queueAtTop        bool
	agent             int
	tags              []string
	watch             bool
	web               bool
	dryRun            bool
	json              bool
}

func newRunStartCmd() *cobra.Command {
	opts := &runStartOptions{
		params:      make(map[string]string),
		systemProps: make(map[string]string),
		envVars:     make(map[string]string),
	}

	cmd := &cobra.Command{
		Use:   "start <job-id>",
		Short: "Start a new run",
		Args:  cobra.ExactArgs(1),
		Example: `  tc run start Falcon_Build
  tc run start Falcon_Build --branch feature/test
  tc run start Falcon_Build -P version=1.0 -S build.number=123 -E CI=true
  tc run start Falcon_Build --comment "Release build" --tag release --tag v1.0
  tc run start Falcon_Build --clean --rebuild-deps --top
  tc run start Falcon_Build --local-changes # personal build with uncommitted Git changes
  tc run start Falcon_Build --local-changes changes.patch  # from file
  tc run start Falcon_Build --dry-run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRunStart(args[0], opts)
		},
	}

	cmd.Flags().StringVarP(&opts.branch, "branch", "b", "", "Branch to build")
	cmd.Flags().StringToStringVarP(&opts.params, "param", "P", nil, "Build parameters (key=value)")
	cmd.Flags().StringToStringVarP(&opts.systemProps, "system", "S", nil, "System properties (key=value)")
	cmd.Flags().StringToStringVarP(&opts.envVars, "env", "E", nil, "Environment variables (key=value)")
	cmd.Flags().StringVarP(&opts.comment, "comment", "m", "", "Run comment")
	cmd.Flags().StringSliceVarP(&opts.tags, "tag", "t", nil, "Run tags (can be repeated)")
	cmd.Flags().BoolVar(&opts.personal, "personal", false, "Run as personal build")
	localChangesFlag := cmd.Flags().VarPF(&localChangesValue{val: &opts.localChanges}, "local-changes", "l", "Include local changes (git, p4, auto, -, or path; default: git)")
	localChangesFlag.NoOptDefVal = "git"
	cmd.Flags().BoolVar(&opts.noPush, "no-push", false, "Skip auto-push of branch to remote")
	cmd.Flags().BoolVar(&opts.cleanSources, "clean", false, "Clean sources before run")
	cmd.Flags().BoolVar(&opts.rebuildDeps, "rebuild-deps", false, "Rebuild all dependencies")
	cmd.Flags().BoolVar(&opts.rebuildFailedDeps, "rebuild-failed-deps", false, "Rebuild failed/incomplete dependencies")
	cmd.Flags().BoolVar(&opts.queueAtTop, "top", false, "Add to top of queue")
	cmd.Flags().IntVar(&opts.agent, "agent", 0, "Run on specific agent (by ID)")
	cmd.Flags().BoolVar(&opts.watch, "watch", false, "Watch run until it completes")
	cmd.Flags().BoolVarP(&opts.web, "web", "w", false, "Open run in browser")
	cmd.Flags().BoolVarP(&opts.dryRun, "dry-run", "n", false, "Show what would be triggered without running")
	cmd.Flags().BoolVar(&opts.json, "json", false, "Output as JSON (for scripting)")

	return cmd
}

func runRunStart(jobID string, opts *runStartOptions) error {
	if opts.dryRun {
		fmt.Printf("%s Would trigger run for %s\n", output.Faint("[dry-run]"), output.Cyan(jobID))
		if opts.branch != "" {
			fmt.Printf("  Branch: %s\n", opts.branch)
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

	var headCommit string
	var personalChangeID string
	var localPatch []byte

	if opts.localChanges != "" {
		vcs := DetectVCS()

		if opts.branch == "" {
			if vcs == nil {
				return tcerrors.WithSuggestion(
					"no supported VCS detected",
					"Run this command from within a git repository or Perforce workspace, or specify --branch explicitly",
				)
			}
			branch, err := vcs.GetCurrentBranch()
			if err != nil {
				return err
			}
			opts.branch = branch
			output.Info("Using current %s branch: %s", vcs.Name(), branch)
		}

		if !opts.noPush && vcs != nil && !vcs.BranchExistsOnRemote(opts.branch) {
			output.Info("Pushing branch to remote...")
			if err := vcs.PushBranch(opts.branch); err != nil {
				return err
			}
			output.Success("Branch pushed to remote")
		}

		if vcs != nil {
			commit, err := vcs.GetHeadRevision()
			if err != nil {
				return err
			}
			headCommit = commit
		}

		patch, err := loadLocalChanges(opts.localChanges)
		if err != nil {
			return err
		}
		localPatch = patch
		opts.personal = true
	}

	client, err := getClient()
	if err != nil {
		return err
	}

	if localPatch != nil {
		output.Info("Uploading local changes...")
		description := opts.comment
		if description == "" {
			description = "Personal build with local changes"
		}
		changeID, err := client.UploadDiffChanges(localPatch, description)
		if err != nil {
			return fmt.Errorf("failed to upload changes: %w", err)
		}
		personalChangeID = changeID
		output.Success("Uploaded changes (ID: %s)", changeID)
	}

	buildOpts := api.RunBuildOptions{
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
		Revision:                  headCommit,
	}

	build, err := client.RunBuild(jobID, buildOpts)
	if err != nil {
		return err
	}

	if opts.json {
		return output.PrintJSON(build)
	}

	runRef := fmt.Sprintf("#%s", build.Number)
	if build.Number == "" {
		runRef = fmt.Sprintf("(ID: %d)", build.ID)
	}
	output.Success("Queued run %s for %s", runRef, jobID)

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
		fmt.Printf("  %s tc agent term %d\n", output.Faint("Agent terminal:"), opts.agent)
	} else {
		fmt.Printf("  %s tc agent term <agent-id>\n", output.Faint("Agent terminal:"))
	}

	if opts.web {
		_ = browser.OpenURL(build.WebURL)
	}

	if opts.watch {
		fmt.Println()
		return doRunWatch(fmt.Sprintf("%d", build.ID), &runWatchOptions{interval: 3, logs: true})
	}

	return nil
}

type runCancelOptions struct {
	comment string
	force   bool
}

func newRunCancelCmd() *cobra.Command {
	opts := &runCancelOptions{}

	cmd := &cobra.Command{
		Use:   "cancel <run-id>",
		Short: "Cancel a running build",
		Long:  `Cancel a running or queued run.`,
		Args:  cobra.ExactArgs(1),
		Example: `  tc run cancel 12345
  tc run cancel 12345 --comment "Cancelling for hotfix"
  tc run cancel 12345 --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRunCancel(args[0], opts)
		},
	}

	cmd.Flags().StringVar(&opts.comment, "comment", "", "Comment for cancellation")
	cmd.Flags().BoolVarP(&opts.force, "force", "f", false, "Skip confirmation prompt")

	return cmd
}

func runRunCancel(runID string, opts *runCancelOptions) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	needsConfirmation := !opts.force && opts.comment == "" && !NoInput && output.IsStdinTerminal()

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
			output.Info("Cancelled")
			return nil
		}
	}

	comment := opts.comment
	if comment == "" {
		comment = "Cancelled via tc CLI"
	}

	if err := client.CancelBuild(runID, comment); err != nil {
		return err
	}

	output.Success("Cancelled run #%s", runID)
	return nil
}

type runWatchOptions struct {
	interval int
	logs     bool
}

func newRunWatchCmd() *cobra.Command {
	opts := &runWatchOptions{}

	cmd := &cobra.Command{
		Use:   "watch <run-id>",
		Short: "Watch a run until it completes",
		Long:  `Watch a run in real-time until it completes.`,
		Args:  cobra.ExactArgs(1),
		Example: `  tc run watch 12345
  tc run watch 12345 --interval 10
  tc run watch 12345 --logs`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return doRunWatch(args[0], opts)
		},
	}

	cmd.Flags().IntVarP(&opts.interval, "interval", "i", 5, "Refresh interval in seconds")
	cmd.Flags().BoolVar(&opts.logs, "logs", false, "Stream build logs while watching")

	return cmd
}

func doRunWatch(runID string, opts *runWatchOptions) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	if opts.logs {
		return runWatchTUI(client, runID, opts.interval)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	go func() {
		<-sigCh
		fmt.Println()
		fmt.Println()
		fmt.Println(output.Faint("Interrupted. Run continues in background."))
		fmt.Printf("%s Resume watching: tc run watch %s\n", output.Faint("Hint:"), runID)
		cancel()
	}()

	output.Info("Watching run #%s... %s\n", runID, output.Faint("(Ctrl-C to stop watching)"))

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		build, err := client.GetBuild(runID)
		if err != nil {
			return err
		}

		jobName := build.BuildTypeID
		if build.BuildType != nil {
			jobName = build.BuildType.Name
		}

		status := output.Yellow("Running")
		if build.State == "queued" {
			status = output.Faint("Queued")
		}
		progress := ""
		if build.PercentageComplete > 0 {
			progress = fmt.Sprintf(" (%d%%)", build.PercentageComplete)
		}
		fmt.Printf("\r%s %s #%s %s · %s%s    ",
			output.StatusIcon(build.Status, build.State),
			output.Cyan(jobName),
			build.Number,
			output.Faint(build.WebURL),
			status,
			progress)

		if build.State == "finished" {
			fmt.Println()
			fmt.Println()

			if build.Status == "SUCCESS" {
				fmt.Printf("%s %s #%s succeeded!\n", output.Green("✓"), output.Cyan(jobName), build.Number)
			} else {
				fmt.Printf("%s %s #%s failed: %s\n", output.Red("✗"), output.Cyan(jobName), build.Number, build.StatusText)
			}

			fmt.Printf("\nView details: %s\n", build.WebURL)
			return nil
		}

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(time.Duration(opts.interval) * time.Second):
		}
	}
}

type runRestartOptions struct {
	watch bool
	web   bool
}

func newRunRestartCmd() *cobra.Command {
	opts := &runRestartOptions{}

	cmd := &cobra.Command{
		Use:   "restart <run-id>",
		Short: "Restart a run",
		Long:  `Restart a run with the same configuration.`,
		Args:  cobra.ExactArgs(1),
		Example: `  tc run restart 12345
  tc run restart 12345 --watch`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRunRestart(args[0], opts)
		},
	}

	cmd.Flags().BoolVar(&opts.watch, "watch", false, "Watch the new run after restarting")
	cmd.Flags().BoolVarP(&opts.web, "web", "w", false, "Open run in browser")

	return cmd
}

func runRunRestart(runID string, opts *runRestartOptions) error {
	client, err := getClient()
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

	newRef := fmt.Sprintf("#%s", newBuild.Number)
	if newBuild.Number == "" {
		newRef = fmt.Sprintf("(ID: %d)", newBuild.ID)
	}
	fmt.Printf("%s Queued run %s (restart of #%s)\n", output.Green("✓"), newRef, originalBuild.Number)
	fmt.Printf("  Job: %s\n", originalBuild.BuildTypeID)
	if originalBuild.BranchName != "" {
		fmt.Printf("  Branch: %s\n", originalBuild.BranchName)
	}
	fmt.Printf("  URL: %s\n", newBuild.WebURL)

	if opts.web {
		_ = browser.OpenURL(newBuild.WebURL)
	}

	if opts.watch {
		fmt.Println()
		return doRunWatch(fmt.Sprintf("%d", newBuild.ID), &runWatchOptions{interval: 3, logs: true})
	}

	return nil
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
	case "-":
		patch, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("failed to read from stdin: %w", err)
		}
		if len(patch) == 0 {
			return nil, tcerrors.WithSuggestion(
				"no changes provided via stdin",
				"Pipe a diff file to stdin, e.g.: git diff | tc run start Job --local-changes -",
			)
		}
		return patch, nil
	case "git", "p4", "perforce", "auto":
		return loadVCSDiff(source)
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

func loadVCSDiff(source string) ([]byte, error) {
	var vcs VCSProvider
	if source == "auto" {
		vcs = DetectVCS()
	} else if p := DetectVCSByName(source); p != nil && p.IsAvailable() {
		vcs = p
	}
	if vcs == nil {
		return nil, tcerrors.WithSuggestion(
			"no supported VCS detected",
			"Run this command from within a git repository or Perforce workspace, or use --local-changes <path>",
		)
	}
	patch, err := vcs.GetLocalDiff()
	if err != nil {
		return nil, err
	}
	if len(patch) == 0 {
		return nil, tcerrors.WithSuggestion(
			"no local changes found",
			"Make some changes before running a personal build, or use --local-changes <path>",
		)
	}
	return patch, nil
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
