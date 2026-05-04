package apimeter

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

const ExportManifestSignatureAlgorithmHMACSHA256 = "hmac-sha256"

type ExportManifestSigner interface {
	SignExportManifestDigest(digestHex string) (ExportManifestSignature, error)
}

type ExportManifestSignatureVerifier interface {
	VerifyExportManifestSignature(signature ExportManifestSignature) (bool, error)
}

type ExportManifestSignature struct {
	Algorithm       string `json:"algorithm"`
	KeyID           string `json:"key_id"`
	SignedDigestHex string `json:"signed_digest_hex"`
	SignatureHex    string `json:"signature_hex"`
}

type HMACExportManifestSigner struct {
	KeyID  string
	Secret []byte
}

func (s HMACExportManifestSigner) SignExportManifestDigest(digestHex string) (ExportManifestSignature, error) {
	digestHex = strings.ToLower(strings.TrimSpace(digestHex))
	if !isLowerHexSHA256(digestHex) {
		return ExportManifestSignature{}, fmt.Errorf("digest_hex must be 64 lowercase hex characters")
	}
	keyID := strings.TrimSpace(s.KeyID)
	if keyID == "" {
		return ExportManifestSignature{}, fmt.Errorf("key id is required")
	}
	if len(s.Secret) == 0 {
		return ExportManifestSignature{}, fmt.Errorf("signing secret is required")
	}
	mac := hmac.New(sha256.New, s.Secret)
	_, _ = mac.Write([]byte(digestHex))
	return ExportManifestSignature{
		Algorithm:       ExportManifestSignatureAlgorithmHMACSHA256,
		KeyID:           keyID,
		SignedDigestHex: digestHex,
		SignatureHex:    hex.EncodeToString(mac.Sum(nil)),
	}, nil
}

type HMACExportManifestSignatureVerifier struct {
	Secret []byte
}

func (v HMACExportManifestSignatureVerifier) VerifyExportManifestSignature(signature ExportManifestSignature) (bool, error) {
	return VerifyExportManifestSignature(signature, v.Secret)
}

func VerifyExportManifestSignature(signature ExportManifestSignature, secret []byte) (bool, error) {
	if signature.Algorithm != ExportManifestSignatureAlgorithmHMACSHA256 {
		return false, fmt.Errorf("unsupported signature algorithm %q", signature.Algorithm)
	}
	expected, err := (HMACExportManifestSigner{
		KeyID:  signature.KeyID,
		Secret: secret,
	}).SignExportManifestDigest(signature.SignedDigestHex)
	if err != nil {
		return false, err
	}
	got, err := hex.DecodeString(strings.TrimSpace(signature.SignatureHex))
	if err != nil {
		return false, fmt.Errorf("signature_hex must be hex: %w", err)
	}
	want, err := hex.DecodeString(expected.SignatureHex)
	if err != nil {
		return false, fmt.Errorf("expected signature must be hex: %w", err)
	}
	return hmac.Equal(got, want), nil
}

func isLowerHexSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, r := range value {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}
