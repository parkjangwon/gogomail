package apimeter

import (
	"bytes"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	ExportManifestSignatureAlgorithmHMACSHA256 = "hmac-sha256"
	ExportManifestSignatureAlgorithmEd25519    = "ed25519"
	maxExportManifestSignatureKeyIDBytes       = 200
	maxExportManifestSigningSecretBytes        = 4096
	maxRemoteSignerTokenBytes                  = 4096
	hmacSHA256SignatureHexBytes                = 64
	ed25519SignatureHexBytes                   = ed25519.SignatureSize * 2
)

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
	keyID, err := normalizeExportManifestSignatureKeyID(s.KeyID)
	if err != nil {
		return ExportManifestSignature{}, err
	}
	if len(s.Secret) == 0 {
		return ExportManifestSignature{}, fmt.Errorf("signing secret is required")
	}
	if len(s.Secret) > maxExportManifestSigningSecretBytes {
		return ExportManifestSignature{}, fmt.Errorf("signing secret must be at most %d bytes", maxExportManifestSigningSecretBytes)
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
	KeyID  string
	Secret []byte
}

func (v HMACExportManifestSignatureVerifier) VerifyExportManifestSignature(signature ExportManifestSignature) (bool, error) {
	if keyID := strings.TrimSpace(v.KeyID); keyID != "" && strings.TrimSpace(signature.KeyID) != keyID {
		return false, nil
	}
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
	signatureHex := strings.TrimSpace(signature.SignatureHex)
	if len(signatureHex) != hmacSHA256SignatureHexBytes {
		return false, fmt.Errorf("hmac signature must be %d hex characters", hmacSHA256SignatureHexBytes)
	}
	got, err := hex.DecodeString(signatureHex)
	if err != nil {
		return false, fmt.Errorf("signature_hex must be hex: %w", err)
	}
	want, err := hex.DecodeString(expected.SignatureHex)
	if err != nil {
		return false, fmt.Errorf("expected signature must be hex: %w", err)
	}
	return hmac.Equal(got, want), nil
}

type Ed25519ExportManifestSigner struct {
	KeyID      string
	PrivateKey ed25519.PrivateKey
}

func (s Ed25519ExportManifestSigner) SignExportManifestDigest(digestHex string) (ExportManifestSignature, error) {
	digestHex = strings.ToLower(strings.TrimSpace(digestHex))
	if !isLowerHexSHA256(digestHex) {
		return ExportManifestSignature{}, fmt.Errorf("digest_hex must be 64 lowercase hex characters")
	}
	keyID, err := normalizeExportManifestSignatureKeyID(s.KeyID)
	if err != nil {
		return ExportManifestSignature{}, err
	}
	if len(s.PrivateKey) != ed25519.PrivateKeySize {
		return ExportManifestSignature{}, fmt.Errorf("ed25519 private key must be %d bytes", ed25519.PrivateKeySize)
	}
	return ExportManifestSignature{
		Algorithm:       ExportManifestSignatureAlgorithmEd25519,
		KeyID:           keyID,
		SignedDigestHex: digestHex,
		SignatureHex:    hex.EncodeToString(ed25519.Sign(s.PrivateKey, []byte(digestHex))),
	}, nil
}

type Ed25519ExportManifestSignatureVerifier struct {
	KeyID     string
	PublicKey ed25519.PublicKey
}

func (v Ed25519ExportManifestSignatureVerifier) VerifyExportManifestSignature(signature ExportManifestSignature) (bool, error) {
	if signature.Algorithm != ExportManifestSignatureAlgorithmEd25519 {
		return false, fmt.Errorf("unsupported signature algorithm %q", signature.Algorithm)
	}
	if len(v.PublicKey) != ed25519.PublicKeySize {
		return false, fmt.Errorf("ed25519 public key must be %d bytes", ed25519.PublicKeySize)
	}
	if keyID := strings.TrimSpace(v.KeyID); keyID != "" && strings.TrimSpace(signature.KeyID) != keyID {
		return false, nil
	}
	digestHex := strings.ToLower(strings.TrimSpace(signature.SignedDigestHex))
	if !isLowerHexSHA256(digestHex) {
		return false, fmt.Errorf("signed_digest_hex must be 64 lowercase hex characters")
	}
	signatureHex := strings.TrimSpace(signature.SignatureHex)
	if len(signatureHex) != ed25519SignatureHexBytes {
		return false, fmt.Errorf("ed25519 signature must be %d hex characters", ed25519SignatureHexBytes)
	}
	signatureBytes, err := hex.DecodeString(signatureHex)
	if err != nil {
		return false, fmt.Errorf("signature_hex must be hex: %w", err)
	}
	if len(signatureBytes) != ed25519.SignatureSize {
		return false, fmt.Errorf("ed25519 signature must be %d bytes", ed25519.SignatureSize)
	}
	return ed25519.Verify(v.PublicKey, []byte(digestHex), signatureBytes), nil
}

