package main

import (
	"errors"
	"fmt"
	"os"
	"runtime/debug"

	"github.com/JetBrains/teamcity-cli/internal/cmd"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/config"
)

func main() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "panic: %v\n\n%s\n", r, debug.Stack())
			fmt.Fprintln(os.Stderr, "This is a bug. Please report it at https://jb.gg/tc/issues")
			os.Exit(1)
		}
	}()

	if err := config.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing config: %v\n", err)
		os.Exit(1)
	}

	if err := cmd.Execute(); err != nil {
		if exitErr, ok := errors.AsType[*cmdutil.ExitError](err); ok {
			os.Exit(exitErr.Code)
		}
		os.Exit(1)
	}
}
