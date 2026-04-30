package output_test

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/output"
)

// httpErr builds a typed api error via the real parser (only way to set the unexported Category).
func httpErr(t *testing.T, status int, body string) error {
	t.Helper()
	resp := &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
	return api.ErrorFromResponse(resp)
}

// permErr returns a *api.PermissionError with AuthSource set, routed via ErrorFromResponse so cat is populated.
func permErr(t *testing.T, body string, src api.AuthSource) *api.PermissionError {
	t.Helper()
	var pe *api.PermissionError
	ok := errors.As(httpErr(t, http.StatusForbidden, body), &pe)
	require.True(t, ok, "expected *api.PermissionError")
	pe.AuthSource = src
	return pe
}

func notFoundErr(t *testing.T, message string) *api.NotFoundError {
	t.Helper()
	body := `{"errors":[{"message":` + jsonString(message) + `}]}`
	var nf *api.NotFoundError
	ok := errors.As(httpErr(t, http.StatusNotFound, body), &nf)
	require.True(t, ok, "expected *api.NotFoundError")
	return nf
}

func jsonString(s string) string {
	// minimal JSON escape — we control inputs so this is enough.
	return `"` + strings.ReplaceAll(strings.ReplaceAll(s, `\`, `\\`), `"`, `\"`) + `"`
}

// TestClassifyError_Categories pins the (code, tip) pair for each error class.
func TestClassifyError_Categories(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		err      error
		wantCode output.JSONErrorCode
		wantTip  string // substring expected in tip; "" means no tip
	}{
		{
			name:     "401 → auth re-login",
			err:      httpErr(t, http.StatusUnauthorized, ""),
			wantCode: output.ErrCodeAuth,
			wantTip:  "teamcity auth login",
		},
		{
			name:     "403 generic → broader permissions",
			err:      httpErr(t, http.StatusForbidden, ""),
			wantCode: output.ErrCodePermission,
			wantTip:  "broader permissions",
		},
		{
			name:     "404 with no resource → fallback tip",
			err:      httpErr(t, http.StatusNotFound, ""),
			wantCode: output.ErrCodeNotFound,
			wantTip:  "teamcity",
		},
		{
			name:     "read-only sentinel",
			err:      api.ErrReadOnly,
			wantCode: output.ErrCodeReadOnly,
			wantTip:  "TEAMCITY_RO",
		},
		{
			name:     "network error generic",
			err:      &api.NetworkError{URL: "https://x", Cause: errors.New("dial tcp: timeout")},
			wantCode: output.ErrCodeNetwork,
			wantTip:  "network connection",
		},
		{
			name:     "network error sandbox-blocked",
			err:      &api.NetworkError{URL: "https://x", Cause: errors.New("dial tcp: Forbidden")},
			wantCode: output.ErrCodeNetwork,
			wantTip:  "sandbox",
		},
		{
			name:     "validation with explicit tip",
			err:      api.Validation("bad input", "Pass --foo instead"),
			wantCode: output.ErrCodeValidation,
			wantTip:  "Pass --foo instead",
		},
		{
			name:     "non-typed error → internal",
			err:      errors.New("something broke"),
			wantCode: output.ErrCodeInternal,
			wantTip:  "",
		},
		{
			name:     "cobra-style 'unknown command' → validation, no tip",
			err:      errors.New("unknown command \"foo\""),
			wantCode: output.ErrCodeValidation,
			wantTip:  "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			code, msg, tip := output.ClassifyError(tc.err)
			assert.Equal(t, tc.wantCode, code, "code")
			assert.NotEmpty(t, msg, "message must not be empty")
			if tc.wantTip == "" {
				assert.Empty(t, tip, "expected no tip")
			} else {
				assert.Contains(t, tip, tc.wantTip, "tip should contain %q", tc.wantTip)
			}
		})
	}
}

// TestClassifyError_PermissionAuthSource pins the per-AuthSource branch in permissionTip.
func TestClassifyError_PermissionAuthSource(t *testing.T) {
	t.Parallel()

	body := `{"errors":[{"message":"You do not have \"Run build\" permission in project My_Project"}]}`

	cases := []struct {
		src     api.AuthSource
		wantTip string
	}{
		{api.AuthSourcePKCE, "permissions picker"},
		{api.AuthSourceEnv, "TEAMCITY_TOKEN"},
		{api.AuthSourceBuild, "Build-level credentials can't be widened"},
		{api.AuthSourceGuest, "Guest access lacks"},
		{api.AuthSourceManual, "Generate a new access token"},
		{api.AuthSourceUnknown, "Generate a new access token"},
	}
	for _, tc := range cases {
		t.Run(string(tc.src), func(t *testing.T) {
			t.Parallel()
			pe := permErr(t, body, tc.src)
			_, _, tip := output.ClassifyError(pe)
			assert.Contains(t, tip, tc.wantTip)
			// Tip must include the enum (RUN_BUILD) so it matches the picker's "(ENUM)" suffix.
			assert.Contains(t, tip, "RUN_BUILD")
		})
	}
}

// TestClassifyError_NotFoundCascade pins the order-dependent string-contains cascade in notFoundTip ("agent pool" must match before "agent").
func TestClassifyError_NotFoundCascade(t *testing.T) {
	t.Parallel()

	cases := []struct {
		message string
		wantTip string
	}{
		{"agent pool 'Default' not found", "teamcity pool list"},
		{"No pool found by locator 'id:99'", "teamcity pool list"},
		{"agent 'mac-01' not found", "teamcity agent list"},
		{"project 'X' not found", "teamcity project list"},
		{"build type 'Foo_Bar' not found", "teamcity job list"},
		{"job 'Foo' not found", "teamcity job list"},
		{"some other thing not found", "teamcity job list"}, // fallthrough
	}
	for _, tc := range cases {
		t.Run(tc.message, func(t *testing.T) {
			t.Parallel()
			nf := notFoundErr(t, tc.message)
			_, _, tip := output.ClassifyError(nf)
			assert.Contains(t, tip, tc.wantTip)
		})
	}
}

// TestClassifyError_NotFoundResource pins the resource-driven tip shortcut (parsed Resource bypasses the message cascade).
func TestClassifyError_NotFoundResource(t *testing.T) {
	t.Parallel()
	cases := []struct {
		resource string
		want     string
	}{
		{"run", "teamcity run list"},
		{"job", "teamcity job list"},
		{"project", "teamcity project list"},
		{"agent", "teamcity agent list"},
	}
	for _, tc := range cases {
		t.Run(tc.resource, func(t *testing.T) {
			t.Parallel()
			// Build via parser to populate Category, then override Resource/ID.
			nf := notFoundErr(t, "placeholder not found")
			nf.Resource = tc.resource
			nf.ID = "x"
			_, _, tip := output.ClassifyError(nf)
			assert.Contains(t, tip, tc.want)
		})
	}
}

// TestClassifyError_InputErrors pins the prefix/substring list that classifies cobra error strings as validation.
func TestClassifyError_InputErrors(t *testing.T) {
	t.Parallel()

	inputs := []string{
		"unknown command \"foo\" for \"teamcity\"",
		"unknown flag: --bogus",
		"required flag(s) \"server\" not set",
		"invalid argument \"x\" for --limit",
		"invalid status: foo",
		"accepts 1 arg(s), received 2",
		"if any flags in the group [a b] are set none of the others can be",
		"--limit must be > 0",
		"unknown fields: extra_thing",
		"foo: flag needs an argument",
		"--a and --b are mutually exclusive",
		"required (or use --no-input)",
		"key 'foo' not found in configuration",
	}
	for _, in := range inputs {
		t.Run(in, func(t *testing.T) {
			t.Parallel()
			code, _, _ := output.ClassifyError(errors.New(in))
			assert.Equal(t, output.ErrCodeValidation, code, "should classify as validation")
		})
	}

	t.Run("not_input/internal", func(t *testing.T) {
		t.Parallel()
		code, _, _ := output.ClassifyError(errors.New("kaboom"))
		assert.Equal(t, output.ErrCodeInternal, code)
	})
}

// TestRenderError pins that RenderError appends a FormatTip line when a tip exists, else returns the original error.
func TestRenderError(t *testing.T) {
	t.Parallel()

	t.Run("no tip → unchanged", func(t *testing.T) {
		t.Parallel()
		original := errors.New("kaboom")
		rendered := output.RenderError(original)
		assert.Equal(t, "kaboom", rendered.Error())
	})

	t.Run("with tip → appended", func(t *testing.T) {
		t.Parallel()
		rendered := output.RenderError(api.ErrReadOnly)
		got := rendered.Error()
		assert.Contains(t, got, "read-only mode")
		assert.Contains(t, got, "Tip:")
		assert.Contains(t, got, "TEAMCITY_RO")
	})

	t.Run("validation tip preserved", func(t *testing.T) {
		t.Parallel()
		rendered := output.RenderError(api.Validation("bad", "Use --baz"))
		got := rendered.Error()
		assert.Contains(t, got, "bad")
		assert.Contains(t, got, "Use --baz")
	})
}
