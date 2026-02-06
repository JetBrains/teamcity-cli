package cmd

import (
	"testing"

	"github.com/JetBrains/teamcity-cli/internal/api"
	"github.com/stretchr/testify/assert"
)

func TestParseKotlinErrors(T *testing.T) {
	T.Parallel()

	tests := []struct {
		name  string
		input string
		want  int    // expected number of errors
		match string // substring expected in first error
	}{
		{
			name:  "kotlin compiler error",
			input: "e: /path/to/Settings.kts:42:10: Unresolved reference: foo",
			want:  1,
			match: "Unresolved reference: foo",
		},
		{
			name: "multiple kotlin errors",
			input: `some output
e: /src/Settings.kts:10:5: Type mismatch
e: /src/Other.kts:20:1: Expecting member declaration`,
			want: 2,
		},
		{
			name:  "maven ERROR fallback",
			input: "[ERROR] Failed to execute goal org.jetbrains.maven:compile",
			want:  1,
			match: "Failed to execute goal",
		},
		{
			name:  "BUILD FAILURE excluded from fallback",
			input: "[ERROR] BUILD FAILURE",
			want:  0,
		},
		{
			name:  "empty input",
			input: "",
			want:  0,
		},
		{
			name:  "no errors in output",
			input: "[INFO] Build completed successfully\n[WARNING] Something minor",
			want:  0,
		},
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

func TestFormatWatchLogLine(T *testing.T) {
	T.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "standard log line with step",
			input: "[10:30:45] : [Step 1/3] Compiling sources",
			want:  "[10:30:45] [Step 1/3] Compiling sources",
		},
		{
			name:  "too short",
			input: "[short]",
			want:  "",
		},
		{
			name:  "no opening bracket",
			input: "plain text without timestamp",
			want:  "",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "close bracket at position 8 passes",
			input: "[1234567]rest",
			want:  "[1234567] rest",
		},
	}

	for _, tc := range tests {
		T.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, formatWatchLogLine(tc.input))
		})
	}
}

func TestParseWatchLogLines(T *testing.T) {
	T.Parallel()

	tests := []struct {
		name  string
		input string
		want  int
	}{
		{"filters empty lines", "\n\n\n", 0},
		{"filters export and exec", "export FOO=bar\nexec /bin/sh\n", 0},
		{"filters Current time", "Current time: 2026-01-01 10:00:00", 0},
		{"keeps valid log line", "[10:30:45] : [Step 1/1] Hello\r\n", 1},
	}

	for _, tc := range tests {
		T.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Len(t, parseWatchLogLines(tc.input), tc.want)
		})
	}
}

func TestFlattenArtifacts(T *testing.T) {
	T.Parallel()

	tests := []struct {
		name      string
		artifacts []api.Artifact
		wantNames []string
		wantSize  int64
	}{
		{
			name:      "empty list",
			artifacts: nil,
			wantNames: nil,
			wantSize:  0,
		},
		{
			name: "flat files",
			artifacts: []api.Artifact{
				{Name: "a.txt", Size: 100},
				{Name: "b.txt", Size: 200},
			},
			wantNames: []string{"a.txt", "b.txt"},
			wantSize:  300,
		},
		{
			name: "nested directory",
			artifacts: []api.Artifact{
				{Name: "dir", Children: &api.Artifacts{
					File: []api.Artifact{
						{Name: "inner.txt", Size: 50},
					},
				}},
				{Name: "root.txt", Size: 10},
			},
			wantNames: []string{"dir/inner.txt", "root.txt"},
			wantSize:  60,
		},
		{
			name: "deeply nested",
			artifacts: []api.Artifact{
				{Name: "a", Children: &api.Artifacts{
					File: []api.Artifact{
						{Name: "b", Children: &api.Artifacts{
							File: []api.Artifact{
								{Name: "c.txt", Size: 1},
							},
						}},
					},
				}},
			},
			wantNames: []string{"a/b/c.txt"},
			wantSize:  1,
		},
	}

	for _, tc := range tests {
		T.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, size := flattenArtifacts(tc.artifacts, "")
			assert.Equal(t, tc.wantSize, size)
			var names []string
			for _, a := range got {
				names = append(names, a.Name)
			}
			assert.Equal(t, tc.wantNames, names)
		})
	}
}
