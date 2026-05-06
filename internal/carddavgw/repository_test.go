package carddavgw

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func stringPtr(value string) *string {
	return &value
}

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

func TestValidateCreateAddressBookAtPathRequest(t *testing.T) {
	t.Parallel()

	req, normalizedName, syncToken, err := ValidateCreateAddressBookAtPathRequest(CreateAddressBookAtPathRequest{
		UserID:        " user-1 ",
		AddressBookID: "11111111-1111-4111-8111-111111111111",
		Name:          " Team ",
		Description:   " Launch contacts ",
	})
	if err != nil {
		t.Fatalf("ValidateCreateAddressBookAtPathRequest returned error: %v", err)
	}
	if req.UserID != "user-1" || req.AddressBookID != "11111111-1111-4111-8111-111111111111" || req.Name != "Team" {
		t.Fatalf("request = %+v", req)
	}
	if normalizedName != "team" {
		t.Fatalf("normalized name = %q", normalizedName)
	}
	if !strings.HasPrefix(syncToken, "sync-") {
		t.Fatalf("sync token = %q", syncToken)
	}
}

func TestValidateCreateAddressBookAtPathRequestRejectsNonUUIDPathIDs(t *testing.T) {
	t.Parallel()

	for _, id := range []string{"team", "11111111-1111-4111-8111-11111111111z", "11111111111141118111111111111111"} {
		id := id
		t.Run(id, func(t *testing.T) {
			t.Parallel()

			if _, _, _, err := ValidateCreateAddressBookAtPathRequest(CreateAddressBookAtPathRequest{
				UserID:        "user-1",
				AddressBookID: id,
			}); err == nil {
				t.Fatal("ValidateCreateAddressBookAtPathRequest error = nil, want rejection")
			}
		})
	}
}

func TestMapContactObjectUpsertErrorMapsUniqueUIDViolation(t *testing.T) {
	t.Parallel()

	err := mapContactObjectUpsertError(&pgconn.PgError{
		Code:           "23505",
		ConstraintName: "idx_carddav_contact_objects_active_uid",
	})
	if err == nil || !strings.Contains(err.Error(), "UID already exists") {
		t.Fatalf("mapped error = %v, want duplicate UID context", err)
	}
}

func TestMapContactObjectUpsertErrorMapsUniqueNameViolation(t *testing.T) {
	t.Parallel()

	err := mapContactObjectUpsertError(&pgconn.PgError{
		Code:           "23505",
		ConstraintName: "idx_carddav_contact_objects_active_name",
	})
	if err == nil || !strings.Contains(err.Error(), "object already exists") {
		t.Fatalf("mapped error = %v, want duplicate object context", err)
	}
}

