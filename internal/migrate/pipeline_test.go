package migrate

import (
	"strings"
	"testing"

	"github.com/JetBrains/teamcity-cli/internal/pipelineschema"
	"github.com/stretchr/testify/assert"
)

func TestMarshalYAMLSingleJob(t *testing.T) {
	t.Parallel()
	p := &Pipeline{
		Comment: "# Converted from: ci.yml\n\n",
		Jobs: []Job{{
			ID:           "build",
			Name:         "Build",
			RunsOn:       "Linux-Large",
			Dependencies: []string{},
			Steps: []Step{{
				Name:          "Run tests",
				ScriptContent: "go test ./...",
			}},
		}},
	}
	yaml := p.String()
	assert.True(t, strings.HasPrefix(yaml, "# Converted from: ci.yml"))
	assert.Contains(t, yaml, "  build:\n")
	assert.Contains(t, yaml, `    name: "Build"`)
	assert.Contains(t, yaml, "    runs-on: Linux-Large")
	assert.Contains(t, yaml, `        name: "Run tests"`)
	assert.Contains(t, yaml, "script-content: go test ./...")
	assert.NotContains(t, yaml, "dependencies:", "empty dependency list must not be emitted")

	valErr := pipelineschema.Validate(yaml)
	assert.Empty(t, valErr, "generated YAML should be valid: %s", valErr)
}

func TestMarshalYAMLIDCollision(t *testing.T) {
	t.Parallel()
	p := &Pipeline{
		Jobs: []Job{
			{ID: "build-test", Name: "A", RunsOn: "Linux-Large",
				Steps: []Step{{Name: "s", ScriptContent: "echo a"}}},
			{ID: "build_test", Name: "B", RunsOn: "Linux-Large",
				Steps: []Step{{Name: "s", ScriptContent: "echo b"}}},
		},
	}
	yaml := p.String()
	assert.Contains(t, yaml, "  build_test:\n")
	assert.Contains(t, yaml, "  build_test_2:\n")
	assert.Empty(t, pipelineschema.Validate(yaml))
}

func TestMarshalYAMLParameters(t *testing.T) {
	t.Parallel()
	p := &Pipeline{
		Jobs: []Job{{
			ID: "build", Name: "Build", RunsOn: "Linux-Large",
			Parameters: map[string]string{"FOO": "bar", "BAZ": "qux"},
			Steps: []Step{{
				Name:          "s",
				ScriptContent: "echo ok",
				Parameters:    map[string]string{"MY_VAR": "val"},
			}},
		}},
		Parameters: map[string]string{"GLOBAL": "val"},
	}
	yaml := p.String()
	assert.Contains(t, yaml, "      env.BAZ: \"qux\"")
	assert.Contains(t, yaml, "      env.FOO: \"bar\"")
	assert.Contains(t, yaml, "  env.GLOBAL: \"val\"")
	assert.Contains(t, yaml, "          env.MY_VAR: \"val\"")
	assert.Empty(t, pipelineschema.Validate(yaml))
}

func TestMarshalYAMLFilesPublication(t *testing.T) {
	t.Parallel()
	p := &Pipeline{
		Jobs: []Job{{
			ID: "build", Name: "Build", RunsOn: "Linux-Large",
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
			ID: "build", Name: "Build", RunsOn: "Linux-Large",
			Steps: []Step{{Name: "multi", ScriptContent: "echo hello\necho world"}},
		}},
	}
	yaml := p.String()
	assert.Contains(t, yaml, "script-content: |-\n")
	assert.Contains(t, yaml, "          echo hello\n")
	assert.Contains(t, yaml, "          echo world\n")
}

func TestMarshalYAMLDepRefResolution(t *testing.T) {
	t.Parallel()
	p := &Pipeline{
		Jobs: []Job{
			{ID: "build-it", Name: "Build", RunsOn: "Linux-Large",
				Steps: []Step{{Name: "s", ScriptContent: "make"}}},
			{ID: "test-it", Name: "Test", RunsOn: "Linux-Large",
				Dependencies: []string{"build-it"},
				Steps:        []Step{{Name: "s", ScriptContent: "make test"}}},
		},
	}
	yaml := p.String()
	assert.Contains(t, yaml, "  build_it:\n")
	assert.Contains(t, yaml, "      - build_it\n")
	assert.Empty(t, pipelineschema.Validate(yaml))
}
