package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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

func TestBasicAuthSendsCorrectHeaders(T *testing.T) {
	T.Parallel()

	var receivedUser, receivedPass string
	var authHeaderPresent bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedUser, receivedPass, authHeaderPresent = r.BasicAuth()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(User{ID: 1, Username: receivedUser, Name: "Test"})
	}))
	T.Cleanup(server.Close)

	client := NewClientWithBasicAuth(server.URL, "buildUser", "buildPass")
	_, err := client.GetCurrentUser()

	require.NoError(T, err)
	assert.True(T, authHeaderPresent, "basic auth header should be present")
	assert.Equal(T, "buildUser", receivedUser)
	assert.Equal(T, "buildPass", receivedPass)
}

func TestBasicAuthWorksForAPIRequests(T *testing.T) {
	T.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "testuser" || pass != "testpass" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ProjectList{
			Count:    1,
			Projects: []Project{{ID: "Test", Name: "Test Project"}},
		})
	}))
	T.Cleanup(server.Close)

	client := NewClientWithBasicAuth(server.URL, "testuser", "testpass")
	projects, err := client.GetProjects(ProjectsOptions{})

	require.NoError(T, err)
	assert.Equal(T, 1, projects.Count)
	assert.Equal(T, "Test", projects.Projects[0].ID)
}

func TestBasicAuthRejectsInvalidCredentials(T *testing.T) {
	T.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "validuser" || pass != "validpass" {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("Unauthorized"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(User{ID: 1})
	}))
	T.Cleanup(server.Close)

	client := NewClientWithBasicAuth(server.URL, "wronguser", "wrongpass")
	_, err := client.GetCurrentUser()

	assert.Error(T, err)
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
		{"malformed JSON", `not json`, "not json"}, // Now handled as plain text
		{"empty body", ``, ""},
		{"missing errors field", `{"other":"field"}`, ""}, // Valid JSON with no errors field
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
		{"nested parentheses", "No build types found by locator 'project:(id:Test)'.", "job 'Test' not found"}, // Now extracts ID
		{"special chars in id", "No build types found by locator 'My_Project-Config'.", "job 'My_Project-Config' not found"},
		{"complex locator", "Nothing is found by locator 'count:1,buildType:(id:My_Project_Build),branch:(default:any)'.", "no runs found for job 'My_Project_Build'"},
		{"without buildType", "Nothing is found by locator 'count:1,project:(id:Test)'.", "resource 'Test' not found"}, // Now extracts ID
		{"unicode", "No project found by locator '日本語プロジェクト'.", "project '日本語プロジェクト' not found"},
		{"no locator pattern", "Some error without locator pattern", "Some error without locator pattern"},
		{"incomplete buildType", "Nothing is found by locator 'buildType:(id:'.", "resource '' not found"}, // Empty ID
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

func TestCleanupBuildTriggered(T *testing.T) {
	T.Parallel()

	T.Run("nil triggered", func(t *testing.T) {
		t.Parallel()

		build := Build{}
		cleanupBuildTriggered(&build)
		assert.Nil(t, build.Triggered)
	})

	T.Run("nil user", func(t *testing.T) {
		t.Parallel()

		build := Build{Triggered: &Triggered{Type: "vcs"}}
		cleanupBuildTriggered(&build)
		assert.Nil(t, build.Triggered.User)
	})

	T.Run("empty user struct removed", func(t *testing.T) {
		t.Parallel()

		build := Build{
			Triggered: &Triggered{
				Type: "user",
				User: &User{}, // Empty user
			},
		}
		cleanupBuildTriggered(&build)
		assert.Nil(t, build.Triggered.User, "empty user should be nil")
	})

	T.Run("valid user preserved", func(t *testing.T) {
		t.Parallel()

		build := Build{
			Triggered: &Triggered{
				Type: "user",
				User: &User{ID: 1, Username: "admin", Name: "Admin"},
			},
		}
		cleanupBuildTriggered(&build)
		assert.NotNil(t, build.Triggered.User, "valid user should be preserved")
		assert.Equal(t, "admin", build.Triggered.User.Username)
	})

	T.Run("user with only username preserved", func(t *testing.T) {
		t.Parallel()

		build := Build{
			Triggered: &Triggered{
				Type: "user",
				User: &User{Username: "testuser"},
			},
		}
		cleanupBuildTriggered(&build)
		assert.NotNil(t, build.Triggered.User, "user with username should be preserved")
	})
}

