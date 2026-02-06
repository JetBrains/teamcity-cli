package cmd

import (
	"fmt"

	"github.com/JetBrains/teamcity-cli/internal/api"
	"github.com/JetBrains/teamcity-cli/internal/output"
)

const maxFailedTestsToShow = 10

func printFailureSummary(client api.ClientInterface, buildID, buildNumber, webURL string) {
	fmt.Printf("\n%s Build #%s failed\n", output.Red("✗"), buildNumber)

	if problems, err := client.GetBuildProblems(buildID); err != nil {
		output.Debug("Failed to fetch build problems: %v", err)
	} else if problems.Count > 0 {
		fmt.Printf("\nProblems:\n")
		for _, p := range problems.ProblemOccurrence {
			detail := p.Details
			if detail == "" {
				detail = p.Identity
			}
			fmt.Printf("  %s %s\n", output.Red("•"), detail)
		}
	}

	if tests, err := client.GetBuildTests(buildID, true, maxFailedTestsToShow); err != nil {
		output.Debug("Failed to fetch build tests: %v", err)
	} else if tests.Failed > 0 {
		fmt.Printf("\nFailed tests (%d):\n", tests.Failed)
		for _, t := range tests.TestOccurrence {
			fmt.Printf("  %s %s\n", output.Red("•"), t.Name)
		}
		if tests.Failed > len(tests.TestOccurrence) {
			fmt.Printf("  %s\n", output.Faint(fmt.Sprintf("... and %d more", tests.Failed-len(tests.TestOccurrence))))
		}
	}

	fmt.Printf("\nView details: %s\n", webURL)
}
