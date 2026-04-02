package config_test

import (
	"bytes"
	"testing"

	"github.com/JetBrains/teamcity-cli/internal/cmd"
	"github.com/JetBrains/teamcity-cli/internal/cmdtest"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/config"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setup(t *testing.T) {
	t.Helper()
	t.Setenv("TEAMCITY_URL", "https://test.example.com")
	t.Setenv("TEAMCITY_TOKEN", "test-token")
	config.SetConfigPathForTest(t.TempDir() + "/config.yml")
	config.ResetForTest()
}

func setupWithServer(t *testing.T) {
	t.Helper()
	setup(t)
	require.NoError(t, config.SetServer("https://tc.example.com", "tok", "alice"))
}

func capture(t *testing.T, args ...string) string {
	t.Helper()
	var buf bytes.Buffer
	f := cmdutil.NewFactory()
	f.Printer = &output.Printer{Out: &buf, ErrOut: &buf}
	root := cmd.NewRootCmdWithFactory(f)
	root.SetArgs(args)
	root.SetOut(&buf)
	root.SetErr(&buf)
	require.NoError(t, root.Execute())
	return buf.String()
}

func captureErr(t *testing.T, args ...string) error {
	t.Helper()
	var buf bytes.Buffer
	f := cmdutil.NewFactory()
	f.Printer = &output.Printer{Out: &buf, ErrOut: &buf}
	root := cmd.NewRootCmdWithFactory(f)
	root.SetArgs(args)
	root.SetOut(&buf)
	root.SetErr(&buf)
	err := root.Execute()
	require.Error(t, err)
	return err
}

func TestConfigSubcommandRequired(t *testing.T) {
	setup(t)
	cmdtest.RunCmdExpectErr(t, "requires a subcommand", "config")
}

func TestConfigListEmpty(t *testing.T) {
	setup(t)
	out := capture(t, "config", "list")
	assert.Contains(t, out, "default_server=")
}

func TestConfigListWithServer(t *testing.T) {
	setupWithServer(t)
	out := capture(t, "config", "list")
	assert.Contains(t, out, "default_server=https://tc.example.com")
	assert.Contains(t, out, "guest=false")
	assert.Contains(t, out, "ro=false")
}

func TestConfigListJSON(t *testing.T) {
	setupWithServer(t)
	out := capture(t, "config", "list", "--json")
	assert.Contains(t, out, `"default_server"`)
	assert.Contains(t, out, `"https://tc.example.com"`)
	assert.Contains(t, out, `"servers"`)
}

func TestConfigListEnvOverrides(t *testing.T) {
	setupWithServer(t)
	out := capture(t, "config", "list")
	assert.Contains(t, out, "Environment overrides")
	assert.Contains(t, out, "TEAMCITY_URL")
	assert.Contains(t, out, "TEAMCITY_TOKEN=****")
}

func TestConfigGetDefaultServer(t *testing.T) {
	setupWithServer(t)
	out := capture(t, "config", "get", "default_server")
	assert.Contains(t, out, "https://tc.example.com")
}

func TestConfigGetPerServerKey(t *testing.T) {
	setupWithServer(t)
	out := capture(t, "config", "get", "guest", "--server", "https://tc.example.com")
	assert.Contains(t, out, "false")
}

func TestConfigGetInvalidKey(t *testing.T) {
	setupWithServer(t)
	err := captureErr(t, "config", "get", "nonexistent")
	assert.Contains(t, err.Error(), "unknown key")
}

func TestConfigGetServerNotFound(t *testing.T) {
	setupWithServer(t)
	err := captureErr(t, "config", "get", "ro", "--server", "https://unknown.example.com")
	assert.Contains(t, err.Error(), "not found in configuration")
}

func TestConfigSetDefaultServer(t *testing.T) {
	setupWithServer(t)
	out := capture(t, "config", "set", "default_server", "tc.example.com")
	assert.Contains(t, out, "Set default_server")

	got := capture(t, "config", "get", "default_server")
	assert.Contains(t, got, "https://tc.example.com")
}

func TestConfigSetDefaultServerRejectsUnknown(t *testing.T) {
	setupWithServer(t)
	err := captureErr(t, "config", "set", "default_server", "typo.example.com")
	assert.Contains(t, err.Error(), "not found in configuration")
}

func TestConfigSetRO(t *testing.T) {
	setupWithServer(t)
	out := capture(t, "config", "set", "ro", "true", "--server", "https://tc.example.com")
	assert.Contains(t, out, "Set ro")

	got := capture(t, "config", "get", "ro", "--server", "https://tc.example.com")
	assert.Contains(t, got, "true")
}

func TestConfigSetInvalidBool(t *testing.T) {
	setupWithServer(t)
	err := captureErr(t, "config", "set", "ro", "maybe", "--server", "https://tc.example.com")
	assert.Contains(t, err.Error(), "invalid boolean")
}

func TestConfigResetRO(t *testing.T) {
	setupWithServer(t)
	capture(t, "config", "set", "ro", "true", "--server", "https://tc.example.com")
	out := capture(t, "config", "reset", "ro", "--server", "https://tc.example.com")
	assert.Contains(t, out, "Reset ro")

	got := capture(t, "config", "get", "ro", "--server", "https://tc.example.com")
	assert.Contains(t, got, "false")
}

func TestConfigResetDefaultServer(t *testing.T) {
	setupWithServer(t)
	out := capture(t, "config", "reset", "default_server")
	assert.Contains(t, out, "Reset default_server")
}

func TestConfigSetTokenExpiry(t *testing.T) {
	setupWithServer(t)
	capture(t, "config", "set", "token_expiry", "2026-01-01T00:00:00Z", "--server", "https://tc.example.com")
	got := capture(t, "config", "get", "token_expiry", "--server", "https://tc.example.com")
	assert.Contains(t, got, "2026-01-01T00:00:00Z")
}

func TestConfigListAliases(t *testing.T) {
	setupWithServer(t)
	require.NoError(t, config.AddAlias("rl", "run list"))
	require.NoError(t, config.AddAlias("mine", "run list --user=@me"))

	out := capture(t, "config", "list")
	assert.Contains(t, out, "Aliases:")
	assert.Contains(t, out, "2 configured")
}
