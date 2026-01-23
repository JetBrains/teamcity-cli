package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tiulpin/teamcity-cli/internal/api"
)

func TestParseJSONFields(T *testing.T) {
	T.Parallel()
	spec := &api.FieldSpec{Available: []string{"id", "name", "status"}, Default: []string{"id", "name"}}

	tests := []struct {
		name        string
		flagChanged bool
		flagValue   string
		wantEnabled bool
		wantHelp    bool
		wantErr     bool
	}{
		{"not set", false, "", false, false, false},
		{"default", true, "default", true, false, false},
		{"specific", true, "id,status", true, false, false},
		{"help empty", true, "", false, true, false},
		{"help ?", true, "?", false, true, false},
		{"invalid", true, "invalid", false, false, true},
	}

	for _, tc := range tests {
		T.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cmd := &cobra.Command{}
			var jsonFields string
			AddJSONFieldsFlag(cmd, &jsonFields)
			if tc.flagChanged {
				_ = cmd.Flags().Set("json", tc.flagValue)
			}

			result, showHelp, err := ParseJSONFields(cmd, tc.flagValue, spec)

			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantHelp, showHelp)
			assert.Equal(t, tc.wantEnabled, result.Enabled)
		})
	}
}

func TestAddJSONFieldsFlag(T *testing.T) {
	T.Parallel()
	cmd := &cobra.Command{}
	var jsonFields string
	AddJSONFieldsFlag(cmd, &jsonFields)

	flag := cmd.Flags().Lookup("json")
	require.NotNil(T, flag)
	assert.Equal(T, "default", flag.NoOptDefVal)
}
