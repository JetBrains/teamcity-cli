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

func TestMapGHAExpressions(t *testing.T) {
	t.Parallel()
	tests := []struct{ input, want string }{
		{"${{ github.sha }}", "%%build.vcs.number%%"},
		{"${{ github.ref }}", "%%teamcity.build.branch%%"},
		{"${{ github.run_number }}", "%%build.number%%"},
		{"${{ env.MY_VAR }}", "%%env.MY_VAR%%"},
		{"${{ secrets.SECRET_TOKEN }}", "%%SECRET_TOKEN%%"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, migrate.MapGHAExpressions(tt.input))
	}
}
