package api

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
)

// TLSConfig builds a tls.Config for mutual TLS client certificate authentication.
// certFile and keyFile are paths to PEM-encoded client certificate and key.
// caFile is an optional path to a PEM-encoded CA certificate for server verification.
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

// WithTransport sets a custom http.Transport on the client.
func WithTransport(transport *http.Transport) ClientOption {
	return func(c *Client) {
		c.HTTPClient.Transport = transport
	}
}
