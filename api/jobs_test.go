package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetBuildTypes(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/app/rest/buildTypes")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(BuildTypeList{
			Count:      1,
			BuildTypes: []BuildType{{ID: "bt1", Name: "Build"}},
		})
	})

	result, _, err := client.GetBuildTypes(BuildTypesOptions{Project: "MyProject"})
	require.NoError(t, err)
	assert.Equal(t, 1, result.Count)
}

func TestGetBuildTypesVcsRootURLFilter(t *testing.T) {
	t.Parallel()
	var seenLocator string
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		seenLocator = r.URL.Query().Get("locator")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(BuildTypeList{Count: 0})
	})

	_, _, err := client.GetBuildTypes(BuildTypesOptions{VcsRootURL: "acme/repo"})
	require.NoError(t, err)
	assert.Contains(t, seenLocator, "vcsRoot:(property:(name:url,value:acme/repo,matchType:contains))")
}

func TestGetBuildType(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/app/rest/buildTypes/id:bt1")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(BuildType{ID: "bt1", Name: "Build", ProjectID: "P1"})
	})

	bt, err := client.GetBuildType("bt1")
	require.NoError(t, err)
	assert.Equal(t, "Build", bt.Name)
}

func TestSetBuildTypePaused(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		assert.Contains(t, r.URL.Path, "/app/rest/buildTypes/id:bt1/paused")
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
	})

	err := client.SetBuildTypePaused("bt1", true)
	require.NoError(t, err)
}

func TestBuildTypeExists(t *testing.T) {
	t.Parallel()

	t.Run("exists", func(t *testing.T) {
		t.Parallel()
		client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(BuildType{ID: "bt1"})
		})
		assert.True(t, client.BuildTypeExists("bt1"))
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"errors": []map[string]string{{"message": "not found"}}})
		})
		assert.False(t, client.BuildTypeExists("missing"))
	})
}

func TestCreateBuildType(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/app/rest/buildTypes", r.URL.Path)
		var req CreateBuildTypeRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		require.NotNil(t, req.Project)
		assert.Equal(t, "P1", req.Project.ID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(BuildType{ID: "P1_NewBuild", Name: "NewBuild"})
	})

	bt, err := client.CreateBuildType("P1", CreateBuildTypeRequest{Name: "NewBuild"})
	require.NoError(t, err)
	assert.Equal(t, "P1_NewBuild", bt.ID)
}

func TestCreateBuildStep(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/app/rest/buildTypes/id:bt1/steps")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(BuildStep{ID: "RUNNER_1", Name: "test step", Type: "simpleRunner"})
	})

	step, err := client.CreateBuildStep("bt1", BuildStep{Name: "test step", Type: "simpleRunner"})
	require.NoError(t, err)
	assert.Equal(t, "RUNNER_1", step.ID)
}

func TestGetBuildSteps(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/app/rest/buildTypes/id:bt1/steps")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(BuildStepList{Count: 2, Step: []BuildStep{
			{ID: "RUNNER_1", Name: "Compile", Type: "gradle"},
			{ID: "RUNNER_2", Name: "Run Tests", Type: "simpleRunner", Disabled: true},
		}})
	})

	steps, err := client.GetBuildSteps("bt1")
	require.NoError(t, err)
	assert.Equal(t, 2, steps.Count)
	assert.Equal(t, "Compile", steps.Step[0].Name)
	assert.True(t, steps.Step[1].Disabled)
}

func TestGetBuildStep(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/app/rest/buildTypes/id:bt1/steps/RUNNER_1")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(BuildStep{ID: "RUNNER_1", Name: "Compile", Type: "gradle"})
	})

	step, err := client.GetBuildStep("bt1", "RUNNER_1")
	require.NoError(t, err)
	assert.Equal(t, "Compile", step.Name)
	assert.Equal(t, "gradle", step.Type)
}

func TestDeleteBuildStep(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		assert.Contains(t, r.URL.Path, "/app/rest/buildTypes/id:bt1/steps/RUNNER_1")
		w.WriteHeader(http.StatusNoContent)
	})

	err := client.DeleteBuildStep("bt1", "RUNNER_1")
	require.NoError(t, err)
}

func TestGetSnapshotDependencies(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/app/rest/buildTypes/id:bt1/snapshot-dependencies")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(SnapshotDependencyList{Count: 0})
	})

	deps, err := client.GetSnapshotDependencies("bt1")
	require.NoError(t, err)
	assert.Equal(t, 0, deps.Count)
}

func TestGetDependentBuildTypes(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/app/rest/buildTypes")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(BuildTypeList{Count: 0})
	})

	result, err := client.GetDependentBuildTypes("bt1")
	require.NoError(t, err)
	assert.Equal(t, 0, result.Count)
}

func TestGetVcsRootEntries(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/app/rest/buildTypes/id:bt1/vcs-root-entries")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(VcsRootEntries{Count: 0})
	})

	entries, err := client.GetVcsRootEntries("bt1")
	require.NoError(t, err)
	assert.Equal(t, 0, entries.Count)
}

func TestVcsRootEntryDecodesNestedProperties(t *testing.T) {
	t.Parallel()
	const payload = `{
		"id": "Repo",
		"vcs-root": {
			"id": "Repo",
			"name": "repo",
			"properties": {
				"property": [
					{"name": "url", "value": "git@github.com:acme/repo.git"},
					{"name": "branch", "value": "refs/heads/main"}
				]
			}
		}
	}`
	var entry VcsRootEntry
	require.NoError(t, json.Unmarshal([]byte(payload), &entry))
	require.NotNil(t, entry.VcsRoot)
	require.NotNil(t, entry.VcsRoot.Properties)

	got := map[string]string{}
	for _, p := range entry.VcsRoot.Properties.Property {
		got[p.Name] = p.Value
	}
	assert.Equal(t, "git@github.com:acme/repo.git", got["url"])
	assert.Equal(t, "refs/heads/main", got["branch"])
}

func TestSetBuildTypeSetting(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		assert.Contains(t, r.URL.Path, "/app/rest/buildTypes/id:bt1/settings/maxRunningBuilds")
		w.WriteHeader(http.StatusNoContent)
	})

	err := client.SetBuildTypeSetting("bt1", "maxRunningBuilds", "3")
	require.NoError(t, err)
}

func TestGetBuildStepsEmptyIsNonNil(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		// Server omits the "step" key for a job with no steps.
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"count":0}`))
	})

	steps, err := client.GetBuildSteps("bt1")
	require.NoError(t, err)
	assert.NotNil(t, steps.Step)
	b, err := json.Marshal(steps)
	require.NoError(t, err)
	assert.Contains(t, string(b), `"step":[]`)
}
