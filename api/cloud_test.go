package api

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func cloudNameLocatorExpectation(value string) string {
	return "name:(value:($base64:" + base64.RawURLEncoding.EncodeToString([]byte(value)) + "))"
}

func TestGetCloudImageBareSelectorUsesNameLocator(t *testing.T) {
	t.Parallel()

	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/app/rest/cloud/images/"+cloudNameLocatorExpectation("ubuntu-22-large"), r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(CloudImage{ID: "id:img-1,profileId:aws-prod", Name: "ubuntu-22-large"})
	})

	image, err := client.GetCloudImage("ubuntu-22-large")
	require.NoError(t, err)
	assert.Equal(t, "id:img-1,profileId:aws-prod", image.ID)
	assert.Equal(t, "ubuntu-22-large", image.Name)
}

func TestGetCloudImageColonInNameUsesWrappedNameLocator(t *testing.T) {
	t.Parallel()

	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/app/rest/cloud/images/"+cloudNameLocatorExpectation("ubuntu:22.04"), r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(CloudImage{ID: "id:img-1,profileId:aws-prod", Name: "ubuntu:22.04"})
	})

	image, err := client.GetCloudImage("ubuntu:22.04")
	require.NoError(t, err)
	assert.Equal(t, "id:img-1,profileId:aws-prod", image.ID)
	assert.Equal(t, "ubuntu:22.04", image.Name)
}

func TestCloudLocatorPreservesAlreadyWrappedCompoundID(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "id:(id:img-1,profileId:aws-prod)", cloudLocator("id:(id:img-1,profileId:aws-prod)", "name"))
}

func TestGetCloudImageCompoundIDWrapsUnderOuterIDLocator(t *testing.T) {
	t.Parallel()

	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/app/rest/cloud/images/id:(id:img-1,profileId:aws-prod)", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(CloudImage{ID: "id:img-1,profileId:aws-prod", Name: "ubuntu-22-large"})
	})

	image, err := client.GetCloudImage("id:img-1,profileId:aws-prod")
	require.NoError(t, err)
	assert.Equal(t, "id:img-1,profileId:aws-prod", image.ID)
}

func TestGetCloudInstancesBareImageSelectorUsesNameLocator(t *testing.T) {
	t.Parallel()

	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.RawQuery, url.QueryEscape("image:("+cloudNameLocatorExpectation("ubuntu-22-large")+")"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(CloudInstanceList{})
	})

	_, err := client.GetCloudInstances(CloudInstancesOptions{
		ProjectID: "TestProject",
		Image:     "ubuntu-22-large",
	})
	require.NoError(t, err)
}

func TestGetCloudInstancesColonInImageNameUsesWrappedNameLocator(t *testing.T) {
	t.Parallel()

	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.RawQuery, url.QueryEscape("image:("+cloudNameLocatorExpectation("ubuntu:22.04")+")"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(CloudInstanceList{})
	})

	_, err := client.GetCloudInstances(CloudInstancesOptions{
		ProjectID: "TestProject",
		Image:     "ubuntu:22.04",
	})
	require.NoError(t, err)
}

func TestGetCloudInstancesPreservesExplicitCompoundImageLocator(t *testing.T) {
	t.Parallel()

	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.RawQuery, "image%3A%28id%3A%28id%3Aimg-1%2CprofileId%3Aaws-prod%29%29")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(CloudInstanceList{})
	})

	_, err := client.GetCloudInstances(CloudInstancesOptions{
		ProjectID: "TestProject",
		Image:     "id:img-1,profileId:aws-prod",
	})
	require.NoError(t, err)
}

func TestStopCloudInstanceCompoundIDWrapsUnderOuterIDLocator(t *testing.T) {
	t.Parallel()

	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/app/rest/cloud/instances/id:(id:i-123,imageId:id:img-1,profileId:aws-prod)/actions/forceStop", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	})

	err := client.StopCloudInstance("id:i-123,imageId:id:img-1,profileId:aws-prod", true)
	require.NoError(t, err)
}
