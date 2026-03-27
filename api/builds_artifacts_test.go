package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetArtifacts(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/app/rest/builds" || r.URL.Path == "/httpAuth/app/rest/builds" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(BuildList{Count: 1, Builds: []Build{{ID: 1}}})
			return
		}
		assert.Contains(t, r.URL.Path, "/artifacts/children")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Artifacts{
			Count: 2,
			File:  []Artifact{{Name: "build.jar", Size: 1234}, {Name: "report.html", Size: 567}},
		})
	})

	artifacts, err := client.GetArtifacts("1", "")
	require.NoError(t, err)
	assert.Equal(t, 2, artifacts.Count)
}

func TestGetArtifactsWithSubpath(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/app/rest/builds" || r.URL.Path == "/httpAuth/app/rest/builds" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(BuildList{Count: 1, Builds: []Build{{ID: 1}}})
			return
		}
		assert.Contains(t, r.URL.Path, "/artifacts/children/logs")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Artifacts{Count: 1, File: []Artifact{{Name: "build.log"}}})
	})

	artifacts, err := client.GetArtifacts("1", "logs")
	require.NoError(t, err)
	assert.Equal(t, 1, artifacts.Count)
}

func TestDownloadArtifact(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/app/rest/builds" || r.URL.Path == "/httpAuth/app/rest/builds" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(BuildList{Count: 1, Builds: []Build{{ID: 1}}})
			return
		}
		assert.Contains(t, r.URL.Path, "/artifacts/content/build.jar")
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write([]byte("fake-jar-content"))
	})

	data, err := client.DownloadArtifact("1", "build.jar")
	require.NoError(t, err)
	assert.Equal(t, "fake-jar-content", string(data))
}


func TestGetBuildLog(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/app/rest/builds" || r.URL.Path == "/httpAuth/app/rest/builds" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(BuildList{Count: 1, Builds: []Build{{ID: 1}}})
			return
		}
		assert.Contains(t, r.URL.Path, "/downloadBuildLog.html")
		w.Write([]byte("[12:00:00] Build started\n[12:00:01] Done"))
	})

	log, err := client.GetBuildLog("1")
	require.NoError(t, err)
	assert.Contains(t, log, "Build started")
}
