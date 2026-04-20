package api

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
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

// Client represents a TeamCity API client
type Client struct {
	BaseURL    string
	Token      string
	APIVersion string // Optional: pin to a specific API version (e.g., "2020.1")
	HTTPClient *http.Client

	// DebugFunc, when set, receives debug log messages for HTTP requests/responses.
	// Use WithDebugFunc to configure.
	DebugFunc func(format string, args ...any)

	// ReadOnly, when true, blocks all non-GET requests.
	// Use WithReadOnly to configure.
	ReadOnly bool

	// Basic auth credentials (used instead of Token if set)
	basicUser string
	basicPass string

	// Guest auth (no credentials, uses /guestAuth/ URL prefix)
	guestAuth bool

	version     string // CLI version for request headers
	commandName string // CLI command name for X-TeamCity-Client header

	// Cached server info
	serverInfo     *Server
	serverInfoOnce sync.Once
	serverInfoErr  error
}

func (c *Client) debugLog(format string, args ...any) {
	if c.DebugFunc != nil {
		c.DebugFunc(format, args...)
	}
}

func (c *Client) debugLogRequest(req *http.Request) {
	if c.DebugFunc == nil {
		return
	}
	c.debugLog("> %s %s", req.Method, req.URL.String())
	c.debugLogHeaders(">", req.Header)
}

func (c *Client) debugLogResponse(resp *http.Response) {
	if c.DebugFunc == nil {
		return
	}
	c.debugLog("< %s %s", resp.Proto, resp.Status)
	c.debugLogHeaders("<", resp.Header)
}

