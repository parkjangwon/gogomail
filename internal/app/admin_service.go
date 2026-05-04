package app

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/gogomail/gogomail/internal/apimeter"
	"github.com/gogomail/gogomail/internal/backpressure"
	"github.com/gogomail/gogomail/internal/maildb"
)

type backpressureStore interface {
	State(ctx context.Context) (backpressure.State, error)
	SetState(ctx context.Context, update backpressure.StateUpdate) (backpressure.State, error)
}

type adminService struct {
	*maildb.Repository
	backpressure                backpressureStore
	exportStore                 apimeter.ExportArtifactStore
	exportManifestSigner        apimeter.ExportManifestSigner
	exportManifestSignerBackend string
	exportManifestVerifySecret  []byte
}

func (s adminService) GetBackpressure(ctx context.Context) (backpressure.State, error) {
	if s.backpressure == nil {
		return backpressure.State{}, fmt.Errorf("backpressure backend is not configured")
	}
	return s.backpressure.State(ctx)
}

func (s adminService) UpdateBackpressure(ctx context.Context, req backpressure.StateUpdate) (backpressure.State, error) {
	if s.backpressure == nil {
		return backpressure.State{}, fmt.Errorf("backpressure backend is not configured")
	}
	return s.backpressure.SetState(ctx, req)
}

func (s adminService) WriteAPIUsageExportArtifact(ctx context.Context, batchID string, req maildb.WriteAPIUsageExportArtifactRequest) (maildb.APIUsageExportArtifactView, error) {
	if s.Repository == nil {
		return maildb.APIUsageExportArtifactView{}, fmt.Errorf("repository is required")
	}
	if s.exportStore == nil {
		return maildb.APIUsageExportArtifactView{}, fmt.Errorf("api usage export artifact store is not configured")
	}
	batch, err := s.GetAPIUsageExportBatch(ctx, batchID)
	if err != nil {
		return maildb.APIUsageExportArtifactView{}, err
	}
	objectKey := strings.TrimSpace(req.ObjectKey)
	if objectKey == "" {
		objectKey, err = apimeter.DefaultExportArtifactObjectKey(batch.ID)
		if err != nil {
			return maildb.APIUsageExportArtifactView{}, err
		}
	}
	metadata := req.Metadata
	if len(metadata) == 0 {
		metadata, err = json.Marshal(map[string]string{
			"batch_id": batch.ID,
			"writer":   "gogomail-admin-api",
		})
		if err != nil {
			return maildb.APIUsageExportArtifactView{}, fmt.Errorf("marshal export artifact metadata: %w", err)
		}
	}

	ledgerReq := apiUsageLedgerRequestFromBatch(batch, maildb.APIUsageLedgerNoLimit)
	var eventCount int64
	result, err := apimeter.WriteExportArtifact(ctx, s.exportStore, apimeter.ExportArtifactWriteRequest{
		ObjectKey: objectKey,
		Metadata:  metadata,
		Encode: func(w io.Writer) error {
			return s.StreamAPIUsageLedger(ctx, ledgerReq, func(usage maildb.APIUsageLedgerView) error {
				eventCount++
				return json.NewEncoder(w).Encode(usage)
			})
		},
	})
	if err != nil {
		return maildb.APIUsageExportArtifactView{}, err
	}
	if batch.EventCount != eventCount {
		s.cleanupAPIUsageExportArtifactObject(ctx, result.ObjectKey)
		return maildb.APIUsageExportArtifactView{}, fmt.Errorf("api usage export batch expected %d events but wrote %d", batch.EventCount, eventCount)
	}
	storageBackend := strings.TrimSpace(req.StorageBackend)
	if storageBackend == "" {
		storageBackend = "local"
	}
	artifact, err := s.CreateAPIUsageExportArtifact(ctx, maildb.CreateAPIUsageExportArtifactRequest{
		BatchID:        batch.ID,
		StorageBackend: storageBackend,
		ObjectKey:      result.ObjectKey,
		ContentType:    result.ContentType,
		ByteCount:      result.ByteCount,
		SHA256Hex:      result.SHA256Hex,
		EventCount:     eventCount,
		Metadata:       result.Metadata,
	})
	if err != nil {
		s.cleanupAPIUsageExportArtifactObject(ctx, result.ObjectKey)
		return maildb.APIUsageExportArtifactView{}, err
	}
	return artifact, nil
}

func (s adminService) cleanupAPIUsageExportArtifactObject(ctx context.Context, objectKey string) {
	if deleter, ok := s.exportStore.(interface {
		Delete(context.Context, string) error
	}); ok {
		_ = deleter.Delete(ctx, objectKey)
	}
}

func (s adminService) OpenAPIUsageExportArtifact(ctx context.Context, batchID string, artifactID string) (maildb.APIUsageExportArtifactView, io.ReadCloser, error) {
	if s.Repository == nil {
		return maildb.APIUsageExportArtifactView{}, nil, fmt.Errorf("repository is required")
	}
	getter, ok := s.exportStore.(interface {
		Get(context.Context, string) (io.ReadCloser, error)
	})
	if !ok {
		return maildb.APIUsageExportArtifactView{}, nil, fmt.Errorf("api usage export artifact store does not support reads")
	}
	artifact, err := s.GetAPIUsageExportArtifact(ctx, batchID, artifactID)
	if err != nil {
		return maildb.APIUsageExportArtifactView{}, nil, err
	}
	body, err := getter.Get(ctx, artifact.ObjectKey)
	if err != nil {
		return maildb.APIUsageExportArtifactView{}, nil, err
	}
	return artifact, body, nil
}

