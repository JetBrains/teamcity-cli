package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
)

const (
	repoOwner    = "JetBrains"
	repoName     = "teamcity-cli"
	checkTimeout = 5 * time.Second
)

var (
	latestReleaseAPIURL      = fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", repoOwner, repoName)
	latestReleaseRedirectURL = fmt.Sprintf("https://github.com/%s/%s/releases/latest", repoOwner, repoName)
)

type ReleaseInfo struct {
	Version string
	URL     string
}

type githubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

func LatestRelease(ctx context.Context) (*ReleaseInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()

	release, err := latestReleaseFromAPI(ctx)
	if err != nil {
		fallbackRelease, fallbackErr := latestReleaseFromRedirect(ctx)
		if fallbackErr != nil {
			return nil, fmt.Errorf("%w; fallback failed: %w", err, fallbackErr)
		}
		return fallbackRelease, nil
	}
	return release, nil
}

func latestReleaseFromAPI(ctx context.Context) (*ReleaseInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, latestReleaseAPIURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "teamcity-cli")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	return &ReleaseInfo{
		Version: strings.TrimPrefix(release.TagName, "v"),
		URL:     release.HTMLURL,
	}, nil
}

func latestReleaseFromRedirect(ctx context.Context) (*ReleaseInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, latestReleaseRedirectURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "teamcity-cli")

	client := &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusMovedPermanently {
		return nil, fmt.Errorf("GitHub releases page returned %d", resp.StatusCode)
	}

	location := resp.Header.Get("Location")
	if location == "" {
		return nil, errors.New("GitHub releases page did not provide a redirect location")
	}

	redirectURL, err := url.Parse(location)
	if err != nil {
		return nil, fmt.Errorf("parse redirect location: %w", err)
	}

	version := strings.TrimPrefix(path.Base(redirectURL.Path), "v")
	if version == "" {
		return nil, fmt.Errorf("could not extract version from %q", location)
	}

	return &ReleaseInfo{
		Version: version,
		URL:     location,
	}, nil
}

func IsNewer(current, latest string) bool {
	curMajor, curMinor, curPatch := parseSemver(current)
	latMajor, latMinor, latPatch := parseSemver(latest)

	if latMajor != curMajor {
		return latMajor > curMajor
	}
	if latMinor != curMinor {
		return latMinor > curMinor
	}
	return latPatch > curPatch
}

func parseSemver(v string) (major, minor, patch int) {
	v = strings.TrimPrefix(v, "v")
	parts := strings.SplitN(v, "-", 2)
	parts = strings.SplitN(parts[0], ".", 4)

	if len(parts) >= 1 {
		major, _ = strconv.Atoi(parts[0])
	}
	if len(parts) >= 2 {
		minor, _ = strconv.Atoi(parts[1])
	}
	if len(parts) >= 3 {
		patch, _ = strconv.Atoi(parts[2])
	}
	return
}
