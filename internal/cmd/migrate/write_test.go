package migrate

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigrateRefusesToOverwriteEditedOutput(t *testing.T) {
	t.Setenv("DO_NOT_TRACK", "1")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	src := filepath.Join(t.TempDir(), "ci.yml")
	require.NoError(t, os.WriteFile(src, []byte("name: ci\non: push\njobs:\n  b:\n    runs-on: ubuntu-latest\n    steps:\n      - run: echo hi\n"), 0o644))

	outDir := t.TempDir()
	var buf bytes.Buffer
	f := cmdutil.NewFactory()
	f.Printer = &output.Printer{Out: &buf, ErrOut: &buf}
	f.ClientFunc = func() (api.ClientInterface, error) { return nil, errors.New("no auth") }

	opts := &migrateOptions{file: src, outputDir: outDir, from: "github-actions"}
	require.NoError(t, runMigrate(f, opts))
	gen := filepath.Join(outDir, "ci.tc.yml")
	generated, err := os.ReadFile(gen)
	require.NoError(t, err)

	// Identical content: rerun stays idempotent and clean.
	require.NoError(t, runMigrate(f, opts))

	// Edited output survives a rerun and the command exits non-zero.
	require.NoError(t, os.WriteFile(gen, []byte("# edited\n"), 0o644))
	var exitErr *cmdutil.ExitError
	require.ErrorAs(t, runMigrate(f, opts), &exitErr)
	kept, err := os.ReadFile(gen)
	require.NoError(t, err)
	assert.Equal(t, "# edited\n", string(kept))
	assert.Contains(t, buf.String(), "--force")

	// --force overwrites the edit.
	opts.force = true
	require.NoError(t, runMigrate(f, opts))
	forced, err := os.ReadFile(gen)
	require.NoError(t, err)
	assert.Equal(t, string(generated), string(forced))
}
