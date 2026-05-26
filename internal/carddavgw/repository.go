package carddavgw

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/lib/pq"
)

type Repository struct {
	db *sql.DB
}

const carddavContactObjectLookupBatchSize = 256
const carddavWriteMaxAttempts = 4
const carddavWriteBaseDelay = 5 * time.Millisecond
const carddavWriteMaxDelay = 80 * time.Millisecond

type contactObjectNameLookup struct {
	addressBookID string
	objectName    string
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

type CreateAddressBookRequest struct {
	UserID          string
	ActorUserID     string
	Name            string
	NameLang        string
	Description     string
	DescriptionLang string
}

type CreateAddressBookAtPathRequest struct {
	UserID          string
	ActorUserID     string
	AddressBookID   string
	Name            string
	NameLang        string
	Description     string
	DescriptionLang string
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
	UserID          string
	ActorUserID     string
	AddressBookID   string
	Name            *string
	NameLang        *string
	Description     *string
	DescriptionLang *string
	ObservedETag    string
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

type CreateACLRuleRequest struct {
	AddressBookID   string
	PrincipalID     string
	GrantPrivileges []string
	DenyPrivileges  []string
	Protected       bool
}

type GetACLRulesRequest struct {
	AddressBookID string
}

type UpdateACLRuleRequest struct {
	ACLRuleID       string
	GrantPrivileges []string
	DenyPrivileges  []string
}

type DeleteACLRuleRequest struct {
	ACLRuleID string
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
  company_id, domain_id, user_id, name, normalized_name, displayname_lang, description, description_lang, sync_token
)
SELECT company_id, domain_id, user_id, $2, $3, $4, $5, $6, $7
FROM active_user
RETURNING id::text, user_id::text, name, displayname_lang, description, description_lang, sync_token, created_at, updated_at`
	var book AddressBook
	err = tx.QueryRowContext(ctx, query,
		req.UserID,
		req.Name,
		normalizedName,
		req.NameLang,
		req.Description,
		req.DescriptionLang,
		syncToken,
	).Scan(
		&book.ID,
		&book.UserID,
		&book.Name,
		&book.NameLang,
		&book.Description,
		&book.DescriptionLang,
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
  id, company_id, domain_id, user_id, name, normalized_name, displayname_lang, description, description_lang, sync_token
)
SELECT $2::uuid, company_id, domain_id, user_id, $3, $4, $5, $6, $7, $8
FROM active_user
RETURNING id::text, user_id::text, name, displayname_lang, description, description_lang, sync_token, created_at, updated_at`
	var book AddressBook
	err = tx.QueryRowContext(ctx, query,
		req.UserID,
		req.AddressBookID,
		req.Name,
		normalizedName,
		req.NameLang,
		req.Description,
		req.DescriptionLang,
		syncToken,
	).Scan(
		&book.ID,
		&book.UserID,
		&book.Name,
		&book.NameLang,
		&book.Description,
		&book.DescriptionLang,
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
SELECT id::text, user_id::text, name, displayname_lang, description, description_lang, sync_token, created_at, updated_at
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
		if err := rows.Scan(&book.ID, &book.UserID, &book.Name, &book.NameLang, &book.Description, &book.DescriptionLang, &book.SyncToken, &book.CreatedAt, &book.UpdatedAt); err != nil {
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
SELECT id::text, user_id::text, name, displayname_lang, description, description_lang, sync_token, created_at, updated_at
FROM carddav_addressbooks
WHERE user_id = $1::uuid
  AND id = $2::uuid
  AND status = $3`
	var book AddressBook
	if err := r.db.QueryRowContext(ctx, query, req.UserID, req.AddressBookID, req.Status).Scan(&book.ID, &book.UserID, &book.Name, &book.NameLang, &book.Description, &book.DescriptionLang, &book.SyncToken, &book.CreatedAt, &book.UpdatedAt); err != nil {
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
	var book AddressBook
	if err := runCardDAVWriteWithRetry(ctx, func() error {
		tx, err := r.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin CardDAV address book update: %w", err)
		}
		defer tx.Rollback()
		if req.ObservedETag != "" {
			if err := ensureAddressBookCollectionETag(ctx, tx, req.UserID, req.AddressBookID, req.ObservedETag); err != nil {
				return err
			}
		}
		nameValue, nameSet := optionalStringArg(req.Name)
		nameLangValue, nameLangSet := optionalStringArg(req.NameLang)
		descriptionValue, descriptionSet := optionalStringArg(req.Description)
		descriptionLangValue, descriptionLangSet := optionalStringArg(req.DescriptionLang)
		const query = `
UPDATE carddav_addressbooks
SET
  name = CASE WHEN $3 THEN $4 ELSE name END,
  normalized_name = CASE WHEN $3 THEN $5 ELSE normalized_name END,
  displayname_lang = CASE WHEN $6 THEN $7 ELSE displayname_lang END,
  description = CASE WHEN $8 THEN $9 ELSE description END,
  description_lang = CASE WHEN $10 THEN $11 ELSE description_lang END,
  sync_token = $12,
  updated_at = now()
WHERE user_id = $1::uuid
  AND id = $2::uuid
  AND status = 'active'
RETURNING id::text, user_id::text, name, displayname_lang, description, description_lang, sync_token, created_at, updated_at`
		err = tx.QueryRowContext(ctx, query,
			req.UserID,
			req.AddressBookID,
			nameSet,
			nameValue,
			normalizedName,
			nameLangSet,
			nameLangValue,
			descriptionSet,
			descriptionValue,
			descriptionLangSet,
			descriptionLangValue,
			syncToken,
		).Scan(
			&book.ID,
			&book.UserID,
			&book.Name,
			&book.NameLang,
			&book.Description,
			&book.DescriptionLang,
			&book.SyncToken,
			&book.CreatedAt,
			&book.UpdatedAt,
		)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("CardDAV address book not found")
			}
			return fmt.Errorf("update CardDAV address book properties: %w", err)
		}
		if err := insertAddressBookChange(ctx, tx, req.UserID, req.ActorUserID, req.AddressBookID, syncToken, "addressbook-updated", "", ""); err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit CardDAV address book update: %w", err)
		}
		return nil
	}); err != nil {
		return AddressBook{}, err
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
	var book AddressBook
	if err := runCardDAVWriteWithRetry(ctx, func() error {
		tx, err := r.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin CardDAV address book delete: %w", err)
		}
		defer tx.Rollback()
		if req.ObservedETag != "" {
			if err := ensureAddressBookCollectionETag(ctx, tx, req.UserID, req.AddressBookID, req.ObservedETag); err != nil {
				return err
			}
		}
		const query = `
UPDATE carddav_addressbooks
SET status = 'deleted', deleted_at = now(), updated_at = now()
WHERE user_id = $1::uuid
  AND id = $2::uuid
  AND status = 'active'
RETURNING id::text, user_id::text, name, displayname_lang, description, description_lang, sync_token, created_at, updated_at`
		err = tx.QueryRowContext(ctx, query, req.UserID, req.AddressBookID).Scan(
			&book.ID,
			&book.UserID,
			&book.Name,
			&book.NameLang,
			&book.Description,
			&book.DescriptionLang,
			&book.SyncToken,
			&book.CreatedAt,
			&book.UpdatedAt,
		)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("CardDAV address book not found")
			}
			return fmt.Errorf("delete CardDAV address book: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
UPDATE carddav_contact_objects
SET status = 'deleted', deleted_at = COALESCE(deleted_at, now()), updated_at = now()
WHERE user_id = $1::uuid
  AND addressbook_id = $2::uuid
  AND status = 'active'`, req.UserID, req.AddressBookID); err != nil {
			return fmt.Errorf("delete CardDAV contact objects: %w", err)
		}
		syncToken := AddressBookSyncToken(req.UserID, req.AddressBookID, "addressbook-delete", time.Now().UTC().Format(time.RFC3339Nano))
		if err := insertAddressBookChange(ctx, tx, req.UserID, req.ActorUserID, req.AddressBookID, syncToken, "addressbook-deleted", "", ""); err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit CardDAV address book delete: %w", err)
		}
		return nil
	}); err != nil {
		return AddressBook{}, err
	}
	return book, nil
}

