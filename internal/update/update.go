package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/JetBrains/teamcity-cli/internal/version"
	"github.com/mattn/go-isatty"
)

const (
	EnvNoUpdateCheck = "TC_NO_UPDATE_CHECK"
	CheckInterval    = 24 * time.Hour
	stateFileName    = "update-check.json"
)

// --- CI detection ---

func IsCI() bool {
	for _, key := range []string{"CI", "BUILD_NUMBER", "TEAMCITY_VERSION"} {
		if os.Getenv(key) != "" {
			return true
		}
	}
	return false
}

// --- State persistence ---

type State struct {
	LastCheckedAt time.Time `json:"last_checked_at"`
	LatestVersion string    `json:"latest_version"`
	LatestURL     string    `json:"latest_url"`
}

func (s *State) IsStale(interval time.Duration) bool {
	return time.Since(s.LastCheckedAt) > interval
}

func stateFilePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "tc", stateFileName)
}

func LoadState() *State {
	path := stateFilePath()
	if path == "" {
		return &State{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return &State{}
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return &State{}
	}
	return &s
}

func SaveState(s *State) {
	path := stateFilePath()
	if path == "" {
		return
	}
	data, err := json.Marshal(s)
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(path), 0700)
	_ = os.WriteFile(path, data, 0600)
}

// --- Update check orchestration ---

func IsDisabled() bool {
	if v := os.Getenv(EnvNoUpdateCheck); v == "1" || v == "true" || v == "yes" {
		return true
	}
	if !isatty.IsTerminal(os.Stderr.Fd()) {
		return true
	}
	return IsCI()
}

// Check fetches the latest release (respecting the 24h throttle) and returns
// it if it's newer than the running version. Returns nil otherwise.
func Check(ctx context.Context) *ReleaseInfo {
	state := LoadState()

	if !state.IsStale(CheckInterval) {
		if state.LatestVersion != "" && IsNewer(version.Version, state.LatestVersion) {
			return &ReleaseInfo{Version: state.LatestVersion, URL: state.LatestURL}
		}
		return nil
	}

	release, err := LatestRelease(ctx)
	if err != nil {
		return nil
	}

	state.LastCheckedAt = time.Now()
	state.LatestVersion = release.Version
	state.LatestURL = release.URL
	SaveState(state)

	if IsNewer(version.Version, release.Version) {
		return release
	}
	return nil
}

const noticeWait = 500 * time.Millisecond

// CheckInBackground starts an update check in a goroutine and returns a
// function that, when called, waits briefly for the result and prints a
// one-line notice if a new version is available. The wait is bounded so
// slow networks don't delay command exit.
func CheckInBackground(w io.Writer, quiet bool) func() {
	if IsDisabled() || quiet {
		return func() {}
	}

	done := make(chan *ReleaseInfo, 1)
	go func() {
		done <- Check(context.Background())
	}()

	return func() {
		select {
		case release := <-done:
			if release != nil {
				PrintNotice(w, version.Version, release)
			}
		case <-time.After(noticeWait):
			// Don't delay exit — the check will be cached next time.
		}
	}
}

func PrintNotice(w io.Writer, currentVersion string, r *ReleaseInfo) {
	_, _ = fmt.Fprintf(w, "\n%s A new version is available: %s → %s — run %s to see how to upgrade\n",
		output.Yellow("!"),
		output.Faint("v"+currentVersion),
		output.Green("v"+r.Version),
		output.Cyan(`"teamcity update"`),
	)
}
