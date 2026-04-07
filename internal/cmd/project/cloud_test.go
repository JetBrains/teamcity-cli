package project_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/fatih/color"
	"github.com/stretchr/testify/assert"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdtest"
)

func init() { color.NoColor = true }

func TestCloudProfileList(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)

	out := cmdtest.CaptureOutput(t, ts.Factory, "project", "cloud", "profile", "list", "--project", "TestProject")
	assert.Contains(t, out, "aws-prod")
	assert.Contains(t, out, "AWS Production")

	cmdtest.RunCmdWithFactory(t, ts.Factory, "project", "cloud", "profile", "list", "--project", "TestProject", "--json")
}

func TestCloudProfileView(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)

	out := cmdtest.CaptureOutput(t, ts.Factory, "project", "cloud", "profile", "view", "aws-prod")
	assert.Contains(t, out, "AWS Production")
	assert.Contains(t, out, "amazon")
	assert.Contains(t, out, "Project: TestProject")
}

func TestCloudProfileViewUsesNestedProject(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)

	ts.Handle("GET /app/rest/cloud/profiles/", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.CloudProfile{
			ID:              "aws-prod",
			Name:            "AWS Production",
			CloudProviderID: "amazon",
			Project:         &api.Project{ID: "NestedProject", Name: "Nested Project"},
		})
	})

	out := cmdtest.CaptureOutput(t, ts.Factory, "project", "cloud", "profile", "view", "aws-prod")
	assert.Contains(t, out, "AWS Production")
	assert.Contains(t, out, "Project: NestedProject")
}

func TestCloudImageList(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)

	out := cmdtest.CaptureOutput(t, ts.Factory, "project", "cloud", "image", "list", "--project", "TestProject")
	assert.Contains(t, out, "ubuntu-22-large")
	assert.Contains(t, out, "windows-2022")

	cmdtest.RunCmdWithFactory(t, ts.Factory, "project", "cloud", "image", "list", "--project", "TestProject", "--profile", "aws-prod")
	cmdtest.RunCmdWithFactory(t, ts.Factory, "project", "cloud", "image", "list", "--project", "TestProject", "--json")
}

func TestCloudImageView(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)

	out := cmdtest.CaptureOutput(t, ts.Factory, "project", "cloud", "image", "view", "id:img-1,profileId:aws-prod")
	assert.Contains(t, out, "ubuntu-22-large")
	assert.Contains(t, out, "id:img-1,profileId:aws-prod")
}

func TestCloudImageStart(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)

	out := cmdtest.CaptureOutput(t, ts.Factory, "project", "cloud", "image", "start", "id:img-1,profileId:aws-prod")
	assert.Contains(t, out, "Started instance")
	assert.Contains(t, out, "i-new-instance")
}

func TestCloudImageStartByName(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)

	ts.Handle("POST /app/rest/cloud/instances", func(w http.ResponseWriter, r *http.Request) {
		var req api.StartCloudInstanceRequest
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, "id:img-1,profileId:aws-prod", req.Image.ID)
		cmdtest.JSON(w, api.CloudInstance{
			ID:    "i-new-instance",
			Name:  "agent-cloud-new",
			State: "starting",
			Image: &api.CloudImage{ID: "id:img-1,profileId:aws-prod", Name: "ubuntu-22-large"},
		})
	})

	out := cmdtest.CaptureOutput(t, ts.Factory, "project", "cloud", "image", "start", "ubuntu-22-large")
	assert.Contains(t, out, "Started instance")
	assert.Contains(t, out, "i-new-instance")
	assert.Contains(t, out, "ubuntu-22-large")
}

func TestCloudInstanceList(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)

	out := cmdtest.CaptureOutput(t, ts.Factory, "project", "cloud", "instance", "list", "--project", "TestProject")
	assert.Contains(t, out, "agent-cloud-1")
	assert.Contains(t, out, "running")

	cmdtest.RunCmdWithFactory(t, ts.Factory, "project", "cloud", "instance", "list", "--project", "TestProject", "--image", "id:img-1,profileId:aws-prod")
}

func TestCloudInstanceView(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)

	out := cmdtest.CaptureOutput(t, ts.Factory, "project", "cloud", "instance", "view", "i-0245b46070c443201")
	assert.Contains(t, out, "agent-cloud-1")
	assert.Contains(t, out, "running")
}

func TestCloudInstanceStop(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)

	out := cmdtest.CaptureOutput(t, ts.Factory, "project", "cloud", "instance", "stop", "i-0245b46070c443201")
	assert.Contains(t, out, "Stopped instance")

	out = cmdtest.CaptureOutput(t, ts.Factory, "project", "cloud", "instance", "stop", "i-0245b46070c443201", "--force")
	assert.Contains(t, out, "Force-stopped instance")
}
