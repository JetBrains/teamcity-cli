package project

import (
	"cmp"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/JetBrains/teamcity-cli/internal/browserflow"
	"github.com/JetBrains/teamcity-cli/internal/output"
)

const (
	manifestFlowTimeout = 10 * time.Minute
	defaultGitHubAPI    = "https://api.github.com"
	defaultGitHubURL    = "https://github.com"
	manifestStartPath   = "/start"
	manifestCBPath      = "/cb"
)

// githubAPIHost is overridden in tests to point at a mock api.github.com.
var githubAPIHost = defaultGitHubAPI

// githubURLHost is overridden in tests to point at a mock github.com (form action).
var githubURLHost = defaultGitHubURL

// defaultManifestPermissions matches what TeamCity's GitHub App needs for VCS clone, status reporting, PR check-runs, and issue lookups.
var defaultManifestPermissions = map[string]string{
	"contents":      "write",
	"metadata":      "read",
	"pull_requests": "write",
	"issues":        "read",
	"statuses":      "write",
	"checks":        "write",
}

// defaultManifestEvents is empty: TeamCity polls VCS roots; webhooks need a public TeamCity URL we shouldn't assume.
var defaultManifestEvents = []string{}

// manifestCreds is the subset of GitHub's app-manifest conversion response we need.
type manifestCreds struct {
	AppID         int64         `json:"id"`
	Slug          string        `json:"slug"`
	ClientID      string        `json:"client_id"`
	ClientSecret  string        `json:"client_secret"`
	PEM           string        `json:"pem"`
	HTMLURL       string        `json:"html_url"`
	WebhookSecret string        `json:"webhook_secret"`
	Owner         manifestOwner `json:"owner"`
}

// manifestOwner captures the App owner (the user/org under which it was registered).
type manifestOwner struct {
	Login   string `json:"login"`
	HTMLURL string `json:"html_url"`
}

// runGitHubAppManifestFlow registers a GitHub App via the manifest flow and returns its credentials.
func runGitHubAppManifestFlow(ctx context.Context, p *output.Printer, serverURL, appName, projectID, org string) (*manifestCreds, error) {
	listener, err := browserflow.FindAvailableListener()
	if err != nil {
		return nil, fmt.Errorf("find available port: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port

	state, err := browserflow.GenerateState()
	if err != nil {
		return nil, fmt.Errorf("generate state: %w", err)
	}

	redirectURL := fmt.Sprintf("http://localhost:%d%s", port, manifestCBPath)
	manifestJSON, err := buildManifest(appName, serverURL, redirectURL, projectID)
	if err != nil {
		return nil, fmt.Errorf("build manifest: %w", err)
	}

	startHandler := buildStartHandler(org, manifestJSON, state)

	target := cmp.Or(org, "your account")
	p.Info("Opening browser to register a GitHub App for %s...", target)
	_, _ = fmt.Fprintf(p.Out, "  %s Click \"Create GitHub App for %s\" on GitHub\n", output.Yellow("→"), target)

	openURL := fmt.Sprintf("http://localhost:%d%s", port, manifestStartPath)
	result, err := browserflow.Run(ctx, browserflow.Options{
		Listener:     listener,
		State:        state,
		OpenURL:      openURL,
		StartHandler: startHandler,
		CallbackPath: manifestCBPath,
		Timeout:      manifestFlowTimeout,
		Logger:       p,
		SuccessTitle: "GitHub App registered",
		SuccessBody:  "Credentials captured. You can close this tab and return to the terminal.",
	})
	if err != nil {
		return nil, err
	}

	exchangeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	creds, err := exchangeManifestCode(exchangeCtx, result.Code)
	if err != nil {
		return nil, fmt.Errorf("exchange manifest code: %w", err)
	}
	return creds, nil
}

// projectConnectionsURL is where TeamCity admins manage this project's connections; we use it as the App's homepage.
func projectConnectionsURL(serverURL, projectID string) string {
	return fmt.Sprintf("%s/admin/editProject.html?projectId=%s&tab=oauthConnections",
		strings.TrimSuffix(serverURL, "/"), url.QueryEscape(projectID))
}

// buildManifest produces the JSON we POST to GitHub's manifest endpoint. appName must already be sanitized.
func buildManifest(appName, serverURL, redirectURL, projectID string) ([]byte, error) {
	trimmed := strings.TrimSuffix(serverURL, "/")
	manifest := map[string]any{
		"name":                appName,
		"url":                 projectConnectionsURL(trimmed, projectID),
		"redirect_url":        redirectURL,
		"callback_urls":       []string{trimmed + "/oauth/githubapp/accessToken.html"},
		"description":         fmt.Sprintf("Created by teamcity-cli for project %s.", projectID),
		"public":              false,
		"default_permissions": defaultManifestPermissions,
		"default_events":      defaultManifestEvents,
	}
	return json.Marshal(manifest)
}

// githubAppMaxNameLen is GitHub's documented limit for App display names.
const githubAppMaxNameLen = 34

// githubAppName sanitizes name to fit GitHub's rules: ≤34 chars, no leading "GitHub"/"Gist", no slashes.
func githubAppName(name string) string {
	name = strings.ReplaceAll(strings.TrimSpace(name), "/", "-")
	lower := strings.ToLower(name)
	if strings.HasPrefix(lower, "github") || strings.HasPrefix(lower, "gist") {
		name = "TC " + name
	}
	if len(name) > githubAppMaxNameLen {
		name = strings.TrimSpace(name[:githubAppMaxNameLen])
	}
	return name
}

// defaultGitHubAppName builds a unique-by-default name combining project + server host: "TC <project>@<host>".
func defaultGitHubAppName(projectID, serverURL string) string {
	host := serverURL
	if u, err := url.Parse(serverURL); err == nil && u.Host != "" {
		host = u.Host
	}
	return githubAppName(fmt.Sprintf("TC %s@%s", projectID, host))
}

//go:embed templates/manifest_start.html
var startPageHTML string
var startPageTmpl = template.Must(template.New("manifest-start").Parse(startPageHTML))

// buildStartHandler returns an http.Handler that auto-submits the manifest to GitHub.
func buildStartHandler(org string, manifestJSON []byte, state string) http.Handler {
	data := struct {
		Action, Manifest, State string
	}{
		Action:   manifestSubmitURL(org),
		Manifest: string(manifestJSON),
		State:    state,
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		_ = startPageTmpl.Execute(w, data)
	})
}

// manifestSubmitURL returns the GitHub URL the form should POST to (org or personal).
func manifestSubmitURL(org string) string {
	base := strings.TrimSuffix(githubURLHost, "/")
	if org != "" {
		return fmt.Sprintf("%s/organizations/%s/settings/apps/new", base, org)
	}
	return base + "/settings/apps/new"
}

// exchangeManifestCode trades the one-shot code for the new GitHub App's credentials.
func exchangeManifestCode(ctx context.Context, code string) (*manifestCreds, error) {
	url := fmt.Sprintf("%s/app-manifests/%s/conversions", strings.TrimSuffix(githubAPIHost, "/"), code)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "teamcity-cli")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call api.github.com: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("github returned status %d: %s", resp.StatusCode, body)
	}
	var creds manifestCreds
	if err := json.Unmarshal(body, &creds); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if creds.ClientID == "" || creds.PEM == "" {
		return nil, errors.New("github response missing required fields")
	}
	return &creds, nil
}
