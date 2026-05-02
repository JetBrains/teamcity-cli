package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPinBuild(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			// ResolveBuildID calls GetBuilds
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(BuildList{Count: 1, Builds: []Build{{ID: 1, Number: "1"}}})
			return
		}
		assert.Equal(t, "PUT", r.Method)
		assert.Contains(t, r.URL.Path, "/pin")
		w.WriteHeader(http.StatusNoContent)
	})

	err := client.PinBuild("1", "pinned for release")
	require.NoError(t, err)
}

func TestUnpinBuild(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(BuildList{Count: 1, Builds: []Build{{ID: 1}}})
			return
		}
		assert.Equal(t, "DELETE", r.Method)
		assert.Contains(t, r.URL.Path, "/pin")
		w.WriteHeader(http.StatusNoContent)
	})

	err := client.UnpinBuild("1")
	require.NoError(t, err)
}

func TestAddBuildTags(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(BuildList{Count: 1, Builds: []Build{{ID: 1}}})
			return
		}
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/tags")
		w.WriteHeader(http.StatusOK)
	})

	err := client.AddBuildTags("1", []string{"release", "stable"})
	require.NoError(t, err)
}

func TestSetBuildComment(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(BuildList{Count: 1, Builds: []Build{{ID: 1}}})
			return
		}
		assert.Equal(t, "PUT", r.Method)
		assert.Contains(t, r.URL.Path, "/comment")
		w.WriteHeader(http.StatusNoContent)
	})

	err := client.SetBuildComment("1", "deployed to prod")
	require.NoError(t, err)
}

func TestDeleteBuildComment(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(BuildList{Count: 1, Builds: []Build{{ID: 1}}})
			return
		}
		assert.Equal(t, "DELETE", r.Method)
		assert.Contains(t, r.URL.Path, "/comment")
		w.WriteHeader(http.StatusNoContent)
	})

	err := client.DeleteBuildComment("1")
	require.NoError(t, err)
}

func TestGetBuildChanges(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/app/rest/builds" || r.URL.Path == "/httpAuth/app/rest/builds" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(BuildList{Count: 1, Builds: []Build{{ID: 1}}})
			return
		}
		assert.Contains(t, r.URL.Path, "/app/rest/changes")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ChangeList{Count: 1, Change: []Change{{ID: 42, Username: "dev"}}})
	})

	changes, err := client.GetBuildChanges(t.Context(), "1")
	require.NoError(t, err)
	assert.Equal(t, 1, changes.Count)
}

func TestGetBuildTests(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		opts         BuildTestsOptions
		wantLocators []string
	}{
		{
			name: "all",
			opts: BuildTestsOptions{Limit: 10},
			wantLocators: []string{
				"build:(id:1)",
				"build:(id:1),count:10",
			},
		},
		{
			name: "failed_only_excludes_muted",
			opts: BuildTestsOptions{FailedOnly: true, Limit: 10},
			wantLocators: []string{
				"build:(id:1),status:FAILURE,muted:false",
				"build:(id:1),status:FAILURE,muted:false,count:10",
			},
		},
		{
			name: "muted_only",
			opts: BuildTestsOptions{MutedOnly: true, Limit: 5},
			wantLocators: []string{
				"build:(id:1),status:FAILURE,muted:true",
				"build:(id:1),status:FAILURE,muted:true,count:5",
			},
		},
		{
			name: "no_limit_uses_summary_count",
			opts: BuildTestsOptions{},
			wantLocators: []string{
				"build:(id:1)",
				"build:(id:1),count:2",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var mu sync.Mutex
			var locators []string
			var fields []string

			client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path != "/app/rest/testOccurrences" {
					http.NotFound(w, r)
					return
				}

				field := r.URL.Query().Get("fields")
				mu.Lock()
				locators = append(locators, r.URL.Query().Get("locator"))
				fields = append(fields, field)
				mu.Unlock()

				if strings.Contains(field, "testOccurrence(") {
					json.NewEncoder(w).Encode(TestOccurrences{
						TestOccurrence: []TestOccurrence{{ID: "1", Name: "TestFoo", Status: "FAILURE", Muted: true}},
					})
					return
				}
				json.NewEncoder(w).Encode(TestOccurrences{Count: 2, Passed: 1, Failed: 1, Muted: 1})
			})

			tests, err := client.GetBuildTests(t.Context(), "1", tc.opts)
			require.NoError(t, err)
			assert.Equal(t, 1, tests.Failed)
			assert.Equal(t, 1, tests.Muted)
			require.Len(t, tests.TestOccurrence, 1)
			assert.True(t, tests.TestOccurrence[0].Muted)

			mu.Lock()
			defer mu.Unlock()
			assert.Equal(t, tc.wantLocators, locators)
			assert.Equal(t, []string{
				"count,passed,failed,ignored,muted",
				"testOccurrence(id,name,status,duration,details,newFailure,muted,firstFailed(build(id,number)))",
			}, fields)
		})
	}
}

func TestGetBuildTestsRejectsConflictingFilters(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
	})

	_, err := client.GetBuildTests(t.Context(), "1", BuildTestsOptions{FailedOnly: true, MutedOnly: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mutually exclusive")
}

func TestGetBuildProblems(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/app/rest/builds" || r.URL.Path == "/httpAuth/app/rest/builds" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(BuildList{Count: 1, Builds: []Build{{ID: 1}}})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ProblemOccurrences{
			Count: 1,
			ProblemOccurrence: []ProblemOccurrence{
				{ID: "1", Type: "TC_COMPILATION_ERROR", Details: "compile error"},
			},
		})
	})

	problems, err := client.GetBuildProblems("1")
	require.NoError(t, err)
	assert.Equal(t, 1, problems.Count)
}
