package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	tcerrors "github.com/tiulpin/teamcity-cli/internal/errors"
)

// Minimum supported TeamCity version
const (
	MinMajorVersion = 2020
	MinMinorVersion = 1
)

// Client represents a TeamCity API client
type Client struct {
	BaseURL    string
	Token      string
	APIVersion string // Optional: pin to a specific API version (e.g., "2020.1")
	HTTPClient *http.Client

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

// NewClient creates a new TeamCity API client
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

// apiPath returns the API path, optionally with version prefix
func (c *Client) apiPath(path string) string {
	if c.APIVersion != "" && strings.HasPrefix(path, "/app/rest/") {
		return strings.Replace(path, "/app/rest/", "/app/rest/"+c.APIVersion+"/", 1)
	}
	return path
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

	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Accept", accept)
	if body != nil {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	return resp, nil
}

func (c *Client) get(path string, result interface{}) error {
	resp, err := c.doRequest("GET", path, nil)
	if err != nil {
		return tcerrors.NetworkError(c.BaseURL)
	}
	defer resp.Body.Close()

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

	// Try to parse TeamCity's structured error response
	message := extractErrorMessage(bodyBytes)

	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return tcerrors.AuthenticationFailed()
	case http.StatusForbidden:
		return tcerrors.PermissionDenied("perform this action")
	case http.StatusNotFound:
		if message != "" {
			return tcerrors.WithSuggestion(message, "Use 'tc job list' or 'tc run list' to see available resources")
		}
		return tcerrors.WithSuggestion("Resource not found", "Check the ID and try again")
	default:
		if message != "" {
			return tcerrors.New(message)
		}
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}
}

// extractErrorMessage tries to extract a clean error message from TeamCity's API response
func extractErrorMessage(body []byte) string {
	var errResp APIErrorResponse
	if err := json.Unmarshal(body, &errResp); err == nil && len(errResp.Errors) > 0 {
		return humanizeErrorMessage(errResp.Errors[0].Message)
	}
	return ""
}

// humanizeErrorMessage converts TeamCity's technical error messages to user-friendly ones
func humanizeErrorMessage(msg string) string {
	// "No build types found by locator 'X'." -> "job 'X' not found"
	if strings.HasPrefix(msg, "No build types found by locator '") {
		id := strings.TrimPrefix(msg, "No build types found by locator '")
		id = strings.TrimSuffix(id, "'.")
		id = strings.TrimSuffix(id, "'")
		return fmt.Sprintf("job '%s' not found", id)
	}

	// "No build found by locator 'X'." -> "run 'X' not found"
	if strings.HasPrefix(msg, "No build found by locator '") {
		id := strings.TrimPrefix(msg, "No build found by locator '")
		id = strings.TrimSuffix(id, "'.")
		id = strings.TrimSuffix(id, "'")
		return fmt.Sprintf("run '%s' not found", id)
	}

	// "No project found by locator 'X'." -> "project 'X' not found"
	if strings.HasPrefix(msg, "No project found by locator '") {
		id := strings.TrimPrefix(msg, "No project found by locator '")
		id = strings.TrimSuffix(id, "'.")
		id = strings.TrimSuffix(id, "'")
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

	return msg
}

func (c *Client) post(path string, body io.Reader, result interface{}) error {
	resp, err := c.doRequest("POST", path, body)
	if err != nil {
		return tcerrors.NetworkError(c.BaseURL)
	}
	defer resp.Body.Close()

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

	// Set default headers
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Apply custom headers (can override defaults)
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, tcerrors.NetworkError(c.BaseURL)
	}
	defer resp.Body.Close()

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
