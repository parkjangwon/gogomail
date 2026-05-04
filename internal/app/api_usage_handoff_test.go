package app

import (
	"crypto/ed25519"
	"encoding/json"
	"strings"
	"testing"

	"github.com/gogomail/gogomail/internal/apimeter"
	"github.com/gogomail/gogomail/internal/maildb"
)

func TestAPIUsageExportManifestCoversArtifacts(t *testing.T) {
	t.Parallel()

	artifacts := []maildb.APIUsageExportArtifactView{
		{
			ID:             "artifact-b",
			StorageBackend: "local",
			ObjectKey:      "exports/b.ndjson",
			ContentType:    apimeter.ExportArtifactContentTypeNDJSON,
			ByteCount:      20,
			SHA256Hex:      strings.Repeat("b", 64),
			EventCount:     2,
		},
		{
			ID:             "artifact-a",
			StorageBackend: "local",
			ObjectKey:      "exports/a.ndjson",
			ContentType:    apimeter.ExportArtifactContentTypeNDJSON,
			ByteCount:      10,
			SHA256Hex:      strings.Repeat("a", 64),
			EventCount:     1,
		},
	}
	raw, err := json.Marshal(apimeter.ExportManifest{
		SchemaVersion: apimeter.ExportManifestSchemaV1,
		Artifacts: []apimeter.ExportManifestArtifact{
			{
				ID:             "artifact-a",
				StorageBackend: "local",
				ObjectKey:      "exports/a.ndjson",
				ContentType:    apimeter.ExportArtifactContentTypeNDJSON,
				ByteCount:      10,
				SHA256Hex:      strings.Repeat("a", 64),
				EventCount:     1,
			},
			{
				ID:             "artifact-b",
				StorageBackend: "local",
				ObjectKey:      "exports/b.ndjson",
				ContentType:    apimeter.ExportArtifactContentTypeNDJSON,
				ByteCount:      20,
				SHA256Hex:      strings.Repeat("b", 64),
				EventCount:     2,
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}

	valid, err := apiUsageExportManifestCoversArtifacts(raw, artifacts)
	if err != nil {
		t.Fatalf("apiUsageExportManifestCoversArtifacts returned error: %v", err)
	}
	if !valid {
		t.Fatal("coverage = false, want true")
	}
}

func TestAPIUsageExportManifestCoversArtifactsRejectsMismatch(t *testing.T) {
	t.Parallel()

	raw, err := json.Marshal(apimeter.ExportManifest{
		SchemaVersion: apimeter.ExportManifestSchemaV1,
		Artifacts: []apimeter.ExportManifestArtifact{{
			ID:             "artifact-a",
			StorageBackend: "local",
			ObjectKey:      "exports/a.ndjson",
			ContentType:    apimeter.ExportArtifactContentTypeNDJSON,
			ByteCount:      10,
			SHA256Hex:      strings.Repeat("a", 64),
			EventCount:     1,
		}},
	})
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}

	valid, err := apiUsageExportManifestCoversArtifacts(raw, []maildb.APIUsageExportArtifactView{{
		ID:             "artifact-a",
		StorageBackend: "local",
		ObjectKey:      "exports/a.ndjson",
		ContentType:    apimeter.ExportArtifactContentTypeNDJSON,
		ByteCount:      11,
		SHA256Hex:      strings.Repeat("a", 64),
		EventCount:     1,
	}})
	if err != nil {
		t.Fatalf("apiUsageExportManifestCoversArtifacts returned error: %v", err)
	}
	if valid {
		t.Fatal("coverage = true, want false")
	}
}

func TestAdminServiceAPIUsageExportCapabilities(t *testing.T) {
	t.Parallel()

	service := adminService{
		exportManifestSigner:        apimeter.HMACExportManifestSigner{KeyID: "key-1", Secret: []byte("secret")},
		exportManifestSignerBackend: "local-hmac",
		exportManifestVerifier:      apimeter.HMACExportManifestSignatureVerifier{Secret: []byte("secret")},
	}
	view, err := service.GetAPIUsageExportCapabilities(t.Context())
	if err != nil {
		t.Fatalf("GetAPIUsageExportCapabilities returned error: %v", err)
	}
	if view.ExportFormat != "ndjson" || view.ArtifactContentType != apimeter.ExportArtifactContentTypeNDJSON {
		t.Fatalf("capabilities = %+v", view)
	}
	if !view.SignerConfigured || !view.VerifierConfigured || view.ProductionSignatureReady || view.BillingReadySupported || view.VerifiedBillingReadySupported {
		t.Fatalf("capabilities = %+v", view)
	}
	if view.SignerKeyID != "key-1" || strings.Join(view.BlockingReasons, ",") != "production_manifest_signer_required" {
		t.Fatalf("capabilities = %+v", view)
	}
}

func TestAdminServiceAPIUsageExportCapabilitiesLocalEd25519(t *testing.T) {
	t.Parallel()

	privateKey := ed25519.NewKeyFromSeed([]byte(strings.Repeat("s", ed25519.SeedSize)))
	service := adminService{
		exportManifestSigner:        apimeter.Ed25519ExportManifestSigner{KeyID: "key-2", PrivateKey: privateKey},
		exportManifestSignerBackend: "local-ed25519",
		exportManifestVerifier:      apimeter.Ed25519ExportManifestSignatureVerifier{KeyID: "key-2", PublicKey: privateKey.Public().(ed25519.PublicKey)},
	}
	view, err := service.GetAPIUsageExportCapabilities(t.Context())
	if err != nil {
		t.Fatalf("GetAPIUsageExportCapabilities returned error: %v", err)
	}
	if !view.SignerConfigured || !view.VerifierConfigured || view.ProductionSignatureReady || view.BillingReadySupported || view.VerifiedBillingReadySupported {
		t.Fatalf("capabilities = %+v", view)
	}
	if view.SignerKeyID != "key-2" || strings.Join(view.BlockingReasons, ",") != "production_manifest_signer_required" {
		t.Fatalf("capabilities = %+v", view)
	}
}

func TestAdminServiceAPIUsageExportCapabilitiesRemoteEd25519ProductionReady(t *testing.T) {
	t.Parallel()

	privateKey := ed25519.NewKeyFromSeed([]byte(strings.Repeat("s", ed25519.SeedSize)))
	service := adminService{
		exportManifestSigner: apimeter.RemoteEd25519ExportManifestSigner{
			KeyID:     "key-3",
			Endpoint:  "https://signer.example.test/sign",
			PublicKey: privateKey.Public().(ed25519.PublicKey),
		},
		exportManifestSignerBackend: "remote-ed25519",
		exportManifestVerifier:      apimeter.Ed25519ExportManifestSignatureVerifier{KeyID: "key-3", PublicKey: privateKey.Public().(ed25519.PublicKey)},
	}
	view, err := service.GetAPIUsageExportCapabilities(t.Context())
	if err != nil {
		t.Fatalf("GetAPIUsageExportCapabilities returned error: %v", err)
	}
	if !view.SignerConfigured || !view.VerifierConfigured || !view.ProductionSignatureReady || !view.BillingReadySupported || !view.VerifiedBillingReadySupported {
		t.Fatalf("capabilities = %+v", view)
	}
	if view.SignerKeyID != "key-3" || len(view.BlockingReasons) != 0 {
		t.Fatalf("capabilities = %+v", view)
	}
}

func TestAdminServiceAPIUsageExportCapabilitiesDisabled(t *testing.T) {
	t.Parallel()

	view, err := (adminService{}).GetAPIUsageExportCapabilities(t.Context())
	if err != nil {
		t.Fatalf("GetAPIUsageExportCapabilities returned error: %v", err)
	}
	if view.SignerConfigured || view.VerifierConfigured || view.ProductionSignatureReady {
		t.Fatalf("capabilities = %+v", view)
	}
	want := "manifest_signer_not_configured,manifest_signature_verifier_not_configured"
	if strings.Join(view.BlockingReasons, ",") != want {
		t.Fatalf("blocking = %v, want %s", view.BlockingReasons, want)
	}
}
