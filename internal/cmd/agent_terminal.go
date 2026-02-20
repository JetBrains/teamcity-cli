package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/config"
	tcerrors "github.com/JetBrains/teamcity-cli/internal/errors"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/spf13/cobra"
)

// execTimeout is the default timeout for non-interactive command execution.
const execTimeout = 5 * time.Minute

func newAgentTerminalCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "term <agent>",
		Short: "Open interactive terminal to agent",
		Long:  `Open an interactive shell session to a TeamCity build agent.`,
		Args:  cobra.ExactArgs(1),
		Example: `  teamcity agent term 1
  teamcity agent term Agent-Linux-01`,
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, err := connectToAgent(cmd.Context(), args[0], true)
			if err != nil {
				return err
			}
			return conn.RunInteractive(cmd.Context())
		},
	}
}

func newAgentExecCmd() *cobra.Command {
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:   "exec <agent> <command>",
		Short: "Execute command on agent",
		Long:  `Execute a command on a TeamCity build agent and return the output.`,
		Args:  cobra.MinimumNArgs(2),
		Example: `  teamcity agent exec 1 "ls -la"
  teamcity agent exec Agent-Linux-01 "cat /etc/os-release"
  teamcity agent exec Agent-Linux-01 --timeout 10m -- long-running-script.sh`,
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, err := connectToAgent(cmd.Context(), args[0], false)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()

			return conn.Exec(ctx, strings.Join(args[1:], " "))
		},
	}

	cmd.Flags().DurationVar(&timeout, "timeout", execTimeout, "Command timeout")
	return cmd
}

func connectToAgent(ctx context.Context, nameOrID string, showProgress bool) (*api.TerminalConn, error) {
	serverURL := config.GetServerURL()
	token := config.GetToken()
	if serverURL == "" || token == "" {
		return nil, tcerrors.NotAuthenticated()
	}

	client, err := getClient()
	if err != nil {
		return nil, err
	}

	agent, err := resolveAgent(client, nameOrID)
	if err != nil {
		return nil, err
	}

	if !agent.Connected {
		return nil, tcerrors.WithSuggestion(
			fmt.Sprintf("Agent %s is not connected", agent.Name),
			"Wait for the agent to connect or check agent status with 'teamcity agent view'",
		)
	}
	if !agent.Authorized {
		return nil, tcerrors.WithSuggestion(
			fmt.Sprintf("Agent %s is not authorized", agent.Name),
			"Authorize the agent in TeamCity or use 'teamcity agent authorize'",
		)
	}
	if !agent.Enabled {
		return nil, tcerrors.WithSuggestion(
			fmt.Sprintf("Agent %s is disabled", agent.Name),
			"Enable the agent in TeamCity or use 'teamcity agent enable'",
		)
	}

	agentURL := fmt.Sprintf("%s/agentDetails.html?id=%d", serverURL, agent.ID)

	if showProgress {
		fmt.Printf("Connecting to %s...\n", output.Cyan(agent.Name))
	}

	termClient := api.NewTerminalClient(serverURL, config.GetCurrentUser(), token)
	session, err := termClient.OpenSession(agent.ID)
	if err != nil {
		return nil, err
	}

	cols, rows := output.TerminalSize()
	conn, err := termClient.Connect(session, cols, rows)
	if err != nil {
		return nil, err
	}

	fmt.Printf("%s %s\n", output.Green("✓"), agentURL)

	return conn, nil
}
