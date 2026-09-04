package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstallRelease(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unmanaged self-update is Unix-only")
	}
	for _, scenario := range []string{"success", "symlink", "checksum mismatch", "missing checksum", "wrong version", "missing binary", "symlink member", "download failure", "canceled"} {
		t.Run(scenario, func(t *testing.T) {
			var archive bytes.Buffer
			compressed := gzip.NewWriter(&archive)
			writer := tar.NewWriter(compressed)
			binary := "#!/bin/sh\nprintf 'teamcity version 9.9.9\\n'\n"
			if scenario == "wrong version" {
				binary = "#!/bin/sh\nprintf 'teamcity version 8.8.8\\n'\n"
			}
			header := &tar.Header{Name: "teamcity", Mode: 0755, Size: int64(len(binary))}
			if scenario == "missing binary" {
				header.Name = "../teamcity"
			}
			if scenario == "symlink member" {
				header.Typeflag, header.Size, header.Linkname = tar.TypeSymlink, 0, "/bin/sh"
				binary = ""
			}
			require.NoError(t, writer.WriteHeader(header))
			_, err := io.WriteString(writer, binary)
			require.NoError(t, err)
			require.NoError(t, writer.Close())
			require.NoError(t, compressed.Close())
			arch := runtime.GOARCH
			if arch == "amd64" {
				arch = "x86_64"
			}
			asset := fmt.Sprintf("teamcity_9.9.9_%s_%s.tar.gz", runtime.GOOS, arch)
			checksum := fmt.Sprintf("%x  %s\n", sha256.Sum256(archive.Bytes()), asset)
			if scenario == "checksum mismatch" {
				checksum = fmt.Sprintf("%064d  %s\n", 0, asset)
			}
			if scenario == "missing checksum" {
				checksum = ""
			}
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				assert.Empty(t, request.Header.Get("Authorization"))
				if strings.HasSuffix(request.URL.Path, "/checksums.txt") {
					_, _ = io.WriteString(response, checksum)
					return
				}
				assert.Equal(t, "/v9.9.9/"+asset, request.URL.Path)
				if scenario == "download failure" {
					http.Error(response, "unavailable", http.StatusServiceUnavailable)
					return
				}
				_, _ = response.Write(archive.Bytes())
			}))
			defer server.Close()
			originalURL := releaseDownloadBaseURL
			releaseDownloadBaseURL = server.URL
			t.Cleanup(func() { releaseDownloadBaseURL = originalURL })
			dir := t.TempDir()
			executable := filepath.Join(dir, "teamcity")
			require.NoError(t, os.WriteFile(executable, []byte("old binary"), 0755))
			target := executable
			if scenario == "symlink" {
				target = filepath.Join(dir, "link")
				require.NoError(t, os.Symlink(executable, target))
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if scenario == "canceled" {
				cancel()
			}
			err = installRelease(ctx, "9.9.9", target)
			contents, readErr := os.ReadFile(executable)
			require.NoError(t, readErr)
			if scenario == "success" || scenario == "symlink" {
				require.NoError(t, err)
				assert.Equal(t, binary, string(contents))
				result, err := exec.Command(target, "--version").Output()
				require.NoError(t, err)
				assert.Equal(t, "teamcity version 9.9.9\n", string(result))
			} else {
				require.Error(t, err)
				assert.Equal(t, "old binary", string(contents))
			}
			entries, err := os.ReadDir(dir)
			require.NoError(t, err)
			for _, entry := range entries {
				assert.False(t, strings.HasPrefix(entry.Name(), ".teamcity-update-"))
			}
		})
	}
}

func TestUpgradeCommand(t *testing.T) {
	t.Parallel()
	for method, want := range map[InstallMethod][]string{
		InstallHomebrew:   {"brew", "upgrade", "teamcity"},
		InstallScoop:      {"scoop", "update", "teamcity"},
		InstallChocolatey: {"choco", "upgrade", "TeamCityCLI", "-y"},
		InstallNPM:        {"npm", "update", "-g", "@jetbrains/teamcity-cli"},
		InstallWinGet:     {"winget", "upgrade", "--id", "JetBrains.TeamCityCLI", "--exact", "--accept-package-agreements", "--accept-source-agreements"},
	} {
		command, args := method.upgradeCommand()
		assert.Equal(t, want, append([]string{command}, args...))
		assert.True(t, method.CanUpgrade())
	}
	for _, method := range []InstallMethod{InstallApt, InstallRPM, InstallArch, InstallGoInstall} {
		assert.False(t, method.CanUpgrade())
	}
}

func TestDownloadReleaseFileLimit(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		_, _ = io.WriteString(response, "too large")
	}))
	defer server.Close()
	require.ErrorContains(t, downloadReleaseFile(t.Context(), server.URL, io.Discard, 2), "size limit")
}

func TestInstallReleaseRejectsInvalidVersion(t *testing.T) {
	t.Parallel()
	require.ErrorContains(t, installRelease(t.Context(), "../../bad", "unused"), "invalid release version")
}

func TestUpgradeRunsScopedPackageManager(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix command fixture")
	}
	dir := t.TempDir()
	shim := filepath.Join(dir, "brew")
	require.NoError(t, os.WriteFile(shim, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\"\nexit \"${UPGRADE_TEST_EXIT:-0}\"\n"), 0755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	var output bytes.Buffer
	require.NoError(t, Upgrade(t.Context(), InstallHomebrew, "9.9.9", &output, io.Discard))
	assert.Equal(t, "upgrade\nteamcity\n", output.String())
	t.Setenv("UPGRADE_TEST_EXIT", "1")
	require.Error(t, Upgrade(t.Context(), InstallHomebrew, "9.9.9", io.Discard, io.Discard))
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	require.ErrorIs(t, Upgrade(ctx, InstallHomebrew, "9.9.9", io.Discard, io.Discard), context.Canceled)
}
