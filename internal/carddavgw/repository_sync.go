package carddavgw

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

func (r *Repository) ListAddressBookChangesSince(ctx context.Context, req ListAddressBookChangesSinceRequest) ([]AddressBookChange, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	req, err := ValidateListAddressBookChangesSinceRequest(req)
	if err != nil {
		return nil, err
	}
	const query = `
WITH marker AS (
  SELECT id
  FROM carddav_addressbook_changes
  WHERE user_id = $1::uuid
    AND addressbook_id = $2::uuid
    AND sync_token = $3
)
SELECT c.id, c.user_id::text, c.addressbook_id::text, c.object_name, c.etag, c.action, c.sync_token, c.changed_at
FROM carddav_addressbook_changes c
JOIN marker m ON c.id > m.id
WHERE c.user_id = $1::uuid
  AND c.addressbook_id = $2::uuid
ORDER BY c.id ASC
LIMIT $4`
	rows, err := r.db.QueryContext(ctx, query, req.UserID, req.AddressBookID, req.SyncToken, req.Limit)
	if err != nil {
		return nil, fmt.Errorf("list CardDAV sync changes: %w", err)
	}
	defer rows.Close()
	var changes []AddressBookChange
	for rows.Next() {
		var change AddressBookChange
		if err := rows.Scan(&change.ID, &change.UserID, &change.AddressBookID, &change.ObjectName, &change.ETag, &change.Action, &change.SyncToken, &change.ChangedAt); err != nil {
			return nil, fmt.Errorf("scan CardDAV sync change: %w", err)
		}
		changes = append(changes, change)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate CardDAV sync changes: %w", err)
	}
	if len(changes) == 0 {
		var markerExists bool
		err := r.db.QueryRowContext(ctx, `
SELECT EXISTS (
  SELECT 1
  FROM carddav_addressbook_changes
  WHERE user_id = $1::uuid
    AND addressbook_id = $2::uuid
    AND sync_token = $3
)`, req.UserID, req.AddressBookID, req.SyncToken).Scan(&markerExists)
		if err != nil {
			return nil, fmt.Errorf("check CardDAV sync marker: %w", err)
		}
		if !markerExists {
			return nil, InvalidSyncTokenError{Token: req.SyncToken}
		}
	}
	return changes, nil
}

