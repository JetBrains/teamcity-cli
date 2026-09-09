// Package httpclient shares HTTP security policies between API and terminal clients.
package httpclient

import (
	"errors"
	"net/http"
	"strings"
)

// RedirectPolicy rejects downgrades and optionally allows credential-free cross-origin downloads.
func RedirectPolicy(allowCrossOrigin bool) func(*http.Request, []*http.Request) error {
	return func(request *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		if via[len(via)-1].URL.Scheme == "https" && request.URL.Scheme != "https" {
			return errors.New("refusing HTTPS downgrade redirect")
		}
		original := via[0].URL
		if request.URL.Scheme == original.Scheme && strings.EqualFold(request.URL.Host, original.Host) {
			return nil
		}
		if !allowCrossOrigin || (request.Method != http.MethodGet && request.Method != http.MethodHead) || request.Body != nil {
			return errors.New("refusing cross-origin redirect")
		}
		request.Header = make(http.Header)
		return nil
	}
}
