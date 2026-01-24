package cmd

import (
	"fmt"

	"github.com/JetBrains/teamcity-cli/internal/api"
	"github.com/spf13/cobra"
)

// AddJSONFieldsFlag adds a --json flag that accepts optional field specification
func AddJSONFieldsFlag(cmd *cobra.Command, target *string) {
	cmd.Flags().StringVar(target, "json", "", "Output JSON with fields (use --json= to list, --json=f1,f2 for specific)")
	cmd.Flags().Lookup("json").NoOptDefVal = "default"
}

// JSONFieldsResult represents the parsed result of --json flag
type JSONFieldsResult struct {
	Enabled bool
	Fields  []string
}

// ParseJSONFields parses the --json flag value, returns (result, showHelp, error).
func ParseJSONFields(cmd *cobra.Command, flagValue string, spec *api.FieldSpec) (JSONFieldsResult, bool, error) {
	if !cmd.Flags().Changed("json") {
		return JSONFieldsResult{}, false, nil
	}

	if flagValue == "" || flagValue == "?" {
		fmt.Println(spec.Help())
		return JSONFieldsResult{}, true, nil
	}

	var fields []string
	var err error
	if flagValue == "default" {
		fields = spec.Default
	} else {
		fields, err = spec.ParseFields(flagValue)
		if err != nil {
			return JSONFieldsResult{}, false, err
		}
	}

	return JSONFieldsResult{Enabled: true, Fields: fields}, false, nil
}
