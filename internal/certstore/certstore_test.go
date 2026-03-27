package certstore

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestThumbprint(t *testing.T) {
	t.Parallel()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "Test"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)

	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)

	tp := Thumbprint(cert)
	assert.Len(t, tp, 40)
	assert.Regexp(t, `^[0-9A-F]{40}$`, tp)
	assert.Equal(t, tp, Thumbprint(cert))
}

func TestNormalizeThumbprint(t *testing.T) {
	t.Parallel()

	tests := []struct{ in, want string }{
		{"aa:bb:cc", "AABBCC"},
		{"AA BB CC", "AABBCC"},
		{"aaBBcc", "AABBCC"},
		{"", ""},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, NormalizeThumbprint(tt.in))
	}
}

func TestLoadIdentityNotFound(t *testing.T) {
	t.Parallel()
	_, err := LoadIdentity("0000000000000000000000000000000000000000")
	assert.Error(t, err)
}
