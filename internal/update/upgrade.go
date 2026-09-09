package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

var releaseDownloadBaseURL = "https://github.com/JetBrains/teamcity-cli/releases/download"

var releaseVersionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$`)

func (m InstallMethod) upgradeCommand() (string, []string) {
	switch m {
	case InstallHomebrew:
		return "brew", []string{"upgrade", "teamcity"}
	case InstallScoop:
		return "scoop", []string{"update", "teamcity"}
	case InstallChocolatey:
		return "choco", []string{"upgrade", "TeamCityCLI", "-y"}
	case InstallWinGet:
		return "winget", []string{"upgrade", "--id", "JetBrains.TeamCityCLI", "--exact", "--accept-package-agreements", "--accept-source-agreements"}
	case InstallNPM:
		return "npm", []string{"update", "-g", "@jetbrains/teamcity-cli"}
	default:
		return "", nil
	}
}

// CanUpgrade reports whether this installation has a safe automatic upgrade path.
func (m InstallMethod) CanUpgrade() bool {
	command, _ := m.upgradeCommand()
	return command != "" || ((m == InstallScript || m == InstallUnknown) && (runtime.GOOS == "darwin" || runtime.GOOS == "linux"))
}

// Upgrade updates through the owning package manager or replaces an unmanaged Unix executable in place.
func Upgrade(parent context.Context, method InstallMethod, version string, out, errOut io.Writer) error {
	ctx, cancel := context.WithTimeout(parent, 10*time.Minute)
	defer cancel()
	if command, args := method.upgradeCommand(); command != "" {
		process := exec.CommandContext(ctx, command, args...)
		process.Stdout, process.Stderr = out, errOut
		return process.Run()
	}
	if !method.CanUpgrade() {
		return fmt.Errorf("automatic updates are unavailable for %s; %s", method, method.UpdateCommand())
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	return installRelease(ctx, version, executable)
}

func installRelease(ctx context.Context, version, executable string) error {
	if !releaseVersionPattern.MatchString(version) {
		return errors.New("invalid release version")
	}
	executable, err := filepath.EvalSymlinks(executable)
	if err != nil {
		return err
	}
	info, err := os.Stat(executable)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("installed executable is not a regular file")
	}
	staged, err := os.CreateTemp(filepath.Dir(executable), ".teamcity-update-*")
	if err != nil {
		return fmt.Errorf("cannot stage update beside %s: %w", executable, err)
	}
	defer func() { _ = staged.Close(); _ = os.Remove(staged.Name()) }()

	arch := runtime.GOARCH
	if arch == "amd64" {
		arch = "x86_64"
	}
	asset := fmt.Sprintf("teamcity_%s_%s_%s.tar.gz", version, runtime.GOOS, arch)
	baseURL := releaseDownloadBaseURL + "/v" + version + "/"
	var manifest bytes.Buffer
	if err := downloadReleaseFile(ctx, baseURL+"checksums.txt", &manifest, 1<<20); err != nil {
		return fmt.Errorf("download checksums: %w", err)
	}
	var checksum string
	for line := range strings.SplitSeq(manifest.String(), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == asset {
			if checksum != "" {
				return errors.New("duplicate archive checksum")
			}
			checksum = fields[0]
		}
	}
	expected, err := hex.DecodeString(checksum)
	if err != nil || len(expected) != sha256.Size {
		return errors.New("missing or invalid archive checksum")
	}
	archive, err := os.CreateTemp("", "teamcity-release-*.tar.gz")
	if err != nil {
		return err
	}
	defer func() { _ = archive.Close(); _ = os.Remove(archive.Name()) }()
	hash := sha256.New()
	if err := downloadReleaseFile(ctx, baseURL+asset, io.MultiWriter(archive, hash), 256<<20); err != nil {
		return fmt.Errorf("download release: %w", err)
	}
	if !bytes.Equal(hash.Sum(nil), expected) {
		return errors.New("release archive checksum mismatch; installed binary unchanged")
	}
	if _, err := archive.Seek(0, io.SeekStart); err != nil {
		return err
	}
	compressed, err := gzip.NewReader(archive)
	if err != nil {
		return err
	}
	defer func() { _ = compressed.Close() }()
	reader := tar.NewReader(io.LimitReader(compressed, 512<<20))
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return errors.New("release archive does not contain teamcity")
		}
		if err != nil {
			return err
		}
		if header.Name != "teamcity" {
			continue
		}
		if header.Typeflag != tar.TypeReg || header.Size <= 0 || header.Size > 512<<20 {
			return errors.New("invalid teamcity executable in release archive")
		}
		if _, err := io.Copy(staged, reader); err != nil {
			return err
		}
		break
	}
	if err := staged.Chmod(info.Mode().Perm()); err != nil {
		return err
	}
	if err := staged.Sync(); err != nil {
		return err
	}
	if err := staged.Close(); err != nil {
		return err
	}
	verifyCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	verify := exec.CommandContext(verifyCtx, staged.Name(), "--version")
	verify.Env = append(os.Environ(), "DO_NOT_TRACK=1", "TEAMCITY_NO_UPDATE=1")
	output, err := verify.Output()
	if err != nil {
		return fmt.Errorf("verify downloaded executable: %w", err)
	}
	if strings.TrimSpace(string(output)) != "teamcity version "+version {
		return errors.New("downloaded executable version does not match release; installed binary unchanged")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return os.Rename(staged.Name(), executable)
}

func downloadReleaseFile(ctx context.Context, url string, destination io.Writer, limit int64) error {
	client := &http.Client{Timeout: 2 * time.Minute, CheckRedirect: func(request *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		if via[len(via)-1].URL.Scheme == "https" && request.URL.Scheme != "https" {
			return errors.New("refusing HTTPS downgrade redirect")
		}
		return nil
	}}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", response.StatusCode)
	}
	written, err := io.Copy(destination, io.LimitReader(response.Body, limit+1))
	if err != nil {
		return err
	}
	if written > limit {
		return errors.New("release download exceeds size limit")
	}
	return nil
}
