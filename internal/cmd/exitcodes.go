package cmd

import "fmt"

const (
	ExitFailure   = 1
	ExitCancelled = 2
	ExitTimeout   = 124
)

// ExitError is returned by commands that need a specific exit code.
type ExitError struct {
	Code int
}

func (e *ExitError) Error() string {
	return fmt.Sprintf("exit status %d", e.Code)
}
