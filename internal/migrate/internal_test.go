package migrate

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOutputFileName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected string
	}{
		{".github/workflows/release.yaml", "release.tc.yaml"},
		{".gitlab-ci.yml", ".gitlab-ci.tc.yml"},
		{"Jenkinsfile", "Jenkinsfile.tc.yml"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, OutputFileName(tt.input))
	}
}

func TestSanitizeJobID(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "test_unit", SanitizeJobID("test-unit"))
	assert.Equal(t, "build_v2_0", SanitizeJobID("build-v2.0"))
	assert.Equal(t, "simple", SanitizeJobID("simple"))
	// Punctuation outside the safe set must collapse to "_" so the id stays a valid dependency reference.
	assert.Equal(t, "Build_Test", SanitizeJobID("Build & Test"))
	assert.Equal(t, "Deploy_prod_", SanitizeJobID("Deploy (prod)"))
}
