package completion

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/JetBrains/teamcity-cli/internal/link"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLinkedJobsAndProjectsReadFromTOML(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, link.Save(filepath.Join(dir, "teamcity.toml"), &link.Config{
		Servers: []link.Server{{
			URL:     "https://tc.example.com",
			Project: "Falcon",
			Job:     "Falcon_Build",
			Jobs:    []string{"Falcon_BuildDocker", "Falcon_Build"}, // duplicate
			Paths: map[string]link.PathScope{
				"backend": {Project: "Falcon_API", Job: "Falcon_APITest"},
				"docs":    {Project: "Falcon", Jobs: []string{"Falcon_BuildDocker"}}, // dup project + job
			},
		}},
	}))

	prev, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Chdir(prev) })
	require.NoError(t, os.Chdir(dir))

	jobs, dir2 := LinkedJobs()(nil, nil, "")
	assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, dir2)
	assert.Equal(t, []string{"Falcon_APITest", "Falcon_Build", "Falcon_BuildDocker"}, jobs)

	projects, _ := LinkedProjects()(nil, nil, "")
	assert.Equal(t, []string{"Falcon", "Falcon_API", "_Root"}, projects)

	scopes, _ := LinkScopes()(nil, nil, "")
	assert.Equal(t, []string{"backend", "docs"}, scopes)
}
