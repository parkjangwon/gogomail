package maildb

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/gogomail/gogomail/internal/apimeter"
	"github.com/gogomail/gogomail/internal/audit"
)

func (r *Repository) CreateAPIUsageExportManifestDigest(ctx context.Context, batchID string) (APIUsageExportManifestDigestView, error) {
	if r.db == nil {
		return APIUsageExportManifestDigestView{}, fmt.Errorf("database handle is required")
	}
	batch, err := r.GetAPIUsageExportBatch(ctx, batchID)
	if err != nil {
		return APIUsageExportManifestDigestView{}, err
	}
	artifacts, err := r.listAllAPIUsageExportArtifacts(ctx, batch.ID)
	if err != nil {
		return APIUsageExportManifestDigestView{}, err
	}
	manifest := apiUsageExportManifest(batch, artifacts)
	digest, raw, err := apimeter.DigestExportManifest(manifest)
	if err != nil {
		return APIUsageExportManifestDigestView{}, err
	}
	id, err := newAPIUsageExportManifestDigestID()
	if err != nil {
		return APIUsageExportManifestDigestView{}, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return APIUsageExportManifestDigestView{}, fmt.Errorf("begin api usage export manifest digest transaction: %w", err)
	}
	defer tx.Rollback()
	const query = `
INSERT INTO api_usage_export_manifest_digests (
  id,
  batch_id,
  schema_version,
  digest_algorithm,
  digest_hex,
  manifest
) VALUES ($1, $2, $3, 'sha256', $4, $5)
ON CONFLICT (batch_id, digest_algorithm, digest_hex) DO UPDATE SET
  manifest = EXCLUDED.manifest
RETURNING id, batch_id, created_at, schema_version, digest_algorithm, digest_hex, manifest`
	var view APIUsageExportManifestDigestView
	if err := tx.QueryRowContext(ctx, query, id, batch.ID, manifest.SchemaVersion, digest, raw).Scan(
		&view.ID,
		&view.BatchID,
		&view.CreatedAt,
		&view.SchemaVersion,
		&view.DigestAlgorithm,
		&view.DigestHex,
		&view.Manifest,
	); err != nil {
		return APIUsageExportManifestDigestView{}, fmt.Errorf("create api usage export manifest digest: %w", err)
	}
	detail, err := apiUsageExportManifestDigestAuditDetail(view, len(manifest.Artifacts))
	if err != nil {
		return APIUsageExportManifestDigestView{}, err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		Category:   "admin",
		Action:     "api_usage_export.manifest_digest_create",
		TargetType: "api_usage_export_manifest_digest",
		TargetID:   view.ID,
		Result:     "created",
		Detail:     detail,
	}); err != nil {
		return APIUsageExportManifestDigestView{}, fmt.Errorf("record api usage export manifest digest audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return APIUsageExportManifestDigestView{}, fmt.Errorf("commit api usage export manifest digest transaction: %w", err)
	}
	return view, nil
}

