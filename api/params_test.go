package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetProjectParameters(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/app/rest/projects/id:MyProject/parameters")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ParameterList{
			Count:    2,
			Property: []Parameter{{Name: "env.FOO", Value: "bar"}, {Name: "env.BAZ", Value: "qux"}},
		})
	})

	params, err := client.GetProjectParameters("MyProject")
	require.NoError(t, err)
	assert.Equal(t, 2, params.Count)
	assert.Equal(t, "env.FOO", params.Property[0].Name)
}

func TestGetProjectParameter(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/app/rest/projects/id:MyProject/parameters/env.FOO")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Parameter{Name: "env.FOO", Value: "bar"})
	})

	p, err := client.GetProjectParameter("MyProject", "env.FOO")
	require.NoError(t, err)
	assert.Equal(t, "bar", p.Value)
}

func TestSetProjectParameter(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		assert.Contains(t, r.URL.Path, "/app/rest/projects/id:MyProject/parameters/env.FOO")
		body, _ := io.ReadAll(r.Body)
		assert.Contains(t, string(body), `"name":"env.FOO"`)
		assert.Contains(t, string(body), `"value":"newval"`)
		w.WriteHeader(http.StatusNoContent)
	})

	err := client.SetProjectParameter("MyProject", "env.FOO", "newval", false)
	require.NoError(t, err)
}

func TestSetProjectParameterSecure(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		assert.Contains(t, string(body), `"rawValue":"password"`)
		w.WriteHeader(http.StatusNoContent)
	})

	err := client.SetProjectParameter("MyProject", "secret", "s3cret", true)
	require.NoError(t, err)
}

func TestDeleteProjectParameter(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		assert.Contains(t, r.URL.Path, "/app/rest/projects/id:MyProject/parameters/env.FOO")
		w.WriteHeader(http.StatusNoContent)
	})

	err := client.DeleteProjectParameter("MyProject", "env.FOO")
	require.NoError(t, err)
}

func TestGetBuildTypeParameters(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/app/rest/buildTypes/id:MyBuild/parameters")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ParameterList{Count: 1, Property: []Parameter{{Name: "p1", Value: "v1"}}})
	})

	params, err := client.GetBuildTypeParameters("MyBuild")
	require.NoError(t, err)
	assert.Equal(t, 1, params.Count)
}

func TestGetBuildTypeParameter(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Parameter{Name: "p1", Value: "v1"})
	})

	p, err := client.GetBuildTypeParameter("MyBuild", "p1")
	require.NoError(t, err)
	assert.Equal(t, "v1", p.Value)
}

func TestSetBuildTypeParameter(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		w.WriteHeader(http.StatusNoContent)
	})

	err := client.SetBuildTypeParameter("MyBuild", "p1", "v1", false)
	require.NoError(t, err)
}

func TestDeleteBuildTypeParameter(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		w.WriteHeader(http.StatusNoContent)
	})

	err := client.DeleteBuildTypeParameter("MyBuild", "p1")
	require.NoError(t, err)
}

func TestGetParameterValue(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("my-value"))
	})

	val, err := client.GetParameterValue("/app/rest/projects/id:P/parameters/env.X/value")
	require.NoError(t, err)
	assert.Equal(t, "my-value", val)
}

func TestGetParameterValueError(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]any{
			"errors": []map[string]string{{"message": "not found"}},
		})
	})

	_, err := client.GetParameterValue("/app/rest/projects/id:P/parameters/missing/value")
	require.Error(t, err)
}

func TestParameterTypeJSON(t *testing.T) {
	t.Parallel()
	p := Parameter{
		Name:  "secret",
		Value: "hidden",
		Type:  &ParameterType{RawValue: "password"},
	}
	data, err := json.Marshal(p)
	require.NoError(t, err)
	assert.True(t, strings.Contains(string(data), `"rawValue":"password"`))
}
