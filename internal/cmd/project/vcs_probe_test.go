package project

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSavedVcsRootCredentials(t *testing.T) {
	for _, auth := range []string{"ACCESS_TOKEN", "PASSWORD", "PRIVATE_KEY_FILE"} {
		t.Run(auth, func(t *testing.T) {
			t.Parallel()
			properties := []api.Property{{Name: "authMethod", Value: auth}, {Name: "url", Value: "https://example.test/private"}, {Name: "branch", Value: "%checkout.branch%"}}
			secret := ""
			switch auth {
			case "ACCESS_TOKEN":
				properties = append(properties, api.Property{Name: "tokenId", Value: "tc_token_id:CID_root:stored"})
			case "PASSWORD":
				secret = "secure:password"
			case "PRIVATE_KEY_FILE":
				secret = "secure:passphrase"
			}
			if secret != "" {
				properties = append(properties, api.Property{Name: secret, Value: ""})
			}
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				assert.Equal(t, "/admin/editVcsRoot.html", r.URL.Path)
				assert.Equal(t, "editVcsRoot", r.URL.Query().Get("action"))
				assert.Equal(t, "root & special", r.URL.Query().Get("vcsRootId"))
				if r.Method == http.MethodGet {
					_, _ = fmt.Fprint(w, `<html><input name="publicKey" type="hidden" value="key">`)
					if secret != "" {
						_, _ = fmt.Fprintf(w, `<input value="encrypted&amp;value" type="hidden" name="prop:encrypted:%s">`, secret)
					}
					_, _ = fmt.Fprint(w, `</html>`)
					return
				}
				assert.Equal(t, http.MethodPost, r.Method)
				require.NoError(t, r.ParseForm())
				assert.Equal(t, "testConnection", r.PostForm.Get("submitVcsRoot"))
				assert.Equal(t, "editProject:child", r.PostForm.Get("editingScope"))
				assert.Equal(t, "key", r.PostForm.Get("publicKey"))
				assert.Equal(t, auth, r.PostForm.Get("prop:authMethod"))
				assert.Equal(t, "https://example.test/private", r.PostForm.Get("prop:url"))
				assert.Equal(t, "%checkout.branch%", r.PostForm.Get("prop:branch"))
				if secret != "" {
					assert.Equal(t, "encrypted&value", r.PostForm.Get("prop:encrypted:"+secret))
					assert.False(t, r.PostForm.Has("prop:"+secret))
				} else {
					assert.Equal(t, "tc_token_id:CID_root:stored", r.PostForm.Get("prop:tokenId"))
				}
				_, _ = fmt.Fprint(w, `<response><testConnectionResult/></response>`)
			}))
			t.Cleanup(server.Close)
			f := &cmdutil.Factory{}
			f.SetContext(t.Context())
			err := testSavedVcsRoot(f, api.NewClient(server.URL, ""), &api.VcsRoot{
				ID: "root & special", Project: &api.Project{ID: "child"}, Properties: &api.PropertyList{Property: properties},
			})
			require.NoError(t, err)
			assert.EqualValues(t, 2, requests.Load())
		})
	}
}

func TestSavedVcsRootFailures(t *testing.T) {
	t.Setenv("TEAMCITY_URL", "https://teamcity.example.test")
	for _, test := range []struct {
		name, page, result, want string
		status                   int
		secure                   bool
	}{
		{name: "login page", page: `<html>Sign in</html>`, want: "browser sign-in"},
		{name: "blocked editor", status: 403, want: "HTTP 403"},
		{name: "missing credential", secure: true, want: "encrypted credentials"},
		{name: "login after submit", result: `<html>Sign in</html>`, want: "browser sign-in"},
		{name: "empty response", result: `<response/>`, want: "did not contain a test result"},
		{name: "wrong XML root", result: `<other><testConnectionResult/></other>`, want: "did not return a test result"},
		{name: "authentication failure", result: `<response><errors><error id="errorConnectionTest">private repo: not authorized</error></errors></response>`, want: "test connection failed: private repo: not authorized"},
		{name: "error despite result", result: `<response><testConnectionResult/><errors><error id="url">invalid URL</error></errors></response>`, want: "test connection failed: invalid URL"},
		{name: "expired key", result: `<response><errors><error id="publicKeyExpired">new-key</error></errors></response>`, want: "encryption key changed; retry"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if test.status != 0 {
					w.WriteHeader(test.status)
					return
				}
				if r.Method == http.MethodGet {
					page := test.page
					if page == "" {
						page = `<input type="hidden" name="publicKey" value="key">`
					}
					_, _ = fmt.Fprint(w, page)
					return
				}
				assert.False(t, test.secure, "must not test with missing credentials")
				_, _ = fmt.Fprint(w, test.result)
			}))
			t.Cleanup(server.Close)
			properties := &api.PropertyList{}
			if test.secure {
				properties.Property = []api.Property{{Name: "secure:password"}}
			}
			f := &cmdutil.Factory{}
			f.SetContext(t.Context())
			err := testSavedVcsRoot(f, api.NewClient(server.URL, ""), &api.VcsRoot{ID: "root", Project: &api.Project{ID: "child"}, Properties: properties})
			require.ErrorContains(t, err, test.want)
		})
	}
}