func (c *Client) debugLogHeaders(prefix string, headers http.Header) {
	names := make([]string, 0, len(headers))
	for name := range headers {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		values := headers[name]
		if sensitiveHeaders[name] {
			c.debugLog("%s %s: [REDACTED]", prefix, name)
		} else {
			for _, value := range values {
				c.debugLog("%s %s: %s", prefix, name, value)
			}
		}
	}
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

// WithDebugFunc sets a function to receive debug log messages for HTTP requests/responses.
func WithDebugFunc(f func(format string, args ...any)) ClientOption {
	return func(c *Client) {
		c.DebugFunc = f
	}
}

// WithReadOnly sets the client to read-only mode, blocking all non-GET requests.
func WithReadOnly(readOnly bool) ClientOption {
	return func(c *Client) {
		c.ReadOnly = readOnly
	}
}

// WithVersion sets the CLI version for request identification headers.
func WithVersion(v string) ClientOption {
	return func(c *Client) {
		c.version = v
	}
}

// WithCommandName sets the command name for X-TeamCity-Client header.
func WithCommandName(name string) ClientOption {
	return func(c *Client) {
		c.commandName = name
	}
}

// NewClient creates a new TeamCity API client with Bearer token authentication
func NewClient(baseURL, token string, opts ...ClientOption) *Client {
	baseURL = strings.TrimSuffix(baseURL, "/")

	c := &Client{
		BaseURL: baseURL,
		Token:   token,
		HTTPClient: &http.Client{
			Timeout:   30 * time.Second,
			Transport: defaultTransport(),
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

	c := &Client{
		BaseURL:   baseURL,
		basicUser: username,
		basicPass: password,
		HTTPClient: &http.Client{
			Timeout:   30 * time.Second,
			Transport: defaultTransport(),
		},
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// NewGuestClient creates a new TeamCity API client with guest authentication.
// Guest auth uses the /guestAuth/ URL prefix and sends no credentials.
func NewGuestClient(baseURL string, opts ...ClientOption) *Client {
	baseURL = strings.TrimSuffix(baseURL, "/")

	c := &Client{
		BaseURL:   baseURL,
		guestAuth: true,
		HTTPClient: &http.Client{
			Timeout:   30 * time.Second,
			Transport: defaultTransport(),
		},
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// apiPath returns the API path, optionally with version prefix
func (c *Client) apiPath(path string) string {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if c.APIVersion != "" && strings.HasPrefix(path, "/app/rest/") {
		path = strings.Replace(path, "/app/rest/", "/app/rest/"+c.APIVersion+"/", 1)
	}
	if c.guestAuth && !strings.HasPrefix(path, "/guestAuth/") {
		path = "/guestAuth" + path
	}
	return path
}

func (c *Client) cliVersion() string {
	return cmp.Or(c.version, "dev")
}

func (c *Client) userAgent() string {
	return fmt.Sprintf("teamcity-cli/%s (%s; %s)", c.cliVersion(), runtime.GOOS, runtime.GOARCH)
}

func (c *Client) teamCityClientHeader() string {
	h := "teamcity-cli/" + c.cliVersion()
	if c.commandName != "" {
		h += " (command: " + c.commandName + ")"
	}
	return h
}

// SetCommandName sets the command name for X-TeamCity-Client header.
func (c *Client) SetCommandName(name string) {
	c.commandName = name
}

func (c *Client) setAuth(req *http.Request) {
	if c.guestAuth {
		return
	}
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
	case "vcs_test_connection":
		return server.VersionMajor > 2024 ||
			(server.VersionMajor == 2024 && server.VersionMinor >= 12)
	default:
		return true
	}
}

func (c *Client) doRequest(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	return c.doRequestWithContentType(ctx, method, path, body, "application/json")
}

func (c *Client) doRequestWithContentType(ctx context.Context, method, path string, body io.Reader, contentType string) (*http.Response, error) {
	return c.doRequestFull(ctx, method, path, body, contentType, "application/json")
}

func (c *Client) doRequestWithAccept(ctx context.Context, method, path string, body io.Reader, accept string) (*http.Response, error) {
	return c.doRequestFull(ctx, method, path, body, "application/json", accept)
}

func (c *Client) doRequestFull(ctx context.Context, method, path string, body io.Reader, contentType, accept string) (*http.Response, error) {
	if c.ReadOnly && method != "GET" {
		return nil, fmt.Errorf("%w: %s %s", ErrReadOnly, method, path)
	}

	reqURL := fmt.Sprintf("%s%s", c.BaseURL, c.apiPath(path))

	req, err := http.NewRequestWithContext(ctx, method, reqURL, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setAuth(req)
	req.Header.Set("Accept", accept)
	req.Header.Set("User-Agent", c.userAgent())
	req.Header.Set("X-TeamCity-Client", c.teamCityClientHeader())
	if body != nil {
		req.Header.Set("Content-Type", contentType)
	}

	c.debugLogRequest(req)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	c.debugLogResponse(resp)

	return resp, nil
}

func (c *Client) get(ctx context.Context, path string, result any) error {
	return c.getWithRetry(ctx, path, result, ReadRetry)
}

// doGetStream GETs with ReadRetry and returns the raw 2xx response; non-2xx → typed api error.
func (c *Client) doGetStream(ctx context.Context, path string) (*http.Response, error) {
	resp, err := withRetry(ReadRetry, func() (*http.Response, error) {
		return c.doRequest(ctx, "GET", path, nil)
	})
	if err != nil {
		if resp != nil {
			defer func() { _ = resp.Body.Close() }()
			return nil, c.handleErrorResponse(resp)
		}
		return nil, &NetworkError{URL: c.BaseURL, Cause: err}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer func() { _ = resp.Body.Close() }()
		return nil, c.handleErrorResponse(resp)
	}
	return resp, nil
}

func (c *Client) getWithRetry(ctx context.Context, path string, result any, retry RetryConfig) error {
	resp, err := withRetry(retry, func() (*http.Response, error) {
		return c.doRequest(ctx, "GET", path, nil)
	})
	if err != nil {
		if resp != nil { // exhausted on HTTP status, preserve the typed error
			defer func() { _ = resp.Body.Close() }()
			return c.handleErrorResponse(resp)
		}
		return &NetworkError{URL: c.BaseURL, Cause: err}
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

// handleErrorResponse is a method-receiver alias for ErrorFromResponse.
func (c *Client) handleErrorResponse(resp *http.Response) error {
	return ErrorFromResponse(resp)
}

// ExtractErrorMessage returns the primary message from a TeamCity error body.
func ExtractErrorMessage(body []byte) string {
	return parseWire(body).Message
}

// post performs a POST request without retry (non-idempotent by default).
func (c *Client) post(ctx context.Context, path string, body io.Reader, result any) error {
	return c.postWithRetry(ctx, path, body, result, NoRetry)
}

// postWithRetry performs a POST request with configurable retry.
func (c *Client) postWithRetry(ctx context.Context, path string, body io.Reader, result any, retry RetryConfig) error {
	resp, err := withRetry(retry, func() (*http.Response, error) {
		return c.doRequest(ctx, "POST", path, body)
	})
	if err != nil {
		if resp != nil {
			defer func() { _ = resp.Body.Close() }()
			return c.handleErrorResponse(resp)
		}
		return &NetworkError{URL: c.BaseURL, Cause: err}
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
func (c *Client) doNoContent(ctx context.Context, method, path string, body io.Reader, contentType string) error {
	var resp *http.Response
	var err error

	if contentType == "" {
		resp, err = c.doRequest(ctx, method, path, body)
	} else {
		accept := "application/json"
		if contentType == "text/plain" {
			accept = "text/plain"
		}
		resp, err = c.doRequestFull(ctx, method, path, body, contentType, accept)
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

// RawRequest performs a raw HTTP request and returns the response without parsing.
func (c *Client) RawRequest(ctx context.Context, method, path string, body io.Reader, headers map[string]string) (*RawResponse, error) {
	if c.ReadOnly && method != "GET" {
		return nil, fmt.Errorf("%w: %s %s", ErrReadOnly, method, path)
	}

	resp, err := c.doRawRequest(ctx, method, path, body, headers, "application/json")
	if err != nil {
		return nil, err
	}

	// TeamCity returns 406 when it can only produce XML for an error but the client
	// requested JSON. Retry with Accept: */* to get the real error.
	if resp.StatusCode == http.StatusNotAcceptable {
		resp, err = c.doRawRequest(ctx, method, path, body, headers, "*/*")
		if err != nil {
			return nil, err
		}
	}

	return resp, nil
}

func (c *Client) doRawRequest(ctx context.Context, method, path string, body io.Reader, headers map[string]string, accept string) (*RawResponse, error) {
	reqURL := fmt.Sprintf("%s%s", c.BaseURL, c.apiPath(path))

	req, err := http.NewRequestWithContext(ctx, method, reqURL, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setAuth(req)
	req.Header.Set("Accept", accept)
	req.Header.Set("User-Agent", c.userAgent())
	req.Header.Set("X-TeamCity-Client", c.teamCityClientHeader())
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	c.debugLogRequest(req)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, &NetworkError{URL: c.BaseURL, Cause: err}
	}
	defer func() { _ = resp.Body.Close() }()

	c.debugLogResponse(resp)

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
