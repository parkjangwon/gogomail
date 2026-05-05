package carddavgw

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

type CreateAddressBookRequest struct {
	UserID      string
	Name        string
	Description string
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
	if err := insertAddressBookChange(ctx, tx, book.UserID, book.ID, book.SyncToken, "addressbook-created", "", ""); err != nil {
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

func ValidateCreateAddressBookRequest(req CreateAddressBookRequest) (CreateAddressBookRequest, string, string, error) {
	userID, err := validateCardDAVID("user_id", req.UserID, true)
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
	return CreateAddressBookRequest{UserID: userID, Name: name, Description: description}, normalizedName, syncToken, nil
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

type addressBookChangeExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func insertAddressBookChange(ctx context.Context, execer addressBookChangeExecer, userID string, addressBookID string, syncToken string, action string, objectName string, etag string) error {
	_, err := execer.ExecContext(ctx, `
INSERT INTO carddav_addressbook_changes (
  user_id, addressbook_id, sync_token, action, object_name, etag
) VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6)`, userID, addressBookID, syncToken, action, objectName, etag)
	if err != nil {
		return fmt.Errorf("insert CardDAV address book change: %w", err)
	}
	return nil
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

func normalizeCardDAVLimit(limit int) int {
	if limit <= 0 {
		return 200
	}
	if limit > 1000 {
		return 1000
	}
	return limit
}
