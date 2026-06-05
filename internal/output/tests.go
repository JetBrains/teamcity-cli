package output

import (
	"fmt"
	"strings"

	"github.com/JetBrains/teamcity-cli/api"
)

// TestCountsSummary renders nonzero passed/failed/muted/ignored counts, colored and joined; empty when all zero.
func TestCountsSummary(t *api.TestOccurrences) string {
	var parts []string
	if t.Passed > 0 {
		parts = append(parts, Green(fmt.Sprintf("%d passed", t.Passed)))
	}
	if t.Failed > 0 {
		parts = append(parts, Red(fmt.Sprintf("%d failed", t.Failed)))
	}
	if t.Muted > 0 {
		parts = append(parts, Faint(fmt.Sprintf("%d muted", t.Muted)))
	}
	if t.Ignored > 0 {
		parts = append(parts, Faint(fmt.Sprintf("%d ignored", t.Ignored)))
	}
	return strings.Join(parts, ", ")
}
