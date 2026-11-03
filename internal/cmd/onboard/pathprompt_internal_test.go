package onboard

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDirSuggestions(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"alpha", "alps", "Alto", "beta", ".hidden"} {
		require.NoError(t, os.Mkdir(filepath.Join(root, d), 0o755))
	}
	require.NoError(t, os.WriteFile(filepath.Join(root, "alpaca.txt"), nil, 0o644))

	sep := string(filepath.Separator)
	in := func(name string) string { return filepath.Join(root, name) }

	tests := []struct {
		name  string
		typed string
		want  []string
	}{
		{"prefix matches dirs, not files", in("al"), []string{in("alpha") + sep, in("alps") + sep}},
		{"case-sensitive", in("Al"), []string{in("Alto") + sep}},
		{"trailing separator lists visible children", root + sep, []string{in("Alto") + sep, in("alpha") + sep, in("alps") + sep, in("beta") + sep}},
		{"dot prefix shows hidden", root + sep + ".", []string{in(".hidden") + sep}},
		{"missing parent", in("nope") + sep + "x", nil},
		{"empty input", "", nil},
		{"quoted input", `"` + in("al"), nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, dirSuggestions(tc.typed))
		})
	}

	t.Run("symlink to dir is suggested", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.Mkdir(filepath.Join(dir, "target"), 0o755))
		if err := os.Symlink(filepath.Join(dir, "target"), filepath.Join(dir, "link")); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		assert.Equal(t, []string{filepath.Join(dir, "link") + sep}, dirSuggestions(filepath.Join(dir, "l")))
	})

	t.Run("relative path keeps typed prefix", func(t *testing.T) {
		t.Chdir(root)
		assert.Equal(t, []string{"alpha" + sep, "alps" + sep}, dirSuggestions("alp"))
	})

	t.Run("tilde stays unexpanded", func(t *testing.T) {
		t.Setenv("HOME", root)
		t.Setenv("USERPROFILE", root)
		assert.Equal(t, []string{"~" + sep + "beta" + sep}, dirSuggestions("~"+sep+"b"))
	})
}
