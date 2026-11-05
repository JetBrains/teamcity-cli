package api

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
)

// ErrNoGitHubAppUserToken means the current user has not authorized the GitHub App connection yet.
var ErrNoGitHubAppUserToken = errors.New("no GitHub App user token for the current user")

// validTokenName matches what TeamCity rejects in stored token names; allowed are [A-Za-z0-9_-], at most 30 characters.
var validTokenName = regexp.MustCompile(`[^A-Za-z0-9_-]`)

// IssueGitHubAppInstallationToken stores an installation token for repoURL on a GitHub App connection, scoped to projectID, and returns its token ID.
// The server derives it from the current user's stored token for the connection.
func (c *Client) IssueGitHubAppInstallationToken(projectID, connectionID, repoURL, tokenName string) (string, error) {
	const endpoint = "/app/oauth/githubapp/installationToken"
	repo := strings.TrimSuffix(path.Base(strings.TrimSuffix(repoURL, "/")), ".git")
	tokenName = validTokenName.ReplaceAllString(tokenName, "-")
	tokenName = tokenName[:min(len(tokenName), 30)]
	form := url.Values{
		"projectId":      {projectID},
		"connectionId":   {connectionID},
		"repositoryName": {repo},
		"tokenName":      {tokenName},
	}
	resp, err := c.doRequestFull(c.ctx(), "POST", endpoint, strings.NewReader(form.Encode()), "application/x-www-form-urlencoded", "application/json")
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}
	if mediaType, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type")); mediaType != "application/json" {
		return "", fmt.Errorf("%s answered with %q instead of JSON; a proxy in front of the server may require browser sign-in for it", endpoint, mediaType)
	}

	var result struct {
		TokenID string `json:"tokenId"`
		Message string `json:"message"`
	}
	_ = json.Unmarshal(body, &result)
	if resp.StatusCode == http.StatusOK && result.TokenID != "" {
		return result.TokenID, nil
	}
	if resp.StatusCode == http.StatusNotFound && strings.Contains(result.Message, "personal access token") {
		return "", ErrNoGitHubAppUserToken
	}
	return "", fmt.Errorf("installation token for %s (status %d): %s", repo, resp.StatusCode, cmp.Or(result.Message, strings.TrimSpace(string(body))))
}