type RemoteEd25519ExportManifestSigner struct {
	Endpoint  string
	Token     string
	KeyID     string
	PublicKey ed25519.PublicKey
	Client    *http.Client
}

func (s RemoteEd25519ExportManifestSigner) SignExportManifestDigest(digestHex string) (ExportManifestSignature, error) {
	digestHex = strings.ToLower(strings.TrimSpace(digestHex))
	if !isLowerHexSHA256(digestHex) {
		return ExportManifestSignature{}, fmt.Errorf("digest_hex must be 64 lowercase hex characters")
	}
	keyID, err := normalizeExportManifestSignatureKeyID(s.KeyID)
	if err != nil {
		return ExportManifestSignature{}, err
	}
	endpoint := strings.TrimSpace(s.Endpoint)
	if endpoint == "" {
		return ExportManifestSignature{}, fmt.Errorf("remote signer endpoint is required")
	}
	body, err := json.Marshal(remoteEd25519SignRequest{
		Algorithm:       ExportManifestSignatureAlgorithmEd25519,
		KeyID:           keyID,
		SignedDigestHex: digestHex,
	})
	if err != nil {
		return ExportManifestSignature{}, fmt.Errorf("encode remote signer request: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return ExportManifestSignature{}, fmt.Errorf("create remote signer request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if token := strings.TrimSpace(s.Token); token != "" {
		if strings.ContainsAny(token, "\r\n") {
			return ExportManifestSignature{}, fmt.Errorf("remote signer token cannot contain line breaks")
		}
		if len(token) > maxRemoteSignerTokenBytes {
			return ExportManifestSignature{}, fmt.Errorf("remote signer token must be at most %d bytes", maxRemoteSignerTokenBytes)
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}
	client := s.Client
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return ExportManifestSignature{}, fmt.Errorf("call remote signer: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return ExportManifestSignature{}, fmt.Errorf("remote signer returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var signature ExportManifestSignature
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&signature); err != nil {
		return ExportManifestSignature{}, fmt.Errorf("decode remote signer response: %w", err)
	}
	signature.Algorithm = strings.TrimSpace(signature.Algorithm)
	signature.KeyID, err = normalizeExportManifestSignatureKeyID(signature.KeyID)
	if err != nil {
		return ExportManifestSignature{}, err
	}
	signature.SignedDigestHex = strings.ToLower(strings.TrimSpace(signature.SignedDigestHex))
	signature.SignatureHex = strings.ToLower(strings.TrimSpace(signature.SignatureHex))
	if signature.KeyID != keyID {
		return ExportManifestSignature{}, fmt.Errorf("remote signer returned key_id %q, want %q", signature.KeyID, keyID)
	}
	if signature.SignedDigestHex != digestHex {
		return ExportManifestSignature{}, fmt.Errorf("remote signer returned signed_digest_hex that does not match request")
	}
	valid, err := (Ed25519ExportManifestSignatureVerifier{
		KeyID:     keyID,
		PublicKey: s.PublicKey,
	}).VerifyExportManifestSignature(signature)
	if err != nil {
		return ExportManifestSignature{}, err
	}
	if !valid {
		return ExportManifestSignature{}, fmt.Errorf("remote signer returned an invalid signature")
	}
	return signature, nil
}

func normalizeExportManifestSignatureKeyID(value string) (string, error) {
	value = strings.ToValidUTF8(strings.TrimSpace(value), "")
	if value == "" {
		return "", fmt.Errorf("key id is required")
	}
	if strings.ContainsAny(value, "\r\n") {
		return "", fmt.Errorf("key id cannot contain line breaks")
	}
	if len(value) > maxExportManifestSignatureKeyIDBytes {
		return "", fmt.Errorf("key id must be at most %d bytes", maxExportManifestSignatureKeyIDBytes)
	}
	return value, nil
}

type remoteEd25519SignRequest struct {
	Algorithm       string `json:"algorithm"`
	KeyID           string `json:"key_id"`
	SignedDigestHex string `json:"signed_digest_hex"`
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
