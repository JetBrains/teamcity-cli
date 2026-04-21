package output

import (
	"errors"
	"fmt"
	"strings"

	"github.com/JetBrains/teamcity-cli/api"
)

// RenderError returns a terminal-ready error with a Tip line appended when available.
func RenderError(err error) error {
	tip := tipFor(err)
	if tip == "" {
		return err
	}
	return fmt.Errorf("%s\n\n%s", err.Error(), FormatTip(tip))
}

// ClassifyError maps an error to a JSON error envelope (code + message + tip).
func ClassifyError(err error) (JSONErrorCode, string, string) {
	var ue api.UserError
	if errors.As(err, &ue) {
		return JSONErrorCode(ue.Category()), ue.Error(), tipFor(err)
	}
	if isInputError(err) {
		return ErrCodeValidation, err.Error(), ""
	}
	return ErrCodeInternal, err.Error(), ""
}

// tipFor returns the next-step suggestion: explicit Suggestion() first, then category default.
func tipFor(err error) string {
	if h, ok := err.(interface{ Suggestion() string }); ok {
		if s := h.Suggestion(); s != "" {
			return s
		}
	}
	var ue api.UserError
	if !errors.As(err, &ue) {
		return ""
	}
	if h, ok := ue.(interface{ Suggestion() string }); ok {
		if s := h.Suggestion(); s != "" {
			return s
		}
	}
	switch ue.Category() {
	case api.CatAuth:
		return "Run 'teamcity auth login' to re-authenticate"
	case api.CatPermission:
		// Only confident default when we identified the specific permission;
		// otherwise (e.g. TeamCity's ambiguous queue-remove 403) the server's
		// own message is the clearer guidance.
		if pe, ok := errors.AsType[*api.PermissionError](ue); ok && pe.Permission != "" {
			return "Ask your TeamCity administrator to grant this permission"
		}
		return ""
	case api.CatReadOnly:
		return "Unset the TEAMCITY_RO environment variable to allow write operations"
	case api.CatNotFound:
		if nf, ok := errors.AsType[*api.NotFoundError](ue); ok && hasListCommand(nf.Resource) {
			return fmt.Sprintf("Run 'teamcity %s list' to see available %ss", nf.Resource, nf.Resource)
		}
		return notFoundTip(ue.Error())
	case api.CatNetwork:
		if netErr, ok := errors.AsType[*api.NetworkError](ue); ok && api.IsSandboxBlocked(netErr) {
			return "Add the server domain to the sandbox allowlist, or exclude teamcity from sandboxing"
		}
		return "Check your network connection and verify the server URL"
	}
	return ""
}

// hasListCommand reports whether 'teamcity <resource> list' is a real command we can suggest
func hasListCommand(resource string) bool {
	switch resource {
	case "run", "job", "project", "agent":
		return true
	}
	return false
}

// notFoundTip suggests the matching 'teamcity X list' command for a 404 message.
func notFoundTip(message string) string {
	msg := strings.ToLower(message)
	switch {
	case strings.Contains(msg, "agent pool"), strings.Contains(msg, "pool"):
		return "Use 'teamcity pool list' to see available pools"
	case strings.Contains(msg, "agent"):
		return "Use 'teamcity agent list' to see available agents"
	case strings.Contains(msg, "project"):
		return "Use 'teamcity project list' to see available projects"
	case strings.Contains(msg, "build type"), strings.Contains(msg, "job"):
		return "Use 'teamcity job list' to see available jobs"
	default:
		return "Use 'teamcity job list' or 'teamcity run list' to see available resources"
	}
}

// isInputError reports whether a raw error string looks like cobra/CLI input validation.
func isInputError(err error) bool {
	msg := err.Error()
	for _, prefix := range []string{
		"unknown command",
		"unknown flag",
		"required flag",
		"invalid argument",
		"invalid status",
		"accepts ",
		"if any flags in the group",
		"--limit must be",
		"unknown fields:",
		"unknown key",
	} {
		if strings.HasPrefix(msg, prefix) {
			return true
		}
	}
	for _, substr := range []string{
		"flag needs an argument",
		"mutually exclusive",
		"required (or use",
		"not found in configuration",
	} {
		if strings.Contains(msg, substr) {
			return true
		}
	}
	return false
}
