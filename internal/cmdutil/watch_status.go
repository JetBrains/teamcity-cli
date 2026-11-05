package cmdutil

import (
	"context"
	"errors"

	"github.com/JetBrains/teamcity-cli/internal/output"
)

// WatchEnded reports the observed outcome without treating an early exit as completion.
func WatchEnded(p *output.Printer, ctx context.Context, runID string, finished bool, err error) {
	state, message := watchOutcome(ctx, finished, err)
	if message != "" {
		message = "Run " + runID + ": " + message
	}
	p.ProgramStatus(state, message)
}

func watchOutcome(ctx context.Context, finished bool, err error) (string, string) {
	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded), errors.Is(err, context.DeadlineExceeded):
		return output.ProgramError, "watch timed out"
	case errors.Is(ctx.Err(), context.Canceled), errors.Is(err, context.Canceled):
		return output.ProgramIdle, ""
	case err == nil && finished:
		return output.ProgramDone, "succeeded"
	case err == nil:
		return output.ProgramIdle, ""
	}
	if exit, ok := errors.AsType[*ExitError](err); ok {
		switch exit.Code {
		case ExitTimeout:
			return output.ProgramError, "watch timed out"
		case ExitFailure:
			if finished {
				return output.ProgramError, "failed"
			}
		case ExitCancelled:
			if finished {
				return output.ProgramError, "canceled"
			}
		}
	}
	return output.ProgramError, "could not continue watching"
}
