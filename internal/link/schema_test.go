package link

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// repoRoot walks up from this test file to locate schemas/teamcity.toml.json so the test runs from any cwd.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok, "runtime.Caller failed")
	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "schemas", "teamcity.toml.json")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not locate schemas/teamcity.toml.json above %s", filepath.Dir(file))
		}
		dir = parent
	}
}

func loadSchemaDoc(t *testing.T) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "schemas", "teamcity.toml.json"))
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(data, &doc))
	return doc
}

// schemaWalker checks that a Go type and a schema node describe the same shape:
// names, types, required flags, and nested item/value schemas. Resolves local $refs.
type schemaWalker struct {
	t    *testing.T
	root map[string]any
}

func (w *schemaWalker) resolveRef(node map[string]any) map[string]any {
	seen := map[string]bool{}
	for {
		ref, ok := node["$ref"].(string)
		if !ok {
			return node
		}
		require.Falsef(w.t, seen[ref], "$ref cycle through %s", ref)
		seen[ref] = true
		require.Truef(w.t, strings.HasPrefix(ref, "#/"), "non-local $ref: %s", ref)
		cur := w.root
		for p := range strings.SplitSeq(strings.TrimPrefix(ref, "#/"), "/") {
			next, ok := cur[p].(map[string]any)
			require.Truef(w.t, ok, "$ref %s does not resolve at %q", ref, p)
			cur = next
		}
		node = cur
	}
}

func (w *schemaWalker) check(label string, goType reflect.Type, node map[string]any) {
	node = w.resolveRef(node)
	switch goType.Kind() {
	case reflect.String:
		assert.Equalf(w.t, "string", node["type"], "%s: expected schema type string", label)
	case reflect.Slice:
		assert.Equalf(w.t, "array", node["type"], "%s: expected schema type array", label)
		items, ok := node["items"].(map[string]any)
		require.Truef(w.t, ok, "%s: array schema missing items", label)
		w.check(label+"[]", goType.Elem(), items)
	case reflect.Map:
		require.Equalf(w.t, reflect.String, goType.Key().Kind(), "%s: only string-keyed maps supported", label)
		assert.Equalf(w.t, "object", node["type"], "%s: expected schema type object", label)
		ap, ok := node["additionalProperties"].(map[string]any)
		require.Truef(w.t, ok, "%s: map schema needs additionalProperties as a schema object", label)
		w.check(label+"{}", goType.Elem(), ap)
	case reflect.Struct:
		assert.Equalf(w.t, "object", node["type"], "%s: expected schema type object", label)
		assert.Equalf(w.t, false, node["additionalProperties"], "%s: struct schema must set additionalProperties:false", label)
		props, _ := node["properties"].(map[string]any)

		wantFields := map[string]reflect.Type{}
		wantRequired := map[string]bool{}
		for f := range goType.Fields() {
			tag := f.Tag.Get("toml")
			if tag == "" || tag == "-" {
				continue
			}
			name, opts, _ := strings.Cut(tag, ",")
			if name == "" {
				continue
			}
			wantFields[name] = f.Type
			if !strings.Contains(opts, "omitempty") {
				wantRequired[name] = true
			}
		}

		gotNames := map[string]struct{}{}
		for k := range props {
			gotNames[k] = struct{}{}
		}
		wantNames := map[string]struct{}{}
		for k := range wantFields {
			wantNames[k] = struct{}{}
		}
		assert.Equalf(w.t, wantNames, gotNames, "%s: schema properties and struct toml fields differ — update schemas/teamcity.toml.json or the struct so they stay in sync", label)

		gotRequired := map[string]bool{}
		if rl, _ := node["required"].([]any); rl != nil {
			for _, r := range rl {
				if s, ok := r.(string); ok {
					gotRequired[s] = true
				}
			}
		}
		assert.Equalf(w.t, wantRequired, gotRequired, "%s: schema 'required' and non-omitempty struct fields differ", label)

		for name, ft := range wantFields {
			sub, ok := props[name].(map[string]any)
			if !ok {
				continue
			}
			w.check(label+"."+name, ft, sub)
		}
	default:
		w.t.Fatalf("%s: unsupported Go kind %s — extend schemaWalker", label, goType.Kind())
	}
}

// TestSchemaMatchesGoStructs walks Config recursively against the schema, catching name, type, and required drift in either direction.
func TestSchemaMatchesGoStructs(t *testing.T) {
	doc := loadSchemaDoc(t)
	w := &schemaWalker{t: t, root: doc}
	w.check("Config", reflect.TypeFor[Config](), doc)
}

// TestRepoTomlValidatesAgainstSchema validates the repo's own teamcity.toml against the schema.
func TestRepoTomlValidatesAgainstSchema(t *testing.T) {
	root := repoRoot(t)
	tomlBytes, err := os.ReadFile(filepath.Join(root, FileName))
	require.NoError(t, err)

	var asAny any
	require.NoError(t, toml.Unmarshal(tomlBytes, &asAny))

	schemaBytes, err := os.ReadFile(filepath.Join(root, "schemas", "teamcity.toml.json"))
	require.NoError(t, err)
	schema, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemaBytes))
	require.NoError(t, err)

	c := jsonschema.NewCompiler()
	require.NoError(t, c.AddResource("teamcity.toml.json", schema))
	compiled, err := c.Compile("teamcity.toml.json")
	require.NoError(t, err)

	require.NoError(t, compiled.Validate(asAny), "schemas/teamcity.toml.json rejects the repo's own teamcity.toml — fix the schema or the file")
}
