package pipelineschema

import (
	"encoding/json"
	"fmt"

	"github.com/JetBrains/teamcity-cli/schemas"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"
)

var Bytes = schemas.Pipeline

// HostedAgentNames extracts the JetBrains-hosted agent enum from a schema's runs-on definition; nil when the schema doesn't constrain agent names.
func HostedAgentNames(schemaData []byte) []string {
	var s struct {
		Definitions struct {
			RunOn struct {
				AnyOf []struct {
					Enum []string `json:"enum"`
				} `json:"anyOf"`
			} `json:"runOn"`
		} `json:"definitions"`
	}
	if err := json.Unmarshal(schemaData, &s); err != nil {
		return nil
	}
	for _, opt := range s.Definitions.RunOn.AnyOf {
		if len(opt.Enum) == 1 && opt.Enum[0] == "self-hosted" {
			continue
		}
		if len(opt.Enum) > 0 {
			return opt.Enum
		}
	}
	return nil
}

// ValidateWithSchema checks TC pipeline YAML against the given JSON schema; returns "" if valid.
func ValidateWithSchema(yamlData string, schemaData []byte) string {
	var doc any
	if err := yaml.Unmarshal([]byte(yamlData), &doc); err != nil {
		return fmt.Sprintf("invalid YAML: %s", err)
	}

	schema, err := Compile(schemaData)
	if err != nil {
		return fmt.Sprintf("internal error: %s", err)
	}

	if err := schema.Validate(ConvertYAMLToJSON(doc)); err != nil {
		return err.Error()
	}
	return ""
}

// Compile parses and compiles a pipeline JSON schema document.
func Compile(schemaData []byte) (*jsonschema.Schema, error) {
	var schemaDoc any
	if err := json.Unmarshal(schemaData, &schemaDoc); err != nil {
		return nil, fmt.Errorf("invalid schema: %w", err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("schema.json", schemaDoc); err != nil {
		return nil, err
	}
	return compiler.Compile("schema.json")
}

// ConvertYAMLToJSON normalizes YAML-decoded values to JSON-compatible types.
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
