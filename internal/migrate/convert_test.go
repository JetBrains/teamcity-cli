package migrate

import (
	"os"
	"strings"
	"testing"

	"github.com/JetBrains/teamcity-cli/internal/pipelineschema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertGitHubActions(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("testdata/github/ci.yml")
	require.NoError(t, err)

	cfg := CIConfig{Source: GitHubActions, File: ".github/workflows/ci.yml"}
	result, err := Convert(cfg, data, Options{})
	require.NoError(t, err)

	assert.Equal(t, "ci.tc.yml", result.OutputFile)
	assert.Equal(t, GitHubActions, result.Source)
	assert.Equal(t, 4, result.JobsConverted)
	assert.Greater(t, result.StepsConverted, 0)
	assert.GreaterOrEqual(t, len(result.Simplified), 10, "should simplify many steps")

	for _, want := range []string{
		"jobs:", "runs-on: Linux-Large", "type: script",
		"./gradlew jsBrowserProductionWebpack", "./gradlew jsTest",
		"files-publication:", "dependencies:", "- build", "- test_unit",
		"mkdir -p dist", "npx playwright test", "Deploy to GitHub Pages",
	} {
		assert.Contains(t, result.YAML, want)
	}
	assert.NotContains(t, result.YAML, "actions/checkout")
	assert.NotContains(t, result.YAML, "actions/setup-java")

	valErr := pipelineschema.ValidateWithSchema(result.YAML, pipelineschema.Bytes)
	assert.Empty(t, valErr, "generated YAML should validate against schema: %s", valErr)

}

func TestActionTransformers(t *testing.T) {
	t.Parallel()

	t.Run("registry lookup", func(t *testing.T) {
		t.Parallel()
		tests := []struct {
			action string
			found  bool
		}{
			{"my-org/custom-action@v1", false},
			// One entry per data table, plus a version ref containing "/".
			{"codecov/codecov-action@v3", true},
			{"azure/login@v2", true},
			{"dorny/paths-filter@v3", true},
			{"pypa/gh-action-pypi-publish@release/v1", true},
		}
		for _, tt := range tests {
			t.Run(tt.action, func(t *testing.T) {
				_, ok := LookupActionTransformer(tt.action)
				assert.Equal(t, tt.found, ok)
			})
		}
	})

	t.Run("script action converts to fixed step", func(t *testing.T) {
		t.Parallel()
		transformer, ok := LookupActionTransformer("JetBrains/qodana-action@v2025.1")
		require.True(t, ok)
		r := transformer("", nil)
		assert.Equal(t, StatusConverted, r.Status)
		require.Len(t, r.Steps, 1)
		assert.Equal(t, "Qodana", r.Steps[0].Name)
		assert.Contains(t, r.Steps[0].ScriptContent, "native Qodana integration")
	})

	t.Run("cache enables dependency cache", func(t *testing.T) {
		t.Parallel()
		transformer, _ := LookupActionTransformer("actions/cache@v3")
		r := transformer("", nil)
		assert.Equal(t, StatusSimplified, r.Status)
		assert.True(t, r.EnableDependencyCache)
	})

	t.Run("upload-artifact produces file publication", func(t *testing.T) {
		t.Parallel()
		transformer, _ := LookupActionTransformer("actions/upload-artifact@v4")
		r := transformer("", map[string]string{"path": "dist/**"})
		assert.Equal(t, StatusSimplified, r.Status)
		require.Len(t, r.Artifacts, 1)
		assert.Equal(t, "dist/**", r.Artifacts[0].Path)
	})

	t.Run("missing required inputs emit shell guards", func(t *testing.T) {
		t.Parallel()
		transformer, ok := LookupActionTransformer("azure/k8s-set-context@v4")
		require.True(t, ok)
		r := transformer("", map[string]string{})
		require.Len(t, r.Steps, 1)
		assert.Contains(t, r.Steps[0].ScriptContent, "${RESOURCE_GROUP:?")
		assert.Contains(t, r.Steps[0].ScriptContent, "${CLUSTER_NAME:?")
		assert.NotContains(t, r.Steps[0].ScriptContent, `""`, "no empty-string arguments")
	})

	t.Run("docker build-push", func(t *testing.T) {
		t.Parallel()
		transformer, _ := LookupActionTransformer("docker/build-push-action@v5")
		r := transformer("Build", map[string]string{
			"tags": "myapp:latest", "push": "true", "context": ".",
		})
		assert.Equal(t, StatusConverted, r.Status)
		require.Len(t, r.Steps, 1)
		assert.Contains(t, r.Steps[0].ScriptContent, "docker build")
		assert.Contains(t, r.Steps[0].ScriptContent, "docker push")
	})

	t.Run("docker build-push whitespace-only tags fall back to IMAGE guard", func(t *testing.T) {
		t.Parallel()
		transformer, _ := LookupActionTransformer("docker/build-push-action@v5")
		r := transformer("Build", map[string]string{"tags": " \n", "push": "true"})
		require.Len(t, r.Steps, 1)
		assert.Contains(t, r.Steps[0].ScriptContent, "${IMAGE:?")
	})
}

