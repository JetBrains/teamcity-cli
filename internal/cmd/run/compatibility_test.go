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
	"github.com/JetBrains/teamcity-cli/internal/output"
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

func TestRenderCompatibilityGroupCollapsesLargePool(T *testing.T) {
	previousNoColor := output.NoColor
	output.NoColor = true
	T.Cleanup(func() { output.NoColor = previousNoColor })

	entries := make([]api.Compatibility, compatibilityInlineLimit+1)
	for i := range entries {
		entries[i] = api.Compatibility{
			Agent: &api.Agent{Name: fmt.Sprintf("agent-%d", i), Pool: &api.Pool{Name: "Linux Pool"}},
			UnmetRequirements: &api.UnmetRequirements{
				Description: "Unmet requirements:\n\tParameter 'os.name' contains 'Linux'",
			},
		}
	}

	var buf bytes.Buffer
	renderCompatibilityGroup(&buf, "Incompatible resources", entries, output.Yellow)
	out := buf.String()

	assert.Contains(T, out, "Incompatible resources (21)")
	assert.Contains(T, out, "[Linux Pool] 21 resources")
	assert.Contains(T, out, "Incompatibility reasons:")
	assert.Equal(T, 1, strings.Count(out, "Parameter 'os.name' contains 'Linux'"))
	assert.Contains(T, out, "21 resources not shown")
	assert.NotContains(T, out, "agent-0")
}
