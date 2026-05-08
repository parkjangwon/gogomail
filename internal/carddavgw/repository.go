package carddavgw

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

type CreateAddressBookRequest struct {
	UserID      string
	ActorUserID string
	Name        string
	Description string
}

type CreateAddressBookAtPathRequest struct {
	UserID        string
	ActorUserID   string
	AddressBookID string
	Name          string
	Description   string
}

type ListAddressBooksRequest struct {
	UserID string
	Status string
	Limit  int
}

type GetAddressBookRequest struct {
	UserID        string
	AddressBookID string
	Status        string
}

type UpsertContactObjectRequest struct {
	UserID        string
	ActorUserID   string
	AddressBookID string
	ObjectName    string
	UID           string
	VCard         []byte
	ObservedETag  string
}

type ListContactObjectsRequest struct {
	UserID        string
	AddressBookID string
	Status        string
	Limit         int
}

type GetContactObjectRequest struct {
	UserID        string
	AddressBookID string
	ObjectName    string
	Status        string
}

type DeleteContactObjectRequest struct {
	UserID        string
	ActorUserID   string
	AddressBookID string
	ObjectName    string
	ObservedETag  string
}

type DeleteAddressBookRequest struct {
	UserID        string
	ActorUserID   string
	AddressBookID string
	ObservedETag  string
}

type UpdateAddressBookRequest struct {
	UserID        string
	ActorUserID   string
	AddressBookID string
	Name          *string
	Description   *string
	ObservedETag  string
}

type ListAddressBookChangesSinceRequest struct {
	UserID        string
	AddressBookID string
	SyncToken     string
	Limit         int
}

type PruneAddressBookChangesRequest struct {
	Cutoff        time.Time
	UserID        string
	AddressBookID string
	Limit         int
	DryRun        bool
}

type AddressBookChangePruneResult struct {
	Cutoff         time.Time
	UserID         string
	AddressBookID  string
	Limit          int
	DryRun         bool
	CandidateCount int64
	DeletedCount   int64
}

func (r *Repository) CreateAddressBook(ctx context.Context, req CreateAddressBookRequest) (AddressBook, error) {
	if r == nil || r.db == nil {
		return AddressBook{}, fmt.Errorf("database handle is required")
	}
	req, normalizedName, syncToken, err := ValidateCreateAddressBookRequest(req)
	if err != nil {
		return AddressBook{}, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return AddressBook{}, fmt.Errorf("begin CardDAV address book create: %w", err)
	}
	defer tx.Rollback()
	const query = `
WITH active_user AS (
  SELECT u.id AS user_id, d.id AS domain_id, c.id AS company_id
  FROM users u
  JOIN domains d ON d.id = u.domain_id
  JOIN companies c ON c.id = d.company_id
  WHERE u.id = $1::uuid
    AND u.status = 'active'
    AND d.status = 'active'
    AND c.status = 'active'
)
INSERT INTO carddav_addressbooks (
  company_id, domain_id, user_id, name, normalized_name, description, sync_token
)
SELECT company_id, domain_id, user_id, $2, $3, $4, $5
FROM active_user
RETURNING id::text, user_id::text, name, description, sync_token, created_at, updated_at`
	var book AddressBook
	err = tx.QueryRowContext(ctx, query,
		req.UserID,
		req.Name,
		normalizedName,
		req.Description,
		syncToken,
	).Scan(
		&book.ID,
		&book.UserID,
		&book.Name,
		&book.Description,
		&book.SyncToken,
		&book.CreatedAt,
		&book.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AddressBook{}, fmt.Errorf("active user not found")
		}
		return AddressBook{}, fmt.Errorf("create CardDAV address book: %w", err)
	}
	if err := insertAddressBookChange(ctx, tx, book.UserID, req.ActorUserID, book.ID, book.SyncToken, "addressbook-created", "", ""); err != nil {
		return AddressBook{}, err
	}
	if err := tx.Commit(); err != nil {
		return AddressBook{}, fmt.Errorf("commit CardDAV address book create: %w", err)
	}
	return book, nil
}

func (r *Repository) CreateAddressBookAtPath(ctx context.Context, req CreateAddressBookAtPathRequest) (AddressBook, error) {
	if r == nil || r.db == nil {
		return AddressBook{}, fmt.Errorf("database handle is required")
	}
	req, normalizedName, syncToken, err := ValidateCreateAddressBookAtPathRequest(req)
	if err != nil {
		return AddressBook{}, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return AddressBook{}, fmt.Errorf("begin CardDAV address book create: %w", err)
	}
	defer tx.Rollback()
	const query = `
WITH active_user AS (
  SELECT u.id AS user_id, d.id AS domain_id, c.id AS company_id
  FROM users u
  JOIN domains d ON d.id = u.domain_id
  JOIN companies c ON c.id = d.company_id
  WHERE u.id = $1::uuid
    AND u.status = 'active'
    AND d.status = 'active'
    AND c.status = 'active'
)
INSERT INTO carddav_addressbooks (
  id, company_id, domain_id, user_id, name, normalized_name, description, sync_token
)
SELECT $2::uuid, company_id, domain_id, user_id, $3, $4, $5, $6
FROM active_user
RETURNING id::text, user_id::text, name, description, sync_token, created_at, updated_at`
	var book AddressBook
	err = tx.QueryRowContext(ctx, query,
		req.UserID,
		req.AddressBookID,
		req.Name,
		normalizedName,
		req.Description,
		syncToken,
	).Scan(
		&book.ID,
		&book.UserID,
		&book.Name,
		&book.Description,
		&book.SyncToken,
		&book.CreatedAt,
		&book.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AddressBook{}, fmt.Errorf("active user not found")
		}
		return AddressBook{}, fmt.Errorf("create CardDAV address book at path: %w", err)
	}
	if err := insertAddressBookChange(ctx, tx, book.UserID, req.ActorUserID, book.ID, book.SyncToken, "addressbook-created", "", ""); err != nil {
		return AddressBook{}, err
	}
	if err := tx.Commit(); err != nil {
		return AddressBook{}, fmt.Errorf("commit CardDAV address book create: %w", err)
	}
	return book, nil
}

