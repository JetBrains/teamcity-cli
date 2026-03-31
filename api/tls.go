package api

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
)

// defaultTransport returns a transport with PEM fallback when the platform TLS verifier is blocked.
var defaultTransport = sync.OnceValue(func() http.RoundTripper {
	platform := http.DefaultTransport.(*http.Transport).Clone()
	pool := loadRootCAs()
	if pool == nil {
		return platform
	}
	pem := http.DefaultTransport.(*http.Transport).Clone()
	pem.TLSClientConfig = &tls.Config{RootCAs: pool}
	return &pemFallbackTransport{platform: platform, pem: pem}
})

// pemFallbackTransport tries the platform verifier first, switching permanently to PEM on TLS errors.
type pemFallbackTransport struct {
	platform http.RoundTripper
	pem      http.RoundTripper
	usePEM   atomic.Bool
}

func (t *pemFallbackTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.usePEM.Load() {
		return t.pem.RoundTrip(req)
	}
	resp, err := t.platform.RoundTrip(req)
	if err != nil && isPlatformTLSError(err) {
		t.usePEM.Store(true)
		return t.pem.RoundTrip(req)
	}
	return resp, err
}

// isPlatformTLSError reports whether err is a TLS error where the PEM fallback may help.
func isPlatformTLSError(err error) bool {
	if err == nil {
		return false
	}
	if _, ok := errors.AsType[x509.UnknownAuthorityError](err); ok {
		return true
	}
	return strings.Contains(err.Error(), "OSStatus")
}

// loadRootCAs loads root CAs from system PEM bundles (nil if none found).
func loadRootCAs() *x509.CertPool {
	var pool *x509.CertPool
	for _, path := range certBundlePaths[runtime.GOOS] {
		if data, err := os.ReadFile(path); err == nil {
			if pool == nil {
				pool = x509.NewCertPool()
			}
			pool.AppendCertsFromPEM(data)
		}
	}
	return pool
}

var certBundlePaths = map[string][]string{
	"darwin": {"/etc/ssl/cert.pem"},
	"linux":  {"/etc/ssl/certs/ca-certificates.crt", "/etc/pki/tls/certs/ca-bundle.crt", "/etc/ssl/cert.pem"},
}
