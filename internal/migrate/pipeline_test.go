package migrate

import (
	"strings"
	"testing"

	"github.com/JetBrains/teamcity-cli/internal/pipelineschema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarshalYAMLSingleJob(t *testing.T) {
	t.Parallel()
	p := &Pipeline{
		Jobs: []Job{{
			ID:     "build",
			Name:   "Build",
			RunsOn: "Ubuntu-24.04-Large",
			Steps: []Step{{
				Name:          "Run tests",
				ScriptContent: "go test ./...",
			}},
		}},
	}
	yaml := p.String()
	assert.Contains(t, yaml, "  build:\n")
	assert.Contains(t, yaml, `    name: "Build"`)
	assert.Contains(t, yaml, "    runs-on: Ubuntu-24.04-Large")
	assert.Contains(t, yaml, `        name: "Run tests"`)
	assert.Contains(t, yaml, "script-content: go test ./...")

	valErr := pipelineschema.Validate(yaml)
	assert.Empty(t, valErr, "generated YAML should be valid: %s", valErr)
}

func TestMarshalYAMLDependencies(t *testing.T) {
	t.Parallel()
	p := &Pipeline{
		Jobs: []Job{
			{ID: "build", Name: "Build", RunsOn: "Ubuntu-24.04-Large",
				Steps: []Step{{Name: "build", ScriptContent: "make"}}},
			{ID: "test", Name: "Test", RunsOn: "Ubuntu-24.04-Large",
				Dependencies: []string{"build"},
				Steps:        []Step{{Name: "test", ScriptContent: "make test"}}},
		},
	}
	yaml := p.String()
	assert.Contains(t, yaml, "    dependencies:\n      - build\n")
	assert.Empty(t, pipelineschema.Validate(yaml))
}

func TestMarshalYAMLIDCollision(t *testing.T) {
	t.Parallel()
	p := &Pipeline{
		Jobs: []Job{
			{ID: "build-test", Name: "A", RunsOn: "Ubuntu-24.04-Large",
				Steps: []Step{{Name: "s", ScriptContent: "echo a"}}},
			{ID: "build_test", Name: "B", RunsOn: "Ubuntu-24.04-Large",
				Steps: []Step{{Name: "s", ScriptContent: "echo b"}}},
		},
	}
	yaml := p.String()
	assert.Contains(t, yaml, "  build_test:\n")
	assert.Contains(t, yaml, "  build_test_2:\n")
	assert.Empty(t, pipelineschema.Validate(yaml))
}

func TestMarshalYAMLEmptyDeps(t *testing.T) {
	t.Parallel()
	p := &Pipeline{
		Jobs: []Job{{
			ID: "build", Name: "Build", RunsOn: "Ubuntu-24.04-Large",
			Dependencies: []string{},
			Steps:        []Step{{Name: "s", ScriptContent: "echo ok"}},
		}},
	}
	yaml := p.String()
	assert.NotContains(t, yaml, "dependencies:")
}

func TestMarshalYAMLParameters(t *testing.T) {
	t.Parallel()
	p := &Pipeline{
		Jobs: []Job{{
			ID: "build", Name: "Build", RunsOn: "Ubuntu-24.04-Large",
			Parameters: map[string]string{"FOO": "bar", "BAZ": "qux"},
			Steps:      []Step{{Name: "s", ScriptContent: "echo ok"}},
		}},
		Parameters: map[string]string{"GLOBAL": "val"},
	}
	yaml := p.String()
	assert.Contains(t, yaml, "      env.BAZ: \"qux\"")
	assert.Contains(t, yaml, "      env.FOO: \"bar\"")
	assert.Contains(t, yaml, "  env.GLOBAL: \"val\"")
	assert.Empty(t, pipelineschema.Validate(yaml))
}

func TestMarshalYAMLFilesPublication(t *testing.T) {
	t.Parallel()
	p := &Pipeline{
		Jobs: []Job{{
			ID: "build", Name: "Build", RunsOn: "Ubuntu-24.04-Large",
			Steps: []Step{{Name: "s", ScriptContent: "make"}},
			FilesPublication: []FilePublication{{
				Path: "dist/**", ShareWithJobs: true, PublishArtifact: true,
			}},
		}},
	}
	yaml := p.String()
	assert.Contains(t, yaml, "    files-publication:\n")
	assert.Contains(t, yaml, `      - path: "dist/**"`)
	assert.Contains(t, yaml, "        share-with-jobs: true")
}

func TestMarshalYAMLMultilineScript(t *testing.T) {
	t.Parallel()
	p := &Pipeline{
		Jobs: []Job{{
			ID: "build", Name: "Build", RunsOn: "Ubuntu-24.04-Large",
			Steps: []Step{{Name: "multi", ScriptContent: "echo hello\necho world"}},
		}},
	}
	yaml := p.String()
	assert.Contains(t, yaml, "script-content: |-\n")
	assert.Contains(t, yaml, "          echo hello\n")
	assert.Contains(t, yaml, "          echo world\n")
}

func TestMarshalYAMLComment(t *testing.T) {
	t.Parallel()
	p := &Pipeline{
		Comment: "# Converted from: ci.yml\n\n",
		Jobs: []Job{{
			ID: "build", Name: "Build", RunsOn: "Ubuntu-24.04-Large",
			Steps: []Step{{Name: "s", ScriptContent: "echo ok"}},
		}},
	}
	yaml := p.String()
	require.True(t, strings.HasPrefix(yaml, "# Converted from: ci.yml"))
}

func TestMarshalYAMLStepParameters(t *testing.T) {
	t.Parallel()
	p := &Pipeline{
		Jobs: []Job{{
			ID: "build", Name: "Build", RunsOn: "Ubuntu-24.04-Large",
			Steps: []Step{{
				Name:          "s",
				ScriptContent: "echo ok",
				Parameters:    map[string]string{"MY_VAR": "val"},
			}},
		}},
	}
	yaml := p.String()
	assert.Contains(t, yaml, "          env.MY_VAR: \"val\"")
}

func TestMarshalYAMLDepRefResolution(t *testing.T) {
	t.Parallel()
	p := &Pipeline{
		Jobs: []Job{
			{ID: "build-it", Name: "Build", RunsOn: "Ubuntu-24.04-Large",
				Steps: []Step{{Name: "s", ScriptContent: "make"}}},
			{ID: "test-it", Name: "Test", RunsOn: "Ubuntu-24.04-Large",
				Dependencies: []string{"build-it"},
				Steps:        []Step{{Name: "s", ScriptContent: "make test"}}},
		},
	}
	yaml := p.String()
	assert.Contains(t, yaml, "  build_it:\n")
	assert.Contains(t, yaml, "      - build_it\n")
}
