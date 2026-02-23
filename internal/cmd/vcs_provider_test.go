package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGitProvider_Name(t *testing.T) {
	assert.Equal(t, "git", (&GitProvider{}).Name())
}

func TestGitProvider_FormatRevision(t *testing.T) {
	p := &GitProvider{}
	assert.Equal(t, "abc1234", p.FormatRevision("abc12345678901234567890123456789012345678"))
	assert.Equal(t, "abc", p.FormatRevision("abc"))
	assert.Equal(t, "abc1234", p.FormatRevision("abc1234"))
}

func TestGitProvider_DiffHint(t *testing.T) {
	assert.Equal(t, "git diff abc1234^..def1234", (&GitProvider{}).DiffHint("abc1234567890", "def1234567890"))
}

func TestGitProvider_IsAvailable(t *testing.T) {
	t.Run("in git repo", func(t *testing.T) {
		dir := setupGitRepo(t)
		restore := chdir(t, dir)
		defer restore()
		assert.True(t, (&GitProvider{}).IsAvailable())
	})

	t.Run("not in git repo", func(t *testing.T) {
		dir := t.TempDir()
		restore := chdir(t, dir)
		defer restore()
		assert.False(t, (&GitProvider{}).IsAvailable())
	})
}

func TestGitProvider_GetCurrentBranch(t *testing.T) {
	dir := setupGitRepo(t)
	restore := chdir(t, dir)
	defer restore()

	createFile(t, dir, "test.txt", "content")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "initial")

	branch, err := (&GitProvider{}).GetCurrentBranch()
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

	rev, err := (&GitProvider{}).GetHeadRevision()
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
	createFile(t, dir, "test.txt", "modified")

	diff, err := (&GitProvider{}).GetLocalDiff()
	assert.NoError(t, err)
	assert.Contains(t, string(diff), "modified")
}

func TestPerforceProvider_Name(t *testing.T) {
	assert.Equal(t, "perforce", (&PerforceProvider{}).Name())
}

func TestPerforceProvider_FormatRevision(t *testing.T) {
	p := &PerforceProvider{}
	assert.Equal(t, "12345", p.FormatRevision("12345"))
	assert.Equal(t, "1", p.FormatRevision("1"))
}

func TestPerforceProvider_DiffHint(t *testing.T) {
	assert.Equal(t, "p4 changes -l @100,@200", (&PerforceProvider{}).DiffHint("100", "200"))
}

func TestPerforceProvider_PushBranch(t *testing.T) {
	assert.NoError(t, (&PerforceProvider{}).PushBranch("//depot/main"))
}

func TestPerforceProvider_IsAvailable(t *testing.T) {
	dir := t.TempDir()
	restore := chdir(t, dir)
	defer restore()
	// Just exercise the code path; result depends on whether p4 is installed
	_ = (&PerforceProvider{}).IsAvailable()
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
	assert.Nil(t, DetectVCS())
}

func TestDetectVCSByName(t *testing.T) {
	for _, tc := range []struct {
		name     string
		expected string
	}{
		{"git", "git"},
		{"p4", "perforce"},
		{"perforce", "perforce"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			vcs := DetectVCSByName(tc.name)
			assert.NotNil(t, vcs)
			assert.Equal(t, tc.expected, vcs.Name())
		})
	}

	t.Run("unknown", func(t *testing.T) {
		assert.Nil(t, DetectVCSByName("svn"))
	})
}
