package api

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
)

// TLSConfig builds a tls.Config for mTLS client certificate authentication.
func TLSConfig(certFile, keyFile, caFile string) (*tls.Config, error) {
	tlsCfg := &tls.Config{}

	if certFile != "" && keyFile != "" {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, fmt.Errorf("load client certificate: %w", err)
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}

	if caFile != "" {
		caCert, err := os.ReadFile(caFile)
		if err != nil {
			return nil, fmt.Errorf("read CA certificate: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("parse CA certificate: no valid certificates found in %s", caFile)
		}
		tlsCfg.RootCAs = pool
	}

	return tlsCfg, nil
}

// TLSConfigWithCert creates a tls.Config with the given certificate (private key may be keystore-backed).
func TLSConfigWithCert(cert tls.Certificate, caFile string) (*tls.Config, error) {
	tlsCfg := &tls.Config{
		GetClientCertificate: func(_ *tls.CertificateRequestInfo) (*tls.Certificate, error) {
			return &cert, nil
		},
	}
	if caFile != "" {
		caCertPEM, err := os.ReadFile(caFile)
		if err != nil {
			return nil, fmt.Errorf("read CA certificate: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caCertPEM) {
			return nil, fmt.Errorf("parse CA certificate: no valid certificates found in %s", caFile)
		}
		tlsCfg.RootCAs = pool
	}
	return tlsCfg, nil
}

// WithTransport sets a custom http.Transport on the client.
func WithTransport(transport *http.Transport) ClientOption {
	return func(c *Client) {
		c.HTTPClient.Transport = transport
	}
}

// defaultTransport returns a transport that uses the platform TLS verifier by default, auto-switching to PEM-based verification when the platform verifier is blocked (e.g. sandbox).
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

// pemFallbackTransport uses the platform verifier until a macOS Security.framework error is seen, then permanently switches to PEM-based verification.
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
	if err != nil && strings.Contains(err.Error(), "OSStatus") {
		t.usePEM.Store(true)
		return t.pem.RoundTrip(req)
	}
	return resp, err
}

// loadRootCAs loads root CAs from PEM bundles and TEAMCITY_CA_CERT (nil if none found).
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
	if caFile := os.Getenv("TEAMCITY_CA_CERT"); caFile != "" {
		if data, err := os.ReadFile(caFile); err == nil {
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