func TestMapContactObjectUpsertErrorPreservesUnknownErrors(t *testing.T) {
	t.Parallel()

	base := errors.New("driver failed")
	err := mapContactObjectUpsertError(base)
	if !errors.Is(err, base) {
		t.Fatalf("mapped error = %v, want wrapped driver error", err)
	}
	if !strings.Contains(err.Error(), "upsert CardDAV contact object") {
		t.Fatalf("mapped error = %v, want upsert context", err)
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

func TestValidateUpdateAddressBookRequest(t *testing.T) {
	t.Parallel()

	name := " Team "
	description := " Launch contacts "
	req, normalizedName, syncToken, err := ValidateUpdateAddressBookRequest(UpdateAddressBookRequest{
		UserID:        " user-1 ",
		AddressBookID: " book-1 ",
		Name:          &name,
		Description:   &description,
	})
	if err != nil {
		t.Fatalf("ValidateUpdateAddressBookRequest returned error: %v", err)
	}
	if req.UserID != "user-1" || req.AddressBookID != "book-1" {
		t.Fatalf("request = %+v", req)
	}
	if req.Name == nil || *req.Name != "Team" || req.Description == nil || *req.Description != "Launch contacts" {
		t.Fatalf("properties = %+v", req)
	}
	if normalizedName != "team" {
		t.Fatalf("normalized name = %q", normalizedName)
	}
	if !strings.HasPrefix(syncToken, "sync-") {
		t.Fatalf("sync token = %q", syncToken)
	}
}

func TestValidateUpdateAddressBookRequestRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	badName := "bad\nname"
	badDescription := "bad\nline"
	tests := []UpdateAddressBookRequest{
		{UserID: "user-1", AddressBookID: "book-1"},
		{UserID: "", AddressBookID: "book-1", Name: stringPtr("Team")},
		{UserID: "user-1", AddressBookID: "", Name: stringPtr("Team")},
		{UserID: "user-1", AddressBookID: "book-1", Name: &badName},
		{UserID: "user-1", AddressBookID: "book-1", Description: &badDescription},
	}
	for _, req := range tests {
		req := req
		t.Run(req.UserID+"/"+req.AddressBookID, func(t *testing.T) {
			t.Parallel()

			if _, _, _, err := ValidateUpdateAddressBookRequest(req); err == nil {
				t.Fatalf("ValidateUpdateAddressBookRequest(%+v) error = nil, want rejection", req)
			}
		})
	}
}

func TestValidateListAddressBookChangesSinceRequest(t *testing.T) {
	t.Parallel()

	req, err := ValidateListAddressBookChangesSinceRequest(ListAddressBookChangesSinceRequest{
		UserID:        " user-1 ",
		AddressBookID: " book-1 ",
		SyncToken:     " sync-123 ",
		Limit:         2000,
	})
	if err != nil {
		t.Fatalf("ValidateListAddressBookChangesSinceRequest returned error: %v", err)
	}
	if req.UserID != "user-1" || req.AddressBookID != "book-1" || req.SyncToken != "sync-123" || req.Limit != MaxWebDAVReportLimit+1 {
		t.Fatalf("request = %+v", req)
	}
	if _, err := ValidateListAddressBookChangesSinceRequest(ListAddressBookChangesSinceRequest{UserID: "user-1", AddressBookID: "book-1"}); err == nil {
		t.Fatal("ValidateListAddressBookChangesSinceRequest accepted missing sync token")
	}
	if _, err := ValidateListAddressBookChangesSinceRequest(ListAddressBookChangesSinceRequest{UserID: "user-1", AddressBookID: "book-1", SyncToken: "bad\nsync"}); err == nil {
		t.Fatal("ValidateListAddressBookChangesSinceRequest accepted unsafe sync token")
	}
}

func TestValidateDeleteAddressBookRequest(t *testing.T) {
	t.Parallel()

	req, err := ValidateDeleteAddressBookRequest(DeleteAddressBookRequest{
		UserID:        " user-1 ",
		AddressBookID: " book-1 ",
	})
	if err != nil {
		t.Fatalf("ValidateDeleteAddressBookRequest returned error: %v", err)
	}
	if req.UserID != "user-1" || req.AddressBookID != "book-1" {
		t.Fatalf("request = %+v", req)
	}
	if _, err := ValidateDeleteAddressBookRequest(DeleteAddressBookRequest{UserID: "user-1"}); err == nil {
		t.Fatal("ValidateDeleteAddressBookRequest accepted missing address book id")
	}
	if _, err := ValidateDeleteAddressBookRequest(DeleteAddressBookRequest{UserID: "user-1", AddressBookID: "bad\nbook"}); err == nil {
		t.Fatal("ValidateDeleteAddressBookRequest accepted unsafe address book id")
	}
}

func TestValidateUpsertContactObjectRequest(t *testing.T) {
	t.Parallel()

	body := []byte("BEGIN:VCARD\r\nVERSION:4.0\r\nUID:contact-1\r\nFN:Contact One\r\nEND:VCARD\r\n")
	etag, err := ContactObjectETag(body)
	if err != nil {
		t.Fatalf("ContactObjectETag returned error: %v", err)
	}
	req, gotETag, syncToken, err := ValidateUpsertContactObjectRequest(UpsertContactObjectRequest{
		UserID:        " user-1 ",
		AddressBookID: " book-1 ",
		ObjectName:    " contact-1.vcf ",
		UID:           " contact-1 ",
		VCard:         body,
		ObservedETag:  etag,
	})
	if err != nil {
		t.Fatalf("ValidateUpsertContactObjectRequest returned error: %v", err)
	}
	if req.UserID != "user-1" || req.AddressBookID != "book-1" || req.ObjectName != "contact-1.vcf" {
		t.Fatalf("request ids = %+v", req)
	}
	if req.UID != "contact-1" || req.ObservedETag != etag {
		t.Fatalf("request metadata = %+v", req)
	}
	if gotETag != etag {
		t.Fatalf("etag = %q, want %q", gotETag, etag)
	}
	if !strings.HasPrefix(syncToken, "sync-") {
		t.Fatalf("sync token = %q", syncToken)
	}
}

func TestValidateUpsertContactObjectRequestUsesVCardUIDWhenRequestUIDEmpty(t *testing.T) {
	t.Parallel()

	body := []byte("BEGIN:VCARD\r\nVERSION:4.0\r\nUID:contact-1\r\nFN:Contact One\r\nEND:VCARD\r\n")
	req, _, _, err := ValidateUpsertContactObjectRequest(UpsertContactObjectRequest{
		UserID:        "user-1",
		AddressBookID: "book-1",
		ObjectName:    "contact-1.vcf",
		VCard:         body,
	})
	if err != nil {
		t.Fatalf("ValidateUpsertContactObjectRequest returned error: %v", err)
	}
	if req.UID != "contact-1" {
		t.Fatalf("uid = %q", req.UID)
	}
}

func TestValidateUpsertContactObjectRequestRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	validBody := []byte("BEGIN:VCARD\r\nVERSION:4.0\r\nUID:contact-1\r\nFN:Contact One\r\nEND:VCARD\r\n")
	tests := []UpsertContactObjectRequest{
		{AddressBookID: "book-1", ObjectName: "contact-1.vcf", VCard: validBody},
		{UserID: "user-1", ObjectName: "contact-1.vcf", VCard: validBody},
		{UserID: "user-1", AddressBookID: "book-1", ObjectName: "contact-1.txt", VCard: validBody},
		{UserID: "user-1", AddressBookID: "book-1", ObjectName: "contact-1.vcf", UID: "other", VCard: validBody},
		{UserID: "user-1", AddressBookID: "book-1", ObjectName: "contact-1.vcf", VCard: []byte("BEGIN:VCARD\r\nVERSION:4.0\r\nFN:Contact\r\nEND:VCARD\r\n")},
		{UserID: "user-1", AddressBookID: "book-1", ObjectName: "contact-1.vcf", VCard: validBody, ObservedETag: `"ABC"`},
	}
	for _, req := range tests {
		req := req
		t.Run(req.ObjectName+"/"+req.UID, func(t *testing.T) {
			t.Parallel()

			if _, _, _, err := ValidateUpsertContactObjectRequest(req); err == nil {
				t.Fatalf("ValidateUpsertContactObjectRequest(%+v) error = nil, want rejection", req)
			}
		})
	}
}

