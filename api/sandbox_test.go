//go:build sandbox

package api_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSandboxGuestAuth builds the CLI and runs a guest API call inside
// @anthropic-ai/sandbox-runtime, verifying that TLS and proxy work.
//
// Prerequisites: Node.js 22+ (npx), bubblewrap+socat on Linux.
// Run: go test -tags=sandbox -run TestSandboxGuestAuth ./api -v -timeout 2m
func TestSandboxGuestAuth(T *testing.T) {
	if runtime.GOOS == "windows" {
		T.Skip("sandbox-runtime not supported on Windows")
	}

	npx, err := exec.LookPath("npx")
	if err != nil {
		T.Skip("npx not found, skipping sandbox test")
	}

	binary := filepath.Join(T.TempDir(), "teamcity")
	build := exec.Command("go", "build", "-o", binary, "../tc")
	build.Stdout = os.Stdout
	build.Stderr = os.Stderr
	require.NoError(T, build.Run(), "build CLI binary")

	settings := filepath.Join(T.TempDir(), "srt-settings.json")
	require.NoError(T, os.WriteFile(settings, []byte(`{
		"network": {
			"allowedDomains": ["cli.teamcity.com"],
			"deniedDomains": []
		},
		"filesystem": {
			"denyRead": [],
			"allowWrite": ["/tmp"],
			"denyWrite": []
		}
	}`), 0o644))

	// sandbox-runtime sets HTTPS_PROXY=localhost but may bind IPv4-only → force 127.0.0.1
	wrapper := filepath.Join(T.TempDir(), "proxy-fix.sh")
	require.NoError(T, os.WriteFile(wrapper, []byte(`#!/bin/sh
HTTPS_PROXY=$(echo "$HTTPS_PROXY" | sed 's/localhost/127.0.0.1/g; s/\[::1\]/127.0.0.1/g')
HTTP_PROXY=$(echo "$HTTP_PROXY" | sed 's/localhost/127.0.0.1/g; s/\[::1\]/127.0.0.1/g')
export HTTPS_PROXY HTTP_PROXY
exec "$@"
`), 0o755))

	sandboxCmd := func(env []string, args ...string) *exec.Cmd {
		full := []string{"@anthropic-ai/sandbox-runtime", "-s", settings, wrapper}
		full = append(full, args...)
		cmd := exec.Command(npx, full...)
		cmd.Env = append(os.Environ(), env...)
		return cmd
	}

	// Probe: verify sandbox-runtime works on this host (bwrap may fail on restricted VMs).
	probe := sandboxCmd(nil, "true")
	if out, err := probe.CombinedOutput(); err != nil {
		T.Skipf("sandbox-runtime not functional on this host: %s", bytes.TrimSpace(out))
	}

	T.Run("guest API call", func(T *testing.T) {
		cmd := sandboxCmd(
			[]string{"TEAMCITY_URL=https://cli.teamcity.com", "TEAMCITY_GUEST=1", "NO_COLOR=1"},
			binary, "api", "/app/rest/server",
		)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		require.NoError(T, cmd.Run(), "sandbox command failed: stdout=%s stderr=%s", stdout.String(), stderr.String())

		var resp map[string]any
		require.NoError(T, json.Unmarshal(stdout.Bytes(), &resp), "response is not JSON: %s", stdout.String())
		assert.NotEmpty(T, resp["buildNumber"], "expected buildNumber in server response")
	})

	T.Run("blocked domain", func(T *testing.T) {
		cmd := sandboxCmd(
			[]string{"TEAMCITY_URL=https://not-allowed.example.com", "TEAMCITY_GUEST=1", "NO_COLOR=1"},
			binary, "api", "/app/rest/server",
		)
		out, _ := cmd.CombinedOutput()
		// The renderer surfaces the raw transport error plus the category-default
		// sandbox hint. Assert on the hint phrasing — it's the signal that the
		// CLI recognised this as a sandbox block rather than a generic network error.
		assert.Contains(T, string(out), "sandbox allowlist")
	})
}
