package apimeter

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHMACExportManifestSignerSignsAndVerifiesDigest(t *testing.T) {
	t.Parallel()

	digest := strings.Repeat("a", 64)
	signer := HMACExportManifestSigner{KeyID: "local-key-1", Secret: []byte("secret")}
	signature, err := signer.SignExportManifestDigest(digest)
	if err != nil {
		t.Fatalf("SignExportManifestDigest returned error: %v", err)
	}
	if signature.Algorithm != ExportManifestSignatureAlgorithmHMACSHA256 {
		t.Fatalf("Algorithm = %q", signature.Algorithm)
	}
	if signature.KeyID != "local-key-1" || signature.SignedDigestHex != digest || len(signature.SignatureHex) != 64 {
		t.Fatalf("signature = %+v", signature)
	}

	valid, err := VerifyExportManifestSignature(signature, []byte("secret"))
	if err != nil {
		t.Fatalf("VerifyExportManifestSignature returned error: %v", err)
	}
	if !valid {
		t.Fatal("signature should be valid")
	}

	valid, err = VerifyExportManifestSignature(signature, []byte("other-secret"))
	if err != nil {
		t.Fatalf("VerifyExportManifestSignature returned error: %v", err)
	}
	if valid {
		t.Fatal("signature should not verify with a different secret")
	}

	verifier := HMACExportManifestSignatureVerifier{Secret: []byte("secret")}
	valid, err = verifier.VerifyExportManifestSignature(signature)
	if err != nil {
		t.Fatalf("verifier returned error: %v", err)
	}
	if !valid {
		t.Fatal("verifier should accept signature")
	}

	verifier.KeyID = "other-key"
	valid, err = verifier.VerifyExportManifestSignature(signature)
	if err != nil {
		t.Fatalf("verifier returned error: %v", err)
	}
	if valid {
		t.Fatal("verifier should reject a different key id")
	}
}

func TestEd25519ExportManifestSignerSignsAndVerifiesDigest(t *testing.T) {
	t.Parallel()

	digest := strings.Repeat("b", 64)
	privateKey := ed25519.NewKeyFromSeed([]byte(strings.Repeat("s", ed25519.SeedSize)))
	publicKey := privateKey.Public().(ed25519.PublicKey)
	signer := Ed25519ExportManifestSigner{KeyID: "local-ed25519-1", PrivateKey: privateKey}
	signature, err := signer.SignExportManifestDigest(digest)
	if err != nil {
		t.Fatalf("SignExportManifestDigest returned error: %v", err)
	}
	if signature.Algorithm != ExportManifestSignatureAlgorithmEd25519 {
		t.Fatalf("Algorithm = %q", signature.Algorithm)
	}
	if signature.KeyID != "local-ed25519-1" || signature.SignedDigestHex != digest || len(signature.SignatureHex) != 128 {
		t.Fatalf("signature = %+v", signature)
	}

	verifier := Ed25519ExportManifestSignatureVerifier{PublicKey: publicKey}
	valid, err := verifier.VerifyExportManifestSignature(signature)
	if err != nil {
		t.Fatalf("VerifyExportManifestSignature returned error: %v", err)
	}
	if !valid {
		t.Fatal("signature should be valid")
	}

	verifier.KeyID = "other-key"
	valid, err = verifier.VerifyExportManifestSignature(signature)
	if err != nil {
		t.Fatalf("VerifyExportManifestSignature returned error: %v", err)
	}
	if valid {
		t.Fatal("signature should not verify with a different key id")
	}
	verifier.KeyID = ""

	signature.SignatureHex = strings.Repeat("c", 128)
	valid, err = verifier.VerifyExportManifestSignature(signature)
	if err != nil {
		t.Fatalf("VerifyExportManifestSignature returned error: %v", err)
	}
	if valid {
		t.Fatal("signature should not verify after tampering")
	}
}

func TestEd25519ExportManifestSignerRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	validKey := ed25519.NewKeyFromSeed([]byte(strings.Repeat("s", ed25519.SeedSize)))
	tests := []struct {
		name   string
		signer Ed25519ExportManifestSigner
		digest string
	}{
		{name: "missing key id", signer: Ed25519ExportManifestSigner{PrivateKey: validKey}, digest: strings.Repeat("a", 64)},
		{name: "missing private key", signer: Ed25519ExportManifestSigner{KeyID: "key-1"}, digest: strings.Repeat("a", 64)},
		{name: "bad digest", signer: Ed25519ExportManifestSigner{KeyID: "key-1", PrivateKey: validKey}, digest: "nope"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if _, err := tc.signer.SignExportManifestDigest(tc.digest); err == nil {
				t.Fatal("SignExportManifestDigest returned nil error")
			}
		})
	}
}

