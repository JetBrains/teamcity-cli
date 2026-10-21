package onboard

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// addRemote registers a git remote on the repository at dir.
func addRemote(t *testing.T, dir, name, url string) {
	t.Helper()
	cmd := exec.Command("git", "remote", "add", name, url)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git remote add: %s", out)
}

// samePath compares paths after resolving symlinks (macOS temp dirs live under /private).
func samePath(t *testing.T, want, got string) {
	t.Helper()
	w, err := filepath.EvalSymlinks(want)
	require.NoError(t, err)
	g, err := filepath.EvalSymlinks(got)
	require.NoError(t, err)
	assert.Equal(t, w, g)
}

func TestNormalizePath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	base := t.TempDir()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"absolute", base, base},
		{"surrounding whitespace", "  " + base + "\t", base},
		{"double quotes from drag-and-drop", `"` + base + `"`, base},
		{"single quotes", "'" + base + "'", base},
		{"spaces inside path", filepath.Join(base, "my repo"), filepath.Join(base, "my repo")},
		{"tilde", "~", home},
		{"tilde subdir", "~/projects", filepath.Join(home, "projects")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizePath(tc.input)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}

	t.Run("relative resolves against cwd", func(t *testing.T) {
		t.Chdir(base)
		got, err := normalizePath(".")
		require.NoError(t, err)
		samePath(t, base, got)
	})

	for name, input := range map[string]string{
		"empty":            "",
		"whitespace only":  "   ",
		"only quotes":      `""`,
		"NUL byte":         base + "\x00",
		"escape sequence":  base + "\x1b[A",
		"embedded newline": base + "\nrm",
	} {
		t.Run("rejects "+name, func(t *testing.T) {
			_, err := normalizePath(input)
			assert.Error(t, err)
		})
	}
}

func TestValidateRepoPath(t *testing.T) {
	repo := initRepoWithCommit(t, "main")
	sub := filepath.Join(repo, "a", "b")
	require.NoError(t, os.MkdirAll(sub, 0o755))
	file := filepath.Join(repo, "README.md")
	require.NoError(t, os.WriteFile(file, []byte("hi"), 0o644))

	assert.NoError(t, validateRepoPath(repo))
	assert.NoError(t, validateRepoPath(sub), "subdirectory of a repo is accepted")

	assert.ErrorContains(t, validateRepoPath(filepath.Join(repo, "missing")), "does not exist")
	assert.ErrorContains(t, validateRepoPath(file), "is not a directory")
	assert.ErrorContains(t, validateRepoPath(t.TempDir()), "is not inside a git repository")
	assert.ErrorContains(t, validateRepoPath(""), "path is required")
}

func TestResolveRepository(t *testing.T) {
	t.Run("origin preferred", func(t *testing.T) {
		repo := initRepoWithCommit(t, "main")
		addRemote(t, repo, "upstream", "https://example.com/upstream.git")
		addRemote(t, repo, "origin", "git@github.com:acme/app.git")

		got, err := resolveRepository(sourcePath, repo)
		require.NoError(t, err)
		assert.Equal(t, sourcePath, got.source)
		samePath(t, repo, got.dir)
		assert.Equal(t, "git@github.com:acme/app.git", got.remoteURL)
	})

	t.Run("subdirectory resolves to root", func(t *testing.T) {
		repo := initRepoWithCommit(t, "main")
		addRemote(t, repo, "origin", "https://example.com/app.git")
		sub := filepath.Join(repo, "services", "api")
		require.NoError(t, os.MkdirAll(sub, 0o755))

		got, err := resolveRepository(sourcePath, sub)
		require.NoError(t, err)
		samePath(t, repo, got.dir)
		assert.Equal(t, "https://example.com/app.git", got.remoteURL)
	})

	t.Run("no remotes", func(t *testing.T) {
		repo := initRepoWithCommit(t, "main")

		got, err := resolveRepository(sourceCurrent, repo)
		require.NoError(t, err)
		samePath(t, repo, got.dir)
		assert.Empty(t, got.remoteURL)
	})

	t.Run("current directory", func(t *testing.T) {
		repo := initRepoWithCommit(t, "main")
		addRemote(t, repo, "origin", "https://example.com/app.git")
		t.Chdir(repo)

		got, err := resolveRepository(sourceCurrent, ".")
		require.NoError(t, err)
		assert.Equal(t, sourceCurrent, got.source)
		samePath(t, repo, got.dir)
	})

	t.Run("not a repo", func(t *testing.T) {
		_, err := resolveRepository(sourcePath, t.TempDir())
		_, ok := errors.AsType[*api.ValidationError](err)
		assert.True(t, ok, "want *api.ValidationError, got %T: %v", err, err)
	})
}

func TestDemoReposAreValid(t *testing.T) {
	require.NotEmpty(t, demoRepos)
	for _, d := range demoRepos {
		assert.NotEmpty(t, d.name)
		assert.NotEmpty(t, git.CanonicalURL(d.url), "demo URL %q must be a parseable remote", d.url)
	}
}
