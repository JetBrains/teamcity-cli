package httpclient

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedirectPolicy(t *testing.T) {
	t.Parallel()
	original, err := http.NewRequest(http.MethodGet, "https://teamcity.example/api", nil)
	require.NoError(t, err)
	for _, target := range []string{"https://other.example/file", "https://sub.teamcity.example/file", "https://teamcity.example:8443/file"} {
		t.Run(target, func(t *testing.T) {
			t.Parallel()
			request, err := http.NewRequest(http.MethodGet, target, nil)
			require.NoError(t, err)
			request.Header.Set("X-Custom-Secret", "secret")
			require.NoError(t, RedirectPolicy(true)(request, []*http.Request{original}))
			assert.Empty(t, request.Header)
			require.ErrorContains(t, RedirectPolicy(false)(request, []*http.Request{original}), "cross-origin")
		})
	}
	request, err := http.NewRequest(http.MethodGet, "https://TEAMCITY.example/file", nil)
	require.NoError(t, err)
	request.Header.Set("Authorization", "Bearer secret")
	require.NoError(t, RedirectPolicy(true)(request, []*http.Request{original}))
	assert.Equal(t, "Bearer secret", request.Header.Get("Authorization"))
	require.ErrorContains(t, RedirectPolicy(true)(request, make([]*http.Request, 10)), "10 redirects")
}
