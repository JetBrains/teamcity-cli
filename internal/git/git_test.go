package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupRepo(t *testing.T) string {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test User")
	runGit(t, dir, "config", "commit.gpgsign", "false")
	return dir
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v failed: %s", args, string(out))
	return string(out)
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0644))
}

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

func TestIsRepo(t *testing.T) {
	t.Run("inside repo", func(t *testing.T) {
		t.Chdir(setupRepo(t))
		assert.True(t, IsRepo())
	})
	t.Run("outside repo", func(t *testing.T) {
		t.Chdir(t.TempDir())
		assert.False(t, IsRepo())
	})
}

func TestCurrentBranch(t *testing.T) {
	t.Run("on default branch", func(t *testing.T) {
		dir := setupRepo(t)
		t.Chdir(dir)
		writeFile(t, dir, "test.txt", "content")
		runGit(t, dir, "add", ".")
		runGit(t, dir, "commit", "-m", "initial")

		branch, err := CurrentBranch()
		require.NoError(t, err)
		assert.Contains(t, []string{"main", "master"}, branch)
	})

	t.Run("on feature branch", func(t *testing.T) {
		dir := setupRepo(t)
		t.Chdir(dir)
		writeFile(t, dir, "test.txt", "content")
		runGit(t, dir, "add", ".")
		runGit(t, dir, "commit", "-m", "initial")
		runGit(t, dir, "checkout", "-b", "feature/test-branch")

		branch, err := CurrentBranch()
		require.NoError(t, err)
		assert.Equal(t, "feature/test-branch", branch)
	})

	t.Run("detached HEAD returns ErrDetachedHEAD", func(t *testing.T) {
		dir := setupRepo(t)
		t.Chdir(dir)
		writeFile(t, dir, "test.txt", "content")
		runGit(t, dir, "add", ".")
		runGit(t, dir, "commit", "-m", "initial")
		runGit(t, dir, "checkout", "--detach", "HEAD")

		_, err := CurrentBranch()
		assert.ErrorIs(t, err, ErrDetachedHEAD)
	})
}

func TestRemoteForBranch(t *testing.T) {
	t.Run("default origin", func(t *testing.T) {
		dir := setupRepo(t)
		t.Chdir(dir)
		writeFile(t, dir, "test.txt", "content")
		runGit(t, dir, "add", ".")
		runGit(t, dir, "commit", "-m", "initial")

		assert.Equal(t, "origin", RemoteForBranch("main"))
	})

	t.Run("configured upstream", func(t *testing.T) {
		dir := setupRepo(t)
		t.Chdir(dir)
		writeFile(t, dir, "test.txt", "content")
		runGit(t, dir, "add", ".")
		runGit(t, dir, "commit", "-m", "initial")
		runGit(t, dir, "remote", "add", "upstream", "https://example.com/repo.git")
		runGit(t, dir, "config", "branch.master.remote", "upstream")

		assert.Equal(t, "upstream", RemoteForBranch("master"))
	})
}

func TestBranchExistsOnRemote(t *testing.T) {
	dir := setupRepo(t)
	t.Chdir(dir)
	writeFile(t, dir, "test.txt", "content")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "initial")

	assert.False(t, BranchExistsOnRemote("master"))
}

func TestUntrackedFiles(t *testing.T) {
	t.Run("none", func(t *testing.T) {
		t.Chdir(setupRepo(t))
		files, err := UntrackedFiles()
		require.NoError(t, err)
		assert.Empty(t, files)
	})

	t.Run("two files", func(t *testing.T) {
		dir := setupRepo(t)
		t.Chdir(dir)
		writeFile(t, dir, "a.txt", "1")
		writeFile(t, dir, "b.txt", "2")

		files, err := UntrackedFiles()
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"a.txt", "b.txt"}, files)
	})

	t.Run("respects gitignore", func(t *testing.T) {
		dir := setupRepo(t)
		t.Chdir(dir)
		writeFile(t, dir, ".gitignore", "*.log\n")
		runGit(t, dir, "add", ".gitignore")
		runGit(t, dir, "commit", "-m", "ignore")
		writeFile(t, dir, "tracked.txt", "x")
		writeFile(t, dir, "debug.log", "y")

		files, err := UntrackedFiles()
		require.NoError(t, err)
		assert.Contains(t, files, "tracked.txt")
		assert.NotContains(t, files, "debug.log")
	})
}

func TestUncommittedDiff(t *testing.T) {
	t.Run("no changes", func(t *testing.T) {
		dir := setupRepo(t)
		t.Chdir(dir)
		writeFile(t, dir, "test.txt", "content")
		runGit(t, dir, "add", ".")
		runGit(t, dir, "commit", "-m", "initial")

		diff, err := UncommittedDiff()
		require.NoError(t, err)
		assert.Empty(t, diff)
	})

	t.Run("modified file", func(t *testing.T) {
		dir := setupRepo(t)
		t.Chdir(dir)
		writeFile(t, dir, "test.txt", "content")
		runGit(t, dir, "add", ".")
		runGit(t, dir, "commit", "-m", "initial")
		writeFile(t, dir, "test.txt", "modified")

		diff, err := UncommittedDiff()
		require.NoError(t, err)
		assert.Contains(t, string(diff), "modified")
	})

	t.Run("includes untracked", func(t *testing.T) {
		dir := setupRepo(t)
		t.Chdir(dir)
		writeFile(t, dir, "seed.txt", "seed")
		runGit(t, dir, "add", ".")
		runGit(t, dir, "commit", "-m", "initial")
		writeFile(t, dir, "new.txt", "fresh content")

		diff, err := UncommittedDiff()
		require.NoError(t, err)
		assert.Contains(t, string(diff), "new.txt")
		assert.Contains(t, string(diff), "fresh content")
	})
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
