package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListTests(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		opts        TestQueryOptions
		wantLocator string
	}{
		{
			name:        "failing_by_project",
			opts:        TestQueryOptions{Project: "Proj", Failing: true, Limit: 10},
			wantLocator: "currentlyFailing:true,affectedProject:(id:Proj),count:10",
		},
		{
			name:        "muted_by_project",
			opts:        TestQueryOptions{Project: "Proj", Muted: true},
			wantLocator: "currentlyMuted:true,affectedProject:(id:Proj)",
		},
		{
			name:        "investigated_by_job",
			opts:        TestQueryOptions{Job: "Build_Cfg", Investigated: true},
			wantLocator: "currentlyInvestigated:true,buildType:(id:Build_Cfg)",
		},
		{
			name:        "job_takes_precedence_over_project",
			opts:        TestQueryOptions{Project: "Proj", Job: "Build_Cfg", Failing: true},
			wantLocator: "currentlyFailing:true,buildType:(id:Build_Cfg)",
		},
		{
			name:        "default_is_failing",
			opts:        TestQueryOptions{Project: "Proj"},
			wantLocator: "currentlyFailing:true,affectedProject:(id:Proj)",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotLocator, gotFields string
			client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path != "/app/rest/testOccurrences" {
					http.NotFound(w, r)
					return
				}
				gotLocator = r.URL.Query().Get("locator")
				gotFields = r.URL.Query().Get("fields")
				json.NewEncoder(w).Encode(TestOccurrences{
					Count: 1,
					TestOccurrence: []TestOccurrence{{
						ID:     "1",
						Name:   "com.example.FooTest",
						Status: "FAILURE",
						Build:  &Build{ID: 42, Number: "100", BuildType: &BuildType{ID: "Build_Cfg", Name: "Build"}},
					}},
				})
			})

			occ, err := client.ListTests(t.Context(), tc.opts)
			require.NoError(t, err)
			require.Len(t, occ.TestOccurrence, 1)
			assert.Equal(t, "Build", occ.TestOccurrence[0].Build.BuildType.Name)
			assert.Equal(t, tc.wantLocator, gotLocator)
			assert.Equal(t, "count,testOccurrence(id,name,status,duration,muted,newFailure,build(id,number,buildType(id,name)))", gotFields)
		})
	}
}

func TestListTests_RequiresScope(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request: %s", r.URL)
	})

	_, err := client.ListTests(t.Context(), TestQueryOptions{Failing: true})
	require.Error(t, err)
	var ve *ValidationError
	require.True(t, errors.As(err, &ve))
}

func TestGetTestHistory(t *testing.T) {
	t.Parallel()
	var gotLocator, gotFields string
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/app/rest/testOccurrences" {
			http.NotFound(w, r)
			return
		}
		gotLocator = r.URL.Query().Get("locator")
		gotFields = r.URL.Query().Get("fields")
		json.NewEncoder(w).Encode(TestOccurrences{
			Count: 2,
			TestOccurrence: []TestOccurrence{
				{Status: "SUCCESS", Duration: 100, Build: &Build{Number: "100"}},
				{Status: "FAILURE", Duration: 200, Build: &Build{Number: "99"}},
			},
		})
	})

	occ, err := client.GetTestHistory(t.Context(), "com.example.FooTest", TestQueryOptions{Job: "Build_Cfg", Limit: 25})
	require.NoError(t, err)
	require.Len(t, occ.TestOccurrence, 2)
	assert.Equal(t, "test:(name:com.example.FooTest),buildType:(id:Build_Cfg),count:25", gotLocator)
	assert.Equal(t, "count,testOccurrence(status,duration,muted,newFailure,build(id,number,branchName,startDate,agent(name)))", gotFields)
}

func TestGetTestHistory_RequiresScope(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request: %s", r.URL)
	})

	_, err := client.GetTestHistory(t.Context(), "com.example.FooTest", TestQueryOptions{})
	require.Error(t, err)
	var ve *ValidationError
	require.True(t, errors.As(err, &ve))
}

func TestResolveTestID(t *testing.T) {
	t.Parallel()

	t.Run("single_match", func(t *testing.T) {
		var gotLocator, gotFields string
		client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.URL.Path != "/app/rest/tests" {
				http.NotFound(w, r)
				return
			}
			gotLocator = r.URL.Query().Get("locator")
			gotFields = r.URL.Query().Get("fields")
			json.NewEncoder(w).Encode(TestList{Count: 1, Test: []TestRef{{ID: "-99", Name: "com.example.FooTest"}}})
		})

		id, err := client.ResolveTestID(t.Context(), "com.example.FooTest", "Proj")
		require.NoError(t, err)
		assert.Equal(t, "-99", id)
		assert.Equal(t, "name:com.example.FooTest,affectedProject:(id:Proj)", gotLocator)
		assert.Equal(t, "count,test(id,name)", gotFields)
	})

	t.Run("no_match", func(t *testing.T) {
		client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(TestList{Count: 0})
		})

		_, err := client.ResolveTestID(t.Context(), "missing", "Proj")
		require.Error(t, err)
		var nf *NotFoundError
		require.True(t, errors.As(err, &nf))
	})

	t.Run("ambiguous_match", func(t *testing.T) {
		client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(TestList{Count: 2, Test: []TestRef{
				{ID: "-1", Name: "com.example.FooTest"},
				{ID: "-2", Name: "com.example.FooTest"},
			}})
		})

		_, err := client.ResolveTestID(t.Context(), "com.example.FooTest", "")
		require.Error(t, err)
		var ambig *AmbiguousTestError
		require.True(t, errors.As(err, &ambig))
		assert.Equal(t, "com.example.FooTest", ambig.Name)
		require.Len(t, ambig.Candidates, 2)
		assert.Equal(t, "-1", ambig.Candidates[0].ID)
	})

	t.Run("no_project_scope_omits_affectedProject", func(t *testing.T) {
		var gotLocator string
		client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			gotLocator = r.URL.Query().Get("locator")
			json.NewEncoder(w).Encode(TestList{Count: 1, Test: []TestRef{{ID: "-5", Name: "Bar"}}})
		})

		_, err := client.ResolveTestID(t.Context(), "Bar", "")
		require.NoError(t, err)
		assert.Equal(t, "name:Bar", gotLocator)
	})
}
