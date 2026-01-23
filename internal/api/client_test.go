package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestServer creates a test HTTP server and returns a client configured to use it.
func setupTestServer(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return NewClient(server.URL, "test-token")
}

func TestNewClient(T *testing.T) {
	T.Parallel()

	tests := []struct {
		name        string
		baseURL     string
		token       string
		wantBaseURL string
	}{
		{
			name:        "standard URL",
			baseURL:     "https://example.com",
			token:       "test-token",
			wantBaseURL: "https://example.com",
		},
		{
			name:        "URL with trailing slash",
			baseURL:     "https://example.com/",
			token:       "test-token",
			wantBaseURL: "https://example.com",
		},
	}

	for _, tc := range tests {
		T.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client := NewClient(tc.baseURL, tc.token)
			assert.Equal(t, tc.wantBaseURL, client.BaseURL)
			assert.Equal(t, tc.token, client.Token)
		})
	}
}

func TestNewClientWithBasicAuth(T *testing.T) {
	T.Parallel()

	client := NewClientWithBasicAuth("https://example.com", "user", "pass")
	assert.Equal(T, "https://example.com", client.BaseURL)
	assert.Empty(T, client.Token)
}

func TestAPIPath(T *testing.T) {
	T.Parallel()

	tests := []struct {
		name       string
		apiVersion string
		path       string
		want       string
	}{
		{"no version", "", "/app/rest/builds", "/app/rest/builds"},
		{"with version", "2023.1", "/app/rest/builds", "/app/rest/2023.1/builds"},
		{"non-rest path unchanged", "2023.1", "/downloadBuildLog.html", "/downloadBuildLog.html"},
	}

	for _, tc := range tests {
		T.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client := NewClient("https://example.com", "token")
			client.APIVersion = tc.apiVersion
			got := client.apiPath(tc.path)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestClientOptions(T *testing.T) {
	T.Parallel()

	client := NewClient("https://example.com", "token", WithAPIVersion("2023.1"), WithTimeout(60*time.Second))

	assert.Equal(T, "2023.1", client.APIVersion)
	assert.Equal(T, 60*time.Second, client.HTTPClient.Timeout)
}

func TestCheckVersion(T *testing.T) {
	T.Parallel()

	tests := []struct {
		name         string
		versionMajor int
		wantErr      bool
	}{
		{"current version", 2024, false},
		{"old version", 2019, true},
	}

	for _, tc := range tests {
		T.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(Server{
					Version:      "test",
					VersionMajor: tc.versionMajor,
					VersionMinor: 1,
				})
			})

			err := client.CheckVersion()
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSupportsFeature(T *testing.T) {
	T.Parallel()

	tests := []struct {
		name         string
		versionMajor int
		versionMinor int
		feature      string
		want         bool
	}{
		{"csrf_token supported", 2024, 1, "csrf_token", true},
		{"csrf_token not supported old version", 2017, 1, "csrf_token", false},
	}

	for _, tc := range tests {
		T.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(Server{VersionMajor: tc.versionMajor, VersionMinor: tc.versionMinor})
			})

			got := client.SupportsFeature(tc.feature)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestHandleErrorResponse(T *testing.T) {
	T.Parallel()

	tests := []struct {
		name       string
		statusCode int
		body       string
	}{
		{"unauthorized", http.StatusUnauthorized, "error message"},
		{"forbidden", http.StatusForbidden, "error message"},
		{"not found plain text", http.StatusNotFound, "error message"},
		{"not found TeamCity format", http.StatusNotFound, `{"errors":[{"message":"No build found by locator '999'."}]}`},
		{"internal server error", http.StatusInternalServerError, "Internal Server Error"},
	}

	for _, tc := range tests {
		T.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.statusCode)
				w.Write([]byte(tc.body))
			})

			_, err := client.GetBuild("123")
			assert.Error(t, err)
		})
	}
}

func TestHandleErrorResponseWithStructuredError(T *testing.T) {
	T.Parallel()

	client := setupTestServer(T, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"errors":[{"message":"Invalid parameter value"}]}`))
	})

	_, err := client.GetBuild("invalid")
	require.Error(T, err)
	assert.Contains(T, err.Error(), "Invalid parameter value")
}

func TestRemoveBuildTag(T *testing.T) {
	T.Parallel()

	callCount := 0
	client := setupTestServer(T, func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			assert.Equal(T, "GET", r.Method)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"count":2,"tag":[{"name":"mytag"},{"name":"othertag"}]}`))
		} else {
			assert.Equal(T, "PUT", r.Method)
			w.WriteHeader(http.StatusOK)
		}
	})

	err := client.RemoveBuildTag("123", "mytag")
	require.NoError(T, err)
}

