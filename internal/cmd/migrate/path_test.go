package migrate

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMigrateReadPath(t *testing.T) {
	t.Parallel()
	// Regression: an absolute --file path must pass through unchanged.
	// filepath.Join(".", abs) would mangle it (strips the root / drops the drive root).
	abs := filepath.Join(t.TempDir(), "ci.yml")
	assert.Equal(t, abs, migrateReadPath(".", abs))
}