func (r *Repository) ListAddressBookChangesWithObjectsSince(ctx context.Context, req ListAddressBookChangesSinceRequest, includeVCard bool) ([]AddressBookChangeWithObject, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	req, err := ValidateListAddressBookChangesSinceRequest(req)
	if err != nil {
		return nil, err
	}
	const markerQuery = `
SELECT id
FROM carddav_addressbook_changes
WHERE user_id = $1::uuid
  AND addressbook_id = $2::uuid
  AND sync_token = $3`
	var markerID int64
	if err := r.db.QueryRowContext(ctx, markerQuery, req.UserID, req.AddressBookID, req.SyncToken).Scan(&markerID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, InvalidSyncTokenError{Token: req.SyncToken}
		}
		return nil, fmt.Errorf("get CardDAV sync marker: %w", err)
	}

	vcardExpr := "NULL::text AS object_vcard"
	photoDataExpr := "NULL::bytea AS object_photo_data"
	photoMediaTypeExpr := "NULL::text AS object_photo_media_type"
	categoriesExpr := "NULL::text[] AS object_categories_list"
	groupExpr := "NULL::text AS object_group_name"
	if includeVCard {
		vcardExpr = "o.vcard AS object_vcard"
		photoDataExpr = "o.photo_data AS object_photo_data"
		photoMediaTypeExpr = "o.photo_media_type AS object_photo_media_type"
		categoriesExpr = "o.categories_list AS object_categories_list"
		groupExpr = "o.group_name AS object_group_name"
	}
	query := `
SELECT
  c.id,
  c.user_id::text,
  c.addressbook_id::text,
  c.object_name,
  c.etag,
  c.action,
  c.sync_token,
  c.changed_at,
  o.id::text AS object_id,
  o.user_id::text AS object_user_id,
  o.addressbook_id::text AS object_addressbook_id,
  o.object_name AS object_object_name,
  o.uid AS object_uid,
  o.etag AS object_etag,
  o.size AS object_size,
  ` + vcardExpr + `,
  ` + photoDataExpr + `,
  ` + photoMediaTypeExpr + `,
  ` + categoriesExpr + `,
  ` + groupExpr + `,
  o.created_at AS object_created_at,
  o.updated_at AS object_updated_at
FROM carddav_addressbook_changes c
LEFT JOIN carddav_contact_objects o
  ON o.user_id = c.user_id
 AND o.addressbook_id = c.addressbook_id
 AND o.object_name = c.object_name
 AND o.status = 'active'
WHERE c.user_id = $1::uuid
  AND c.addressbook_id = $2::uuid
  AND c.id > $3
ORDER BY c.id ASC
LIMIT $4`
	rows, err := r.db.QueryContext(ctx, query, req.UserID, req.AddressBookID, markerID, req.Limit)
	if err != nil {
		return nil, fmt.Errorf("list CardDAV sync changes with objects: %w", err)
	}
	defer rows.Close()

	changes := make([]AddressBookChangeWithObject, 0, req.Limit)
	for rows.Next() {
		var item AddressBookChangeWithObject
		var (
			objectID             sql.NullString
			objectUserID         sql.NullString
			objectAddressBookID  sql.NullString
			objectObjectName     sql.NullString
			objectUID            sql.NullString
			objectETag           sql.NullString
			objectSize           sql.NullInt64
			objectVCard          sql.NullString
			objectPhotoData      interface{}
			objectPhotoMediaType sql.NullString
			objectCategoriesList interface{}
			objectGroupName      sql.NullString
			objectCreatedAt      sql.NullTime
			objectUpdatedAt      sql.NullTime
		)
		if err := rows.Scan(
			&item.Change.ID,
			&item.Change.UserID,
			&item.Change.AddressBookID,
			&item.Change.ObjectName,
			&item.Change.ETag,
			&item.Change.Action,
			&item.Change.SyncToken,
			&item.Change.ChangedAt,
			&objectID,
			&objectUserID,
			&objectAddressBookID,
			&objectObjectName,
			&objectUID,
			&objectETag,
			&objectSize,
			&objectVCard,
			&objectPhotoData,
			&objectPhotoMediaType,
			&objectCategoriesList,
			&objectGroupName,
			&objectCreatedAt,
			&objectUpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan CardDAV sync change with object: %w", err)
		}
		if objectID.Valid {
			item.HasObject = true
			photoData := []byte(nil)
			if objectPhotoData != nil {
				photoData = objectPhotoData.([]byte)
			}
			categories := []string(nil)
			if objectCategoriesList != nil {
				categories = objectCategoriesList.([]string)
			}
			item.Object = ContactObject{
				ID:             objectID.String,
				UserID:         objectUserID.String,
				AddressBookID:  objectAddressBookID.String,
				ObjectName:     objectObjectName.String,
				UID:            objectUID.String,
				ETag:           objectETag.String,
				Size:           objectSize.Int64,
				VCard:          []byte(objectVCard.String),
				PhotoData:      photoData,
				PhotoMediaType: objectPhotoMediaType.String,
				Categories:     categories,
				Group:          objectGroupName.String,
				CreatedAt:      objectCreatedAt.Time,
				UpdatedAt:      objectUpdatedAt.Time,
			}
			item.Object.VCard = mergePhotoIntoVCard(item.Object.VCard, item.Object.PhotoData, item.Object.PhotoMediaType)
			item.Object.VCard = mergeCategoriesAndGroupIntoVCard(item.Object.VCard, item.Object.Categories, item.Object.Group)
		}
		changes = append(changes, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate CardDAV sync changes with objects: %w", err)
	}
	return changes, nil
}

