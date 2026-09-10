package auth

import (
	"encoding/base64"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/JetBrains/teamcity-cli/api"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPkceCodeChallenge_RFC7636Vector pins the verifier→challenge derivation against the RFC 7636 §B worked example.
func TestPkceCodeChallenge_RFC7636Vector(t *testing.T) {
	t.Parallel()

	const verifier = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	const want = "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"

	got := pkceCodeChallenge(verifier)
	assert.Equal(t, want, got, "RFC 7636 §B test vector failed — PKCE is broken")
}

var verifierAlphabet = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func TestPkceScopePicker(t *testing.T) {
	t.Parallel()
	for _, extra := range []string{"", "EDIT_VERSIONED_SETTINGS", "CHANGE_SERVER_SETTINGS"} {
		t.Run("optional="+extra, func(t *testing.T) {
			selected := api.DefaultScopes()
			picker := newPkceScopePicker(&selected)
			picker.WithKeyMap(huh.NewDefaultKeyMap())
			if extra != "" {
				_, _ = picker.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
				_, _ = picker.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(extra)})
				_, _ = picker.Update(tea.KeyMsg{Type: tea.KeyEnter})
				assert.Contains(t, picker.View(), extra)
				_, _ = picker.Update(tea.KeyMsg{Type: tea.KeySpace})
			}
			_, _ = picker.Update(tea.KeyMsg{Type: tea.KeyEnter})
			want := api.DefaultScopes()
			if extra != "" {
				want = append(want, extra)
			}
			assert.ElementsMatch(t, want, selected)
			u, err := url.Parse(api.BuildAuthorizeURL("https://tc.example", "http://localhost", "challenge", "state", selected))
			require.NoError(t, err)
			assert.Equal(t, strings.Join(selected, " "), u.Query().Get("scope"))
		})
	}
}

// TestGeneratePkceVerifier_RFC7636Constraints pins length (43 chars), alphabet ([A-Za-z0-9_-]), and uniqueness per RFC 7636 §4.1.
func TestGeneratePkceVerifier_RFC7636Constraints(t *testing.T) {
	t.Parallel()

	const samples = 1000
	seen := make(map[string]struct{}, samples)

	for range samples {
		v, err := generatePkceVerifier()
		require.NoError(t, err)
		assert.Len(t, v, 43, "verifier must be 43 chars (32 raw bytes, base64url no-padding)")
		assert.True(t, verifierAlphabet.MatchString(v), "verifier %q has illegal chars", v)

		raw, err := base64.RawURLEncoding.DecodeString(v)
		require.NoError(t, err)
		assert.Len(t, raw, 32)

		_, dup := seen[v]
		assert.False(t, dup, "duplicate verifier — randomness is broken")
		seen[v] = struct{}{}
	}
}

func TestDescribeScope(t *testing.T) {
	t.Parallel()

	t.Run("known scope → description + faint enum", func(t *testing.T) {
		got := describeScope("VIEW_PROJECT")
		assert.Contains(t, got, "View project")
		assert.Contains(t, got, "VIEW_PROJECT")
	})

	t.Run("unknown scope → returns scope verbatim", func(t *testing.T) {
		got := describeScope("CUSTOM_FUTURE_SCOPE_X")
		assert.Equal(t, "CUSTOM_FUTURE_SCOPE_X", got)
	})
}