func apiUsageExportManifestDigestAuditDetail(digest APIUsageExportManifestDigestView, artifactCount int) (json.RawMessage, error) {
	detail, err := json.Marshal(map[string]any{
		"digest_id":        digest.ID,
		"batch_id":         digest.BatchID,
		"schema_version":   digest.SchemaVersion,
		"digest_algorithm": digest.DigestAlgorithm,
		"digest_hex":       digest.DigestHex,
		"manifest_bytes":   len(digest.Manifest),
		"artifact_count":   artifactCount,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal api usage export manifest digest audit detail: %w", err)
	}
	return detail, nil
}

func (r *Repository) ListAPIUsageExportManifestDigests(ctx context.Context, batchID string, limit int) ([]APIUsageExportManifestDigestView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	batchID = strings.TrimSpace(batchID)
	if batchID == "" {
		return nil, fmt.Errorf("batch_id is required")
	}
	limit = normalizeLimit(limit)
	const query = `
SELECT id, batch_id, created_at, schema_version, digest_algorithm, digest_hex, manifest
FROM api_usage_export_manifest_digests
WHERE batch_id = $1
ORDER BY created_at DESC, id DESC
LIMIT $2`
	rows, err := r.db.QueryContext(ctx, query, batchID, limit)
	if err != nil {
		return nil, fmt.Errorf("list api usage export manifest digests: %w", err)
	}
	defer rows.Close()

	var digests []APIUsageExportManifestDigestView
	for rows.Next() {
		var digest APIUsageExportManifestDigestView
		if err := rows.Scan(
			&digest.ID,
			&digest.BatchID,
			&digest.CreatedAt,
			&digest.SchemaVersion,
			&digest.DigestAlgorithm,
			&digest.DigestHex,
			&digest.Manifest,
		); err != nil {
			return nil, fmt.Errorf("scan api usage export manifest digest: %w", err)
		}
		digests = append(digests, digest)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate api usage export manifest digests: %w", err)
	}
	return digests, nil
}

func (r *Repository) GetAPIUsageExportManifestDigest(ctx context.Context, batchID string, digestID string) (APIUsageExportManifestDigestView, error) {
	if r.db == nil {
		return APIUsageExportManifestDigestView{}, fmt.Errorf("database handle is required")
	}
	batchID = strings.TrimSpace(batchID)
	digestID = strings.TrimSpace(digestID)
	if batchID == "" {
		return APIUsageExportManifestDigestView{}, fmt.Errorf("batch_id is required")
	}
	if digestID == "" {
		return APIUsageExportManifestDigestView{}, fmt.Errorf("digest_id is required")
	}
	const query = `
SELECT id, batch_id, created_at, schema_version, digest_algorithm, digest_hex, manifest
FROM api_usage_export_manifest_digests
WHERE batch_id = $1
  AND id = $2`
	var digest APIUsageExportManifestDigestView
	if err := r.db.QueryRowContext(ctx, query, batchID, digestID).Scan(
		&digest.ID,
		&digest.BatchID,
		&digest.CreatedAt,
		&digest.SchemaVersion,
		&digest.DigestAlgorithm,
		&digest.DigestHex,
		&digest.Manifest,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return APIUsageExportManifestDigestView{}, fmt.Errorf("api usage export manifest digest not found")
		}
		return APIUsageExportManifestDigestView{}, fmt.Errorf("get api usage export manifest digest: %w", err)
	}
	return digest, nil
}

func (r *Repository) VerifyAPIUsageExportManifestDigest(ctx context.Context, batchID string, digestID string) (APIUsageExportManifestDigestVerificationView, error) {
	digest, err := r.GetAPIUsageExportManifestDigest(ctx, batchID, digestID)
	if err != nil {
		return APIUsageExportManifestDigestVerificationView{}, err
	}
	return apiUsageExportManifestDigestVerification(digest)
}

func apiUsageExportManifestDigestVerification(digest APIUsageExportManifestDigestView) (APIUsageExportManifestDigestVerificationView, error) {
	actual, canonical, err := apimeter.DigestExportManifestJSON(digest.Manifest)
	if err != nil {
		return APIUsageExportManifestDigestVerificationView{}, err
	}
	return APIUsageExportManifestDigestVerificationView{
		BatchID:           digest.BatchID,
		DigestID:          digest.ID,
		SchemaVersion:     digest.SchemaVersion,
		DigestAlgorithm:   digest.DigestAlgorithm,
		ExpectedDigestHex: digest.DigestHex,
		ActualDigestHex:   actual,
		Valid:             digest.DigestAlgorithm == "sha256" && digest.DigestHex == actual,
		CanonicalManifest: canonical,
	}, nil
}

func applyAPIUsageExportHandoffReadiness(view *APIUsageExportHandoffView) {
	view.EventsCovered = view.ArtifactEventCount >= view.EventCount
	var missing []string
	if !view.BatchCompleted {
		missing = append(missing, "batch_completed")
	}
	if view.ArtifactCount == 0 {
		missing = append(missing, "export_artifact")
	}
	if !view.EventsCovered {
		missing = append(missing, "event_coverage")
	}
	if view.ManifestDigestCount == 0 || view.LatestManifestDigestID == "" {
		missing = append(missing, "manifest_digest")
	}
	if view.LatestDigestSignatureCount == 0 {
		missing = append(missing, "manifest_signature")
	}
	view.MissingRequirements = missing
	view.Ready = len(missing) == 0
	view.ReadinessGrade = "billing_blocked"
	if !view.Ready {
		view.BillingBlockingReasons = []string{"handoff_not_ready"}
		return
	}
	if apiUsageExportManifestSignerNeedsProductionBackend(view.LatestSignatureSigner) {
		view.ReadinessGrade = "operational"
		view.BillingBlockingReasons = []string{"production_manifest_signer_required"}
		return
	}
	view.ReadinessGrade = "billing_candidate"
	view.BillingReady = true
}

