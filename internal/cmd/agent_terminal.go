package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/JetBrains/teamcity-cli/internal/api"
	"github.com/JetBrains/teamcity-cli/internal/config"
	tcerrors "github.com/JetBrains/teamcity-cli/internal/errors"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/spf13/cobra"
)

// execTimeout is the default timeout for non-interactive command execution.
const execTimeout = 5 * time.Minute

func newAgentTerminalCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "term <agent-id>",
		Short: "Open interactive terminal to agent",
		Long:  `Open an interactive shell session to a TeamCity build agent.`,
		Args:  cobra.ExactArgs(1),
		Example: `  tc agent term 1
  tc agent term 42`,
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID(args[0], "agent")
			if err != nil {
				return err
			}
			conn, err := connectToAgent(cmd.Context(), id, true)
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
		Use:   "exec <agent-id> <command>",
		Short: "Execute command on agent",
		Long:  `Execute a command on a TeamCity build agent and return the output.`,
		Args:  cobra.MinimumNArgs(2),
		Example: `  tc agent exec 1 "ls -la"
  tc agent exec 42 "cat /etc/os-release"
  tc agent exec 1 --timeout 10m -- long-running-script.sh`,
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID(args[0], "agent")
			if err != nil {
				return err
			}
			conn, err := connectToAgent(cmd.Context(), id, false)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()

			out, err := conn.Exec(ctx, strings.Join(args[1:], " "))
			if err != nil {
				return err
			}
			if out != "" {
				fmt.Println(out)
			}
			return nil
		},
	}

	cmd.Flags().DurationVar(&timeout, "timeout", execTimeout, "Command timeout")
	return cmd
}

func connectToAgent(ctx context.Context, agentID int, showProgress bool) (*api.TerminalConn, error) {
	serverURL := config.GetServerURL()
	token := config.GetToken()
	if serverURL == "" || token == "" {
		return nil, tcerrors.NotAuthenticated()
	}

	client, err := getClient()
	if err != nil {
		return nil, err
	}

	agent, err := client.GetAgent(agentID)
	if err != nil {
		return nil, err
	}

	if !agent.Connected {
		return nil, tcerrors.WithSuggestion(
			fmt.Sprintf("Agent %d (%s) is not connected", agentID, agent.Name),
			"Wait for the agent to connect or check agent status with 'tc agent view'",
		)
	}
	if !agent.Authorized {
		return nil, tcerrors.WithSuggestion(
			fmt.Sprintf("Agent %d (%s) is not authorized", agentID, agent.Name),
			"Authorize the agent in TeamCity or use 'tc agent authorize'",
		)
	}
	if !agent.Enabled {
		return nil, tcerrors.WithSuggestion(
			fmt.Sprintf("Agent %d (%s) is disabled", agentID, agent.Name),
			"Enable the agent in TeamCity or use 'tc agent enable'",
		)
	}

	agentURL := fmt.Sprintf("%s/agentDetails.html?id=%d", serverURL, agentID)

	if showProgress {
		fmt.Printf("Connecting to %s...\n", output.Cyan(agent.Name))
	}

	termClient := api.NewTerminalClient(serverURL, config.GetCurrentUser(), token)
	session, err := termClient.OpenSession(agentID)
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
