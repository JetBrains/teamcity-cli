package pipeline

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestFindLineNumber(t *testing.T) {
	t.Parallel()

	src := `version: v1.0
jobs:
  build:
    steps:
      - script: go build ./...
  test:
    needs: [build]
    steps:
      - script: go test ./...
`
	var root yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(src), &root))

	// findLineNumber descends to the *value* node, so block-style YAML reports the line after the key.
	cases := []struct {
		path     string
		wantLine int
	}{
		{"", 0},                    // empty path → 0
		{"/version", 1},            // scalar value on the same line as its key
		{"/jobs", 3},               // value starts at the first sub-key ("build:")
		{"/jobs/build", 4},         // value starts at "steps:"
		{"/jobs/build/steps", 5},   // sequence starts on the next line
		{"/jobs/build/steps/0", 5}, // first sequence item
		{"/jobs/test/needs/0", 7},  // inline-array element on key's line
		{"/jobs/test/steps/0/script", 9},
		{"/jobs/missing", 0}, // missing path: walk falls through; we only assert non-negative below
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			t.Parallel()
			got := findLineNumber(&root, tc.path)
			if tc.path == "/jobs/missing" {
				assert.True(t, got >= 0)
				return
			}
			assert.Equal(t, tc.wantLine, got)
		})
	}

	t.Run("nil root", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, 0, findLineNumber(nil, "/anything"))
	})
}

func TestConvertYAMLToJSON(t *testing.T) {
	t.Parallel()

	t.Run("string keys passthrough", func(t *testing.T) {
		in := map[string]any{"k": "v"}
		got := convertYAMLToJSON(in)
		assert.Equal(t, map[string]any{"k": "v"}, got)
	})

	t.Run("integer keys stringified", func(t *testing.T) {
		// jsonschema only accepts string keys, so map[any]any keys must stringify.
		in := map[any]any{1: "one", "two": 2}
		got := convertYAMLToJSON(in)
		want := map[string]any{"1": "one", "two": 2}
		assert.Equal(t, want, got)
	})

	t.Run("nested arrays and maps", func(t *testing.T) {
		in := map[string]any{
			"jobs": map[any]any{
				"build": map[string]any{
					"steps": []any{
						map[any]any{"script": "go build"},
					},
				},
			},
		}
		got := convertYAMLToJSON(in).(map[string]any)
		jobs := got["jobs"].(map[string]any)
		build := jobs["build"].(map[string]any)
		steps := build["steps"].([]any)
		assert.Equal(t, "go build", steps[0].(map[string]any)["script"])
	})

	t.Run("scalars unchanged", func(t *testing.T) {
		assert.Equal(t, "x", convertYAMLToJSON("x"))
		assert.Equal(t, 42, convertYAMLToJSON(42))
		assert.Equal(t, true, convertYAMLToJSON(true))
		assert.Equal(t, nil, convertYAMLToJSON(nil))
	})
}

func TestValidateAgainstSchema(t *testing.T) {
	t.Parallel()

	// Tiny inline schema mirroring the real pipeline schema's top-level shape.
	schema := []byte(`{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type": "object",
		"properties": {
			"version": {"type": "string", "enum": ["v1.0"]},
			"jobs":    {"type": "object"}
		},
		"required": ["version", "jobs"]
	}`)

	t.Run("valid doc → no errors", func(t *testing.T) {
		t.Parallel()
		doc := map[string]any{"version": "v1.0", "jobs": map[string]any{}}
		errs, err := validateAgainstSchema(schema, doc)
		require.NoError(t, err)
		assert.Empty(t, errs)
	})

	t.Run("missing required → reported", func(t *testing.T) {
		t.Parallel()
		doc := map[string]any{"version": "v1.0"}
		errs, err := validateAgainstSchema(schema, doc)
		require.NoError(t, err)
		require.NotEmpty(t, errs)
		// Don't pin exact wording (jsonschema lib may rephrase); just check the field is mentioned.
		assert.Contains(t, errs[0].message, "jobs")
	})

	t.Run("wrong enum → reported with path", func(t *testing.T) {
		t.Parallel()
		doc := map[string]any{"version": "v0.9", "jobs": map[string]any{}}
		errs, err := validateAgainstSchema(schema, doc)
		require.NoError(t, err)
		require.NotEmpty(t, errs)
		// At least one error should point at the version field via its JSON pointer.
		var sawVersion bool
		for _, e := range errs {
			if e.path == "/version" {
				sawVersion = true
				break
			}
		}
		assert.True(t, sawVersion, "expected an error with path /version, got %+v", errs)
	})

	t.Run("invalid schema → returns error", func(t *testing.T) {
		t.Parallel()
		_, err := validateAgainstSchema([]byte(`{not json`), map[string]any{})
		assert.Error(t, err)
	})
}
