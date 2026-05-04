package maildb

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/gogomail/gogomail/internal/apimeter"
)

func TestValidateCreateAPIUsageExportManifestSignatureRequestNormalizes(t *testing.T) {
	t.Parallel()

	req := CreateAPIUsageExportManifestSignatureRequest{
		BatchID:       " batch-1 ",
		DigestID:      " digest-1 ",
		SignerBackend: " local-hmac ",
		Signature: apimeter.ExportManifestSignature{
			Algorithm:       apimeter.ExportManifestSignatureAlgorithmHMACSHA256,
			KeyID:           " key-1 ",
			SignedDigestHex: strings.ToUpper(strings.Repeat("a", 64)),
			SignatureHex:    strings.ToUpper(strings.Repeat("b", 64)),
		},
	}
	if err := ValidateCreateAPIUsageExportManifestSignatureRequest(&req); err != nil {
		t.Fatalf("ValidateCreateAPIUsageExportManifestSignatureRequest returned error: %v", err)
	}
	if req.BatchID != "batch-1" || req.DigestID != "digest-1" || req.SignerBackend != "local-hmac" {
		t.Fatalf("req ids/backend = %+v", req)
	}
	if req.Signature.KeyID != "key-1" || req.Signature.SignedDigestHex != strings.Repeat("a", 64) || req.Signature.SignatureHex != strings.Repeat("b", 64) {
		t.Fatalf("signature = %+v", req.Signature)
	}
	if string(req.Metadata) != "{}" {
		t.Fatalf("Metadata = %s", req.Metadata)
	}
}

func TestValidateCreateAPIUsageExportManifestSignatureRequestRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	validSignature := apimeter.ExportManifestSignature{
		Algorithm:       apimeter.ExportManifestSignatureAlgorithmHMACSHA256,
		KeyID:           "key-1",
		SignedDigestHex: strings.Repeat("a", 64),
		SignatureHex:    strings.Repeat("b", 64),
	}
	tests := []struct {
		name string
		req  CreateAPIUsageExportManifestSignatureRequest
	}{
		{name: "missing batch", req: CreateAPIUsageExportManifestSignatureRequest{DigestID: "digest-1", SignerBackend: "local-hmac", Signature: validSignature}},
		{name: "missing digest", req: CreateAPIUsageExportManifestSignatureRequest{BatchID: "batch-1", SignerBackend: "local-hmac", Signature: validSignature}},
		{name: "missing backend", req: CreateAPIUsageExportManifestSignatureRequest{BatchID: "batch-1", DigestID: "digest-1", Signature: validSignature}},
		{name: "bad algorithm", req: CreateAPIUsageExportManifestSignatureRequest{BatchID: "batch-1", DigestID: "digest-1", SignerBackend: "local-hmac", Signature: apimeter.ExportManifestSignature{Algorithm: "none", KeyID: "key-1", SignedDigestHex: strings.Repeat("a", 64), SignatureHex: strings.Repeat("b", 64)}}},
		{name: "bad digest", req: CreateAPIUsageExportManifestSignatureRequest{BatchID: "batch-1", DigestID: "digest-1", SignerBackend: "local-hmac", Signature: apimeter.ExportManifestSignature{Algorithm: apimeter.ExportManifestSignatureAlgorithmHMACSHA256, KeyID: "key-1", SignedDigestHex: "nope", SignatureHex: strings.Repeat("b", 64)}}},
		{name: "bad signature", req: CreateAPIUsageExportManifestSignatureRequest{BatchID: "batch-1", DigestID: "digest-1", SignerBackend: "local-hmac", Signature: apimeter.ExportManifestSignature{Algorithm: apimeter.ExportManifestSignatureAlgorithmHMACSHA256, KeyID: "key-1", SignedDigestHex: strings.Repeat("a", 64), SignatureHex: "nope"}}},
		{name: "array metadata", req: CreateAPIUsageExportManifestSignatureRequest{BatchID: "batch-1", DigestID: "digest-1", SignerBackend: "local-hmac", Signature: validSignature, Metadata: json.RawMessage(`[]`)}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if err := ValidateCreateAPIUsageExportManifestSignatureRequest(&tc.req); err == nil {
				t.Fatal("ValidateCreateAPIUsageExportManifestSignatureRequest returned nil error")
			}
		})
	}
}
