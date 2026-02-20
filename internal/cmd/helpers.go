package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/output"
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

// validateLimit returns an error if limit is not positive.
func validateLimit(limit int) error {
	if limit <= 0 {
		return fmt.Errorf("--limit must be a positive number, got %d", limit)
	}
	return nil
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

// warnInsecureHTTP prints a warning to stderr when connecting over plain HTTP.
// Suppressed by setting TC_INSECURE_SKIP_WARN=1.
func warnInsecureHTTP(serverURL, credentialType string) {
	if !strings.HasPrefix(serverURL, "http://") || os.Getenv("TC_INSECURE_SKIP_WARN") != "" {
		return
	}
	output.Warn("Using insecure HTTP connection. Your %s will be transmitted in plaintext.", credentialType)
	output.Warn("Consider using HTTPS for secure communication.")
}
