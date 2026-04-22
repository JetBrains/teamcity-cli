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

func TestParseWire(T *testing.T) {
	T.Parallel()

	tests := []struct {
		name string
		body string
		want Wire
	}{
		{"empty", "", Wire{}},
		{"teamcity json", `{"errors":[{"message":"bad locator","additionalMessage":"try again","statusText":"error"}]}`,
			Wire{Message: "bad locator", Additional: "try again", StatusText: "error"}},
		{"teamcity xml", `<errors><error><message>hello</message></error></errors>`, Wire{Message: "hello"}},
		{"plain text", "plain error", Wire{Message: "plain error"}},
		{"html stripped", "<html>login page</html>", Wire{}},
		{"empty errors array", `{"errors":[]}`, Wire{}},
		{"valid json without errors field", `{"foo":"bar"}`, Wire{}},
	}

	for _, tc := range tests {
		T.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, parseWire([]byte(tc.body)))
		})
	}
}

func TestParsePermission(T *testing.T) {
	T.Parallel()

	tests := []struct {
		msg     string
		perm    string
		project string
	}{
		// Quoted-permission shape from the REST layer, double quotes.
		{
			`You do not have "Comment build" permission in project with internal id: project1`,
			"Comment build", "project1",
		},
		{
			// Real server output: single-quoted project id after "internal id:".
			`You do not have "Run build" permission in project with internal id: 'MyProj'.`,
			"Run build", "MyProj",
		},
		{
			`You do not have "Edit project" permission.`,
			"Edit project", "",
		},
		{
			// Single-quoted permission name (pin/unpin, run tagging, etc.).
			`You do not have 'Pin / unpin build' permission`,
			"Pin / unpin build", "",
		},
		{
			// Unquoted-permission shape from server-core PermissionChecker.
			`User admin does not have permission Comment build in project with internal id: 'project1'`,
			"Comment build", "project1",
		},
		{
			// Unquoted permission, trailing period, no project clause.
			`User admin does not have permission Edit project.`,
			"Edit project", "",
		},
		// Non-parseable variants — we fall back to the raw server message.
		{"Access denied. Check the user has enough permissions.", "", ""},
		{"You do not have enough permissions to cancel the build", "", ""},
		{"", "", ""},
	}

	for _, tc := range tests {
		T.Run(tc.msg, func(t *testing.T) {
			t.Parallel()
			perm, proj := parsePermission(tc.msg)
			assert.Equal(t, tc.perm, perm)
			assert.Equal(t, tc.project, proj)
		})
	}
}

func TestParseNotFound(T *testing.T) {
	T.Parallel()

	tests := []struct {
		msg      string
		resource string
		id       string
	}{
		{"No build types found by locator 'Foo'.", "job", "Foo"},
		{"No build types found by locator 'count:1,id:Foo_Bar'.", "job", "Foo_Bar"},
		{"No build found by locator '12345'.", "run", "12345"},
		{"No project found by locator 'MyProj'.", "project", "MyProj"},
		{"No user found by locator 'alice'.", "user", "alice"},
		{"No agent found by locator 'build-agent-01'.", "agent", "build-agent-01"},
		{"something else entirely", "", ""},
		{"", "", ""},
	}

	for _, tc := range tests {
		T.Run(tc.msg, func(t *testing.T) {
			t.Parallel()
			res, id := parseNotFound(tc.msg)
			assert.Equal(t, tc.resource, res)
			assert.Equal(t, tc.id, id)
		})
	}
}

func TestErrorFromBody(T *testing.T) {
	T.Parallel()

	T.Run("401 → HTTPError with auth category", func(t *testing.T) {
		t.Parallel()
		err := errorFromBody(http.StatusUnauthorized, nil)
		he, ok := errors.AsType[*HTTPError](err)
		require.True(t, ok)
		assert.Equal(t, CatAuth, he.Category())
		assert.Contains(t, err.Error(), "authentication failed")
	})

	T.Run("403 with permission parsed", func(t *testing.T) {
		t.Parallel()
		// Real server output: project id is single-quoted in the body.
		body := []byte(`{"errors":[{"message":"You do not have \"Comment build\" permission in project with internal id: 'MyProj'"}]}`)
		err := errorFromBody(http.StatusForbidden, body)
		pe, ok := errors.AsType[*PermissionError](err)
		require.True(t, ok)
		assert.Equal(t, CatPermission, pe.Category())
		assert.Equal(t, "Comment build", pe.Permission)
		assert.Equal(t, "MyProj", pe.Project)
		assert.Equal(t, `missing "Comment build" permission in project MyProj`, err.Error())
	})

	T.Run("403 without parseable permission falls back to wire message", func(t *testing.T) {
		t.Parallel()
		body := []byte(`{"errors":[{"message":"Access denied."}]}`)
		err := errorFromBody(http.StatusForbidden, body)
		pe, ok := errors.AsType[*PermissionError](err)
		require.True(t, ok)
		assert.Empty(t, pe.Permission)
		assert.Equal(t, "Access denied.", err.Error())
	})

	T.Run("404 with locator parsed", func(t *testing.T) {
		t.Parallel()
		body := []byte(`{"errors":[{"message":"No build types found by locator 'Sandbox_Demo'."}]}`)
		err := errorFromBody(http.StatusNotFound, body)
		nf, ok := errors.AsType[*NotFoundError](err)
		require.True(t, ok)
		assert.Equal(t, "job", nf.Resource)
		assert.Equal(t, "Sandbox_Demo", nf.ID)
		assert.Equal(t, `job "Sandbox_Demo" not found`, err.Error())
	})

	T.Run("500 falls back to wire message", func(t *testing.T) {
		t.Parallel()
		body := []byte(`{"errors":[{"message":"database down"}]}`)
		err := errorFromBody(http.StatusInternalServerError, body)
		he, ok := errors.AsType[*HTTPError](err)
		require.True(t, ok)
		assert.Equal(t, CatInternal, he.Category())
		assert.Equal(t, "database down", err.Error())
	})
}

// TestClient_HandleErrorResponse_StampsAuthSource verifies that PermissionError carries the client's AuthSource.
func TestClient_HandleErrorResponse_StampsAuthSource(T *testing.T) {
	T.Parallel()

	body := `{"errors":[{"message":"You do not have \"Comment build\" permission in project with internal id: 'p1'"}]}`
	resp := &http.Response{
		StatusCode: http.StatusForbidden,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
	c := &Client{AuthSource: AuthSourcePKCE}
	err := c.handleErrorResponse(resp)
	pe, ok := errors.AsType[*PermissionError](err)
	require.True(T, ok)
	assert.Equal(T, AuthSourcePKCE, pe.AuthSource)
	assert.Equal(T, "Comment build", pe.Permission)
}

// TestErrReadOnlySentinel covers the sentinel contract: equality, %w wrap, Category().
func TestErrReadOnlySentinel(T *testing.T) {
	T.Parallel()

	assert.True(T, errors.Is(ErrReadOnly, ErrReadOnly))

	wrapped := fmt.Errorf("%w: PUT /foo", ErrReadOnly)
	assert.True(T, errors.Is(wrapped, ErrReadOnly))

	ue, ok := errors.AsType[UserError](wrapped)
	require.True(T, ok)
	assert.Equal(T, CatReadOnly, ue.Category())
}
