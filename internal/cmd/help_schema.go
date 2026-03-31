package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// CommandSchema describes a command for machine consumption via help --json.
type CommandSchema struct {
	Command     string              `json:"command"`
	Summary     string              `json:"summary"`
	Description string              `json:"description,omitempty"`
	Usage       string              `json:"usage"`
	Aliases     []string            `json:"aliases,omitempty"`
	Examples    []string            `json:"examples,omitempty"`
	Flags          []FlagSchema      `json:"flags,omitempty"`
	InheritedFlags []FlagSchema      `json:"inherited_flags,omitempty"`
	JSONFields     *JSONFieldsSchema `json:"json_fields,omitempty"`
	Subcommands []SubcommandSummary `json:"subcommands,omitempty"`
}

// FlagSchema describes a single CLI flag.
type FlagSchema struct {
	Name         string   `json:"name"`
	Shorthand    string   `json:"shorthand,omitempty"`
	Type         string   `json:"type"`
	Default      any      `json:"default"`
	Description  string   `json:"description"`
	Enum         []string `json:"enum,omitempty"`
	NoOptDefault string   `json:"no_opt_default,omitempty"`
}

// JSONFieldsSchema lists the available and default --json output fields.
type JSONFieldsSchema struct {
	Available []string `json:"available"`
	Default   []string `json:"default"`
}

// SubcommandSummary is a name+summary pair for a subcommand.
type SubcommandSummary struct {
	Name    string `json:"name"`
	Summary string `json:"summary"`
}

// buildCommandSchema converts a cobra.Command into a CommandSchema.
func buildCommandSchema(cmd *cobra.Command) CommandSchema {
	schema := CommandSchema{
		Command: cmd.CommandPath(),
		Summary: cmd.Short,
		Usage:   cmd.UseLine(),
	}

	if cmd.Long != "" && cmd.Long != cmd.Short {
		schema.Description = cmd.Long
	}

	if cmd.Aliases != nil {
		schema.Aliases = cmd.Aliases
	}

	schema.Examples = parseExamples(cmd.Example)
	schema.Flags = extractFlags(cmd.LocalFlags())
	schema.InheritedFlags = extractFlags(cmd.InheritedFlags())
	schema.JSONFields = extractJSONFields(cmd)

	for _, sub := range cmd.Commands() {
		if sub.IsAvailableCommand() {
			schema.Subcommands = append(schema.Subcommands, SubcommandSummary{
				Name:    sub.Name(),
				Summary: sub.Short,
			})
		}
	}

	return schema
}

// parseExamples splits a multi-line example string into trimmed lines.
func parseExamples(example string) []string {
	if example == "" {
		return nil
	}
	var examples []string
	for _, line := range strings.Split(example, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			examples = append(examples, line)
		}
	}
	return examples
}

// extractFlags converts a pflag.FlagSet into a slice of FlagSchema.
func extractFlags(fs *pflag.FlagSet) []FlagSchema {
	var flags []FlagSchema
	fs.VisitAll(func(f *pflag.Flag) {
		if f.Hidden || f.Name == "help" {
			return
		}

		schema := FlagSchema{
			Name:        f.Name,
			Shorthand:   f.Shorthand,
			Type:        f.Value.Type(),
			Default:     typedDefault(f),
			Description: f.Usage,
		}

		if f.NoOptDefVal != "" {
			schema.NoOptDefault = f.NoOptDefVal
		}

		if vals, ok := f.Annotations["enum"]; ok {
			schema.Enum = vals
		}

		flags = append(flags, schema)
	})
	return flags
}

// typedDefault returns f.DefValue converted to the appropriate Go type for JSON.
func typedDefault(f *pflag.Flag) any {
	switch f.Value.Type() {
	case "bool":
		if v, err := strconv.ParseBool(f.DefValue); err == nil {
			return v
		}
	case "int", "int32", "int64":
		if v, err := strconv.ParseInt(f.DefValue, 10, 64); err == nil {
			return v
		}
	case "float32", "float64":
		if v, err := strconv.ParseFloat(f.DefValue, 64); err == nil {
			return v
		}
	case "duration":
		if f.DefValue == "0s" || f.DefValue == "0" {
			return "0s"
		}
	case "stringArray", "stringSlice", "intSlice":
		if f.DefValue == "[]" {
			return []string{}
		}
	case "stringToString":
		if f.DefValue == "[]" {
			return map[string]string{}
		}
	}
	return f.DefValue
}

// extractJSONFields reads json_fields annotations from the --json flag, if present.
func extractJSONFields(cmd *cobra.Command) *JSONFieldsSchema {
	jsonFlag := cmd.Flags().Lookup("json")
	if jsonFlag == nil {
		return nil
	}

	vals, ok := jsonFlag.Annotations["json_fields_available"]
	if !ok || len(vals) == 0 {
		return nil
	}

	schema := &JSONFieldsSchema{
		Available: strings.Split(vals[0], ","),
	}
	if def, ok := jsonFlag.Annotations["json_fields_default"]; ok && len(def) > 0 {
		schema.Default = strings.Split(def[0], ",")
	}
	return schema
}

// collectAllCommands returns all available commands in the tree rooted at root.
func collectAllCommands(root *cobra.Command) []*cobra.Command {
	var result []*cobra.Command
	var walk func(cmd *cobra.Command)
	walk = func(cmd *cobra.Command) {
		if !cmd.IsAvailableCommand() && cmd != root {
			return
		}
		result = append(result, cmd)
		for _, sub := range cmd.Commands() {
			walk(sub)
		}
	}
	walk(root)
	return result
}

// runHelpJSON writes JSON schema for the given command (or full tree if args is empty).
func runHelpJSON(root *cobra.Command, args []string, w io.Writer) error {
	var output any

	if len(args) == 0 {
		cmds := collectAllCommands(root)
		schemas := make([]CommandSchema, 0, len(cmds))
		for _, cmd := range cmds {
			schemas = append(schemas, buildCommandSchema(cmd))
		}
		output = schemas
	} else {
		target, remaining, err := root.Find(args)
		if err != nil || len(remaining) > 0 {
			return fmt.Errorf("unknown command: %s", strings.Join(args, " "))
		}
		output = buildCommandSchema(target)
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

// newHelpCmd returns the help command with --json support.
func newHelpCmd(root *cobra.Command) *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "help [command...]",
		Short: "Help about any command",
		Long: `Help provides help for any command in the application.
Simply type teamcity help [path to command] for full details.
Use --json for machine-readable output.`,
		Example: `  teamcity help run list
  teamcity help --json
  teamcity help --json run list`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if jsonOutput {
				return runHelpJSON(root, args, cmd.OutOrStdout())
			}
			target, remaining, err := root.Find(args)
			if err != nil || len(remaining) > 0 {
				return fmt.Errorf("unknown command: %s", strings.Join(args, " "))
			}
			target.SetOut(cmd.OutOrStdout())
			target.SetErr(cmd.ErrOrStderr())
			return target.Help()
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output command schema as JSON for machine consumption")

	return cmd
}
