package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	PkceIsEnabledPath   = "/pkce/is_enabled.html"
	PkceAuthorizePath   = "/pkce/authorize.html"
	PkceTokenPath       = "/pkce/token.html"
	CodeChallengeMethod = "S256"
	DefaultCallbackPath = "/callback"
	CallbackPortMin     = 19000
	CallbackPortMax     = 19100
	maxResponseBody     = 64 * 1024
)

// AvailableScopes lists permissions to request via PKCE.
// The server filters these to only grant what it allows.
var AvailableScopes = []string{
	// View (read-only)
	"VIEW_PROJECT",
	"VIEW_BUILD_CONFIGURATION_SETTINGS",
	"VIEW_AGENT_DETAILS",

	// Builds
	"RUN_BUILD",
	"CANCEL_BUILD",
	"TAG_BUILD",
	"COMMENT_BUILD",
	"PIN_UNPIN_BUILD",
	"REORDER_BUILD_QUEUE",
	"PATCH_BUILD_SOURCES",

	// Jobs
	"PAUSE_ACTIVATE_BUILD_CONFIGURATION",

	// Projects (EDIT_PROJECT also covers build configuration editing)
	"EDIT_PROJECT",

	// Agents
	"ENABLE_DISABLE_AGENT",
	"AUTHORIZE_AGENT",
	"ADMINISTER_AGENT",
	"CONNECT_TO_AGENT",

	// Pools
	"MANAGE_AGENT_POOLS",
}

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ValidUntil  string `json:"valid_until"`
}

type CallbackResult struct {
	Code  string
	State string
	Error string
}

type CallbackServer struct {
	Port       int
	ResultChan chan CallbackResult
	server     *http.Server
	listener   net.Listener
}

func GenerateCodeVerifier() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate random bytes: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func GenerateCodeChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

func GenerateState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate random bytes: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func BuildAuthorizeURL(serverURL, redirectURI, challenge, state string, scopes []string) string {
	params := url.Values{}
	params.Set("response_type", "code")
	params.Set("redirect_uri", redirectURI)
	params.Set("code_challenge", challenge)
	params.Set("code_challenge_method", CodeChallengeMethod)
	params.Set("state", state)
	params.Set("scope", strings.Join(scopes, " "))
	return strings.TrimSuffix(serverURL, "/") + PkceAuthorizePath + "?" + params.Encode()
}

func FindAvailableListener() (net.Listener, int, error) {
	for port := CallbackPortMin; port <= CallbackPortMax; port++ {
		if l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port)); err == nil {
			return l, port, nil
		}
	}
	return nil, 0, fmt.Errorf("no available port in range %d-%d", CallbackPortMin, CallbackPortMax)
}

func IsPkceEnabled(ctx context.Context, serverURL string) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", strings.TrimSuffix(serverURL, "/")+PkceIsEnabledPath, nil)
	if err != nil {
		return false, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("check PKCE status: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode == http.StatusOK, nil
}

func NewCallbackServer(listener net.Listener, port int) *CallbackServer {
	return &CallbackServer{
		Port:       port,
		ResultChan: make(chan CallbackResult, 1),
		listener:   listener,
	}
}

func (cs *CallbackServer) Start() {
	mux := http.NewServeMux()
	mux.HandleFunc(DefaultCallbackPath, cs.handleCallback)
	cs.server = &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = cs.server.Serve(cs.listener) }()
}

func (cs *CallbackServer) handleCallback(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	result := CallbackResult{Code: q.Get("code"), State: q.Get("state"), Error: q.Get("error")}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'unsafe-inline'; style-src 'unsafe-inline'")

	if result.Error != "" {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = fmt.Fprintf(w, `<!DOCTYPE html><html><head><title>TeamCity CLI</title>
<style>body{font-family:system-ui,sans-serif;text-align:center;padding:50px}</style></head><body>
<h1 style="color:#ef4444">✗ Authentication failed</h1><p>Error: %s</p>
<p>Please return to the terminal.</p></body></html>`, html.EscapeString(result.Error))
	} else {
		_, _ = fmt.Fprint(w, `<!DOCTYPE html><html><head><title>TeamCity CLI</title>
<style>body{font-family:system-ui,sans-serif;text-align:center;padding:50px}</style></head><body>
<h1 style="color:#22c55e">✓ Authentication successful!</h1>
<p>You can close this window and return to the terminal.</p>
<script>setTimeout(function(){window.close()},2000)</script></body></html>`)
	}

	select {
	case cs.ResultChan <- result:
	default:
	}
}

func (cs *CallbackServer) Shutdown() {
	if cs.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = cs.server.Shutdown(ctx)
	}
}

func DefaultScopes() []string {
	return append([]string{}, AvailableScopes...)
}

func ExchangeCodeForToken(ctx context.Context, serverURL, code, verifier, redirectURI string) (*TokenResponse, error) {
	data := url.Values{}
	data.Set("code", code)
	data.Set("code_verifier", verifier)
	data.Set("redirect_uri", redirectURI)

	req, err := http.NewRequestWithContext(ctx, "POST", strings.TrimSuffix(serverURL, "/")+PkceTokenPath, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange failed (status %d): %s", resp.StatusCode, body)
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}
	return &tokenResp, nil
}
