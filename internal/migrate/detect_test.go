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
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bamboo.yml"), []byte("version: 2\nplan:\n  key: K\nstages: []\n"), 0644))

	configs, err := Detect(dir, GitHubActions)
	require.NoError(t, err)
	require.Len(t, configs, 1, "the bamboo spec must be filtered out")
	assert.Equal(t, GitHubActions, configs[0].Source)
}

func TestInferSource(t *testing.T) {
	t.Parallel()

	cases := []struct {
		path string
		want SourceCI
	}{
		{".github/workflows/ci.yml", GitHubActions},
		{"/abs/path/.github/workflows/release.yaml", GitHubActions},
		{"bamboo-specs/bamboo.yml", Bamboo},
		{"/tmp/bamboo.yaml", Bamboo},
		{"/tmp/random.yml", ""},
		{"Jenkinsfile", ""},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, InferSource(c.path), c.path)
	}
}

func TestAnalyzeFileExplicitPath(t *testing.T) {
	t.Parallel()

	tmp := filepath.Join(t.TempDir(), "weird-location.yml")
	data, err := os.ReadFile("testdata/github/ci.yml")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(tmp, data, 0644))

	cfg, err := AnalyzeFile(tmp, GitHubActions)
	require.NoError(t, err)
	assert.Equal(t, GitHubActions, cfg.Source)
	assert.Equal(t, 4, cfg.Jobs)
}

func TestAnalyzeFileMissingSourceCannotInfer(t *testing.T) {
	t.Parallel()

	tmp := filepath.Join(t.TempDir(), "ambiguous.yml")
	require.NoError(t, os.WriteFile(tmp, []byte("jobs: {}\n"), 0644))

	_, err := AnalyzeFile(tmp, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot infer CI source")
}
