package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	"github.com/JetBrains/teamcity-cli/internal/cmd"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/config"
)

func main() {
	os.Exit(run())
}

func run() (exitCode int) {
	defer func() {
		if r := recover(); r != nil {
			_, _ = fmt.Fprintf(os.Stderr, "panic: %v\n\n%s\n", r, debug.Stack())
			_, _ = fmt.Fprintln(os.Stderr, "This is a bug. Please report it at https://jb.gg/tc/issues")
			exitCode = 1
		}
	}()

	if err := config.Init(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error initializing config: %v\n", err)
		return 1
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := cmd.Execute(ctx); err != nil {
		if exitErr, ok := errors.AsType[*cmdutil.ExitError](err); ok {
			return exitErr.Code
		}
		return 1
	}
	return 0
}
