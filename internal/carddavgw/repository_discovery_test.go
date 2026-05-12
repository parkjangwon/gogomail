package carddavgw

import (
	"context"
	"strings"
	"testing"

	"github.com/gogomail/gogomail/internal/directory"
)

func TestRepositorySatisfiesDiscoveryStore(t *testing.T) {
	t.Parallel()

	var _ DiscoveryStore = (*Repository)(nil)
	var _ AddressBookQueryCandidateWalker = (*Repository)(nil)
}

func TestRepositoryDiscoveryMethodsRequireDatabase(t *testing.T) {
	t.Parallel()

	repo := NewRepository(nil)
	if _, err := repo.LookupPrincipal(context.Background(), "user-1"); err == nil {
		t.Fatal("LookupPrincipal error = nil, want database requirement")
	}
	if _, err := repo.ListAddressBookCollections(context.Background(), "user-1"); err == nil {
		t.Fatal("ListAddressBookCollections error = nil, want database requirement")
	}
	if _, err := repo.LookupAddressBook(context.Background(), "user-1", "book-1"); err == nil {
		t.Fatal("LookupAddressBook error = nil, want database requirement")
	}
	if _, err := repo.ListAddressBookObjects(context.Background(), "user-1", "book-1"); err == nil {
		t.Fatal("ListAddressBookObjects error = nil, want database requirement")
	}
	if err := repo.WalkAddressBookObjects(context.Background(), "user-1", "book-1", func(ContactObject) (bool, error) { return false, nil }); err == nil {
		t.Fatal("WalkAddressBookObjects error = nil, want database requirement")
	}
	if err := repo.WalkAddressBookQueryCandidates(context.Background(), "user-1", "book-1", "alice", func(ContactObject) (bool, error) { return false, nil }); err == nil {
		t.Fatal("WalkAddressBookQueryCandidates error = nil, want database requirement")
	}
	if _, err := repo.LookupContactObject(context.Background(), "user-1", "book-1", "contact-1.vcf"); err == nil {
		t.Fatal("LookupContactObject error = nil, want database requirement")
	}
}

func TestCardDAVPrincipalFromDirectoryAcceptsUserPrincipal(t *testing.T) {
	t.Parallel()

	principal, err := cardDAVPrincipalFromDirectory(directory.Principal{
		ID:          "user-1",
		Kind:        directory.PrincipalKindUser,
		DisplayName: "User One",
	})
	if err != nil {
		t.Fatalf("cardDAVPrincipalFromDirectory returned error: %v", err)
	}
	if principal.UserID != "user-1" || principal.DisplayName != "User One" {
		t.Fatalf("principal = %+v", principal)
	}
	if principal.PrincipalPath != "/carddav/principals/user-1/" {
		t.Fatalf("PrincipalPath = %q", principal.PrincipalPath)
	}
	if principal.AddressBookHomePath != "/carddav/addressbooks/user-1/" {
		t.Fatalf("AddressBookHomePath = %q", principal.AddressBookHomePath)
	}
}

func TestCardDAVPrincipalFromDirectoryRejectsNonUserPrincipals(t *testing.T) {
	t.Parallel()

	for _, kind := range []string{
		directory.PrincipalKindOrganization,
		directory.PrincipalKindGroup,
		directory.PrincipalKindResource,
	} {
		kind := kind
		t.Run(kind, func(t *testing.T) {
			t.Parallel()

			_, err := cardDAVPrincipalFromDirectory(directory.Principal{ID: "principal-1", Kind: kind})
			if err == nil || !strings.Contains(err.Error(), "carddav principal kind") || !strings.Contains(err.Error(), "is not supported") {
				t.Fatalf("error = %v, want unsupported CardDAV principal kind", err)
			}
		})
	}
}
