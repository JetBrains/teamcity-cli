package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateMute(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name           string
		scope          ProblemScopeOptions
		opts           MuteOptions
		wantResolution string
		wantReason     string
		assertScope    func(t *testing.T, m Mute)
	}{
		{
			name:           "project_scope_default_manually",
			scope:          ProblemScopeOptions{Project: "Proj"},
			opts:           MuteOptions{Reason: "flaky"},
			wantResolution: "manually",
			wantReason:     "flaky",
			assertScope: func(t *testing.T, m Mute) {
				require.NotNil(t, m.Scope.Project)
				assert.Equal(t, "Proj", m.Scope.Project.ID)
				assert.Nil(t, m.Scope.BuildTypes)
			},
		},
		{
			name:           "job_scope_when_fixed",
			scope:          ProblemScopeOptions{Job: "Build_Cfg"},
			opts:           MuteOptions{Resolution: "whenFixed"},
			wantResolution: "whenFixed",
			assertScope: func(t *testing.T, m Mute) {
				require.NotNil(t, m.Scope.BuildTypes)
				require.Len(t, m.Scope.BuildTypes.BuildType, 1)
				assert.Equal(t, "Build_Cfg", m.Scope.BuildTypes.BuildType[0].ID)
				assert.Nil(t, m.Scope.Project)
			},
		},
		{
			name:           "job_takes_precedence",
			scope:          ProblemScopeOptions{Project: "Proj", Job: "Build_Cfg"},
			opts:           MuteOptions{},
			wantResolution: "manually",
			assertScope: func(t *testing.T, m Mute) {
				require.NotNil(t, m.Scope.BuildTypes)
				assert.Nil(t, m.Scope.Project)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var sent Mute
			client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/app/rest/mutes" {
					http.NotFound(w, r)
					return
				}
				require.NoError(t, json.NewDecoder(r.Body).Decode(&sent))
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(Mute{ID: 7})
			})

			mute, err := client.CreateMute(t.Context(), "testid_1", tc.scope, tc.opts)
			require.NoError(t, err)
			assert.Equal(t, 7, mute.ID)

			require.NotNil(t, sent.Resolution)
			assert.Equal(t, tc.wantResolution, sent.Resolution.Type)
			require.NotNil(t, sent.Target)
			require.NotNil(t, sent.Target.Tests)
			require.Len(t, sent.Target.Tests.Test, 1)
			assert.Equal(t, "testid_1", sent.Target.Tests.Test[0].ID)

			if tc.wantReason != "" {
				require.NotNil(t, sent.Assignment)
				assert.Equal(t, tc.wantReason, sent.Assignment.Text)
			} else {
				assert.Nil(t, sent.Assignment)
			}

			require.NotNil(t, sent.Scope)
			tc.assertScope(t, sent)
		})
	}
}

func TestCreateMute_AtTime(t *testing.T) {
	t.Parallel()

	var sent Mute
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&sent))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Mute{ID: 9})
	})

	_, err := client.CreateMute(t.Context(), "testid_1", ProblemScopeOptions{Project: "Proj"}, MuteOptions{
		Resolution:     "atTime",
		ResolutionTime: "20260121T000000+0000",
	})
	require.NoError(t, err)
	require.NotNil(t, sent.Resolution)
	assert.Equal(t, "atTime", sent.Resolution.Type)
	assert.Equal(t, "20260121T000000+0000", sent.Resolution.Time)
}

func TestCreateMute_RequiresScope(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
	})

	_, err := client.CreateMute(t.Context(), "testid_1", ProblemScopeOptions{}, MuteOptions{})
	require.Error(t, err)
	var ve *ValidationError
	require.ErrorAs(t, err, &ve)
}

func TestListMutes(t *testing.T) {
	t.Parallel()

	var gotLocator, gotFields string
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/app/rest/mutes" {
			http.NotFound(w, r)
			return
		}
		gotLocator = r.URL.Query().Get("locator")
		gotFields = r.URL.Query().Get("fields")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Mutes{
			Count: 1,
			Mute:  []Mute{{ID: 42}},
		})
	})

	mutes, err := client.ListMutes(t.Context(), "testid_1", ProblemScopeOptions{Project: "Proj"})
	require.NoError(t, err)
	require.Len(t, mutes.Mute, 1)
	assert.Equal(t, 42, mutes.Mute[0].ID)
	assert.Equal(t, "test:(id:testid_1),affectedProject:(id:Proj)", gotLocator)
	assert.Equal(t, "count,mute(id,scope(project(id),buildTypes(buildType(id))),target(tests(test(id))),resolution(type),assignment(text))", gotFields)
}

func TestListMutes_JobScope(t *testing.T) {
	t.Parallel()

	var gotLocator string
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotLocator = r.URL.Query().Get("locator")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Mutes{})
	})

	_, err := client.ListMutes(t.Context(), "testid_1", ProblemScopeOptions{Job: "Build_Cfg"})
	require.NoError(t, err)
	assert.Equal(t, "test:(id:testid_1),buildType:(id:Build_Cfg)", gotLocator)
}

func TestListMutes_RequiresScope(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
	})

	_, err := client.ListMutes(t.Context(), "testid_1", ProblemScopeOptions{})
	require.Error(t, err)
	var ve *ValidationError
	require.ErrorAs(t, err, &ve)
}

func TestDeleteMute(t *testing.T) {
	t.Parallel()

	var gotMethod, gotPath string
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	})

	err := client.DeleteMute(t.Context(), 42)
	require.NoError(t, err)
	assert.Equal(t, http.MethodDelete, gotMethod)
	assert.Equal(t, "/app/rest/mutes/id:42", gotPath)
}
