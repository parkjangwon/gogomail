package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gogomail/gogomail/internal/audit"
	"github.com/gogomail/gogomail/internal/httpapi"
)

type drivePublicShareAuditRecorder struct {
	audit auditWriter
}

func (r drivePublicShareAuditRecorder) RecordDrivePublicShareAccess(ctx context.Context, event httpapi.DrivePublicShareAccessEvent) error {
	if r.audit == nil {
		return nil
	}
	detail, err := drivePublicShareAuditDetail(event)
	if err != nil {
		return err
	}
	return r.audit.Insert(ctx, audit.Log{
		CompanyID:  event.CompanyID,
		DomainID:   event.DomainID,
		UserID:     event.UserID,
		Category:   "drive",
		Action:     "share_link." + event.Action,
		TargetType: "drive_share_link",
		TargetID:   event.LinkID,
		IPAddress:  event.RemoteAddr,
		UserAgent:  event.UserAgent,
		Result:     event.Result,
		Detail:     detail,
	})
}

func drivePublicShareAuditDetail(event httpapi.DrivePublicShareAccessEvent) (json.RawMessage, error) {
	detail := struct {
		NodeID      string `json:"node_id,omitempty"`
		Permission  string `json:"permission,omitempty"`
		TokenSuffix string `json:"token_suffix,omitempty"`
		Status      int    `json:"status,omitempty"`
		Range       string `json:"range,omitempty"`
	}{
		NodeID:      event.NodeID,
		Permission:  event.Permission,
		TokenSuffix: event.TokenSuffix,
		Status:      event.Status,
		Range:       event.Range,
	}
	raw, err := json.Marshal(detail)
	if err != nil {
		return nil, fmt.Errorf("marshal drive public share audit detail: %w", err)
	}
	return raw, nil
}
