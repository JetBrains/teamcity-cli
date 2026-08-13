package run

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JetBrains/teamcity-cli/api"
)

func stubNoGit(t *testing.T) {
	t.Helper()
	old := isGitRepoFn
	isGitRepoFn = func() bool { return false }
	t.Cleanup(func() { isGitRepoFn = old })
}

func TestParseRevisionFlags(t *testing.T) {
	stubNoGit(t)

	tests := []struct {
		name string
		in   []string
		want []api.RevisionSpec
	}{
		{"empty", nil, []api.RevisionSpec{}},
		{"bare sha", []string{"abc123def"}, []api.RevisionSpec{{Version: "abc123def"}}},
		{
			"keyed sha and branch",
			[]string{"Repo1=abc123@feature/x", "Repo2=def456"},
			[]api.RevisionSpec{
				{VcsRootID: "Repo1", Version: "abc123", Branch: "feature/x"},
				{VcsRootID: "Repo2", Version: "def456"},
			},
		},
		{"branch only", []string{"Repo1=@feature/x"}, []api.RevisionSpec{{VcsRootID: "Repo1", Branch: "feature/x"}}},
		{"branch with @ inside", []string{"Repo1=@release@2024"}, []api.RevisionSpec{{VcsRootID: "Repo1", Branch: "release@2024"}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseRevisionFlags(tc.in)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestParseRevisionFlagsErrors(t *testing.T) {
	stubNoGit(t)

	tests := []struct {
		name string
		in   []string
		want string
	}{
		{"mixed bare and keyed", []string{"abc123", "Repo1=def456"}, "cannot mix"},
		{"two bare", []string{"abc123", "def456"}, "only one bare"},
		{"duplicate root", []string{"Repo1=abc", "Repo1=def"}, "duplicate"},
		{"empty value", []string{"Repo1="}, "invalid --revision"},
		{"empty root", []string{"=abc123"}, "invalid --revision"},
		{"empty branch", []string{"Repo1=abc@"}, "invalid --revision"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseRevisionFlags(tc.in)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestFormatRevisionSpec(t *testing.T) {
	assert.Equal(t, "Repo1=abc123@feature/x", formatRevisionSpec(api.RevisionSpec{VcsRootID: "Repo1", Version: "abc123", Branch: "feature/x"}))
	assert.Equal(t, "Repo1=abc123", formatRevisionSpec(api.RevisionSpec{VcsRootID: "Repo1", Version: "abc123"}))
	assert.Equal(t, "Repo1=@feature/x", formatRevisionSpec(api.RevisionSpec{VcsRootID: "Repo1", Branch: "feature/x"}))
}
