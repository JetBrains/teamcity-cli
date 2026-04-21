package cmdutil

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/output"
)

const maxFailedTestsToShow = 10

func PrintFailureSummary(ctx context.Context, p *output.Printer, client api.ClientInterface, buildID, buildNumber, webURL, statusText string) {
	header := fmt.Sprintf("%s %s  #%s failed", output.Red("✗"), buildID, buildNumber)
	if statusText != "" {
		header += ": " + statusText
	}
	_, _ = fmt.Fprintf(p.Out, "\n%s\n", header)

	var hasTests bool
	var testsErr error
	var tests *api.TestOccurrences

	tests, testsErr = client.GetBuildTests(ctx, buildID, true, maxFailedTestsToShow)
	if testsErr != nil {
		p.Debug("Failed to fetch tests: %v", testsErr)
	} else if tests.Failed > 0 {
		hasTests = true
	}

	if problems, err := client.GetBuildProblems(buildID); err != nil {
		p.Debug("Failed to fetch problems: %v", err)
	} else if problems.Count > 0 {
		_, _ = fmt.Fprintf(p.Out, "\nProblems:\n")
		for _, prob := range problems.ProblemOccurrence {
			if hasTests && prob.Type == "TC_FAILED_TESTS" {
				continue
			}
			detail := prob.Details
			if detail == "" {
				detail = prob.Identity
			}
			_, _ = fmt.Fprintf(p.Out, "  %s %s\n", output.Red("•"), detail)
		}
	}

	if testsErr == nil && tests != nil && tests.Failed > 0 {
		_, _ = fmt.Fprintf(p.Out, "\nFailed tests (%d):\n", tests.Failed)
		for _, t := range tests.TestOccurrence {
			line := fmt.Sprintf("  %s %s", output.Red("•"), t.Name)
			if t.Duration > 0 {
				dur := time.Duration(t.Duration) * time.Millisecond
				line += " " + output.Faint("("+output.FormatDuration(dur)+")")
			}
			if t.NewFailure {
				line += " " + output.Yellow("(new)")
			} else if t.FirstFailed != nil && t.FirstFailed.Build != nil {
				line += " " + output.Faint(fmt.Sprintf("(failing since #%s)", t.FirstFailed.Build.Number))
			}
			_, _ = fmt.Fprintln(p.Out, line)
			if t.Details != "" {
				for dl := range strings.SplitSeq(strings.TrimSpace(t.Details), "\n") {
					_, _ = fmt.Fprintf(p.Out, "    %s\n", output.Faint(dl))
				}
			}
		}
		if tests.Failed > len(tests.TestOccurrence) {
			_, _ = fmt.Fprintf(p.Out, "  %s\n", output.Faint(fmt.Sprintf("... and %d more", tests.Failed-len(tests.TestOccurrence))))
		}
	}

	_, _ = fmt.Fprintf(p.Out, "\nView details: %s\n", webURL)
}

// BuildResultError prints the final build result and returns an appropriate exit error.
// Used by both the standard watch and TUI watch paths.
func BuildResultError(ctx context.Context, p *output.Printer, client api.ClientInterface, build *api.Build, showDetails bool) error {
	jobName := build.BuildTypeID
	if build.BuildType != nil {
		jobName = build.BuildType.Name
	}

	switch build.Status {
	case "SUCCESS":
		_, _ = fmt.Fprintf(p.Out, "%s %s %d  #%s succeeded\n", output.Green("✓"), output.Cyan(jobName), build.ID, build.Number)
		if showDetails {
			_, _ = fmt.Fprintf(p.Out, "\nView details: %s\n", build.WebURL)
		}
		return nil
	case "FAILURE":
		PrintFailureSummary(ctx, p, client, strconv.Itoa(build.ID), build.Number, build.WebURL, build.StatusText)
		return &ExitError{Code: ExitFailure}
	default:
		_, _ = fmt.Fprintf(p.Out, "%s %s %d  #%s canceled\n", output.Yellow("○"), output.Cyan(jobName), build.ID, build.Number)
		return &ExitError{Code: ExitCancelled}
	}
}
