package auth

import (
	"context"
	"errors"
	"net/url"
	"strings"

	"github.com/JetBrains/teamcity-cli/api"
)

// friendlyError maps low-level probe and HTTP errors to short, actionable messages.
func friendlyError(err error, serverURL string) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "connection timed out"
	}
	if errors.Is(err, api.ErrLoginGatewayDetected) {
		if u, perr := url.Parse(serverURL); perr == nil && strings.HasSuffix(strings.ToLower(u.Hostname()), ".labs.intellij.net") {
			return "login gateway detected — https://jb.gg/warp may be required"
		}
		return err.Error()
	}
	s := err.Error()
	switch {
	case strings.Contains(s, "no such host"):
		return "DNS lookup failed — check the hostname"
	case strings.Contains(s, "connection refused"):
		return "connection refused — is the server running?"
	case strings.Contains(s, "i/o timeout"):
		return "connection timed out"
	case strings.Contains(s, "x509"), strings.Contains(s, "certificate"):
		return "TLS certificate error — check the URL scheme and hostname"
	case strings.Contains(s, "401"), strings.Contains(strings.ToLower(s), "unauthorized"):
		return "token rejected — the token is invalid, expired, or lacks required permissions"
	}
	return s
}
