package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupGitRepo creates a temporary git repository for testing: returns the path and a cleanup function.
func setupGitRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test User")

	return dir
}

// runGit runs a git command in the given directory.
func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v failed: %s", args, string(out))
	return string(out)
}

// chdir changes to the given directory and returns a function to restore the original.
func chdir(t *testing.T, dir string) func() {
	t.Helper()

	orig, err := os.Getwd()
	require.NoError(t, err)

	err = os.Chdir(dir)
	require.NoError(t, err)

	return func() {
		_ = os.Chdir(orig)
	}
}

func TestIsGitRepo(t *testing.T) {
	t.Run("in git repo", func(t *testing.T) {
		dir := setupGitRepo(t)
		restore := chdir(t, dir)
		defer restore()

		assert.True(t, isGitRepo())
	})

	t.Run("not in git repo", func(t *testing.T) {
		dir := t.TempDir()
		restore := chdir(t, dir)
		defer restore()

		assert.False(t, isGitRepo())
	})
}

func TestGetCurrentBranch(t *testing.T) {
	t.Run("on branch", func(t *testing.T) {
		dir := setupGitRepo(t)
		restore := chdir(t, dir)
		defer restore()

		// Create initial commit so we have a branch
		createFile(t, dir, "test.txt", "content")
		runGit(t, dir, "add", ".")
		runGit(t, dir, "commit", "-m", "initial")

		branch, err := getCurrentBranch()
		require.NoError(t, err)
		// Could be "main" or "master" depending on git config
		assert.Contains(t, []string{"main", "master"}, branch)
	})

	t.Run("on custom branch", func(t *testing.T) {
		dir := setupGitRepo(t)
		restore := chdir(t, dir)
		defer restore()

		// Create initial commit
		createFile(t, dir, "test.txt", "content")
		runGit(t, dir, "add", ".")
		runGit(t, dir, "commit", "-m", "initial")

		// Create and switch to new branch
		runGit(t, dir, "checkout", "-b", "feature/test-branch")

		branch, err := getCurrentBranch()
		require.NoError(t, err)
		assert.Equal(t, "feature/test-branch", branch)
	})

	t.Run("detached HEAD", func(t *testing.T) {
		dir := setupGitRepo(t)
		restore := chdir(t, dir)
		defer restore()

		// Create initial commit
		createFile(t, dir, "test.txt", "content")
		runGit(t, dir, "add", ".")
		runGit(t, dir, "commit", "-m", "initial")

		// Detach HEAD
		runGit(t, dir, "checkout", "--detach", "HEAD")

		_, err := getCurrentBranch()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "detached HEAD")
	})
}

func TestGetRemoteForBranch(t *testing.T) {
	t.Run("no remote configured", func(t *testing.T) {
		dir := setupGitRepo(t)
		restore := chdir(t, dir)
		defer restore()

		// Create initial commit
		createFile(t, dir, "test.txt", "content")
		runGit(t, dir, "add", ".")
		runGit(t, dir, "commit", "-m", "initial")

		// Should default to "origin"
		remote := getRemoteForBranch("main")
		assert.Equal(t, "origin", remote)
	})

	t.Run("with remote configured", func(t *testing.T) {
		dir := setupGitRepo(t)
		restore := chdir(t, dir)
		defer restore()

		// Create initial commit
		createFile(t, dir, "test.txt", "content")
		runGit(t, dir, "add", ".")
		runGit(t, dir, "commit", "-m", "initial")

		// Add a remote and set upstream
		runGit(t, dir, "remote", "add", "upstream", "https://example.com/repo.git")
		runGit(t, dir, "config", "branch.master.remote", "upstream")

		remote := getRemoteForBranch("master")
		assert.Equal(t, "upstream", remote)
	})
}

func TestGetUntrackedFiles(t *testing.T) {
	t.Run("no untracked files", func(t *testing.T) {
		dir := setupGitRepo(t)
		restore := chdir(t, dir)
		defer restore()

		files, err := getUntrackedFiles()
		require.NoError(t, err)
		assert.Empty(t, files)
	})

	t.Run("with untracked files", func(t *testing.T) {
		dir := setupGitRepo(t)
		restore := chdir(t, dir)
		defer restore()

		// Create untracked files
		createFile(t, dir, "untracked1.txt", "content1")
		createFile(t, dir, "untracked2.txt", "content2")

		files, err := getUntrackedFiles()
		require.NoError(t, err)
		assert.Len(t, files, 2)
		assert.Contains(t, files, "untracked1.txt")
		assert.Contains(t, files, "untracked2.txt")
	})

	t.Run("ignores gitignored files", func(t *testing.T) {
		dir := setupGitRepo(t)
		restore := chdir(t, dir)
		defer restore()

		// Create initial commit with .gitignore that ignores *.log files
		createFile(t, dir, "ignore_rules", "*.log")
		runGit(t, dir, "add", "ignore_rules")
		runGit(t, dir, "commit", "-m", "initial")
		// Rename to .gitignore after commit to avoid global gitignore issues
		runGit(t, dir, "mv", "ignore_rules", ".gitignore")
		runGit(t, dir, "commit", "-m", "rename to gitignore")

		// Create files - one normal, one matching ignore pattern
		createFile(t, dir, "tracked.txt", "content")
		createFile(t, dir, "debug.log", "should be ignored")

		files, err := getUntrackedFiles()
		require.NoError(t, err)
		assert.Contains(t, files, "tracked.txt")
		assert.NotContains(t, files, "debug.log") // ignored by .gitignore
	})
}

