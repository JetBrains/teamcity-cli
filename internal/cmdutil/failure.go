package cmdutil

import (
	"fmt"
	"strings"
	"time"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/output"
)

const maxFailedTestsToShow = 10

func PrintFailureSummary(client api.ClientInterface, buildID, buildNumber, webURL, statusText string) {
	header := fmt.Sprintf("%s Build %s  #%s failed", output.Red("✗"), buildID, buildNumber)
	if statusText != "" {
		header += ": " + statusText
	}
	fmt.Printf("\n%s\n", header)

	var hasTests bool
	var testsErr error
	var tests *api.TestOccurrences

	tests, testsErr = client.GetBuildTests(buildID, true, maxFailedTestsToShow)
	if testsErr != nil {
		output.Debug("Failed to fetch build tests: %v", testsErr)
	} else if tests.Failed > 0 {
		hasTests = true
	}

	if problems, err := client.GetBuildProblems(buildID); err != nil {
		output.Debug("Failed to fetch build problems: %v", err)
	} else if problems.Count > 0 {
		fmt.Printf("\nProblems:\n")
		for _, p := range problems.ProblemOccurrence {
			if hasTests && p.Type == "TC_FAILED_TESTS" {
				continue
			}
			detail := p.Details
			if detail == "" {
				detail = p.Identity
			}
			fmt.Printf("  %s %s\n", output.Red("•"), detail)
		}
	}

	if testsErr == nil && tests != nil && tests.Failed > 0 {
		fmt.Printf("\nFailed tests (%d):\n", tests.Failed)
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
			fmt.Println(line)
			if t.Details != "" {
				for dl := range strings.SplitSeq(strings.TrimSpace(t.Details), "\n") {
					fmt.Printf("    %s\n", output.Faint(dl))
				}
			}
		}
		if tests.Failed > len(tests.TestOccurrence) {
			fmt.Printf("  %s\n", output.Faint(fmt.Sprintf("... and %d more", tests.Failed-len(tests.TestOccurrence))))
		}
	}

	fmt.Printf("\nView details: %s\n", webURL)
}

// BuildResultError prints the final build result and returns an appropriate exit error.
// Used by both the standard watch and TUI watch paths.
func BuildResultError(client api.ClientInterface, build *api.Build, showDetails bool) error {
	jobName := build.BuildTypeID
	if build.BuildType != nil {
		jobName = build.BuildType.Name
	}

	switch build.Status {
	case "SUCCESS":
		fmt.Printf("%s %s %d  #%s succeeded\n", output.Green("✓"), output.Cyan(jobName), build.ID, build.Number)
		if showDetails {
			fmt.Printf("\nView details: %s\n", build.WebURL)
		}
		return nil
	case "FAILURE":
		PrintFailureSummary(client, fmt.Sprintf("%d", build.ID), build.Number, build.WebURL, build.StatusText)
		return &ExitError{Code: ExitFailure}
	default:
		fmt.Printf("%s Build %d  #%s canceled\n", output.Yellow("○"), build.ID, build.Number)
		return &ExitError{Code: ExitCancelled}
	}
}
