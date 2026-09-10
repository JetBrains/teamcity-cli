package run

import (
	"testing"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRevisionFlags(t *testing.T) {
	old := isGitRepoFn
	isGitRepoFn = func() bool { return false }
	t.Cleanup(func() { isGitRepoFn = old })
	for _, tt := range []struct {
		name   string
		values []string
		bare   string
		specs  []api.RevisionSpec
		err    string
	}{
		{name: "empty", specs: []api.RevisionSpec{}},
		{name: "bare", values: []string{"abc123"}, bare: "abc123", specs: []api.RevisionSpec{}},
		{name: "pins", values: []string{"A=abc@feature/x", "B=@release@2026"}, specs: []api.RevisionSpec{{VcsRootID: "A", Version: "abc", Branch: "feature/x"}, {VcsRootID: "B", Branch: "release@2026"}}},
		{name: "head requires checkout", values: []string{"@head"}, err: "requires a git repository"},
		{name: "bare first", values: []string{"abc", "A=def"}, err: "cannot mix"},
		{name: "bare last", values: []string{"A=abc", "def"}, err: "cannot mix"},
		{name: "two bare", values: []string{"abc", "def"}, err: "cannot mix"},
		{name: "duplicate", values: []string{"A=abc", "A=def"}, err: "duplicate"},
		{name: "missing root", values: []string{"=abc"}, err: "invalid"},
		{name: "missing value", values: []string{"A="}, err: "invalid"},
		{name: "empty branch", values: []string{"A=abc@"}, err: "invalid"},
		{name: "empty branch only", values: []string{"A=@"}, err: "invalid"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			bare, specs, err := parseRevisionFlags(tt.values)
			if tt.err != "" {
				require.ErrorContains(t, err, tt.err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.bare, bare)
			assert.Equal(t, tt.specs, specs)
		})
	}
}

func TestRevisionFlagsLocalGitResolution(t *testing.T) {
	oldRepo, oldHead, oldResolve := isGitRepoFn, headRevisionFn, resolveRevisionFn
	t.Cleanup(func() { isGitRepoFn, headRevisionFn, resolveRevisionFn = oldRepo, oldHead, oldResolve })
	isGitRepoFn = func() bool { return true }
	headRevisionFn = func() (string, error) { return "local-head", nil }
	resolveRevisionFn = func(rev string) (string, error) { return "expanded-" + rev, nil }

	bare, _, err := parseRevisionFlags([]string{"@head"})
	require.NoError(t, err)
	assert.Equal(t, "local-head", bare)
	bare, _, err = parseRevisionFlags([]string{"abc"})
	require.NoError(t, err)
	assert.Equal(t, "expanded-abc", bare)
	_, specs, err := parseRevisionFlags([]string{"OtherRepo=abc"})
	require.NoError(t, err)
	assert.Equal(t, "abc", specs[0].Version)
}
