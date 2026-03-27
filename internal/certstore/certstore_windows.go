//go:build windows

package certstore

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"math/big"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	crypt32  = windows.MustLoadDLL("crypt32.dll")
	nCryptDL = windows.MustLoadDLL("ncrypt.dll")

	certOpenSystemStoreW              = crypt32.MustFindProc("CertOpenSystemStoreW")
	certCloseStore                    = crypt32.MustFindProc("CertCloseStore")
	certFindCertificateInStore        = crypt32.MustFindProc("CertFindCertificateInStore")
	cryptAcquireCertificatePrivateKey = crypt32.MustFindProc("CryptAcquireCertificatePrivateKey")

	nCryptSignHash = nCryptDL.MustFindProc("NCryptSignHash")
)

const (
	certFindHash                  = 0x10000 // CERT_FIND_HASH
	x509ASNEncoding               = 0x1
	pkcs7ASNEncoding              = 0x10000
	encodingFlags                 = x509ASNEncoding | pkcs7ASNEncoding
	cryptAcquireOnlyNCryptKeyFlag = 0x40000
	bcryptPadPKCS1                = 0x2
	bcryptPadPSS                  = 0x8
)

type cryptHashBlob struct {
	size uint32
	data *byte
}

// LoadIdentity loads a TLS certificate from the Windows certificate store by SHA-1 thumbprint.
// The private key never leaves the OS — signing is delegated to NCrypt.
func LoadIdentity(thumbprint string) (tls.Certificate, error) {
	thumbprint = NormalizeThumbprint(thumbprint)
	hashBytes, err := hexToBytes(thumbprint)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("invalid thumbprint: %w", err)
	}

	store, err := openMyStore()
	if err != nil {
		return tls.Certificate{}, err
	}
	defer closeStore(store)

	blob := cryptHashBlob{
		size: uint32(len(hashBytes)),
		data: &hashBytes[0],
	}

	r, _, callErr := certFindCertificateInStore.Call(
		uintptr(store), encodingFlags, 0, certFindHash,
		uintptr(unsafe.Pointer(&blob)), 0,
	)
	if r == 0 {
		return tls.Certificate{}, fmt.Errorf("%w: %v", ErrNotFound, callErr)
	}
	ctx := (*windows.CertContext)(unsafe.Pointer(r))

	cert, err := certContextToX509(ctx)
	if err != nil {
		return tls.Certificate{}, err
	}

	keyHandle, err := acquireKey(ctx)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("acquire private key: %w", err)
	}

	signer := &ncryptSigner{handle: keyHandle, pub: cert.PublicKey}
	return MakeTLSCert(cert, signer), nil
}

func openMyStore() (windows.Handle, error) {
	myStore, err := windows.UTF16PtrFromString("MY")
	if err != nil {
		return 0, err
	}
	r, _, callErr := certOpenSystemStoreW.Call(0, uintptr(unsafe.Pointer(myStore)))
	if r == 0 {
		return 0, fmt.Errorf("open certificate store: %v", callErr)
	}
	return windows.Handle(r), nil
}

func closeStore(store windows.Handle) {
	certCloseStore.Call(uintptr(store), 0)
}

func certContextToX509(ctx *windows.CertContext) (*x509.Certificate, error) {
	der := unsafe.Slice(ctx.EncodedCert, ctx.Length)
	return x509.ParseCertificate(der)
}

func acquireKey(ctx *windows.CertContext) (uintptr, error) {
	var handle uintptr
	var spec uint32
	var mustFree int32
	r, _, callErr := cryptAcquireCertificatePrivateKey.Call(
		uintptr(unsafe.Pointer(ctx)),
		cryptAcquireOnlyNCryptKeyFlag,
		0,
		uintptr(unsafe.Pointer(&handle)),
		uintptr(unsafe.Pointer(&spec)),
		uintptr(unsafe.Pointer(&mustFree)),
	)
	if r == 0 {
		return 0, fmt.Errorf("CryptAcquireCertificatePrivateKey: %v", callErr)
	}
	return handle, nil
}

// ncryptSigner implements crypto.Signer using Windows NCrypt.
type ncryptSigner struct {
	handle uintptr
	pub    crypto.PublicKey
}

func (s *ncryptSigner) Public() crypto.PublicKey { return s.pub }

