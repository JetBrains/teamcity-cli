package api

import (
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInsecureTLSIsExplicitAndClientScoped(t *testing.T) {
	t.Parallel()
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"versionMajor":2026,"versionMinor":2}`)
	}))
	server.Config.ErrorLog = log.New(io.Discard, "", 0)
	server.StartTLS()
	t.Cleanup(server.Close)

	for _, skip := range []bool{false, true, false} {
		client := NewClient(server.URL, "token", WithInsecureTLS(skip))
		defer client.HTTPClient.CloseIdleConnections()
		info, err := client.GetServer()
		if skip {
			require.NoError(t, err)
			assert.Equal(t, 2026, info.VersionMajor)
		} else {
			require.ErrorContains(t, err, "certificate")
		}
	}
}

func TestInsecureTLSDoesNotTrustCrossOriginRedirects(t *testing.T) {
	t.Parallel()
	destination := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("an unrelated server with an untrusted certificate must not receive the request")
	}))
	destination.Config.ErrorLog = log.New(io.Discard, "", 0)
	destination.StartTLS()
	t.Cleanup(destination.Close)
	origin := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, destination.URL, http.StatusFound)
	}))
	t.Cleanup(origin.Close)

	client := NewClient(origin.URL, "token", WithInsecureTLS(true))
	defer client.HTTPClient.CloseIdleConnections()
	_, err := client.GetServer()
	require.ErrorContains(t, err, "certificate")
}
