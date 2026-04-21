package run

import (
	"bytes"
	"fmt"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockCompatClient struct {
	api.ClientInterface
	delay time.Duration
	calls atomic.Int32
}

func (m *mockCompatClient) GetAgentBuildTypeCompatibility(agentID int, _ string, _ int) (*api.Compatibility, error) {
	m.calls.Add(1)
	time.Sleep(m.delay)
	return &api.Compatibility{
		Reasons: &api.IncompatibleReasons{Reasons: []string{fmt.Sprintf("missing requirement for agent %d", agentID)}},
	}, nil
}

// TestRenderIncompatibilityReasonsParallel regresses F18/S2: 5×delay sequential collapses to ~delay with fan-out, and input order is preserved.
func TestRenderIncompatibilityReasonsParallel(T *testing.T) {
	T.Parallel()
	const delay = 200 * time.Millisecond

	agents := make([]api.Agent, reasonProbeAgents)
	for i := range agents {
		agents[i] = api.Agent{ID: i + 1, Name: "agent-" + string(rune('A'+i))}
	}

	client := &mockCompatClient{delay: delay}
	var buf bytes.Buffer

	start := time.Now()
	renderIncompatibilityReasons(&buf, client, "BT_Target", agents)
	elapsed := time.Since(start)

	assert.Less(T, elapsed, 500*time.Millisecond, "fan-out should complete in ~%s, sequential would take ~%s", delay, delay*time.Duration(reasonProbeAgents))
	assert.Equal(T, int32(reasonProbeAgents), client.calls.Load())

	out := buf.String()
	assert.Contains(T, out, "Sample incompatibility reasons:")

	positions := make([]int, len(agents))
	for i, a := range agents {
		positions[i] = strings.Index(out, a.Name)
		require.GreaterOrEqual(T, positions[i], 0, "agent %s missing from output", a.Name)
	}
	assert.True(T, slices.IsSorted(positions), "agents printed out of input order: %v", positions)
}