func TestDownloadArtifactTo(T *testing.T) {
	T.Parallel()

	T.Run("successful download", func(t *testing.T) {
		t.Parallel()

		content := []byte("test artifact content 12345")
		client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			assert.Contains(t, r.URL.Path, "/artifacts/content/")
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(content)))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(content)
		})

		var buf bytes.Buffer
		written, err := client.DownloadArtifactTo(context.Background(), "123", "test.txt", &buf)

		require.NoError(t, err)
		assert.Equal(t, int64(len(content)), written)
		assert.Equal(t, content, buf.Bytes())
	})

	T.Run("URL encodes artifact path", func(t *testing.T) {
		t.Parallel()

		var escapedPath string
		client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			escapedPath = r.URL.EscapedPath()
			w.WriteHeader(http.StatusOK)
		})

		var buf bytes.Buffer
		_, _ = client.DownloadArtifactTo(context.Background(), "123", "file with spaces#1.txt", &buf)

		assert.Contains(t, escapedPath, "file%20with%20spaces%231.txt")
	})

	T.Run("returns error on non-200 status", func(t *testing.T) {
		t.Parallel()

		client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})

		var buf bytes.Buffer
		_, err := client.DownloadArtifactTo(context.Background(), "123", "missing.txt", &buf)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "status 404")
	})

	T.Run("uses correct auth header", func(t *testing.T) {
		t.Parallel()

		var authHeader string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader = r.Header.Get("Authorization")
			w.WriteHeader(http.StatusOK)
		}))
		t.Cleanup(server.Close)

		client := NewClient(server.URL, "my-secret-token")
		var buf bytes.Buffer
		_, _ = client.DownloadArtifactTo(context.Background(), "123", "test.txt", &buf)

		assert.Equal(t, "Bearer my-secret-token", authHeader)
	})

	T.Run("uses basic auth when configured", func(t *testing.T) {
		t.Parallel()

		var user, pass string
		var hasBasicAuth bool
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, pass, hasBasicAuth = r.BasicAuth()
			w.WriteHeader(http.StatusOK)
		}))
		t.Cleanup(server.Close)

		client := NewClientWithBasicAuth(server.URL, "myuser", "mypass")
		var buf bytes.Buffer
		_, _ = client.DownloadArtifactTo(context.Background(), "123", "test.txt", &buf)

		assert.True(t, hasBasicAuth)
		assert.Equal(t, "myuser", user)
		assert.Equal(t, "mypass", pass)
	})
}

func TestDebugLogging(T *testing.T) {
	T.Parallel()

	T.Run("logs request/response with redacted auth when DebugFunc set", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{}`))
		})
		client.DebugFunc = func(format string, args ...any) {
			fmt.Fprintf(&buf, format+"\n", args...)
		}

		_, _ = client.RawRequest("GET", "/api/test", nil, nil)

		captured := buf.String()
		assert.Contains(t, captured, "> GET")
		assert.Contains(t, captured, "/api/test")
		assert.Contains(t, captured, "> Authorization: [REDACTED]")
		assert.Contains(t, captured, "< HTTP/1.1 200 OK")
		assert.NotContains(t, captured, "test-token")
	})

	T.Run("silent when DebugFunc not set", func(t *testing.T) {
		t.Parallel()

		client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		// No DebugFunc set — should not panic or produce output

		_, _ = client.RawRequest("GET", "/api/test", nil, nil)
	})
}

func TestApproveQueuedBuild(T *testing.T) {
	T.Parallel()

	T.Run("success", func(t *testing.T) {
		t.Parallel()

		client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "PUT", r.Method)
			assert.Equal(t, "/app/rest/buildQueue/id:456/approval/status", r.URL.Path)

			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			assert.Equal(t, `"approved"`, string(body))

			w.WriteHeader(http.StatusOK)
		})

		err := client.ApproveQueuedBuild("456")
		assert.NoError(t, err)
	})

	T.Run("error not found", func(t *testing.T) {
		t.Parallel()

		client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"errors":[{"message":"No build found by locator '999'."}]}`))
		})

		err := client.ApproveQueuedBuild("999")
		assert.Error(t, err)
	})
}

