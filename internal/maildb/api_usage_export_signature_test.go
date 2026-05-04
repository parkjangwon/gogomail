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

func TestValidateCreateAPIUsageExportManifestSignatureRequestAcceptsEd25519(t *testing.T) {
	t.Parallel()

	req := CreateAPIUsageExportManifestSignatureRequest{
		BatchID:       "batch-1",
		DigestID:      "digest-1",
		SignerBackend: "local-ed25519",
		Signature: apimeter.ExportManifestSignature{
			Algorithm:       apimeter.ExportManifestSignatureAlgorithmEd25519,
			KeyID:           "key-1",
			SignedDigestHex: strings.Repeat("a", 64),
			SignatureHex:    strings.Repeat("b", 128),
		},
	}
	if err := ValidateCreateAPIUsageExportManifestSignatureRequest(&req); err != nil {
		t.Fatalf("ValidateCreateAPIUsageExportManifestSignatureRequest returned error: %v", err)
	}
}

func TestAPIUsageExportManifestSignatureAuditDetail(t *testing.T) {
	t.Parallel()

	detail, err := apiUsageExportManifestSignatureAuditDetail(APIUsageExportManifestSignatureView{
		ID:                 "signature-1",
		DigestID:           "digest-1",
		BatchID:            "batch-1",
		SignerBackend:      "local-hmac",
		KeyID:              "key-1",
		SignatureAlgorithm: apimeter.ExportManifestSignatureAlgorithmHMACSHA256,
		SignedDigestHex:    strings.Repeat("a", 64),
		SignatureHex:       strings.Repeat("b", 64),
		Metadata:           json.RawMessage(`{"ignored":"metadata is not audit detail"}`),
	})
	if err != nil {
		t.Fatalf("apiUsageExportManifestSignatureAuditDetail returned error: %v", err)
	}
	var got struct {
		SignatureID        string `json:"signature_id"`
		DigestID           string `json:"digest_id"`
		BatchID            string `json:"batch_id"`
		SignerBackend      string `json:"signer_backend"`
		KeyID              string `json:"key_id"`
		SignatureAlgorithm string `json:"signature_algorithm"`
		SignedDigestHex    string `json:"signed_digest_hex"`
		SignatureHexLen    int    `json:"signature_hex_len"`
	}
	if err := json.Unmarshal(detail, &got); err != nil {
		t.Fatalf("unmarshal audit detail: %v", err)
	}
	if got.SignatureID != "signature-1" || got.DigestID != "digest-1" || got.BatchID != "batch-1" {
		t.Fatalf("audit detail identity = %+v", got)
	}
	if got.SignerBackend != "local-hmac" || got.KeyID != "key-1" || got.SignatureAlgorithm != string(apimeter.ExportManifestSignatureAlgorithmHMACSHA256) {
		t.Fatalf("audit detail signer = %+v", got)
	}
	if got.SignedDigestHex != strings.Repeat("a", 64) || got.SignatureHexLen != 64 {
		t.Fatalf("audit detail evidence = %+v", got)
	}
	if strings.Contains(string(detail), "metadata") || strings.Contains(string(detail), strings.Repeat("b", 64)) {
		t.Fatalf("audit detail leaked metadata or full signature: %s", detail)
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
		{name: "mismatched backend algorithm", req: CreateAPIUsageExportManifestSignatureRequest{BatchID: "batch-1", DigestID: "digest-1", SignerBackend: "local-hmac", Signature: apimeter.ExportManifestSignature{Algorithm: apimeter.ExportManifestSignatureAlgorithmEd25519, KeyID: "key-1", SignedDigestHex: strings.Repeat("a", 64), SignatureHex: strings.Repeat("b", 128)}}},
		{name: "bad digest", req: CreateAPIUsageExportManifestSignatureRequest{BatchID: "batch-1", DigestID: "digest-1", SignerBackend: "local-hmac", Signature: apimeter.ExportManifestSignature{Algorithm: apimeter.ExportManifestSignatureAlgorithmHMACSHA256, KeyID: "key-1", SignedDigestHex: "nope", SignatureHex: strings.Repeat("b", 64)}}},
		{name: "bad signature", req: CreateAPIUsageExportManifestSignatureRequest{BatchID: "batch-1", DigestID: "digest-1", SignerBackend: "local-hmac", Signature: apimeter.ExportManifestSignature{Algorithm: apimeter.ExportManifestSignatureAlgorithmHMACSHA256, KeyID: "key-1", SignedDigestHex: strings.Repeat("a", 64), SignatureHex: "nope"}}},
		{name: "short ed25519 signature", req: CreateAPIUsageExportManifestSignatureRequest{BatchID: "batch-1", DigestID: "digest-1", SignerBackend: "local-ed25519", Signature: apimeter.ExportManifestSignature{Algorithm: apimeter.ExportManifestSignatureAlgorithmEd25519, KeyID: "key-1", SignedDigestHex: strings.Repeat("a", 64), SignatureHex: strings.Repeat("b", 64)}}},
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
