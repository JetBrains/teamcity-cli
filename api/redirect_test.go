package api

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedirectCredentials(t *testing.T) {
	t.Parallel()
	for _, auth := range []string{"bearer", "basic"} {
		t.Run(auth, func(t *testing.T) {
			t.Parallel()
			destination := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				for _, header := range []string{"Authorization", "Cookie", "X-Api-Key", "X-Raw-Secret", "Referer"} {
					assert.Empty(t, request.Header.Get(header), header)
				}
				if request.URL.Path != "/final" {
					http.Redirect(writer, request, "/final", http.StatusFound)
					return
				}
				_, _ = io.WriteString(writer, "artifact")
			}))
			defer destination.Close()
			origin := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				assert.NotEmpty(t, request.Header.Get("Authorization"))
				assert.Equal(t, "proxy-secret", request.Header.Get("X-Api-Key"))
				if !strings.HasPrefix(request.URL.Path, "/redirected/") {
					http.Redirect(writer, request, "/redirected/"+strings.TrimPrefix(request.URL.Path, "/"), http.StatusFound)
					return
				}
				http.Redirect(writer, request, destination.URL, http.StatusFound)
			}))
			defer origin.Close()
			opts := WithExtraHeaders(map[string]string{"X-Api-Key": "proxy-secret", "Cookie": "session=secret"})
			client := NewClient(origin.URL, "fake-token", opts)
			if auth == "basic" {
				client = NewClientWithBasicAuth(origin.URL, "fake-user", "fake-password", opts)
			}
			response, err := client.RawRequest(t.Context(), http.MethodGet, "/app/rest/server", nil, map[string]string{"X-Raw-Secret": "raw-secret"})
			require.NoError(t, err)
			assert.Equal(t, "artifact", string(response.Body))
			var output bytes.Buffer
			_, err = client.DownloadArtifactTo(t.Context(), "1", "artifact", &output)
			require.NoError(t, err)
			assert.Equal(t, "artifact", output.String())
		})
	}
}

func TestRedirectRejectsHTTPSDowngrade(t *testing.T) {
	t.Parallel()
	destination := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		t.Error("downgrade destination must not receive a request")
	}))
	defer destination.Close()
	origin := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, destination.URL, http.StatusFound)
	}))
	defer origin.Close()
	client := NewClient(origin.URL, "fake-token")
	client.HTTPClient.Transport = origin.Client().Transport
	_, err := client.RawRequest(t.Context(), http.MethodGet, "/app/rest/server", nil, nil)
	require.ErrorContains(t, err, "refusing HTTPS downgrade redirect")
}

func TestRedirectRejectsCrossOriginBody(t *testing.T) {
	t.Parallel()
	destination := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		t.Error("cross-origin destination must not receive a credential-bearing body")
	}))
	defer destination.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, destination.URL, http.StatusTemporaryRedirect)
	}))
	defer origin.Close()
	client := NewClient(origin.URL, "fake-token")
	_, err := client.RawRequest(t.Context(), http.MethodPost, "/app/rest/server", strings.NewReader("secret"), nil)
	require.ErrorContains(t, err, "refusing cross-origin redirect")
}
