package migrate_test

import (
	"os"
	"strings"
	"testing"

	"github.com/JetBrains/teamcity-cli/internal/migrate"
	"github.com/JetBrains/teamcity-cli/internal/pipelineschema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertGitHubActions(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("testdata/github/ci.yml")
	require.NoError(t, err)

	cfg := migrate.CIConfig{Source: migrate.GitHubActions, File: ".github/workflows/ci.yml"}
	result, err := migrate.Convert(cfg, data, migrate.Options{})
	require.NoError(t, err)

	assert.Equal(t, "ci.tc.yml", result.OutputFile)
	assert.Equal(t, migrate.GitHubActions, result.Source)
	assert.Equal(t, 4, result.JobsConverted)
	assert.Greater(t, result.StepsConverted, 0)
	assert.GreaterOrEqual(t, len(result.Simplified), 10, "should simplify many steps")

	for _, want := range []string{
		"jobs:", "runs-on: Ubuntu-24.04-Large", "type: script",
		"./gradlew jsBrowserProductionWebpack", "./gradlew jsTest",
		"files-publication:", "dependencies:", "- build", "- test_unit",
		"mkdir -p dist", "npx playwright test", "Deploy to GitHub Pages",
	} {
		assert.Contains(t, result.YAML, want)
	}
	assert.NotContains(t, result.NeedsReview, "JamesIves/github-pages-deploy-action@v4")
	assert.NotContains(t, result.YAML, "actions/checkout")
	assert.NotContains(t, result.YAML, "actions/setup-java")

	valErr := pipelineschema.Validate(result.YAML)
	assert.Empty(t, valErr, "generated YAML should validate against schema: %s", valErr)

	checkoutCount := 0
	for _, s := range result.Simplified {
		if strings.Contains(s, "checkout") {
			checkoutCount++
		}
	}
	assert.GreaterOrEqual(t, checkoutCount, 1, "should simplify checkout steps")
}

func TestConvertQodana(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("testdata/github/quality.yml")
	require.NoError(t, err)

	cfg := migrate.CIConfig{Source: migrate.GitHubActions, File: ".github/workflows/quality.yml"}
	result, err := migrate.Convert(cfg, data, migrate.Options{})
	require.NoError(t, err)

	assert.Equal(t, "quality.tc.yml", result.OutputFile)
	assert.Equal(t, 1, result.JobsConverted)
	assert.Contains(t, result.YAML, "Qodana")
	assert.Contains(t, result.YAML, "native Qodana integration")
}

func TestActionTransformers(t *testing.T) {
	t.Parallel()

	t.Run("registry lookup", func(t *testing.T) {
		t.Parallel()
		tests := []struct {
			action string
			found  bool
		}{
			{"actions/checkout@v4", true},
			{"docker/build-push-action@v5", true},
			{"codecov/codecov-action@v3", true},
			{"my-org/custom-action@v1", false},
			// Data-table entries (scriptActions, manualActions, unsupportedActions).
			{"JetBrains/qodana-action@v2025.1", true},
			{"aws-actions/amazon-ecs-deploy-task-definition@v2", true},
			{"anothrNick/github-tag-action@v1", true},
			{"azure/login@v2", true},
			{"hashicorp/vault-action@v3", true},
			{"slackapi/slack-github-action@v2", true},
			{"pypa/gh-action-pypi-publish@release/v1", true},
			{"dorny/paths-filter@v3", true},
		}
		for _, tt := range tests {
			t.Run(tt.action, func(t *testing.T) {
				_, ok := migrate.LookupActionTransformer(tt.action)
				assert.Equal(t, tt.found, ok)
			})
		}
	})

	t.Run("checkout simplified", func(t *testing.T) {
		t.Parallel()
		transformer, _ := migrate.LookupActionTransformer("actions/checkout@v4")
		r := transformer("", "actions/checkout@v4", nil)
		assert.Equal(t, migrate.StatusSimplified, r.Status)
	})

	t.Run("cache enables dependency cache", func(t *testing.T) {
		t.Parallel()
		transformer, _ := migrate.LookupActionTransformer("actions/cache@v3")
		r := transformer("", "actions/cache@v3", nil)
		assert.Equal(t, migrate.StatusSimplified, r.Status)
		assert.Contains(t, r.Features, "enable-dependency-cache")
	})

	t.Run("upload-artifact produces file publication", func(t *testing.T) {
		t.Parallel()
		transformer, _ := migrate.LookupActionTransformer("actions/upload-artifact@v4")
		r := transformer("", "actions/upload-artifact@v4", map[string]string{"path": "dist/**"})
		assert.Equal(t, migrate.StatusSimplified, r.Status)
		require.Len(t, r.Artifacts, 1)
		assert.Equal(t, "dist/**", r.Artifacts[0].Path)
	})

	t.Run("qodana converts with native-integration script", func(t *testing.T) {
		t.Parallel()
		transformer, ok := migrate.LookupActionTransformer("JetBrains/qodana-action@v2025.1")
		require.True(t, ok)
		r := transformer("", "JetBrains/qodana-action@v2025.1", nil)
		assert.Equal(t, migrate.StatusConverted, r.Status)
		require.Len(t, r.Steps, 1)
		assert.Equal(t, "Qodana", r.Steps[0].Name)
		assert.Contains(t, r.Steps[0].ScriptContent, "native Qodana integration")
	})

	t.Run("missing required inputs emit shell guards", func(t *testing.T) {
		t.Parallel()
		transformer, ok := migrate.LookupActionTransformer("azure/k8s-set-context@v4")
		require.True(t, ok)
		r := transformer("", "azure/k8s-set-context@v4", map[string]string{})
		require.Len(t, r.Steps, 1)
		assert.Contains(t, r.Steps[0].ScriptContent, "${RESOURCE_GROUP:?")
		assert.Contains(t, r.Steps[0].ScriptContent, "${CLUSTER_NAME:?")
		assert.NotContains(t, r.Steps[0].ScriptContent, `""`, "no empty-string arguments")
	})

	t.Run("docker build-push", func(t *testing.T) {
		t.Parallel()
		transformer, _ := migrate.LookupActionTransformer("docker/build-push-action@v5")
		r := transformer("Build", "docker/build-push-action@v5", map[string]string{
			"tags": "myapp:latest", "push": "true", "context": ".",
		})
		assert.Equal(t, migrate.StatusConverted, r.Status)
		require.Len(t, r.Steps, 1)
		assert.Contains(t, r.Steps[0].ScriptContent, "docker build")
		assert.Contains(t, r.Steps[0].ScriptContent, "docker push")
	})
}

