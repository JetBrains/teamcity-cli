package project

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseKotlinErrors(T *testing.T) {
	T.Parallel()

	tests := []struct {
		name  string
		input string
		want  int
		match string
	}{
		{"kotlin compiler error", "e: /path/to/Settings.kts:42:10: Unresolved reference: foo", 1, "Unresolved reference: foo"},
		{"multiple kotlin errors", "some output\ne: /src/Settings.kts:10:5: Type mismatch\ne: /src/Other.kts:20:1: Expecting member declaration", 2, ""},
		{"maven ERROR fallback", "[ERROR] Failed to execute goal org.jetbrains.maven:compile", 1, "Failed to execute goal"},
		{"BUILD FAILURE excluded from fallback", "[ERROR] BUILD FAILURE", 0, ""},
		{"empty input", "", 0, ""},
		{"no errors in output", "[INFO] Build completed successfully\n[WARNING] Something minor", 0, ""},
	}

	for _, tc := range tests {
		T.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := parseKotlinErrors(tc.input)
			assert.Len(t, got, tc.want)
			if tc.match != "" && len(got) > 0 {
				assert.Contains(t, got[0], tc.match)
			}
		})
	}
}

func TestNewProjectSettingsValidateCmd_HelpClarifiesPathOnly(t *testing.T) {
	t.Parallel()

	cmd := newProjectSettingsValidateCmd(nil)

	assert.Equal(t, "validate [path]", cmd.Use)
	assert.Contains(t, cmd.Long, "Optional [path] must be a filesystem path to a .teamcity directory.")
	assert.Contains(t, cmd.Long, "This command does not accept TeamCity project IDs and has no --dir flag.")
	assert.Nil(t, cmd.Flags().Lookup("dir"))
}