func (r *Repository) ListAddressBooks(ctx context.Context, req ListAddressBooksRequest) ([]AddressBook, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	req, err := ValidateListAddressBooksRequest(req)
	if err != nil {
		return nil, err
	}
	const query = `
SELECT id::text, user_id::text, name, description, sync_token, created_at, updated_at
FROM carddav_addressbooks
WHERE user_id = $1::uuid
  AND status = $2
ORDER BY updated_at DESC, id DESC
LIMIT $3`
	rows, err := r.db.QueryContext(ctx, query, req.UserID, req.Status, req.Limit)
	if err != nil {
		return nil, fmt.Errorf("list CardDAV address books: %w", err)
	}
	defer rows.Close()
	var books []AddressBook
	for rows.Next() {
		var book AddressBook
		if err := rows.Scan(&book.ID, &book.UserID, &book.Name, &book.Description, &book.SyncToken, &book.CreatedAt, &book.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan CardDAV address book: %w", err)
		}
		books = append(books, book)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate CardDAV address books: %w", err)
	}
	return books, nil
}

func (r *Repository) GetAddressBook(ctx context.Context, req GetAddressBookRequest) (AddressBook, error) {
	if r == nil || r.db == nil {
		return AddressBook{}, fmt.Errorf("database handle is required")
	}
	req, err := ValidateGetAddressBookRequest(req)
	if err != nil {
		return AddressBook{}, err
	}
	const query = `
SELECT id::text, user_id::text, name, description, sync_token, created_at, updated_at
FROM carddav_addressbooks
WHERE user_id = $1::uuid
  AND id = $2::uuid
  AND status = $3`
	var book AddressBook
	if err := r.db.QueryRowContext(ctx, query, req.UserID, req.AddressBookID, req.Status).Scan(&book.ID, &book.UserID, &book.Name, &book.Description, &book.SyncToken, &book.CreatedAt, &book.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AddressBook{}, fmt.Errorf("CardDAV address book not found")
		}
		return AddressBook{}, fmt.Errorf("get CardDAV address book: %w", err)
	}
	return book, nil
}

func (r *Repository) UpdateAddressBookProperties(ctx context.Context, req UpdateAddressBookRequest) (AddressBook, error) {
	if r == nil || r.db == nil {
		return AddressBook{}, fmt.Errorf("database handle is required")
	}
	req, normalizedName, syncToken, err := ValidateUpdateAddressBookRequest(req)
	if err != nil {
		return AddressBook{}, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return AddressBook{}, fmt.Errorf("begin CardDAV address book update: %w", err)
	}
	defer tx.Rollback()
	if err := lockActiveAddressBook(ctx, tx, req.UserID, req.AddressBookID); err != nil {
		return AddressBook{}, err
	}
	if req.ObservedETag != "" {
		if err := ensureAddressBookCollectionETag(ctx, tx, req.UserID, req.AddressBookID, req.ObservedETag); err != nil {
			return AddressBook{}, err
		}
	}
	nameValue, nameSet := optionalStringArg(req.Name)
	descriptionValue, descriptionSet := optionalStringArg(req.Description)
	const query = `
UPDATE carddav_addressbooks
SET
  name = CASE WHEN $3 THEN $4 ELSE name END,
  normalized_name = CASE WHEN $3 THEN $5 ELSE normalized_name END,
  description = CASE WHEN $6 THEN $7 ELSE description END,
  sync_token = $8,
  updated_at = now()
WHERE user_id = $1::uuid
  AND id = $2::uuid
  AND status = 'active'
RETURNING id::text, user_id::text, name, description, sync_token, created_at, updated_at`
	var book AddressBook
	err = tx.QueryRowContext(ctx, query,
		req.UserID,
		req.AddressBookID,
		nameSet,
		nameValue,
		normalizedName,
		descriptionSet,
		descriptionValue,
		syncToken,
	).Scan(
		&book.ID,
		&book.UserID,
		&book.Name,
		&book.Description,
		&book.SyncToken,
		&book.CreatedAt,
		&book.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AddressBook{}, fmt.Errorf("CardDAV address book not found")
		}
		return AddressBook{}, fmt.Errorf("update CardDAV address book properties: %w", err)
	}
	if err := insertAddressBookChange(ctx, tx, req.UserID, req.ActorUserID, req.AddressBookID, syncToken, "addressbook-updated", "", ""); err != nil {
		return AddressBook{}, err
	}
	if err := tx.Commit(); err != nil {
		return AddressBook{}, fmt.Errorf("commit CardDAV address book update: %w", err)
	}
	return book, nil
}

