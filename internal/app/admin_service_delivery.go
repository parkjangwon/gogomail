package app

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gogomail/gogomail/internal/apimeter"
	"github.com/gogomail/gogomail/internal/audit"
	"github.com/gogomail/gogomail/internal/backpressure"
	"github.com/gogomail/gogomail/internal/caldavgw"
	"github.com/gogomail/gogomail/internal/carddavgw"
	"github.com/gogomail/gogomail/internal/davsyncretention"
	"github.com/gogomail/gogomail/internal/maildb"
	"github.com/gogomail/gogomail/internal/mailflow"
)

const maxBackpressureAuditReasonBytes = 512

const apiUsageExportLocalEd25519Backend = "local-ed25519"
const apiUsageExportLocalHMACBackend = "local-hmac"

// backpressureAuditState is used for audit log detail.
type backpressureAuditState struct {
	Level     string     `json:"level"`
	Reason    string     `json:"reason,omitempty"`
	Until     *time.Time `json:"until,omitempty"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
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
	previous, err := s.backpressure.State(ctx)
	if err != nil {
		return backpressure.State{}, fmt.Errorf("read previous backpressure state: %w", err)
	}
	state, err := s.backpressure.SetState(ctx, req)
	if err != nil {
		return backpressure.State{}, err
	}
	if s.audit != nil {
		detail, err := backpressureAuditDetail(previous, state)
		if err != nil {
			return backpressure.State{}, err
		}
		if err := s.audit.Insert(ctx, audit.Log{
			Category:   "admin",
			Action:     "backpressure.update",
			TargetType: "backpressure",
			Result:     "updated",
			Detail:     detail,
		}); err != nil {
			return backpressure.State{}, fmt.Errorf("record backpressure audit: %w", err)
		}
	}
	return state, nil
}

func backpressureAuditDetail(previous backpressure.State, current backpressure.State) (json.RawMessage, error) {
	detail := struct {
		Scope    string                 `json:"scope"`
		Previous backpressureAuditState `json:"previous"`
		Current  backpressureAuditState `json:"current"`
	}{
		Scope:    "smtp",
		Previous: backpressureAuditStateFromState(previous),
		Current:  backpressureAuditStateFromState(current),
	}
	raw, err := json.Marshal(detail)
	if err != nil {
		return nil, fmt.Errorf("marshal backpressure audit detail: %w", err)
	}
	return raw, nil
}

func backpressureAuditStateFromState(state backpressure.State) backpressureAuditState {
	level := strings.TrimSpace(state.Level)
	if level == "" {
		level = "normal"
	}
	reason := strings.TrimSpace(state.Reason)
	reason = truncateBackpressureAuditString(reason, maxBackpressureAuditReasonBytes)
	var updatedAt *time.Time
	if !state.UpdatedAt.IsZero() {
		normalized := state.UpdatedAt.UTC()
		updatedAt = &normalized
	}
	return backpressureAuditState{
		Level:     level,
		Reason:    reason,
		Until:     state.Until,
		UpdatedAt: updatedAt,
	}
}

func truncateBackpressureAuditString(value string, maxBytes int) string {
	if maxBytes <= 0 || len(value) <= maxBytes {
		return value
	}
	value = value[:maxBytes]
	for len(value) > 0 && !utf8.ValidString(value) {
		value = value[:len(value)-1]
	}
	return value
}

func (s adminService) ListDAVSyncRetentionRuns(ctx context.Context, req davsyncretention.RunListRequest) ([]davsyncretention.RunRecord, error) {
	if s.davSyncRetention == nil {
		return nil, fmt.Errorf("DAV sync retention repository is not configured")
	}
	return s.davSyncRetention.ListRuns(ctx, req)
}

func (s adminService) GetDAVSyncRetentionRun(ctx context.Context, id string) (davsyncretention.RunRecord, error) {
	if s.davSyncRetention == nil {
		return davsyncretention.RunRecord{}, fmt.Errorf("DAV sync retention repository is not configured")
	}
	return s.davSyncRetention.GetRun(ctx, id)
}

func (s adminService) GetDAVSyncRetentionReadiness(ctx context.Context, req davsyncretention.ReadinessRequest) (davsyncretention.ReadinessView, error) {
	req, err := davsyncretention.NormalizeReadinessRequest(req, time.Now)
	if err != nil {
		return davsyncretention.ReadinessView{}, err
	}
	if s.calDAVSyncRetention == nil {
		return davsyncretention.ReadinessView{}, fmt.Errorf("CalDAV sync retention repository is not configured")
	}
	if s.cardDAVSyncRetention == nil {
		return davsyncretention.ReadinessView{}, fmt.Errorf("CardDAV sync retention repository is not configured")
	}
	calResult, err := s.calDAVSyncRetention.PruneCalendarSyncChanges(ctx, caldavgw.PruneCalendarSyncChangesRequest{
		Cutoff: req.Cutoff,
		Limit:  req.Limit,
		DryRun: true,
	})
	if err != nil {
		return davsyncretention.ReadinessView{}, err
	}
	cardResult, err := s.cardDAVSyncRetention.PruneAddressBookChanges(ctx, carddavgw.PruneAddressBookChangesRequest{
		Cutoff: req.Cutoff,
		Limit:  req.Limit,
		DryRun: true,
	})
	if err != nil {
		return davsyncretention.ReadinessView{}, err
	}
	candidateCount := calResult.CandidateCount + cardResult.CandidateCount
	truncated := calResult.CandidateCount >= int64(req.Limit) || cardResult.CandidateCount >= int64(req.Limit)
	return davsyncretention.ReadinessView{
		Cutoff:             req.Cutoff,
		Limit:              req.Limit,
		Ready:              !truncated,
		Truncated:          truncated,
		CandidateCount:     candidateCount,
		CalDAVCandidates:   calResult.CandidateCount,
		CardDAVCandidates:  cardResult.CandidateCount,
		DestructiveGuarded: true,
	}, nil
}

func (s adminService) RunDAVSyncRetention(ctx context.Context, req davsyncretention.RunRequest) (davsyncretention.RunRecord, error) {
	req, err := davsyncretention.NormalizeRunRequest(req, time.Now)
	if err != nil {
		return davsyncretention.RunRecord{}, err
	}
	if s.davSyncRetention == nil {
		return davsyncretention.RunRecord{}, fmt.Errorf("DAV sync retention repository is not configured")
	}
	if s.calDAVSyncRetention == nil {
		return davsyncretention.RunRecord{}, fmt.Errorf("CalDAV sync retention repository is not configured")
	}
	if s.cardDAVSyncRetention == nil {
		return davsyncretention.RunRecord{}, fmt.Errorf("CardDAV sync retention repository is not configured")
	}
	if !req.DryRun {
		readiness, err := s.GetDAVSyncRetentionReadiness(ctx, davsyncretention.ReadinessRequest{
			Cutoff: req.Cutoff,
			Limit:  req.Limit,
		})
		if err != nil {
			return davsyncretention.RunRecord{}, err
		}
		if !readiness.Ready {
			record, recordErr := s.davSyncRetention.RecordRun(ctx, davsyncretention.RunRecord{
				Cutoff:            req.Cutoff,
				Limit:             req.Limit,
				DryRun:            req.DryRun,
				ConfirmReady:      req.ConfirmReady,
				Status:            davsyncretention.RunStatusFailed,
				ErrorMessage:      "DAV sync retention readiness is truncated",
				CalDAVCandidates:  readiness.CalDAVCandidates,
				CardDAVCandidates: readiness.CardDAVCandidates,
			})
			return record, errors.Join(fmt.Errorf("DAV sync retention readiness is truncated"), recordErr)
		}
	}
	calResult, err := s.calDAVSyncRetention.PruneCalendarSyncChanges(ctx, caldavgw.PruneCalendarSyncChangesRequest{
		Cutoff: req.Cutoff,
		Limit:  req.Limit,
		DryRun: req.DryRun,
	})
	if err != nil {
		record, recordErr := s.davSyncRetention.RecordRun(ctx, davsyncretention.RunRecord{
			Cutoff:       req.Cutoff,
			Limit:        req.Limit,
			DryRun:       req.DryRun,
			ConfirmReady: req.ConfirmReady,
			Status:       davsyncretention.RunStatusFailed,
			ErrorMessage: err.Error(),
		})
		return record, errors.Join(err, recordErr)
	}
	cardResult, err := s.cardDAVSyncRetention.PruneAddressBookChanges(ctx, carddavgw.PruneAddressBookChangesRequest{
		Cutoff: req.Cutoff,
		Limit:  req.Limit,
		DryRun: req.DryRun,
	})
	if err != nil {
		record, recordErr := s.davSyncRetention.RecordRun(ctx, davsyncretention.RunRecord{
			Cutoff:            req.Cutoff,
			Limit:             req.Limit,
			DryRun:            req.DryRun,
			ConfirmReady:      req.ConfirmReady,
			Status:            davsyncretention.RunStatusFailed,
			ErrorMessage:      err.Error(),
			CalDAVCandidates:  calResult.CandidateCount,
			CalDAVDeleted:     calResult.DeletedCount,
			CardDAVCandidates: cardResult.CandidateCount,
			CardDAVDeleted:    cardResult.DeletedCount,
		})
		return record, errors.Join(err, recordErr)
	}
	return s.davSyncRetention.RecordRun(ctx, davsyncretention.RunRecord{
		Cutoff:            req.Cutoff,
		Limit:             req.Limit,
		DryRun:            req.DryRun,
		ConfirmReady:      req.ConfirmReady,
		Status:            davsyncretention.RunStatusCompleted,
		CalDAVCandidates:  calResult.CandidateCount,
		CalDAVDeleted:     calResult.DeletedCount,
		CardDAVCandidates: cardResult.CandidateCount,
		CardDAVDeleted:    cardResult.DeletedCount,
	})
}

func exportManifestSignerKeyID(signer apimeter.ExportManifestSigner) (string, bool) {
	switch signer := signer.(type) {
	case apimeter.HMACExportManifestSigner:
		return strings.TrimSpace(signer.KeyID), strings.TrimSpace(signer.KeyID) != ""
	case *apimeter.HMACExportManifestSigner:
		if signer == nil {
			return "", false
		}
		return strings.TrimSpace(signer.KeyID), strings.TrimSpace(signer.KeyID) != ""
	case apimeter.Ed25519ExportManifestSigner:
		return strings.TrimSpace(signer.KeyID), strings.TrimSpace(signer.KeyID) != ""
	case *apimeter.Ed25519ExportManifestSigner:
		if signer == nil {
			return "", false
		}
		return strings.TrimSpace(signer.KeyID), strings.TrimSpace(signer.KeyID) != ""
	case apimeter.RemoteEd25519ExportManifestSigner:
		return strings.TrimSpace(signer.KeyID), strings.TrimSpace(signer.KeyID) != ""
	case *apimeter.RemoteEd25519ExportManifestSigner:
		if signer == nil {
			return "", false
		}
		return strings.TrimSpace(signer.KeyID), strings.TrimSpace(signer.KeyID) != ""
	default:
		return "", false
	}
}

func apiUsageExportManifestSignerProductionReady(backend string) bool {
	backend = strings.ToLower(strings.TrimSpace(backend))
	return backend != "" && backend != "disabled" && !apiUsageExportManifestSignerLocalOnly(backend)
}

func apiUsageExportManifestSignerLocalOnly(backend string) bool {
	switch strings.ToLower(strings.TrimSpace(backend)) {
	case apiUsageExportLocalHMACBackend, apiUsageExportLocalEd25519Backend:
		return true
	default:
		return false
	}
}

func (s adminService) GetAPIUsageExportCapabilities(context.Context) (maildb.APIUsageExportCapabilityView, error) {
	backend := strings.TrimSpace(s.exportManifestSignerBackend)
	if backend == "" {
		backend = "disabled"
	}
	productionReady := apiUsageExportManifestSignerProductionReady(backend)
	view := maildb.APIUsageExportCapabilityView{
		ExportFormat:                                "ndjson",
		ArtifactContentType:                         apimeter.ExportArtifactContentTypeNDJSON,
		ManifestDigestAlgorithm:                     "sha256",
		SignerBackend:                               backend,
		SignerConfigured:                            s.exportManifestSigner != nil,
		VerifierConfigured:                          s.exportManifestVerifier != nil,
		ProductionSignatureReady:                    s.exportManifestSigner != nil && s.exportManifestVerifier != nil && productionReady,
		BillingReadySupported:                       s.exportManifestSigner != nil && productionReady,
		VerifiedBillingReadySupported:               s.exportManifestSigner != nil && s.exportManifestVerifier != nil && productionReady,
		RetentionRunsSupported:                      true,
		RetentionWorkerSupported:                    true,
		RetentionWorkerDestructiveRequiresRemoteKey: true,
	}
	if keyID, ok := exportManifestSignerKeyID(s.exportManifestSigner); ok {
		view.SignerKeyID = keyID
	}
	var blocking []string
	if s.exportManifestSigner == nil {
		blocking = append(blocking, "manifest_signer_not_configured")
	}
	if s.exportManifestVerifier == nil {
		blocking = append(blocking, "manifest_signature_verifier_not_configured")
	}
	if apiUsageExportManifestSignerLocalOnly(backend) {
		blocking = append(blocking, "production_manifest_signer_required")
	}
	view.BlockingReasons = uniqueStrings(blocking)
	return view, nil
}

func (s adminService) GetAPIUsageExportHandoff(ctx context.Context, batchID string, deep bool) (maildb.APIUsageExportHandoffView, error) {
	if s.Repository == nil {
		return maildb.APIUsageExportHandoffView{}, fmt.Errorf("repository is required")
	}
	handoff, err := s.Repository.GetAPIUsageExportHandoff(ctx, batchID)
	if err != nil {
		return maildb.APIUsageExportHandoffView{}, err
	}
	if !deep {
		return handoff, nil
	}
	s.applyAPIUsageExportDeepHandoff(ctx, &handoff)
	return handoff, nil
}

func (s adminService) applyAPIUsageExportDeepHandoff(ctx context.Context, handoff *maildb.APIUsageExportHandoffView) {
	handoff.DeepVerification = true
	var blocking []string

	artifacts, err := s.Repository.ListAllAPIUsageExportArtifacts(ctx, handoff.BatchID)
	if err != nil {
		handoff.DeepVerificationErrors = append(handoff.DeepVerificationErrors, fmt.Sprintf("list artifacts: %v", err))
		blocking = append(blocking, "artifact_verification_error")
	} else {
		for _, artifact := range artifacts {
			verification, err := s.VerifyAPIUsageExportArtifact(ctx, handoff.BatchID, artifact.ID)
			if err != nil {
				handoff.DeepVerificationErrors = append(handoff.DeepVerificationErrors, fmt.Sprintf("verify artifact %s: %v", artifact.ID, err))
				blocking = append(blocking, "artifact_verification_error")
				continue
			}
			handoff.ArtifactVerifications = append(handoff.ArtifactVerifications, verification)
			if !verification.Valid {
				blocking = append(blocking, "artifact_verification_failed")
			}
		}
	}

	if handoff.LatestManifestDigestID != "" {
		verification, err := s.Repository.VerifyAPIUsageExportManifestDigest(ctx, handoff.BatchID, handoff.LatestManifestDigestID)
		if err != nil {
			handoff.DeepVerificationErrors = append(handoff.DeepVerificationErrors, fmt.Sprintf("verify manifest digest %s: %v", handoff.LatestManifestDigestID, err))
			blocking = append(blocking, "manifest_digest_verification_error")
		} else {
			handoff.ManifestDigestVerification = &verification
			if !verification.Valid {
				blocking = append(blocking, "manifest_digest_verification_failed")
			}
			coverageValid, err := apiUsageExportManifestCoversArtifacts(verification.CanonicalManifest, artifacts)
			if err != nil {
				handoff.DeepVerificationErrors = append(handoff.DeepVerificationErrors, fmt.Sprintf("verify manifest artifact coverage: %v", err))
				blocking = append(blocking, "manifest_artifact_coverage_error")
			} else {
				handoff.ManifestArtifactCoverageValid = &coverageValid
				if !coverageValid {
					blocking = append(blocking, "manifest_artifact_mismatch")
				}
			}
		}
	}

	if handoff.LatestManifestDigestID != "" && handoff.LatestSignatureID != "" {
		if s.exportManifestVerifier == nil {
			blocking = append(blocking, "manifest_signature_verifier_unavailable")
		} else {
			verification, err := s.VerifyAPIUsageExportManifestSignature(ctx, handoff.BatchID, handoff.LatestManifestDigestID, handoff.LatestSignatureID)
			if err != nil {
				handoff.DeepVerificationErrors = append(handoff.DeepVerificationErrors, fmt.Sprintf("verify manifest signature %s: %v", handoff.LatestSignatureID, err))
				blocking = append(blocking, "manifest_signature_verification_error")
			} else {
				handoff.ManifestSignatureVerification = &verification
				if !verification.Valid {
					blocking = append(blocking, "manifest_signature_verification_failed")
				}
			}
		}
	}

	handoff.DeepBlockingReasons = uniqueStrings(blocking)
	handoff.DeepReady = handoff.Ready && len(handoff.DeepBlockingReasons) == 0
	handoff.VerifiedBillingReady = handoff.BillingReady && handoff.DeepReady
}

func apiUsageExportManifestCoversArtifacts(raw []byte, artifacts []maildb.APIUsageExportArtifactView) (bool, error) {
	var manifest apimeter.ExportManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return false, fmt.Errorf("unmarshal manifest: %w", err)
	}
	current := make([]apimeter.ExportManifestArtifact, 0, len(artifacts))
	for _, artifact := range artifacts {
		current = append(current, apimeter.ExportManifestArtifact{
			ID:             artifact.ID,
			StorageBackend: artifact.StorageBackend,
			ObjectKey:      artifact.ObjectKey,
			ContentType:    artifact.ContentType,
			ByteCount:      artifact.ByteCount,
			SHA256Hex:      artifact.SHA256Hex,
			EventCount:     artifact.EventCount,
		})
	}
	sort.Slice(current, func(i, j int) bool { return current[i].ID < current[j].ID })
	manifestArtifacts := append([]apimeter.ExportManifestArtifact(nil), manifest.Artifacts...)
	sort.Slice(manifestArtifacts, func(i, j int) bool { return manifestArtifacts[i].ID < manifestArtifacts[j].ID })
	if len(current) != len(manifestArtifacts) {
		return false, nil
	}
	for i := range current {
		if current[i] != manifestArtifacts[i] {
			return false, nil
		}
	}
	return true, nil
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	unique := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		unique = append(unique, value)
	}
	return unique
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
		if err := deleter.Delete(ctx, objectKey); err != nil && !errors.Is(err, os.ErrNotExist) {
			s.loggerOrDefault().WarnContext(ctx, "failed to delete api usage export artifact object", "object_key", objectKey, "error", err)
		}
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
		backend = apiUsageExportLocalHMACBackend
	}
	return s.Repository.CreateAPIUsageExportManifestSignature(ctx, maildb.CreateAPIUsageExportManifestSignatureRequest{
		BatchID:       batchID,
		DigestID:      digestID,
		SignerBackend: backend,
		Signature:     signature,
	})
}

func (s adminService) VerifyAPIUsageExportManifestSignature(ctx context.Context, batchID string, digestID string, signatureID string) (maildb.APIUsageExportManifestSignatureVerificationView, error) {
	if s.exportManifestVerifier == nil {
		return maildb.APIUsageExportManifestSignatureVerificationView{}, fmt.Errorf("api usage export manifest signature verifier is not configured")
	}
	digest, err := s.GetAPIUsageExportManifestDigest(ctx, batchID, digestID)
	if err != nil {
		return maildb.APIUsageExportManifestSignatureVerificationView{}, err
	}
	signature, err := s.GetAPIUsageExportManifestSignature(ctx, batchID, digestID, signatureID)
	if err != nil {
		return maildb.APIUsageExportManifestSignatureVerificationView{}, err
	}
	valid, err := s.exportManifestVerifier.VerifyExportManifestSignature(apimeter.ExportManifestSignature{
		Algorithm:       signature.SignatureAlgorithm,
		KeyID:           signature.KeyID,
		SignedDigestHex: signature.SignedDigestHex,
		SignatureHex:    signature.SignatureHex,
	})
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

func (s adminService) GetMailFlowLogStats(ctx context.Context, req maildb.MailFlowLogStatsRequest) (maildb.MailFlowLogStatsView, error) {
	if s.mailFlowStats == nil {
		return maildb.MailFlowLogStatsView{}, fmt.Errorf("mail flow stats provider is not configured")
	}
	mailflowReq := mailflow.MailFlowStatsRequest{
		Direction: req.Direction,
		CompanyID: req.CompanyID,
		DomainID:  req.DomainID,
		UserID:    req.UserID,
		Since:     req.Since,
		Until:     req.Until,
	}
	result, err := s.mailFlowStats.GetStats(ctx, mailflowReq)
	if err != nil {
		return maildb.MailFlowLogStatsView{}, err
	}
	return maildb.MailFlowLogStatsView{
		TotalMessages:    result.TotalMessages,
		UniqueSenders:    result.UniqueSenders,
		UniqueDomains:    result.UniqueDomains,
		TotalSizeBytes:   result.TotalSizeBytes,
		AverageSizeBytes: result.AverageSizeBytes,
		MaxSizeBytes:     result.MaxSizeBytes,
		Delivered:        result.Delivered,
		Failed:           result.Failed,
		Bounced:          result.Bounced,
		Filtered:         result.Filtered,
		Rejected:         result.Rejected,
		DeliveryRate:     result.DeliveryRate,
	}, nil
}

func (s adminService) GetMailFlowLogDailyStats(ctx context.Context, req maildb.MailFlowLogDailyStatsRequest) ([]maildb.MailFlowLogDailyStatsView, error) {
	if s.mailFlowStats == nil {
		return nil, fmt.Errorf("mail flow stats provider is not configured")
	}
	mailflowReq := mailflow.MailFlowStatsRequest{
		Direction: req.Direction,
		CompanyID: req.CompanyID,
		DomainID:  req.DomainID,
		UserID:    req.UserID,
		Since:     req.Since,
		Until:     req.Until,
	}
	results, err := s.mailFlowStats.GetDailyStats(ctx, mailflowReq)
	if err != nil {
		return nil, err
	}
	views := make([]maildb.MailFlowLogDailyStatsView, 0, len(results))
	for _, r := range results {
		views = append(views, maildb.MailFlowLogDailyStatsView{
			Date:             r.Date,
			InboundMessages:  r.InboundMessages,
			OutboundMessages: r.OutboundMessages,
			InboundSize:      r.InboundSize,
			OutboundSize:     r.OutboundSize,
			Delivered:        r.Delivered,
			Failed:           r.Failed,
			Bounced:          r.Bounced,
			Filtered:         r.Filtered,
			Rejected:         r.Rejected,
		})
	}
	return views, nil
}
