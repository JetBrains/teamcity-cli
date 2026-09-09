package terminal

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