func (r *Repository) DeleteAddressBook(ctx context.Context, req DeleteAddressBookRequest) (AddressBook, error) {
	if r == nil || r.db == nil {
		return AddressBook{}, fmt.Errorf("database handle is required")
	}
	req, err := ValidateDeleteAddressBookRequest(req)
	if err != nil {
		return AddressBook{}, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return AddressBook{}, fmt.Errorf("begin CardDAV address book delete: %w", err)
	}
	defer tx.Rollback()
	if req.ObservedETag != "" {
		if err := ensureAddressBookCollectionETag(ctx, tx, req.UserID, req.AddressBookID, req.ObservedETag); err != nil {
			return AddressBook{}, err
		}
	}
	const query = `
UPDATE carddav_addressbooks
SET status = 'deleted', deleted_at = now(), updated_at = now()
WHERE user_id = $1::uuid
  AND id = $2::uuid
  AND status = 'active'
RETURNING id::text, user_id::text, name, description, sync_token, created_at, updated_at`
	var book AddressBook
	err = tx.QueryRowContext(ctx, query, req.UserID, req.AddressBookID).Scan(
		&book.ID,
		&book.UserID,
		&book.Name,
		&book.Description,
		&book.SyncToken,
		&book.CreatedAt,
		&book.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AddressBook{}, fmt.Errorf("CardDAV address book not found")
		}
		return AddressBook{}, fmt.Errorf("delete CardDAV address book: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE carddav_contact_objects
SET status = 'deleted', deleted_at = COALESCE(deleted_at, now()), updated_at = now()
WHERE user_id = $1::uuid
  AND addressbook_id = $2::uuid
  AND status = 'active'`, req.UserID, req.AddressBookID); err != nil {
		return AddressBook{}, fmt.Errorf("delete CardDAV contact objects: %w", err)
	}
	syncToken := AddressBookSyncToken(req.UserID, req.AddressBookID, "addressbook-delete", time.Now().UTC().Format(time.RFC3339Nano))
	if err := insertAddressBookChange(ctx, tx, req.UserID, req.ActorUserID, req.AddressBookID, syncToken, "addressbook-deleted", "", ""); err != nil {
		return AddressBook{}, err
	}
	if err := tx.Commit(); err != nil {
		return AddressBook{}, fmt.Errorf("commit CardDAV address book delete: %w", err)
	}
	return book, nil
}

func (r *Repository) UpsertContactObject(ctx context.Context, req UpsertContactObjectRequest) (ContactObject, error) {
	if r == nil || r.db == nil {
		return ContactObject{}, fmt.Errorf("database handle is required")
	}
	req, etag, syncToken, err := ValidateUpsertContactObjectRequest(req)
	if err != nil {
		return ContactObject{}, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return ContactObject{}, fmt.Errorf("begin CardDAV contact upsert: %w", err)
	}
	defer tx.Rollback()
	if err := lockActiveAddressBook(ctx, tx, req.UserID, req.AddressBookID); err != nil {
		return ContactObject{}, err
	}
	if err := ensureAddressBookSyncMarker(ctx, tx, req.UserID, req.AddressBookID); err != nil {
		return ContactObject{}, err
	}
	if req.ObservedETag != "" {
		if err := ensureContactObjectETag(ctx, tx, req.UserID, req.AddressBookID, req.ObjectName, req.ObservedETag); err != nil {
			return ContactObject{}, err
		}
	}
	if err := ensureContactObjectUIDAvailable(ctx, tx, req.UserID, req.AddressBookID, req.ObjectName, req.UID); err != nil {
		return ContactObject{}, err
	}
	const query = `
INSERT INTO carddav_contact_objects (
  user_id, addressbook_id, object_name, uid, etag, size, vcard
) VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7)
ON CONFLICT (addressbook_id, object_name) WHERE status = 'active'
DO UPDATE SET
  uid = EXCLUDED.uid,
  etag = EXCLUDED.etag,
  size = EXCLUDED.size,
  vcard = EXCLUDED.vcard,
  updated_at = now()
RETURNING id::text, user_id::text, addressbook_id::text, object_name, uid, etag, size, vcard, created_at, updated_at`
	var object ContactObject
	err = tx.QueryRowContext(ctx, query,
		req.UserID,
		req.AddressBookID,
		req.ObjectName,
		req.UID,
		etag,
		len(req.VCard),
		string(req.VCard),
	).Scan(
		&object.ID,
		&object.UserID,
		&object.AddressBookID,
		&object.ObjectName,
		&object.UID,
		&object.ETag,
		&object.Size,
		&object.VCard,
		&object.CreatedAt,
		&object.UpdatedAt,
	)
	if err != nil {
		return ContactObject{}, mapContactObjectUpsertError(err)
	}
	if err := updateAddressBookSyncToken(ctx, tx, req.UserID, req.AddressBookID, syncToken); err != nil {
		return ContactObject{}, err
	}
	if err := insertAddressBookChange(ctx, tx, req.UserID, req.ActorUserID, req.AddressBookID, syncToken, "contact-upserted", req.ObjectName, etag); err != nil {
		return ContactObject{}, err
	}
	if err := tx.Commit(); err != nil {
		return ContactObject{}, fmt.Errorf("commit CardDAV contact upsert: %w", err)
	}
	return object, nil
}

func (r *Repository) ListContactObjects(ctx context.Context, req ListContactObjectsRequest) ([]ContactObject, error) {
	req, err := ValidateListContactObjectsRequest(req)
	if err != nil {
		return nil, err
	}
	return r.listContactObjects(ctx, req)
}

func (r *Repository) listContactObjectsForSync(ctx context.Context, req ListContactObjectsRequest) ([]ContactObject, error) {
	req, err := ValidateListContactObjectsForSyncRequest(req)
	if err != nil {
		return nil, err
	}
	return r.listContactObjects(ctx, req)
}

func (r *Repository) listContactObjects(ctx context.Context, req ListContactObjectsRequest) ([]ContactObject, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	const query = `
SELECT id::text, user_id::text, addressbook_id::text, object_name, uid, etag, size, vcard, created_at, updated_at
FROM carddav_contact_objects
WHERE user_id = $1::uuid
  AND addressbook_id = $2::uuid
  AND status = $3
ORDER BY updated_at DESC, id DESC
LIMIT $4`
	rows, err := r.db.QueryContext(ctx, query, req.UserID, req.AddressBookID, req.Status, req.Limit)
	if err != nil {
		return nil, fmt.Errorf("list CardDAV contact objects: %w", err)
	}
	defer rows.Close()
	var objects []ContactObject
	for rows.Next() {
		var object ContactObject
		if err := rows.Scan(&object.ID, &object.UserID, &object.AddressBookID, &object.ObjectName, &object.UID, &object.ETag, &object.Size, &object.VCard, &object.CreatedAt, &object.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan CardDAV contact object: %w", err)
		}
		objects = append(objects, object)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate CardDAV contact objects: %w", err)
	}
	return objects, nil
}

func (r *Repository) GetContactObject(ctx context.Context, req GetContactObjectRequest) (ContactObject, error) {
	if r == nil || r.db == nil {
		return ContactObject{}, fmt.Errorf("database handle is required")
	}
	req, err := ValidateGetContactObjectRequest(req)
	if err != nil {
		return ContactObject{}, err
	}
	const query = `
SELECT id::text, user_id::text, addressbook_id::text, object_name, uid, etag, size, vcard, created_at, updated_at
FROM carddav_contact_objects
WHERE user_id = $1::uuid
  AND addressbook_id = $2::uuid
  AND object_name = $3
  AND status = $4`
	var object ContactObject
	err = r.db.QueryRowContext(ctx, query, req.UserID, req.AddressBookID, req.ObjectName, req.Status).Scan(
		&object.ID,
		&object.UserID,
		&object.AddressBookID,
		&object.ObjectName,
		&object.UID,
		&object.ETag,
		&object.Size,
		&object.VCard,
		&object.CreatedAt,
		&object.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ContactObject{}, fmt.Errorf("CardDAV contact object not found")
		}
		return ContactObject{}, fmt.Errorf("get CardDAV contact object: %w", err)
	}
	return object, nil
}

func (r *Repository) SearchContactsByEmail(ctx context.Context, req SearchContactsByEmailRequest) ([]ContactObject, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	if req.UserID == "" {
		return nil, fmt.Errorf("user id is required")
	}
	if req.Email == "" {
		return nil, fmt.Errorf("email is required")
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}
	const query = `
SELECT c.id::text,
       c.user_id::text,
       c.addressbook_id::text,
       c.object_name,
       c.uid,
       c.etag,
       c.size,
       c.vcard,
       c.created_at,
       c.updated_at
FROM carddav_contact_objects c
JOIN carddav_addressbooks a ON a.id = c.addressbook_id
WHERE a.user_id = $1::uuid
  AND a.status = 'active'
  AND c.status = 'active'
  AND lower(c.vcard::text) LIKE '%' || lower($2) || '%'
ORDER BY c.updated_at DESC
LIMIT $3`
	rows, err := r.db.QueryContext(ctx, query, req.UserID, req.Email, limit)
	if err != nil {
		return nil, fmt.Errorf("search contacts by email: %w", err)
	}
	defer rows.Close()
	var objects []ContactObject
	for rows.Next() {
		var object ContactObject
		if err := rows.Scan(
			&object.ID,
			&object.UserID,
			&object.AddressBookID,
			&object.ObjectName,
			&object.UID,
			&object.ETag,
			&object.Size,
			&object.VCard,
			&object.CreatedAt,
			&object.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan contact object: %w", err)
		}
		objects = append(objects, object)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate contact objects: %w", err)
	}
	return objects, nil
}

func (r *Repository) SearchContacts(ctx context.Context, req SearchContactsRequest) ([]ContactObject, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	if req.UserID == "" {
		return nil, fmt.Errorf("user id is required")
	}
	if req.Query == "" {
		return nil, fmt.Errorf("query is required")
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}
	const query = `
SELECT c.id::text,
       c.user_id::text,
       c.addressbook_id::text,
       c.object_name,
       c.uid,
       c.etag,
       c.size,
       c.vcard,
       c.created_at,
       c.updated_at
FROM carddav_contact_objects c
JOIN carddav_addressbooks a ON a.id = c.addressbook_id
WHERE a.user_id = $1::uuid
  AND a.status = 'active'
  AND c.status = 'active'
  AND (lower(c.vcard::text) LIKE '%' || lower($2) || '%')
ORDER BY c.updated_at DESC
LIMIT $3`
	rows, err := r.db.QueryContext(ctx, query, req.UserID, req.Query, limit)
	if err != nil {
		return nil, fmt.Errorf("search contacts: %w", err)
	}
	defer rows.Close()
	var objects []ContactObject
	for rows.Next() {
		var object ContactObject
		if err := rows.Scan(
			&object.ID,
			&object.UserID,
			&object.AddressBookID,
			&object.ObjectName,
			&object.UID,
			&object.ETag,
			&object.Size,
			&object.VCard,
			&object.CreatedAt,
			&object.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan contact object: %w", err)
		}
		objects = append(objects, object)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate contact objects: %w", err)
	}
	return objects, nil
}

func (r *Repository) DeleteContactObject(ctx context.Context, req DeleteContactObjectRequest) (ContactObject, error) {
	if r == nil || r.db == nil {
		return ContactObject{}, fmt.Errorf("database handle is required")
	}
	req, syncToken, err := ValidateDeleteContactObjectRequest(req)
	if err != nil {
		return ContactObject{}, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return ContactObject{}, fmt.Errorf("begin CardDAV contact delete: %w", err)
	}
	defer tx.Rollback()
	if err := lockActiveAddressBook(ctx, tx, req.UserID, req.AddressBookID); err != nil {
		return ContactObject{}, err
	}
	if err := ensureAddressBookSyncMarker(ctx, tx, req.UserID, req.AddressBookID); err != nil {
		return ContactObject{}, err
	}
	if req.ObservedETag != "" {
		if err := ensureContactObjectETag(ctx, tx, req.UserID, req.AddressBookID, req.ObjectName, req.ObservedETag); err != nil {
			return ContactObject{}, err
		}
	}
	const query = `
UPDATE carddav_contact_objects
SET status = 'deleted', deleted_at = now(), updated_at = now()
WHERE user_id = $1::uuid
  AND addressbook_id = $2::uuid
  AND object_name = $3
  AND status = 'active'
RETURNING id::text, user_id::text, addressbook_id::text, object_name, uid, etag, size, vcard, created_at, updated_at`
	var object ContactObject
	err = tx.QueryRowContext(ctx, query, req.UserID, req.AddressBookID, req.ObjectName).Scan(
		&object.ID,
		&object.UserID,
		&object.AddressBookID,
		&object.ObjectName,
		&object.UID,
		&object.ETag,
		&object.Size,
		&object.VCard,
		&object.CreatedAt,
		&object.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ContactObject{}, fmt.Errorf("CardDAV contact object not found")
		}
		return ContactObject{}, fmt.Errorf("delete CardDAV contact object: %w", err)
	}
	if err := updateAddressBookSyncToken(ctx, tx, req.UserID, req.AddressBookID, syncToken); err != nil {
		return ContactObject{}, err
	}
	if err := insertAddressBookChange(ctx, tx, req.UserID, req.ActorUserID, req.AddressBookID, syncToken, "contact-deleted", req.ObjectName, object.ETag); err != nil {
		return ContactObject{}, err
	}
	if err := tx.Commit(); err != nil {
		return ContactObject{}, fmt.Errorf("commit CardDAV contact delete: %w", err)
	}
	return object, nil
}

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
		const query = `
WITH candidates AS (
  SELECT c.id
  FROM carddav_addressbook_changes c
  WHERE c.changed_at < $1
    AND ($2 = '' OR c.user_id = NULLIF($2, '')::uuid)
    AND ($3 = '' OR c.addressbook_id = NULLIF($3, '')::uuid)
    AND EXISTS (
      SELECT 1
      FROM carddav_addressbook_changes newer
      WHERE newer.addressbook_id = c.addressbook_id
        AND newer.id > c.id
    )
  ORDER BY c.id ASC
  LIMIT $4
)
SELECT count(*)::bigint FROM candidates`
		if err := r.db.QueryRowContext(ctx, query, req.Cutoff, req.UserID, req.AddressBookID, req.Limit).Scan(&result.CandidateCount); err != nil {
			return AddressBookChangePruneResult{}, fmt.Errorf("check CardDAV sync change prune candidates: %w", err)
		}
		return result, nil
	}
	const query = `
WITH candidates AS (
  SELECT c.id
  FROM carddav_addressbook_changes c
  WHERE c.changed_at < $1
    AND ($2 = '' OR c.user_id = NULLIF($2, '')::uuid)
    AND ($3 = '' OR c.addressbook_id = NULLIF($3, '')::uuid)
    AND EXISTS (
      SELECT 1
      FROM carddav_addressbook_changes newer
      WHERE newer.addressbook_id = c.addressbook_id
        AND newer.id > c.id
    )
  ORDER BY c.id ASC
  LIMIT $4
),
deleted AS (
  DELETE FROM carddav_addressbook_changes c
  USING candidates
  WHERE c.id = candidates.id
  RETURNING c.id
)
SELECT (SELECT count(*)::bigint FROM candidates), (SELECT count(*)::bigint FROM deleted)`
	if err := r.db.QueryRowContext(ctx, query, req.Cutoff, req.UserID, req.AddressBookID, req.Limit).Scan(&result.CandidateCount, &result.DeletedCount); err != nil {
		return AddressBookChangePruneResult{}, fmt.Errorf("prune CardDAV sync changes: %w", err)
	}
	return result, nil
}

func ValidateCreateAddressBookRequest(req CreateAddressBookRequest) (CreateAddressBookRequest, string, string, error) {
	userID, err := validateCardDAVID("user_id", req.UserID, true)
	if err != nil {
		return CreateAddressBookRequest{}, "", "", err
	}
	actorUserID, err := validateCardDAVActorUserID(req.ActorUserID, userID)
	if err != nil {
		return CreateAddressBookRequest{}, "", "", err
	}
	name, err := ValidateAddressBookName(req.Name)
	if err != nil {
		return CreateAddressBookRequest{}, "", "", err
	}
	normalizedName, err := NormalizeAddressBookName(name)
	if err != nil {
		return CreateAddressBookRequest{}, "", "", err
	}
	description, err := ValidateAddressBookDescription(req.Description)
	if err != nil {
		return CreateAddressBookRequest{}, "", "", err
	}
	syncToken := AddressBookSyncToken(userID, normalizedName, time.Now().UTC().Format(time.RFC3339Nano))
	return CreateAddressBookRequest{UserID: userID, ActorUserID: actorUserID, Name: name, Description: description}, normalizedName, syncToken, nil
}

func ValidateCreateAddressBookAtPathRequest(req CreateAddressBookAtPathRequest) (CreateAddressBookAtPathRequest, string, string, error) {
	userID, err := validateCardDAVID("user_id", req.UserID, true)
	if err != nil {
		return CreateAddressBookAtPathRequest{}, "", "", err
	}
	bookID, err := ValidateAddressBookPathID(req.AddressBookID)
	if err != nil {
		return CreateAddressBookAtPathRequest{}, "", "", err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = bookID
	}
	create, normalizedName, syncToken, err := ValidateCreateAddressBookRequest(CreateAddressBookRequest{
		UserID:      userID,
		ActorUserID: req.ActorUserID,
		Name:        name,
		Description: req.Description,
	})
	if err != nil {
		return CreateAddressBookAtPathRequest{}, "", "", err
	}
	return CreateAddressBookAtPathRequest{
		UserID:        create.UserID,
		ActorUserID:   create.ActorUserID,
		AddressBookID: bookID,
		Name:          create.Name,
		Description:   create.Description,
	}, normalizedName, syncToken, nil
}

func ValidateListAddressBooksRequest(req ListAddressBooksRequest) (ListAddressBooksRequest, error) {
	userID, err := validateCardDAVID("user_id", req.UserID, true)
	if err != nil {
		return ListAddressBooksRequest{}, err
	}
	status, err := ValidateAddressBookStatus(req.Status)
	if err != nil {
		return ListAddressBooksRequest{}, err
	}
	return ListAddressBooksRequest{UserID: userID, Status: status, Limit: normalizeCardDAVLimit(req.Limit)}, nil
}

func ValidateGetAddressBookRequest(req GetAddressBookRequest) (GetAddressBookRequest, error) {
	userID, err := validateCardDAVID("user_id", req.UserID, true)
	if err != nil {
		return GetAddressBookRequest{}, err
	}
	bookID, err := validateCardDAVID("addressbook_id", req.AddressBookID, true)
	if err != nil {
		return GetAddressBookRequest{}, err
	}
	status, err := ValidateAddressBookStatus(req.Status)
	if err != nil {
		return GetAddressBookRequest{}, err
	}
	return GetAddressBookRequest{UserID: userID, AddressBookID: bookID, Status: status}, nil
}

func ValidateUpdateAddressBookRequest(req UpdateAddressBookRequest) (UpdateAddressBookRequest, string, string, error) {
	userID, err := validateCardDAVID("user_id", req.UserID, true)
	if err != nil {
		return UpdateAddressBookRequest{}, "", "", err
	}
	actorUserID, err := validateCardDAVActorUserID(req.ActorUserID, userID)
	if err != nil {
		return UpdateAddressBookRequest{}, "", "", err
	}
	bookID, err := validateCardDAVID("addressbook_id", req.AddressBookID, true)
	if err != nil {
		return UpdateAddressBookRequest{}, "", "", err
	}
	observedETag, err := validateOptionalContactETag(req.ObservedETag)
	if err != nil {
		return UpdateAddressBookRequest{}, "", "", err
	}
	if req.Name == nil && req.Description == nil {
		return UpdateAddressBookRequest{}, "", "", fmt.Errorf("at least one address book property is required")
	}
	var normalizedName string
	var name *string
	if req.Name != nil {
		value, err := ValidateAddressBookName(*req.Name)
		if err != nil {
			return UpdateAddressBookRequest{}, "", "", err
		}
		normalizedName, err = NormalizeAddressBookName(value)
		if err != nil {
			return UpdateAddressBookRequest{}, "", "", err
		}
		name = &value
	}
	var description *string
	if req.Description != nil {
		value, err := ValidateAddressBookDescription(*req.Description)
		if err != nil {
			return UpdateAddressBookRequest{}, "", "", err
		}
		description = &value
	}
	syncToken := AddressBookSyncToken(userID, bookID, "addressbook-update", time.Now().UTC().Format(time.RFC3339Nano))
	return UpdateAddressBookRequest{UserID: userID, ActorUserID: actorUserID, AddressBookID: bookID, Name: name, Description: description, ObservedETag: observedETag}, normalizedName, syncToken, nil
}

func ValidateDeleteAddressBookRequest(req DeleteAddressBookRequest) (DeleteAddressBookRequest, error) {
	userID, err := validateCardDAVID("user_id", req.UserID, true)
	if err != nil {
		return DeleteAddressBookRequest{}, err
	}
	actorUserID, err := validateCardDAVActorUserID(req.ActorUserID, userID)
	if err != nil {
		return DeleteAddressBookRequest{}, err
	}
	bookID, err := validateCardDAVID("addressbook_id", req.AddressBookID, true)
	if err != nil {
		return DeleteAddressBookRequest{}, err
	}
	observedETag, err := validateOptionalContactETag(req.ObservedETag)
	if err != nil {
		return DeleteAddressBookRequest{}, err
	}
	return DeleteAddressBookRequest{UserID: userID, ActorUserID: actorUserID, AddressBookID: bookID, ObservedETag: observedETag}, nil
}

func ValidateUpsertContactObjectRequest(req UpsertContactObjectRequest) (UpsertContactObjectRequest, string, string, error) {
	userID, err := validateCardDAVID("user_id", req.UserID, true)
	if err != nil {
		return UpsertContactObjectRequest{}, "", "", err
	}
	actorUserID, err := validateCardDAVActorUserID(req.ActorUserID, userID)
	if err != nil {
		return UpsertContactObjectRequest{}, "", "", err
	}
	bookID, err := validateCardDAVID("addressbook_id", req.AddressBookID, true)
	if err != nil {
		return UpsertContactObjectRequest{}, "", "", err
	}
	objectName, err := ValidateContactObjectName(req.ObjectName)
	if err != nil {
		return UpsertContactObjectRequest{}, "", "", err
	}
	meta, err := ValidateVCardObject(req.VCard)
	if err != nil {
		return UpsertContactObjectRequest{}, "", "", err
	}
	uid := strings.TrimSpace(req.UID)
	if uid == "" {
		uid = meta.UID
	}
	uid, err = ValidateContactObjectUID(uid)
	if err != nil {
		return UpsertContactObjectRequest{}, "", "", err
	}
	if uid != meta.UID {
		return UpsertContactObjectRequest{}, "", "", fmt.Errorf("contact object uid does not match vcard UID")
	}
	observedETag, err := validateOptionalContactETag(req.ObservedETag)
	if err != nil {
		return UpsertContactObjectRequest{}, "", "", err
	}
	etag, err := ContactObjectETag(req.VCard)
	if err != nil {
		return UpsertContactObjectRequest{}, "", "", err
	}
	syncToken := AddressBookSyncToken(userID, bookID, objectName, etag, time.Now().UTC().Format(time.RFC3339Nano))
	return UpsertContactObjectRequest{UserID: userID, ActorUserID: actorUserID, AddressBookID: bookID, ObjectName: objectName, UID: uid, VCard: req.VCard, ObservedETag: observedETag}, etag, syncToken, nil
}

func ValidateListContactObjectsRequest(req ListContactObjectsRequest) (ListContactObjectsRequest, error) {
	return validateListContactObjectsRequest(req, normalizeCardDAVLimit)
}

func ValidateListContactObjectsForSyncRequest(req ListContactObjectsRequest) (ListContactObjectsRequest, error) {
	return validateListContactObjectsRequest(req, normalizeCardDAVChangeLimit)
}

func validateListContactObjectsRequest(req ListContactObjectsRequest, normalizeLimit func(int) int) (ListContactObjectsRequest, error) {
	userID, err := validateCardDAVID("user_id", req.UserID, true)
	if err != nil {
		return ListContactObjectsRequest{}, err
	}
	bookID, err := validateCardDAVID("addressbook_id", req.AddressBookID, true)
	if err != nil {
		return ListContactObjectsRequest{}, err
	}
	status, err := ValidateAddressBookStatus(req.Status)
	if err != nil {
		return ListContactObjectsRequest{}, err
	}
	return ListContactObjectsRequest{UserID: userID, AddressBookID: bookID, Status: status, Limit: normalizeLimit(req.Limit)}, nil
}

func ValidateGetContactObjectRequest(req GetContactObjectRequest) (GetContactObjectRequest, error) {
	userID, err := validateCardDAVID("user_id", req.UserID, true)
	if err != nil {
		return GetContactObjectRequest{}, err
	}
	bookID, err := validateCardDAVID("addressbook_id", req.AddressBookID, true)
	if err != nil {
		return GetContactObjectRequest{}, err
	}
	objectName, err := ValidateContactObjectName(req.ObjectName)
	if err != nil {
		return GetContactObjectRequest{}, err
	}
	status, err := ValidateAddressBookStatus(req.Status)
	if err != nil {
		return GetContactObjectRequest{}, err
	}
	return GetContactObjectRequest{UserID: userID, AddressBookID: bookID, ObjectName: objectName, Status: status}, nil
}

func ValidateDeleteContactObjectRequest(req DeleteContactObjectRequest) (DeleteContactObjectRequest, string, error) {
	userID, err := validateCardDAVID("user_id", req.UserID, true)
	if err != nil {
		return DeleteContactObjectRequest{}, "", err
	}
	actorUserID, err := validateCardDAVActorUserID(req.ActorUserID, userID)
	if err != nil {
		return DeleteContactObjectRequest{}, "", err
	}
	bookID, err := validateCardDAVID("addressbook_id", req.AddressBookID, true)
	if err != nil {
		return DeleteContactObjectRequest{}, "", err
	}
	objectName, err := ValidateContactObjectName(req.ObjectName)
	if err != nil {
		return DeleteContactObjectRequest{}, "", err
	}
	observedETag, err := validateOptionalContactETag(req.ObservedETag)
	if err != nil {
		return DeleteContactObjectRequest{}, "", err
	}
	syncToken := AddressBookSyncToken(userID, bookID, objectName, "contact-delete", time.Now().UTC().Format(time.RFC3339Nano))
	return DeleteContactObjectRequest{UserID: userID, ActorUserID: actorUserID, AddressBookID: bookID, ObjectName: objectName, ObservedETag: observedETag}, syncToken, nil
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

func optionalStringArg(value *string) (string, bool) {
	if value == nil {
		return "", false
	}
	return *value, true
}

func lockActiveAddressBook(ctx context.Context, tx *sql.Tx, userID string, addressBookID string) error {
	var id string
	err := tx.QueryRowContext(ctx, `
SELECT id::text
FROM carddav_addressbooks
WHERE user_id = $1::uuid
  AND id = $2::uuid
  AND status = 'active'
FOR UPDATE`, userID, addressBookID).Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("CardDAV address book not found")
		}
		return fmt.Errorf("lock CardDAV address book: %w", err)
	}
	return nil
}

func ensureContactObjectUIDAvailable(ctx context.Context, tx *sql.Tx, userID string, addressBookID string, objectName string, uid string) error {
	var existingObject string
	err := tx.QueryRowContext(ctx, `
SELECT object_name
FROM carddav_contact_objects
WHERE user_id = $1::uuid
  AND addressbook_id = $2::uuid
  AND uid = $3
  AND object_name <> $4
  AND status = 'active'
LIMIT 1`, userID, addressBookID, uid, objectName).Scan(&existingObject)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("read CardDAV contact object UID: %w", err)
	}
	return fmt.Errorf("CardDAV contact object UID %q already exists as %q", uid, existingObject)
}

func mapContactObjectUpsertError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		switch pgErr.ConstraintName {
		case "idx_carddav_contact_objects_active_uid":
			return fmt.Errorf("CardDAV contact object UID already exists")
		case "idx_carddav_contact_objects_active_name":
			return fmt.Errorf("CardDAV contact object already exists")
		}
	}
	return fmt.Errorf("upsert CardDAV contact object: %w", err)
}

func ensureContactObjectETag(ctx context.Context, tx *sql.Tx, userID string, addressBookID string, objectName string, etag string) error {
	var current string
	err := tx.QueryRowContext(ctx, `
SELECT etag
FROM carddav_contact_objects
WHERE user_id = $1::uuid
  AND addressbook_id = $2::uuid
  AND object_name = $3
  AND status = 'active'
FOR UPDATE`, userID, addressBookID, objectName).Scan(&current)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("CardDAV contact object not found")
		}
		return fmt.Errorf("read CardDAV contact object etag: %w", err)
	}
	if current != etag {
		return fmt.Errorf("CardDAV contact object etag mismatch")
	}
	return nil
}

func ensureAddressBookCollectionETag(ctx context.Context, tx *sql.Tx, userID string, addressBookID string, etag string) error {
	var book AddressBook
	err := tx.QueryRowContext(ctx, `
SELECT id::text, user_id::text, name, description, sync_token, created_at, updated_at
FROM carddav_addressbooks
WHERE user_id = $1::uuid
  AND id = $2::uuid
  AND status = 'active'
FOR UPDATE`, userID, addressBookID).Scan(
		&book.ID,
		&book.UserID,
		&book.Name,
		&book.Description,
		&book.SyncToken,
		&book.CreatedAt,
		&book.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("CardDAV address book not found")
		}
		return fmt.Errorf("read CardDAV address book collection etag: %w", err)
	}
	current, err := AddressBookCollectionETag(userID, book)
	if err != nil {
		return fmt.Errorf("build CardDAV address book collection etag: %w", err)
	}
	if current != etag {
		return fmt.Errorf("CardDAV address book collection etag mismatch")
	}
	return nil
}

func updateAddressBookSyncToken(ctx context.Context, tx *sql.Tx, userID string, addressBookID string, syncToken string) error {
	res, err := tx.ExecContext(ctx, `
UPDATE carddav_addressbooks
SET sync_token = $3, updated_at = now()
WHERE user_id = $1::uuid
  AND id = $2::uuid
  AND status = 'active'`, userID, addressBookID, syncToken)
	if err != nil {
		return fmt.Errorf("update CardDAV sync token: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("read CardDAV sync token update count: %w", err)
	}
	if affected != 1 {
		return fmt.Errorf("CardDAV address book not found")
	}
	return nil
}

func ensureAddressBookSyncMarker(ctx context.Context, tx *sql.Tx, userID string, addressBookID string) error {
	var token string
	err := tx.QueryRowContext(ctx, `
SELECT sync_token
FROM carddav_addressbooks
WHERE user_id = $1::uuid
  AND id = $2::uuid
  AND status = 'active'`, userID, addressBookID).Scan(&token)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("CardDAV address book not found")
		}
		return fmt.Errorf("read CardDAV sync marker: %w", err)
	}
	_, err = tx.ExecContext(ctx, `
INSERT INTO carddav_addressbook_changes (
  user_id, addressbook_id, sync_token, action
)
SELECT $1::uuid, $2::uuid, $3, 'addressbook-created'
WHERE NOT EXISTS (
  SELECT 1
  FROM carddav_addressbook_changes
  WHERE addressbook_id = $2::uuid
    AND sync_token = $3
    AND action = 'addressbook-created'
)`, userID, addressBookID, token)
	if err != nil {
		return fmt.Errorf("ensure CardDAV sync marker: %w", err)
	}
	return nil
}

func validateOptionalContactETag(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	return ValidateContactObjectETag(value)
}

func validateCardDAVID(field string, value string, required bool) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		if required {
			return "", fmt.Errorf("%s is required", field)
		}
		return "", nil
	}
	if len(value) > maxSegmentBytes {
		return "", fmt.Errorf("%s is too long", field)
	}
	if strings.ContainsAny(value, "\r\n") {
		return "", fmt.Errorf("%s must not contain line breaks", field)
	}
	return value, nil
}

func validateCardDAVActorUserID(actorUserID string, ownerUserID string) (string, error) {
	actorUserID = strings.TrimSpace(actorUserID)
	if actorUserID == "" {
		return ownerUserID, nil
	}
	return validateCardDAVID("actor_user_id", actorUserID, true)
}

func normalizeCardDAVLimit(limit int) int {
	if limit <= 0 {
		return 200
	}
	if limit > 1000 {
		return 1000
	}
	return limit
}

func normalizeCardDAVChangeLimit(limit int) int {
	if limit <= 0 {
		return 200
	}
	if limit > MaxWebDAVReportLimit+1 {
		return MaxWebDAVReportLimit + 1
	}
	return limit
}
