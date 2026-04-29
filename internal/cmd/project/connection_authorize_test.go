package project_test

import (
	"net/http"
	"testing"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdtest"
	"github.com/stretchr/testify/assert"
)

func TestConnectionAuthorize(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	f := ts.Factory

	ts.Handle("GET /app/rest/projects/id:TestProject/projectFeatures", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.ProjectFeatureList{
			ProjectFeature: []api.ProjectFeature{{
				ID:   "PROJECT_EXT_42",
				Type: "OAuthProvider",
				Properties: &api.PropertyList{
					Property: []api.Property{
						{Name: "providerType", Value: "GitHubApp"},
					},
				},
			}},
		})
	})

	out := cmdtest.CaptureOutput(t, f, "project", "connection", "authorize", "PROJECT_EXT_42",
		"--project", "TestProject")

	assert.Contains(t, out, "Opening browser")
	assert.Contains(t, out, "Complete the flow in your browser")
}

func TestConnectionAuthorizeNotFound(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	f := ts.Factory

	ts.Handle("GET /app/rest/projects/id:TestProject/projectFeatures", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.ProjectFeatureList{ProjectFeature: []api.ProjectFeature{}})
	})

	cmdtest.RunCmdWithFactoryExpectErr(t, f, "not found", "project", "connection", "authorize", "PROJECT_EXT_99",
		"--project", "TestProject")
}

func TestConnectionAuthorizeRejectsNonOAuthType(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	f := ts.Factory

	ts.Handle("GET /app/rest/projects/id:TestProject/projectFeatures", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.ProjectFeatureList{
			ProjectFeature: []api.ProjectFeature{{
				ID:   "PROJECT_EXT_55",
				Type: "OAuthProvider",
				Properties: &api.PropertyList{
					Property: []api.Property{{Name: "providerType", Value: "Docker"}},
				},
			}},
		})
	})

	cmdtest.RunCmdWithFactoryExpectErr(t, f, "does not require browser authorization",
		"project", "connection", "authorize", "PROJECT_EXT_55",
		"--project", "TestProject")
}