func (r *Repository) CreateACLRule(ctx context.Context, req CreateACLRuleRequest) (ACLRule, error) {
	if r == nil || r.db == nil {
		return ACLRule{}, fmt.Errorf("database handle is required")
	}
	const query = `
INSERT INTO carddav_acl_rules (addressbook_id, principal_id, grant_privileges, deny_privileges, protected)
VALUES ($1::uuid, $2, $3, $4, $5)
ON CONFLICT (addressbook_id, principal_id)
DO UPDATE SET
  grant_privileges = EXCLUDED.grant_privileges,
  deny_privileges = EXCLUDED.deny_privileges,
  updated_at = now()
RETURNING id::text, addressbook_id::text, principal_id, grant_privileges, deny_privileges, protected, created_at, updated_at`

	var rule ACLRule
	err := r.db.QueryRowContext(ctx, query,
		req.AddressBookID,
		req.PrincipalID,
		pq.Array(req.GrantPrivileges),
		pq.Array(req.DenyPrivileges),
		req.Protected,
	).Scan(
		&rule.ID,
		&rule.AddressBookID,
		&rule.PrincipalID,
		pq.Array(&rule.GrantPrivileges),
		pq.Array(&rule.DenyPrivileges),
		&rule.Protected,
		&rule.CreatedAt,
		&rule.UpdatedAt,
	)
	if err != nil {
		return ACLRule{}, fmt.Errorf("create CardDAV ACL rule: %w", err)
	}
	return rule, nil
}

