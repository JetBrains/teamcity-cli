package update

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/output"
	updater "github.com/JetBrains/teamcity-cli/internal/update"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateBehavior(t *testing.T) {
	for _, scenario := range []struct {
		name        string
		args        []string
		wantUpgrade bool
		wantError   string
	}{
		{"check", []string{"--check"}, false, ""},
		{"json", []string{"--json"}, false, ""},
		{"noninteractive", nil, false, "confirmation required"},
		{"yes", []string{"--yes"}, true, ""},
		{"failed", []string{"--yes"}, true, "update failed"},
		{"conflicting", []string{"--check", "--yes"}, false, "none of the others"},
		{"current", nil, false, ""},
		{"unsupported", nil, false, ""},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			previousLatest, previousDetect, previousUpgrade := latestRelease, detectInstallMethod, upgrade
			t.Cleanup(func() { latestRelease, detectInstallMethod, upgrade = previousLatest, previousDetect, previousUpgrade })
			latestRelease = func(context.Context) (*updater.ReleaseInfo, error) {
				version := "9999.0.0"
				if scenario.name == "current" {
					version = "0.0.0"
				}
				return &updater.ReleaseInfo{Version: version, URL: "https://example.invalid/release"}, nil
			}
			detectInstallMethod = func() updater.InstallMethod {
				if scenario.name == "unsupported" {
					return updater.InstallApt
				}
				return updater.InstallHomebrew
			}
			called := false
			upgrade = func(context.Context, updater.InstallMethod, string, io.Writer, io.Writer) error {
				called = true
				if scenario.name == "failed" {
					return errors.New("manager failed")
				}
				return nil
			}
			var stdout, stderr bytes.Buffer
			factory := cmdutil.NewFactory()
			factory.NoInput = true
			factory.Printer = &output.Printer{Out: &stdout, ErrOut: &stderr}
			factory.IOStreams = &cmdutil.IOStreams{Out: &stdout, ErrOut: &stderr}
			command := NewCmd(factory)
			command.SetOut(&stdout)
			command.SetErr(&stderr)
			command.SetArgs(scenario.args)
			err := command.Execute()
			if scenario.wantError != "" {
				require.ErrorContains(t, err, scenario.wantError)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, scenario.wantUpgrade, called)
			if scenario.name == "json" {
				assert.Contains(t, stdout.String(), `"update_available": true`)
				assert.NotContains(t, stdout.String(), "Checking for updates")
			}
		})
	}
}
