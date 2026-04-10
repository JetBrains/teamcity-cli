package cmdutil

import (
	"bytes"
	"testing"

	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarkExperimental(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	f := &Factory{
		Printer: &output.Printer{
			Out:    &bytes.Buffer{},
			ErrOut: &stderr,
		},
	}

	ran := false
	cmd := &cobra.Command{
		Use:   "frobnicate",
		Short: "Do the thing",
		RunE: func(cmd *cobra.Command, args []string) error {
			ran = true
			return nil
		},
	}

	MarkExperimental(f, cmd)

	assert.Equal(t, "[experimental] Do the thing", cmd.Short)
	assert.Equal(t, "true", cmd.Annotations["experimental"])
	assert.True(t, IsExperimental(cmd))

	// Execute and verify the notice is printed
	cmd.SetArgs([]string{})
	require.NoError(t, cmd.Execute())
	assert.True(t, ran)
	assert.Contains(t, stderr.String(), "experimental")
	assert.Contains(t, stderr.String(), "frobnicate")
}

func TestMarkExperimental_Quiet(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	f := &Factory{
		Quiet: true,
		Printer: &output.Printer{
			Out:    &bytes.Buffer{},
			ErrOut: &stderr,
			Quiet:  true,
		},
	}

	cmd := &cobra.Command{
		Use:   "frobnicate",
		Short: "Do the thing",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}

	MarkExperimental(f, cmd)
	cmd.SetArgs([]string{})
	require.NoError(t, cmd.Execute())
	assert.Empty(t, stderr.String())
}

func TestIsExperimental_Parent(t *testing.T) {
	t.Parallel()

	parent := &cobra.Command{Use: "parent", Annotations: map[string]string{"experimental": "true"}}
	child := &cobra.Command{Use: "child", RunE: func(cmd *cobra.Command, args []string) error { return nil }}
	parent.AddCommand(child)

	assert.True(t, IsExperimental(child))
	assert.False(t, IsExperimental(&cobra.Command{Use: "standalone"}))
}
