package errors

import (
	"fmt"

	"github.com/fatih/color"
)

// UserError represents an error with a user-friendly message and optional suggestion
type UserError struct {
	Message    string
	Suggestion string
}

func (e *UserError) Error() string {
	if e.Suggestion != "" {
		return fmt.Sprintf("%s\n\n%s %s", e.Message, color.YellowString("Hint:"), e.Suggestion)
	}
	return e.Message
}

// New creates a new UserError with just a message
func New(message string) *UserError {
	return &UserError{Message: message}
}

// WithSuggestion creates a new UserError with a message and suggestion
func WithSuggestion(message, suggestion string) *UserError {
	return &UserError{
		Message:    message,
		Suggestion: suggestion,
	}
}

// NotAuthenticated returns an error for unauthenticated users
func NotAuthenticated() *UserError {
	return &UserError{
		Message:    "Not authenticated",
		Suggestion: "Run 'tc auth login' to authenticate with TeamCity",
	}
}

// NotFound returns an error for resources that don't exist
func NotFound(resource, id string) *UserError {
	return &UserError{
		Message:    fmt.Sprintf("%s '%s' not found", resource, id),
		Suggestion: fmt.Sprintf("Run 'tc %s list' to see available %ss", resource, resource),
	}
}

// AuthenticationFailed returns an error for failed authentication
func AuthenticationFailed() *UserError {
	return &UserError{
		Message:    "Authentication failed: invalid or expired token",
		Suggestion: "Run 'tc auth login' to re-authenticate",
	}
}

// PermissionDenied returns an error for permission issues
func PermissionDenied(action string) *UserError {
	return &UserError{
		Message:    fmt.Sprintf("Permission denied: cannot %s", action),
		Suggestion: "Check your TeamCity permissions or contact your administrator",
	}
}

// NetworkError returns an error for network issues
func NetworkError(serverURL string) *UserError {
	return &UserError{
		Message:    fmt.Sprintf("Cannot connect to TeamCity server at %s", serverURL),
		Suggestion: "Check your network connection and verify the server URL",
	}
}

// RequiredFlag returns an error for missing required flags in non-interactive mode
func RequiredFlag(flag string) *UserError {
	return &UserError{
		Message:    fmt.Sprintf("--%s is required in non-interactive mode", flag),
		Suggestion: "Provide the flag value or run without --no-input for interactive prompts",
	}
}

// MutuallyExclusive returns an error when mutually exclusive options are both provided
func MutuallyExclusive(arg, flag string) *UserError {
	return &UserError{
		Message:    fmt.Sprintf("cannot specify both %s argument and --%s flag", arg, flag),
		Suggestion: fmt.Sprintf("Use either '%s' or '--%s', not both", arg, flag),
	}
}