func TestUnknownActionMultilineInputCommented(t *testing.T) {
	t.Parallel()
	r := Unknown("acme/dangerous@v1", map[string]string{
		"note": "hello\nrm -rf tmp",
	})
	require.Len(t, r.Steps, 1)
	assert.NotContains(t, r.Steps[0].ScriptContent, "\nrm -rf tmp", "multiline input line 2 must be commented, not executable")
}

func TestGHReleaseMultilineFilesNotInjected(t *testing.T) {
	t.Parallel()

	transformer, ok := LookupActionTransformer("softprops/action-gh-release@v2")
	require.True(t, ok)
	r := transformer("Release", map[string]string{
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
	cfg := CIConfig{Source: GitHubActions, File: ".github/workflows/ci.yml"}
	result, err := Convert(cfg, []byte(wf), Options{})
	require.NoError(t, err)

	manuals := strings.Join(result.ManualSetup, "\n")
	// Windows step warns; the Linux step must not, so exactly one warning is expected.
	assert.Equal(t, 1, strings.Count(manuals, "runs on a Windows runner with no explicit shell"))
}

func TestGHAParseErrorsSurfacedInNeedsReview(t *testing.T) {
	t.Parallel()

	wf := `name: ci
on: push
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - name: broken
        env:
          FOO: bar
      - run: echo hi
`
	cfg := CIConfig{Source: GitHubActions, File: ".github/workflows/ci.yml"}
	result, err := Convert(cfg, []byte(wf), Options{})
	require.NoError(t, err)

	// A step with neither run: nor uses: must not silently produce a clean report.
	require.NotEmpty(t, result.NeedsReview)
	review := strings.Join(result.NeedsReview, "\n")
	assert.Contains(t, review, "Workflow parse error at line 7")
	assert.Contains(t, review, `step must run script with "run" section or run action with "uses" section`)
	assert.Contains(t, review, `Step "broken" has neither run: nor uses: → dropped from output; rewrite it as a script step manually`)
}

func TestGHAParseErrorsCapped(t *testing.T) {
	t.Parallel()

	wf := `name: ci
on: push
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - name: b1
      - name: b2
      - name: b3
      - name: b4
      - name: b5
`
	cfg := CIConfig{Source: GitHubActions, File: ".github/workflows/ci.yml"}
	result, err := Convert(cfg, []byte(wf), Options{})
	require.NoError(t, err)

	review := strings.Join(result.NeedsReview, "\n")
	assert.Equal(t, 3, strings.Count(review, "Workflow parse error at line"))
	assert.Contains(t, review, "...and 2 more parse errors → review the source workflow")
}

func TestGHAJobLevelFeaturesFlagged(t *testing.T) {
	t.Parallel()

	wf := `name: ci
on: push
jobs:
  build:
    runs-on: ubuntu-latest
    timeout-minutes: 30
    continue-on-error: true
    outputs:
      version: v1
    steps:
      - name: compile
        run: echo hi
        timeout-minutes: 5
`
	cfg := CIConfig{Source: GitHubActions, File: ".github/workflows/ci.yml"}
	result, err := Convert(cfg, []byte(wf), Options{})
	require.NoError(t, err)

	manuals := strings.Join(result.ManualSetup, "\n")
	assert.Contains(t, manuals, `Job "build" sets timeout-minutes: 30 → configure an execution timeout in TeamCity failure conditions`)
	assert.Contains(t, manuals, `Job "build" has continue-on-error: true → its failure must not fail the pipeline; relax dependency failure conditions in TeamCity`)
	assert.Contains(t, manuals, `Job "build" defines outputs (version) → expose them as TeamCity output parameters and rewire consumers of needs.build.outputs.<name> to %dep.build.<param>%`)
	assert.Contains(t, manuals, `Step "compile" sets timeout-minutes: 5 → no per-step timeout in TeamCity; configure an execution timeout in the job's failure conditions`)
}

func TestGHAWindowsActionTransformerWarned(t *testing.T) {
	t.Parallel()

	wf := `name: ci
on: push
jobs:
  win:
    runs-on: windows-latest
    steps:
      - name: Build image
        uses: docker/build-push-action@v5
        with:
          tags: myapp:latest
          push: 'true'
  lin:
    runs-on: ubuntu-latest
    steps:
      - name: Build image linux
        uses: docker/build-push-action@v5
        with:
          tags: myapp:latest
`
	cfg := CIConfig{Source: GitHubActions, File: ".github/workflows/ci.yml"}
	result, err := Convert(cfg, []byte(wf), Options{})
	require.NoError(t, err)

	manuals := strings.Join(result.ManualSetup, "\n")
	// Only the Windows job's converted action step warns, so exactly one note is expected.
	assert.Equal(t, 1, strings.Count(manuals, "emits a POSIX shell script on a Windows runner"))
	assert.Contains(t, manuals, `Step "Build image" converted from "docker/build-push-action@v5" emits a POSIX shell script on a Windows runner → provide Git Bash/WSL on the agent or rewrite for cmd/PowerShell`)
}

func TestMatrixRunsOnEmitsDefaultRunner(t *testing.T) {
	t.Parallel()

	wf := `name: ci
on: push
jobs:
  build:
    strategy:
      matrix:
        os: [ubuntu-latest, macos-latest]
    runs-on: ${{ matrix.os }}
    steps:
      - run: echo hi
`
	cfg := CIConfig{Source: GitHubActions, File: ".github/workflows/ci.yml"}
	result, err := Convert(cfg, []byte(wf), Options{})
	require.NoError(t, err)

	// A matrix expression must not silently drop runs-on — emit the default runner and flag it.
	assert.Contains(t, result.YAML, "runs-on: Linux-Large")
	assert.Contains(t, strings.Join(result.ManualSetup, "\n"), "runs-on uses expression")
}

func TestSelfHostedStyleRunnerLabels(t *testing.T) {
	t.Parallel()

	wf := `name: ci
on: push
jobs:
  gpu:
    runs-on: my-gpu-box
    steps:
      - run: echo hi
  std:
    runs-on: [self-hosted, linux, x64]
    steps:
      - run: echo hi
`
	cfg := CIConfig{Source: GitHubActions, File: ".github/workflows/ci.yml"}
	result, err := Convert(cfg, []byte(wf), Options{})
	require.NoError(t, err)

	assert.Contains(t, result.YAML, "runs-on: self-hosted")
	assert.NotContains(t, result.YAML, "runs-on: my-gpu-box")
	assert.Contains(t, strings.Join(result.ManualSetup, "\n"), `Job "gpu" runs-on "my-gpu-box" is not a GitHub-hosted runner`)
}

func TestWorkflowCallPreservesInputsAndSecrets(t *testing.T) {
	t.Parallel()

	wf := `name: publish
on: push
jobs:
  publish-openapi:
    uses: hmcts/workflow-publish-openapi-spec/.github/workflows/publish-openapi.yml@v1
    with:
      test_to_run: 'uk.gov.hmcts.dm.openapi.OpenAPIPublisherTest'
      java_version: 21
    secrets:
      SWAGGER_PUBLISHER_API_TOKEN: ${{ secrets.SWAGGER_PUBLISHER_API_TOKEN }}
  inherit-job:
    uses: org/repo/.github/workflows/other.yml@v2
    secrets: inherit
`
	cfg := CIConfig{Source: GitHubActions, File: ".github/workflows/publish.yml"}
	result, err := Convert(cfg, []byte(wf), Options{})
	require.NoError(t, err)

	for _, want := range []string{
		"test_to_run: uk.gov.hmcts.dm.openapi.OpenAPIPublisherTest",
		"java_version: 21",
		"swagger_publisher_api_token",
	} {
		assert.Contains(t, strings.ToLower(result.YAML), strings.ToLower(want))
	}
	manuals := strings.Join(result.ManualSetup, "\n")
	assert.Contains(t, manuals, "Secret SWAGGER_PUBLISHER_API_TOKEN")
	assert.Contains(t, manuals, "secrets: inherit")
}

func TestMultilineIfCondensedInManualSetup(t *testing.T) {
	t.Parallel()

	wf := `name: ci
on: push
jobs:
  pages:
    runs-on: ubuntu-latest
    steps:
      - name: GitHub Pages action
        if: |
          github.ref == 'refs/heads/master' &&
          matrix.java == 21
        run: echo deploy
`
	cfg := CIConfig{Source: GitHubActions, File: ".github/workflows/ci.yml"}
	result, err := Convert(cfg, []byte(wf), Options{})
	require.NoError(t, err)

	for _, item := range result.ManualSetup {
		assert.NotContains(t, item, "\n", "manual-setup items must stay on one line: %q", item)
	}
	assert.Contains(t, strings.Join(result.ManualSetup, "\n"), "github.ref == 'refs/heads/master' && matrix.java == 21")
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
		assert.Equal(t, tt.want, MapGHAExpressions(tt.input))
	}
}
