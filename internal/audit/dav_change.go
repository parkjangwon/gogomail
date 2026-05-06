package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gogomail/gogomail/internal/eventstream"
)

const (
	DAVChangeSchemaVersion = "2026-05-06.dav-change.v1"

	DAVChangeEventCalendar = "calendar.changed"
	DAVChangeEventContacts = "contacts.changed"
)

type DAVChangeHandler struct {
	repository Repository
}

func NewDAVChangeHandler(repository Repository) *DAVChangeHandler {
	return &DAVChangeHandler{repository: repository}
}

func (h *DAVChangeHandler) HandleEvent(ctx context.Context, msg eventstream.Message) error {
	if h.repository == nil {
		return fmt.Errorf("audit repository is required")
	}
	log, err := DAVChangeAuditLog(msg.Payload)
	if err != nil {
		return err
	}
	return h.repository.Insert(ctx, log)
}

func DAVChangeAuditLog(payload json.RawMessage) (Log, error) {
	var event struct {
		Event         string `json:"event"`
		SchemaVersion string `json:"schema_version"`
		DAVKind       string `json:"dav_kind"`
		Action        string `json:"action"`
		UserID        string `json:"user_id"`
		OwnerUserID   string `json:"owner_user_id"`
		ActorUserID   string `json:"actor_user_id"`
		Delegated     bool   `json:"delegated"`
		CollectionID  string `json:"collection_id"`
		ObjectName    string `json:"object_name"`
		ETag          string `json:"etag"`
		SyncToken     string `json:"sync_token"`
		ChangedAt     string `json:"changed_at"`
	}
	if err := json.Unmarshal(payload, &event); err != nil {
		return Log{}, fmt.Errorf("decode DAV change audit payload: %w", err)
	}
	event.Event = strings.TrimSpace(event.Event)
	targetType, err := davChangeTargetType(event.Event)
	if err != nil {
		return Log{}, err
	}
	event.SchemaVersion = strings.TrimSpace(event.SchemaVersion)
	if event.SchemaVersion != DAVChangeSchemaVersion {
		return Log{}, fmt.Errorf("unsupported DAV change audit schema_version %q", event.SchemaVersion)
	}
	if event.DAVKind, err = requiredDAVChangeValue("dav_kind", event.DAVKind); err != nil {
		return Log{}, err
	}
	if event.Action, err = requiredDAVChangeValue("action", event.Action); err != nil {
		return Log{}, err
	}
	if event.UserID, err = requiredDAVChangeValue("user_id", event.UserID); err != nil {
		return Log{}, err
	}
	if event.OwnerUserID, err = optionalDAVChangeValue("owner_user_id", event.OwnerUserID); err != nil {
		return Log{}, err
	}
	if event.OwnerUserID == "" {
		event.OwnerUserID = event.UserID
	}
	if event.ActorUserID, err = optionalDAVChangeValue("actor_user_id", event.ActorUserID); err != nil {
		return Log{}, err
	}
	if event.ActorUserID == "" {
		event.ActorUserID = event.UserID
	}
	if event.CollectionID, err = requiredDAVChangeValue("collection_id", event.CollectionID); err != nil {
		return Log{}, err
	}
	if event.SyncToken, err = requiredDAVChangeValue("sync_token", event.SyncToken); err != nil {
		return Log{}, err
	}
	if event.ChangedAt, err = requiredDAVChangeValue("changed_at", event.ChangedAt); err != nil {
		return Log{}, err
	}
	if event.ObjectName, err = optionalDAVChangeValue("object_name", event.ObjectName); err != nil {
		return Log{}, err
	}
	if event.ETag, err = optionalDAVChangeValue("etag", event.ETag); err != nil {
		return Log{}, err
	}

	detail, err := json.Marshal(map[string]any{
		"dav_kind":      event.DAVKind,
		"action":        event.Action,
		"owner_user_id": event.OwnerUserID,
		"actor_user_id": event.ActorUserID,
		"delegated":     event.Delegated,
		"collection_id": event.CollectionID,
		"object_name":   event.ObjectName,
		"etag":          event.ETag,
		"sync_token":    event.SyncToken,
		"changed_at":    event.ChangedAt,
	})
	if err != nil {
		return Log{}, fmt.Errorf("marshal DAV change audit detail: %w", err)
	}

	return Log{
		UserID:     event.OwnerUserID,
		ActorID:    event.ActorUserID,
		Category:   "dav",
		Action:     event.Event,
		TargetType: targetType,
		TargetID:   event.CollectionID,
		Result:     "success",
		Detail:     detail,
	}, nil
}

func davChangeTargetType(event string) (string, error) {
	switch event {
	case DAVChangeEventCalendar:
		return "calendar", nil
	case DAVChangeEventContacts:
		return "addressbook", nil
	default:
		return "", fmt.Errorf("unexpected DAV change audit event %q", event)
	}
}

func requiredDAVChangeValue(name string, value string) (string, error) {
	value, err := optionalDAVChangeValue(name, value)
	if err != nil {
		return "", err
	}
	if value == "" {
		return "", fmt.Errorf("DAV change audit payload is missing %s", name)
	}
	return value, nil
}

func optionalDAVChangeValue(name string, value string) (string, error) {
	value = strings.TrimSpace(value)
	if strings.ContainsAny(value, "\r\n") {
		return "", fmt.Errorf("DAV change audit payload has invalid %s", name)
	}
	if len(value) > maxAuditScalarBytes {
		return "", fmt.Errorf("DAV change audit payload has oversized %s", name)
	}
	return value, nil
}
