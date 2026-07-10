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

func TestEmptyRunnerMapOmitsRunsOn(t *testing.T) {
	t.Parallel()

	wf := "on: push\njobs:\n  test:\n    runs-on: ubuntu-latest\n    steps:\n      - run: echo hi\n"
	result, err := Convert(CIConfig{Source: GitHubActions, File: ".github/workflows/ci.yml"}, []byte(wf), Options{RunnerMap: map[string]string{}})
	require.NoError(t, err)

	assert.NotContains(t, result.YAML, "runs-on:")
}

func TestSetupJavaPinsJavaHome(t *testing.T) {
	t.Parallel()

	wf := `on: push
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/setup-java@v5
        with:
          distribution: temurin
          java-version: "17"
      - run: ./mvnw verify
`
	result, err := Convert(CIConfig{Source: GitHubActions, File: ".github/workflows/ci.yml"}, []byte(wf), Options{})
	require.NoError(t, err)

	assert.Contains(t, result.YAML, `env.JAVA_HOME: "%env.JDK_17_0%"`)
	assert.Contains(t, result.YAML, "./mvnw verify")
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