func apiUsageExportManifestSignerNeedsProductionBackend(backend string) bool {
	switch strings.ToLower(strings.TrimSpace(backend)) {
	case "", "local-hmac", "local-ed25519":
		return true
	default:
		return false
	}
}

func (r *Repository) CreateAPIUsageExportManifestSignature(ctx context.Context, req CreateAPIUsageExportManifestSignatureRequest) (APIUsageExportManifestSignatureView, error) {
	if r.db == nil {
		return APIUsageExportManifestSignatureView{}, fmt.Errorf("database handle is required")
	}
	if err := ValidateCreateAPIUsageExportManifestSignatureRequest(&req); err != nil {
		return APIUsageExportManifestSignatureView{}, err
	}
	digest, err := r.GetAPIUsageExportManifestDigest(ctx, req.BatchID, req.DigestID)
	if err != nil {
		return APIUsageExportManifestSignatureView{}, err
	}
	if digest.DigestHex != req.Signature.SignedDigestHex {
		return APIUsageExportManifestSignatureView{}, fmt.Errorf("signed_digest_hex must match manifest digest")
	}
	id, err := newAPIUsageExportManifestSignatureID()
	if err != nil {
		return APIUsageExportManifestSignatureView{}, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return APIUsageExportManifestSignatureView{}, fmt.Errorf("begin api usage export manifest signature transaction: %w", err)
	}
	defer tx.Rollback()
	const query = `
INSERT INTO api_usage_export_manifest_signatures (
  id,
  digest_id,
  batch_id,
  signer_backend,
  key_id,
  signature_algorithm,
  signed_digest_hex,
  signature_hex,
  metadata
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (digest_id, signature_algorithm, key_id, signature_hex) DO UPDATE SET
  metadata = EXCLUDED.metadata
RETURNING id, digest_id, batch_id, created_at, signer_backend, key_id,
  signature_algorithm, signed_digest_hex, signature_hex, metadata`
	var view APIUsageExportManifestSignatureView
	if err := tx.QueryRowContext(
		ctx,
		query,
		id,
		req.DigestID,
		req.BatchID,
		req.SignerBackend,
		req.Signature.KeyID,
		req.Signature.Algorithm,
		req.Signature.SignedDigestHex,
		req.Signature.SignatureHex,
		req.Metadata,
	).Scan(
		&view.ID,
		&view.DigestID,
		&view.BatchID,
		&view.CreatedAt,
		&view.SignerBackend,
		&view.KeyID,
		&view.SignatureAlgorithm,
		&view.SignedDigestHex,
		&view.SignatureHex,
		&view.Metadata,
	); err != nil {
		return APIUsageExportManifestSignatureView{}, fmt.Errorf("create api usage export manifest signature: %w", err)
	}
	detail, err := apiUsageExportManifestSignatureAuditDetail(view)
	if err != nil {
		return APIUsageExportManifestSignatureView{}, err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		Category:   "admin",
		Action:     "api_usage_export.manifest_signature_create",
		TargetType: "api_usage_export_manifest_signature",
		TargetID:   view.ID,
		Result:     "created",
		Detail:     detail,
	}); err != nil {
		return APIUsageExportManifestSignatureView{}, fmt.Errorf("record api usage export manifest signature audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return APIUsageExportManifestSignatureView{}, fmt.Errorf("commit api usage export manifest signature transaction: %w", err)
	}
	return view, nil
}

func apiUsageExportManifestSignatureAuditDetail(signature APIUsageExportManifestSignatureView) (json.RawMessage, error) {
	detail, err := json.Marshal(map[string]any{
		"signature_id":        signature.ID,
		"digest_id":           signature.DigestID,
		"batch_id":            signature.BatchID,
		"signer_backend":      signature.SignerBackend,
		"key_id":              signature.KeyID,
		"signature_algorithm": signature.SignatureAlgorithm,
		"signed_digest_hex":   signature.SignedDigestHex,
		"signature_hex_len":   len(signature.SignatureHex),
	})
	if err != nil {
		return nil, fmt.Errorf("marshal api usage export manifest signature audit detail: %w", err)
	}
	return detail, nil
}

