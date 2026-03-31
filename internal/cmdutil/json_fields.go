package cmdutil

import (
	"fmt"
	"io"
	"strings"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/spf13/cobra"
)

// AddJSONFieldsFlag adds a --json flag that accepts optional field specification
func AddJSONFieldsFlag(cmd *cobra.Command, target *string) {
	cmd.Flags().StringVar(target, "json", "", "Output JSON with fields (use --json= to list, --json=f1,f2 for specific)")
	cmd.Flags().Lookup("json").NoOptDefVal = "default"
}

// AnnotateJSONFields attaches FieldSpec metadata to the --json flag for help --json.
func AnnotateJSONFields(cmd *cobra.Command, spec *api.FieldSpec) {
	f := cmd.Flags().Lookup("json")
	if f == nil {
		return
	}
	if f.Annotations == nil {
		f.Annotations = map[string][]string{}
	}
	f.Annotations["json_fields_available"] = []string{strings.Join(spec.Available, ",")}
	f.Annotations["json_fields_default"] = []string{strings.Join(spec.Default, ",")}
}

// AnnotateEnum attaches allowed values to a flag for help --json.
func AnnotateEnum(cmd *cobra.Command, flagName string, values []string) {
	f := cmd.Flags().Lookup(flagName)
	if f == nil {
		return
	}
	if f.Annotations == nil {
		f.Annotations = map[string][]string{}
	}
	f.Annotations["enum"] = values
}

// JSONFieldsResult represents the parsed result of --json flag
type JSONFieldsResult struct {
	Enabled bool
	Fields  []string
}

// ParseJSONFields parses the --json flag value, returns (result, showHelp, error).
func ParseJSONFields(cmd *cobra.Command, flagValue string, spec *api.FieldSpec, out ...io.Writer) (JSONFieldsResult, bool, error) {
	if !cmd.Flags().Changed("json") {
		return JSONFieldsResult{}, false, nil
	}

	if flagValue == "" || flagValue == "?" {
		w := io.Writer(cmd.OutOrStdout())
		if len(out) > 0 {
			w = out[0]
		}
		_, _ = fmt.Fprintln(w, spec.Help())
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
