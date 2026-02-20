package api

import (
	"errors"
	"fmt"
)

// ErrAuthentication is returned when credentials are invalid or expired.
var ErrAuthentication = errors.New("authentication failed: invalid or expired credentials")

// PermissionError is returned when the user lacks permission for an action.
type PermissionError struct {
	Action string
}

func (e *PermissionError) Error() string {
	return fmt.Sprintf("permission denied: cannot %s", e.Action)
}

// NotFoundError is returned when a requested resource does not exist.
type NotFoundError struct {
	Resource string
	ID       string
	Message  string
}

func (e *NotFoundError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("%s '%s' not found", e.Resource, e.ID)
}

// NetworkError is returned when a connection to the server fails.
type NetworkError struct {
	URL   string
	Cause error
}

func (e *NetworkError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("cannot connect to %s: %v", e.URL, e.Cause)
	}
	return fmt.Sprintf("cannot connect to %s", e.URL)
}

func (e *NetworkError) Unwrap() error {
	return e.Cause
}
