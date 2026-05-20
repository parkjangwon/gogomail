package drive

import (
	"strings"
	"testing"
)

func TestValidateObjectCleanupFailure(t *testing.T) {
	t.Parallel()

	failure, err := ValidateObjectCleanupFailure(ObjectCleanupFailure{
		UserID:         " user-1 ",
		NodeID:         " node-1 ",
		StorageBackend: " s3 ",
		StoragePath:    "drive/users/user-1/objects/node-1",
		LastError:      " delete failed\r\ntry later ",
	})
	if err != nil {
		t.Fatalf("ValidateObjectCleanupFailure returned error: %v", err)
	}
	if failure.UserID != "user-1" || failure.NodeID != "node-1" || failure.StorageBackend != "s3" {
		t.Fatalf("failure = %+v, want trimmed IDs/backend", failure)
	}
	if failure.LastError != "delete failed  try later" {
		t.Fatalf("LastError = %q, want one-line sanitized error", failure.LastError)
	}
}

func TestValidateObjectCleanupFailureTruncatesErrorAtUTF8Boundary(t *testing.T) {
	t.Parallel()

	failure, err := ValidateObjectCleanupFailure(ObjectCleanupFailure{
		UserID:         "user-1",
		StorageBackend: "s3",
		StoragePath:    "drive/users/user-1/objects/node-1",
		LastError:      strings.Repeat("가", maxObjectCleanupErrorBytes),
	})
	if err != nil {
		t.Fatalf("ValidateObjectCleanupFailure returned error: %v", err)
	}
	if len(failure.LastError) > maxObjectCleanupErrorBytes || !strings.HasPrefix(failure.LastError, "가") {
		t.Fatalf("LastError length/prefix = %d/%q, want bounded UTF-8", len(failure.LastError), failure.LastError[:3])
	}
}

func TestValidateObjectCleanupFailureRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	tests := []ObjectCleanupFailure{
		{StorageBackend: "s3", StoragePath: "drive/users/user-1/objects/node-1", LastError: "failed"},
		{UserID: "user\n1", StorageBackend: "s3", StoragePath: "drive/users/user-1/objects/node-1", LastError: "failed"},
		{UserID: "user-1", NodeID: "node\n1", StorageBackend: "s3", StoragePath: "drive/users/user-1/objects/node-1", LastError: "failed"},
		{UserID: "user-1", StorageBackend: "", StoragePath: "drive/users/user-1/objects/node-1", LastError: "failed"},
		{UserID: "user-1", StorageBackend: "s3", StoragePath: "../bad", LastError: "failed"},
		{UserID: "user-1", StorageBackend: "s3", StoragePath: "drive/users/user-2/objects/node-1", LastError: "failed"},
		{UserID: "user-1", StorageBackend: "s3", StoragePath: "drive/users/user-1/objects/node-1"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.UserID+"-"+tc.StorageBackend+"-"+tc.StoragePath, func(t *testing.T) {
			t.Parallel()

			if _, err := ValidateObjectCleanupFailure(tc); err == nil {
				t.Fatalf("ValidateObjectCleanupFailure(%+v) error = nil, want rejection", tc)
			}
		})
	}
}

func TestValidateListObjectCleanupFailuresRequest(t *testing.T) {
	t.Parallel()

	req, err := ValidateListObjectCleanupFailuresRequest(ListObjectCleanupFailuresRequest{
		UserID: " user-1 ",
		Status: " Resolved ",
		Limit:  500,
	})
	if err != nil {
		t.Fatalf("ValidateListObjectCleanupFailuresRequest returned error: %v", err)
	}
	if req.UserID != "user-1" || req.Status != ObjectCleanupFailureStatusResolved {
		t.Fatalf("request = %+v, want trimmed normalized fields", req)
	}
	if req.Limit != MaxObjectCleanupFailureListLimit {
		t.Fatalf("Limit = %d, want cap %d", req.Limit, MaxObjectCleanupFailureListLimit)
	}

	defaulted, err := ValidateListObjectCleanupFailuresRequest(ListObjectCleanupFailuresRequest{})
	if err != nil {
		t.Fatalf("ValidateListObjectCleanupFailuresRequest default returned error: %v", err)
	}
	if defaulted.Status != ObjectCleanupFailureStatusPending || defaulted.Limit != DefaultObjectCleanupFailureListLimit {
		t.Fatalf("defaulted request = %+v, want pending/default limit", defaulted)
	}
}

func TestValidateListObjectCleanupFailuresRequestRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	tests := []ListObjectCleanupFailuresRequest{
		{UserID: "user\n1"},
		{Status: "failed"},
		{Status: "pending\nbad"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.UserID+"-"+tc.Status, func(t *testing.T) {
			t.Parallel()

			if _, err := ValidateListObjectCleanupFailuresRequest(tc); err == nil {
				t.Fatalf("ValidateListObjectCleanupFailuresRequest(%+v) error = nil, want rejection", tc)
			}
		})
	}
}

func TestListObjectCleanupFailuresQueryUsesSargableOptionalFilters(t *testing.T) {
	t.Parallel()

	query, args := buildListObjectCleanupFailuresQuery(ListObjectCleanupFailuresRequest{
		UserID: "user-1",
		Status: ObjectCleanupFailureStatusPending,
		Limit:  25,
	})
	for _, want := range []string{
		"FROM drive_object_cleanup_failures",
		"WHERE status = $1",
		"AND user_id = $2::uuid",
		"ORDER BY updated_at ASC, id ASC",
		"LIMIT $3",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("list cleanup failures query missing %q:\n%s", want, query)
		}
	}
	for _, forbidden := range []string{
		"NULLIF($2, '') IS NULL",
		"OR user_id",
	} {
		if strings.Contains(query, forbidden) {
			t.Fatalf("list cleanup failures query contains non-sargable optional filter %q:\n%s", forbidden, query)
		}
	}
	if len(args) != 3 {
		t.Fatalf("args len = %d, want 3", len(args))
	}
	if args[0] != ObjectCleanupFailureStatusPending || args[1] != "user-1" || args[2] != 25 {
		t.Fatalf("args = %#v", args)
	}

	query, args = buildListObjectCleanupFailuresQuery(ListObjectCleanupFailuresRequest{Status: ObjectCleanupFailureStatusResolved, Limit: 50})
	if strings.Contains(query, "user_id = $") {
		t.Fatalf("user-agnostic cleanup failure query unexpectedly includes user filter:\n%s", query)
	}
	if len(args) != 2 || args[0] != ObjectCleanupFailureStatusResolved || args[1] != 50 {
		t.Fatalf("unfiltered args = %#v", args)
	}
}

func TestValidateResolveObjectCleanupFailureRequest(t *testing.T) {
	t.Parallel()

	req, err := ValidateResolveObjectCleanupFailureRequest(ResolveObjectCleanupFailureRequest{ID: " cleanup-1 "})
	if err != nil {
		t.Fatalf("ValidateResolveObjectCleanupFailureRequest returned error: %v", err)
	}
	if req.ID != "cleanup-1" {
		t.Fatalf("ID = %q, want trimmed id", req.ID)
	}
}

func TestValidateResolveObjectCleanupFailureRequestRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	for _, id := range []string{"", "cleanup\n1", strings.Repeat("x", 129)} {
		id := id
		t.Run(id, func(t *testing.T) {
			t.Parallel()

			if _, err := ValidateResolveObjectCleanupFailureRequest(ResolveObjectCleanupFailureRequest{ID: id}); err == nil {
				t.Fatalf("ValidateResolveObjectCleanupFailureRequest(%q) error = nil, want rejection", id)
			}
		})
	}
}

func TestNormalizeObjectCleanupFailureIDsDeduplicatesAndDropsUnsafeIDs(t *testing.T) {
	t.Parallel()

	got := normalizeObjectCleanupFailureIDs([]string{
		" failure-1 ",
		"failure-1",
		"failure-2",
		"bad\nid",
		"",
	})
	if strings.Join(got, ",") != "failure-1,failure-2" {
		t.Fatalf("normalizeObjectCleanupFailureIDs = %v, want unique safe ids", got)
	}
}
