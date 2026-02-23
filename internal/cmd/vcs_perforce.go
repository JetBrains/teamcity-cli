package cmd

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	tcerrors "github.com/JetBrains/teamcity-cli/internal/errors"
)

const p4Timeout = 10 * time.Second

func p4Output(args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), p4Timeout)
	defer cancel()
	return exec.CommandContext(ctx, "p4", args...).Output()
}

func p4ZtagField(output []byte, field string) string {
	prefix := "... " + field + " "
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			return strings.TrimPrefix(line, prefix)
		}
	}
	return ""
}

type PerforceProvider struct{}

func (p *PerforceProvider) Name() string { return "perforce" }

func (p *PerforceProvider) IsAvailable() bool {
	out, err := p4Output("info")
	if err != nil {
		return false
	}
	s := string(out)
	return !strings.Contains(s, "Client unknown") && strings.Contains(s, "Client root:")
}

func (p *PerforceProvider) GetCurrentBranch() (string, error) {
	clientName, err := getPerforceClientName()
	if err != nil {
		return "", err
	}

	out, err := p4Output("-ztag", "client", "-o", clientName)
	if err != nil {
		return "", tcerrors.WithSuggestion(
			"failed to get Perforce client spec",
			"Ensure p4 is configured and you are in a valid workspace",
		)
	}

	if stream := p4ZtagField(out, "Stream"); stream != "" {
		return stream, nil
	}
	if view := p4ZtagField(out, "View0"); view != "" {
		if parts := strings.Fields(view); len(parts) >= 1 {
			return strings.TrimSuffix(parts[0], "/..."), nil
		}
	}

	return "", tcerrors.WithSuggestion(
		"could not determine Perforce stream or depot path",
		"Ensure your workspace is mapped to a stream or depot path",
	)
}

func (p *PerforceProvider) GetHeadRevision() (string, error) {
	clientName, err := getPerforceClientName()
	if err != nil {
		return "", err
	}

	out, err := p4Output("changes", "-m1", "-s", "submitted", "@"+clientName)
	if err != nil {
		return "", tcerrors.WithSuggestion(
			"failed to get Perforce changelist",
			"Ensure you have submitted changes in your workspace",
		)
	}

	fields := strings.Fields(strings.TrimSpace(string(out)))
	if len(fields) >= 2 && fields[0] == "Change" {
		return fields[1], nil
	}

	return "", tcerrors.WithSuggestion(
		"no submitted changelists found",
		"Submit at least one changelist, or specify --branch explicitly",
	)
}

func (p *PerforceProvider) GetLocalDiff() ([]byte, error) {
	openOut, err := p4Output("opened")
	if err != nil {
		return nil, tcerrors.WithSuggestion(
			"failed to list opened files",
			"Ensure you are in a Perforce workspace with pending changes",
		)
	}
	if strings.TrimSpace(string(openOut)) == "" {
		return nil, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), p4Timeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "p4", "diff", "-du").Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return out, nil // p4 diff returns exit 1 when differences exist
		}
		return nil, tcerrors.WithSuggestion(
			"failed to generate Perforce diff",
			"Ensure you have pending changes in your workspace",
		)
	}
	return out, nil
}

func (p *PerforceProvider) BranchExistsOnRemote(_ string) bool { return true }
func (p *PerforceProvider) PushBranch(_ string) error          { return nil }
func (p *PerforceProvider) FormatRevision(rev string) string { return rev }

func (p *PerforceProvider) DiffHint(firstRev, lastRev string) string {
	return fmt.Sprintf("p4 changes -l @%s,@%s", firstRev, lastRev)
}

func getPerforceClientName() (string, error) {
	out, err := p4Output("-ztag", "info")
	if err != nil {
		return "", tcerrors.WithSuggestion(
			"failed to get Perforce info",
			"Ensure p4 is installed and P4PORT/P4USER/P4CLIENT are configured",
		)
	}
	if name := p4ZtagField(out, "clientName"); name != "" {
		return name, nil
	}
	return "", tcerrors.WithSuggestion(
		"no Perforce client found",
		"Set P4CLIENT or run 'p4 set P4CLIENT=<workspace-name>'",
	)
}
