package project

import (
	"bytes"
	"errors"
	"testing"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthorizeURLFallback(t *testing.T) {
	for _, tt := range []struct {
		name      string
		noInput   bool
		openError error
	}{
		{name: "success"}, {name: "browser failure", openError: errors.New("cannot launch")}, {name: "no input", noInput: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			previous := openBrowser
			t.Cleanup(func() { openBrowser = previous })
			calls := 0
			openBrowser = func(rawURL string) error { calls++; return tt.openError }
			var out, stderr bytes.Buffer
			f := &cmdutil.Factory{NoInput: tt.noInput, Printer: &output.Printer{Out: &out, ErrOut: &stderr}}
			err := openConnectionAuthorize(f, api.NewClient("https://tc.example/context", "unused"), "Project +", "PROJECT_EXT_42", "GitHubApp")
			require.NoError(t, err)
			assert.Contains(t, out.String(), "https://tc.example/context/oauth/githubapp/repositories.html?projectId=Project+%2B&connectionId=PROJECT_EXT_42")
			if tt.noInput {
				assert.Zero(t, calls)
				assert.NotContains(t, out.String(), "Opening browser")
			} else {
				assert.Equal(t, 1, calls)
			}
		})
	}
}
