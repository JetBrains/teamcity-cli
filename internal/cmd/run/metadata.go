package run

import (
	"fmt"
	"strings"

	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/spf13/cobra"
)

func newRunPinCmd(f *cmdutil.Factory) *cobra.Command {
	var comment string
	cmd := &cobra.Command{
		Use:   "pin <id>",
		Short: "Pin to prevent cleanup",
		Long:  `Pin a run to prevent it from being automatically cleaned up by retention policies.`,
		Args:  cobra.ExactArgs(1),
		Example: `  teamcity run pin 12345
  teamcity run pin 12345 --comment "Release candidate"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.Client()
			if err != nil {
				return err
			}
			if err := client.PinBuild(args[0], comment); err != nil {
				return fmt.Errorf("failed to pin run: %w", err)
			}
			f.Printer.Success("Pinned #%s", args[0])
			if comment != "" {
				f.Printer.Info("  Comment: %s", comment)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&comment, "comment", "m", "", "Reason for pinning")
	return cmd
}

func newRunUnpinCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:     "unpin <id>",
		Short:   "Unpin a run",
		Long:    `Remove the pin from a run, allowing it to be cleaned up by retention policies.`,
		Args:    cobra.ExactArgs(1),
		Example: `  teamcity run unpin 12345`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.Client()
			if err != nil {
				return err
			}
			if err := client.UnpinBuild(args[0]); err != nil {
				return fmt.Errorf("failed to unpin run: %w", err)
			}
			f.Printer.Success("Unpinned #%s", args[0])
			return nil
		},
	}
}

func newRunTagCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tag <id> <tag>...",
		Short: "Add tags",
		Long:  `Add one or more tags for categorization and filtering.`,
		Args:  cobra.MinimumNArgs(2),
		Example: `  teamcity run tag 12345 release
  teamcity run tag 12345 release v1.0 production`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRunTag(f, args[0], args[1:])
		},
	}

	return cmd
}

func runRunTag(f *cmdutil.Factory, runID string, tags []string) error {
	var filtered []string
	for _, t := range tags {
		if t != "" {
			filtered = append(filtered, t)
		}
	}
	if len(filtered) == 0 {
		return fmt.Errorf("at least one non-empty tag is required")
	}
	tags = filtered

	client, err := f.Client()
	if err != nil {
		return err
	}

	if err := client.AddBuildTags(runID, tags); err != nil {
		return fmt.Errorf("failed to add tags: %w", err)
	}

	f.Printer.Success("Added %d tag(s) to #%s", len(tags), runID)
	f.Printer.Info("  Tags: %s", strings.Join(tags, ", "))
	return nil
}

func newRunUntagCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "untag <id> <tag>...",
		Short: "Remove tags",
		Long:  `Remove one or more tags.`,
		Args:  cobra.MinimumNArgs(2),
		Example: `  teamcity run untag 12345 release
  teamcity run untag 12345 release v1.0`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRunUntag(f, args[0], args[1:])
		},
	}

	return cmd
}

func runRunUntag(f *cmdutil.Factory, runID string, tags []string) error {
	client, err := f.Client()
	if err != nil {
		return err
	}

	var errors []string
	removed := 0
	for _, tag := range tags {
		if err := client.RemoveBuildTag(runID, tag); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", tag, err))
		} else {
			removed++
		}
	}

	if removed > 0 {
		f.Printer.Success("Removed %d tag(s) from #%s", removed, runID)
	}

	if len(errors) > 0 {
		for _, e := range errors {
			f.Printer.Warn("  Failed: %s", e)
		}
		if removed == 0 {
			return fmt.Errorf("failed to remove any tags")
		}
	}

	return nil
}

type runCommentOptions struct {
	delete bool
	json   bool
}

func newRunCommentCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &runCommentOptions{}

	cmd := &cobra.Command{
		Use:   "comment <id> [comment]",
		Short: "Set or view comment",
		Long: `Set, view, or delete a comment on a run.

Without a comment argument, displays the current comment.
With a comment argument, sets the comment.
Use --delete to remove the comment.`,
		Args: cobra.RangeArgs(1, 2),
		Example: `  teamcity run comment 12345
  teamcity run comment 12345 --json
  teamcity run comment 12345 "Deployed to production"
  teamcity run comment 12345 --delete`,
		RunE: func(cmd *cobra.Command, args []string) error {
			comment := ""
			if len(args) > 1 {
				comment = args[1]
			}
			return runRunComment(f, args[0], comment, opts)
		},
	}

	cmd.Flags().BoolVar(&opts.delete, "delete", false, "Delete the comment")
	cmd.Flags().BoolVar(&opts.json, "json", false, "Output as JSON")

	return cmd
}

func runRunComment(f *cmdutil.Factory, runID string, comment string, opts *runCommentOptions) error {
	client, err := f.Client()
	if err != nil {
		return err
	}

	if opts.delete {
		if err := client.DeleteBuildComment(runID); err != nil {
			return fmt.Errorf("failed to delete comment: %w", err)
		}
		if opts.json {
			return f.Printer.PrintJSON(map[string]string{"run_id": runID, "comment": ""})
		}
		f.Printer.Success("Deleted comment from #%s", runID)
		return nil
	}

	if comment != "" {
		if err := client.SetBuildComment(runID, comment); err != nil {
			return fmt.Errorf("failed to set comment: %w", err)
		}
		if opts.json {
			return f.Printer.PrintJSON(map[string]string{"run_id": runID, "comment": comment})
		}
		f.Printer.Success("Set comment on #%s", runID)
		f.Printer.Info("  Comment: %s", comment)
		return nil
	}

	existingComment, err := client.GetBuildComment(runID)
	if err != nil {
		return fmt.Errorf("failed to get comment: %w", err)
	}

	if opts.json {
		return f.Printer.PrintJSON(map[string]string{"run_id": runID, "comment": existingComment})
	}

	p := f.Printer
	if existingComment == "" {
		p.Info("No comment set on #%s", runID)
	} else {
		_, _ = fmt.Fprintln(p.Out, existingComment)
	}
	return nil
}