func (r *Repository) PruneAddressBookChanges(ctx context.Context, req PruneAddressBookChangesRequest) (AddressBookChangePruneResult, error) {
	if r == nil || r.db == nil {
		return AddressBookChangePruneResult{}, fmt.Errorf("database handle is required")
	}
	req, err := ValidatePruneAddressBookChangesRequest(req)
	if err != nil {
		return AddressBookChangePruneResult{}, err
	}
	result := AddressBookChangePruneResult{
		Cutoff:        req.Cutoff,
		UserID:        req.UserID,
		AddressBookID: req.AddressBookID,
		Limit:         req.Limit,
		DryRun:        req.DryRun,
	}
	if req.DryRun {
		query, args := buildPruneAddressBookChangesSQL(req, true)
		if err := r.db.QueryRowContext(ctx, query, args...).Scan(&result.CandidateCount); err != nil {
			return AddressBookChangePruneResult{}, fmt.Errorf("check CardDAV sync change prune candidates: %w", err)
		}
		return result, nil
	}
	query, args := buildPruneAddressBookChangesSQL(req, false)
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&result.CandidateCount, &result.DeletedCount); err != nil {
		return AddressBookChangePruneResult{}, fmt.Errorf("prune CardDAV sync changes: %w", err)
	}
	return result, nil
}

func buildPruneAddressBookChangesSQL(req PruneAddressBookChangesRequest, dryRun bool) (string, []any) {
	args := []any{req.Cutoff}
	where := []string{"c.changed_at < $1"}
	if req.UserID != "" {
		args = append(args, req.UserID)
		where = append(where, fmt.Sprintf("c.user_id = $%d::uuid", len(args)))
	}
	if req.AddressBookID != "" {
		args = append(args, req.AddressBookID)
		where = append(where, fmt.Sprintf("c.addressbook_id = $%d::uuid", len(args)))
	}
	args = append(args, req.Limit)
	limitParam := len(args)

	query := fmt.Sprintf(`
WITH candidates AS (
  SELECT c.id
  FROM carddav_addressbook_changes c
  WHERE %s
    AND EXISTS (
      SELECT 1
      FROM carddav_addressbook_changes newer
      WHERE newer.addressbook_id = c.addressbook_id
        AND newer.id > c.id
    )
    AND NOT EXISTS (
      SELECT 1
      FROM carddav_addressbooks a
      WHERE a.id = c.addressbook_id
        AND a.sync_token = c.sync_token
    )
  ORDER BY c.id ASC
  LIMIT $%d
)`, strings.Join(where, "\n    AND "), limitParam)
	if dryRun {
		return query + `
SELECT count(*)::bigint FROM candidates`, args
	}
	return query + `,
deleted AS (
  DELETE FROM carddav_addressbook_changes c
  USING candidates
  WHERE c.id = candidates.id
  RETURNING c.id
)
SELECT (SELECT count(*)::bigint FROM candidates), (SELECT count(*)::bigint FROM deleted)`, args
}

type addressBookChangeExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

const (
	contactsChangedEvent       = "contacts.changed"
	davChangeOutboxTopic       = "dav.event"
	davChangeSchemaVersion     = "2026-05-06.dav-change.v1"
	davChangeKindCardDAV       = "carddav"
	davChangePartitionFallback = "unknown"
)

func insertAddressBookChange(ctx context.Context, execer addressBookChangeExecer, userID string, actorUserID string, addressBookID string, syncToken string, action string, objectName string, etag string) error {
	_, err := execer.ExecContext(ctx, `
INSERT INTO carddav_addressbook_changes (
  user_id, addressbook_id, sync_token, action, object_name, etag
) VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6)`, userID, addressBookID, syncToken, action, objectName, etag)
	if err != nil {
		return fmt.Errorf("insert CardDAV address book change: %w", err)
	}
	if err := insertAddressBookChangeOutbox(ctx, execer, userID, actorUserID, addressBookID, syncToken, action, objectName, etag); err != nil {
		return err
	}
	return nil
}