func TestRebootAgentHTTPErrors(T *testing.T) {
	T.Parallel()

	tests := []struct {
		name       string
		statusCode int
		wantErr    string
	}{
		{
			name:       "401 Unauthorized",
			statusCode: http.StatusUnauthorized,
			wantErr:    "authentication failed",
		},
		{
			name:       "403 Forbidden",
			statusCode: http.StatusForbidden,
			wantErr:    "permission denied",
		},
		{
			name:       "404 Not Found",
			statusCode: http.StatusNotFound,
			wantErr:    "not found",
		},
	}

	for _, tc := range tests {
		T.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "/remoteAccess/reboot.html", r.URL.Path)
				w.WriteHeader(tc.statusCode)
			})

			err := client.RebootAgent(context.Background(), 42, false)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestGetBuildCommentServerError(T *testing.T) {
	T.Parallel()

	client := setupTestServer(T, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	})

	_, err := client.GetBuildComment("123")
	assert.Error(T, err)
}

func TestUploadDiffChangesServerError(T *testing.T) {
	T.Parallel()

	client := setupTestServer(T, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(T, "POST", r.Method)
		assert.Contains(T, r.URL.Path, "/uploadDiffChanges.html")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	})

	_, err := client.UploadDiffChanges([]byte("diff content"), "test change")
	require.Error(T, err)
	assert.Contains(T, err.Error(), "500")
}

func TestCreateProjectServerError(T *testing.T) {
	T.Parallel()

	client := setupTestServer(T, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(T, "POST", r.Method)
		assert.Equal(T, "/app/rest/projects", r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	})

	_, err := client.CreateProject(CreateProjectRequest{Name: "TestProject"})
	assert.Error(T, err)
}

func TestCreateUserServerError(T *testing.T) {
	T.Parallel()

	client := setupTestServer(T, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(T, "POST", r.Method)
		assert.Equal(T, "/app/rest/users", r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	})

	_, err := client.CreateUser(CreateUserRequest{Username: "newuser", Password: "pass"})
	assert.Error(T, err)
}

func TestGetServerError(T *testing.T) {
	T.Parallel()

	client := setupTestServer(T, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(T, "GET", r.Method)
		assert.Equal(T, "/app/rest/server", r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	})

	_, err := client.GetServer()
	assert.Error(T, err)
}

func TestNewGuestClient(T *testing.T) {
	T.Parallel()

	client := NewGuestClient("https://example.com/")
	assert.Equal(T, "https://example.com", client.BaseURL)
	assert.Empty(T, client.Token)
	assert.True(T, client.guestAuth)
}

func TestGuestClientNoAuthHeader(T *testing.T) {
	T.Parallel()

	var authHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Server{VersionMajor: 2024, VersionMinor: 12, BuildNumber: "176523"})
	}))
	T.Cleanup(server.Close)

	client := NewGuestClient(server.URL)
	_, err := client.GetServer()

	require.NoError(T, err)
	assert.Empty(T, authHeader, "guest client should not send Authorization header")
}

func TestGuestClientAPIPath(T *testing.T) {
	T.Parallel()

	tests := []struct {
		name string
		path string
		want string
	}{
		{"rest path", "/app/rest/server", "/guestAuth/app/rest/server"},
		{"rest path with version", "/app/rest/builds", "/guestAuth/app/rest/builds"},
		{"non-rest path", "/downloadBuildLog.html", "/guestAuth/downloadBuildLog.html"},
		{"already prefixed", "/guestAuth/app/rest/server", "/guestAuth/app/rest/server"},
	}

	for _, tc := range tests {
		T.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client := NewGuestClient("https://example.com")
			got := client.apiPath(tc.path)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestGuestClientAPIPathWithVersion(T *testing.T) {
	T.Parallel()

	client := NewGuestClient("https://example.com", WithAPIVersion("2023.1"))
	got := client.apiPath("/app/rest/builds")
	assert.Equal(T, "/guestAuth/app/rest/2023.1/builds", got)
}

func TestGuestClientUsesGuestAuthPrefix(T *testing.T) {
	T.Parallel()

	var requestPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Server{VersionMajor: 2024, VersionMinor: 12})
	}))
	T.Cleanup(server.Close)

	client := NewGuestClient(server.URL)
	_, _ = client.GetServer()

	assert.Equal(T, "/guestAuth/app/rest/server", requestPath)
}
