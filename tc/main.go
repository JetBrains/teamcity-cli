package main

import (
	"fmt"
	"os"

	"github.com/tiulpin/teamcity-cli/internal/cmd"
	"github.com/tiulpin/teamcity-cli/internal/config"
)

func main() {
	if err := config.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing config: %v\n", err)
		os.Exit(1)
	}

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
