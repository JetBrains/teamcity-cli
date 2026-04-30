package cmdutil

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
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

func TestRequireNonEmpty(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in      string
		wantErr bool
	}{
		{"hello", false},
		{"  hello  ", false}, // trim leaves content
		{"", true},
		{"   ", true}, // whitespace-only rejected
		{"\t\n", true},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			err := RequireNonEmpty(tc.in)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// fakeAgentClient stubs the agent-resolution surface only.
type fakeAgentClient struct {
	api.ClientInterface
	byID   map[int]*api.Agent
	byName map[string]*api.Agent
}

func (c *fakeAgentClient) GetAgent(id int) (*api.Agent, error) {
	if a, ok := c.byID[id]; ok {
		return a, nil
	}
	return nil, errors.New("not found")
}

func (c *fakeAgentClient) GetAgentByName(name string) (*api.Agent, error) {
	if a, ok := c.byName[name]; ok {
		return a, nil
	}
	return nil, errors.New("not found")
}

func TestResolveAgent(t *testing.T) {
	t.Parallel()
	client := &fakeAgentClient{
		byID:   map[int]*api.Agent{42: {ID: 42, Name: "linux-01"}},
		byName: map[string]*api.Agent{"linux-01": {ID: 42, Name: "linux-01"}},
	}

	t.Run("numeric → GetAgent", func(t *testing.T) {
		t.Parallel()
		a, err := ResolveAgent(client, "42")
		require.NoError(t, err)
		assert.Equal(t, 42, a.ID)
	})
	t.Run("name → GetAgentByName", func(t *testing.T) {
		t.Parallel()
		a, err := ResolveAgent(client, "linux-01")
		require.NoError(t, err)
		assert.Equal(t, "linux-01", a.Name)
	})
	t.Run("ResolveAgentID returns id+name pair", func(t *testing.T) {
		t.Parallel()
		id, name, err := ResolveAgentID(client, "linux-01")
		require.NoError(t, err)
		assert.Equal(t, 42, id)
		assert.Equal(t, "linux-01", name)
	})
	t.Run("unknown name → error", func(t *testing.T) {
		t.Parallel()
		_, err := ResolveAgent(client, "ghost")
		assert.Error(t, err)
	})
	t.Run("ResolveAgentID propagates error", func(t *testing.T) {
		t.Parallel()
		_, _, err := ResolveAgentID(client, "ghost")
		assert.Error(t, err)
	})
}

func TestProbeGuestAccess(t *testing.T) {
	t.Parallel()

	t.Run("empty url → false (no network call)", func(t *testing.T) {
		t.Parallel()
		assert.False(t, ProbeGuestAccess(context.Background(), ""))
	})

	t.Run("200 → true", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			// Minimal valid /app/rest/server JSON so GetServer decodes.
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"2025.7","versionMajor":2025,"versionMinor":7,"buildNumber":"1"}`))
		}))
		defer srv.Close()
		assert.True(t, ProbeGuestAccess(context.Background(), srv.URL))
	})

	t.Run("401 → false", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer srv.Close()
		assert.False(t, ProbeGuestAccess(context.Background(), srv.URL))
	})

	t.Run("dead address → false", func(t *testing.T) {
		t.Parallel()
		// Closed port; ProbeGuestAccess must return false without panicking.
		assert.False(t, ProbeGuestAccess(context.Background(), "http://127.0.0.1:1"))
	})
}

func TestNotAuthenticatedError(t *testing.T) {
	t.Parallel()

	t.Run("no keyring err", func(t *testing.T) {
		t.Parallel()
		err := NotAuthenticatedError(context.Background(), "", nil)
		require.NotNil(t, err)
		assert.Equal(t, "Not authenticated", err.Error())
		assert.Contains(t, err.Suggestion(), "TEAMCITY_URL")
	})

	t.Run("with keyring err mentions it", func(t *testing.T) {
		t.Parallel()
		ke := errors.New("keychain locked")
		err := NotAuthenticatedError(context.Background(), "", ke)
		require.NotNil(t, err)
		assert.Contains(t, err.Error(), "keyring")
		assert.Contains(t, err.Error(), "keychain locked")
	})

	t.Run("guest-capable server adds guest hint", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"2025.7","versionMajor":2025,"versionMinor":7,"buildNumber":"1"}`))
		}))
		defer srv.Close()

		err := NotAuthenticatedError(context.Background(), srv.URL, nil)
		require.NotNil(t, err)
		assert.Contains(t, err.Suggestion(), "TEAMCITY_GUEST=1")
	})
}

func TestDeprecateCommand(t *testing.T) {
	t.Parallel()

	cmd := &cobra.Command{Use: "old"}
	DeprecateCommand(cmd, "new", "v2.0.0")

	assert.NotEmpty(t, cmd.Deprecated)
	assert.Contains(t, cmd.Deprecated, "new")
	assert.Contains(t, cmd.Deprecated, "v2.0.0")
}

func TestDeprecateFlag(t *testing.T) {
	t.Parallel()

	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().Bool("old", false, "old flag")
	DeprecateFlag(cmd, "old", "new", "v2.0.0")

	flag := cmd.Flag("old")
	require.NotNil(t, flag)
	assert.NotEmpty(t, flag.Deprecated, "flag should carry deprecation note")
	assert.Contains(t, flag.Deprecated, "new")
	assert.Contains(t, flag.Deprecated, "v2.0.0")
}
