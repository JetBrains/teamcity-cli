package onboard

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCloneDirName(t *testing.T) {
	assert.Equal(t, "spring-petclinic", cloneDirName("https://github.com/spring-projects/spring-petclinic"))
	assert.Equal(t, "spring-petclinic", cloneDirName("https://github.com/spring-projects/spring-petclinic.git"))
	assert.Equal(t, "demo", cloneDirName("not-a-url"))
}

func TestValidateCloneParent(t *testing.T) {
	parent := t.TempDir()
	file := filepath.Join(parent, "README.md")
	require.NoError(t, os.WriteFile(file, []byte("hi"), 0o644))

	assert.NoError(t, validateCloneParent(parent))

	require.NoError(t, os.Mkdir(filepath.Join(parent, "demo"), 0o755))
	assert.NoError(t, validateCloneParent(parent), "an existing clone target is resolved by the overwrite/reuse prompt, not rejected")
	assert.ErrorContains(t, validateCloneParent(filepath.Join(parent, "missing")), "does not exist")
	assert.ErrorContains(t, validateCloneParent(file), "is not a directory")
	assert.ErrorContains(t, validateCloneParent(""), "path is required")
}
