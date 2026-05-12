package carddavgw

import (
	"context"
	"fmt"
	"strings"

	"github.com/gogomail/gogomail/internal/directory"
)

var _ DiscoveryStore = (*Repository)(nil)

func (r *Repository) LookupPrincipal(ctx context.Context, userID string) (Principal, error) {
	if r == nil || r.db == nil {
		return Principal{}, fmt.Errorf("database handle is required")
	}
	userID, err := validateCardDAVID("user_id", userID, true)
	if err != nil {
		return Principal{}, err
	}
	resolved, err := directory.NewRepository(r.db).ResolvePrincipal(ctx, directory.ResolvePrincipalRequest{
		ID:         userID,
		Kind:       directory.PrincipalKindUser,
		ActiveOnly: true,
	})
	if err != nil {
		return Principal{}, fmt.Errorf("lookup CardDAV principal: %w", err)
	}
	principal, err := cardDAVPrincipalFromDirectory(resolved)
	if err != nil {
		return Principal{}, err
	}
	return principal, nil
}

func cardDAVPrincipalFromDirectory(resolved directory.Principal) (Principal, error) {
	if resolved.Kind != directory.PrincipalKindUser {
		return Principal{}, fmt.Errorf("carddav principal kind %q is not supported", resolved.Kind)
	}
	principal := Principal{UserID: resolved.ID, DisplayName: resolved.DisplayName}
	principalPath, err := PrincipalPath(principal.UserID)
	if err != nil {
		return Principal{}, err
	}
	homePath, err := AddressBookHomePath(principal.UserID)
	if err != nil {
		return Principal{}, err
	}
	principal.PrincipalPath = principalPath
	principal.AddressBookHomePath = homePath
	return principal, nil
}

func (r *Repository) ListAddressBookCollections(ctx context.Context, userID string) ([]AddressBook, error) {
	return r.ListAddressBooks(ctx, ListAddressBooksRequest{UserID: userID, Status: AddressBookStatusActive})
}

func (r *Repository) LookupAddressBook(ctx context.Context, userID string, addressBookID string) (AddressBook, error) {
	return r.GetAddressBook(ctx, GetAddressBookRequest{UserID: userID, AddressBookID: addressBookID, Status: AddressBookStatusActive})
}

func (r *Repository) ListAddressBookObjects(ctx context.Context, userID string, addressBookID string) ([]ContactObject, error) {
	return r.ListContactObjects(ctx, ListContactObjectsRequest{UserID: userID, AddressBookID: addressBookID, Status: AddressBookStatusActive})
}

func (r *Repository) ListAddressBookObjectsLimit(ctx context.Context, userID string, addressBookID string, limit int) ([]ContactObject, error) {
	return r.listContactObjectsForSync(ctx, ListContactObjectsRequest{UserID: userID, AddressBookID: addressBookID, Status: AddressBookStatusActive, Limit: limit})
}

func (r *Repository) WalkAddressBookObjects(ctx context.Context, userID string, addressBookID string, yield func(ContactObject) (bool, error)) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	if yield == nil {
		return fmt.Errorf("CardDAV contact object yield function is required")
	}
	userID, err := validateCardDAVID("user_id", userID, true)
	if err != nil {
		return err
	}
	addressBookID, err = validateCardDAVID("addressbook_id", addressBookID, true)
	if err != nil {
		return err
	}
	const query = `
SELECT id::text, user_id::text, addressbook_id::text, object_name, uid, etag, size, vcard, created_at, updated_at
FROM carddav_contact_objects
WHERE user_id = $1::uuid
  AND addressbook_id = $2::uuid
  AND status = 'active'
ORDER BY updated_at DESC, id DESC`
	rows, err := r.db.QueryContext(ctx, query, userID, addressBookID)
	if err != nil {
		return fmt.Errorf("walk CardDAV contact objects: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var object ContactObject
		if err := rows.Scan(&object.ID, &object.UserID, &object.AddressBookID, &object.ObjectName, &object.UID, &object.ETag, &object.Size, &object.VCard, &object.CreatedAt, &object.UpdatedAt); err != nil {
			return fmt.Errorf("scan CardDAV contact object: %w", err)
		}
		keepGoing, err := yield(object)
		if err != nil {
			return err
		}
		if !keepGoing {
			break
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate CardDAV contact objects: %w", err)
	}
	return nil
}

func (r *Repository) WalkAddressBookQueryCandidates(ctx context.Context, userID string, addressBookID string, containsText string, yield func(ContactObject) (bool, error)) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	if yield == nil {
		return fmt.Errorf("CardDAV contact object yield function is required")
	}
	userID, err := validateCardDAVID("user_id", userID, true)
	if err != nil {
		return err
	}
	addressBookID, err = validateCardDAVID("addressbook_id", addressBookID, true)
	if err != nil {
		return err
	}
	containsText = strings.TrimSpace(containsText)
	if containsText == "" {
		return fmt.Errorf("CardDAV addressbook-query candidate text is required")
	}
	pattern := "%" + escapeSQLLikePattern(strings.ToLower(containsText)) + "%"
	const query = `
SELECT id::text, user_id::text, addressbook_id::text, object_name, uid, etag, size, vcard, created_at, updated_at
FROM carddav_contact_objects
WHERE user_id = $1::uuid
  AND addressbook_id = $2::uuid
  AND status = 'active'
  AND lower(vcard::text) LIKE $3 ESCAPE '\'
ORDER BY updated_at DESC, id DESC`
	rows, err := r.db.QueryContext(ctx, query, userID, addressBookID, pattern)
	if err != nil {
		return fmt.Errorf("walk CardDAV addressbook-query candidates: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var object ContactObject
		if err := rows.Scan(&object.ID, &object.UserID, &object.AddressBookID, &object.ObjectName, &object.UID, &object.ETag, &object.Size, &object.VCard, &object.CreatedAt, &object.UpdatedAt); err != nil {
			return fmt.Errorf("scan CardDAV addressbook-query candidate: %w", err)
		}
		keepGoing, err := yield(object)
		if err != nil {
			return err
		}
		if !keepGoing {
			break
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate CardDAV addressbook-query candidates: %w", err)
	}
	return nil
}

func escapeSQLLikePattern(value string) string {
	var b strings.Builder
	for _, r := range value {
		if r == '\\' || r == '%' || r == '_' {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

func (r *Repository) LookupContactObject(ctx context.Context, userID string, addressBookID string, objectName string) (ContactObject, error) {
	return r.GetContactObject(ctx, GetContactObjectRequest{UserID: userID, AddressBookID: addressBookID, ObjectName: objectName, Status: AddressBookStatusActive})
}