func TestGetGitDiff(t *testing.T) {
	t.Run("no changes", func(t *testing.T) {
		dir := setupGitRepo(t)
		restore := chdir(t, dir)
		defer restore()

		// Create initial commit
		createFile(t, dir, "test.txt", "content")
		runGit(t, dir, "add", ".")
		runGit(t, dir, "commit", "-m", "initial")

		diff, err := getGitDiff()
		require.NoError(t, err)
		assert.Empty(t, diff)
	})

	t.Run("with staged changes", func(t *testing.T) {
		dir := setupGitRepo(t)
		restore := chdir(t, dir)
		defer restore()

		// Create initial commit
		createFile(t, dir, "test.txt", "content")
		runGit(t, dir, "add", ".")
		runGit(t, dir, "commit", "-m", "initial")

		// Make and stage changes
		createFile(t, dir, "test.txt", "modified content")
		runGit(t, dir, "add", "test.txt")

		diff, err := getGitDiff()
		require.NoError(t, err)
		assert.Contains(t, string(diff), "modified content")
	})

	t.Run("with unstaged changes", func(t *testing.T) {
		dir := setupGitRepo(t)
		restore := chdir(t, dir)
		defer restore()

		// Create initial commit
		createFile(t, dir, "test.txt", "content")
		runGit(t, dir, "add", ".")
		runGit(t, dir, "commit", "-m", "initial")

		// Make unstaged changes
		createFile(t, dir, "test.txt", "unstaged changes")

		diff, err := getGitDiff()
		require.NoError(t, err)
		assert.Contains(t, string(diff), "unstaged changes")
	})

	t.Run("includes untracked files", func(t *testing.T) {
		dir := setupGitRepo(t)
		restore := chdir(t, dir)
		defer restore()

		// Create initial commit
		createFile(t, dir, "test.txt", "content")
		runGit(t, dir, "add", ".")
		runGit(t, dir, "commit", "-m", "initial")

		// Create untracked file
		createFile(t, dir, "new_file.txt", "new file content")

		diff, err := getGitDiff()
		require.NoError(t, err)
		assert.Contains(t, string(diff), "new_file.txt")
		assert.Contains(t, string(diff), "new file content")

		// Verify untracked file was reset (not staged)
		files, err := getUntrackedFiles()
		require.NoError(t, err)
		assert.Contains(t, files, "new_file.txt")
	})
}

func TestLoadLocalChanges(t *testing.T) {
	t.Run("git source with changes", func(t *testing.T) {
		dir := setupGitRepo(t)
		restore := chdir(t, dir)
		defer restore()

		// Create initial commit
		createFile(t, dir, "test.txt", "content")
		runGit(t, dir, "add", ".")
		runGit(t, dir, "commit", "-m", "initial")

		// Make changes
		createFile(t, dir, "test.txt", "modified")

		patch, err := loadLocalChanges("git")
		require.NoError(t, err)
		assert.Contains(t, string(patch), "modified")
	})

	t.Run("git source no changes", func(t *testing.T) {
		dir := setupGitRepo(t)
		restore := chdir(t, dir)
		defer restore()

		// Create initial commit
		createFile(t, dir, "test.txt", "content")
		runGit(t, dir, "add", ".")
		runGit(t, dir, "commit", "-m", "initial")

		_, err := loadLocalChanges("git")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no uncommitted changes")
	})

	t.Run("git source not in repo", func(t *testing.T) {
		dir := t.TempDir()
		restore := chdir(t, dir)
		defer restore()

		_, err := loadLocalChanges("git")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not a git repository")
	})

	t.Run("file source", func(t *testing.T) {
		t.Parallel()
		patchFile := filepath.Join(t.TempDir(), "changes.patch")
		require.NoError(t, os.WriteFile(patchFile, []byte("diff content"), 0644))

		patch, err := loadLocalChanges(patchFile)
		require.NoError(t, err)
		assert.Equal(t, "diff content", string(patch))
	})

	t.Run("file source not found", func(t *testing.T) {
		t.Parallel()
		_, err := loadLocalChanges("/nonexistent/path.patch")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("file source empty", func(t *testing.T) {
		t.Parallel()
		patchFile := filepath.Join(t.TempDir(), "empty.patch")
		require.NoError(t, os.WriteFile(patchFile, []byte{}, 0644))

		_, err := loadLocalChanges(patchFile)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "empty")
	})
}

// createFile creates a file with the given content.
func createFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err)
}
