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
	TipNoConnections  = "Add one in the TeamCity UI under project → Connections"
	TipCancelAnytime  = "Press Ctrl+C at any time to cancel"
)

// TipNoArtifactsFor returns the tip for a run that has no artifacts, pointing at
// the specific run's log command so the user can copy-paste it.
func TipNoArtifactsFor(runID string) string {
	return fmt.Sprintf("Use 'teamcity run log %s' to view build output", runID)
}

// TipNoLogFor returns the tip for a run with no log yet, pointing at the
// specific run's view command.
func TipNoLogFor(runID string) string {
	return fmt.Sprintf("The run may still be queued; 'teamcity run view %s' shows its state", runID)
}

// TipNoCommentFor returns the tip for a run with no comment, pre-filling the
// specific run ID in the suggested command.
func TipNoCommentFor(runID string) string {
	return fmt.Sprintf("Add one with 'teamcity run comment %s --set \"<text>\"'", runID)
}

// TipNoParametersFor returns the tip for an empty parameter list, pre-filling
// the scope (project or job ID).
func TipNoParametersFor(scope string) string {
	return fmt.Sprintf("Add one with 'teamcity param set %s <name> <value>'", scope)
}

// TipEnableReadOnly returns a tip suggesting how to switch the CLI into read-only mode.
func TipEnableReadOnly() string {
	return fmt.Sprintf("To enable read-only mode, set %s or run %s",
		Cyan("TEAMCITY_RO=1"),
		Cyan("teamcity config set ro true"))
}

// TipSwitchDefaultServer returns a tip pointing at the command that switches the default server.
func TipSwitchDefaultServer() string {
	return "To switch the default server, run " + Cyan("teamcity config set default_server <url>")
}

// TipResumeLogFor returns the resume-hint for an interrupted `teamcity run log` follow session.
func TipResumeLogFor(runID string) string {
	return "Resume: teamcity run log -f " + runID
}

// TipResumeWatchFor returns the resume-hint for an interrupted `teamcity run watch` session.
func TipResumeWatchFor(runID string) string {
	return "Resume watching: teamcity run watch " + runID
}

// TipRegisterGitHubApp points the user at GitHub's App registration page (manual mode).
func TipRegisterGitHubApp(owner string) string {
	if owner == "" {
		return "Register a GitHub App at " + Cyan("https://github.com/settings/apps/new")
	}
	return "Register a GitHub App at " + Cyan(fmt.Sprintf("https://github.com/organizations/%s/settings/apps/new", owner))
}
