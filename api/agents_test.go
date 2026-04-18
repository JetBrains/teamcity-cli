package api

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetAgents(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/app/rest/agents")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(AgentList{
			Count:  1,
			Agents: []Agent{{ID: 1, Name: "Agent-1", Connected: true}},
		})
	})

	result, err := client.GetAgents(AgentsOptions{})
	require.NoError(t, err)
	assert.Equal(t, 1, result.Count)
	assert.Equal(t, "Agent-1", result.Agents[0].Name)
}

func TestGetAgentsWithFilters(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.RawQuery
		assert.Contains(t, q, "authorized%3Atrue")
		assert.Contains(t, q, "connected%3Atrue")
		assert.Contains(t, q, "enabled%3Atrue")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(AgentList{Count: 0})
	})

	_, err := client.GetAgents(AgentsOptions{Authorized: true, Connected: true, Enabled: true})
	require.NoError(t, err)
}

func TestGetAgentsWithPoolFilter(t *testing.T) {
	t.Parallel()

	t.Run("by name", func(t *testing.T) {
		t.Parallel()
		client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			assert.Contains(t, r.URL.RawQuery, "pool%3A%28name%3ADefault%29")
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(AgentList{Count: 0})
		})
		_, err := client.GetAgents(AgentsOptions{Pool: "Default"})
		require.NoError(t, err)
	})

	t.Run("by id", func(t *testing.T) {
		t.Parallel()
		client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			assert.Contains(t, r.URL.RawQuery, "pool%3A%28id%3A5%29")
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(AgentList{Count: 0})
		})
		_, err := client.GetAgents(AgentsOptions{Pool: "5"})
		require.NoError(t, err)
	})
}

func TestGetAgent(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/app/rest/agents/id:42")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Agent{ID: 42, Name: "Agent-42"})
	})

	agent, err := client.GetAgent(42)
	require.NoError(t, err)
	assert.Equal(t, 42, agent.ID)
}

func TestGetAgentByName(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/app/rest/agents/name:my-agent")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Agent{ID: 1, Name: "my-agent"})
	})

	agent, err := client.GetAgentByName("my-agent")
	require.NoError(t, err)
	assert.Equal(t, "my-agent", agent.Name)
}

func TestAuthorizeAgent(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		assert.Contains(t, r.URL.Path, "/app/rest/agents/id:1/authorized")
		w.WriteHeader(http.StatusNoContent)
	})

	assert.NoError(t, client.AuthorizeAgent(1, true))
}

func TestEnableAgent(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		assert.Contains(t, r.URL.Path, "/app/rest/agents/id:1/enabled")
		w.WriteHeader(http.StatusNoContent)
	})

	assert.NoError(t, client.EnableAgent(1, true))
}

func TestGetAgentCompatibleBuildTypes(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/app/rest/agents/id:1/compatibleBuildTypes")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(BuildTypeList{Count: 2, BuildTypes: []BuildType{{ID: "bt1"}, {ID: "bt2"}}})
	})

	result, err := client.GetAgentCompatibleBuildTypes(1)
	require.NoError(t, err)
	assert.Equal(t, 2, result.Count)
}

func TestGetAgentIncompatibleBuildTypes(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/app/rest/agents/id:1/incompatibleBuildTypes")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(CompatibilityList{Count: 0})
	})

	result, err := client.GetAgentIncompatibleBuildTypes(1)
	require.NoError(t, err)
	assert.Equal(t, 0, result.Count)
}

func TestGetBuildCompatibleAgents(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.RawQuery, "compatible%3A%28build%3A%28id%3A99%29%29")
		assert.Contains(t, r.URL.RawQuery, "defaultFilter%3Afalse")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(AgentList{
			Count:  1,
			Agents: []Agent{{ID: 7, Name: "compat-agent", Pool: &Pool{Name: "Linux"}}},
		})
	})
	result, err := client.GetBuildCompatibleAgents(99)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Count)
	assert.Equal(t, "compat-agent", result.Agents[0].Name)
}

func TestGetBuildIncompatibleAgents(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.RawQuery, "incompatible%3A%28build%3A%28id%3A99%29%29")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(AgentList{Count: 0})
	})
	result, err := client.GetBuildIncompatibleAgents(99)
	require.NoError(t, err)
	assert.Equal(t, 0, result.Count)
}

func TestGetAgentBuildTypeCompatibility_match(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/app/rest/agents/id:5/incompatibleBuildTypes")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(CompatibilityList{
			Count: 2,
			Compatibility: []Compatibility{
				{BuildType: &BuildType{ID: "Other_BT"}},
				{BuildType: &BuildType{ID: "Target_BT"}, UnmetRequirements: &UnmetRequirements{Description: "Missing: docker"}},
			},
		})
	})
	c, err := client.GetAgentBuildTypeCompatibility(5, "Target_BT", 100)
	require.NoError(t, err)
	require.NotNil(t, c)
	assert.Equal(t, "Target_BT", c.BuildType.ID)
	assert.Equal(t, []string{"Missing: docker"}, c.ReasonsList())
}

func TestGetAgentBuildTypeCompatibility_noMatch(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(CompatibilityList{Count: 1, Compatibility: []Compatibility{{BuildType: &BuildType{ID: "Other_BT"}}}})
	})
	c, err := client.GetAgentBuildTypeCompatibility(5, "Target_BT", 100)
	require.NoError(t, err)
	assert.Nil(t, c)
}

func TestReasonsListMergesLegacyAndUnmet(t *testing.T) {
	t.Parallel()
	c := Compatibility{
		Reasons:           &IncompatibleReasons{Reasons: []string{"old-style reason"}},
		UnmetRequirements: &UnmetRequirements{Description: "Unmet requirements:\n\tline A\n\tline B"},
	}
	reasons := c.ReasonsList()
	assert.Equal(t, []string{"old-style reason", "Unmet requirements:", "line A", "line B"}, reasons)
}

func TestRebootAgent(t *testing.T) {
	t.Parallel()
	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/remoteAccess/reboot.html")
		w.WriteHeader(http.StatusOK)
	})

	err := client.RebootAgent(context.Background(), 1, false)
	require.NoError(t, err)
}