func TestUnknownActionMultilineInputCommented(t *testing.T) {
	t.Parallel()
	r := migrate.Unknown("acme/dangerous@v1", map[string]string{
		"note": "hello\nrm -rf tmp",
	})
	require.Len(t, r.Steps, 1)
	assert.NotContains(t, r.Steps[0].ScriptContent, "\nrm -rf tmp", "multiline input line 2 must be commented, not executable")
}

func TestGHReleaseMultilineFilesNotInjected(t *testing.T) {
	t.Parallel()

	transformer, ok := migrate.LookupActionTransformer("softprops/action-gh-release@v2")
	require.True(t, ok)
	r := transformer("Release", "softprops/action-gh-release@v2", map[string]string{
		"tag_name": "v1.0.0",
		"files":    "dist/app.zip\nrm -rf tmp",
	})
	require.Len(t, r.Steps, 1)
	script := r.Steps[0].ScriptContent
	// The multiline value must stay on the single gh-release command line, not become a new shell line.
	assert.NotContains(t, script, "\n")
	assert.Contains(t, script, "dist/app.zip")
}

func TestGHAWindowsImplicitShellWarned(t *testing.T) {
	t.Parallel()

	wf := `name: ci
on: push
jobs:
  win:
    runs-on: windows-latest
    steps:
      - run: Write-Host hi
  lin:
    runs-on: ubuntu-latest
    steps:
      - run: echo hi
`
	cfg := migrate.CIConfig{Source: migrate.GitHubActions, File: ".github/workflows/ci.yml"}
	result, err := migrate.Convert(cfg, []byte(wf), migrate.Options{})
	require.NoError(t, err)

	manuals := strings.Join(result.ManualSetup, "\n")
	// Windows step warns; the Linux step must not, so exactly one warning is expected.
	assert.Equal(t, 1, strings.Count(manuals, "runs on a Windows runner with no explicit shell"))
}

func TestMapGHAExpressions(t *testing.T) {
	t.Parallel()
	tests := []struct{ input, want string }{
		{"${{ github.sha }}", "%build.vcs.number%"},
		{"${{ github.ref_name }}", "%teamcity.build.branch%"},
		{"${{ github.run_number }}", "%build.number%"},
		{"${{ env.MY_VAR }}", "%env.MY_VAR%"},
		{"${{ secrets.SECRET_TOKEN }}", "%SECRET_TOKEN%"},
		// github.ref (full ref) and github.event_name have no direct TC equivalent — must stay untouched so they're flagged as manual setup.
		{"${{ github.ref }}", "${{ github.ref }}"},
		{"${{ github.event_name }}", "${{ github.event_name }}"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, migrate.MapGHAExpressions(tt.input))
	}
}
