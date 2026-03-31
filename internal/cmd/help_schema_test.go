package cmd

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHelpJSONSingleCommand(t *testing.T) {
	t.Parallel()

	rootCmd := NewRootCmd()
	rootCmd.SetArgs([]string{"help", "--json", "run", "list"})
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	err := rootCmd.Execute()
	require.NoError(t, err)

	var schema CommandSchema
	err = json.Unmarshal(out.Bytes(), &schema)
	require.NoError(t, err, "output should be valid JSON")

	assert.Equal(t, "teamcity run list", schema.Command)
	assert.Equal(t, "List recent runs", schema.Summary)
	assert.NotEmpty(t, schema.Examples)

	flagNames := make(map[string]FlagSchema)
	for _, f := range schema.Flags {
		flagNames[f.Name] = f
	}
	assert.Contains(t, flagNames, "job")
	assert.Contains(t, flagNames, "status")
	assert.Contains(t, flagNames, "limit")
	assert.Contains(t, flagNames, "json")

	assert.Equal(t, "j", flagNames["job"].Shorthand)
	assert.Equal(t, "n", flagNames["limit"].Shorthand)
	assert.Equal(t, []string{"success", "failure", "running", "queued", "error", "unknown"}, flagNames["status"].Enum)
	assert.Equal(t, float64(30), flagNames["limit"].Default)
	assert.Equal(t, "", flagNames["job"].Default)

	require.NotNil(t, schema.JSONFields)
	assert.NotEmpty(t, schema.JSONFields.Available)
	assert.NotEmpty(t, schema.JSONFields.Default)
	assert.Contains(t, schema.JSONFields.Available, "id")
	assert.Contains(t, schema.JSONFields.Available, "status")

	// Inherited flags should include root persistent flags
	inheritedNames := map[string]bool{}
	for _, f := range schema.InheritedFlags {
		inheritedNames[f.Name] = true
	}
	assert.True(t, inheritedNames["no-color"])
	assert.True(t, inheritedNames["quiet"])
}

func TestHelpJSONFullTree(t *testing.T) {
	t.Parallel()

	rootCmd := NewRootCmd()
	rootCmd.SetArgs([]string{"help", "--json"})
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	err := rootCmd.Execute()
	require.NoError(t, err)

	var schemas []CommandSchema
	err = json.Unmarshal(out.Bytes(), &schemas)
	require.NoError(t, err, "output should be valid JSON array")
	assert.Greater(t, len(schemas), 10, "should have many commands")

	commandNames := map[string]bool{}
	for _, s := range schemas {
		commandNames[s.Command] = true
	}
	assert.True(t, commandNames["teamcity"])
	assert.True(t, commandNames["teamcity run list"])
	assert.True(t, commandNames["teamcity job list"])
	assert.True(t, commandNames["teamcity agent list"])
	assert.True(t, commandNames["teamcity auth login"])
}

func TestHelpJSONParentCommand(t *testing.T) {
	t.Parallel()

	rootCmd := NewRootCmd()
	rootCmd.SetArgs([]string{"help", "--json", "run"})
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	err := rootCmd.Execute()
	require.NoError(t, err)

	var schema CommandSchema
	err = json.Unmarshal(out.Bytes(), &schema)
	require.NoError(t, err)

	assert.Equal(t, "teamcity run", schema.Command)
	assert.NotEmpty(t, schema.Subcommands)

	subNames := map[string]bool{}
	for _, s := range schema.Subcommands {
		subNames[s.Name] = true
	}
	assert.True(t, subNames["list"])
	assert.True(t, subNames["view"])
	assert.True(t, subNames["start"])
}

func TestHelpUnknownCommand(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{
		{"help", "--json", "nonexistent"},
		{"help", "nonexistent"},
	} {
		rootCmd := NewRootCmd()
		rootCmd.SetArgs(args)
		var out bytes.Buffer
		rootCmd.SetOut(&out)
		rootCmd.SetErr(&out)
		err := rootCmd.Execute()
		assert.Error(t, err, "args: %v", args)
		assert.Contains(t, err.Error(), "unknown command", "args: %v", args)
	}
}

func TestHelpJSONSliceDefaults(t *testing.T) {
	t.Parallel()

	rootCmd := NewRootCmd()
	rootCmd.SetArgs([]string{"help", "--json", "api"})
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	err := rootCmd.Execute()
	require.NoError(t, err)

	var schema CommandSchema
	err = json.Unmarshal(out.Bytes(), &schema)
	require.NoError(t, err)

	flagMap := map[string]FlagSchema{}
	for _, f := range schema.Flags {
		flagMap[f.Name] = f
	}
	assert.Equal(t, []any{}, flagMap["header"].Default)
	assert.Equal(t, []any{}, flagMap["field"].Default)
}

func TestHelpJSONMapFlagDefault(t *testing.T) {
	t.Parallel()

	rootCmd := NewRootCmd()
	rootCmd.SetArgs([]string{"help", "--json", "run", "start"})
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	err := rootCmd.Execute()
	require.NoError(t, err)

	var schema CommandSchema
	err = json.Unmarshal(out.Bytes(), &schema)
	require.NoError(t, err)

	flagMap := map[string]FlagSchema{}
	for _, f := range schema.Flags {
		flagMap[f.Name] = f
	}

	// stringToString flags should have typed map defaults, not string "[]"
	assert.Equal(t, map[string]any{}, flagMap["param"].Default)
	assert.Equal(t, map[string]any{}, flagMap["system"].Default)
	assert.Equal(t, map[string]any{}, flagMap["env"].Default)
}

func TestHelpJSONJobTreeEnumAnnotation(t *testing.T) {
	t.Parallel()

	rootCmd := NewRootCmd()
	rootCmd.SetArgs([]string{"help", "--json", "job", "tree"})
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	err := rootCmd.Execute()
	require.NoError(t, err)

	var schema CommandSchema
	err = json.Unmarshal(out.Bytes(), &schema)
	require.NoError(t, err)

	flagMap := map[string]FlagSchema{}
	for _, f := range schema.Flags {
		flagMap[f.Name] = f
	}
	assert.Equal(t, []string{"dependents", "dependencies"}, flagMap["only"].Enum)
}

func TestHelpWithoutJSON(t *testing.T) {
	t.Parallel()

	rootCmd := NewRootCmd()
	rootCmd.SetArgs([]string{"help", "run", "list"})
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	err := rootCmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, out.String(), "List recent runs")
	assert.NotContains(t, out.String(), `"command"`)
}

func TestHelpJSONFieldsAnnotation(t *testing.T) {
	t.Parallel()

	rootCmd := NewRootCmd()
	rootCmd.SetArgs([]string{"help", "--json"})
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	err := rootCmd.Execute()
	require.NoError(t, err)

	var schemas []CommandSchema
	err = json.Unmarshal(out.Bytes(), &schemas)
	require.NoError(t, err)

	for _, s := range schemas {
		for _, f := range s.Flags {
			// FieldSpec-style --json flags have NoOptDefault set to "default"
			if f.Name == "json" && f.NoOptDefault == "default" {
				assert.NotNilf(t, s.JSONFields,
					"%s has --json field-selection flag but no json_fields annotation — call cmdutil.AnnotateJSONFields", s.Command)
			}
		}
	}
}
