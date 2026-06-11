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

func TestMapRunnerOverride(t *testing.T) {
	t.Parallel()

	// Default mapping is covered by TestResolveRunner; overrides win, missing keys fall through.
	override := Options{RunnerMap: map[string]string{"ubuntu-latest": "My-Linux-Image"}}
	assert.Equal(t, "My-Linux-Image", override.MapRunner("ubuntu-latest"))
	assert.Equal(t, "Mac-Medium", override.MapRunner("macos-latest"))
}

func TestResolveRunner(t *testing.T) {
	t.Parallel()

	tests := []struct {
		label string
		want  string
		known bool
	}{
		{"ubuntu-latest", "Linux-Large", true},
		{"self-hosted", "self-hosted", true},
		// OS-shaped labels outside the map resolve via substring heuristics.
		{"ubuntu-18.04", "Linux-Large", true},
		{"windows-11", "Windows-Medium", true},
		{"macos-12", "Mac-Medium", true},
		{"darwin-arm64", "Mac-Medium", true},
		// Anything else names a self-hosted runner in GHA semantics.
		{"buildjet-4vcpu", "self-hosted", false},
		{"my-gpu-box", "self-hosted", false},
	}
	for _, tt := range tests {
		got, known := Options{}.ResolveRunner(tt.label)
		assert.Equal(t, tt.want, got, tt.label)
		assert.Equal(t, tt.known, known, tt.label)
	}
}

func TestBuildRunnerMapPrefersLargeOverXLarge(t *testing.T) {
	t.Parallel()

	// The 2026.2 hosted-agent enum, in schema order: XLarge precedes Large.
	m := BuildRunnerMap([]string{"Windows-Small", "Mac-Medium", "Linux-XLarge", "Linux-Large", "Linux-Medium", "Linux-Small", "Windows-Medium"})
	assert.Equal(t, "Linux-Large", m["ubuntu-latest"])
	assert.Equal(t, "Mac-Medium", m["macos-latest"])
	assert.Equal(t, "Windows-Medium", m["windows-latest"])
	assert.Equal(t, "Linux-Large", m["ubuntu-22.04"])
}
