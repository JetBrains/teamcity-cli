package certstore

import (
	"crypto/sha1"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"strings"
)

var (
	ErrNotSupported = errors.New("OS certificate store is not supported on this platform; use client_cert and client_key with PEM files instead")
	ErrNotFound     = errors.New("no certificate found matching the given thumbprint")
)

// Thumbprint returns the uppercase hex SHA-1 fingerprint of a DER-encoded certificate.
func Thumbprint(cert *x509.Certificate) string {
	h := sha1.Sum(cert.Raw)
	return strings.ToUpper(hex.EncodeToString(h[:]))
}

// NormalizeThumbprint strips colons/spaces and uppercases.
func NormalizeThumbprint(s string) string {
	s = strings.ReplaceAll(s, ":", "")
	s = strings.ReplaceAll(s, " ", "")
	return strings.ToUpper(s)
}

// MakeTLSCert wraps a parsed certificate and OS-backed signer into a tls.Certificate.
func MakeTLSCert(cert *x509.Certificate, key any) tls.Certificate {
	return tls.Certificate{
		Certificate: [][]byte{cert.Raw},
		PrivateKey:  key,
		Leaf:        cert,
	}
}
