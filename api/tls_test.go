package api

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// generateTestCerts creates a self-signed CA and a client certificate signed by it.
// Returns paths to PEM files written into t.TempDir().
func generateTestCerts(t *testing.T) (caCertPath, clientCertPath, clientKeyPath string) {
	t.Helper()
	dir := t.TempDir()

	// CA key and cert
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Test CA"},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	caCertDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	require.NoError(t, err)

	caCert, err := x509.ParseCertificate(caCertDER)
	require.NoError(t, err)

	caCertPath = filepath.Join(dir, "ca.crt")
	writePEM(t, caCertPath, "CERTIFICATE", caCertDER)

	// Client key and cert
	clientKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	clientTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "Test Client"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	clientCertDER, err := x509.CreateCertificate(rand.Reader, clientTemplate, caCert, &clientKey.PublicKey, caKey)
	require.NoError(t, err)

	clientCertPath = filepath.Join(dir, "client.crt")
	writePEM(t, clientCertPath, "CERTIFICATE", clientCertDER)

	clientKeyDER, err := x509.MarshalECPrivateKey(clientKey)
	require.NoError(t, err)
	clientKeyPath = filepath.Join(dir, "client.key")
	writePEM(t, clientKeyPath, "EC PRIVATE KEY", clientKeyDER)

	return caCertPath, clientCertPath, clientKeyPath
}

func writePEM(t *testing.T, path, blockType string, data []byte) {
	t.Helper()
	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()
	require.NoError(t, pem.Encode(f, &pem.Block{Type: blockType, Bytes: data}))
}

func TestTLSConfig(t *testing.T) {
	t.Parallel()

	caCertPath, clientCertPath, clientKeyPath := generateTestCerts(t)

	t.Run("client cert and CA", func(t *testing.T) {
		t.Parallel()
		cfg, err := TLSConfig(clientCertPath, clientKeyPath, caCertPath)
		require.NoError(t, err)
		assert.Len(t, cfg.Certificates, 1)
		assert.NotNil(t, cfg.RootCAs)
	})

	t.Run("CA only", func(t *testing.T) {
		t.Parallel()
		cfg, err := TLSConfig("", "", caCertPath)
		require.NoError(t, err)
		assert.Empty(t, cfg.Certificates)
		assert.NotNil(t, cfg.RootCAs)
	})

	t.Run("client cert only", func(t *testing.T) {
		t.Parallel()
		cfg, err := TLSConfig(clientCertPath, clientKeyPath, "")
		require.NoError(t, err)
		assert.Len(t, cfg.Certificates, 1)
		assert.Nil(t, cfg.RootCAs)
	})

	t.Run("missing cert file", func(t *testing.T) {
		t.Parallel()
		_, err := TLSConfig("/nonexistent/cert.pem", clientKeyPath, "")
		assert.ErrorContains(t, err, "load client certificate")
	})

	t.Run("missing key file", func(t *testing.T) {
		t.Parallel()
		_, err := TLSConfig(clientCertPath, "/nonexistent/key.pem", "")
		assert.ErrorContains(t, err, "load client certificate")
	})

	t.Run("missing CA file", func(t *testing.T) {
		t.Parallel()
		_, err := TLSConfig("", "", "/nonexistent/ca.pem")
		assert.ErrorContains(t, err, "read CA certificate")
	})

	t.Run("invalid CA content", func(t *testing.T) {
		t.Parallel()
		badCA := filepath.Join(t.TempDir(), "bad-ca.pem")
		require.NoError(t, os.WriteFile(badCA, []byte("not a cert"), 0o600))
		_, err := TLSConfig("", "", badCA)
		assert.ErrorContains(t, err, "parse CA certificate")
	})
}

func TestMTLSHandshake(t *testing.T) {
	t.Parallel()

	caCertPath, clientCertPath, clientKeyPath := generateTestCerts(t)

	// Load CA cert for server-side verification
	caCertPEM, err := os.ReadFile(caCertPath)
	require.NoError(t, err)
	caPool := x509.NewCertPool()
	require.True(t, caPool.AppendCertsFromPEM(caCertPEM))

	// Server that requires client certs
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"version":"2024.12 (build 166579)","versionMajor":2024,"versionMinor":12,"buildNumber":"166579"}`))
	}))
	server.TLS = &tls.Config{
		ClientAuth: tls.RequireAndVerifyClientCert,
		ClientCAs:  caPool,
	}
	server.StartTLS()
	t.Cleanup(server.Close)

	t.Run("with client cert succeeds", func(t *testing.T) {
		t.Parallel()
		tlsCfg, err := TLSConfig(clientCertPath, clientKeyPath, caCertPath)
		require.NoError(t, err)
		// Trust the test server's self-signed cert
		tlsCfg.InsecureSkipVerify = true

		client := NewClient(server.URL, "test-token",
			WithTransport(&http.Transport{TLSClientConfig: tlsCfg}),
		)
		srv, err := client.GetServer()
		require.NoError(t, err)
		assert.Equal(t, 2024, srv.VersionMajor)
	})

	t.Run("without client cert fails", func(t *testing.T) {
		t.Parallel()
		client := NewClient(server.URL, "test-token",
			WithTransport(&http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			}),
		)
		_, err := client.GetServer()
		assert.Error(t, err)
	})
}