func (r *Repository) ListAPIUsageExportManifestSignatures(ctx context.Context, batchID string, digestID string, limit int) ([]APIUsageExportManifestSignatureView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	batchID = strings.TrimSpace(batchID)
	digestID = strings.TrimSpace(digestID)
	if batchID == "" {
		return nil, fmt.Errorf("batch_id is required")
	}
	if digestID == "" {
		return nil, fmt.Errorf("digest_id is required")
	}
	limit = normalizeLimit(limit)
	const query = `
SELECT id, digest_id, batch_id, created_at, signer_backend, key_id,
  signature_algorithm, signed_digest_hex, signature_hex, metadata
FROM api_usage_export_manifest_signatures
WHERE batch_id = $1
  AND digest_id = $2
ORDER BY created_at DESC, id DESC
LIMIT $3`
	rows, err := r.db.QueryContext(ctx, query, batchID, digestID, limit)
	if err != nil {
		return nil, fmt.Errorf("list api usage export manifest signatures: %w", err)
	}
	defer rows.Close()

	var signatures []APIUsageExportManifestSignatureView
	for rows.Next() {
		var signature APIUsageExportManifestSignatureView
		if err := scanAPIUsageExportManifestSignature(rows, &signature); err != nil {
			return nil, fmt.Errorf("scan api usage export manifest signature: %w", err)
		}
		signatures = append(signatures, signature)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate api usage export manifest signatures: %w", err)
	}
	return signatures, nil
}

func (r *Repository) GetAPIUsageExportManifestSignature(ctx context.Context, batchID string, digestID string, signatureID string) (APIUsageExportManifestSignatureView, error) {
	if r.db == nil {
		return APIUsageExportManifestSignatureView{}, fmt.Errorf("database handle is required")
	}
	batchID = strings.TrimSpace(batchID)
	digestID = strings.TrimSpace(digestID)
	signatureID = strings.TrimSpace(signatureID)
	if batchID == "" {
		return APIUsageExportManifestSignatureView{}, fmt.Errorf("batch_id is required")
	}
	if digestID == "" {
		return APIUsageExportManifestSignatureView{}, fmt.Errorf("digest_id is required")
	}
	if signatureID == "" {
		return APIUsageExportManifestSignatureView{}, fmt.Errorf("signature_id is required")
	}
	const query = `
SELECT id, digest_id, batch_id, created_at, signer_backend, key_id,
  signature_algorithm, signed_digest_hex, signature_hex, metadata
FROM api_usage_export_manifest_signatures
WHERE batch_id = $1
  AND digest_id = $2
  AND id = $3`
	var signature APIUsageExportManifestSignatureView
	if err := scanAPIUsageExportManifestSignature(r.db.QueryRowContext(ctx, query, batchID, digestID, signatureID), &signature); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return APIUsageExportManifestSignatureView{}, fmt.Errorf("api usage export manifest signature not found")
		}
		return APIUsageExportManifestSignatureView{}, fmt.Errorf("get api usage export manifest signature: %w", err)
	}
	return signature, nil
}

type apiUsageExportManifestSignatureScanner interface {
	Scan(dest ...any) error
}

func scanAPIUsageExportManifestSignature(scanner apiUsageExportManifestSignatureScanner, signature *APIUsageExportManifestSignatureView) error {
	return scanner.Scan(
		&signature.ID,
		&signature.DigestID,
		&signature.BatchID,
		&signature.CreatedAt,
		&signature.SignerBackend,
		&signature.KeyID,
		&signature.SignatureAlgorithm,
		&signature.SignedDigestHex,
		&signature.SignatureHex,
		&signature.Metadata,
	)
}

