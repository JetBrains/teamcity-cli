package analytics

import (
	"os"
	"testing"
)

// TestMain locks the analytics-package tests into a fail-closed default: with XDG_CONFIG_HOME pointing at a non-directory, DataDir fails, boot noops, and any naive Track call cannot reach a real FUS endpoint. Tests that need a writable dir (TestPipeline_EndToEnd) override it explicitly via t.Setenv.
func TestMain(m *testing.M) {
	os.Setenv("XDG_CONFIG_HOME", "/dev/null")
	os.Exit(m.Run())
}