func (r *Repository) GetACLRules(ctx context.Context, req GetACLRulesRequest) ([]ACLRule, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	const query = `
SELECT id::text, addressbook_id::text, principal_id, grant_privileges, deny_privileges, protected, created_at, updated_at
FROM carddav_acl_rules
WHERE addressbook_id = $1::uuid
ORDER BY created_at ASC`

	rows, err := r.db.QueryContext(ctx, query, req.AddressBookID)
	if err != nil {
		return nil, fmt.Errorf("get CardDAV ACL rules: %w", err)
	}
	defer rows.Close()

	var rules []ACLRule
	for rows.Next() {
		var rule ACLRule
		if err := rows.Scan(
			&rule.ID,
			&rule.AddressBookID,
			&rule.PrincipalID,
			pq.Array(&rule.GrantPrivileges),
			pq.Array(&rule.DenyPrivileges),
			&rule.Protected,
			&rule.CreatedAt,
			&rule.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan CardDAV ACL rule: %w", err)
		}
		rules = append(rules, rule)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate CardDAV ACL rules: %w", err)
	}
	return rules, nil
}

func (r *Repository) DeleteACLRule(ctx context.Context, req DeleteACLRuleRequest) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	result, err := r.db.ExecContext(ctx, `
DELETE FROM carddav_acl_rules
WHERE id = $1::uuid`, req.ACLRuleID)
	if err != nil {
		return fmt.Errorf("delete CardDAV ACL rule: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("CardDAV ACL rule not found")
	}
	return nil
}

