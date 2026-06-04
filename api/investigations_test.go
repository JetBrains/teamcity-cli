package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateInvestigation(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		scope        ProblemScopeOptions
		assignee     string
		wantAssignee string
		assertScope  func(t *testing.T, inv Investigation)
	}{
		{
			name:         "project_scope_with_assignee",
			scope:        ProblemScopeOptions{Project: "Proj"},
			assignee:     "alice",
			wantAssignee: "alice",
			assertScope: func(t *testing.T, inv Investigation) {
				require.NotNil(t, inv.Scope.Project)
				assert.Equal(t, "Proj", inv.Scope.Project.ID)
			},
		},
		{
			name:     "job_scope_no_assignee",
			scope:    ProblemScopeOptions{Job: "Build_Cfg"},
			assignee: "",
			assertScope: func(t *testing.T, inv Investigation) {
				require.NotNil(t, inv.Scope.BuildTypes)
				require.Len(t, inv.Scope.BuildTypes.BuildType, 1)
				assert.Equal(t, "Build_Cfg", inv.Scope.BuildTypes.BuildType[0].ID)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var sent Investigation
			client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/app/rest/investigations" {
					http.NotFound(w, r)
					return
				}
				require.NoError(t, json.NewDecoder(r.Body).Decode(&sent))
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(Investigation{ID: "inv1", State: "TAKEN"})
			})

			inv, err := client.CreateInvestigation(t.Context(), "testid_1", tc.scope, tc.assignee)
			require.NoError(t, err)
			assert.Equal(t, "inv1", inv.ID)

			assert.Equal(t, "TAKEN", sent.State)
			require.NotNil(t, sent.Target)
			require.NotNil(t, sent.Target.Tests)
			require.Len(t, sent.Target.Tests.Test, 1)
			assert.Equal(t, "testid_1", sent.Target.Tests.Test[0].ID)

			if tc.wantAssignee != "" {
				require.NotNil(t, sent.Assignee)
				assert.Equal(t, tc.wantAssignee, sent.Assignee.Username)
			} else {
				assert.Nil(t, sent.Assignee)
			}

			require.NotNil(t, sent.Scope)
			tc.assertScope(t, sent)
		})
	}
}

func TestCreateInvestigation_RequiresScope(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
	})

	_, err := client.CreateInvestigation(t.Context(), "testid_1", ProblemScopeOptions{}, "")
	require.Error(t, err)
	var ve *ValidationError
	require.ErrorAs(t, err, &ve)
}

func TestResolveInvestigation(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		state     string
		wantState string
	}{
		{name: "fixed", state: "FIXED", wantState: "FIXED"},
		{name: "given_up", state: "GIVEN_UP", wantState: "GIVEN_UP"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var listLocator, putPath string
			var putBody Investigation
			client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodGet && r.URL.Path == "/app/rest/investigations":
					listLocator = r.URL.Query().Get("locator")
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(Investigations{
						Count: 1,
						Investigation: []Investigation{{
							ID:    "inv1",
							State: "TAKEN",
							Scope: &ProblemScope{Project: &Project{ID: "Proj"}},
						}},
					})
				case r.Method == http.MethodPut && r.URL.Path == "/app/rest/investigations/id:inv1":
					putPath = r.URL.Path
					require.NoError(t, json.NewDecoder(r.Body).Decode(&putBody))
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(putBody)
				default:
					http.NotFound(w, r)
				}
			})

			err := client.ResolveInvestigation(t.Context(), "testid_1", ProblemScopeOptions{Project: "Proj"}, tc.state)
			require.NoError(t, err)
			assert.Equal(t, "test:(id:testid_1),affectedProject:(id:Proj)", listLocator)
			assert.Equal(t, "/app/rest/investigations/id:inv1", putPath)
			assert.Equal(t, tc.wantState, putBody.State)
		})
	}
}

func TestResolveInvestigation_NotFound(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/app/rest/investigations" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(Investigations{Count: 0})
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
	})

	err := client.ResolveInvestigation(t.Context(), "testid_1", ProblemScopeOptions{Project: "Proj"}, "FIXED")
	require.Error(t, err)
	var nf *NotFoundError
	require.ErrorAs(t, err, &nf)
}

func TestResolveInvestigation_RequiresScope(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
	})

	err := client.ResolveInvestigation(t.Context(), "testid_1", ProblemScopeOptions{}, "FIXED")
	require.Error(t, err)
	var ve *ValidationError
	require.ErrorAs(t, err, &ve)
}
