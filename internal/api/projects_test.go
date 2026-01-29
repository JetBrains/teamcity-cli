package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetVersionedSettingsStatus(T *testing.T) {
	T.Parallel()

	T.Run("success", func(t *testing.T) {
		t.Parallel()

		client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.True(t, strings.HasSuffix(r.URL.Path, "/versionedSettings/status"))
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(VersionedSettingsStatus{
				Type:        "info",
				Message:     "Settings are up to date",
				Timestamp:   "Mon Jan 27 10:30:00 UTC 2025",
				DslOutdated: false,
			})
		})

		status, err := client.GetVersionedSettingsStatus("TestProject")
		require.NoError(t, err)
		assert.Equal(t, "info", status.Type)
		assert.Equal(t, "Settings are up to date", status.Message)
		assert.False(t, status.DslOutdated)
	})

	T.Run("warning status", func(t *testing.T) {
		t.Parallel()

		client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(VersionedSettingsStatus{
				Type:        "warning",
				Message:     "DSL scripts need to be regenerated",
				DslOutdated: true,
			})
		})

		status, err := client.GetVersionedSettingsStatus("TestProject")
		require.NoError(t, err)
		assert.Equal(t, "warning", status.Type)
		assert.True(t, status.DslOutdated)
	})

	T.Run("error status", func(t *testing.T) {
		t.Parallel()

		client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(VersionedSettingsStatus{
				Type:    "error",
				Message: "Failed to sync settings",
			})
		})

		status, err := client.GetVersionedSettingsStatus("TestProject")
		require.NoError(t, err)
		assert.Equal(t, "error", status.Type)
	})

	T.Run("not found", func(t *testing.T) {
		t.Parallel()

		client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"errors":[{"message":"Versioned settings are not configured"}]}`))
		})

		_, err := client.GetVersionedSettingsStatus("NoSettingsProject")
		assert.Error(t, err)
	})
}

func TestGetVersionedSettingsConfig(T *testing.T) {
	T.Parallel()

	T.Run("kotlin format", func(t *testing.T) {
		t.Parallel()

		client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.True(t, strings.HasSuffix(r.URL.Path, "/versionedSettings/config"))
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(VersionedSettingsConfig{
				SynchronizationMode: "enabled",
				Format:              "kotlin",
				BuildSettingsMode:   "useFromVCS",
				VcsRootID:           "TestProject_GitRepo",
				SettingsPath:        ".teamcity",
				AllowUIEditing:      true,
				ShowSettingsChanges: true,
			})
		})

		config, err := client.GetVersionedSettingsConfig("TestProject")
		require.NoError(t, err)
		assert.Equal(t, "enabled", config.SynchronizationMode)
		assert.Equal(t, "kotlin", config.Format)
		assert.Equal(t, "useFromVCS", config.BuildSettingsMode)
		assert.Equal(t, "TestProject_GitRepo", config.VcsRootID)
		assert.Equal(t, ".teamcity", config.SettingsPath)
		assert.True(t, config.AllowUIEditing)
	})

	T.Run("xml format", func(t *testing.T) {
		t.Parallel()

		client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(VersionedSettingsConfig{
				SynchronizationMode: "enabled",
				Format:              "xml",
				BuildSettingsMode:   "useCurrentByDefault",
			})
		})

		config, err := client.GetVersionedSettingsConfig("TestProject")
		require.NoError(t, err)
		assert.Equal(t, "xml", config.Format)
		assert.Equal(t, "useCurrentByDefault", config.BuildSettingsMode)
	})

	T.Run("not configured", func(t *testing.T) {
		t.Parallel()

		client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"errors":[{"message":"Versioned settings are not configured for this project"}]}`))
		})

		_, err := client.GetVersionedSettingsConfig("NoSettingsProject")
		assert.Error(t, err)
	})

	T.Run("forbidden", func(t *testing.T) {
		t.Parallel()

		client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"errors":[{"message":"Access denied"}]}`))
		})

		_, err := client.GetVersionedSettingsConfig("RestrictedProject")
		assert.Error(t, err)
	})
}

func TestCreateSecureToken(T *testing.T) {
	T.Parallel()

	T.Run("success", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.True(t, strings.HasSuffix(r.URL.Path, "/secure/tokens"))
			assert.Equal(t, "text/plain", r.Header.Get("Content-Type"))
			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte("credentialsJSON:abc123xyz"))
		}))
		t.Cleanup(server.Close)
		client := NewClient(server.URL, "test-token")

		token, err := client.CreateSecureToken("TestProject", "my-secret-value")
		require.NoError(t, err)
		assert.Equal(t, "credentialsJSON:abc123xyz", token)
	})

	T.Run("forbidden", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"errors":[{"message":"Requires EDIT_PROJECT permission"}]}`))
		}))
		t.Cleanup(server.Close)
		client := NewClient(server.URL, "test-token")

		_, err := client.CreateSecureToken("TestProject", "secret")
		assert.Error(t, err)
	})
}

func TestGetSecureValue(T *testing.T) {
	T.Parallel()

	T.Run("success", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.True(t, strings.Contains(r.URL.Path, "/secure/values/"))
			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte("my-secret-value"))
		}))
		t.Cleanup(server.Close)
		client := NewClient(server.URL, "test-token")

		value, err := client.GetSecureValue("TestProject", "abc123xyz")
		require.NoError(t, err)
		assert.Equal(t, "my-secret-value", value)
	})

	T.Run("forbidden", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"errors":[{"message":"Requires CHANGE_SERVER_SETTINGS permission"}]}`))
		}))
		t.Cleanup(server.Close)
		client := NewClient(server.URL, "test-token")

		_, err := client.GetSecureValue("TestProject", "abc123")
		assert.Error(t, err)
	})

	T.Run("not found", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"errors":[{"message":"Token not found"}]}`))
		}))
		t.Cleanup(server.Close)
		client := NewClient(server.URL, "test-token")

		_, err := client.GetSecureValue("TestProject", "nonexistent")
		assert.Error(t, err)
	})
}

func TestProjectExists(T *testing.T) {
	T.Parallel()

	T.Run("exists", func(t *testing.T) {
		t.Parallel()

		client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(Project{ID: "TestProject", Name: "Test Project"})
		})

		exists := client.ProjectExists("TestProject")
		assert.True(t, exists)
	})

	T.Run("not exists", func(t *testing.T) {
		t.Parallel()

		client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})

		exists := client.ProjectExists("NonExistentProject")
		assert.False(t, exists)
	})
}
