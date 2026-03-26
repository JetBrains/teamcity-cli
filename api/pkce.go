package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"
)

//go:embed templates/pkce_callback.html
var callbackPageHTML string
var callbackPageTmpl = template.Must(template.New("callback").Parse(callbackPageHTML))

const (
	PkceIsEnabledPath   = "/pkce/is_enabled.html"
	PkceAuthorizePath   = "/pkce/authorize.html"
	PkceTokenPath       = "/pkce/token.html"
	PkceClientID        = "teamcity-cli"
	CodeChallengeMethod = "S256"
	DefaultCallbackPath = "/callback"
	maxResponseBody     = 64 * 1024
)

var fallbackScopes = []string{
	"VIEW_PROJECT",
	"VIEW_BUILD_CONFIGURATION_SETTINGS",
	"VIEW_AGENT_DETAILS",
	"RUN_BUILD",
	"CANCEL_BUILD",
	"TAG_BUILD",
	"COMMENT_BUILD",
	"PIN_UNPIN_BUILD",
	"REORDER_BUILD_QUEUE",
	"PATCH_BUILD_SOURCES",
	"PAUSE_ACTIVATE_BUILD_CONFIGURATION",
	"EDIT_PROJECT",
	"ENABLE_DISABLE_AGENT",
	"AUTHORIZE_AGENT",
	"ADMINISTER_AGENT",
	"CONNECT_TO_AGENT",
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

func GenerateCodeVerifier() (string, error) { return randomBase64URL(32) }

func GenerateState() (string, error) { return randomBase64URL(16) }

func GenerateCodeChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

func randomBase64URL(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate random bytes: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func BuildAuthorizeURL(serverURL, redirectURI, challenge, state string, scopes []string) string {
	params := url.Values{}
	params.Set("client_id", PkceClientID)
	params.Set("response_type", "code")
	params.Set("redirect_uri", redirectURI)
	params.Set("code_challenge", challenge)
	params.Set("code_challenge_method", CodeChallengeMethod)
	params.Set("state", state)
	params.Set("scope", strings.Join(scopes, " "))
	return strings.TrimSuffix(serverURL, "/") + PkceAuthorizePath + "?" + params.Encode()
}

func FindAvailableListener() (net.Listener, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("listen on loopback: %w", err)
	}
	return l, nil
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

func NewCallbackServer(listener net.Listener) *CallbackServer {
	return &CallbackServer{
		Port:       listener.Addr().(*net.TCPAddr).Port,
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
	}
	_ = callbackPageTmpl.Execute(w, result)

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
	return slices.Clone(fallbackScopes)
}

func ExchangeCodeForToken(ctx context.Context, serverURL, code, verifier, redirectURI string) (*TokenResponse, error) {
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("client_id", PkceClientID)
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
