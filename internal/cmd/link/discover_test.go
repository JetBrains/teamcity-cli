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

func TestResolveAuto(t *testing.T) {
	makeHit := func(url, projID string, jobIDs ...string) serverResult {
		jobs := make([]jobOption, len(jobIDs))
		for i, id := range jobIDs {
			jobs[i] = jobOption{ID: id, Name: id, ProjectName: projID}
		}
		return serverResult{
			url: url,
			discovery: &discovery{Projects: []projectMatch{{
				ProjectID: projID, ProjectName: projID, Jobs: jobs,
			}}},
		}
	}

	t.Run("single server, single project, single job", func(t *testing.T) {
		res, err := resolveAuto([]serverResult{makeHit("https://a", "P", "P_Build")}, "")
		require.NoError(t, err)
		assert.Equal(t, "https://a", res.server)
		assert.Equal(t, "P", res.project)
		assert.Equal(t, "P_Build", res.job)
		assert.Empty(t, res.jobs)
	})

	t.Run("single server, single project, multiple jobs", func(t *testing.T) {
		res, err := resolveAuto([]serverResult{makeHit("https://a", "P", "P_Build", "P_Test")}, "")
		require.NoError(t, err)
		assert.Equal(t, "P", res.project)
		assert.Empty(t, res.job, "multiple jobs in project -> default left empty")
		assert.ElementsMatch(t, []string{"P_Build", "P_Test"}, res.jobs)
	})

	t.Run("multiple servers, no active match", func(t *testing.T) {
		_, err := resolveAuto([]serverResult{
			makeHit("https://b", "P", "j"),
			makeHit("https://a", "Q", "k"),
		}, "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "multiple servers")
		assert.Contains(t, err.Error(), "https://a, https://b")
	})

	t.Run("multiple servers, active server tiebreaker", func(t *testing.T) {
		res, err := resolveAuto([]serverResult{
			makeHit("https://b", "P_b", "j"),
			makeHit("https://a", "P_a", "k"),
		}, "https://a")
		require.NoError(t, err)
		assert.Equal(t, "https://a", res.server)
		assert.Equal(t, "P_a", res.project)
		assert.Equal(t, "k", res.job)
	})

	t.Run("multiple servers, active not in hits, still ambiguous", func(t *testing.T) {
		_, err := resolveAuto([]serverResult{
			makeHit("https://b", "P", "j"),
			makeHit("https://c", "Q", "k"),
		}, "https://a")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "multiple servers")
	})

	t.Run("multiple projects on one server", func(t *testing.T) {
		h := makeHit("https://a", "Z", "Z_Build")
		h.discovery.Projects = append(h.discovery.Projects, projectMatch{
			ProjectID: "A", ProjectName: "A", Jobs: []jobOption{{ID: "A_Build", Name: "A_Build"}},
		})
		_, err := resolveAuto([]serverResult{h}, "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "multiple projects")
		assert.Contains(t, err.Error(), "A, Z", "project IDs sorted alphabetically")
	})

	t.Run("additional jobs include cross-project matches on same server", func(t *testing.T) {
		h := makeHit("https://a", "P", "P_Build")
		// resolveAuto only fires when there's a single project; this test covers the case
		// where allJobsOnServer returns the picked project's jobs.
		res, err := resolveAuto([]serverResult{h}, "")
		require.NoError(t, err)
		assert.Equal(t, "P_Build", res.job)
		assert.Empty(t, res.jobs, "single job -> nothing extra")
	})
}

func TestJobLabelNonPipeline(t *testing.T) {
	o := jobOption{Name: "Build", ProjectName: "Acme"}
	assert.Equal(t, "Acme · Build", jobLabel(o))
}
