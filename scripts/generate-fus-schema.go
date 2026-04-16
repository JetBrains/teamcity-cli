//go:build ignore

// Script to generate internal/analytics/schema.json from the in-Go FUS scheme.
//
// The output is the file the AP team registers in the metadata repo. The same
// Scheme value is used at runtime by fus.NewValidator, so there is no drift
// between the registered schema and the collector code.
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
)

const outPath = "internal/analytics/schema.json"

func main() {
	if _, err := fus.NewValidator(analytics.Scheme); err != nil {
		fmt.Fprintf(os.Stderr, "scheme failed validator construction: %v\n", err)
		os.Exit(1)
	}
	f, err := os.Create(outPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create %s: %v\n", outPath, err)
		os.Exit(1)
	}
	defer f.Close()
	if err := fus.WriteSchemeJSON(analytics.Scheme, f); err != nil {
		fmt.Fprintf(os.Stderr, "write scheme: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s\n", outPath)
}
