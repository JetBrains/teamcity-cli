package cmd

import (
	"fmt"
	"strings"

	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/spf13/cobra"
)

func newRunPinCmd() *cobra.Command {
	var comment string
	cmd := &cobra.Command{
		Use:   "pin <run-id>",
		Short: "Pin a run to prevent cleanup",
		Long:  `Pin a run to prevent it from being automatically cleaned up by retention policies.`,
		Args:  cobra.ExactArgs(1),
		Example: `  teamcity run pin 12345
  teamcity run pin 12345 --comment "Release candidate"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := getClient()
			if err != nil {
				return err
			}
			if err := client.PinBuild(args[0], comment); err != nil {
				return fmt.Errorf("failed to pin run: %w", err)
			}
			output.Success("Pinned run #%s", args[0])
			if comment != "" {
				output.Info("  Comment: %s", comment)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&comment, "comment", "m", "", "Comment explaining why the run is pinned")
	return cmd
}

func newRunUnpinCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "unpin <run-id>",
		Short:   "Unpin a run",
		Long:    `Remove the pin from a run, allowing it to be cleaned up by retention policies.`,
		Args:    cobra.ExactArgs(1),
		Example: `  teamcity run unpin 12345`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := getClient()
			if err != nil {
				return err
			}
			if err := client.UnpinBuild(args[0]); err != nil {
				return fmt.Errorf("failed to unpin run: %w", err)
			}
			output.Success("Unpinned run #%s", args[0])
			return nil
		},
	}
}

func newRunTagCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tag <run-id> <tag>...",
		Short: "Add tags to a run",
		Long:  `Add one or more tags to a run for categorization and filtering.`,
		Args:  cobra.MinimumNArgs(2),
		Example: `  teamcity run tag 12345 release
  teamcity run tag 12345 release v1.0 production`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRunTag(args[0], args[1:])
		},
	}

	return cmd
}

func runRunTag(runID string, tags []string) error {
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

	client, err := getClient()
	if err != nil {
		return err
	}

	if err := client.AddBuildTags(runID, tags); err != nil {
		return fmt.Errorf("failed to add tags: %w", err)
	}

	output.Success("Added %d tag(s) to run #%s", len(tags), runID)
	output.Info("  Tags: %s", strings.Join(tags, ", "))
	return nil
}

func newRunUntagCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "untag <run-id> <tag>...",
		Short: "Remove tags from a run",
		Long:  `Remove one or more tags from a run.`,
		Args:  cobra.MinimumNArgs(2),
		Example: `  teamcity run untag 12345 release
  teamcity run untag 12345 release v1.0`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRunUntag(args[0], args[1:])
		},
	}

	return cmd
}

func runRunUntag(runID string, tags []string) error {
	client, err := getClient()
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
		output.Success("Removed %d tag(s) from run #%s", removed, runID)
	}

	if len(errors) > 0 {
		for _, e := range errors {
			output.Warn("  Failed: %s", e)
		}
		if removed == 0 {
			return fmt.Errorf("failed to remove any tags")
		}
	}

	return nil
}

type runCommentOptions struct {
	delete bool
}

func newRunCommentCmd() *cobra.Command {
	opts := &runCommentOptions{}

	cmd := &cobra.Command{
		Use:   "comment <run-id> [comment]",
		Short: "Set or view run comment",
		Long: `Set, view, or delete a comment on a run.

Without a comment argument, displays the current comment.
With a comment argument, sets the comment.
Use --delete to remove the comment.`,
		Args: cobra.RangeArgs(1, 2),
		Example: `  teamcity run comment 12345
  teamcity run comment 12345 "Deployed to production"
  teamcity run comment 12345 --delete`,
		RunE: func(cmd *cobra.Command, args []string) error {
			comment := ""
			if len(args) > 1 {
				comment = args[1]
			}
			return runRunComment(args[0], comment, opts)
		},
	}

	cmd.Flags().BoolVar(&opts.delete, "delete", false, "Delete the comment")

	return cmd
}

func runRunComment(runID string, comment string, opts *runCommentOptions) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	if opts.delete {
		if err := client.DeleteBuildComment(runID); err != nil {
			return fmt.Errorf("failed to delete comment: %w", err)
		}
		output.Success("Deleted comment from run #%s", runID)
		return nil
	}

	if comment != "" {
		if err := client.SetBuildComment(runID, comment); err != nil {
			return fmt.Errorf("failed to set comment: %w", err)
		}
		output.Success("Set comment on run #%s", runID)
		output.Info("  Comment: %s", comment)
		return nil
	}

	existingComment, err := client.GetBuildComment(runID)
	if err != nil {
		return fmt.Errorf("failed to get comment: %w", err)
	}

	if existingComment == "" {
		output.Info("No comment set on run #%s", runID)
	} else {
		fmt.Println(existingComment)
	}
	return nil
}
