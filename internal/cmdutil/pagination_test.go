package cmdutil

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContinueTokenRoundTrip(t *testing.T) {
	token, err := EncodeContinueToken("teamcity project list", "/app/rest/projects?locator=count:2,start:2", 0)
	require.NoError(t, err)

	path, offset, err := DecodeContinueToken("teamcity project list", token)
	require.NoError(t, err)
	assert.Equal(t, "/app/rest/projects?locator=count:2,start:2", path)
	assert.Zero(t, offset)
}

func TestContinueTokenCommandMismatch(t *testing.T) {
	token, err := EncodeContinueToken("teamcity agent list", "/app/rest/agents?locator=count:2,start:2", 0)
	require.NoError(t, err)

	_, _, err = DecodeContinueToken("teamcity project list", token)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not belong")
}

func TestValidateContinueConflicts(t *testing.T) {
	cmd := &cobra.Command{Use: "list"}
	cmd.Flags().String("continue", "", "")
	cmd.Flags().String("project", "", "")
	cmd.Flags().Bool("all", false, "")
	SetContinueConflicts(cmd, "project", "all")

	require.NoError(t, cmd.Flags().Set("continue", "token"))
	require.NoError(t, cmd.Flags().Set("project", "TestProject"))

	err := ValidateContinueConflicts(cmd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--continue cannot be used with --project")
}
