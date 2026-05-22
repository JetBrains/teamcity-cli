package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

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

	artifacts, err := client.GetArtifacts(t.Context(), "1", "")
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

	artifacts, err := client.GetArtifacts(t.Context(), "1", "logs")
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

	data, err := client.DownloadArtifact(t.Context(), "1", "build.jar")
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

	log, err := client.GetBuildLog(t.Context(), "1")
	require.NoError(t, err)
	assert.Contains(t, log, "Build started")
}

func TestGetBuildLogStream(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/app/rest/builds" || r.URL.Path == "/httpAuth/app/rest/builds" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(BuildList{Count: 1, Builds: []Build{{ID: 1}}})
			return
		}
		assert.Contains(t, r.URL.Path, "/downloadBuildLog.html")
		flusher, _ := w.(http.Flusher)
		_, _ = w.Write([]byte("chunk-one\n"))
		if flusher != nil {
			flusher.Flush()
		}
		_, _ = w.Write([]byte("chunk-two\n"))
	})

	rc, err := client.GetBuildLogStream(t.Context(), "1")
	require.NoError(t, err)
	t.Cleanup(func() { _ = rc.Close() })

	data, err := io.ReadAll(rc)
	require.NoError(t, err)
	assert.Equal(t, "chunk-one\nchunk-two\n", string(data))
}

func TestGetBuildLogStreamHonorsCheckRedirect(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/app/rest/builds"):
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(BuildList{Count: 1, Builds: []Build{{ID: 1}}})
		case r.URL.Path == "/after-redirect":
			_, _ = w.Write([]byte("followed"))
		default:
			w.Header().Set("Location", "/after-redirect")
			w.WriteHeader(http.StatusFound)
		}
	})

	rc, err := client.GetBuildLogStream(t.Context(), "1")
	require.NoError(t, err)
	data, _ := io.ReadAll(rc)
	_ = rc.Close()
	assert.Equal(t, "followed", string(data))

	client.HTTPClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	_, err = client.GetBuildLogStream(t.Context(), "1")
	require.Error(t, err)
}

func TestGetBuildLogStreamBypassesHTTPClientTimeout(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/app/rest/builds") {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(BuildList{Count: 1, Builds: []Build{{ID: 1}}})
			return
		}
		flusher, _ := w.(http.Flusher)
		_, _ = w.Write([]byte("first\n"))
		if flusher != nil {
			flusher.Flush()
		}
		time.Sleep(250 * time.Millisecond)
		_, _ = w.Write([]byte("second\n"))
	})
	client.HTTPClient.Timeout = 100 * time.Millisecond

	rc, err := client.GetBuildLogStream(t.Context(), "1")
	require.NoError(t, err)
	t.Cleanup(func() { _ = rc.Close() })

	data, err := io.ReadAll(rc)
	require.NoError(t, err)
	assert.Equal(t, "first\nsecond\n", string(data))
}