func TestRemoveBuildTagNotFound(T *testing.T) {
	T.Parallel()

	client := setupTestServer(T, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"count":1,"tag":[{"name":"othertag"}]}`))
	})

	err := client.RemoveBuildTag("123", "nonexistent")
	assert.Error(T, err)
}

func TestParseTeamCityTime(T *testing.T) {
	T.Parallel()

	tests := []struct {
		input   string
		want    time.Time
		wantErr bool
	}{
		{"20250710T080607+0000", time.Date(2025, 7, 10, 8, 6, 7, 0, time.UTC), false},
		{"20240115T143022+0000", time.Date(2024, 1, 15, 14, 30, 22, 0, time.UTC), false},
		{"", time.Time{}, true},
	}

	for _, tc := range tests {
		T.Run(tc.input, func(t *testing.T) {
			t.Parallel()

			got, err := ParseTeamCityTime(tc.input)
			if tc.wantErr {
				assert.Error(t, err)
				assert.True(t, got.IsZero())
				return
			}
			require.NoError(t, err)
			assert.True(t, got.Equal(tc.want))
		})
	}
}

func TestExtractErrorMessage(T *testing.T) {
	T.Parallel()

	tests := []struct {
		name string
		body string
		want string
	}{
		{"valid error response", `{"errors":[{"message":"No build types found by locator 'Test'."}]}`, "job 'Test' not found"},
		{"empty errors array", `{"errors":[]}`, ""},
		{"malformed JSON", `not json`, ""},
		{"empty body", ``, ""},
		{"missing errors field", `{"other":"field"}`, ""},
	}

	for _, tc := range tests {
		T.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := extractErrorMessage([]byte(tc.body))
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestHumanizeErrorMessage(T *testing.T) {
	T.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"build type not found with period", "No build types found by locator 'Sandbox_Demo'.", "job 'Sandbox_Demo' not found"},
		{"build type not found without period", "No build types found by locator 'Sandbox_Demo'", "job 'Sandbox_Demo' not found"},
		{"build not found", "No build found by locator '12345'.", "run '12345' not found"},
		{"project not found", "No project found by locator 'MyProject'.", "project 'MyProject' not found"},
		{"nothing found with buildType locator", "Nothing is found by locator 'count:1,buildType:(id:Sandbox_Demo)'.", "no runs found for job 'Sandbox_Demo'"},
		{"unrecognized message", "Some other error message", "Some other error message"},
		{"empty message", "", ""},
		{"nested parentheses", "No build types found by locator 'project:(id:Test)'.", "job 'project:(id:Test)' not found"},
		{"special chars in id", "No build types found by locator 'My_Project-Config'.", "job 'My_Project-Config' not found"},
		{"complex locator", "Nothing is found by locator 'count:1,buildType:(id:My_Project_Build),branch:(default:any)'.", "no runs found for job 'My_Project_Build'"},
		{"without buildType", "Nothing is found by locator 'count:1,project:(id:Test)'.", "Nothing is found by locator 'count:1,project:(id:Test)'."},
		{"unicode", "No project found by locator '日本語プロジェクト'.", "project '日本語プロジェクト' not found"},
		{"no locator pattern", "Some error without locator pattern", "Some error without locator pattern"},
		{"incomplete buildType", "Nothing is found by locator 'buildType:(id:'.", "Nothing is found by locator 'buildType:(id:'."},
	}

	for _, tc := range tests {
		T.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := humanizeErrorMessage(tc.input)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestResolveBuildID(T *testing.T) {
	T.Parallel()

	T.Run("passthrough IDs", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name  string
			input string
			want  string
		}{
			{"plain numeric ID", "12345", "12345"},
			{"ID with letters", "abc123", "abc123"},
			{"empty string", "", ""},
		}

		client := NewClient("https://example.com", "token")
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				got, err := client.ResolveBuildID(tc.input)
				require.NoError(t, err)
				assert.Equal(t, tc.want, got)
			})
		}
	})

	T.Run("hash prefix resolution", func(t *testing.T) {
		t.Parallel()

		client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			assert.Contains(t, r.URL.RawQuery, "number")
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(BuildList{
				Count:  1,
				Builds: []Build{{ID: 99999, Number: "42"}},
			})
		})

		got, err := client.ResolveBuildID("#42")
		require.NoError(t, err)
		assert.Equal(t, "99999", got)
	})

	T.Run("not found", func(t *testing.T) {
		t.Parallel()

		client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(BuildList{Count: 0, Builds: []Build{}})
		})

		_, err := client.ResolveBuildID("#999999")
		assert.Error(t, err)
	})

	T.Run("server error", func(t *testing.T) {
		t.Parallel()

		client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})

		_, err := client.ResolveBuildID("#42")
		assert.Error(t, err)
	})
}
