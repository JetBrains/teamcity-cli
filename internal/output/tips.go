package output

import "fmt"

// FormatTip returns "Tip: <text>" with a Yellow prefix (plain when NO_COLOR).
func FormatTip(tip string) string {
	return fmt.Sprintf("%s %s", Yellow("Tip:"), tip)
}

// Empty-state tip constants — one canonical copy per list surface.
const (
	TipNoRuns         = "Try --since 7d for a wider window, or --all for everything"
	TipNoFavoriteRuns = "Pin a run with 'teamcity run pin <id>'"
	TipNoAgents       = "Check connectivity or run 'teamcity auth status'"
	TipNoProjects     = "Check your permissions or run 'teamcity auth status'"
	TipNoJobs         = "Verify the project with 'teamcity project list'"
	TipNoPipelines    = "Enable pipelines on the server, or check 'teamcity project list'"
	TipNoQueue        = "Nothing is queued; 'teamcity run list' shows recent runs"
	TipNoPools        = "Contact your administrator to create an agent pool"
	TipNoArtifacts    = "Use 'teamcity run log <id>' to view build output"
	TipNoLog          = "The run may still be queued; 'teamcity run view <id>' shows its state"
	TipNoComment      = "Add one with 'teamcity run comment <id> --set \"<text>\"'"
	TipNoConnections  = "Add one in the TeamCity UI under project → Connections"
	TipNoParameters   = "Add one with 'teamcity param set <scope> <name> <value>'"
)
