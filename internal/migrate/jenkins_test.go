package migrate

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/JetBrains/teamcity-cli/internal/pipelineschema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sampleJenkinsAST = `{
  "data": {
    "result": "success",
    "json": {
      "pipeline": {
        "agent": {"type": "docker", "arguments": [{"key": "image", "value": {"value": "maven:3.9-eclipse-temurin-17"}}]},
        "environment": [
          {"key": "REGISTRY", "value": {"isLiteral": true, "value": "ghcr.io"}},
          {"key": "APP_NAME", "value": {"isLiteral": true, "value": "my-service"}},
          {"key": "CREDS", "value": {"isLiteral": false, "value": "credentials('deploy-key')"}}
        ],
        "parameters": {
          "parameters": [
            {"name": "string", "arguments": [{"key": "name", "value": "DEPLOY_ENV"}, {"key": "defaultValue", "value": "staging"}]}
          ]
        },
        "stages": [
          {
            "name": "Build",
            "branches": [{"name": "default", "steps": [
              {"name": "sh", "arguments": [{"value": {"isLiteral": true, "value": "mvn clean compile -B"}}]},
              {"name": "sh", "arguments": [{"value": {"isLiteral": true, "value": "mvn package -DskipTests -B"}}]}
            ]}]
          },
          {
            "name": "Test",
            "branches": [{"name": "default", "steps": [
              {"name": "sh", "arguments": [{"value": {"isLiteral": true, "value": "mvn test -B"}}]},
              {"name": "junit", "arguments": [{"value": {"isLiteral": true, "value": "target/surefire-reports/*.xml"}}]}
            ]}]
          },
          {
            "name": "Parallel QA",
            "parallel": [
              {"name": "Lint", "branches": [{"name": "default", "steps": [
                {"name": "sh", "arguments": [{"value": {"isLiteral": true, "value": "mvn checkstyle:check"}}]}
              ]}]},
              {"name": "Security", "branches": [{"name": "default", "steps": [
                {"name": "sh", "arguments": [{"value": {"isLiteral": true, "value": "mvn dependency-check:check"}}]}
              ]}]}
            ]
          },
          {
            "name": "Docker Build",
            "branches": [{"name": "default", "steps": [
              {"name": "sh", "arguments": [{"value": {"isLiteral": true, "value": "docker build -t $REGISTRY/$APP_NAME:$BUILD_NUMBER ."}}]},
              {"name": "sh", "arguments": [{"value": {"isLiteral": true, "value": "docker push $REGISTRY/$APP_NAME:$BUILD_NUMBER"}}]}
            ]}]
          },
          {
            "name": "Deploy",
            "when": {"conditions": [{"name": "branch", "arguments": {"value": "main"}}]},
            "branches": [{"name": "default", "steps": [
              {"name": "withCredentials", "arguments": [{"value": {"value": "[usernamePassword(credentialsId: 'deploy-creds')]"}}], "children": [
                {"name": "sh", "arguments": [{"value": {"isLiteral": true, "value": "kubectl set image deployment/$APP_NAME $APP_NAME=$REGISTRY/$APP_NAME:$BUILD_NUMBER"}}]}
              ]},
              {"name": "archiveArtifacts", "arguments": [{"key": "artifacts", "value": "deploy-report.html"}]}
            ]}],
            "post": {"conditions": [
              {"condition": "success", "branch": {"steps": [
                {"name": "slackSend", "arguments": [{"key": "channel", "value": "#deploys"}, {"key": "message", "value": "Deployed!"}]}
              ]}}
            ]}
          }
        ],
        "post": {
          "conditions": [
            {"condition": "always", "branch": {"steps": [{"name": "cleanWs", "arguments": []}]}},
            {"condition": "failure", "branch": {"steps": [{"name": "mail", "arguments": [{"key": "to", "value": "team@example.com"}]}]}}
          ]
        }
      }
    }
  }
}`

