package migrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JetBrains/teamcity-cli/internal/pipelineschema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestBambooDetectFromFixture(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	specs := filepath.Join(dir, "bamboo-specs")
	require.NoError(t, os.MkdirAll(specs, 0755))

	data, err := os.ReadFile("testdata/bamboo/bamboo.yml")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(specs, "bamboo.yml"), data, 0644))

	configs, err := Detect(dir, "")
	require.NoError(t, err)
	require.Len(t, configs, 1)

	cfg := configs[0]
	assert.Equal(t, Bamboo, cfg.Source)
	assert.Equal(t, "bamboo-specs/bamboo.yml", cfg.File)
	assert.Equal(t, 4, cfg.Jobs, "Build + Unit Tests + Integration Tests + Deploy")
	assert.Greater(t, cfg.Steps, 0)
	assert.Contains(t, cfg.Features, "manual-stage")
	assert.Contains(t, cfg.Features, "tests")
	assert.Contains(t, cfg.Features, "triggers")
	assert.Contains(t, cfg.Features, "variables")
	assert.Contains(t, cfg.Features, "artifacts")
	assert.Contains(t, cfg.Features, "aws-deploy")
}

func TestBambooConvertFixture(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("testdata/bamboo/bamboo.yml")
	require.NoError(t, err)

	cfg := CIConfig{Source: Bamboo, File: "bamboo-specs/bamboo.yml"}
	result, err := Convert(cfg, data, Options{})
	require.NoError(t, err)
	require.NotNil(t, result.Pipeline)

	yaml := result.YAML
	assert.Contains(t, yaml, "Build_Build:")
	assert.Contains(t, yaml, "Test_Unit_Tests:")
	assert.Contains(t, yaml, "Test_Integration_Tests:")
	assert.Contains(t, yaml, "Deploy_Deploy:")
	assert.Contains(t, yaml, "dependencies:\n      - Build_Build")
	assert.Contains(t, yaml, "greeting:")
	assert.Contains(t, yaml, "release_channel:")
	assert.NotContains(t, yaml, "env.greeting:", "plan variables are config params, not env vars")
	assert.Contains(t, yaml, "%build.number%")
	assert.Contains(t, yaml, "%teamcity.build.branch%")

	manuals := strings.Join(result.ManualSetup, "\n")
	assert.Contains(t, manuals, "Stage \"Deploy\" is manual")
	assert.Contains(t, manuals, "Triggers (polling, cron)")
	assert.Contains(t, manuals, "branch policy")
	assert.Contains(t, manuals, "AWS CodeDeploy")
	assert.Contains(t, manuals, "final-task")

	simplified := strings.Join(result.Simplified, "\n")
	assert.Contains(t, simplified, "checkout")

	assert.Empty(t, pipelineschema.Validate(yaml))
}

func TestBambooCollidingJobIDsStayDistinctInDeps(t *testing.T) {
	t.Parallel()

	// "a-b" and "a.b" both sanitize to "S_a_b"; a dependent job must depend on both
	// distinct keys, not collapse onto the first.
	spec := `---
version: 2
plan:
  project-key: P
  key: K
  name: Plan
stages:
  - 'S':
      jobs:
        - 'a-b'
        - 'a.b'
  - 'T':
      jobs:
        - 'c'
'a-b':
  tasks:
    - script:
        - echo a
'a.b':
  tasks:
    - script:
        - echo b
'c':
  tasks:
    - script:
        - echo c
`
	cfg := CIConfig{Source: Bamboo, File: "bamboo-specs/bamboo.yml"}
	result, err := Convert(cfg, []byte(spec), Options{})
	require.NoError(t, err)

	var doc struct {
		Jobs map[string]struct {
			Dependencies []string `yaml:"dependencies"`
		} `yaml:"jobs"`
	}
	require.NoError(t, yaml.Unmarshal([]byte(result.YAML), &doc))
	assert.ElementsMatch(t, []string{"S_a_b", "S_a_b_2"}, doc.Jobs["T_c"].Dependencies)
}

