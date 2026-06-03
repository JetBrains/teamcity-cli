package job_test

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJobStepList(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)

	out := cmdtest.CaptureOutput(T, ts.Factory, "job", "step", "list", testJob)
	assert.Contains(T, out, "Compile")
	assert.Contains(T, out, "Run Tests")
	assert.Contains(T, out, "gradle")

	cmdtest.RunCmdWithFactory(T, ts.Factory, "job", "step", "list", testJob, "--json")
	cmdtest.RunCmdWithFactory(T, ts.Factory, "job", "step", "list", testJob, "--plain")
}

func TestJobStepView(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)

	out := cmdtest.CaptureOutput(T, ts.Factory, "job", "step", "view", testJob, "RUNNER_1")
	assert.Contains(T, out, "ID:")
	assert.Contains(T, out, "Type:")

	cmdtest.RunCmdWithFactory(T, ts.Factory, "job", "step", "view", testJob, "RUNNER_1", "--json")
}

func TestJobStepViewWeb(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)

	out := cmdtest.CaptureOutput(t, ts.Factory, "job", "step", "view", testJob, "RUNNER_1", "--web")
	assert.Contains(t, out, ts.URL+"/admin/editBuildRunners.html?id=buildType:"+testJob)
}

func TestJobStepAdd(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)

	var captured []byte
	ts.Handle("POST /app/rest/buildTypes/id:TestProject_Build/steps", func(w http.ResponseWriter, r *http.Request) {
		captured, _ = io.ReadAll(r.Body)
		cmdtest.JSON(w, api.BuildStep{ID: "RUNNER_3", Name: "Run Tests", Type: "commandLine"})
	})

	out := cmdtest.CaptureOutput(T, ts.Factory, "job", "step", "add", testJob,
		"--type", "commandLine", "--name", "Run Tests", "--param", "script.content=./gradlew test")
	assert.Contains(T, out, "Added step")
	assert.Contains(T, out, "RUNNER_3")

	var payload api.BuildStep
	require.NoError(T, json.Unmarshal(captured, &payload))
	assert.Equal(T, "commandLine", payload.Type)
	assert.Equal(T, "Run Tests", payload.Name)
	require.Len(T, payload.Properties.Property, 1)
	assert.Equal(T, "script.content", payload.Properties.Property[0].Name)
	assert.Equal(T, "./gradlew test", payload.Properties.Property[0].Value)
}

func TestJobStepAddRequiresType(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)

	cmdtest.RunCmdWithFactoryExpectErr(T, ts.Factory, "type", "job", "step", "add", testJob, "--name", "x")
}

func TestJobStepAddInvalidParam(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)

	cmdtest.RunCmdWithFactoryExpectErr(T, ts.Factory, "invalid --param",
		"job", "step", "add", testJob, "--type", "commandLine", "--param", "noequals")
}

func TestJobStepDelete(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)

	out := cmdtest.CaptureOutput(T, ts.Factory, "job", "step", "delete", testJob, "RUNNER_1")
	assert.Contains(T, out, "Deleted step RUNNER_1")
}

func TestJobStepMissingJob(T *testing.T) {
	ts := cmdtest.SetupMockClient(T)
	T.Setenv("TEAMCITY_JOB", "")

	cmdtest.RunCmdWithFactoryExpectErr(T, ts.Factory, "job id is required", "job", "step", "list")
}
