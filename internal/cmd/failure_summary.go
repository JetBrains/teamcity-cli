package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/JetBrains/teamcity-cli/internal/api"
	"github.com/JetBrains/teamcity-cli/internal/output"
)

const maxFailedTestsToShow = 10

func printFailureSummary(client api.ClientInterface, buildID, buildNumber, webURL, statusText string) {
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
			// Skip TC_FAILED_TESTS when we already show the tests section.
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
			fmt.Println(line)
			if t.Details != "" {
				msg := firstLine(t.Details)
				msg = output.Truncate(msg, 120)
				fmt.Printf("    %s\n", output.Faint(msg))
			}
		}
		if tests.Failed > len(tests.TestOccurrence) {
			fmt.Printf("  %s\n", output.Faint(fmt.Sprintf("... and %d more", tests.Failed-len(tests.TestOccurrence))))
		}
	}

	fmt.Printf("\nView details: %s\n", webURL)
}

// firstLine returns the first non-empty line of s.
func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if before, _, ok := strings.Cut(s, "\n"); ok {
		return strings.TrimSpace(before)
	}
	return s
}
