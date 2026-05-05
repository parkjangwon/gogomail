package carddavgw

import (
	"context"
	"fmt"

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

func (r *Repository) LookupContactObject(ctx context.Context, userID string, addressBookID string, objectName string) (ContactObject, error) {
	return r.GetContactObject(ctx, GetContactObjectRequest{UserID: userID, AddressBookID: addressBookID, ObjectName: objectName, Status: AddressBookStatusActive})
}
