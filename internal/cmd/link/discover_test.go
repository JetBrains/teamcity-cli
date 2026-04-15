package link

import (
	"testing"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeClient implements just enough of api.ClientInterface for discovery tests.
type fakeClient struct {
	api.ClientInterface
	byFragment map[string][]api.BuildType
}

func (f *fakeClient) GetBuildTypes(opts api.BuildTypesOptions) (*api.BuildTypeList, error) {
	bts := f.byFragment[opts.VcsRootURL]
	return &api.BuildTypeList{Count: len(bts), BuildTypes: bts}, nil
}

// SupportsFeature returns false so discoverProjects skips the pipelines-enrichment query.
func (f *fakeClient) SupportsFeature(string) bool { return false }

func bt(id, name, projectID, projectName, vcsURL string) api.BuildType {
	return api.BuildType{
		ID:          id,
		Name:        name,
		ProjectID:   projectID,
		ProjectName: projectName,
		VcsRootEntries: &api.VcsRootEntries{
			VcsRootEntry: []api.VcsRootEntry{
				{VcsRoot: &api.VcsRoot{Properties: &api.PropertyList{Property: []api.Property{
					{Name: "url", Value: vcsURL},
					{Name: "branch", Value: "refs/heads/main"},
				}}}},
			},
		},
	}
}

func TestDiscoverProjects_MapsBuildTypesToProjects(t *testing.T) {
	c := &fakeClient{byFragment: map[string][]api.BuildType{
		"acme/backend": {
			bt("Acme_Backend_Build", "Build", "Acme_Backend", "Backend", "https://github.com/acme/backend.git"),
			bt("Acme_Backend_Test", "Test", "Acme_Backend", "Backend", "https://github.com/acme/backend.git"),
		},
	}}

	got, err := discoverProjects(c, []string{"git@github.com:acme/backend.git"})
	require.NoError(t, err)
	require.Len(t, got.Projects, 1)
	assert.Equal(t, "Acme_Backend", got.Projects[0].ID)
	assert.Len(t, got.Projects[0].Jobs, 2)
	assert.Len(t, got.AllJobs, 2)
}

func TestDiscoverProjects_RejectsForkWithSimilarName(t *testing.T) {
	c := &fakeClient{byFragment: map[string][]api.BuildType{
		"acme/backend": {
			bt("Fork_Build", "Build", "Fork_Proj", "Fork", "https://github.com/acme/backend-plugin.git"),
			bt("Real_Build", "Build", "Real_Proj", "Real", "https://github.com/acme/backend.git"),
		},
	}}

	got, err := discoverProjects(c, []string{"https://github.com/acme/backend"})
	require.NoError(t, err)
	require.Len(t, got.Projects, 1)
	assert.Equal(t, "Real_Proj", got.Projects[0].ID)
}

func TestDiscoverProjects_DropsPaused(t *testing.T) {
	bp := bt("A_Paused", "Paused", "P", "P", "https://github.com/acme/backend.git")
	bp.Paused = true
	c := &fakeClient{byFragment: map[string][]api.BuildType{
		"acme/backend": {
			bp,
			bt("A_Build", "Build", "P", "P", "https://github.com/acme/backend.git"),
		},
	}}
	got, err := discoverProjects(c, []string{"git@github.com:acme/backend.git"})
	require.NoError(t, err)
	require.Len(t, got.Projects, 1)
	assert.Len(t, got.Projects[0].Jobs, 1, "paused job dropped")
	assert.Equal(t, "A_Build", got.Projects[0].Jobs[0].ID)
}

func TestDiscoverProjects_GroupsAcrossProjects(t *testing.T) {
	c := &fakeClient{byFragment: map[string][]api.BuildType{
		"acme/backend": {
			bt("P1_Build", "Build", "P1", "Project One", "https://github.com/acme/backend.git"),
			bt("P2_Build", "Build", "P2", "Project Two", "https://github.com/acme/backend.git"),
			bt("P1_Test", "Test", "P1", "Project One", "https://github.com/acme/backend.git"),
		},
	}}
	got, err := discoverProjects(c, []string{"git@github.com:acme/backend.git"})
	require.NoError(t, err)
	require.Len(t, got.Projects, 2)
	assert.Len(t, got.AllJobs, 3)
}

func TestDiscoverProjects_NoRemotes(t *testing.T) {
	c := &fakeClient{byFragment: map[string][]api.BuildType{}}
	got, err := discoverProjects(c, nil)
	require.NoError(t, err)
	assert.Empty(t, got.Projects)
}

func TestPickCwdAffinity(t *testing.T) {
	projects := []projectMatch{
		{ID: "Acme_Platform_Web", Name: "Web"},
		{ID: "Acme_Platform_API", Name: "API"},
		{ID: "Acme_Platform_Release", Name: "Release"},
	}
	assert.Equal(t, 1, pickCwdAffinity(projects, "services/api"))
	assert.Equal(t, 0, pickCwdAffinity(projects, "services/web/components"))
	assert.Equal(t, 0, pickCwdAffinity(projects, ""))
	assert.Equal(t, 0, pickCwdAffinity(projects, "nonmatching"))
}
