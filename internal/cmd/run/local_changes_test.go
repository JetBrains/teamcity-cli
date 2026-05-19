package run

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init"},
		{"config", "user.email", "test@test.com"},
		{"config", "user.name", "Test User"},
		{"config", "commit.gpgsign", "false"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, string(out))
	}
	return dir
}

func gitDo(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, string(out))
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0644))
}

func TestLoadLocalChanges(t *testing.T) {
	t.Run("git source with changes", func(t *testing.T) {
		dir := setupRepo(t)
		t.Chdir(dir)
		writeFile(t, dir, "test.txt", "content")
		gitDo(t, dir, "add", ".")
		gitDo(t, dir, "commit", "-m", "initial")
		writeFile(t, dir, "test.txt", "modified")

		patch, err := loadLocalChanges("git", nil)
		require.NoError(t, err)
		assert.Contains(t, string(patch), "modified")
	})

	t.Run("git source no changes", func(t *testing.T) {
		dir := setupRepo(t)
		t.Chdir(dir)
		writeFile(t, dir, "test.txt", "content")
		gitDo(t, dir, "add", ".")
		gitDo(t, dir, "commit", "-m", "initial")

		_, err := loadLocalChanges("git", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no local changes found")
	})

	t.Run("git source committed but not pushed", func(t *testing.T) {
		remoteDir := t.TempDir()
		cmd := exec.Command("git", "init", "--bare", remoteDir)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git init --bare: %s", out)

		localDir := setupRepo(t)
		gitDo(t, localDir, "remote", "add", "origin", remoteDir)
		writeFile(t, localDir, "test.txt", "content")
		gitDo(t, localDir, "add", ".")
		gitDo(t, localDir, "commit", "-m", "initial")

		branchOut, err := exec.Command("git", "-C", localDir, "symbolic-ref", "--short", "HEAD").Output()
		require.NoError(t, err)
		branch := strings.TrimSpace(string(branchOut))
		gitDo(t, localDir, "push", "-u", "origin", branch)

		writeFile(t, localDir, "test.txt", "modified")
		gitDo(t, localDir, "add", ".")
		gitDo(t, localDir, "commit", "-m", "unpushed change")

		t.Chdir(localDir)
		patch, err := loadLocalChanges("git", nil)
		require.NoError(t, err)
		assert.Contains(t, string(patch), "modified")
	})

	t.Run("git source behind upstream clean tree after fetch", func(t *testing.T) {
		remoteDir := t.TempDir()
		cmd := exec.Command("git", "init", "--bare", remoteDir)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git init --bare: %s", out)

		localDir := setupRepo(t)
		gitDo(t, localDir, "remote", "add", "origin", remoteDir)
		writeFile(t, localDir, "test.txt", "content")
		gitDo(t, localDir, "add", ".")
		gitDo(t, localDir, "commit", "-m", "initial")

		branchOut, err := exec.Command("git", "-C", localDir, "symbolic-ref", "--short", "HEAD").Output()
		require.NoError(t, err)
		branch := strings.TrimSpace(string(branchOut))
		gitDo(t, localDir, "push", "-u", "origin", branch)

		otherDir := t.TempDir()
		out, err = exec.Command("git", "clone", remoteDir, otherDir).CombinedOutput()
		require.NoError(t, err, "git clone: %s", string(out))
		writeFile(t, otherDir, "from_remote.txt", "added on remote")
		gitDo(t, otherDir, "add", ".")
		gitDo(t, otherDir, "commit", "-m", "remote ahead commit")
		gitDo(t, otherDir, "push", "origin", branch)

		gitDo(t, localDir, "fetch", "origin")
		diffOut, err := exec.Command("git", "-C", localDir, "diff", "@{u}").CombinedOutput()
		require.NoError(t, err)
		require.NotEmpty(t, diffOut, "sanity: git diff @{u} must be non-empty when behind upstream (reverse patch)")

		t.Chdir(localDir)
		_, err = loadLocalChanges("git", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no local changes found")
	})

	t.Run("git source behind remote with local working tree changes", func(t *testing.T) {
		remoteDir := t.TempDir()
		cmd := exec.Command("git", "init", "--bare", remoteDir)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git init --bare: %s", out)

		localDir := setupRepo(t)
		gitDo(t, localDir, "remote", "add", "origin", remoteDir)
		writeFile(t, localDir, "test.txt", "content")
		gitDo(t, localDir, "add", ".")
		gitDo(t, localDir, "commit", "-m", "initial")

		branchOut, err := exec.Command("git", "-C", localDir, "symbolic-ref", "--short", "HEAD").Output()
		require.NoError(t, err)
		branch := strings.TrimSpace(string(branchOut))
		gitDo(t, localDir, "push", "-u", "origin", branch)

		otherDir := t.TempDir()
		out, err = exec.Command("git", "clone", remoteDir, otherDir).CombinedOutput()
		require.NoError(t, err, "git clone: %s", string(out))
		writeFile(t, otherDir, "from_remote.txt", "added on remote")
		gitDo(t, otherDir, "add", ".")
		gitDo(t, otherDir, "commit", "-m", "remote ahead commit")
		gitDo(t, otherDir, "push", "origin", branch)

		gitDo(t, localDir, "fetch", "origin")
		writeFile(t, localDir, "test.txt", "local working tree edit")

		t.Chdir(localDir)
		patch, err := loadLocalChanges("git", nil)
		require.NoError(t, err)
		p := string(patch)
		assert.Contains(t, p, "local working tree edit")
		assert.NotContains(t, p, "from_remote.txt")
	})

	t.Run("git source not in repo", func(t *testing.T) {
		t.Chdir(t.TempDir())
		_, err := loadLocalChanges("git", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not a git repository")
	})

	t.Run("file source", func(t *testing.T) {
		t.Parallel()
		patchFile := filepath.Join(t.TempDir(), "changes.patch")
		require.NoError(t, os.WriteFile(patchFile, []byte("diff content"), 0644))

		patch, err := loadLocalChanges(patchFile, nil)
		require.NoError(t, err)
		assert.Equal(t, "diff content", string(patch))
	})

	t.Run("file source not found", func(t *testing.T) {
		t.Parallel()
		_, err := loadLocalChanges("/nonexistent/path.patch", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("file source empty", func(t *testing.T) {
		t.Parallel()
		patchFile := filepath.Join(t.TempDir(), "empty.patch")
		require.NoError(t, os.WriteFile(patchFile, []byte{}, 0644))

		_, err := loadLocalChanges(patchFile, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "empty")
	})
}
