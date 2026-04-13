package migrate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectGitHubActions(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	workflowDir := filepath.Join(dir, ".github", "workflows")
	require.NoError(t, os.MkdirAll(workflowDir, 0755))

	data, err := os.ReadFile("testdata/github/ci.yml")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(workflowDir, "ci.yml"), data, 0644))

	configs, err := Detect(dir, "")
	require.NoError(t, err)
	require.Len(t, configs, 1)

	cfg := configs[0]
	assert.Equal(t, GitHubActions, cfg.Source)
	assert.Equal(t, ".github/workflows/ci.yml", cfg.File)
	assert.Equal(t, 4, cfg.Jobs)
	assert.Greater(t, cfg.Steps, 0)
	assert.Contains(t, cfg.Features, "artifacts")
}

func TestDetectWithFilter(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	workflowDir := filepath.Join(dir, ".github", "workflows")
	require.NoError(t, os.MkdirAll(workflowDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(workflowDir, "ci.yml"), []byte("on: push\njobs:\n  test:\n    runs-on: ubuntu-latest\n    steps:\n      - run: echo hi\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gitlab-ci.yml"), []byte("test:\n  script: echo hi\n"), 0644))

	// Filter to GitLab only
	configs, err := Detect(dir, GitLabCI)
	require.NoError(t, err)
	require.Len(t, configs, 1)
	assert.Equal(t, GitLabCI, configs[0].Source)
}

func TestDetectEmptyDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configs, err := Detect(dir, "")
	require.NoError(t, err)
	assert.Empty(t, configs)
}
