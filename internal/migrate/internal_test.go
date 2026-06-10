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
	// Punctuation outside the safe set must collapse to "_" so the id stays a valid dependency reference.
	assert.Equal(t, "Build_Test", SanitizeJobID("Build & Test"))
	assert.Equal(t, "Deploy_prod_", SanitizeJobID("Deploy (prod)"))
	assert.Regexp(t, `^[A-Za-z0-9_]+$`, SanitizeJobID("Build & Test"))
}

func TestBambooDockerDefaultsImage(t *testing.T) {
	t.Parallel()

	// push/run without an image must fall back like the build branch, not emit "docker push ".
	push := bambooDocker(map[string]any{"cmd": "push"}, nil, "job")
	assert.Equal(t, "docker push build:latest", push.Steps[0].ScriptContent)

	run := bambooDocker(map[string]any{"cmd": "run"}, nil, "job")
	assert.Equal(t, "docker run build:latest", run.Steps[0].ScriptContent)

	run = bambooDocker(map[string]any{"cmd": "run", "arguments": "-e FOO=1", "image": "app:1"}, nil, "job")
	assert.Equal(t, "docker run -e FOO=1 app:1", run.Steps[0].ScriptContent)
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
