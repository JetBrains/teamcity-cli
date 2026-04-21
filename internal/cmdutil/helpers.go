package cmdutil

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
)

// ViewOptions is shared by view commands that support JSON and web output.
type ViewOptions struct {
	JSON bool
	Web  bool
}

// AddViewFlags adds --json and --web flags to a command.
func AddViewFlags(cmd *cobra.Command, opts *ViewOptions) {
	cmd.Flags().BoolVar(&opts.JSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVarP(&opts.Web, "web", "w", false, "Open in browser")
}

// OpenURLOrWarn opens url in the browser, warning on failure. Never returns an error — safe to call after a mutation.
func OpenURLOrWarn(p *output.Printer, url string) {
	if url == "" {
		return
	}
	if err := browser.OpenURL(url); err != nil {
		p.Warn("could not open browser: %v", err)
	}
}

// ValidateLimit returns an error if limit is not positive.
func ValidateLimit(limit int) error {
	if limit <= 0 {
		return fmt.Errorf("--limit must be a positive number, got %d", limit)
	}
	return nil
}

// ParseID converts a string argument to an integer ID.
func ParseID(s string, entity string) (int, error) {
	id, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("invalid %s ID: %s (must be a number)", entity, s)
	}
	return id, nil
}

// ResolveAgent resolves an agent name or ID to an Agent object.
func ResolveAgent(client api.ClientInterface, nameOrID string) (*api.Agent, error) {
	if id, err := strconv.Atoi(nameOrID); err == nil {
		return client.GetAgent(id)
	}
	return client.GetAgentByName(nameOrID)
}

// ResolveAgentID resolves an agent name or ID to a numeric agent ID and name.
func ResolveAgentID(client api.ClientInterface, nameOrID string) (int, string, error) {
	agent, err := ResolveAgent(client, nameOrID)
	if err != nil {
		return 0, "", err
	}
	return agent.ID, agent.Name, nil
}

// WarnInsecureHTTP prints a warning to stderr when connecting over plain HTTP.
func (f *Factory) WarnInsecureHTTP(serverURL, credentialType string) {
	if !strings.HasPrefix(serverURL, "http://") || os.Getenv("TC_INSECURE_SKIP_WARN") != "" {
		return
	}
	f.Printer.Warn("Using insecure HTTP connection. Your %s will be transmitted in plaintext.", credentialType)
	f.Printer.Warn("Consider using HTTPS for secure communication.")
}

// FormatAgentStatus returns a formatted status string for an agent.
func FormatAgentStatus(a api.Agent) string {
	if !a.Authorized {
		return output.Yellow("Unauthorized")
	}
	if !a.Enabled {
		return output.Faint("Disabled")
	}
	if !a.Connected {
		return output.Red("Disconnected")
	}
	return output.Green("Connected")
}