func TestValidateContactObjectReadAndDeleteRequests(t *testing.T) {
	t.Parallel()

	list, err := ValidateListContactObjectsRequest(ListContactObjectsRequest{
		UserID:        " user-1 ",
		AddressBookID: " book-1 ",
		Limit:         2000,
	})
	if err != nil {
		t.Fatalf("ValidateListContactObjectsRequest returned error: %v", err)
	}
	if list.UserID != "user-1" || list.AddressBookID != "book-1" || list.Status != AddressBookStatusActive || list.Limit != 1000 {
		t.Fatalf("list request = %+v", list)
	}
	get, err := ValidateGetContactObjectRequest(GetContactObjectRequest{
		UserID:        " user-1 ",
		AddressBookID: " book-1 ",
		ObjectName:    " contact-1.vcf ",
		Status:        " deleted ",
	})
	if err != nil {
		t.Fatalf("ValidateGetContactObjectRequest returned error: %v", err)
	}
	if get.UserID != "user-1" || get.AddressBookID != "book-1" || get.ObjectName != "contact-1.vcf" || get.Status != AddressBookStatusDeleted {
		t.Fatalf("get request = %+v", get)
	}
	del, syncToken, err := ValidateDeleteContactObjectRequest(DeleteContactObjectRequest{
		UserID:        " user-1 ",
		AddressBookID: " book-1 ",
		ObjectName:    " contact-1.vcf ",
		ObservedETag:  ` "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" `,
	})
	if err != nil {
		t.Fatalf("ValidateDeleteContactObjectRequest returned error: %v", err)
	}
	if del.UserID != "user-1" ||
		del.AddressBookID != "book-1" ||
		del.ObjectName != "contact-1.vcf" ||
		del.ObservedETag != `"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"` ||
		!strings.HasPrefix(syncToken, "sync-") {
		t.Fatalf("delete request = %+v sync = %q", del, syncToken)
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
		{name: "upsert contact", run: func() error {
			_, err := repo.UpsertContactObject(ctx, UpsertContactObjectRequest{
				UserID:        "user-1",
				AddressBookID: "book-1",
				ObjectName:    "contact-1.vcf",
				VCard:         []byte("BEGIN:VCARD\r\nVERSION:4.0\r\nUID:contact-1\r\nFN:Contact One\r\nEND:VCARD\r\n"),
			})
			return err
		}},
		{name: "list contacts", run: func() error {
			_, err := repo.ListContactObjects(ctx, ListContactObjectsRequest{UserID: "user-1", AddressBookID: "book-1"})
			return err
		}},
		{name: "get contact", run: func() error {
			_, err := repo.GetContactObject(ctx, GetContactObjectRequest{UserID: "user-1", AddressBookID: "book-1", ObjectName: "contact-1.vcf"})
			return err
		}},
		{name: "delete contact", run: func() error {
			_, err := repo.DeleteContactObject(ctx, DeleteContactObjectRequest{UserID: "user-1", AddressBookID: "book-1", ObjectName: "contact-1.vcf"})
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
