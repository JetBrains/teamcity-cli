package pipeline

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type validateOptions struct {
	schemaPath    string
	refreshSchema bool
}

func newPipelineValidateCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &validateOptions{}

	cmd := &cobra.Command{
		Use:   "validate [file]",
		Short: "Validate pipeline YAML against server schema",
		Args:  cobra.MaximumNArgs(1),
		Example: `  teamcity pipeline validate
  teamcity pipeline validate .teamcity.yml
  teamcity pipeline validate --schema custom-schema.json
  teamcity pipeline validate --refresh-schema`,
		RunE: func(cmd *cobra.Command, args []string) error {
			file := ".teamcity.yml"
			if len(args) > 0 {
				file = args[0]
			}
			return runPipelineValidate(f, file, opts)
		},
	}

	cmd.Flags().StringVar(&opts.schemaPath, "schema", "", "Path to a local JSON schema file")
	cmd.Flags().BoolVar(&opts.refreshSchema, "refresh-schema", false, "Force re-fetch schema from server")

	return cmd
}

func runPipelineValidate(f *cmdutil.Factory, file string, opts *validateOptions) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", file, err)
	}

	// Parse YAML into a node tree (preserves line numbers)
	var rootNode yaml.Node
	if err := yaml.Unmarshal(data, &rootNode); err != nil {
		return fmt.Errorf("invalid YAML in %s: %w", file, err)
	}

	// Parse YAML into generic structure for JSON schema validation
	var doc any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("invalid YAML in %s: %w", file, err)
	}

	schemaData, err := loadSchema(f, opts)
	if err != nil {
		return err
	}

	validationErrs, err := validateAgainstSchema(schemaData, doc)
	if err != nil {
		return fmt.Errorf("schema validation failed: %w", err)
	}

	if len(validationErrs) == 0 {
		f.Printer.Success("%s is valid", file)
		printJobNames(f, &rootNode)
		return nil
	}

	_, _ = fmt.Fprintf(f.Printer.ErrOut, "%s %s has %d error(s)\n\n",
		output.Red("✗"), file, len(validationErrs))

	for _, ve := range validationErrs {
		line := findLineNumber(&rootNode, ve.path)
		if line > 0 {
			_, _ = fmt.Fprintf(f.Printer.ErrOut, "  %s %s\n", output.Faint(fmt.Sprintf("Line %d:", line)), ve.path)
		} else {
			_, _ = fmt.Fprintf(f.Printer.ErrOut, "  %s\n", ve.path)
		}
		_, _ = fmt.Fprintf(f.Printer.ErrOut, "    %s\n\n", ve.message)
	}

	return &cmdutil.ExitError{Code: 1}
}

func loadSchema(f *cmdutil.Factory, opts *validateOptions) ([]byte, error) {
	if opts.schemaPath != "" {
		return os.ReadFile(opts.schemaPath)
	}

	client, err := f.Client()
	if err != nil {
		return nil, err
	}

	c, ok := client.(*api.Client)
	if !ok {
		return nil, fmt.Errorf("schema caching requires a real API client")
	}

	return fetchOrCacheSchema(c, opts.refreshSchema)
}

type validationError struct {
	path    string
	message string
}

func validateAgainstSchema(schemaData []byte, doc any) ([]validationError, error) {
	var schemaDoc any
	if err := json.Unmarshal(schemaData, &schemaDoc); err != nil {
		return nil, fmt.Errorf("invalid JSON schema: %w", err)
	}

	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("schema.json", schemaDoc); err != nil {
		return nil, fmt.Errorf("failed to compile schema: %w", err)
	}

	schema, err := compiler.Compile("schema.json")
	if err != nil {
		return nil, fmt.Errorf("failed to compile schema: %w", err)
	}

	converted := convertYAMLToJSON(doc)
	err = schema.Validate(converted)
	if err == nil {
		return nil, nil
	}

	valErr, ok := errors.AsType[*jsonschema.ValidationError](err)
	if !ok {
		return nil, err
	}

	return flattenValidationErrors(valErr, ""), nil
}

func flattenValidationErrors(ve *jsonschema.ValidationError, prefix string) []validationError {
	var result []validationError

	path := prefix
	if len(ve.InstanceLocation) > 0 {
		path = "/" + strings.Join(ve.InstanceLocation, "/")
	}

	if len(ve.Causes) == 0 {
		msg := ve.Error()
		if idx := strings.LastIndex(msg, ": "); idx >= 0 {
			msg = msg[idx+2:]
		}
		result = append(result, validationError{
			path:    path,
			message: msg,
		})
		return result
	}

	for _, cause := range ve.Causes {
		result = append(result, flattenValidationErrors(cause, path)...)
	}

	return result
}

// convertYAMLToJSON converts YAML-parsed values to JSON-compatible types.
// yaml.v3 uses map[string]any for mappings, which jsonschema expects,
// but integer keys and other edge cases need conversion.
func convertYAMLToJSON(v any) any {
	switch val := v.(type) {
	case map[string]any:
		result := make(map[string]any, len(val))
		for k, v := range val {
			result[k] = convertYAMLToJSON(v)
		}
		return result
	case map[any]any:
		result := make(map[string]any, len(val))
		for k, v := range val {
			result[fmt.Sprint(k)] = convertYAMLToJSON(v)
		}
		return result
	case []any:
		result := make([]any, len(val))
		for i, v := range val {
			result[i] = convertYAMLToJSON(v)
		}
		return result
	default:
		return v
	}
}

// findLineNumber walks the YAML node tree to find the line number for a JSON pointer path.
func findLineNumber(root *yaml.Node, path string) int {
	if path == "" || root == nil {
		return 0
	}

	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	node := root

	// Skip document node
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		node = node.Content[0]
	}

	for _, part := range parts {
		if part == "" {
			continue
		}

		if node.Kind == yaml.MappingNode {
			for i := 0; i+1 < len(node.Content); i += 2 {
				if node.Content[i].Value == part {
					node = node.Content[i+1]
					break
				}
			}
		} else if node.Kind == yaml.SequenceNode {
			var idx int
			if _, err := fmt.Sscanf(part, "%d", &idx); err == nil && idx < len(node.Content) {
				node = node.Content[idx]
			}
		}
	}

	if node.Line > 0 {
		return node.Line
	}
	return 0
}

// printJobNames extracts and prints job names from the YAML
func printJobNames(f *cmdutil.Factory, root *yaml.Node) {
	if root == nil {
		return
	}

	node := root
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		node = node.Content[0]
	}

	if node.Kind != yaml.MappingNode {
		return
	}

	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == "jobs" {
			jobsNode := node.Content[i+1]
			if jobsNode.Kind == yaml.MappingNode {
				var names []string
				for j := 0; j+1 < len(jobsNode.Content); j += 2 {
					names = append(names, jobsNode.Content[j].Value)
				}
				if len(names) > 0 {
					_, _ = fmt.Fprintf(f.Printer.Out, "  Jobs: %s\n", strings.Join(names, ", "))
				}
			}
			return
		}
	}
}
