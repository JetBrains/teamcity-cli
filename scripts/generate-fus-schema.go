//go:build ignore

// Script to generate FUS schema files from the in-Go FUS scheme.
//
// Produces two files:
//   - internal/analytics/schema.json: the AP metadata format (EventGroupRemoteDescriptors),
//     used at runtime by fus.NewValidator as the embedded fallback scheme.
//   - internal/analytics/events-scheme.json: the events scheme format (EventsScheme),
//     used by the FUS metadata team to generate metadata entries. Attach this to
//     YT issues in the FUS project when requesting metadata changes.
//
// Run with:
//
//	go run scripts/generate-fus-schema.go
package main

import (
	"fmt"
	"os"

	fus "github.com/JetBrains/fus-reporting-api-go"
	"github.com/JetBrains/teamcity-cli/internal/analytics"
	"github.com/JetBrains/teamcity-cli/internal/version"
)

func main() {
	if _, err := fus.NewValidator(analytics.Scheme); err != nil {
		fmt.Fprintf(os.Stderr, "scheme failed validator construction: %v\n", err)
		os.Exit(1)
	}

	if err := writeFile("internal/analytics/schema.json", func(f *os.File) error {
		return fus.WriteSchemeJSON(analytics.Scheme, f)
	}); err != nil {
		fmt.Fprintf(os.Stderr, "schema.json: %v\n", err)
		os.Exit(1)
	}

	es, err := fus.BuildEventsScheme(analytics.Scheme, fus.RecorderConfig{
		RecorderID:      analytics.RecorderID,
		RecorderVersion: analytics.RecorderVersion,
		ProductCode:     analytics.ProductCode,
	}, version.String())
	if err != nil {
		fmt.Fprintf(os.Stderr, "build events scheme: %v\n", err)
		os.Exit(1)
	}
	if err := writeFile("internal/analytics/events-scheme.json", func(f *os.File) error {
		return fus.WriteEventsSchemeJSON(es, f)
	}); err != nil {
		fmt.Fprintf(os.Stderr, "events-scheme.json: %v\n", err)
		os.Exit(1)
	}
}

func writeFile(path string, write func(*os.File) error) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := write(f); err != nil {
		return err
	}
	fmt.Printf("wrote %s\n", path)
	return nil
}
