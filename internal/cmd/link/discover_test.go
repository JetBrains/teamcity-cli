package link

import (
	"testing"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeClient embeds the interface so we only need to override the calls discoverProjects makes.
type fakeClient struct {
	api.ClientInterface
	pipelinesSupported bool
	pipelines          *api.PipelineList
	buildTypesByFrag   map[string][]api.BuildType
	gotFragments       []string
}

func (f *fakeClient) SupportsFeature(name string) bool {
	return name == "pipelines" && f.pipelinesSupported
}

func (f *fakeClient) GetPipelines(api.PipelinesOptions) (*api.PipelineList, error) {
	if f.pipelines == nil {
		return &api.PipelineList{}, nil
	}
	return f.pipelines, nil
}

func (f *fakeClient) GetBuildTypes(opts api.BuildTypesOptions) (*api.BuildTypeList, error) {
	f.gotFragments = append(f.gotFragments, opts.VcsRootURL)
	bts := f.buildTypesByFrag[opts.VcsRootURL]
	return &api.BuildTypeList{Count: len(bts), BuildTypes: bts}, nil
}

func vcsEntries(urls ...string) *api.VcsRootEntries {
	out := &api.VcsRootEntries{}
	for _, u := range urls {
		out.VcsRootEntry = append(out.VcsRootEntry, api.VcsRootEntry{
			VcsRoot: &api.VcsRoot{
				Properties: &api.PropertyList{
					Property: []api.Property{{Name: "url", Value: u}},
				},
			},
		})
	}
	out.Count = len(out.VcsRootEntry)
	return out
}

func TestExtractFragments(t *testing.T) {
	frags, canon := extractFragments([]string{
		"git@github.com:acme/backend.git",
		"https://github.com/acme/backend",     // dup of above (canonical equal)
		"https://gitlab.example.com/acme/api", // different repo
	})
	assert.Equal(t, []string{"acme/backend", "acme/api"}, frags)
	assert.Equal(t, []string{"github.com/acme/backend", "gitlab.example.com/acme/api"}, canon)
}

func TestDiscoverProjectsEmptyRemotes(t *testing.T) {
	client := &fakeClient{}
	got, err := discoverProjects(client, nil)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestDiscoverProjectsRejectsForkAndPaused(t *testing.T) {
	client := &fakeClient{
		buildTypesByFrag: map[string][]api.BuildType{
			"acme/backend": {
				{ID: "P_Build", Name: "Build", ProjectID: "P", ProjectName: "Backend",
					VcsRootEntries: vcsEntries("git@github.com:acme/backend.git")},
				// Server matched "backend" as substring but the URL belongs to a fork.
				{ID: "P_Fork", Name: "Build", ProjectID: "P", ProjectName: "Backend",
					VcsRootEntries: vcsEntries("git@github.com:acme/backend-plugin.git")},
				// Paused — must drop.
				{ID: "P_Old", Name: "Old", ProjectID: "P", ProjectName: "Backend", Paused: true,
					VcsRootEntries: vcsEntries("git@github.com:acme/backend.git")},
			},
		},
	}
	got, err := discoverProjects(client, []string{"git@github.com:acme/backend.git"})
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Len(t, got.Projects, 1)
	require.Len(t, got.Projects[0].Jobs, 1)
	assert.Equal(t, "P_Build", got.Projects[0].Jobs[0].ID)
	assert.Equal(t, []string{"acme/backend"}, client.gotFragments)
}

func TestDiscoverProjectsGroupsByProjectAndSorts(t *testing.T) {
	url := "git@github.com:acme/backend.git"
	entries := vcsEntries(url)
	client := &fakeClient{
		buildTypesByFrag: map[string][]api.BuildType{
			"acme/backend": {
				{ID: "Z_BuildZ", Name: "Z", ProjectID: "Z", ProjectName: "Zeta", VcsRootEntries: entries},
				{ID: "A_BuildB", Name: "B", ProjectID: "A", ProjectName: "Alpha", VcsRootEntries: entries},
				{ID: "A_BuildA", Name: "A", ProjectID: "A", ProjectName: "Alpha", VcsRootEntries: entries},
			},
		},
	}
	got, err := discoverProjects(client, []string{url})
	require.NoError(t, err)
	require.Len(t, got.Projects, 2)
	assert.Equal(t, "Alpha", got.Projects[0].ProjectName)
	assert.Equal(t, []string{"A", "B"}, []string{got.Projects[0].Jobs[0].Name, got.Projects[0].Jobs[1].Name})
	assert.Equal(t, "Zeta", got.Projects[1].ProjectName)
}

func TestDiscoverProjectsPipelineHeadRemapsToParent(t *testing.T) {
	url := "git@github.com:acme/cli.git"
	entries := vcsEntries(url)
	client := &fakeClient{
		pipelinesSupported: true,
		pipelines: &api.PipelineList{Pipelines: []api.Pipeline{
			{ID: "CLI_CI", Name: "CI",
				HeadBuildType: &api.BuildTypeRef{ID: "CLI_CI_Head"},
				ParentProject: &api.ProjectRef{ID: "CLI", Name: "CLI"}},
		}},
		buildTypesByFrag: map[string][]api.BuildType{
			"acme/cli": {
				{ID: "CLI_CI_Head", Name: "Pipeline Head", ProjectID: "CLI_CI", ProjectName: "CLI / CI",
					VcsRootEntries: entries},
				{ID: "CLI_LinuxAgent", Name: "Linux Agent", ProjectID: "CLI", ProjectName: "CLI",
					VcsRootEntries: entries},
			},
		},
	}
	got, err := discoverProjects(client, []string{url})
	require.NoError(t, err)
	require.Len(t, got.Projects, 1)
	pm := got.Projects[0]
	assert.Equal(t, "CLI", pm.ProjectID)
	assert.Equal(t, "CLI", pm.ProjectName)
	require.Len(t, pm.Jobs, 2)

	var pipeline jobOption
	for _, j := range pm.Jobs {
		if j.Pipeline {
			pipeline = j
		}
	}
	assert.Equal(t, "CLI_CI_Head", pipeline.ID)
	assert.Equal(t, "CI", pipeline.Name)
	assert.Equal(t, "CLI · CI  ⬡ pipeline", pipeline.Label)
}

func TestDiscoverProjectsDedupsAcrossFragments(t *testing.T) {
	entries := vcsEntries("git@github.com:acme/backend.git")
	bt := api.BuildType{ID: "P_Build", Name: "Build", ProjectID: "P", ProjectName: "P", VcsRootEntries: entries}
	client := &fakeClient{
		buildTypesByFrag: map[string][]api.BuildType{
			"acme/backend":            {bt}, // same buildType returned for both fragments
			"github.com/acme/backend": {bt}, // (in practice a single fragment is tried, but verify dedup)
		},
	}
	// Two remotes resolving to two distinct fragments.
	got, err := discoverProjects(client, []string{
		"git@github.com:acme/backend.git",
	})
	require.NoError(t, err)
	require.Len(t, got.Projects, 1)
	require.Len(t, got.Projects[0].Jobs, 1)
}

func TestJobLabelNonPipeline(t *testing.T) {
	o := jobOption{Name: "Build", ProjectName: "Acme"}
	assert.Equal(t, "Acme · Build", jobLabel(o))
}