func TestRemoteEd25519ExportManifestSignerSignsAndVerifiesResponse(t *testing.T) {
	t.Parallel()

	digest := strings.Repeat("d", 64)
	privateKey := ed25519.NewKeyFromSeed([]byte(strings.Repeat("r", ed25519.SeedSize)))
	publicKey := privateKey.Public().(ed25519.PublicKey)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer token-1" {
			t.Fatalf("authorization = %q", got)
		}
		var req remoteEd25519SignRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.KeyID != "remote-key-1" || req.SignedDigestHex != digest || req.Algorithm != ExportManifestSignatureAlgorithmEd25519 {
			t.Fatalf("request = %+v", req)
		}
		_ = json.NewEncoder(w).Encode(ExportManifestSignature{
			Algorithm:       ExportManifestSignatureAlgorithmEd25519,
			KeyID:           req.KeyID,
			SignedDigestHex: req.SignedDigestHex,
			SignatureHex:    hex.EncodeToString(ed25519.Sign(privateKey, []byte(req.SignedDigestHex))),
		})
	}))
	defer server.Close()

	signature, err := (RemoteEd25519ExportManifestSigner{
		Endpoint:  server.URL,
		Token:     "token-1",
		KeyID:     "remote-key-1",
		PublicKey: publicKey,
		Client:    server.Client(),
	}).SignExportManifestDigest(digest)
	if err != nil {
		t.Fatalf("SignExportManifestDigest returned error: %v", err)
	}
	if signature.Algorithm != ExportManifestSignatureAlgorithmEd25519 || signature.KeyID != "remote-key-1" || len(signature.SignatureHex) != 128 {
		t.Fatalf("signature = %+v", signature)
	}
}

func TestRemoteEd25519ExportManifestSignerRejectsInvalidRemoteSignature(t *testing.T) {
	t.Parallel()

	digest := strings.Repeat("e", 64)
	privateKey := ed25519.NewKeyFromSeed([]byte(strings.Repeat("r", ed25519.SeedSize)))
	publicKey := privateKey.Public().(ed25519.PublicKey)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(ExportManifestSignature{
			Algorithm:       ExportManifestSignatureAlgorithmEd25519,
			KeyID:           "remote-key-1",
			SignedDigestHex: digest,
			SignatureHex:    strings.Repeat("f", 128),
		})
	}))
	defer server.Close()

	_, err := (RemoteEd25519ExportManifestSigner{
		Endpoint:  server.URL,
		KeyID:     "remote-key-1",
		PublicKey: publicKey,
		Client:    server.Client(),
	}).SignExportManifestDigest(digest)
	if err == nil || !strings.Contains(err.Error(), "invalid signature") {
		t.Fatalf("err = %v", err)
	}
}

func TestHMACExportManifestSignerRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		signer HMACExportManifestSigner
		digest string
	}{
		{name: "missing key", signer: HMACExportManifestSigner{Secret: []byte("secret")}, digest: strings.Repeat("a", 64)},
		{name: "key line break", signer: HMACExportManifestSigner{KeyID: "key-1\nbad", Secret: []byte("secret")}, digest: strings.Repeat("a", 64)},
		{name: "key too long", signer: HMACExportManifestSigner{KeyID: strings.Repeat("k", maxExportManifestSignatureKeyIDBytes+1), Secret: []byte("secret")}, digest: strings.Repeat("a", 64)},
		{name: "missing secret", signer: HMACExportManifestSigner{KeyID: "key-1"}, digest: strings.Repeat("a", 64)},
		{name: "secret too long", signer: HMACExportManifestSigner{KeyID: "key-1", Secret: []byte(strings.Repeat("s", maxExportManifestSigningSecretBytes+1))}, digest: strings.Repeat("a", 64)},
		{name: "bad digest", signer: HMACExportManifestSigner{KeyID: "key-1", Secret: []byte("secret")}, digest: "nope"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if _, err := tc.signer.SignExportManifestDigest(tc.digest); err == nil {
				t.Fatal("SignExportManifestDigest returned nil error")
			}
		})
	}
}

func TestRemoteEd25519ExportManifestSignerRejectsUnsafeTokenBeforeRequest(t *testing.T) {
	t.Parallel()

	digest := strings.Repeat("a", 64)
	publicKey := ed25519.NewKeyFromSeed([]byte(strings.Repeat("r", ed25519.SeedSize))).Public().(ed25519.PublicKey)
	for _, tc := range []struct {
		name  string
		token string
	}{
		{name: "line_break", token: "token\nbad"},
		{name: "too_long", token: strings.Repeat("t", maxRemoteSignerTokenBytes+1)},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := (RemoteEd25519ExportManifestSigner{
				Endpoint:  "https://signer.example.test/sign",
				Token:     tc.token,
				KeyID:     "remote-key-1",
				PublicKey: publicKey,
			}).SignExportManifestDigest(digest)
			if err == nil || !strings.Contains(err.Error(), "token") {
				t.Fatalf("err = %v, want token validation error", err)
			}
		})
	}
}

func TestRemoteEd25519ExportManifestSignerRejectsInvalidKeyIDMetadata(t *testing.T) {
	t.Parallel()

	digest := strings.Repeat("a", 64)
	privateKey := ed25519.NewKeyFromSeed([]byte(strings.Repeat("r", ed25519.SeedSize)))
	publicKey := privateKey.Public().(ed25519.PublicKey)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(ExportManifestSignature{
			Algorithm:       ExportManifestSignatureAlgorithmEd25519,
			KeyID:           "remote-key-1\nbad",
			SignedDigestHex: digest,
			SignatureHex:    hex.EncodeToString(ed25519.Sign(privateKey, []byte(digest))),
		})
	}))
	defer server.Close()

	_, err := (RemoteEd25519ExportManifestSigner{
		Endpoint:  server.URL,
		KeyID:     "remote-key-1",
		PublicKey: publicKey,
		Client:    server.Client(),
	}).SignExportManifestDigest(digest)
	if err == nil || !strings.Contains(err.Error(), "key id") {
		t.Fatalf("err = %v, want key id error", err)
	}
}