func ensureAddressBookCollectionETag(ctx context.Context, tx *sql.Tx, userID string, addressBookID string, etag string) error {
	var book AddressBook
	err := tx.QueryRowContext(ctx, `
SELECT id::text, user_id::text, name, displayname_lang, description, description_lang, sync_token, created_at, updated_at
FROM carddav_addressbooks
WHERE user_id = $1::uuid
  AND id = $2::uuid
  AND status = 'active'`, userID, addressBookID).Scan(
		&book.ID,
		&book.UserID,
		&book.Name,
		&book.NameLang,
		&book.Description,
		&book.DescriptionLang,
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
	var hasActiveAddressBook bool
	err := tx.QueryRowContext(ctx, `
WITH active_addressbook AS (
  SELECT sync_token
  FROM carddav_addressbooks
  WHERE user_id = $1::uuid
    AND id = $2::uuid
    AND status = 'active'
),
insert_marker AS (
  INSERT INTO carddav_addressbook_changes (
    user_id, addressbook_id, sync_token, action
  )
  SELECT $1::uuid, $2::uuid, sync_token, 'addressbook-created'
  FROM active_addressbook
  WHERE NOT EXISTS (
    SELECT 1
    FROM carddav_addressbook_changes existing
    JOIN active_addressbook active ON active.sync_token = existing.sync_token
    WHERE existing.addressbook_id = $2::uuid
      AND existing.sync_token = active.sync_token
      AND existing.action = 'addressbook-created'
  )
)
SELECT EXISTS (SELECT 1 FROM active_addressbook)`, userID, addressBookID).Scan(&hasActiveAddressBook)
	if err != nil {
		return fmt.Errorf("read CardDAV sync marker: %w", err)
	}
	if !hasActiveAddressBook {
		return fmt.Errorf("CardDAV address book not found")
	}
	return nil
}

func optionalStringArg(value *string) (string, bool) {
	if value == nil {
		return "", false
	}
	return *value, true
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

func isRetryableCardDAVWriteError(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	switch pgErr.Code {
	case "40001", "40P01", "40P02", "55P03":
		return true
	default:
		return false
	}
}

func sleepWithContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func runCardDAVWriteWithRetry(ctx context.Context, fn func() error) error {
	for attempt := 0; attempt < carddavWriteMaxAttempts; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}
		if !isRetryableCardDAVWriteError(err) || attempt+1 >= carddavWriteMaxAttempts {
			return err
		}
		delay := carddavWriteBaseDelay << attempt
		if delay > carddavWriteMaxDelay {
			delay = carddavWriteMaxDelay
		}
		jitter := time.Duration(time.Now().UnixNano() % int64(delay))
		if err := sleepWithContext(ctx, delay+jitter); err != nil {
			return err
		}
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
	nameLang, err := validateXMLLangString("displayname xml:lang", req.NameLang)
	if err != nil {
		return CreateAddressBookRequest{}, "", "", err
	}
	descriptionLang, err := validateXMLLangString("addressbook-description xml:lang", req.DescriptionLang)
	if err != nil {
		return CreateAddressBookRequest{}, "", "", err
	}
	syncToken := AddressBookSyncToken(userID, normalizedName, time.Now().UTC().Format(time.RFC3339Nano))
	return CreateAddressBookRequest{UserID: userID, ActorUserID: actorUserID, Name: name, NameLang: nameLang, Description: description, DescriptionLang: descriptionLang}, normalizedName, syncToken, nil
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
		UserID:          userID,
		ActorUserID:     req.ActorUserID,
		Name:            name,
		NameLang:        req.NameLang,
		Description:     req.Description,
		DescriptionLang: req.DescriptionLang,
	})
	if err != nil {
		return CreateAddressBookAtPathRequest{}, "", "", err
	}
	return CreateAddressBookAtPathRequest{
		UserID:          create.UserID,
		ActorUserID:     create.ActorUserID,
		AddressBookID:   bookID,
		Name:            create.Name,
		NameLang:        create.NameLang,
		Description:     create.Description,
		DescriptionLang: create.DescriptionLang,
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
	var nameLang *string
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
		valueLang, err := validateOptionalXMLLangPointer("displayname xml:lang", req.NameLang)
		if err != nil {
			return UpdateAddressBookRequest{}, "", "", err
		}
		nameLang = valueLang
	}
	var description *string
	var descriptionLang *string
	if req.Description != nil {
		value, err := ValidateAddressBookDescription(*req.Description)
		if err != nil {
			return UpdateAddressBookRequest{}, "", "", err
		}
		description = &value
		valueLang, err := validateOptionalXMLLangPointer("addressbook-description xml:lang", req.DescriptionLang)
		if err != nil {
			return UpdateAddressBookRequest{}, "", "", err
		}
		descriptionLang = valueLang
	}
	syncToken := AddressBookSyncToken(userID, bookID, "addressbook-update", time.Now().UTC().Format(time.RFC3339Nano))
	return UpdateAddressBookRequest{UserID: userID, ActorUserID: actorUserID, AddressBookID: bookID, Name: name, NameLang: nameLang, Description: description, DescriptionLang: descriptionLang, ObservedETag: observedETag}, normalizedName, syncToken, nil
}

func validateXMLLangPointer(field string, value *string) (*string, error) {
	if value == nil {
		empty := ""
		return &empty, nil
	}
	lang, err := validateXMLLangString(field, *value)
	if err != nil {
		return nil, err
	}
	return &lang, nil
}

func validateOptionalXMLLangPointer(field string, value *string) (*string, error) {
	if value == nil {
		return nil, nil
	}
	lang, err := validateXMLLangString(field, *value)
	if err != nil {
		return nil, err
	}
	return &lang, nil
}

func validateXMLLangString(field string, value string) (string, error) {
	lang, err := validateXMLLang(value)
	if err != nil {
		return "", fmt.Errorf("%s is invalid: %w", field, err)
	}
	return lang, nil
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
