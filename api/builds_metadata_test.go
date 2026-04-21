package api

import (
	"encoding/json"
	"net/http"
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
	callCount := 0
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/app/rest/builds" || r.URL.Path == "/httpAuth/app/rest/builds" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(BuildList{Count: 1, Builds: []Build{{ID: 1}}})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		callCount++
		if callCount <= 1 {
			// Summary call
			json.NewEncoder(w).Encode(TestOccurrences{Count: 2, Passed: 1, Failed: 1})
		} else {
			// Detail call
			json.NewEncoder(w).Encode(TestOccurrences{
				TestOccurrence: []TestOccurrence{{ID: "1", Name: "TestFoo", Status: "FAILURE"}},
			})
		}
	})

	tests, err := client.GetBuildTests(t.Context(), "1", true, 10)
	require.NoError(t, err)
	assert.Equal(t, 1, tests.Failed)
	assert.Len(t, tests.TestOccurrence, 1)
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
