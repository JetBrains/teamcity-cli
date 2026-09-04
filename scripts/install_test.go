package scripts_test

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstallScript(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix installer")
	}
	script, err := filepath.Abs("../install.sh")
	require.NoError(t, err)
	for _, scenario := range []string{"zsh", "bash", "unknown shell", "existing", "explicit", "relative", "symlink", "failed download"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			home := t.TempDir()
			tools := filepath.Join(home, "tools")
			require.NoError(t, os.MkdirAll(tools, 0755))
			archivePath := filepath.Join(home, "release.tar.gz")
			archive, err := os.Create(archivePath)
			require.NoError(t, err)
			compressed := gzip.NewWriter(archive)
			writer := tar.NewWriter(compressed)
			binary := "#!/bin/sh\nprintf 'teamcity version 9.9.9\\n'\n"
			require.NoError(t, writer.WriteHeader(&tar.Header{Name: "teamcity", Mode: 0755, Size: int64(len(binary))}))
			_, err = writer.Write([]byte(binary))
			require.NoError(t, err)
			require.NoError(t, writer.Close())
			require.NoError(t, compressed.Close())
			require.NoError(t, archive.Close())
			curl := "#!/bin/sh\ncat \"$INSTALL_TEST_ARCHIVE\"\n"
			if scenario == "failed download" {
				curl = "#!/bin/sh\nexit 22\n"
			}
			require.NoError(t, os.WriteFile(filepath.Join(tools, "curl"), []byte(curl), 0755))
			destination := filepath.Join(home, ".local", "bin")
			searchPath := tools + ":/usr/bin:/bin"
			args := []string{script, "v9.9.9"}
			shell := "/bin/zsh"
			profile := filepath.Join(home, ".zshrc")
			switch scenario {
			case "bash":
				shell = "/bin/bash"
				profile = filepath.Join(home, ".bashrc")
			case "unknown shell":
				shell = "/bin/fish"
			case "existing", "failed download", "symlink":
				destination = filepath.Join(home, "existing bin")
				require.NoError(t, os.MkdirAll(destination, 0755))
				if scenario == "symlink" {
					require.NoError(t, os.Symlink("/bin/echo", filepath.Join(destination, "teamcity")))
				} else {
					require.NoError(t, os.WriteFile(filepath.Join(destination, "teamcity"), []byte("old version"), 0755))
				}
				searchPath = destination + ":" + searchPath
			case "explicit":
				destination = filepath.Join(home, "custom ' bin")
				args = append(args, destination)
			case "relative":
				destination = filepath.Join(home, "relative bin")
				args = append(args, "relative bin")
			}
			require.NoError(t, os.WriteFile(profile, []byte("export KEEP_ME=1\n"), 0600))
			run := func() ([]byte, error) {
				command := exec.Command("bash", args...)
				command.Dir = home
				command.Env = append(os.Environ(), "HOME="+home, "PATH="+searchPath, "SHELL="+shell, "ZDOTDIR="+home, "INSTALL_TEST_ARCHIVE="+archivePath)
				return command.CombinedOutput()
			}
			output, err := run()
			if scenario == "symlink" || scenario == "failed download" {
				require.Error(t, err, string(output))
				if scenario == "failed download" {
					contents, err := os.ReadFile(filepath.Join(destination, "teamcity"))
					require.NoError(t, err)
					assert.Equal(t, "old version", string(contents))
				}
				return
			}
			require.NoError(t, err, string(output))
			contents, err := os.ReadFile(filepath.Join(destination, "teamcity"))
			require.NoError(t, err)
			assert.Equal(t, binary, string(contents))
			output, err = run()
			require.NoError(t, err, string(output))
			contents, err = os.ReadFile(profile)
			require.NoError(t, err)
			assert.Contains(t, string(contents), "export KEEP_ME=1")
			if scenario == "existing" || scenario == "unknown shell" {
				assert.NotContains(t, string(contents), "export PATH=")
			} else {
				assert.Equal(t, 1, strings.Count(string(contents), "export PATH="))
				command := exec.Command("bash", "-c", `. "$1"; command -v teamcity`, "probe", profile)
				command.Env = []string{"PATH=" + searchPath}
				output, err = command.Output()
				require.NoError(t, err)
				resolved, err := filepath.EvalSymlinks(strings.TrimSpace(string(output)))
				require.NoError(t, err)
				expected, err := filepath.EvalSymlinks(filepath.Join(destination, "teamcity"))
				require.NoError(t, err)
				assert.Equal(t, expected, resolved)
			}
		})
	}
}
