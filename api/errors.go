package api

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// Category classifies user-facing errors for UI rendering and JSON output.
type Category string

const (
	CatAuth       Category = "auth_expired"
	CatPermission Category = "permission_denied"
	CatNotFound   Category = "not_found"
	CatNetwork    Category = "network_error"
	CatReadOnly   Category = "read_only"
	CatValidation Category = "validation_error"
	CatInternal   Category = "internal_error"
)

// UserError is the contract consumed by the CLI renderer.
type UserError interface {
	error
	Category() Category
}

// Wire holds the fields parsed from a TeamCity error response body.
type Wire struct {
	Message, Additional, StatusText string
}

// HTTPError covers HTTP-derived errors without extra structured fields (401, generic 4xx/5xx).
type HTTPError struct {
	Status  int
	Wire    Wire
	rawBody []byte // surfaced only when Wire has no message — preserves diagnostic detail from proxy HTML / oversized text
	cat     Category
}

func (e *HTTPError) Error() string {
	if e.Wire.Message != "" {
		return e.Wire.Message
	}
	snippet := bodySnippet(e.rawBody)
	switch e.cat {
	case CatAuth:
		return joinSnippet("authentication failed: invalid or expired credentials", snippet)
	case CatPermission:
		return joinSnippet("permission denied", snippet)
	case CatNotFound:
		return joinSnippet("resource not found", snippet)
	}
	return joinSnippet(fmt.Sprintf("server returned %d", e.Status), snippet)
}

func (e *HTTPError) Category() Category { return e.cat }

// PermissionError is returned for HTTP 403 responses.
type PermissionError struct {
	HTTPError
	Permission string     // e.g. "Comment build"
	Project    string     // TeamCity internal project id
	AuthSource AuthSource // how the client authenticated; drives the tip wording
}

func (e *PermissionError) Error() string {
	switch {
	case e.Permission != "" && e.Project != "":
		return fmt.Sprintf("missing %q permission in project %s", e.Permission, e.Project)
	case e.Permission != "":
		return fmt.Sprintf("missing %q permission", e.Permission)
	}
	if e.Wire.Message != "" {
		return e.Wire.Message
	}
	return joinSnippet("permission denied", bodySnippet(e.rawBody))
}

// NotFoundError is returned for HTTP 404 responses.
type NotFoundError struct {
	HTTPError
	Resource string // "job", "run", "project", "user", "agent"
	ID       string
}

func (e *NotFoundError) Error() string {
	if e.Resource != "" && e.ID != "" {
		return fmt.Sprintf("%s %q not found", e.Resource, e.ID)
	}
	if e.Wire.Message != "" {
		return e.Wire.Message
	}
	return joinSnippet("resource not found", bodySnippet(e.rawBody))
}

// NetworkError wraps transport-level failures (DNS, connect, TLS, timeout).
type NetworkError struct {
	URL   string
	Cause error
}

func (e *NetworkError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("cannot connect to %s: %v", e.URL, e.Cause)
	}
	return "cannot connect to " + e.URL
}

func (e *NetworkError) Unwrap() error    { return e.Cause }
func (*NetworkError) Category() Category { return CatNetwork }

// ValidationError is a CLI-constructed user-input error with an optional imperative Tip.
type ValidationError struct {
	Msg string
	Tip string
}

func (e *ValidationError) Error() string      { return e.Msg }
func (*ValidationError) Category() Category   { return CatValidation }
func (e *ValidationError) Suggestion() string { return e.Tip }

// Validation wraps a user-input error with an imperative tip.
func Validation(msg, tip string) *ValidationError {
	return &ValidationError{Msg: msg, Tip: tip}
}

// RequiredFlag is a validation error for missing required flags in non-interactive mode.
func RequiredFlag(flag string) *ValidationError {
	return &ValidationError{
		Msg: fmt.Sprintf("--%s is required in non-interactive mode", flag),
		Tip: "Provide the flag value or run without --no-input for interactive prompts",
	}
}

// MutuallyExclusive is a validation error for conflicting options.
func MutuallyExclusive(arg, flag string) *ValidationError {
	return &ValidationError{
		Msg: fmt.Sprintf("cannot specify both %s argument and --%s flag", arg, flag),
		Tip: fmt.Sprintf("Use either '%s' or '--%s', not both", arg, flag),
	}
}

// TestMatch is a candidate test surfaced when a name resolves to more than one test.
type TestMatch struct {
	ID   string
	Name string
}

// AmbiguousTestError is returned when a test name matches more than one test id.
type AmbiguousTestError struct {
	Name       string
	Candidates []TestMatch
}

func (e *AmbiguousTestError) Error() string {
	return fmt.Sprintf("test name %q matches %d tests", e.Name, len(e.Candidates))
}

func (*AmbiguousTestError) Category() Category { return CatValidation }

// readOnlyError is a value-type sentinel so errors.Is matches by equality.
type readOnlyError struct{}

func (readOnlyError) Error() string      { return "read-only mode: writes are blocked" }
func (readOnlyError) Category() Category { return CatReadOnly }

// ErrReadOnly is returned when a non-GET request is attempted in read-only mode.
var ErrReadOnly UserError = readOnlyError{}

// joinSnippet appends a body-snippet diagnostic to base when one is available.
func joinSnippet(base, snippet string) string {
	if snippet == "" {
		return base
	}
	return base + ": " + snippet
}

// bodySnippet returns a single-line, control-stripped excerpt of body (≤512 bytes), or "" if there is nothing useful to show.
func bodySnippet(body []byte) string {
	s := strings.TrimSpace(string(body))
	if s == "" {
		return ""
	}
	s = strings.Join(strings.Fields(s), " ")
	s = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, s)
	const max = 512
	if len(s) > max {
		cut := max
		for cut > 0 && !utf8.RuneStart(s[cut]) {
			cut--
		}
		s = s[:cut] + "..."
	}
	return s
}

// IsSandboxBlocked reports whether a sandbox is blocking outbound network access.
func IsSandboxBlocked(err error) bool {
	ne, ok := errors.AsType[*NetworkError](err)
	if !ok || ne.Cause == nil {
		return false
	}
	msg := ne.Cause.Error()
	return strings.Contains(msg, ": Forbidden") ||
		strings.Contains(msg, ": Blocked") ||
		strings.Contains(msg, "sandbox")
}
