//go:build darwin

package certstore

import (
	"crypto/tls"
	"fmt"

	"lds.li/keychain"
)

// LoadIdentity loads a TLS certificate from the macOS Keychain by thumbprint.
// The private key never leaves the Keychain — signing is delegated to Security.framework via purego.
func LoadIdentity(thumbprint string) (tls.Certificate, error) {
	thumbprint = NormalizeThumbprint(thumbprint)

	ids, err := keychain.ListIdentities(keychain.IdentityQuery{})
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("list keychain identities: %w", err)
	}

	for _, id := range ids {
		cert, err := id.Certificate()
		if err != nil {
			continue
		}
		if Thumbprint(cert) != thumbprint {
			continue
		}

		signer, err := id.Signer()
		if err != nil {
			return tls.Certificate{}, fmt.Errorf("get signer from keychain: %w", err)
		}

		return MakeTLSCert(cert, signer), nil
	}

	return tls.Certificate{}, ErrNotFound
}
