package cmdutil

import (
	"context"
	"errors"

	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/charmbracelet/huh"
)

// ProgramEnded reports a command result without exposing errors that may contain credentials.
func ProgramEnded(p *output.Printer, ctx context.Context, err error, done, failed string) {
	switch {
	case errors.Is(ctx.Err(), context.Canceled), errors.Is(err, context.Canceled), errors.Is(err, huh.ErrUserAborted):
		p.ProgramStatus(output.ProgramIdle, "")
	case errors.Is(ctx.Err(), context.DeadlineExceeded), errors.Is(err, context.DeadlineExceeded):
		p.ProgramStatus(output.ProgramError, failed+": timed out")
	case err != nil:
		p.ProgramStatus(output.ProgramError, failed)
	default:
		p.ProgramStatus(output.ProgramDone, done)
	}
}
