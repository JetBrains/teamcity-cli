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
		{".github/workflows/ci.yml", "ci.tc.yml"},
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
}

func TestMapRunner(t *testing.T) {
	t.Parallel()

	defaults := Options{}
	assert.Equal(t, "Ubuntu-24.04-Large", defaults.MapRunner("ubuntu-latest"))
	assert.Equal(t, "macOS-15-Sequoia-Large-Arm64", defaults.MapRunner("macos-latest"))
	assert.Equal(t, "custom-label", defaults.MapRunner("custom-label"))

	override := Options{RunnerMap: map[string]string{"ubuntu-latest": "My-Linux-Image"}}
	assert.Equal(t, "My-Linux-Image", override.MapRunner("ubuntu-latest"))
	assert.Equal(t, "macOS-15-Sequoia-Large-Arm64", override.MapRunner("macos-latest"))
}
