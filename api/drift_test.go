//go:build integration

// Drift detection tests validate that Go types match TeamCity's swagger spec.
// Runs as part of integration tests using testcontainers.
// Run with: go test -tags=integration ./api -v -run TestAPIDrift
package api_test

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"net/http"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"
)

// SwaggerSpec represents relevant parts of OpenAPI/Swagger spec
type SwaggerSpec struct {
	Definitions map[string]SwaggerDefinition `json:"definitions"`
}

// SwaggerDefinition represents a schema definition
type SwaggerDefinition struct {
	Type       string                     `json:"type"`
	Properties map[string]SwaggerProperty `json:"properties"`
	AllOf      []SwaggerDefinition        `json:"allOf,omitempty"`
	Ref        string                     `json:"$ref,omitempty"`
}

// SwaggerProperty represents a property in a definition
type SwaggerProperty struct {
	Type   string           `json:"type"`
	Ref    string           `json:"$ref,omitempty"`
	Items  *SwaggerProperty `json:"items,omitempty"`
	Format string           `json:"format,omitempty"`
}

// GoStruct represents a parsed Go struct
type GoStruct struct {
	Name   string
	Fields []GoField
}

// GoField represents a field in a Go struct
type GoField struct {
	Name     string
	JSONName string
	Type     string
}

// Mapping of Go struct names to TeamCity swagger definition names
var typeMapping = map[string]string{
	"User":                "user",
	"Project":             "project",
	"ProjectList":         "projects",
	"BuildType":           "buildType",
	"BuildTypeList":       "buildTypes",
	"Build":               "build",
	"BuildList":           "builds",
	"Triggered":           "TriggeredBy",
	"Agent":               "agent",
	"AgentList":           "agents",
	"Pool":                "agentPool",
	"QueuedBuild":         "build", // Same as Build in API
	"BuildQueue":          "builds",
	"TriggerBuildRequest": "build",
	"TagList":             "tags",
	"Tag":                 "tag",
	"ApprovalInfo":        "approvalInfo",
	"PropertyList":        "properties",
	"Property":            "property",
	"Server":              "server",
	"Change":              "change",
	"ChangeList":          "changes",
	"Files":               "files",
	"FileChange":          "FileChange",
	"TestOccurrence":      "testOccurrence",
	"TestOccurrences":     "testOccurrences",
	"TriggeringOptions":   "buildTriggeringOptions",
	"BuildComment":        "comment",
}

// Types intentionally not validated against swagger:
// - AgentRef: simplified reference type we created for requests
// - BuildTypeRef: simplified reference type we created for requests
// - APIError, APIErrorResponse: error parsing types, not in swagger
func TestAPIDrift(t *testing.T) {
	serverURL := os.Getenv("TEAMCITY_URL")
	if serverURL == "" {
		t.Skip("TEAMCITY_URL not set - skipping drift detection")
	}

	swagger, err := fetchSwaggerSpec(serverURL)
	if err != nil {
		t.Fatalf("Failed to fetch swagger: %v", err)
	}
	t.Logf("Fetched swagger from %s (%d definitions)", serverURL, len(swagger.Definitions))

	goTypes, err := parseGoStructs("types.go")
	if err != nil {
		t.Fatalf("Failed to parse types.go: %v", err)
	}
	t.Logf("Parsed %d Go structs from types.go", len(goTypes))

	for _, gs := range goTypes {
		swaggerName, ok := typeMapping[gs.Name]
		if !ok {
			continue
		}

		t.Run(gs.Name, func(t *testing.T) {
			swaggerDef, ok := swagger.Definitions[swaggerName]
			if !ok {
				t.Errorf("swagger definition '%s' not found", swaggerName)
				return
			}

			props := getAllSwaggerProperties(swagger, swaggerDef)

			for _, field := range gs.Fields {
				if !swaggerPropertyExists(props, field.JSONName) {
					t.Errorf("field '%s' (json: %s) not in swagger definition", field.Name, field.JSONName)
				}
			}
		})
	}
}

func fetchSwaggerSpec(baseURL string) (*SwaggerSpec, error) {
	url := strings.TrimSuffix(baseURL, "/") + "/app/rest/swagger.json"

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var spec SwaggerSpec
	if err := json.NewDecoder(resp.Body).Decode(&spec); err != nil {
		return nil, fmt.Errorf("JSON decode failed: %w", err)
	}

	return &spec, nil
}

func parseGoStructs(filename string) ([]GoStruct, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	var structs []GoStruct

	ast.Inspect(node, func(n ast.Node) bool {
		typeSpec, ok := n.(*ast.TypeSpec)
		if !ok {
			return true
		}

		structType, ok := typeSpec.Type.(*ast.StructType)
		if !ok {
			return true
		}

		gs := GoStruct{Name: typeSpec.Name.Name}

		for _, field := range structType.Fields.List {
			if len(field.Names) == 0 {
				continue // Skip embedded fields
			}

			gf := GoField{
				Name: field.Names[0].Name,
				Type: fieldTypeToString(field.Type),
			}

			// Parse json tag
			if field.Tag != nil {
				tag := reflect.StructTag(strings.Trim(field.Tag.Value, "`"))
				jsonTag := tag.Get("json")
				if jsonTag != "" {
					parts := strings.Split(jsonTag, ",")
					if parts[0] != "" && parts[0] != "-" {
						gf.JSONName = parts[0]
					}
				}
			}

			if gf.JSONName != "" { // Only add fields with json tags
				gs.Fields = append(gs.Fields, gf)
			}
		}

		if len(gs.Fields) > 0 {
			structs = append(structs, gs)
		}

		return true
	})

	return structs, nil
}

func fieldTypeToString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + fieldTypeToString(t.X)
	case *ast.ArrayType:
		return "[]" + fieldTypeToString(t.Elt)
	case *ast.SelectorExpr:
		return fieldTypeToString(t.X) + "." + t.Sel.Name
	case *ast.MapType:
		return "map[" + fieldTypeToString(t.Key) + "]" + fieldTypeToString(t.Value)
	default:
		return fmt.Sprintf("%T", expr)
	}
}

func getAllSwaggerProperties(swagger *SwaggerSpec, def SwaggerDefinition) map[string]SwaggerProperty {
	props := make(map[string]SwaggerProperty)

	for k, v := range def.Properties {
		props[k] = v
	}

	for _, ao := range def.AllOf {
		if ao.Ref != "" {
			refName := extractSwaggerRefName(ao.Ref)
			if refDef, ok := swagger.Definitions[refName]; ok {
				for k, v := range getAllSwaggerProperties(swagger, refDef) {
					props[k] = v
				}
			}
		}
		for k, v := range ao.Properties {
			props[k] = v
		}
	}

	return props
}

func extractSwaggerRefName(ref string) string {
	// "$ref": "#/definitions/foo" -> "foo"
	re := regexp.MustCompile(`#/definitions/(.+)`)
	matches := re.FindStringSubmatch(ref)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

func swaggerPropertyExists(props map[string]SwaggerProperty, jsonName string) bool {
	if _, ok := props[jsonName]; ok {
		return true
	}

	lowerName := strings.ToLower(jsonName)
	for k := range props {
		if strings.ToLower(k) == lowerName {
			return true
		}
	}

	return false
}