func (s adminService) VerifyAPIUsageExportArtifact(ctx context.Context, batchID string, artifactID string) (maildb.APIUsageExportArtifactVerificationView, error) {
	artifact, body, err := s.OpenAPIUsageExportArtifact(ctx, batchID, artifactID)
	if err != nil {
		return maildb.APIUsageExportArtifactVerificationView{}, err
	}
	defer body.Close()

	hash := sha256.New()
	byteCount, err := io.Copy(hash, body)
	if err != nil {
		return maildb.APIUsageExportArtifactVerificationView{}, fmt.Errorf("read api usage export artifact: %w", err)
	}
	actual := fmt.Sprintf("%x", hash.Sum(nil))
	return maildb.APIUsageExportArtifactVerificationView{
		BatchID:           artifact.BatchID,
		ArtifactID:        artifact.ID,
		ObjectKey:         artifact.ObjectKey,
		ExpectedByteCount: artifact.ByteCount,
		ActualByteCount:   byteCount,
		ExpectedSHA256Hex: artifact.SHA256Hex,
		ActualSHA256Hex:   actual,
		Valid:             artifact.ByteCount == byteCount && artifact.SHA256Hex == actual,
	}, nil
}

func (s adminService) CreateAPIUsageExportManifestSignature(ctx context.Context, batchID string, digestID string) (maildb.APIUsageExportManifestSignatureView, error) {
	if s.Repository == nil {
		return maildb.APIUsageExportManifestSignatureView{}, fmt.Errorf("repository is required")
	}
	if s.exportManifestSigner == nil {
		return maildb.APIUsageExportManifestSignatureView{}, fmt.Errorf("api usage export manifest signer is not configured")
	}
	digest, err := s.GetAPIUsageExportManifestDigest(ctx, batchID, digestID)
	if err != nil {
		return maildb.APIUsageExportManifestSignatureView{}, err
	}
	signature, err := s.exportManifestSigner.SignExportManifestDigest(digest.DigestHex)
	if err != nil {
		return maildb.APIUsageExportManifestSignatureView{}, err
	}
	backend := strings.TrimSpace(s.exportManifestSignerBackend)
	if backend == "" {
		backend = "local-hmac"
	}
	return s.Repository.CreateAPIUsageExportManifestSignature(ctx, maildb.CreateAPIUsageExportManifestSignatureRequest{
		BatchID:       batchID,
		DigestID:      digestID,
		SignerBackend: backend,
		Signature:     signature,
	})
}

func (s adminService) VerifyAPIUsageExportManifestSignature(ctx context.Context, batchID string, digestID string, signatureID string) (maildb.APIUsageExportManifestSignatureVerificationView, error) {
	if len(s.exportManifestVerifySecret) == 0 {
		return maildb.APIUsageExportManifestSignatureVerificationView{}, fmt.Errorf("api usage export manifest verification secret is not configured")
	}
	digest, err := s.GetAPIUsageExportManifestDigest(ctx, batchID, digestID)
	if err != nil {
		return maildb.APIUsageExportManifestSignatureVerificationView{}, err
	}
	signature, err := s.GetAPIUsageExportManifestSignature(ctx, batchID, digestID, signatureID)
	if err != nil {
		return maildb.APIUsageExportManifestSignatureVerificationView{}, err
	}
	valid, err := apimeter.VerifyExportManifestSignature(apimeter.ExportManifestSignature{
		Algorithm:       signature.SignatureAlgorithm,
		KeyID:           signature.KeyID,
		SignedDigestHex: signature.SignedDigestHex,
		SignatureHex:    signature.SignatureHex,
	}, s.exportManifestVerifySecret)
	if err != nil {
		return maildb.APIUsageExportManifestSignatureVerificationView{}, err
	}
	valid = valid && signature.SignedDigestHex == digest.DigestHex
	return maildb.APIUsageExportManifestSignatureVerificationView{
		BatchID:            signature.BatchID,
		DigestID:           signature.DigestID,
		SignatureID:        signature.ID,
		SignerBackend:      signature.SignerBackend,
		KeyID:              signature.KeyID,
		SignatureAlgorithm: signature.SignatureAlgorithm,
		SignedDigestHex:    signature.SignedDigestHex,
		ExpectedDigestHex:  digest.DigestHex,
		Valid:              valid,
	}, nil
}

func apiUsageLedgerRequestFromBatch(batch maildb.APIUsageExportBatchView, limit int) maildb.APIUsageLedgerListRequest {
	req := maildb.APIUsageLedgerListRequest{
		Limit:       limit,
		TenantID:    batch.TenantID,
		PrincipalID: batch.PrincipalID,
	}
	if batch.WindowStart != nil {
		req.From = batch.WindowStart.UTC()
	}
	if batch.WindowEnd != nil {
		req.To = batch.WindowEnd.UTC()
	}
	return req
}