func ValidateCreateAPIUsageExportManifestSignatureRequest(req *CreateAPIUsageExportManifestSignatureRequest) error {
	req.BatchID = strings.TrimSpace(req.BatchID)
	req.DigestID = strings.TrimSpace(req.DigestID)
	req.SignerBackend = strings.TrimSpace(req.SignerBackend)
	req.Signature.Algorithm = strings.TrimSpace(req.Signature.Algorithm)
	req.Signature.KeyID = strings.TrimSpace(req.Signature.KeyID)
	req.Signature.SignedDigestHex = strings.ToLower(strings.TrimSpace(req.Signature.SignedDigestHex))
	req.Signature.SignatureHex = strings.ToLower(strings.TrimSpace(req.Signature.SignatureHex))
	if req.BatchID == "" {
		return fmt.Errorf("batch_id is required")
	}
	if req.DigestID == "" {
		return fmt.Errorf("digest_id is required")
	}
	if req.SignerBackend == "" {
		return fmt.Errorf("signer_backend is required")
	}
	switch req.Signature.Algorithm {
	case apimeter.ExportManifestSignatureAlgorithmHMACSHA256, apimeter.ExportManifestSignatureAlgorithmEd25519:
	default:
		return fmt.Errorf("signature_algorithm must be hmac-sha256 or ed25519")
	}
	if !apiUsageExportManifestSignatureBackendMatchesAlgorithm(req.SignerBackend, req.Signature.Algorithm) {
		return fmt.Errorf("signer_backend %q is not compatible with signature_algorithm %q", req.SignerBackend, req.Signature.Algorithm)
	}
	if req.Signature.KeyID == "" {
		return fmt.Errorf("key_id is required")
	}
	if !isLowerHexSHA256(req.Signature.SignedDigestHex) {
		return fmt.Errorf("signed_digest_hex must be 64 lowercase hex characters")
	}
	if req.Signature.Algorithm == apimeter.ExportManifestSignatureAlgorithmHMACSHA256 && !isLowerHexBytes(req.Signature.SignatureHex, 32) {
		return fmt.Errorf("signature_hex must be 64 lowercase hex characters")
	}
	if req.Signature.Algorithm == apimeter.ExportManifestSignatureAlgorithmEd25519 && !isLowerHexBytes(req.Signature.SignatureHex, 64) {
		return fmt.Errorf("signature_hex must be 128 lowercase hex characters")
	}
	if len(req.Metadata) == 0 {
		req.Metadata = json.RawMessage(`{}`)
	}
	var metadata map[string]any
	if err := json.Unmarshal(req.Metadata, &metadata); err != nil {
		return fmt.Errorf("metadata must be a JSON object: %w", err)
	}
	return nil
}

func apiUsageExportManifestSignatureBackendMatchesAlgorithm(backend string, algorithm string) bool {
	switch strings.ToLower(strings.TrimSpace(backend)) {
	case "local-hmac":
		return algorithm == apimeter.ExportManifestSignatureAlgorithmHMACSHA256
	case "local-ed25519":
		return algorithm == apimeter.ExportManifestSignatureAlgorithmEd25519
	default:
		return true
	}
}

func apiUsageExportManifest(batch APIUsageExportBatchView, artifacts []APIUsageExportArtifactView) apimeter.ExportManifest {
	ordered := append([]APIUsageExportArtifactView(nil), artifacts...)
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].ID < ordered[j].ID
	})
	manifest := apimeter.ExportManifest{
		SchemaVersion: apimeter.ExportManifestSchemaV1,
		Batch: apimeter.ExportManifestBatch{
			ID:             batch.ID,
			TenantID:       batch.TenantID,
			PrincipalID:    batch.PrincipalID,
			WindowStart:    apimeter.FormatManifestTime(batch.WindowStart),
			WindowEnd:      apimeter.FormatManifestTime(batch.WindowEnd),
			EventCount:     batch.EventCount,
			RequestCount:   batch.RequestCount,
			RequestBytes:   batch.RequestBytes,
			ResponseBytes:  batch.ResponseBytes,
			LatencyMSTotal: batch.LatencyMSTotal,
			LatencyMSMax:   batch.LatencyMSMax,
		},
		Artifacts: make([]apimeter.ExportManifestArtifact, 0, len(ordered)),
	}
	for _, artifact := range ordered {
		manifest.Artifacts = append(manifest.Artifacts, apimeter.ExportManifestArtifact{
			ID:             artifact.ID,
			StorageBackend: artifact.StorageBackend,
			ObjectKey:      artifact.ObjectKey,
			ContentType:    artifact.ContentType,
			ByteCount:      artifact.ByteCount,
			SHA256Hex:      artifact.SHA256Hex,
			EventCount:     artifact.EventCount,
		})
	}
	return manifest
}

func newAPIUsageExportManifestDigestID() (string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("generate api usage export manifest digest id: %w", err)
	}
	return "api-usage-manifest-" + hex.EncodeToString(random[:]), nil
}

func newAPIUsageExportManifestSignatureID() (string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("generate api usage export manifest signature id: %w", err)
	}
	return "api-usage-signature-" + hex.EncodeToString(random[:]), nil
}
