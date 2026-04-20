package output

import "fmt"

// FormatHint returns "Hint: <text>" with a Yellow prefix (plain when NO_COLOR).
func FormatHint(hint string) string {
	return fmt.Sprintf("%s %s", Yellow("Hint:"), hint)
}

// Empty-state hint constants — one canonical copy per list surface.
const (
	HintNoRuns         = "Try --since 7d for a wider window, or --all for everything"
	HintNoFavoriteRuns = "Pin a run with 'teamcity run pin <id>'"
	HintNoAgents       = "Check connectivity or run 'teamcity auth status'"
	HintNoProjects     = "Check your permissions or run 'teamcity auth status'"
	HintNoJobs         = "Verify the project with 'teamcity project list'"
	HintNoPipelines    = "Enable pipelines on the server, or check 'teamcity project list'"
	HintNoQueue        = "Nothing is queued; 'teamcity run list' shows recent runs"
	HintNoPools        = "Contact your administrator to create an agent pool"
	HintNoArtifacts    = "Use 'teamcity run log <id>' to view build output"
	HintNoLog          = "The run may still be queued; 'teamcity run view <id>' shows its state"
	HintNoComment      = "Add one with 'teamcity run comment <id> --set \"<text>\"'"
	HintNoConnections  = "Add one in the TeamCity UI under project → Connections"
	HintNoParameters   = "Add one with 'teamcity param set <scope> <name> <value>'"
)
