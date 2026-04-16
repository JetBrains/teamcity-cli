package agent

import (
	"context"
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/JetBrains/teamcity-cli/internal/analytics"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/spf13/cobra"
)

func newAgentMoveCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "move <agent> <pool-id>",
		Short: "Move an agent to a different pool",
		Long:  `Move an agent to a different agent pool.`,
		Args:  cobra.ExactArgs(2),
		Example: `  teamcity agent move 1 0
  teamcity agent move Agent-Linux-01 2`,
		RunE: func(cmd *cobra.Command, args []string) error {
			poolID, err := cmdutil.ParseID(args[1], "pool")
			if err != nil {
				return err
			}
			return runAgentMove(f, args[0], poolID)
		},
	}

	return cmd
}

func runAgentMove(f *cmdutil.Factory, nameOrID string, poolID int) error {
	client, err := f.Client()
	if err != nil {
		return err
	}

	agentID, agentName, err := cmdutil.ResolveAgentID(client, nameOrID)
	if err != nil {
		return err
	}

	if err := client.SetAgentPool(agentID, poolID); err != nil {
		return fmt.Errorf("failed to move agent: %w", err)
	}

	f.Analytics.Track(analytics.GroupAgent, analytics.EventStateChanged, map[string]any{"action": analytics.AgentActionMove})
	f.Printer.Success("Moved agent %s to pool %d", agentName, poolID)
	return nil
}

type agentRebootOptions struct {
	graceful bool
	yes      bool
}

func newAgentRebootCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &agentRebootOptions{}

	cmd := &cobra.Command{
		Use:   "reboot <agent>",
		Short: "Reboot an agent",
		Long: `Request a reboot of an agent.

The agent can be specified by ID or name. By default, the agent reboots immediately.
Use --graceful to wait for current work to finish before rebooting.

Note: Local agents (running on the same machine as the server) cannot be rebooted.`,
		Args: cobra.ExactArgs(1),
		Example: `  teamcity agent reboot 1
  teamcity agent reboot Agent-Linux-01
  teamcity agent reboot Agent-Linux-01 --graceful
  teamcity agent reboot Agent-Linux-01 --yes`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAgentReboot(f, cmd.Context(), args[0], opts)
		},
	}

	cmd.Flags().BoolVar(&opts.graceful, "graceful", false, "Wait for current work to finish before rebooting")
	cmd.Flags().BoolVar(&opts.graceful, "after-build", false, "Deprecated: use --graceful")
	_ = cmd.Flags().MarkDeprecated("after-build", "use --graceful instead")
	cmd.Flags().BoolVarP(&opts.yes, "yes", "y", false, "Skip confirmation prompt")
	cmd.Flags().BoolVarP(&opts.yes, "force", "f", false, "Deprecated: use --yes")
	_ = cmd.Flags().MarkDeprecated("force", "use --yes instead")

	return cmd
}

func runAgentReboot(f *cmdutil.Factory, ctx context.Context, nameOrID string, opts *agentRebootOptions) error {
	client, err := f.Client()
	if err != nil {
		return err
	}

	agentID, agentName, err := cmdutil.ResolveAgentID(client, nameOrID)
	if err != nil {
		return err
	}

	needsConfirmation := !opts.yes && f.IsInteractive()
	if needsConfirmation {
		var confirm bool
		prompt := &survey.Confirm{
			Message: fmt.Sprintf("Reboot agent %s?", agentName),
			Default: false,
		}
		if err := survey.AskOne(prompt, &confirm); err != nil {
			return err
		}
		if !confirm {
			f.Printer.Info("Canceled")
			return nil
		}
	}

	if err := client.RebootAgent(ctx, agentID, opts.graceful); err != nil {
		return fmt.Errorf("failed to reboot agent: %w", err)
	}

	f.Analytics.Track(analytics.GroupAgent, analytics.EventStateChanged, map[string]any{"action": analytics.AgentActionReboot})

	if opts.graceful {
		f.Printer.Success("Reboot scheduled for %s", agentName)
		_, _ = fmt.Fprintln(f.Printer.Out, "  The agent will reboot after current work finishes.")
	} else {
		f.Printer.Success("Reboot initiated for %s", agentName)
	}
	return nil
}
