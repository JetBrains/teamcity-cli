package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/JetBrains/teamcity-cli/internal/cmd"
	"github.com/JetBrains/teamcity-cli/internal/config"
)

func main() {
	if err := config.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing config: %v\n", err)
		os.Exit(1)
	}

	if err := cmd.Execute(); err != nil {
		if exitErr, ok := errors.AsType[*cmd.ExitError](err); ok {
			os.Exit(exitErr.Code)
		}
		os.Exit(1)
	}
}
