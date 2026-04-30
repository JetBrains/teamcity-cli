package api

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustClassify(t *testing.T, status int, body string) error {
	t.Helper()
	resp := &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
	return ErrorFromResponse(resp)
}

func TestHTTPError_ErrorMessage(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"401 no body → auth phrasing", 401, "", "authentication failed"},
		{"403 no body → permission phrasing", 403, "", "permission denied"},
		{"404 no body → not-found phrasing", 404, "", "resource not found"},
		{"500 no body → status fallback", 500, "", "server returned 500"},
		{"401 with wire message → wire wins",
			401, `{"errors":[{"message":"token revoked"}]}`, "token revoked"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := mustClassify(t, tc.status, tc.body)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestPermissionError_ErrorMessage(t *testing.T) {
	t.Parallel()

	t.Run("perm + project", func(t *testing.T) {
		err := mustClassify(t, 403, `{"errors":[{"message":"You do not have \"Run build\" permission in project: 'My_Project'"}]}`)
		var pe *PermissionError
		ok := errors.As(err, &pe)
		require.True(t, ok)
		assert.Equal(t, `missing "Run build" permission in project My_Project`, pe.Error())
	})

	t.Run("perm only", func(t *testing.T) {
		pe := &PermissionError{Permission: "Edit project"}
		assert.Equal(t, `missing "Edit project" permission`, pe.Error())
	})

	t.Run("neither → wire fallback", func(t *testing.T) {
		pe := &PermissionError{HTTPError: HTTPError{Wire: Wire{Message: "go away"}}}
		assert.Equal(t, "go away", pe.Error())
	})

	t.Run("nothing at all", func(t *testing.T) {
		pe := &PermissionError{}
		assert.Equal(t, "permission denied", pe.Error())
	})
}

func TestNotFoundError_ErrorMessage(t *testing.T) {
	t.Parallel()

	t.Run("resource + id", func(t *testing.T) {
		nf := &NotFoundError{Resource: "agent", ID: "mac-01"}
		assert.Equal(t, `agent "mac-01" not found`, nf.Error())
	})
	t.Run("wire fallback", func(t *testing.T) {
		nf := &NotFoundError{HTTPError: HTTPError{Wire: Wire{Message: "no such thing"}}}
		assert.Equal(t, "no such thing", nf.Error())
	})
	t.Run("nothing", func(t *testing.T) {
		nf := &NotFoundError{}
		assert.Equal(t, "resource not found", nf.Error())
	})
}

func TestNetworkError(t *testing.T) {
	t.Parallel()

	cause := errors.New("dial tcp: connection refused")
	ne := &NetworkError{URL: "https://x.example.com", Cause: cause}

	assert.Contains(t, ne.Error(), "cannot connect to https://x.example.com")
	assert.Contains(t, ne.Error(), "connection refused")
	assert.Equal(t, CatNetwork, ne.Category())
	assert.Same(t, cause, ne.Unwrap())
	assert.True(t, errors.Is(ne, cause))

	t.Run("nil cause", func(t *testing.T) {
		bare := &NetworkError{URL: "https://y.example.com"}
		assert.Equal(t, "cannot connect to https://y.example.com", bare.Error())
		assert.Nil(t, bare.Unwrap())
	})
}

func TestIsSandboxBlocked(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"forbidden", &NetworkError{Cause: errors.New("dial tcp 1.2.3.4:443: Forbidden")}, true},
		{"blocked", &NetworkError{Cause: errors.New("dial tcp: Blocked by policy")}, true},
		{"sandbox literal", &NetworkError{Cause: errors.New("operation blocked by sandbox")}, true},
		{"plain timeout", &NetworkError{Cause: errors.New("dial tcp: i/o timeout")}, false},
		{"no cause", &NetworkError{URL: "https://x"}, false},
		{"non-network", errors.New("kaboom"), false},
		{"nil", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, IsSandboxBlocked(tc.err))
		})
	}
}

func TestValidationError(t *testing.T) {
	t.Parallel()

	v := Validation("bad input", "Try --foo")
	assert.Equal(t, "bad input", v.Error())
	assert.Equal(t, CatValidation, v.Category())
	assert.Equal(t, "Try --foo", v.Suggestion())

	t.Run("RequiredFlag", func(t *testing.T) {
		err := RequiredFlag("server")
		assert.Equal(t, "--server is required in non-interactive mode", err.Error())
		assert.Contains(t, err.Suggestion(), "--no-input")
	})

	t.Run("MutuallyExclusive", func(t *testing.T) {
		err := MutuallyExclusive("<id>", "all")
		assert.Equal(t, "cannot specify both <id> argument and --all flag", err.Error())
		assert.Contains(t, err.Suggestion(), "<id>")
		assert.Contains(t, err.Suggestion(), "--all")
	})
}

func TestErrReadOnly(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "read-only mode: writes are blocked", ErrReadOnly.Error())
	assert.Equal(t, CatReadOnly, ErrReadOnly.Category())

	// The interesting contract: callers must be able to detect ErrReadOnly
	// even when it's wrapped through fmt.Errorf. Identity comparison is
	// stdlib behavior; what matters here is that the unwrap chain works.
	wrapped := fmt.Errorf("during request: %w", ErrReadOnly)
	assert.True(t, errors.Is(wrapped, ErrReadOnly), "must unwrap through fmt.Errorf")
}
