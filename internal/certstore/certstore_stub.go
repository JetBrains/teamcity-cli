//go:build !darwin && !windows

package certstore

import "crypto/tls"

// LoadIdentity is not supported on this platform.
func LoadIdentity(_ string) (tls.Certificate, error) {
	return tls.Certificate{}, ErrNotSupported
}
