package cmd

import (
	"fmt"
	"strconv"

	"github.com/JetBrains/teamcity-cli/internal/api"
	"github.com/spf13/cobra"
)

// viewOptions is shared by view commands that support JSON and web output.
type viewOptions struct {
	json bool
	web  bool
}

// addViewFlags adds --json and --web flags to a command.
func addViewFlags(cmd *cobra.Command, opts *viewOptions) {
	cmd.Flags().BoolVar(&opts.json, "json", false, "Output as JSON")
	cmd.Flags().BoolVarP(&opts.web, "web", "w", false, "Open in browser")
}

// parseID converts a string argument to an integer ID.
// Used for parsing agent and pool IDs from command line arguments.
func parseID(s string, entity string) (int, error) {
	id, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("invalid %s ID: %s (must be a number)", entity, s)
	}
	return id, nil
}

// resolveAgent resolves an agent name or ID to an Agent object.
// If the input is a number, it's used directly as the ID.
// Otherwise, it looks up the agent by name.
// Note: If an agent is named with a numeric string (e.g., "123"),
// it will be interpreted as an ID, not a name.
func resolveAgent(client api.ClientInterface, nameOrID string) (*api.Agent, error) {
	if id, err := strconv.Atoi(nameOrID); err == nil {
		return client.GetAgent(id)
	}
	return client.GetAgentByName(nameOrID)
}

// resolveAgentID resolves an agent name or ID to a numeric agent ID and name.
// Use resolveAgent() if you need the full agent object to avoid double API calls.
func resolveAgentID(client api.ClientInterface, nameOrID string) (int, string, error) {
	agent, err := resolveAgent(client, nameOrID)
	if err != nil {
		return 0, "", err
	}
	return agent.ID, agent.Name, nil
}
