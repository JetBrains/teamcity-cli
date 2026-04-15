package link

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRoundTripWithPaths(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	in := &Config{Servers: []Server{
		{URL: "https://a.example", Project: "A", Job: "A_Build", Paths: map[string]PathScope{
			"services/api": {Project: "A_API", Job: "A_API_Build"},
			"services/web": {Project: "A_Web", Jobs: []string{"A_Web_Build", "A_Web_Deploy"}},
		}},
		{URL: "https://b.example", Project: "B"},
	}}
	require.NoError(t, Save(path, in))

	got, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, in, got)
}

func TestServerResolveDeepestPath(t *testing.T) {
	s := &Server{
		Project: "Mono",
		Job:     "Mono_Build",
		Paths: map[string]PathScope{
			"services/api":    {Project: "API"},
			"services/api/v2": {Project: "APIv2", Job: "APIv2_Build"},
			"web":             {Jobs: []string{"Web_Build", "Web_Deploy"}},
		},
	}

	assert.Equal(t, PathScope{Project: "Mono", Job: "Mono_Build"}, s.Resolve("docs"))
	assert.Equal(t, PathScope{Project: "API", Job: "Mono_Build"}, s.Resolve("services/api/handlers"))
	assert.Equal(t, PathScope{Project: "APIv2", Job: "APIv2_Build"}, s.Resolve("services/api/v2/internal"))
	assert.Equal(t, PathScope{Project: "Mono", Job: "Mono_Build", Jobs: []string{"Web_Build", "Web_Deploy"}}, s.Resolve("web/components"))
}

func TestMatchNormalizesURL(t *testing.T) {
	c := &Config{Servers: []Server{
		{URL: "https://A.Example.com/", Project: "A"},
		{URL: "https://b.example", Project: "B"},
	}}
	assert.Equal(t, "A", c.Match("https://a.example.com").Project)
	assert.Equal(t, "B", c.Match("https://b.example/").Project)
	assert.Nil(t, c.Match("https://other.example"))
	assert.Nil(t, c.Match(""))
}

// TestUpsertScopeMerging covers all merge rules in one place: unspecified fields
// are preserved on partial updates, but a project change drops job bindings tied
// to the old project — including those of inheriting child paths.
func TestUpsertScopeMerging(t *testing.T) {
	t.Run("preserves unspecified fields", func(t *testing.T) {
		c := &Config{}
		c.UpsertScope("https://a.example", "", PathScope{Project: "Acme", Job: "Acme_Build"})
		c.UpsertScope("https://a.example", "", PathScope{Job: "Acme_Build_v2"})
		assert.Equal(t, "Acme", c.Servers[0].Project)
		assert.Equal(t, "Acme_Build_v2", c.Servers[0].Job)

		c.UpsertScope("https://a.example", "api", PathScope{Project: "API", Job: "API_Build"})
		c.UpsertScope("https://a.example", "api", PathScope{Jobs: []string{"API_A", "API_B"}})
		api := c.Servers[0].Paths["api"]
		assert.Equal(t, "API", api.Project)
		assert.Equal(t, "API_Build", api.Job)
		assert.Equal(t, []string{"API_A", "API_B"}, api.Jobs)
	})

	t.Run("project change clears stale jobs in same scope", func(t *testing.T) {
		c := &Config{}
		c.UpsertScope("https://a.example", "", PathScope{Project: "Old", Job: "Old_Build", Jobs: []string{"Old_A"}})
		c.UpsertScope("https://a.example", "", PathScope{Project: "New"})
		assert.Equal(t, "New", c.Servers[0].Project)
		assert.Empty(t, c.Servers[0].Job)
		assert.Empty(t, c.Servers[0].Jobs)

		c.UpsertScope("https://a.example", "svc", PathScope{Project: "P1", Job: "P1_Build"})
		c.UpsertScope("https://a.example", "svc", PathScope{Project: "P2"})
		assert.Empty(t, c.Servers[0].Paths["svc"].Job)
	})

	t.Run("server project change clears inheriting children but not explicit ones", func(t *testing.T) {
		c := &Config{}
		c.UpsertScope("https://a.example", "", PathScope{Project: "A"})
		c.UpsertScope("https://a.example", "svc", PathScope{Job: "A_Svc_Build"})                   // inherits A
		c.UpsertScope("https://a.example", "api", PathScope{Project: "A_API", Job: "A_API_Build"}) // explicit

		c.UpsertScope("https://a.example", "", PathScope{Project: "B"})

		assert.Empty(t, c.Servers[0].Paths["svc"].Job, "inheriting child's job belonged to old project")
		assert.Equal(t, "A_API_Build", c.Servers[0].Paths["api"].Job, "explicit-project child untouched")
	})
}

func TestUpsertScopePreservesSiblings(t *testing.T) {
	c := &Config{}
	c.UpsertScope("https://a.example", "", PathScope{Project: "Mono", Job: "Mono_Build"})
	c.UpsertScope("https://a.example", "services/api", PathScope{Project: "API"})
	c.UpsertScope("https://a.example", "services/web", PathScope{Project: "Web"})
	c.UpsertScope("https://b.example", "", PathScope{Project: "B"})

	require.Len(t, c.Servers, 2)
	a := c.Servers[0]
	assert.Equal(t, "Mono", a.Project, "top-level scope preserved when upserting paths")
	assert.Equal(t, "API", a.Paths["services/api"].Project)
	assert.Equal(t, "Web", a.Paths["services/web"].Project)
	assert.Equal(t, "B", c.Servers[1].Project)
}

func TestFindWalksUp(t *testing.T) {
	root := t.TempDir()
	deep := filepath.Join(root, "a", "b", "c")
	require.NoError(t, os.MkdirAll(deep, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, FileName), []byte(""), 0o644))

	got, ok := Find(deep)
	require.True(t, ok)
	want, _ := filepath.EvalSymlinks(filepath.Join(root, FileName))
	gotResolved, _ := filepath.EvalSymlinks(got)
	assert.Equal(t, want, gotResolved)
}

func TestFindMissing(t *testing.T) {
	_, ok := Find(t.TempDir())
	assert.False(t, ok)
}

func TestFindStopsAtGitRoot(t *testing.T) {
	outer := t.TempDir()
	repo := filepath.Join(outer, "repo")
	deep := filepath.Join(repo, "a", "b")
	require.NoError(t, os.MkdirAll(deep, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(repo, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(outer, FileName), []byte(""), 0o644))

	_, ok := Find(deep)
	assert.False(t, ok, "Find must not walk past the git root")
}
