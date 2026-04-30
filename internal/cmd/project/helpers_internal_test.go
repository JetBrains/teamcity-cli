package project

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsSSHURL(t *testing.T) {
	t.Parallel()

	cases := []struct {
		url  string
		want bool
	}{
		// Explicit ssh:// scheme.
		{"ssh://git@github.com/org/repo", true},
		{"ssh://example.com:2222/repo.git", true},
		// SCP-style (user@host:path), no scheme.
		{"git@github.com:org/repo.git", true},
		{"jenkins@gerrit.example.com:project", true},
		// HTTPS / HTTP — has :// but doesn't start with ssh://, even with @ in userinfo.
		{"https://github.com/org/repo", false},
		{"http://example.com/repo", false},
		{"https://user@example.com/repo", false}, // @ in HTTPS userinfo: still not SSH
		// Edge cases.
		{"", false},
		{"file:///tmp/repo", false},
		{"plain.example.com", false},
	}
	for _, tc := range cases {
		t.Run(tc.url, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, isSSHURL(tc.url))
		})
	}
}

func TestInferAuthFromURL(t *testing.T) {
	t.Parallel()

	assert.Equal(t, authSSHKey, inferAuthFromURL("git@github.com:org/repo.git"))
	assert.Equal(t, authSSHKey, inferAuthFromURL("ssh://example.com/repo"))
	assert.Equal(t, authAnonymous, inferAuthFromURL("https://github.com/org/repo"))
	assert.Equal(t, authAnonymous, inferAuthFromURL(""))
}

func TestBaseName(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in, want string
	}{
		{"foo", "foo"},
		{"path/to/foo", "foo"},
		{"path\\to\\foo", "foo"},    // Windows separator
		{"path/to\\mixed", "mixed"}, // mixed separators take the last one
		{"/abs/path/key", "key"},
		{"./relative/key.pem", "key.pem"},
		{"", ""},
		{"trailing/", ""},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, baseName(tc.in))
		})
	}
}

func TestParseValidationStats(t *testing.T) {
	t.Parallel()

	// Real DSL shape: target/generated-configs/<ProjectName>/{buildTypes,vcsRoots}/*.xml.
	t.Run("missing target dir → empty", func(t *testing.T) {
		t.Parallel()
		assert.Empty(t, parseValidationStats(t.TempDir()))
	})

	t.Run("empty configs dir → empty (no projects)", func(t *testing.T) {
		t.Parallel()
		dsl := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(dsl, "target", "generated-configs"), 0700))
		// projects=0 → function short-circuits to "" by design, regardless of buildTypes/vcsRoots.
		assert.Empty(t, parseValidationStats(dsl))
	})

	t.Run("two projects with builds and vcs roots", func(t *testing.T) {
		t.Parallel()
		dsl := t.TempDir()
		base := filepath.Join(dsl, "target", "generated-configs")

		// Real shape: <project>/{project-config.xml, buildTypes/*.xml, vcsRoots/*.xml}
		require.NoError(t, os.MkdirAll(filepath.Join(base, "MyApp", "buildTypes"), 0700))
		require.NoError(t, os.MkdirAll(filepath.Join(base, "MyApp", "vcsRoots"), 0700))
		require.NoError(t, os.MkdirAll(filepath.Join(base, "Infra", "buildTypes"), 0700))

		require.NoError(t, os.WriteFile(filepath.Join(base, "MyApp", "buildTypes", "Build.xml"), []byte("<x/>"), 0600))
		require.NoError(t, os.WriteFile(filepath.Join(base, "MyApp", "buildTypes", "Test.xml"), []byte("<x/>"), 0600))
		require.NoError(t, os.WriteFile(filepath.Join(base, "MyApp", "vcsRoots", "MainRepo.xml"), []byte("<x/>"), 0600))
		require.NoError(t, os.WriteFile(filepath.Join(base, "Infra", "buildTypes", "Deploy.xml"), []byte("<x/>"), 0600))

		got := parseValidationStats(dsl)
		assert.Equal(t, "Projects: 2, Build configurations: 3, VCS roots: 1", got)
	})

	t.Run("projects with no vcs roots → omits VCS line", func(t *testing.T) {
		t.Parallel()
		dsl := t.TempDir()
		base := filepath.Join(dsl, "target", "generated-configs")
		require.NoError(t, os.MkdirAll(filepath.Join(base, "MyApp", "buildTypes"), 0700))
		require.NoError(t, os.WriteFile(filepath.Join(base, "MyApp", "buildTypes", "Build.xml"), []byte("<x/>"), 0600))

		got := parseValidationStats(dsl)
		assert.Equal(t, "Projects: 1, Build configurations: 1", got)
		assert.NotContains(t, got, "VCS roots", "VCS line should be suppressed when count is 0")
	})
}
