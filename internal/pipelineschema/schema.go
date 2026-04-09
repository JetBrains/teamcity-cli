package pipelineschema

import (
	_ "embed"
	"encoding/json"
	"fmt"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"
)

//go:embed schema.json
var Bytes []byte

// Validate checks TC pipeline YAML against the embedded schema.
// Returns empty string if valid, or an error description.
func Validate(yamlData string) string {
	return ValidateWithSchema(yamlData, Bytes)
}

// ValidateWithSchema checks TC pipeline YAML against the provided JSON schema bytes.
// Returns empty string if valid, or an error description.
func ValidateWithSchema(yamlData string, schemaData []byte) string {
	var doc any
	if err := yaml.Unmarshal([]byte(yamlData), &doc); err != nil {
		return fmt.Sprintf("invalid YAML: %s", err)
	}

	var schemaDoc any
	if err := json.Unmarshal(schemaData, &schemaDoc); err != nil {
		return fmt.Sprintf("internal error: invalid schema: %s", err)
	}

	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("schema.json", schemaDoc); err != nil {
		return fmt.Sprintf("internal error: %s", err)
	}

	schema, err := compiler.Compile("schema.json")
	if err != nil {
		return fmt.Sprintf("internal error: %s", err)
	}

	if err := schema.Validate(ConvertYAMLToJSON(doc)); err != nil {
		return err.Error()
	}
	return ""
}

// ConvertYAMLToJSON converts YAML-parsed values to JSON-compatible types.
func ConvertYAMLToJSON(v any) any {
	switch val := v.(type) {
	case map[string]any:
		result := make(map[string]any, len(val))
		for k, v := range val {
			result[k] = ConvertYAMLToJSON(v)
		}
		return result
	case map[any]any:
		result := make(map[string]any, len(val))
		for k, v := range val {
			result[fmt.Sprint(k)] = ConvertYAMLToJSON(v)
		}
		return result
	case []any:
		result := make([]any, len(val))
		for i, v := range val {
			result[i] = ConvertYAMLToJSON(v)
		}
		return result
	default:
		return v
	}
}
