package git

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepoRoot(t *testing.T) {
	outer := t.TempDir()
	repo := filepath.Join(outer, "repo")
	deep := filepath.Join(repo, "a", "b")
	require.NoError(t, os.MkdirAll(deep, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(repo, ".git"), 0o755))

	got, ok := RepoRoot(deep)
	require.True(t, ok)
	want, _ := filepath.EvalSymlinks(repo)
	gotResolved, _ := filepath.EvalSymlinks(got)
	assert.Equal(t, want, gotResolved)

	_, ok = RepoRoot(outer)
	assert.False(t, ok, "RepoRoot must return false outside a git worktree")
}

func TestCanonicalURL(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"git@github.com:acme/backend.git", "github.com/acme/backend"},
		{"git@github.com:acme/backend", "github.com/acme/backend"},
		{"https://github.com/acme/backend.git", "github.com/acme/backend"},
		{"https://github.com/acme/backend", "github.com/acme/backend"},
		{"https://user@github.com/acme/backend.git", "github.com/acme/backend"},
		{"https://GITHUB.com/Acme/Backend.git", "github.com/Acme/Backend"},
		{"ssh://git@github.com/acme/backend.git", "github.com/acme/backend"},
		{"ssh://git@github.com:22/acme/backend.git", "github.com/acme/backend"},
		{"https://github.com/acme/backend/", "github.com/acme/backend"},
		{"  https://github.com/acme/backend.git  ", "github.com/acme/backend"},
		{"", ""},
		{"not-a-url", ""},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			assert.Equal(t, c.want, CanonicalURL(c.in))
		})
	}
}

func TestRepoPath(t *testing.T) {
	assert.Equal(t, "acme/backend", RepoPath("git@github.com:acme/backend.git"))
	assert.Equal(t, "acme/backend", RepoPath("https://github.com/acme/backend.git"))
	assert.Equal(t, "acme/platform/api", RepoPath("https://gitlab.com/acme/platform/api.git"))
	assert.Equal(t, "", RepoPath(""))
	assert.Equal(t, "", RepoPath("not-a-url"))
}