func TestBambooScriptShorthand(t *testing.T) {
	t.Parallel()

	yaml := `---
version: 2
plan:
  project-key: P
  key: K
  name: Plan
stages:
  - 'Stage':
      jobs:
        - Job
Job:
  tasks:
    - script:
        - echo one
        - echo two
`
	cfg := CIConfig{Source: Bamboo, File: "bamboo-specs/bamboo.yml"}
	result, err := Convert(cfg, []byte(yaml), Options{})
	require.NoError(t, err)
	require.Len(t, result.Pipeline.Jobs, 1)
	require.Len(t, result.Pipeline.Jobs[0].Steps, 1)
	step := result.Pipeline.Jobs[0].Steps[0]
	assert.Contains(t, step.ScriptContent, "echo one")
	assert.Contains(t, step.ScriptContent, "echo two")
}

func TestBambooMavenFullForm(t *testing.T) {
	t.Parallel()

	yaml := `---
version: 2
plan:
  project-key: P
  key: K
  name: Plan
stages:
  - 'Build':
      jobs:
        - Job
Job:
  tasks:
    - maven:
        goal: clean install
        project-file: pom.xml
        jdk: 'JDK 17'
        tests: 'true'
`
	cfg := CIConfig{Source: Bamboo, File: "bamboo-specs/bamboo.yml"}
	result, err := Convert(cfg, []byte(yaml), Options{})
	require.NoError(t, err)
	require.Len(t, result.Pipeline.Jobs[0].Steps, 1)
	step := result.Pipeline.Jobs[0].Steps[0]
	assert.Contains(t, step.ScriptContent, "mvn -f pom.xml clean install")

	manuals := strings.Join(result.ManualSetup, "\n")
	assert.Contains(t, manuals, `JDK "JDK 17"`)
	assert.Contains(t, manuals, "surefire reports")
}

func TestBambooUnknownTaskBecomesStub(t *testing.T) {
	t.Parallel()

	yaml := `---
version: 2
plan:
  project-key: P
  key: K
  name: Plan
stages:
  - 'Build':
      jobs:
        - Job
Job:
  tasks:
    - made-up-task:
        config: 1
`
	cfg := CIConfig{Source: Bamboo, File: "bamboo-specs/bamboo.yml"}
	result, err := Convert(cfg, []byte(yaml), Options{})
	require.NoError(t, err)
	require.NotEmpty(t, result.NeedsReview)

	require.Len(t, result.Pipeline.Jobs[0].Steps, 1)
	step := result.Pipeline.Jobs[0].Steps[0]
	assert.Contains(t, step.ScriptContent, "TODO: implement equivalent of made-up-task")
	assert.Contains(t, step.ScriptContent, "config: 1")
}

func TestBambooNoPlanSurfacesAsReview(t *testing.T) {
	t.Parallel()

	yaml := `---
version: 2
deployment:
  name: Deploy Plan
environments:
  - production
production:
  tasks: []
`
	cfg := CIConfig{Source: Bamboo, File: "bamboo-specs/deployment.yaml"}
	result, err := Convert(cfg, []byte(yaml), Options{})
	require.NoError(t, err)
	assert.NotEmpty(t, result.NeedsReview)
	assert.Contains(t, strings.Join(result.NeedsReview, "\n"), "no top-level `plan:`")
}

func TestMapBambooExpressions(t *testing.T) {
	t.Parallel()

	cases := []struct{ in, want string }{
		{"v${bamboo.build.number}", "v%build.number%"},
		{"branch=${bamboo.repository.branch.name}", "branch=%teamcity.build.branch%"},
		{"custom=${bamboo.my_custom_var}", "custom=%my_custom_var%"},
		{"shell=$HOME ${bamboo.build.number}", "shell=$HOME %build.number%"},
		{"echo ${HOME}", "echo ${HOME}"},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, MapBambooExpressions(c.in))
	}
}

