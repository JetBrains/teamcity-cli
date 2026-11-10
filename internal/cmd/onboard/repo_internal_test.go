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
		{"tilde subdir with platform separator", "~" + string(filepath.Separator) + "projects", filepath.Join(home, "projects")},
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
	assert.ErrorContains(t, validateRepoPath(sub), "is not the root of a git repository", "a subdirectory of a repo is not its root")

	assert.ErrorContains(t, validateRepoPath(filepath.Join(repo, "missing")), "does not exist")
	assert.ErrorContains(t, validateRepoPath(file), "is not a directory")
	assert.ErrorContains(t, validateRepoPath(t.TempDir()), "is not the root of a git repository")
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

	t.Run("subdirectory is not walked up to the root", func(t *testing.T) {
		repo := initRepoWithCommit(t, "main")
		sub := filepath.Join(repo, "services", "api")
		require.NoError(t, os.MkdirAll(sub, 0o755))

		_, err := resolveRepository(sourcePath, sub)
		_, ok := errors.AsType[*api.ValidationError](err)
		assert.True(t, ok, "want *api.ValidationError, got %T: %v", err, err)
	})

	t.Run("worktree with a .git file", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: /elsewhere\n"), 0o644))

		got, err := resolveRepository(sourcePath, dir)
		require.NoError(t, err)
		samePath(t, dir, got.dir)
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

func TestSampleReposAreValid(t *testing.T) {
	require.NotEmpty(t, sampleRepos)
	for _, d := range sampleRepos {
		assert.NotEmpty(t, d.name)
		assert.NotEmpty(t, d.stack, "sample %q needs a tech stack", d.name)
		assert.NotEmpty(t, git.CanonicalURL(d.url), "sample URL %q must be a parseable remote", d.url)
		if d.revision != "" {
			assert.Regexp(t, `^[0-9a-f]{40}$`, d.revision, "sample %q revision must be a full commit SHA", d.name)
		}
	}
}

func TestSampleRepoLabel(t *testing.T) {
	assert.Equal(t, "Spring PetClinic [Java, Spring Boot, Maven]", sampleRepo{name: "Spring PetClinic", stack: "Java, Spring Boot, Maven"}.label())
	assert.Equal(t, "Spring PetClinic", sampleRepo{name: "Spring PetClinic"}.label())
	assert.Equal(t, "Spring PetClinic [Java, Spring Boot, Maven] [100 tests, fast pipeline]", sampleRepo{name: "Spring PetClinic", stack: "Java, Spring Boot, Maven", tag: "100 tests, fast pipeline"}.label())
}

func TestCurrentRepoRoot(t *testing.T) {
	t.Run("repository root", func(t *testing.T) {
		repo := initRepoWithCommit(t, "main")
		t.Chdir(repo)
		samePath(t, repo, currentRepoRoot())
	})

	t.Run("plain folder inside a repository", func(t *testing.T) {
		// a folder that is not tracked, under a parent repository such as a home folder under git
		repo := initRepoWithCommit(t, "main")
		plain := filepath.Join(repo, "verification-cli")
		require.NoError(t, os.MkdirAll(plain, 0o755))
		t.Chdir(plain)
		assert.Empty(t, currentRepoRoot())
	})

	t.Run("outside any repository", func(t *testing.T) {
		t.Chdir(t.TempDir())
		assert.Empty(t, currentRepoRoot())
	})
}
