package analytics

import "testing"

// TestLintScheme asserts every reference event in SampleEvents passes the client-side validator with no sentinels.
func TestLintScheme(t *testing.T) {
	findings, err := LintScheme()
	if err != nil {
		t.Fatalf("LintScheme: %v", err)
	}
	for _, f := range findings {
		t.Errorf("scheme drift: %s", f)
	}
}
