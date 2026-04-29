package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
)

const (
	PkceIsEnabledPath   = "/pkce/is_enabled.html"
	PkceAuthorizePath   = "/pkce/authorize.html"
	PkceTokenPath       = "/pkce/token.html"
	PkceClientID        = "teamcity-cli"
	CodeChallengeMethod = "S256"
	maxResponseBody     = 64 * 1024
)

// fallbackScopes is ordered from most-useful for a typical CLI user (read + build actions) down to admin-only
// scopes, so the scope picker UI defaults to the most-commonly-needed permissions near the top.
var fallbackScopes = []string{
	// Read
	"VIEW_PROJECT",
	"VIEW_BUILD_CONFIGURATION_SETTINGS",
	"VIEW_AGENT_DETAILS",

	// Build actions (daily developer workflow)
	"RUN_BUILD",
	"CANCEL_BUILD",
	"TAG_BUILD",
	"COMMENT_BUILD",
	"PIN_UNPIN_BUILD",
	"PATCH_BUILD_SOURCES",
	"REORDER_BUILD_QUEUE",

	// Project administration
	"PAUSE_ACTIVATE_BUILD_CONFIGURATION",
	"EDIT_PROJECT",
	"CREATE_SUB_PROJECT",
	"CREATE_DELETE_VCS_ROOT",

	// Agent administration
	"CONNECT_TO_AGENT",
	"ENABLE_DISABLE_AGENT",
	"AUTHORIZE_AGENT",
	"ADMINISTER_AGENT",
	"MANAGE_AGENT_POOLS",
}

// TokenResponse is TeamCity's reply to a successful PKCE token exchange.
type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ValidUntil  string `json:"valid_until"`
}

// BuildAuthorizeURL builds the TeamCity PKCE authorize URL with the given parameters.
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

// IsPkceEnabled reports whether the server at c.BaseURL advertises PKCE support.
func (c *Client) IsPkceEnabled(ctx context.Context) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimSuffix(c.BaseURL, "/")+PkceIsEnabledPath, nil)
	if err != nil {
		return false, err
	}
	c.applyStandardHeaders(req)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("check PKCE status: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode == http.StatusOK, nil
}

// DefaultScopes returns the curated default scope list for a CLI PKCE login.
func DefaultScopes() []string {
	return slices.Clone(fallbackScopes)
}

// ExchangeCodeForToken trades a PKCE authorization code for an access token at c.BaseURL.
func (c *Client) ExchangeCodeForToken(ctx context.Context, code, verifier, redirectURI string) (*TokenResponse, error) {
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("client_id", PkceClientID)
	data.Set("code", code)
	data.Set("code_verifier", verifier)
	data.Set("redirect_uri", redirectURI)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimSuffix(c.BaseURL, "/")+PkceTokenPath, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c.applyStandardHeaders(req)

	resp, err := c.HTTPClient.Do(req)
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
