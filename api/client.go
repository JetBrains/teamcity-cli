package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	tcerrors "github.com/JetBrains/teamcity-cli/internal/errors"
	"github.com/JetBrains/teamcity-cli/internal/output"
)

// Minimum supported TeamCity version
const (
	MinMajorVersion = 2020
	MinMinorVersion = 1
)

// sensitiveHeaders lists headers that should be redacted in debug output
var sensitiveHeaders = map[string]bool{
	"Authorization": true,
	"Cookie":        true,
	"Set-Cookie":    true,
}

// debugLogRequest logs HTTP request details when verbose mode is enabled
func debugLogRequest(req *http.Request) {
	if !output.Verbose {
		return
	}
	output.Debug("> %s %s", req.Method, req.URL.String())
	debugLogHeaders(">", req.Header)
}

// debugLogResponse logs HTTP response details when verbose mode is enabled
func debugLogResponse(resp *http.Response) {
	if !output.Verbose {
		return
	}
	output.Debug("< %s %s", resp.Proto, resp.Status)
	debugLogHeaders("<", resp.Header)
}

// debugLogHeaders logs HTTP headers with sensitive values redacted
func debugLogHeaders(prefix string, headers http.Header) {
	names := make([]string, 0, len(headers))
	for name := range headers {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		values := headers[name]
		if sensitiveHeaders[name] {
			output.Debug("%s %s: [REDACTED]", prefix, name)
		} else {
			for _, value := range values {
				output.Debug("%s %s: %s", prefix, name, value)
			}
		}
	}
}

// Client represents a TeamCity API client
type Client struct {
	BaseURL    string
	Token      string
	APIVersion string // Optional: pin to a specific API version (e.g., "2020.1")
	HTTPClient *http.Client

	// Basic auth credentials (used instead of Token if set)
	basicUser string
	basicPass string

	// Cached server info
	serverInfo     *Server
	serverInfoOnce sync.Once
	serverInfoErr  error
}

// ClientOption allows configuring the client
type ClientOption func(*Client)

// WithAPIVersion pins the client to a specific API version
func WithAPIVersion(version string) ClientOption {
	return func(c *Client) {
		c.APIVersion = version
	}
}

// WithTimeout sets a custom HTTP timeout
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *Client) {
		c.HTTPClient.Timeout = timeout
	}
}

