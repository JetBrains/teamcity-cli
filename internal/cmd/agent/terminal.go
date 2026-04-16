package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/analytics"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/config"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/JetBrains/teamcity-cli/internal/terminal"
	"github.com/spf13/cobra"
)

const execTimeout = 5 * time.Minute

func newAgentTerminalCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "term <agent>",
		Short: "Open interactive terminal to agent",
		Long: `Open an interactive shell session to a TeamCity build agent.

Requires the agent to be connected, authorized, and enabled. The
session runs over a WebSocket and exits when the remote shell exits
or the connection drops.`,
		Args: cobra.ExactArgs(1),
		Example: `  teamcity agent term 1
  teamcity agent term Agent-Linux-01`,
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, err := connectToAgent(f, cmd.Context(), args[0], true)
			if err != nil {
				return err
			}
			start := time.Now()
			termErr := conn.RunInteractive(cmd.Context())
			f.Analytics.Track(analytics.GroupAgent, analytics.EventTerminalClosed, map[string]any{
				"duration_seconds": int(time.Since(start).Seconds()),
				"exit_reason":      terminalExitReason(termErr, cmd.Context().Err()),
			})
			return termErr
		},
	}
}

func terminalExitReason(termErr, ctxErr error) string {
	switch {
	case termErr == nil:
		return analytics.AgentExitUser
	case errors.Is(ctxErr, context.DeadlineExceeded):
		return analytics.AgentExitTimeout
	case errors.Is(ctxErr, context.Canceled):
		return analytics.AgentExitUser
	}
	return analytics.AgentExitError
}

func newAgentExecCmd(f *cmdutil.Factory) *cobra.Command {
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:   "exec <agent> <command>",
		Short: "Execute command on agent",
		Long: `Run a one-shot command on an agent and return its output.

Use 'teamcity agent term' for an interactive shell. Commands longer
than the default timeout (5m) need --timeout; use -- to separate
agent-side commands from teamcity flags.`,
		Args: cobra.MinimumNArgs(2),
		Example: `  teamcity agent exec 1 "ls -la"
  teamcity agent exec Agent-Linux-01 "cat /etc/os-release"
  teamcity agent exec Agent-Linux-01 --timeout 10m -- long-running-script.sh`,
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, err := connectToAgent(f, cmd.Context(), args[0], false)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()

			start := time.Now()
			execErr := conn.Exec(ctx, strings.Join(args[1:], " "))
			exitCode := 0
			if execErr != nil {
				exitCode = 1
				if ee, ok := errors.AsType[*cmdutil.ExitError](execErr); ok {
					exitCode = ee.Code
				}
			}
			f.Analytics.Track(analytics.GroupAgent, analytics.EventExecFinished, map[string]any{
				"duration_seconds": int(time.Since(start).Seconds()),
				"exit_code":        exitCode,
				"had_timeout":      errors.Is(ctx.Err(), context.DeadlineExceeded),
			})
			return execErr
		},
	}

	cmd.Flags().DurationVar(&timeout, "timeout", execTimeout, "Command timeout")
	return cmd
}

func connectToAgent(f *cmdutil.Factory, ctx context.Context, nameOrID string, showProgress bool) (*terminal.Conn, error) {
	serverURL := config.GetServerURL()
	token, _, keyringErr := config.GetTokenWithSource()
	if serverURL == "" || token == "" {
		return nil, cmdutil.NotAuthenticatedError(serverURL, keyringErr)
	}

	client, err := f.Client()
	if err != nil {
		return nil, err
	}

	agent, err := cmdutil.ResolveAgent(client, nameOrID)
	if err != nil {
		return nil, err
	}

	if !agent.Connected {
		return nil, api.Validation(
			fmt.Sprintf("Agent %s is not connected", agent.Name),
			"Wait for the agent to connect or check agent status with 'teamcity agent view'",
		)
	}
	if !agent.Authorized {
		return nil, api.Validation(
			fmt.Sprintf("Agent %s is not authorized", agent.Name),
			"Authorize the agent in TeamCity or use 'teamcity agent authorize'",
		)
	}
	if !agent.Enabled {
		return nil, api.Validation(
			fmt.Sprintf("Agent %s is disabled", agent.Name),
			"Enable the agent in TeamCity or use 'teamcity agent enable'",
		)
	}

	agentURL := fmt.Sprintf("%s/agentDetails.html?id=%d", serverURL, agent.ID)

	if showProgress {
		_, _ = fmt.Fprintf(f.Printer.Out, "Connecting to %s...\n", output.Cyan(agent.Name))
	}

	username := config.GetCurrentUser()
	if username == "" {
		user, err := client.GetCurrentUser()
		if err != nil {
			return nil, fmt.Errorf("resolve username for terminal auth: %w", err)
		}
		username = user.Username
	}

	termClient := terminal.NewClient(serverURL, username, token, f.Printer.Debug)
	session, err := termClient.OpenSession(agent.ID)
	if err != nil {
		return nil, err
	}

	cols, rows := output.TerminalSize()
	conn, err := termClient.Connect(session, cols, rows)
	if err != nil {
		return nil, err
	}

	_, _ = fmt.Fprintf(f.Printer.Out, "%s %s\n", output.Green("✓"), agentURL)

	return conn, nil
}
