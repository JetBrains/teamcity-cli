//go:build integration

package api_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInheritedProjectConnections(t *testing.T) {
	skipIfGuest(t)
	id := fmt.Sprintf("ConnectionInheritance%d", time.Now().UnixNano())
	parent, err := client.CreateProject(api.CreateProjectRequest{ID: id, Name: id, ParentProject: &api.ProjectRef{ID: testProject}})
	require.NoError(t, err)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), time.Minute)
		defer cancel()
		_, err := client.RawRequest(ctx, "DELETE", "/app/rest/projects/id:"+parent.ID, nil, nil)
		require.NoError(t, err)
	})
	child, err := client.CreateProject(api.CreateProjectRequest{ID: id + "Child", Name: "Child", ParentProject: &api.ProjectRef{ID: parent.ID}})
	require.NoError(t, err)
	feature, err := client.CreateProjectFeature(parent.ID, api.ProjectFeature{Type: "OAuthProvider", Properties: &api.PropertyList{Property: []api.Property{
		{Name: "providerType", Value: "GitHub"}, {Name: "displayName", Value: "Inheritance test"}, {Name: "clientId", Value: "test-client"}, {Name: "secure:clientSecret", Value: "test-secret"},
	}}})
	require.NoError(t, err)
	connections, err := client.GetProjectConnections(child.ID)
	require.NoError(t, err)
	found := false
	for _, connection := range connections.ProjectFeature {
		if connection.ID == feature.ID {
			found = true
		}
	}
	assert.True(t, found, "child should expose parent's connection")
}
