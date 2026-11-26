package auth_test

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmd"
	"github.com/JetBrains/teamcity-cli/internal/cmdtest"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/config"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInsecureTLSLoginAndSubsequentCommands(t *testing.T) {
	for _, method := range []string{"token", "web", "guest"} {
		t.Run(method, func(t *testing.T) {
			ts := newWebLoginServer(t, true)
			handler := ts.Server.Config.Handler
			ts.Server.Close()
			ts.Server = httptest.NewUnstartedServer(handler)
			ts.Server.Config.ErrorLog = log.New(io.Discard, "", 0)
			ts.Server.StartTLS()
			t.Setenv("TEAMCITY_GUEST", "")
			t.Setenv("TEAMCITY_RO", "")
			t.Cleanup(config.ResetForTest)
			ts.Handle("GET /guestAuth/app/rest/server", func(w http.ResponseWriter, r *http.Request) {
				cmdtest.JSON(w, api.Server{VersionMajor: 2026, VersionMinor: 2})
			})
			for _, prefix := range []string{"", "/guestAuth"} {
				ts.Handle("GET "+prefix+"/app/rest/projects", func(w http.ResponseWriter, r *http.Request) {
					cmdtest.JSON(w, api.ProjectList{Count: 1, Projects: []api.Project{{ID: "Test", Name: "TLS Test"}}})
				})
			}
			approveInBrowser(t, nil)

			var stdout, stderr bytes.Buffer
			f := cmdutil.NewFactory()
			f.NoInput = true
			f.SkipLinkLookup()
			f.Printer = &output.Printer{Out: &stdout, ErrOut: &stderr}
			run := func(args ...string) error {
				stdout.Reset()
				stderr.Reset()
				root := cmd.NewCommand(f)
				root.SetArgs(args)
				return root.Execute()
			}
			args := []string{"auth", "login", "--server", ts.URL, "--json", "--insecure-storage"}
			switch method {
			case "token":
				args = append(args, "--token", "web-token")
			case "web":
				args = append(args, "--web")
			case "guest":
				args = append(args, "--guest")
			}
			require.ErrorContains(t, run(args...), "TLS certificate")
			require.NoError(t, run(append(args, "--insecure")...))
			assert.Contains(t, stdout.String(), `"event":"login"`)

			require.NoError(t, run("auth", "status", "--json", "--insecure"))
			assert.NotContains(t, stdout.String(), `"status": "error"`)
			assert.Contains(t, stdout.String(), `"server_info"`)
			require.NoError(t, run("project", "list", "--json", "--insecure"))
			assert.Contains(t, stdout.String(), "TLS Test")
			require.ErrorContains(t, run("project", "list", "--json"), "certificate")
		})
	}
}
