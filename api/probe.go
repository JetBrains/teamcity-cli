package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const probeTimeout = 10 * time.Second

// ErrLoginGatewayDetected indicates the API endpoint was served by an SSO / login gateway rather than TeamCity itself.
var ErrLoginGatewayDetected = errors.New("login gateway detected — VPN may be required")

// ProbeTeamCity checks whether serverURL points to a reachable TeamCity server without sending credentials.
// Pass ClientOption values (e.g. WithExtraHeaders) to include additional headers in the probe request.
func ProbeTeamCity(ctx context.Context, serverURL string, opts ...ClientOption) error {
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	u := strings.TrimSuffix(serverURL, "/") + "/app/rest/server/version"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	applyExtraHeaders(req, opts)
	client := &http.Client{Transport: defaultTransport()}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	// An HTML body on the API endpoint means a login gateway is intercepting the request.
	if ct := strings.ToLower(resp.Header.Get("Content-Type")); strings.Contains(ct, "text/html") {
		return ErrLoginGatewayDetected
	}
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300,
		resp.StatusCode == http.StatusUnauthorized,
		resp.StatusCode == http.StatusForbidden:
		return nil
	case resp.StatusCode == http.StatusNotFound:
		return errors.New("not a TeamCity server (endpoint not found)")
	default:
		return fmt.Errorf("unexpected response HTTP %d", resp.StatusCode)
	}
}
