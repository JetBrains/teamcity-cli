package cmdutil

import (
	"testing"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateLimit(t *testing.T) {
	assert.NoError(t, ValidateLimit(1))
	assert.NoError(t, ValidateLimit(100))
	assert.Error(t, ValidateLimit(0))
	assert.Error(t, ValidateLimit(-1))
	assert.Contains(t, ValidateLimit(-5).Error(), "--limit must be a positive number")
}

func TestParseID(t *testing.T) {
	id, err := ParseID("42", "build")
	require.NoError(t, err)
	assert.Equal(t, 42, id)

	_, err = ParseID("abc", "build")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid build ID: abc")

	_, err = ParseID("", "agent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid agent ID")
}

func TestFormatAgentStatus(t *testing.T) {
	tests := []struct {
		name  string
		agent api.Agent
		want  string
	}{
		{"unauthorized", api.Agent{Authorized: false}, "Unauthorized"},
		{"disabled", api.Agent{Authorized: true, Enabled: false}, "Disabled"},
		{"disconnected", api.Agent{Authorized: true, Enabled: true, Connected: false}, "Disconnected"},
		{"connected", api.Agent{Authorized: true, Enabled: true, Connected: true}, "Connected"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatAgentStatus(tt.agent)
			assert.Contains(t, result, tt.want)
		})
	}
}

func TestAddViewFlags(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	opts := &ViewOptions{}
	AddViewFlags(cmd, opts)

	assert.NotNil(t, cmd.Flags().Lookup("json"))
	assert.NotNil(t, cmd.Flags().Lookup("web"))
}

func TestSubcommandRequired(t *testing.T) {
	cmd := &cobra.Command{Use: "parent"}
	err := SubcommandRequired(cmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires a subcommand")
}

func TestExitError(t *testing.T) {
	err := &ExitError{Code: ExitFailure}
	assert.Equal(t, "exit status 1", err.Error())

	err2 := &ExitError{Code: ExitCancelled}
	assert.Equal(t, "exit status 2", err2.Error())
}

func TestNewFactory(t *testing.T) {
	f := NewFactory()
	assert.NotNil(t, f.IOStreams)
	assert.NotNil(t, f.IOStreams.In)
	assert.NotNil(t, f.IOStreams.Out)
	assert.NotNil(t, f.IOStreams.ErrOut)
	assert.NotNil(t, f.Printer)
	assert.NotNil(t, f.ClientFunc)
	assert.False(t, f.NoColor)
	assert.False(t, f.Quiet)
	assert.False(t, f.Verbose)
	assert.False(t, f.NoInput)
}

func TestFactoryClient(t *testing.T) {
	called := false
	f := &Factory{
		ClientFunc: func() (api.ClientInterface, error) {
			called = true
			return nil, nil
		},
	}
	_, _ = f.Client()
	assert.True(t, called)
}

func TestWarnInsecureHTTP(t *testing.T) {
	f := NewFactory()
	// Should not panic with HTTPS
	f.WarnInsecureHTTP("https://tc.example.com", "token")

	// Should not panic with env var set
	t.Setenv("TC_INSECURE_SKIP_WARN", "1")
	f.WarnInsecureHTTP("http://tc.example.com", "token")
}
