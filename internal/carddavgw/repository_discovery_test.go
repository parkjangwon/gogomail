package carddavgw

import (
	"context"
	"testing"
)

func TestRepositorySatisfiesDiscoveryStore(t *testing.T) {
	t.Parallel()

	var _ DiscoveryStore = (*Repository)(nil)
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
	if _, err := repo.LookupContactObject(context.Background(), "user-1", "book-1", "contact-1.vcf"); err == nil {
		t.Fatal("LookupContactObject error = nil, want database requirement")
	}
}
