package carddavgw

import (
	"context"
	"strings"
	"testing"
)

func TestValidateCreateAddressBookRequest(t *testing.T) {
	t.Parallel()

	req, normalizedName, syncToken, err := ValidateCreateAddressBookRequest(CreateAddressBookRequest{
		UserID:      " user-1 ",
		Name:        " Personal ",
		Description: " People I know ",
	})
	if err != nil {
		t.Fatalf("ValidateCreateAddressBookRequest returned error: %v", err)
	}
	if req.UserID != "user-1" || req.Name != "Personal" || req.Description != "People I know" {
		t.Fatalf("request = %+v", req)
	}
	if normalizedName != "personal" {
		t.Fatalf("normalized name = %q", normalizedName)
	}
	if !strings.HasPrefix(syncToken, "sync-") {
		t.Fatalf("sync token = %q", syncToken)
	}
}

func TestValidateCreateAddressBookRequestRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	tests := []CreateAddressBookRequest{
		{Name: "Personal"},
		{UserID: "user\n1", Name: "Personal"},
		{UserID: "user-1", Name: "bad\nname"},
		{UserID: "user-1", Name: "Personal", Description: "bad\nline"},
	}
	for _, req := range tests {
		req := req
		t.Run(req.UserID+"/"+req.Name, func(t *testing.T) {
			t.Parallel()

			if _, _, _, err := ValidateCreateAddressBookRequest(req); err == nil {
				t.Fatalf("ValidateCreateAddressBookRequest(%+v) error = nil, want rejection", req)
			}
		})
	}
}

func TestValidateAddressBookReadRequests(t *testing.T) {
	t.Parallel()

	list, err := ValidateListAddressBooksRequest(ListAddressBooksRequest{
		UserID: " user-1 ",
		Limit:  2000,
	})
	if err != nil {
		t.Fatalf("ValidateListAddressBooksRequest returned error: %v", err)
	}
	if list.UserID != "user-1" || list.Status != AddressBookStatusActive || list.Limit != 1000 {
		t.Fatalf("list request = %+v", list)
	}
	get, err := ValidateGetAddressBookRequest(GetAddressBookRequest{
		UserID:        " user-1 ",
		AddressBookID: " book-1 ",
		Status:        " deleted ",
	})
	if err != nil {
		t.Fatalf("ValidateGetAddressBookRequest returned error: %v", err)
	}
	if get.UserID != "user-1" || get.AddressBookID != "book-1" || get.Status != AddressBookStatusDeleted {
		t.Fatalf("get request = %+v", get)
	}
}

func TestRepositoryAddressBookMethodsRequireDatabase(t *testing.T) {
	t.Parallel()

	repo := NewRepository(nil)
	ctx := context.Background()
	tests := []struct {
		name string
		run  func() error
	}{
		{name: "create", run: func() error {
			_, err := repo.CreateAddressBook(ctx, CreateAddressBookRequest{UserID: "user-1", Name: "Personal"})
			return err
		}},
		{name: "list", run: func() error {
			_, err := repo.ListAddressBooks(ctx, ListAddressBooksRequest{UserID: "user-1"})
			return err
		}},
		{name: "get", run: func() error {
			_, err := repo.GetAddressBook(ctx, GetAddressBookRequest{UserID: "user-1", AddressBookID: "book-1"})
			return err
		}},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if err := tc.run(); err == nil || !strings.Contains(err.Error(), "database handle is required") {
				t.Fatalf("error = %v, want database handle requirement", err)
			}
		})
	}
}
