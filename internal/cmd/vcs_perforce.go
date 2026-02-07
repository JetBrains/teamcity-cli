package cmd

import (
	"fmt"
	"os/exec"
	"strings"

	tcerrors "github.com/JetBrains/teamcity-cli/internal/errors"
)

// PerforceProvider implements VCSProvider for Perforce (Helix Core) workspaces.
type PerforceProvider struct{}

func (p *PerforceProvider) Name() string {
	return "perforce"
}

func (p *PerforceProvider) IsAvailable() bool {
	return isPerforceWorkspace()
}

func (p *PerforceProvider) GetCurrentBranch() (string, error) {
	return getPerforceStream()
}

func (p *PerforceProvider) GetHeadRevision() (string, error) {
	return getPerforceChangelist()
}

func (p *PerforceProvider) GetLocalDiff() ([]byte, error) {
	return getPerforceDiff()
}

func (p *PerforceProvider) BranchExistsOnRemote(branch string) bool {
	// Perforce streams/depot paths always exist on the server if the workspace is valid.
	// Check by querying the stream spec.
	if strings.HasPrefix(branch, "//") {
		cmd := exec.Command("p4", "stream", "-o", branch)
		err := cmd.Run()
		return err == nil
	}
	return true
}

func (p *PerforceProvider) PushBranch(_ string) error {
	// Perforce doesn't have a push concept; changes are submitted directly.
	return nil
}

func (p *PerforceProvider) FormatRevision(rev string) string {
	// Perforce changelist numbers are typically short; return as-is.
	return rev
}

func (p *PerforceProvider) FormatVCSBranch(branch string) string {
	// Perforce depot paths are used as-is in TeamCity VCS branch references.
	return branch
}

func (p *PerforceProvider) DiffHint(firstRev, lastRev string) string {
	return fmt.Sprintf("p4 changes -l @%s,@%s", firstRev, lastRev)
}

// isPerforceWorkspace checks if the current directory is inside a Perforce workspace.
func isPerforceWorkspace() bool {
	cmd := exec.Command("p4", "info")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	// p4 info returns "Client unknown" when not in a valid workspace
	outStr := string(out)
	if strings.Contains(outStr, "Client unknown") {
		return false
	}
	// Verify there's a Client root set
	return strings.Contains(outStr, "Client root:")
}

// getPerforceStream returns the current Perforce stream depot path.
// Falls back to the depot path if streams are not used.
func getPerforceStream() (string, error) {
	// Try to get the stream from the current client spec
	clientName, err := getPerforceClientName()
	if err != nil {
		return "", err
	}

	cmd := exec.Command("p4", "-ztag", "client", "-o", clientName)
	out, err := cmd.Output()
	if err != nil {
		return "", tcerrors.WithSuggestion(
			"failed to get Perforce client spec",
			"Ensure p4 is configured and you are in a valid workspace",
		)
	}

	// Parse the stream field from ztag output
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "... Stream ") {
			stream := strings.TrimPrefix(line, "... Stream ")
			return strings.TrimSpace(stream), nil
		}
	}

	// If no stream, try to extract the depot path from the client mapping
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "... View0 ") {
			view := strings.TrimPrefix(line, "... View0 ")
			// View format: "//depot/path/... //client/..."
			parts := strings.Fields(view)
			if len(parts) >= 1 {
				depotPath := strings.TrimSuffix(parts[0], "/...")
				return depotPath, nil
			}
		}
	}

	return "", tcerrors.WithSuggestion(
		"could not determine Perforce stream or depot path",
		"Ensure your workspace is mapped to a stream or depot path",
	)
}

// getPerforceClientName returns the name of the current Perforce client/workspace.
func getPerforceClientName() (string, error) {
	cmd := exec.Command("p4", "-ztag", "info")
	out, err := cmd.Output()
	if err != nil {
		return "", tcerrors.WithSuggestion(
			"failed to get Perforce info",
			"Ensure p4 is installed and P4PORT/P4USER/P4CLIENT are configured",
		)
	}

	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "... clientName ") {
			return strings.TrimPrefix(line, "... clientName "), nil
		}
	}

	return "", tcerrors.WithSuggestion(
		"no Perforce client found",
		"Set P4CLIENT or run 'p4 set P4CLIENT=<workspace-name>'",
	)
}

// getPerforceChangelist returns the latest submitted changelist number
// that affects the current workspace.
func getPerforceChangelist() (string, error) {
	clientName, err := getPerforceClientName()
	if err != nil {
		return "", err
	}

	// Get the latest changelist synced to this workspace
	cmd := exec.Command("p4", "changes", "-m1", "-s", "submitted", "@"+clientName)
	out, err := cmd.Output()
	if err != nil {
		return "", tcerrors.WithSuggestion(
			"failed to get Perforce changelist",
			"Ensure you have submitted changes in your workspace",
		)
	}

	// Output format: "Change 12345 on 2024/01/01 by user@client 'description'"
	outStr := strings.TrimSpace(string(out))
	if outStr == "" {
		return "", tcerrors.WithSuggestion(
			"no submitted changelists found",
			"Submit at least one changelist, or specify --branch explicitly",
		)
	}

	fields := strings.Fields(outStr)
	if len(fields) >= 2 && fields[0] == "Change" {
		return fields[1], nil
	}

	return "", tcerrors.WithSuggestion(
		"unexpected p4 changes output",
		"Ensure p4 is working correctly",
	)
}

// getPerforceDiff generates a unified diff from pending changes in the default changelist.
func getPerforceDiff() ([]byte, error) {
	// First check if there are any open files
	openCmd := exec.Command("p4", "opened")
	openOut, err := openCmd.Output()
	if err != nil {
		return nil, tcerrors.WithSuggestion(
			"failed to list opened files",
			"Ensure you are in a Perforce workspace with pending changes",
		)
	}

	if strings.TrimSpace(string(openOut)) == "" {
		return nil, nil // No pending changes
	}

	// Generate unified diff of all opened files
	cmd := exec.Command("p4", "diff", "-du")
	out, err := cmd.Output()
	if err != nil {
		// p4 diff returns exit code 1 if there are differences (which is expected)
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return out, nil
		}
		return nil, tcerrors.WithSuggestion(
			"failed to generate Perforce diff",
			"Ensure you have pending changes in your workspace",
		)
	}

	return out, nil
}

// getPerforcePort returns the Perforce server address (P4PORT).
func getPerforcePort() (string, error) {
	cmd := exec.Command("p4", "-ztag", "info")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}

	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "... serverAddress ") {
			return strings.TrimPrefix(line, "... serverAddress "), nil
		}
	}

	return "", fmt.Errorf("P4PORT not found")
}