func insertAddressBookChangeOutbox(ctx context.Context, execer addressBookChangeExecer, userID string, actorUserID string, addressBookID string, syncToken string, action string, objectName string, etag string) error {
	ownerUserID := strings.TrimSpace(userID)
	actorUserID = strings.TrimSpace(actorUserID)
	if actorUserID == "" {
		actorUserID = ownerUserID
	}
	payload, err := json.Marshal(map[string]any{
		"event":          contactsChangedEvent,
		"schema_version": davChangeSchemaVersion,
		"dav_kind":       davChangeKindCardDAV,
		"action":         action,
		"user_id":        ownerUserID,
		"owner_user_id":  ownerUserID,
		"actor_user_id":  actorUserID,
		"delegated":      actorUserID != "" && actorUserID != ownerUserID,
		"collection_id":  addressBookID,
		"object_name":    objectName,
		"etag":           etag,
		"sync_token":     syncToken,
		"changed_at":     time.Now().UTC(),
	})
	if err != nil {
		return fmt.Errorf("marshal CardDAV change event: %w", err)
	}
	partitionKey := ownerUserID
	if partitionKey == "" {
		partitionKey = davChangePartitionFallback
	}
	_, err = execer.ExecContext(ctx, `
INSERT INTO outbox (topic, partition_key, payload, status)
VALUES ($1, $2, $3::jsonb, 'pending')`, davChangeOutboxTopic, partitionKey, string(payload))
	if err != nil {
		return fmt.Errorf("insert CardDAV change outbox event: %w", err)
	}
	return nil
}

func ValidateListAddressBookChangesSinceRequest(req ListAddressBookChangesSinceRequest) (ListAddressBookChangesSinceRequest, error) {
	userID, err := validateCardDAVID("user_id", req.UserID, true)
	if err != nil {
		return ListAddressBookChangesSinceRequest{}, err
	}
	bookID, err := validateCardDAVID("addressbook_id", req.AddressBookID, true)
	if err != nil {
		return ListAddressBookChangesSinceRequest{}, err
	}
	syncToken := strings.TrimSpace(req.SyncToken)
	if syncToken == "" {
		return ListAddressBookChangesSinceRequest{}, fmt.Errorf("sync token is required")
	}
	if len(syncToken) > 128 || strings.ContainsAny(syncToken, "\r\n") {
		return ListAddressBookChangesSinceRequest{}, fmt.Errorf("sync token is invalid")
	}
	return ListAddressBookChangesSinceRequest{UserID: userID, AddressBookID: bookID, SyncToken: syncToken, Limit: normalizeCardDAVChangeLimit(req.Limit)}, nil
}

func ValidatePruneAddressBookChangesRequest(req PruneAddressBookChangesRequest) (PruneAddressBookChangesRequest, error) {
	if req.Cutoff.IsZero() {
		return PruneAddressBookChangesRequest{}, fmt.Errorf("cutoff is required")
	}
	cutoff := req.Cutoff.UTC()
	if cutoff.After(time.Now().UTC()) {
		return PruneAddressBookChangesRequest{}, fmt.Errorf("cutoff must not be in the future")
	}
	userID, err := validateCardDAVID("user_id", req.UserID, false)
	if err != nil {
		return PruneAddressBookChangesRequest{}, err
	}
	bookID, err := validateCardDAVID("addressbook_id", req.AddressBookID, false)
	if err != nil {
		return PruneAddressBookChangesRequest{}, err
	}
	return PruneAddressBookChangesRequest{
		Cutoff:        cutoff,
		UserID:        userID,
		AddressBookID: bookID,
		Limit:         normalizeCardDAVChangeLimit(req.Limit),
		DryRun:        req.DryRun,
	}, nil
}
