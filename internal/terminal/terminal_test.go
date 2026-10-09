package terminal

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConnectPreservesServerPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ci/httpAuth/plugins/teamcity-agent-terminal/agentTerminal.html":
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "7", r.URL.Query().Get("id"))
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "cookie-value", Path: "/ci"})
			_, _ = w.Write([]byte(`{"token":"session-token"}`))
		case "/ci/app/agentTerminal/terminal/session-token":
			assert.Equal(t, "80", r.URL.Query().Get("cols"))
			assert.Equal(t, "24", r.URL.Query().Get("rows"))
			assert.Equal(t, "http://"+r.Host, r.Header.Get("Origin"))
			cookie, err := r.Cookie("session")
			if assert.NoError(t, err) {
				assert.Equal(t, "cookie-value", cookie.Value)
			}
			conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
			if err == nil {
				_ = conn.Close()
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL+"/ci", "user", "token", func(string, ...any) {})
	session, err := client.OpenSession(7)
	require.NoError(t, err)
	conn, err := client.Connect(session, 80, 24)
	require.NoError(t, err)
	require.NoError(t, conn.conn.Close())
}

func TestOpenSessionRejectsCrossOriginRedirect(t *testing.T) {
	t.Setenv("TEAMCITY_HEADER_X_API_KEY", "proxy-secret")
	destination := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		t.Error("terminal credentials and cookies must not reach another origin")
	}))
	defer destination.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.SetCookie(writer, &http.Cookie{Name: "session", Value: "secret"})
		http.Redirect(writer, request, destination.URL, http.StatusFound)
	}))
	defer origin.Close()
	client := NewClient(origin.URL, "user", "token", func(string, ...any) {})
	_, err := client.OpenSession(1)
	require.ErrorContains(t, err, "refusing cross-origin redirect")
}

func TestNewClient(t *testing.T) {
	c := NewClient("https://tc.example.com/", "admin", "token123", func(string, ...any) {})
	assert.Equal(t, "https://tc.example.com", c.baseURL)
	assert.Equal(t, "admin", c.username)
	assert.Equal(t, "token123", c.token)
	assert.NotNil(t, c.httpClient)
	assert.NotNil(t, c.httpClient.Jar)
}

func TestNewClientEmptyUsername(t *testing.T) {
	c := NewClient("http://localhost:8111", "", "tok", func(string, ...any) {})
	assert.Equal(t, "http://localhost:8111", c.baseURL)
	assert.Empty(t, c.username)
}
