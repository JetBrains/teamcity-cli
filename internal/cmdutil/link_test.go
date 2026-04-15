package cmdutil

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/JetBrains/teamcity-cli/internal/config"
	"github.com/JetBrains/teamcity-cli/internal/link"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeFile(t *testing.T, dir string, c *link.Config) {
	t.Helper()
	require.NoError(t, link.Save(filepath.Join(dir, link.FileName), c))
}

func TestResolveCascadeMatchesActiveServer(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeFile(t, dir, &link.Config{Servers: []link.Server{
		{URL: "https://primary.example", Project: "Primary", Job: "Primary_Build"},
		{URL: "https://nightly.example", Project: "Nightly", Jobs: []string{"Nightly_Release", "Nightly_Eval"}},
	}})

	t.Setenv(config.EnvServerURL, "https://primary.example")
	t.Setenv(config.EnvProject, "")
	t.Setenv(config.EnvJob, "")
	assert.Equal(t, "Primary", (&Factory{}).ResolveProject(""))
	assert.Equal(t, "Primary_Build", (&Factory{}).ResolveDefaultJob(""))

	t.Setenv(config.EnvServerURL, "https://nightly.example")
	assert.Equal(t, "Nightly", (&Factory{}).ResolveProject(""))
	assert.Empty(t, (&Factory{}).ResolveDefaultJob(""), "two jobs is ambiguous → no auto-default")
}

func TestResolveExplicitAndEnvBeatFile(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeFile(t, dir, &link.Config{Servers: []link.Server{
		{URL: "https://x.example", Project: "FromFile", Job: "FromFile_Build"},
	}})

	t.Setenv(config.EnvServerURL, "https://x.example")
	t.Setenv(config.EnvProject, "FromEnv")
	t.Setenv(config.EnvJob, "FromEnv_Job")
	assert.Equal(t, "FromExplicit", (&Factory{}).ResolveProject("FromExplicit"))
	assert.Equal(t, "FromExplicit_Job", (&Factory{}).ResolveDefaultJob("FromExplicit_Job"))
	assert.Equal(t, "FromEnv", (&Factory{}).ResolveProject(""))
	assert.Equal(t, "FromEnv_Job", (&Factory{}).ResolveDefaultJob(""))
}

func TestResolveNoMatchingServer(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeFile(t, dir, &link.Config{Servers: []link.Server{
		{URL: "https://only.example", Project: "OnlyOne"},
	}})

	t.Setenv(config.EnvServerURL, "https://elsewhere.example")
	t.Setenv(config.EnvProject, "")
	t.Setenv(config.EnvJob, "")
	assert.Empty(t, (&Factory{}).ResolveProject(""))
	assert.Empty(t, (&Factory{}).ResolveDefaultJob(""))
}

func TestResolveSingleJobsFallback(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeFile(t, dir, &link.Config{Servers: []link.Server{
		{URL: "https://x.example", Jobs: []string{"Only_Build"}},
	}})

	t.Setenv(config.EnvServerURL, "https://x.example")
	t.Setenv(config.EnvJob, "")
	assert.Equal(t, "Only_Build", (&Factory{}).ResolveDefaultJob(""))
}

func TestSkipLinkLookup(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeFile(t, dir, &link.Config{Servers: []link.Server{
		{URL: "https://x.example", Project: "ShouldBeSkipped"},
	}})

	t.Setenv(config.EnvServerURL, "https://x.example")
	f := &Factory{}
	f.SkipLinkLookup()
	assert.Empty(t, f.ResolveProject(""))
}

func TestResolveWarnsOnMalformedFile(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, link.FileName), []byte("not = valid = toml"), 0o644))

	var stderr bytes.Buffer
	f := &Factory{Printer: &output.Printer{Out: &bytes.Buffer{}, ErrOut: &stderr}}
	t.Setenv(config.EnvServerURL, "https://x.example")
	t.Setenv(config.EnvProject, "")

	assert.Empty(t, f.ResolveProject(""))
	assert.Contains(t, stderr.String(), link.FileName)
}

func TestResolvePathScopedFromSubdir(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "services", "api")
	require.NoError(t, os.MkdirAll(sub, 0o755))
	writeFile(t, dir, &link.Config{Servers: []link.Server{{
		URL:     "https://x.example",
		Project: "Mono",
		Job:     "Mono_Build",
		Paths: map[string]link.PathScope{
			"services/api": {Project: "API", Job: "API_Build"},
		},
	}}})

	t.Setenv(config.EnvServerURL, "https://x.example")
	t.Setenv(config.EnvProject, "")
	t.Setenv(config.EnvJob, "")

	t.Chdir(dir)
	assert.Equal(t, "Mono", (&Factory{}).ResolveProject(""))
	assert.Equal(t, "Mono_Build", (&Factory{}).ResolveDefaultJob(""))

	t.Chdir(sub)
	assert.Equal(t, "API", (&Factory{}).ResolveProject(""))
	assert.Equal(t, "API_Build", (&Factory{}).ResolveDefaultJob(""))
}
