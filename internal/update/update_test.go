package update

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setHomeForTest points os.UserHomeDir at dir on every platform: HOME for unix, USERPROFILE for Windows.
func setHomeForTest(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
}

func TestIsCI(t *testing.T) {
	// IsCI reads process env, so subtests can't t.Parallel.
	cases := []struct {
		name string
		env  map[string]string
		want bool
	}{
		{"all unset", map[string]string{"CI": "", "BUILD_NUMBER": "", "TEAMCITY_VERSION": ""}, false},
		{"CI=true", map[string]string{"CI": "true", "BUILD_NUMBER": "", "TEAMCITY_VERSION": ""}, true},
		{"BUILD_NUMBER set", map[string]string{"CI": "", "BUILD_NUMBER": "42", "TEAMCITY_VERSION": ""}, true},
		{"TEAMCITY_VERSION set", map[string]string{"CI": "", "BUILD_NUMBER": "", "TEAMCITY_VERSION": "2025.7"}, true},
		{"any non-empty wins", map[string]string{"CI": "false", "BUILD_NUMBER": "", "TEAMCITY_VERSION": ""}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			assert.Equal(t, tc.want, IsCI())
		})
	}
}

func TestStateIsStale(t *testing.T) {
	t.Parallel()

	now := time.Now()
	cases := []struct {
		name     string
		last     time.Time
		interval time.Duration
		want     bool
	}{
		{"never checked → stale", time.Time{}, 24 * time.Hour, true},
		{"just checked → fresh", now, 24 * time.Hour, false},
		{"23h ago, 24h interval → fresh", now.Add(-23 * time.Hour), 24 * time.Hour, false},
		{"25h ago, 24h interval → stale", now.Add(-25 * time.Hour), 24 * time.Hour, true},
		{"future timestamp → fresh (clock skew safe)", now.Add(time.Hour), 24 * time.Hour, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := &State{LastCheckedAt: tc.last}
			assert.Equal(t, tc.want, s.IsStale(tc.interval))
		})
	}
}

func TestLoadSaveState_RoundTrip(t *testing.T) {
	// HOME→tempdir so stateFilePath writes under t.TempDir(), not real ~/.config/tc/.
	home := t.TempDir()
	setHomeForTest(t, home)

	original := &State{
		LastCheckedAt: time.Now().UTC().Truncate(time.Second), // truncate for JSON round-trip stability
		LatestVersion: "1.2.3",
		LatestURL:     "https://example.com/release",
	}

	SaveState(original)

	// File must be under HOME and be 0600 readable. POSIX perm bits aren't
	// meaningful on Windows, so we only assert the perm on unix-like systems.
	want := filepath.Join(home, ".config", "tc", "update-check.json")
	info, err := os.Stat(want)
	require.NoError(t, err, "state file must exist at %s", want)
	if runtime.GOOS != "windows" {
		assert.Equal(t, os.FileMode(0600), info.Mode().Perm(), "file must be user-only")
	}

	// Reading back must restore every field.
	got := LoadState()
	assert.Equal(t, original.LastCheckedAt.Unix(), got.LastCheckedAt.Unix())
	assert.Equal(t, original.LatestVersion, got.LatestVersion)
	assert.Equal(t, original.LatestURL, got.LatestURL)
}

func TestLoadState_MissingFile(t *testing.T) {
	setHomeForTest(t, t.TempDir())
	got := LoadState()
	require.NotNil(t, got)
	assert.True(t, got.LastCheckedAt.IsZero(), "missing file → zero state")
	assert.Empty(t, got.LatestVersion)
}

func TestLoadState_CorruptFile(t *testing.T) {
	home := t.TempDir()
	setHomeForTest(t, home)
	dir := filepath.Join(home, ".config", "tc")
	require.NoError(t, os.MkdirAll(dir, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "update-check.json"), []byte("{not json"), 0600))

	got := LoadState()
	require.NotNil(t, got, "corrupt JSON must not panic")
	assert.True(t, got.LastCheckedAt.IsZero(), "corrupt file → zero state")
}

func TestSaveState_FileFormat(t *testing.T) {
	home := t.TempDir()
	setHomeForTest(t, home)

	SaveState(&State{
		LastCheckedAt: time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC),
		LatestVersion: "v1.0.0",
		LatestURL:     "https://github.com/JetBrains/teamcity-cli/releases/tag/v1.0.0",
	})

	data, err := os.ReadFile(filepath.Join(home, ".config", "tc", "update-check.json"))
	require.NoError(t, err)

	// Format pin: documented field names for old/new client compat.
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(data, &parsed))
	assert.Contains(t, parsed, "last_checked_at")
	assert.Contains(t, parsed, "latest_version")
	assert.Contains(t, parsed, "latest_url")
}

func TestPrintNotice(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	PrintNotice(&buf, "0.10.0", &ReleaseInfo{Version: "1.0.0", URL: "https://github.com/x/y/releases/v1.0.0"})

	got := buf.String()
	assert.Contains(t, got, "A new version is available")
	assert.Contains(t, got, "v0.10.0")
	assert.Contains(t, got, "v1.0.0")
	assert.Contains(t, got, "teamcity update")
}

func TestCheck_CachedAndNotNewer(t *testing.T) {
	// version.Version defaults to "dev" → parses as 0.0.0, so cached "0.0.0" is NOT newer → no notice.
	setHomeForTest(t, t.TempDir())

	SaveState(&State{
		LastCheckedAt: time.Now(),
		LatestVersion: "0.0.0",
		LatestURL:     "https://example.com",
	})

	got := Check(t.Context())
	assert.Nil(t, got, "cached version not newer → no notice")
}

func TestIsDisabled_EnvOverride(t *testing.T) {
	// Only the truthy-env path is testable here; under `go test` stderr is never a TTY.
	for _, v := range []string{"1", "true", "yes"} {
		t.Run("TEAMCITY_NO_UPDATE="+v, func(t *testing.T) {
			t.Setenv(EnvNoUpdateCheck, v)
			assert.True(t, IsDisabled())
		})
	}
}
