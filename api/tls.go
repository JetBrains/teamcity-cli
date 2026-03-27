package api

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"sync"
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

// defaultTransport clones http.DefaultTransport with PEM-based root CAs to bypass platform verifiers that sandboxes block.
var defaultTransport = sync.OnceValue(func() *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	if pool := loadRootCAs(); pool != nil {
		t.TLSClientConfig = &tls.Config{RootCAs: pool}
	}
	return t
})

// loadRootCAs loads root CAs from well-known PEM bundle files (nil if none found).
func loadRootCAs() *x509.CertPool {
	for _, path := range certBundlePaths[runtime.GOOS] {
		if data, err := os.ReadFile(path); err == nil {
			pool := x509.NewCertPool()
			if pool.AppendCertsFromPEM(data) {
				return pool
			}
		}
	}
	return nil
}

var certBundlePaths = map[string][]string{
	"darwin": {"/etc/ssl/cert.pem"},
	"linux":  {"/etc/ssl/certs/ca-certificates.crt", "/etc/pki/tls/certs/ca-bundle.crt", "/etc/ssl/cert.pem"},
}
