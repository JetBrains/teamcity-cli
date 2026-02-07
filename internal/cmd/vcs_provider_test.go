package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGitProvider_Name(t *testing.T) {
	p := &GitProvider{}
	assert.Equal(t, "git", p.Name())
}

func TestGitProvider_FormatRevision(t *testing.T) {
	p := &GitProvider{}

	// Long SHA gets truncated to 7
	assert.Equal(t, "abc1234", p.FormatRevision("abc12345678901234567890123456789012345678"))

	// Short string stays the same
	assert.Equal(t, "abc", p.FormatRevision("abc"))

	// Exactly 7 chars stays the same
	assert.Equal(t, "abc1234", p.FormatRevision("abc1234"))
}

func TestGitProvider_FormatVCSBranch(t *testing.T) {
	p := &GitProvider{}

	assert.Equal(t, "refs/heads/main", p.FormatVCSBranch("main"))
	assert.Equal(t, "refs/heads/feature/test", p.FormatVCSBranch("feature/test"))
	assert.Equal(t, "refs/tags/v1.0", p.FormatVCSBranch("refs/tags/v1.0"))
	assert.Equal(t, "", p.FormatVCSBranch(""))
}

func TestGitProvider_DiffHint(t *testing.T) {
	p := &GitProvider{}

	hint := p.DiffHint("abc1234567890", "def1234567890")
	assert.Equal(t, "git diff abc1234^..def1234", hint)
}

func TestGitProvider_IsAvailable(t *testing.T) {
	t.Run("in git repo", func(t *testing.T) {
		dir := setupGitRepo(t)
		restore := chdir(t, dir)
		defer restore()

		p := &GitProvider{}
		assert.True(t, p.IsAvailable())
	})

	t.Run("not in git repo", func(t *testing.T) {
		dir := t.TempDir()
		restore := chdir(t, dir)
		defer restore()

		p := &GitProvider{}
		assert.False(t, p.IsAvailable())
	})
}

func TestGitProvider_GetCurrentBranch(t *testing.T) {
	dir := setupGitRepo(t)
	restore := chdir(t, dir)
	defer restore()

	createFile(t, dir, "test.txt", "content")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "initial")

	p := &GitProvider{}
	branch, err := p.GetCurrentBranch()
	assert.NoError(t, err)
	assert.Contains(t, []string{"main", "master"}, branch)
}

func TestGitProvider_GetHeadRevision(t *testing.T) {
	dir := setupGitRepo(t)
	restore := chdir(t, dir)
	defer restore()

	createFile(t, dir, "test.txt", "content")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "initial")

	p := &GitProvider{}
	rev, err := p.GetHeadRevision()
	assert.NoError(t, err)
	assert.Regexp(t, "^[0-9a-f]{40}$", rev)
}

func TestGitProvider_GetLocalDiff(t *testing.T) {
	dir := setupGitRepo(t)
	restore := chdir(t, dir)
	defer restore()

	createFile(t, dir, "test.txt", "content")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "initial")

	// Make changes
	createFile(t, dir, "test.txt", "modified")

	p := &GitProvider{}
	diff, err := p.GetLocalDiff()
	assert.NoError(t, err)
	assert.Contains(t, string(diff), "modified")
}

func TestPerforceProvider_Name(t *testing.T) {
	p := &PerforceProvider{}
	assert.Equal(t, "perforce", p.Name())
}

func TestPerforceProvider_FormatRevision(t *testing.T) {
	p := &PerforceProvider{}

	// Perforce changelist numbers are returned as-is
	assert.Equal(t, "12345", p.FormatRevision("12345"))
	assert.Equal(t, "1", p.FormatRevision("1"))
}

func TestPerforceProvider_FormatVCSBranch(t *testing.T) {
	p := &PerforceProvider{}

	// Perforce depot paths pass through unchanged
	assert.Equal(t, "//depot/main", p.FormatVCSBranch("//depot/main"))
	assert.Equal(t, "//stream/dev", p.FormatVCSBranch("//stream/dev"))
}

func TestPerforceProvider_DiffHint(t *testing.T) {
	p := &PerforceProvider{}

	hint := p.DiffHint("100", "200")
	assert.Equal(t, "p4 changes -l @100,@200", hint)
}

func TestPerforceProvider_PushBranch(t *testing.T) {
	// Push is a no-op for Perforce
	p := &PerforceProvider{}
	err := p.PushBranch("//depot/main")
	assert.NoError(t, err)
}

func TestPerforceProvider_IsAvailable(t *testing.T) {
	// Without Perforce installed, should return false
	dir := t.TempDir()
	restore := chdir(t, dir)
	defer restore()

	p := &PerforceProvider{}
	// In CI without p4 binary, this should be false
	// We don't assert true/false since it depends on the environment
	_ = p.IsAvailable()
}

func TestDetectVCS_Git(t *testing.T) {
	dir := setupGitRepo(t)
	restore := chdir(t, dir)
	defer restore()

	vcs := DetectVCS()
	assert.NotNil(t, vcs)
	assert.Equal(t, "git", vcs.Name())
}

func TestDetectVCS_NoVCS(t *testing.T) {
	dir := t.TempDir()
	restore := chdir(t, dir)
	defer restore()

	vcs := DetectVCS()
	assert.Nil(t, vcs)
}

func TestDetectVCSByName(t *testing.T) {
	tests := []struct {
		name     string
		expected string
	}{
		{"git", "git"},
		{"p4", "perforce"},
		{"perforce", "perforce"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			vcs := DetectVCSByName(tc.name)
			assert.NotNil(t, vcs)
			assert.Equal(t, tc.expected, vcs.Name())
		})
	}

	t.Run("unknown", func(t *testing.T) {
		vcs := DetectVCSByName("svn")
		assert.Nil(t, vcs)
	})
}
