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

func TestConvertJenkinsWithoutAPI(t *testing.T) {
	t.Parallel()

	cfg := migrate.CIConfig{Source: migrate.Jenkins, File: "Jenkinsfile"}
	result, err := migrate.Convert(cfg, []byte("pipeline { agent any }"), migrate.Options{})
	require.NoError(t, err)

	assert.Contains(t, result.YAML, "JENKINS_URL")
	assert.Contains(t, result.YAML, "Jenkins API")
	assert.Greater(t, len(result.NeedsReview), 0)

	valErr := pipelineschema.Validate(result.YAML)
	assert.Empty(t, valErr, "generated YAML should validate: %s", valErr)
}

func TestConvertGitLabCI(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("testdata/gitlab/.gitlab-ci.yml")
	require.NoError(t, err)

	cfg := migrate.CIConfig{Source: migrate.GitLabCI, File: ".gitlab-ci.yml"}
	result, err := migrate.Convert(cfg, data, migrate.Options{})
	require.NoError(t, err)

	assert.GreaterOrEqual(t, result.JobsConverted, 4)
	assert.Greater(t, result.StepsConverted, 0)
	for _, want := range []string{"npm ci", "npm run build", "npm run test", "files-publication", "dist/", "dependencies:"} {
		assert.Contains(t, result.YAML, want)
	}

	assertManualSetupContains(t, result, "postgres", "redis")
	assertManualSetupContains(t, result, "merge_request_event", "rule")
	assertManualSetupContains(t, result, "allow_failure")
	assertManualSetupContains(t, result, "staging", "environment")

	assert.Empty(t, pipelineschema.Validate(result.YAML))
}

func TestConvertCircleCI(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("testdata/circleci/config.yml")
	require.NoError(t, err)

	cfg := migrate.CIConfig{Source: migrate.CircleCI, File: ".circleci/config.yml"}
	result, err := migrate.Convert(cfg, data, migrate.Options{})
	require.NoError(t, err)

	assert.GreaterOrEqual(t, result.JobsConverted, 3)
	assert.Greater(t, result.StepsConverted, 0)
	for _, want := range []string{"npm ci", "npm run build", "npm test", "dependencies:"} {
		assert.Contains(t, result.YAML, want)
	}

	assertSimplifiedContains(t, result, "cache")
	assertSimplifiedContains(t, result, "checkout")
	assertManualSetupContains(t, result, "production-secrets", "context")

	assert.Empty(t, pipelineschema.Validate(result.YAML))
}

func TestConvertAzureDevOps(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("testdata/azure/azure-pipelines.yml")
	require.NoError(t, err)

	cfg := migrate.CIConfig{Source: migrate.AzureDevOps, File: "azure-pipelines.yml"}
	result, err := migrate.Convert(cfg, data, migrate.Options{})
	require.NoError(t, err)

	assert.GreaterOrEqual(t, result.JobsConverted, 2)
	assert.Greater(t, result.StepsConverted, 0)
	for _, want := range []string{"dotnet restore", "dotnet build", "dotnet test"} {
		assert.Contains(t, result.YAML, want)
	}
	assertManualSetupContains(t, result, "PublishTestResults", "test report")
	assertManualSetupContains(t, result, "trigger", "branch")

	assert.Empty(t, pipelineschema.Validate(result.YAML))
}

func TestConvertTravisCI(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("testdata/travis/.travis.yml")
	require.NoError(t, err)

	cfg := migrate.CIConfig{Source: migrate.TravisCI, File: ".travis.yml"}
	result, err := migrate.Convert(cfg, data, migrate.Options{})
	require.NoError(t, err)

	assert.GreaterOrEqual(t, result.JobsConverted, 4)
	assert.Greater(t, result.StepsConverted, 0)
	for _, want := range []string{"golangci-lint", "go test", "go build", "enable-dependency-cache: true", "apt-get", "protobuf-compiler"} {
		assert.Contains(t, result.YAML, want)
	}
	assertManualSetupContains(t, result, "docker", "redis")

	foundDeploy := false
	for _, m := range append(result.ManualSetup, result.NeedsReview...) {
		if strings.Contains(m, "release") || strings.Contains(m, "deploy") || strings.Contains(m, "Deploy") {
			foundDeploy = true
			break
		}
	}
	assert.True(t, foundDeploy, "should flag deploy providers")

	assert.Empty(t, pipelineschema.Validate(result.YAML))
}

func TestConvertBitbucket(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("testdata/bitbucket/bitbucket-pipelines.yml")
	require.NoError(t, err)

	cfg := migrate.CIConfig{Source: migrate.Bitbucket, File: "bitbucket-pipelines.yml"}
	result, err := migrate.Convert(cfg, data, migrate.Options{})
	require.NoError(t, err)

	assert.GreaterOrEqual(t, result.JobsConverted, 3)
	assert.Greater(t, result.StepsConverted, 0)
	for _, want := range []string{"npm ci", "npm run build", "Unit Tests", "Lint", "aws-s3-deploy"} {
		assert.Contains(t, result.YAML, want)
	}
	assertManualSetupContains(t, result, "node:20")

	assert.Empty(t, pipelineschema.Validate(result.YAML))
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

func TestMapVariables(t *testing.T) {
	t.Parallel()

	t.Run("GitHub Actions expressions", func(t *testing.T) {
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
	})

	t.Run("Jenkins variables", func(t *testing.T) {
		t.Parallel()
		tests := []struct{ input, want string }{
			{"${BUILD_ID}", "%%teamcity.build.id%%"},
			{"${BUILD_NUMBER}", "%%build.number%%"},
			{"${WORKSPACE}", "%%teamcity.build.checkoutDir%%"},
			{"${GIT_COMMIT}", "%%build.vcs.number%%"},
			{"${env.MY_VAR}", "$MY_VAR"},
			{"${env.BUILD_NUMBER}", "%%build.number%%"},
		}
		for _, tt := range tests {
			assert.Equal(t, tt.want, migrate.MapJenkinsVars(tt.input))
		}
	})
}

func assertManualSetupContains(t *testing.T, result *migrate.ConversionResult, keywords ...string) {
	t.Helper()
	for _, m := range result.ManualSetup {
		for _, kw := range keywords {
			if strings.Contains(m, kw) {
				return
			}
		}
	}
	t.Errorf("ManualSetup should contain one of %v, got: %v", keywords, result.ManualSetup)
}

func assertSimplifiedContains(t *testing.T, result *migrate.ConversionResult, keywords ...string) {
	t.Helper()
	for _, s := range result.Simplified {
		for _, kw := range keywords {
			if strings.Contains(s, kw) {
				return
			}
		}
	}
	t.Errorf("Simplified should contain one of %v, got: %v", keywords, result.Simplified)
}
