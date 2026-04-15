package link_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/JetBrains/teamcity-cli/internal/cmdtest"
	"github.com/JetBrains/teamcity-cli/internal/link"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLinkUpsertSingleServer(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	ts := cmdtest.SetupMockClient(t)
	cmdtest.RunCmdWithFactory(t, ts.Factory, "link",
		"--server", "https://x.example", "--project", "Acme", "--job", "Acme_Build")

	cfg, err := link.Load(filepath.Join(dir, link.FileName))
	require.NoError(t, err)
	require.Len(t, cfg.Servers, 1)
	assert.Equal(t, "https://x.example", cfg.Servers[0].URL)
	assert.Equal(t, "Acme", cfg.Servers[0].Project)
	assert.Equal(t, "Acme_Build", cfg.Servers[0].Job)
}

func TestLinkAddsSecondServer(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	ts := cmdtest.SetupMockClient(t)
	cmdtest.RunCmdWithFactory(t, ts.Factory, "link",
		"--server", "https://primary.example", "--project", "P", "--job", "P_Build")
	cmdtest.RunCmdWithFactory(t, ts.Factory, "link",
		"--server", "https://nightly.example", "--project", "N", "--jobs", "N_Release,N_Eval")

	cfg, err := link.Load(filepath.Join(dir, link.FileName))
	require.NoError(t, err)
	require.Len(t, cfg.Servers, 2)
	assert.Equal(t, "P_Build", cfg.Servers[0].Job)
	assert.Equal(t, []string{"N_Release", "N_Eval"}, cfg.Servers[1].Jobs)
}

func TestLinkUpsertReplacesExistingEntry(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	ts := cmdtest.SetupMockClient(t)
	cmdtest.RunCmdWithFactory(t, ts.Factory, "link",
		"--server", "https://x.example", "--project", "Old")
	cmdtest.RunCmdWithFactory(t, ts.Factory, "link",
		"--server", "https://x.example", "--project", "New", "--job", "New_Build")

	cfg, err := link.Load(filepath.Join(dir, link.FileName))
	require.NoError(t, err)
	require.Len(t, cfg.Servers, 1)
	assert.Equal(t, "New", cfg.Servers[0].Project)
	assert.Equal(t, "New_Build", cfg.Servers[0].Job)
}

func TestLinkRequiresAtLeastOneFieldFlag(t *testing.T) {
	t.Chdir(t.TempDir())
	ts := cmdtest.SetupMockClient(t)
	err := cmdtest.CaptureErr(t, ts.Factory, "link", "--server", "https://x.example")
	assert.Contains(t, err.Error(), "at least one of --project")
}

func TestLinkPathScopedFromSubdir(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "services", "api")
	require.NoError(t, os.MkdirAll(sub, 0o755))

	t.Chdir(dir)
	ts := cmdtest.SetupMockClient(t)
	cmdtest.RunCmdWithFactory(t, ts.Factory, "link",
		"--server", "https://x.example", "--project", "Mono", "--job", "Mono_Build")

	t.Chdir(sub)
	cmdtest.RunCmdWithFactory(t, ts.Factory, "link",
		"--server", "https://x.example", "--project", "API", "--job", "API_Build")

	cfg, err := link.Load(filepath.Join(dir, link.FileName))
	require.NoError(t, err)
	require.Len(t, cfg.Servers, 1)
	srv := cfg.Servers[0]
	assert.Equal(t, "Mono", srv.Project, "top-level scope preserved")
	require.Contains(t, srv.Paths, "services/api")
	assert.Equal(t, "API", srv.Paths["services/api"].Project)
	assert.Equal(t, "API_Build", srv.Paths["services/api"].Job)
}

func TestLinkAddsHTTPSSchemeWhenMissing(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	ts := cmdtest.SetupMockClient(t)
	cmdtest.RunCmdWithFactory(t, ts.Factory, "link",
		"--server", "x.example", "--project", "Acme", "--job", "Acme_Build")

	cfg, err := link.Load(filepath.Join(dir, link.FileName))
	require.NoError(t, err)
	require.Len(t, cfg.Servers, 1)
	assert.Equal(t, "https://x.example", cfg.Servers[0].URL,
		"schemeless --server is stored with an https:// prefix so Match() can find it")
}

func TestLinkRefusesToOverwriteMalformedFile(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	path := filepath.Join(dir, link.FileName)
	require.NoError(t, os.WriteFile(path, []byte("this is = not valid toml ]]"), 0o644))

	ts := cmdtest.SetupMockClient(t)
	err := cmdtest.CaptureErr(t, ts.Factory, "link",
		"--server", "https://x.example", "--project", "Acme")
	require.Error(t, err)

	data, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	assert.Contains(t, string(data), "not valid toml", "existing file must not be overwritten on parse error")
}

func TestLinkScope(t *testing.T) {
	// Each case covers a different normalization or rejection path inside resolveScopePath.
	// wantKey == "" means top-level (no [server.paths.*] entry).
	for _, tc := range []struct {
		name    string
		scope   string
		wantKey string
		wantErr string
	}{
		{"nested path", "services/web", "services/web", ""},
		{"./prefix cleaned", "./services/api", "services/api", ""},
		{"dot-dot segment cleaned", "services/../api", "api", ""},
		{"trailing slash trimmed", "services/web/", "services/web", ""},
		{"dot is top-level", ".", "", ""},
		{"escaping path rejected", "../escape", "", "escapes"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Chdir(dir)
			ts := cmdtest.SetupMockClient(t)
			args := []string{"link", "--server", "https://x.example", "--scope", tc.scope, "--project", "P"}
			if tc.wantErr != "" {
				err := cmdtest.CaptureErr(t, ts.Factory, args...)
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}
			cmdtest.RunCmdWithFactory(t, ts.Factory, args...)
			cfg, err := link.Load(filepath.Join(dir, link.FileName))
			require.NoError(t, err)
			require.Len(t, cfg.Servers, 1)
			if tc.wantKey == "" {
				assert.Equal(t, "P", cfg.Servers[0].Project, "top-level project set")
				assert.Empty(t, cfg.Servers[0].Paths)
				return
			}
			require.Contains(t, cfg.Servers[0].Paths, tc.wantKey,
				"--scope %q must store cleaned key %q so resolution can find it", tc.scope, tc.wantKey)
			assert.Equal(t, "P", cfg.Servers[0].Paths[tc.wantKey].Project)
		})
	}
}
