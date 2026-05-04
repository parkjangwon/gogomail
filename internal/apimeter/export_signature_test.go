package apimeter

import (
	"crypto/ed25519"
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

func TestHMACExportManifestSignerRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		signer HMACExportManifestSigner
		digest string
	}{
		{name: "missing key", signer: HMACExportManifestSigner{Secret: []byte("secret")}, digest: strings.Repeat("a", 64)},
		{name: "missing secret", signer: HMACExportManifestSigner{KeyID: "key-1"}, digest: strings.Repeat("a", 64)},
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
