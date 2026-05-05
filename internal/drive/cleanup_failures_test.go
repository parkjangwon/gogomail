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
