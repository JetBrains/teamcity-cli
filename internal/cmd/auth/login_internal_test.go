package auth

import (
	"bytes"
	"testing"

	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/stretchr/testify/assert"
)

func TestAnnounceAuthorizeURLIgnoresQuiet(t *testing.T) {
	var buf bytes.Buffer
	announceAuthorizeURL(&output.Printer{Out: &buf, ErrOut: &buf, Quiet: true})("https://tc.example.com/pkce/authorize.html?state=x", false)
	assert.Equal(t, "https://tc.example.com/pkce/authorize.html?state=x\n", buf.String())
}