// NewClient creates a new TeamCity API client with Bearer token authentication
func NewClient(baseURL, token string, opts ...ClientOption) *Client {
	baseURL = strings.TrimSuffix(baseURL, "/")

	// Security: Warn about insecure HTTP connections (suppress with TC_INSECURE_SKIP_WARN=1)
	if strings.HasPrefix(baseURL, "http://") && os.Getenv("TC_INSECURE_SKIP_WARN") == "" {
		fmt.Fprintln(os.Stderr, "WARNING: Using insecure HTTP connection. Your authentication token will be transmitted in plaintext.")
		fmt.Fprintln(os.Stderr, "         Consider using HTTPS for secure communication.")
	}

	c := &Client{
		BaseURL: baseURL,
		Token:   token,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// NewClientWithBasicAuth creates a new TeamCity API client with Basic authentication.
// Use empty username with superuser token, or username/password for regular users.
func NewClientWithBasicAuth(baseURL, username, password string, opts ...ClientOption) *Client {
	baseURL = strings.TrimSuffix(baseURL, "/")

	if strings.HasPrefix(baseURL, "http://") && os.Getenv("TC_INSECURE_SKIP_WARN") == "" {
		fmt.Fprintln(os.Stderr, "WARNING: Using insecure HTTP connection. Your credentials will be transmitted in plaintext.")
		fmt.Fprintln(os.Stderr, "         Consider using HTTPS for secure communication.")
	}

	c := &Client{
		BaseURL:   baseURL,
		basicUser: username,
		basicPass: password,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// apiPath returns the API path, optionally with version prefix
func (c *Client) apiPath(path string) string {
	if c.APIVersion != "" && strings.HasPrefix(path, "/app/rest/") {
		return strings.Replace(path, "/app/rest/", "/app/rest/"+c.APIVersion+"/", 1)
	}
	return path
}

func (c *Client) setAuth(req *http.Request) {
	if c.basicPass != "" || c.basicUser != "" {
		req.SetBasicAuth(c.basicUser, c.basicPass)
	} else {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
}

// ServerVersion returns cached server version info
func (c *Client) ServerVersion() (*Server, error) {
	c.serverInfoOnce.Do(func() {
		c.serverInfo, c.serverInfoErr = c.GetServer()
	})
	return c.serverInfo, c.serverInfoErr
}

// CheckVersion verifies the server meets minimum version requirements
func (c *Client) CheckVersion() error {
	server, err := c.ServerVersion()
	if err != nil {
		return fmt.Errorf("failed to get server version: %w", err)
	}

	if server.VersionMajor < MinMajorVersion ||
		(server.VersionMajor == MinMajorVersion && server.VersionMinor < MinMinorVersion) {
		return fmt.Errorf("TeamCity %d.%d is not supported (minimum: %d.%d)",
			server.VersionMajor, server.VersionMinor, MinMajorVersion, MinMinorVersion)
	}

	return nil
}

// SupportsFeature checks if the server supports a specific feature
func (c *Client) SupportsFeature(feature string) bool {
	server, err := c.ServerVersion()
	if err != nil {
		return false
	}

	switch feature {
	case "csrf_token":
		return server.VersionMajor >= 2020
	case "pipelines":
		return server.VersionMajor >= 2024
	default:
		return true
	}
}

func (c *Client) doRequest(method, path string, body io.Reader) (*http.Response, error) {
	return c.doRequestWithContentType(method, path, body, "application/json")
}

func (c *Client) doRequestWithContentType(method, path string, body io.Reader, contentType string) (*http.Response, error) {
	return c.doRequestFull(method, path, body, contentType, "application/json")
}

func (c *Client) doRequestWithAccept(method, path string, body io.Reader, accept string) (*http.Response, error) {
	return c.doRequestFull(method, path, body, "application/json", accept)
}

func (c *Client) doRequestFull(method, path string, body io.Reader, contentType, accept string) (*http.Response, error) {
	reqURL := fmt.Sprintf("%s%s", c.BaseURL, c.apiPath(path))

	req, err := http.NewRequest(method, reqURL, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setAuth(req)
	req.Header.Set("Accept", accept)
	if body != nil {
		req.Header.Set("Content-Type", contentType)
	}

	debugLogRequest(req)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	debugLogResponse(resp)

	return resp, nil
}

func (c *Client) get(path string, result any) error {
	return c.getWithRetry(path, result, ReadRetry)
}

func (c *Client) getWithRetry(path string, result any, retry RetryConfig) error {
	resp, err := withRetry(retry, func() (*http.Response, error) {
		return c.doRequest("GET", path, nil)
	})
	if err != nil {
		return tcerrors.NetworkError(c.BaseURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return c.handleErrorResponse(resp)
	}

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}

func (c *Client) handleErrorResponse(resp *http.Response) error {
	bodyBytes, _ := io.ReadAll(resp.Body)

	message := extractErrorMessage(bodyBytes)

	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return tcerrors.AuthenticationFailed()
	case http.StatusForbidden:
		return tcerrors.PermissionDenied("perform this action")
	case http.StatusNotFound:
		if message != "" {
			return tcerrors.WithSuggestion(message, notFoundHint(message))
		}
		return tcerrors.WithSuggestion("Resource not found", "Check the ID and try again")
	default:
		if message != "" {
			return tcerrors.New(message)
		}
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}
}

// notFoundHint returns a context-aware hint based on the error message
func notFoundHint(message string) string {
	msg := strings.ToLower(message)
	switch {
	case strings.Contains(msg, "agent pool"), strings.Contains(msg, "pool"):
		return "Use 'teamcity pool list' to see available pools"
	case strings.Contains(msg, "agent"):
		return "Use 'teamcity agent list' to see available agents"
	case strings.Contains(msg, "project"):
		return "Use 'teamcity project list' to see available projects"
	case strings.Contains(msg, "build type"), strings.Contains(msg, "job"):
		return "Use 'teamcity job list' to see available jobs"
	default:
		return "Use 'teamcity job list' or 'teamcity run list' to see available resources"
	}
}

// extractErrorMessage extracts a clean error message from TeamCity's API response.
func extractErrorMessage(body []byte) string {
	// Try JSON format first
	var errResp APIErrorResponse
	if err := json.Unmarshal(body, &errResp); err == nil {
		if len(errResp.Errors) > 0 {
			return humanizeErrorMessage(errResp.Errors[0].Message)
		}
		return ""
	}

	text := strings.TrimSpace(string(body))
	if len(text) > 0 && len(text) < 200 && !strings.HasPrefix(text, "<") {
		return humanizeErrorMessage(text)
	}

	return ""
}

// humanizeErrorMessage converts TeamCity's technical error messages to user-friendly ones
func humanizeErrorMessage(msg string) string {
	// "No build types found by locator 'X'." -> "job 'X' not found"
	if strings.HasPrefix(msg, "No build types found by locator '") {
		id := extractIDFromLocator(msg, "No build types found by locator '")
		return fmt.Sprintf("job '%s' not found", id)
	}

	// "No build found by locator 'X'." -> "run 'X' not found"
	if strings.HasPrefix(msg, "No build found by locator '") {
		id := extractIDFromLocator(msg, "No build found by locator '")
		return fmt.Sprintf("run '%s' not found", id)
	}

	// "No project found by locator 'X'." -> "project 'X' not found"
	if strings.HasPrefix(msg, "No project found by locator '") {
		id := extractIDFromLocator(msg, "No project found by locator '")
		return fmt.Sprintf("project '%s' not found", id)
	}

	// "Nothing is found by locator 'count:1,buildType:(id:X)'" -> "no runs found for job 'X'"
	if strings.Contains(msg, "Nothing is found by locator") && strings.Contains(msg, "buildType:(id:") {
		start := strings.Index(msg, "buildType:(id:")
		if start != -1 {
			start += len("buildType:(id:")
			end := strings.Index(msg[start:], ")")
			if end != -1 {
				id := msg[start : start+end]
				return fmt.Sprintf("no runs found for job '%s'", id)
			}
		}
	}

	// Generic "Nothing is found by locator" errors
	if strings.Contains(msg, "Nothing is found by locator") {
		if start := strings.Index(msg, "id:"); start != -1 {
			start += 3
			end := strings.IndexAny(msg[start:], "',)")
			if end != -1 {
				id := msg[start : start+end]
				return fmt.Sprintf("resource '%s' not found", id)
			}
		}
	}

	// Content-Type error typically means the build was not found in queue
	if strings.Contains(msg, "Content-Type") && strings.Contains(msg, "header") {
		return "build not found in queue"
	}

	return msg
}

// extractIDFromLocator extracts the ID from an error message containing a locator.
// For "No X found by locator 'count:1,id:foo'...", returns "foo".
// For simple locators like "id:foo", also returns "foo".
func extractIDFromLocator(msg, prefix string) string {
	locator := strings.TrimPrefix(msg, prefix)
	// Find the closing quote of the locator
	endQuote := strings.Index(locator, "'")
	if endQuote != -1 {
		locator = locator[:endQuote]
	}

	// Try to find "id:" in the locator and extract its value
	if _, after, ok := strings.Cut(locator, "id:"); ok {
		idValue := after
		// The ID ends at comma, closing paren, or end of string
		endIdx := strings.IndexAny(idValue, ",)")
		if endIdx != -1 {
			return idValue[:endIdx]
		}
		return idValue
	}

	// Fallback: return the whole locator if no "id:" found
	return locator
}

// post performs a POST request without retry (non-idempotent by default).
func (c *Client) post(path string, body io.Reader, result any) error {
	return c.postWithRetry(path, body, result, NoRetry)
}

// postWithRetry performs a POST request with configurable retry.
func (c *Client) postWithRetry(path string, body io.Reader, result any, retry RetryConfig) error {
	resp, err := withRetry(retry, func() (*http.Response, error) {
		return c.doRequest("POST", path, body)
	})
	if err != nil {
		return tcerrors.NetworkError(c.BaseURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return c.handleErrorResponse(resp)
	}

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}

// doNoContent performs a request expecting 200/204 with no response body.
// Use for mutations (PUT/DELETE/POST) that don't return data.
func (c *Client) doNoContent(method, path string, body io.Reader, contentType string) error {
	var resp *http.Response
	var err error

	if contentType == "" {
		resp, err = c.doRequest(method, path, body)
	} else {
		accept := "application/json"
		if contentType == "text/plain" {
			accept = "text/plain"
		}
		resp, err = c.doRequestFull(method, path, body, contentType, accept)
	}
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return c.handleErrorResponse(resp)
	}

	return nil
}

// RawResponse represents the response from a raw API request
type RawResponse struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
}

// RawRequest performs a raw HTTP request and returns the response without parsing
func (c *Client) RawRequest(method, path string, body io.Reader, headers map[string]string) (*RawResponse, error) {
	reqURL := fmt.Sprintf("%s%s", c.BaseURL, c.apiPath(path))

	req, err := http.NewRequest(method, reqURL, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setAuth(req)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Apply custom headers (can override defaults)
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	debugLogRequest(req)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, tcerrors.NetworkError(c.BaseURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	debugLogResponse(resp)

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return &RawResponse{
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
		Body:       respBody,
	}, nil
}