func TestBambooArtifactPattern(t *testing.T) {
	t.Parallel()

	yaml := `---
version: 2
plan:
  project-key: P
  key: K
  name: Plan
stages:
  - 'Build':
      jobs:
        - Job
Job:
  tasks:
    - script:
        - make
  artifacts:
    - name: jar
      pattern: '*.jar'
      location: target
      shared: true
    - name: log
      pattern: 'build.log'
`
	cfg := CIConfig{Source: Bamboo, File: "bamboo-specs/bamboo.yml"}
	result, err := Convert(cfg, []byte(yaml), Options{})
	require.NoError(t, err)
	require.Len(t, result.Pipeline.Jobs, 1)
	pubs := result.Pipeline.Jobs[0].FilesPublication
	require.Len(t, pubs, 2)
	assert.Equal(t, "target/*.jar", pubs[0].Path)
	assert.True(t, pubs[0].ShareWithJobs)
	assert.False(t, pubs[0].PublishArtifact)
	assert.Equal(t, "build.log", pubs[1].Path)
	assert.True(t, pubs[1].PublishArtifact)
}

func TestBambooSecretPlanVarRedacted(t *testing.T) {
	t.Parallel()

	yaml := `---
version: 2
plan:
  project-key: P
  key: K
  name: Plan
stages:
  - 'Build':
      jobs:
        - Job
Job:
  tasks:
    - script:
        - echo hi
variables:
  api_token: super-secret-value
  db_password: hunter2
  greeting: hello
`
	cfg := CIConfig{Source: Bamboo, File: "bamboo-specs/bamboo.yml"}
	result, err := Convert(cfg, []byte(yaml), Options{})
	require.NoError(t, err)

	out := result.YAML
	assert.NotContains(t, out, "super-secret-value", "secret value must not leak into generated YAML")
	assert.NotContains(t, out, "hunter2", "secret value must not leak into generated YAML")
	assert.Contains(t, out, "api_token:")
	assert.Contains(t, out, "db_password:")
	assert.Contains(t, out, "greeting: \"hello\"", "non-secret values pass through unchanged")
	assert.Contains(t, out, bambooSecretPlaceholder)

	manuals := strings.Join(result.ManualSetup, "\n")
	assert.Contains(t, manuals, `Variable "api_token" looks like a secret`)
	assert.Contains(t, manuals, `Variable "db_password" looks like a secret`)
}

func TestBambooEnvParamsRedactsSecrets(t *testing.T) {
	t.Parallel()
	result := &ConversionResult{}
	params := bambooEnvParams(map[string]any{
		"API_TOKEN": "leaked-token",
		"LOG_LEVEL": "debug",
	}, result, `Job "X"`)
	assert.Equal(t, bambooSecretPlaceholder, params["API_TOKEN"])
	assert.Equal(t, "debug", params["LOG_LEVEL"])
	assert.NotEmpty(t, result.ManualSetup)
}

func TestBambooWorkingDirExpressionMapped(t *testing.T) {
	t.Parallel()

	yaml := `---
version: 2
plan:
  project-key: P
  key: K
  name: Plan
stages:
  - 'Build':
      jobs:
        - Job
Job:
  tasks:
    - script:
        scripts:
          - make
        working-dir: ${bamboo.working.directory}/sub
`
	cfg := CIConfig{Source: Bamboo, File: "bamboo-specs/bamboo.yml"}
	result, err := Convert(cfg, []byte(yaml), Options{})
	require.NoError(t, err)
	require.Len(t, result.Pipeline.Jobs[0].Steps, 1)
	assert.Equal(t, "%teamcity.build.checkoutDir%/sub", result.Pipeline.Jobs[0].Steps[0].WorkingDirectory)
}
