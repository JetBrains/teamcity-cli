package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	if os.Getenv("TEAMCITY_UPDATE_FIXTURE") == "1" {
		if strings.EqualFold(filepath.Base(os.Args[0]), "winget.exe") {
			os.Exit(runWinGetFixture())
		}
	}
	os.Exit(m.Run())
}

func runWinGetFixture() int {
	data, _ := json.Marshal(struct {
		ProcessID int
		Args      []string
	}{os.Getppid(), os.Args[1:]})
	if err := os.WriteFile(os.Getenv("TEAMCITY_UPDATE_MANAGER_RESULT"), data, 0600); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println("WinGet fixture stdout")
	if os.Getenv("TEAMCITY_UPDATE_FAIL") == "1" {
		fmt.Fprintln(os.Stderr, "WinGet fixture stderr")
		return 23
	}
	installed := os.Getenv("TEAMCITY_UPDATE_CLI_PATH")
	if err := os.Remove(installed); err != nil {
		fmt.Fprintln(os.Stderr, "CLI executable is still locked:", err)
		return 1
	}
	executable, _ := os.Executable()
	data, err := os.ReadFile(executable)
	if err == nil {
		err = os.WriteFile(installed, data, 0700)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println("CLI executable replaced")
	return 0
}

func TestWinGetUpgradeParent(t *testing.T) {
	if os.Getenv("TEAMCITY_UPDATE_FIXTURE") != "1" {
		t.Skip("subprocess fixture")
	}
	result, err := Upgrade(t.Context(), InstallWinGet, "9.9.9", io.Discard, io.Discard)
	require.NoError(t, err)
	require.True(t, result.Deferred)
	data, err := json.Marshal(result)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(os.Getenv("TEAMCITY_UPDATE_RESULT"), data, 0600))
	// The updater must not run while this process still maps the installed executable.
	time.Sleep(200 * time.Millisecond)
	require.NoFileExists(t, os.Getenv("TEAMCITY_UPDATE_MANAGER_RESULT"))
}

func TestWinGetUpgradeHandoff(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprintf("failure=%t", fail), func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "CLI's update & тест")
			require.NoError(t, os.MkdirAll(filepath.Join(dir, "tools"), 0700))
			self, err := os.Executable()
			require.NoError(t, err)
			data, err := os.ReadFile(self)
			require.NoError(t, err)
			executable := filepath.Join(dir, "teamcity.exe")
			require.NoError(t, os.WriteFile(executable, data, 0700))
			require.NoError(t, os.WriteFile(filepath.Join(dir, "tools", "winget.exe"), data, 0700))
			resultPath := filepath.Join(dir, "handoff.json")
			managerResult := filepath.Join(dir, "manager.json")
			t.Setenv("PATH", filepath.Join(dir, "tools")+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("TMP", dir)
			t.Setenv("TEMP", dir)
			t.Setenv("TEAMCITY_UPDATE_FIXTURE", "1")
			t.Setenv("TEAMCITY_UPDATE_FAIL", "0")
			if fail {
				t.Setenv("TEAMCITY_UPDATE_FAIL", "1")
			}
			t.Setenv("TEAMCITY_UPDATE_CLI_PATH", executable)
			t.Setenv("TEAMCITY_UPDATE_RESULT", resultPath)
			t.Setenv("TEAMCITY_UPDATE_MANAGER_RESULT", managerResult)
			parent := exec.CommandContext(t.Context(), executable, "-test.run=^TestWinGetUpgradeParent$", "-test.timeout=30s")
			output, err := parent.CombinedOutput()
			require.NoError(t, err, "%s", output)
			data, err = os.ReadFile(resultPath)
			require.NoError(t, err)
			var result UpgradeResult
			require.NoError(t, json.Unmarshal(data, &result))
			var log string
			require.Eventually(t, func() bool {
				data, err := os.ReadFile(result.LogPath)
				log = string(data)
				return err == nil && (strings.Contains(log, "Update completed.") || strings.Contains(log, "Error:"))
			}, 20*time.Second, 100*time.Millisecond, "update log: %s", result.LogPath)
			data, err = os.ReadFile(managerResult)
			require.NoError(t, err, "%s", log)
			var manager struct {
				ProcessID int
				Args      []string
			}
			require.NoError(t, json.Unmarshal(data, &manager))
			t.Cleanup(func() {
				process, err := os.FindProcess(manager.ProcessID)
				if err == nil {
					_ = process.Kill()
					_ = process.Release()
				}
			})
			_, args := InstallWinGet.upgradeCommand()
			assert.Equal(t, args, manager.Args)
			if fail {
				assert.Contains(t, log, "WinGet failed with exit code 0x00000017")
				assert.NotContains(t, log, "Update completed.")
			} else {
				assert.Contains(t, log, "CLI executable replaced")
				assert.Contains(t, log, "Update completed.")
			}
		})
	}
}

func TestWinGetUpgradeCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := startWinGetUpgrade(ctx, nil)
	require.ErrorIs(t, err, context.Canceled)
}

func TestWinGetUpgradeMissingManager(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := startWinGetUpgrade(t.Context(), nil)
	require.ErrorIs(t, err, exec.ErrNotFound)
}
