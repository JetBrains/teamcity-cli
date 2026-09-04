package run_test

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunDownloadConfinement(t *testing.T) {
	for _, scenario := range []string{"nested", "directory symlink", "file symlink", "hard link", "traversal", "incomplete", "server error"} {
		t.Run(scenario, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			ts := cmdtest.NewTestServer(t)
			outputDir, outsideDir := t.TempDir(), t.TempDir()
			outsidePath := filepath.Join(outsideDir, "target")
			require.NoError(t, os.WriteFile(outsidePath, []byte("original"), 0600))
			artifactName := "nested/target"
			var expectedError string
			switch scenario {
			case "directory symlink":
				if err := os.Symlink(outsideDir, filepath.Join(outputDir, "nested")); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
				expectedError = "downloaded 0 of 1 artifacts"
			case "file symlink":
				artifactName = "target"
				if err := os.Symlink(outsidePath, filepath.Join(outputDir, artifactName)); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
			case "hard link":
				artifactName = "target"
				require.NoError(t, os.Link(outsidePath, filepath.Join(outputDir, artifactName)))
			case "traversal":
				artifactName = "../target"
				expectedError = "downloaded 0 of 1 artifacts"
			case "incomplete", "server error":
				artifactName = "target"
				require.NoError(t, os.WriteFile(filepath.Join(outputDir, artifactName), []byte("original"), 0600))
				expectedError = "downloaded 0 of 1 artifacts"
			}
			ts.Handle("GET /app/rest/builds/id:1/artifacts/children", func(writer http.ResponseWriter, request *http.Request) {
				size := int64(len("replacement"))
				if scenario == "incomplete" {
					size++
				}
				cmdtest.JSON(writer, api.Artifacts{File: []api.Artifact{{Name: artifactName, Size: size, Content: &api.Content{}}}})
			})
			ts.Handle("GET /app/rest/builds/id:1/artifacts/content/", func(writer http.ResponseWriter, request *http.Request) {
				if scenario == "server error" {
					http.Error(writer, "not found", http.StatusNotFound)
					return
				}
				cmdtest.Text(writer, "replacement")
			})
			args := []string{"run", "download", "1", "--output", outputDir}
			if expectedError != "" {
				cmdtest.RunCmdWithFactoryExpectErr(t, ts.Factory, expectedError, args...)
			} else {
				cmdtest.RunCmdWithFactory(t, ts.Factory, args...)
				contents, err := os.ReadFile(filepath.Join(outputDir, artifactName))
				require.NoError(t, err)
				assert.Equal(t, "replacement", string(contents))
			}
			contents, err := os.ReadFile(outsidePath)
			require.NoError(t, err)
			assert.Equal(t, "original", string(contents))
			if scenario == "incomplete" || scenario == "server error" {
				contents, err := os.ReadFile(filepath.Join(outputDir, artifactName))
				require.NoError(t, err)
				assert.Equal(t, "original", string(contents))
			}
			require.NoError(t, filepath.WalkDir(outputDir, func(path string, entry os.DirEntry, err error) error {
				require.NoError(t, err)
				assert.NotContains(t, entry.Name(), ".teamcity-download-")
				return nil
			}))
		})
	}
}
