package skill

import (
	teamcitycli "github.com/JetBrains/teamcity-cli"
	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/analytics"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/spf13/cobra"
)

func newSkillHookEventCmd(f *cmdutil.Factory) *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "hook-event <skill> <offered|accepted|declined>",
		Short: "Report an onboarding hook offer or user response",
		Long: `Report an agent-observed hook offer, explicit consent, or refusal.

This command records telemetry only. It does not grant consent, load a guide,
or execute a hook. No response must not be reported as a refusal.
Telemetry respects the existing analytics opt-out and is best effort.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !teamcitycli.IsHookSkill(args[0]) {
				return api.Validation("unknown hook skill", "run 'teamcity skill list' and use a bundled onboarding hook")
			}
			events := map[string]string{
				"offered":  analytics.EventHookOffered,
				"accepted": analytics.EventHookAccepted,
				"declined": analytics.EventHookDeclined,
			}
			event, ok := events[args[1]]
			if !ok {
				return api.Validation("unknown hook event", "use offered, accepted, or declined")
			}
			f.Analytics.Track(analytics.GroupSkill, event, map[string]any{"skill": args[0]})
			if jsonOutput {
				return f.Printer.PrintJSON(map[string]string{"skill": args[0], "event": args[1]})
			}
			f.Printer.Success("Hook event processed: %s %s", args[0], args[1])
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output result as JSON")
	return cmd
}
