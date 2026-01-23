package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/tiulpin/teamcity-cli/internal/api"
)

func TestParseJSONFields(t *testing.T) {
	spec := &api.FieldSpec{Available: []string{"id", "name", "status"}, Default: []string{"id", "name"}}

	tests := []struct {
		name        string
		flagChanged bool
		flagValue   string
		wantEnabled bool
		wantFields  []string
		wantHelp    bool
		wantErr     bool
	}{
		{"not set", false, "", false, nil, false, false},
		{"default", true, "default", true, []string{"id", "name"}, false, false},
		{"specific", true, "id,status", true, []string{"id", "status"}, false, false},
		{"help empty", true, "", false, nil, true, false},
		{"help ?", true, "?", false, nil, true, false},
		{"invalid", true, "invalid", false, nil, false, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := &cobra.Command{}
			var jsonFields string
			AddJSONFieldsFlag(cmd, &jsonFields)
			if tc.flagChanged {
				_ = cmd.Flags().Set("json", tc.flagValue)
			}

			result, showHelp, err := ParseJSONFields(cmd, tc.flagValue, spec)

			if tc.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if showHelp != tc.wantHelp {
				t.Errorf("showHelp = %v, want %v", showHelp, tc.wantHelp)
			}
			if result.Enabled != tc.wantEnabled {
				t.Errorf("Enabled = %v, want %v", result.Enabled, tc.wantEnabled)
			}
		})
	}
}

func TestAddJSONFieldsFlag(t *testing.T) {
	cmd := &cobra.Command{}
	var jsonFields string
	AddJSONFieldsFlag(cmd, &jsonFields)

	if flag := cmd.Flags().Lookup("json"); flag == nil || flag.NoOptDefVal != "default" {
		t.Error("flag not configured correctly")
	}
}
