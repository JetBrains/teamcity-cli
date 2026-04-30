package auth

import (
	"encoding/base64"
	"regexp"
	"testing"

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
