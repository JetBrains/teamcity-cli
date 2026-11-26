package httpclient

import (
	"crypto/tls"
	"net/http"
	"net/url"
	"strings"
)

// InsecureTransport skips certificate verification only for the specified server origin.
func InsecureTransport(serverURL string, verified http.RoundTripper) http.RoundTripper {
	origin, _ := url.Parse(serverURL)
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} // Explicit opt-in for an untrusted TeamCity server.
	return &originTLSTransport{origin: origin, verified: verified, insecure: transport}
}

type originTLSTransport struct {
	origin   *url.URL
	verified http.RoundTripper
	insecure http.RoundTripper
}

func (t *originTLSTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.origin != nil && req.URL.Scheme == t.origin.Scheme && strings.EqualFold(req.URL.Host, t.origin.Host) {
		return t.insecure.RoundTrip(req)
	}
	return t.verified.RoundTrip(req)
}
