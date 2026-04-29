package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"slices"
	"time"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/browserflow"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/JetBrains/teamcity-cli/internal/version"
	"github.com/charmbracelet/huh"
)

const authCodeLifetime = 5 * time.Minute

// attemptPkceLogin probes for PKCE support, lets the user pick which scopes to grant, then runs the browser flow.
func attemptPkceLogin(ctx context.Context, p *output.Printer, serverURL string) (token, validUntil string) {
	client := api.NewGuestClient(serverURL, api.WithVersion(version.String()))
	pctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	enabled, _ := client.IsPkceEnabled(pctx)
	cancel()
	if !enabled {
		return "", ""
	}
	scopes := selectPkceScopes()
	if len(scopes) == 0 {
		p.Info("Skipping browser login, entering token manually...")
		return "", ""
	}
	resp, err := runPkceLogin(ctx, p, client, scopes)
	if err != nil {
		p.Warn("Browser auth failed: %v", err)
		p.Info("Falling back to manual token entry...")
		return "", ""
	}
	return resp.AccessToken, resp.ValidUntil
}

// describeScope returns the server description of scope with the raw scope name appended faintly for auditability.
func describeScope(scope string) string {
	if desc, ok := api.KnownPermissions[scope]; ok {
		return desc + " " + output.Faint("("+scope+")")
	}
	return scope
}

// selectPkceScopes lets the user review and optionally trim the scopes the CLI will request; returns nil if canceled.
func selectPkceScopes() []string {
	all := api.DefaultScopes()
	selected := slices.Clone(all)

	options := make([]huh.Option[string], len(all))
	for i, s := range all {
		options[i] = huh.NewOption(describeScope(s), s).Selected(true)
	}

	if err := cmdutil.Prompt(huh.NewMultiSelect[string]().
		Title("Select permissions to request").
		Description(fmt.Sprintf("%d total · your server role limits the final permission set", len(all))).
		Options(options...).
		Value(&selected).
		Height(7).
		Validate(func(picked []string) error {
			if len(picked) == 0 {
				return errors.New("select at least one permission")
			}
			return nil
		})); err != nil {
		return nil
	}
	return selected
}

// runPkceLogin orchestrates the browser-based PKCE auth flow with the given scopes and returns the minted access token.
func runPkceLogin(parent context.Context, p *output.Printer, client *api.Client, scopes []string) (*api.TokenResponse, error) {
	verifier, err := generatePkceVerifier()
	if err != nil {
		return nil, fmt.Errorf("generate code verifier: %w", err)
	}
	state, err := browserflow.GenerateState()
	if err != nil {
		return nil, fmt.Errorf("generate state: %w", err)
	}

	listener, err := browserflow.FindAvailableListener()
	if err != nil {
		return nil, fmt.Errorf("find available port: %w", err)
	}

	port := listener.Addr().(*net.TCPAddr).Port
	redirectURI := fmt.Sprintf("http://localhost:%d%s", port, browserflow.DefaultCallbackPath)
	authURL := api.BuildAuthorizeURL(client.BaseURL, redirectURI, pkceCodeChallenge(verifier), state, scopes)

	opening := fmt.Sprintf("Opening browser to authenticate with %d permissions...", len(scopes))
	if total := len(api.DefaultScopes()); len(scopes) < total {
		opening = fmt.Sprintf("Opening browser to authenticate with %d of %d permissions...", len(scopes), total)
	}
	p.Info("%s", opening)
	_, _ = fmt.Fprintf(p.Out, "  %s Approve access in TeamCity\n", output.Yellow("→"))

	result, err := browserflow.Run(parent, browserflow.Options{
		Listener:     listener,
		State:        state,
		OpenURL:      authURL,
		CallbackPath: browserflow.DefaultCallbackPath,
		Timeout:      authCodeLifetime,
		Logger:       p,
	})
	if err != nil {
		return nil, err
	}

	_, _ = fmt.Fprintln(p.Out)
	exchangeCtx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	return client.ExchangeCodeForToken(exchangeCtx, result.Code, verifier, redirectURI)
}

// generatePkceVerifier returns 32 random bytes encoded as base64url, satisfying RFC 7636 §4.1.
func generatePkceVerifier() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate random bytes: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// pkceCodeChallenge returns the SHA256 base64url challenge for a PKCE verifier per RFC 7636 §4.2.
func pkceCodeChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}
