package update_test

import (
	"testing"

	"github.com/JetBrains/teamcity-cli/internal/cmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateCommandRegistered(t *testing.T) {
	root := cmd.NewRootCmd()
	child, _, err := root.Find([]string{"update"})
	require.NoError(t, err)
	assert.Equal(t, "update", child.Name())
}
