package migrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JetBrains/teamcity-cli/internal/pipelineschema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	assert.Contains(t, yaml, "env.greeting:")
	assert.Contains(t, yaml, "env.release_channel:")
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
