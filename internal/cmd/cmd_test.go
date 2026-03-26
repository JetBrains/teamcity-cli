package cmd_test

import (
	"bytes"
	"testing"

	"github.com/JetBrains/teamcity-cli/internal/cmd"
	"github.com/JetBrains/teamcity-cli/internal/cmdtest"
	"github.com/JetBrains/teamcity-cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig(T *testing.T) {
	cmdtest.SetupMockClient(T)

	assert.True(T, config.IsConfigured(), "IsConfigured() with env vars")
	assert.NotEmpty(T, config.GetServerURL(), "GetServerURL()")
	assert.NotEmpty(T, config.GetToken(), "GetToken()")
}

func TestListLimitValidation(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	cmdtest.RunCmdWithFactoryExpectErr(T, f, "--limit must be a positive number", "project", "list", "--limit", "0")
	cmdtest.RunCmdWithFactoryExpectErr(T, f, "--limit must be a positive number", "run", "list", "--limit", "-1")
	cmdtest.RunCmdWithFactoryExpectErr(T, f, "--limit must be a positive number", "job", "list", "--limit", "0")
	cmdtest.RunCmdWithFactoryExpectErr(T, f, "--limit must be a positive number", "agent", "list", "--limit", "-5")
}

func TestHelpCommands(T *testing.T) {
	T.Parallel()

	commands := [][]string{
		{"--help"},
		{"project", "--help"},
		{"job", "--help"},
		{"run", "--help"},
		{"queue", "--help"},
		{"agent", "--help"},
		{"pool", "--help"},
		{"auth", "--help"},
		{"api", "--help"},
		{"skill", "--help"},
	}
	for _, args := range commands {
		T.Run(args[0], func(t *testing.T) {
			t.Parallel()
			rootCmd := cmd.NewRootCmd()
			rootCmd.SetArgs(args)
			var out bytes.Buffer
			rootCmd.SetOut(&out)
			rootCmd.SetErr(&out)
			err := rootCmd.Execute()
			require.NoError(t, err, "Execute(%v)", args)
			assert.NotEmpty(t, out.String(), "expected help output for %v", args)
		})
	}
}

func TestUnknownCommand(T *testing.T) {
	T.Parallel()

	rootCmd := cmd.NewRootCmd()
	rootCmd.SetArgs([]string{"nonexistent"})
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	err := rootCmd.Execute()
	assert.Error(T, err, "expected error for unknown command")
}

func TestGlobalFlags(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	cmdtest.RunCmdWithFactory(T, f, "--quiet", "project", "list", "--limit", "1")
	cmdtest.RunCmdWithFactory(T, f, "--verbose", "project", "list", "--limit", "1")
	cmdtest.RunCmdWithFactory(T, f, "--no-color", "project", "list", "--limit", "1")
}

func TestUnknownSubcommand(T *testing.T) {
	T.Parallel()

	commands := [][]string{
		{"run", "invalid"},
		{"project", "invalid"},
		{"queue", "invalid"},
		{"job", "invalid"},
		{"agent", "invalid"},
		{"pool", "invalid"},
		{"auth", "invalid"},
		{"skill", "invalid"},
	}

	for _, args := range commands {
		T.Run(args[0], func(t *testing.T) {
			t.Parallel()
			cmdtest.RunCmdExpectErr(t, "unknown command", args...)
		})
	}
}

func TestParentCommandWithoutSubcommand(T *testing.T) {
	T.Parallel()

	commands := []string{"run", "project", "queue", "job", "agent", "pool", "auth", "skill"}

	for _, c := range commands {
		T.Run(c, func(t *testing.T) {
			t.Parallel()

			rootCmd := cmd.NewRootCmd()
			rootCmd.SetArgs([]string{c})
			var out bytes.Buffer
			rootCmd.SetOut(&out)
			rootCmd.SetErr(&out)
			err := rootCmd.Execute()
			assert.Error(t, err, "expected error for %s without subcommand", c)
			assert.Contains(t, out.String(), "requires a subcommand")
		})
	}
}

func TestInvalidIDs(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	f := ts.Factory

	cases := []struct {
		name string
		args []string
	}{
		{"project", []string{"project", "view", "NonExistentProject123456"}},
		{"job", []string{"job", "view", "NonExistentJob123456"}},
		{"run", []string{"run", "view", "999999999"}},
	}
	for _, tc := range cases {
		T.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rootCmd := cmd.NewRootCmdWithFactory(f)
			rootCmd.SetArgs(tc.args)
			var out bytes.Buffer
			rootCmd.SetOut(&out)
			rootCmd.SetErr(&out)
			err := rootCmd.Execute()
			assert.Error(t, err, "expected error for invalid %s ID", tc.name)
		})
	}
}
