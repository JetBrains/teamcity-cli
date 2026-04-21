// Package atomicfile writes files atomically at 0600 so concurrent writers
// can't interleave and token-bearing data is never exposed at a looser mode.
package atomicfile

import (
	"os"
	"path/filepath"
)

// Write stages data to a uniquely-named sibling temp file at 0600, then
// renames it into place. Rename is atomic on POSIX, and the unique temp name
// lets concurrent writers each stage their own file without clobbering each
// other. os.CreateTemp creates the temp file at 0600, so token-bearing
// callers never expose data at a wider mode.
func Write(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpPath) }

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		cleanup()
		return err
	}
	return nil
}
