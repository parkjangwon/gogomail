package app

import (
	"context"
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
	backpressure backpressureStore
	exportStore  apimeter.ExportArtifactStore
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
	if batch.EventCount > maildb.MessageListMaxLimit {
		return maildb.APIUsageExportArtifactView{}, fmt.Errorf("api usage export batch has %d events; maximum single artifact write is %d", batch.EventCount, maildb.MessageListMaxLimit)
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

	ledgerReq := apiUsageLedgerRequestFromBatch(batch, maildb.MessageListMaxLimit)
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
	if batch.EventCount > eventCount {
		return maildb.APIUsageExportArtifactView{}, fmt.Errorf("api usage export batch has %d events but only %d were written; paginated artifact writing is required", batch.EventCount, eventCount)
	}
	storageBackend := strings.TrimSpace(req.StorageBackend)
	if storageBackend == "" {
		storageBackend = "local"
	}
	return s.CreateAPIUsageExportArtifact(ctx, maildb.CreateAPIUsageExportArtifactRequest{
		BatchID:        batch.ID,
		StorageBackend: storageBackend,
		ObjectKey:      result.ObjectKey,
		ContentType:    result.ContentType,
		ByteCount:      result.ByteCount,
		SHA256Hex:      result.SHA256Hex,
		EventCount:     eventCount,
		Metadata:       result.Metadata,
	})
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
