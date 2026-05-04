package apimeter

import (
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
