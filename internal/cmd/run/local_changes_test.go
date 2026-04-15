package run

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
		assert.Contains(t, err.Error(), "no uncommitted changes")
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
