// Package status formats build status tokens for display in CLI output.
package status

import "strings"

// FormatBuildStatus returns a lowercase, human-readable status token.
func FormatBuildStatus(raw string) string {
	return strings.ToUpper(strings.TrimSpace(raw))
}