func TestConvertFromAST(t *testing.T) {
	t.Parallel()

	var resp jenkinsConverterResponse
	require.NoError(t, json.Unmarshal([]byte(sampleJenkinsAST), &resp))
	ast := &resp.Data.JSON.Pipeline

	cfg := CIConfig{Source: Jenkins, File: "Jenkinsfile"}
	result := NewResult(cfg)
	pipeline := convertJenkinsAST(ast, cfg, result)

	yaml := pipeline.String()

	assert.GreaterOrEqual(t, len(pipeline.Jobs), 6, "should have at least 6 jobs")

	assert.Contains(t, yaml, "mvn clean compile")
	assert.Contains(t, yaml, "mvn test")
	assert.Contains(t, yaml, "mvn checkstyle:check")
	assert.Contains(t, yaml, "docker build")
	assert.Contains(t, yaml, "kubectl set image")

	lintIdx := -1
	securityIdx := -1
	for i, j := range pipeline.Jobs {
		if j.Name == "Lint" {
			lintIdx = i
		}
		if j.Name == "Security" {
			securityIdx = i
		}
	}
	require.NotEqual(t, -1, lintIdx, "should have Lint job")
	require.NotEqual(t, -1, securityIdx, "should have Security job")
	assert.Equal(t, pipeline.Jobs[lintIdx].Dependencies, pipeline.Jobs[securityIdx].Dependencies,
		"parallel stages should have same dependencies")

	dockerIdx := -1
	for i, j := range pipeline.Jobs {
		if j.Name == "Docker Build" {
			dockerIdx = i
		}
	}
	require.NotEqual(t, -1, dockerIdx)
	assert.GreaterOrEqual(t, len(pipeline.Jobs[dockerIdx].Dependencies), 2,
		"Docker Build should depend on all parallel stages")

	assert.Contains(t, yaml, "env.REGISTRY")
	assert.Contains(t, yaml, "ghcr.io")
	assert.Contains(t, yaml, "env.APP_NAME")

	foundCred := false
	for _, m := range result.ManualSetup {
		if strings.Contains(m, "credential") && strings.Contains(m, "deploy-key") {
			foundCred = true
			break
		}
	}
	assert.True(t, foundCred, "should flag credential binding")

	assert.Contains(t, yaml, "files-publication")
	assert.Contains(t, yaml, "deploy-report.html")

	foundJunit := false
	for _, m := range result.ManualSetup {
		if strings.Contains(m, "junit") {
			foundJunit = true
			break
		}
	}
	assert.True(t, foundJunit, "should flag junit step")

	foundCleanup := false
	foundMail := false
	for _, m := range result.Simplified {
		if strings.Contains(m, "cleanWs") {
			foundCleanup = true
		}
	}
	for _, m := range result.ManualSetup {
		if strings.Contains(m, "email") || strings.Contains(m, "mail") {
			foundMail = true
		}
	}
	assert.True(t, foundCleanup, "should simplify cleanWs")
	assert.True(t, foundMail, "should flag mail notification")

	foundSlack := false
	for _, m := range result.ManualSetup {
		if strings.Contains(m, "Slack") || strings.Contains(m, "slack") {
			foundSlack = true
			break
		}
	}
	assert.True(t, foundSlack, "should flag slackSend in Deploy post")

	foundWhen := false
	for _, m := range result.ManualSetup {
		if strings.Contains(m, "branch main") || strings.Contains(m, "when") {
			foundWhen = true
			break
		}
	}
	assert.True(t, foundWhen, "should flag when condition")

	foundDockerAgent := false
	for _, m := range result.ManualSetup {
		if strings.Contains(m, "maven:3.9") {
			foundDockerAgent = true
			break
		}
	}
	assert.True(t, foundDockerAgent, "should flag Docker agent image")

	assert.Contains(t, yaml, "%%build.number%%", "should map $BUILD_NUMBER to TC parameter")

	valErr := pipelineschema.Validate(yaml)
	assert.Empty(t, valErr, "generated YAML should validate: %s", valErr)
}

func TestConvertFromASTCheckoutSimplified(t *testing.T) {
	t.Parallel()

	ast := &pipelineAST{
		Agent: &agentAST{Type: "any"},
		Stages: []stageAST{{
			Name: "Build",
			Branches: []branchAST{{
				Name: "default",
				Steps: []stepAST{
					{Name: "checkout", Arguments: json.RawMessage(`[{"value": {"value": "scm"}}]`)},
					{Name: "sh", Arguments: json.RawMessage(`[{"value": {"isLiteral": true, "value": "make build"}}]`)},
				},
			}},
		}},
	}

	cfg := CIConfig{Source: Jenkins, File: "Jenkinsfile"}
	result := NewResult(cfg)
	pipeline := convertJenkinsAST(ast, cfg, result)

	assert.Len(t, pipeline.Jobs, 1)
	assert.Len(t, pipeline.Jobs[0].Steps, 1, "checkout should be simplified, only sh step remains")
	assert.Contains(t, pipeline.Jobs[0].Steps[0].ScriptContent, "make build")
	assert.Contains(t, result.Simplified, "checkout (TeamCity VCS checkout is automatic)")
}