func (s *ncryptSigner) Sign(_ io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	var paddingInfo unsafe.Pointer
	var flags uint32

	switch s.pub.(type) {
	case *rsa.PublicKey:
		algo, err := hashAlgoName(opts.HashFunc())
		if err != nil {
			return nil, err
		}
		if _, ok := opts.(*rsa.PSSOptions); ok {
			pss := &bcryptPSSPaddingInfo{algorithm: algo, saltLength: uint32(opts.HashFunc().Size())}
			paddingInfo = unsafe.Pointer(pss)
			flags = bcryptPadPSS
		} else {
			pkcs1 := &bcryptPKCS1PaddingInfo{algorithm: algo}
			paddingInfo = unsafe.Pointer(pkcs1)
			flags = bcryptPadPKCS1
		}
	case *ecdsa.PublicKey:
		// ECDSA: no padding info needed
	default:
		return nil, fmt.Errorf("unsupported key type %T", s.pub)
	}

	// First call to get signature length
	var sigLen uint32
	r, _, callErr := nCryptSignHash.Call(
		s.handle, uintptr(paddingInfo),
		uintptr(unsafe.Pointer(&digest[0])), uintptr(len(digest)),
		0, 0,
		uintptr(unsafe.Pointer(&sigLen)), uintptr(flags),
	)
	if r != 0 {
		return nil, fmt.Errorf("NCryptSignHash (size): %v", callErr)
	}

	sig := make([]byte, sigLen)
	r, _, callErr = nCryptSignHash.Call(
		s.handle, uintptr(paddingInfo),
		uintptr(unsafe.Pointer(&digest[0])), uintptr(len(digest)),
		uintptr(unsafe.Pointer(&sig[0])), uintptr(sigLen),
		uintptr(unsafe.Pointer(&sigLen)), uintptr(flags),
	)
	if r != 0 {
		return nil, fmt.Errorf("NCryptSignHash: %v", callErr)
	}

	// For ECDSA, NCrypt returns r||s as raw big-endian integers; convert to ASN.1
	if _, ok := s.pub.(*ecdsa.PublicKey); ok {
		return ecdsaRawToASN1(sig[:sigLen])
	}
	return sig[:sigLen], nil
}

type bcryptPKCS1PaddingInfo struct {
	algorithm *uint16
}

type bcryptPSSPaddingInfo struct {
	algorithm  *uint16
	saltLength uint32
}

func hashAlgoName(h crypto.Hash) (*uint16, error) {
	switch h {
	case crypto.SHA1:
		return windows.StringToUTF16Ptr("SHA1"), nil
	case crypto.SHA256:
		return windows.StringToUTF16Ptr("SHA256"), nil
	case crypto.SHA384:
		return windows.StringToUTF16Ptr("SHA384"), nil
	case crypto.SHA512:
		return windows.StringToUTF16Ptr("SHA512"), nil
	default:
		return nil, fmt.Errorf("unsupported hash: %v", h)
	}
}

func ecdsaRawToASN1(raw []byte) ([]byte, error) {
	if len(raw)%2 != 0 {
		return nil, fmt.Errorf("invalid ECDSA signature length %d", len(raw))
	}
	half := len(raw) / 2
	r := new(big.Int).SetBytes(raw[:half])
	s := new(big.Int).SetBytes(raw[half:])

	// Determine curve order size for proper encoding
	return encodeECDSASig(r, s), nil
}

func encodeECDSASig(r, s *big.Int) []byte {
	rb := asn1Integer(r)
	sb := asn1Integer(s)
	seq := append(rb, sb...)
	return append([]byte{0x30, byte(len(seq))}, seq...)
}

func asn1Integer(n *big.Int) []byte {
	b := n.Bytes()
	if len(b) > 0 && b[0]&0x80 != 0 {
		b = append([]byte{0}, b...)
	}
	if len(b) == 0 {
		b = []byte{0}
	}
	return append([]byte{0x02, byte(len(b))}, b...)
}

func hexToBytes(s string) ([]byte, error) {
	b := make([]byte, len(s)/2)
	for i := 0; i < len(s); i += 2 {
		hi := unhex(s[i])
		lo := unhex(s[i+1])
		if hi == 0xFF || lo == 0xFF {
			return nil, fmt.Errorf("invalid hex char at position %d", i)
		}
		b[i/2] = hi<<4 | lo
	}
	return b, nil
}

func unhex(c byte) byte {
	switch {
	case '0' <= c && c <= '9':
		return c - '0'
	case 'A' <= c && c <= 'F':
		return c - 'A' + 10
	case 'a' <= c && c <= 'f':
		return c - 'a' + 10
	}
	return 0xFF
}
