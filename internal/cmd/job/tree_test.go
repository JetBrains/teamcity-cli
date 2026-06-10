package job

import (
	"testing"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeTreeClient serves a fixed snapshot-dependency graph; all other client
// methods are unused and will panic via the embedded nil interface if called.
type fakeTreeClient struct {
	api.ClientInterface
	deps map[string][]api.BuildType
}

func (f *fakeTreeClient) GetSnapshotDependencies(buildTypeID string) (*api.SnapshotDependencyList, error) {
	children := f.deps[buildTypeID]
	sd := make([]api.SnapshotDependency, 0, len(children))
	for i := range children {
		sd = append(sd, api.SnapshotDependency{SourceBuildType: &children[i]})
	}
	return &api.SnapshotDependencyList{Count: len(sd), SnapshotDependency: sd}, nil
}

func TestBuildJobTreeNodesDiamondNotCircular(t *testing.T) {
	// A depends on B and C; both depend on D (a re-convergent diamond).
	client := &fakeTreeClient{deps: map[string][]api.BuildType{
		"A": {{ID: "B", Name: "B"}, {ID: "C", Name: "C"}},
		"B": {{ID: "D", Name: "D"}},
		"C": {{ID: "D", Name: "D"}},
	}}

	nodes := buildJobTreeNodes(client, "A", 0, false, map[string]bool{"A": true})

	require.Len(t, nodes, 2)
	for _, n := range nodes {
		require.Len(t, n.Dependencies, 1, "%s should expand its dependency", n.ID)
		d := n.Dependencies[0]
		assert.Equal(t, "D", d.ID)
		assert.False(t, d.circular, "shared dependency D under %s must not be flagged circular", n.ID)
	}
}

func TestBuildJobTreeNodesDetectsRealCycle(t *testing.T) {
	// A -> B -> A is a genuine cycle.
	client := &fakeTreeClient{deps: map[string][]api.BuildType{
		"A": {{ID: "B", Name: "B"}},
		"B": {{ID: "A", Name: "A"}},
	}}

	nodes := buildJobTreeNodes(client, "A", 0, false, map[string]bool{"A": true})

	require.Len(t, nodes, 1)
	require.Len(t, nodes[0].Dependencies, 1)
	assert.Equal(t, "A", nodes[0].Dependencies[0].ID)
	assert.True(t, nodes[0].Dependencies[0].circular, "A revisited as its own ancestor must be circular")
}
