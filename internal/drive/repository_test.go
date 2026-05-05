package drive

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/gogomail/gogomail/internal/storage"
)

func TestValidateCreateFolderRequest(t *testing.T) {
	t.Parallel()

	req, normalizedName, err := ValidateCreateFolderRequest(CreateFolderRequest{
		UserID:   " user-1 ",
		ParentID: " parent-1 ",
		Name:     "  Reports  ",
	})
	if err != nil {
		t.Fatalf("ValidateCreateFolderRequest returned error: %v", err)
	}
	if req.UserID != "user-1" || req.ParentID != "parent-1" || req.Name != "Reports" {
		t.Fatalf("request = %+v, want trimmed fields", req)
	}
	if normalizedName != "reports" {
		t.Fatalf("normalized name = %q, want reports", normalizedName)
	}
}

func TestValidateCreateFolderRequestRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	tests := []CreateFolderRequest{
		{Name: "Reports"},
		{UserID: "user-1", ParentID: "parent\n1", Name: "Reports"},
		{UserID: strings.Repeat("u", 129), Name: "Reports"},
		{UserID: "user-1", Name: ""},
		{UserID: "user-1", Name: "Reports/2026"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.UserID+"-"+tc.Name, func(t *testing.T) {
			t.Parallel()

			if _, _, err := ValidateCreateFolderRequest(tc); err == nil {
				t.Fatalf("ValidateCreateFolderRequest(%+v) error = nil, want rejection", tc)
			}
		})
	}
}

func TestValidateCreateFileFromObjectRequest(t *testing.T) {
	t.Parallel()

	req, normalizedName, err := ValidateCreateFileFromObjectRequest(CreateFileFromObjectRequest{
		UserID:         " user-1 ",
		ParentID:       " parent-1 ",
		Name:           "  Report.PDF  ",
		StorageBackend: " s3 ",
		StoragePath:    "drive/user-1/report.pdf",
		MIMEType:       "",
		ChecksumSHA256: strings.Repeat("A", 64),
	})
	if err != nil {
		t.Fatalf("ValidateCreateFileFromObjectRequest returned error: %v", err)
	}
	if req.UserID != "user-1" || req.ParentID != "parent-1" || req.Name != "Report.PDF" {
		t.Fatalf("request = %+v, want trimmed identity fields", req)
	}
	if req.StorageBackend != "s3" || req.StoragePath != "drive/user-1/report.pdf" {
		t.Fatalf("storage fields = %q/%q", req.StorageBackend, req.StoragePath)
	}
	if req.MIMEType != "application/octet-stream" {
		t.Fatalf("MIMEType = %q, want default application/octet-stream", req.MIMEType)
	}
	if req.ChecksumSHA256 != strings.Repeat("a", 64) {
		t.Fatalf("checksum = %q, want lowercased sha256", req.ChecksumSHA256)
	}
	if normalizedName != "report.pdf" {
		t.Fatalf("normalized name = %q, want report.pdf", normalizedName)
	}
}

func TestValidateCreateFileFromObjectRequestRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	tests := []CreateFileFromObjectRequest{
		{Name: "Report.pdf", StorageBackend: "s3", StoragePath: "drive/user-1/report.pdf"},
		{UserID: "user-1", Name: "Report.pdf", StorageBackend: "", StoragePath: "drive/user-1/report.pdf"},
		{UserID: "user-1", Name: "Report.pdf", StorageBackend: "s3\nbad", StoragePath: "drive/user-1/report.pdf"},
		{UserID: "user-1", Name: "Report.pdf", StorageBackend: "s3", StoragePath: "../bad"},
		{UserID: "user-1", Name: "Report.pdf", StorageBackend: "s3", StoragePath: "drive/user-1/report.pdf", MIMEType: "text/plain\nbad"},
		{UserID: "user-1", Name: "Report.pdf", StorageBackend: "s3", StoragePath: "drive/user-1/report.pdf", ChecksumSHA256: "not-sha"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.UserID+"-"+tc.StorageBackend+"-"+tc.StoragePath, func(t *testing.T) {
			t.Parallel()

			if _, _, err := ValidateCreateFileFromObjectRequest(tc); err == nil {
				t.Fatalf("ValidateCreateFileFromObjectRequest(%+v) error = nil, want rejection", tc)
			}
		})
	}
}

func TestValidateListNodesRequest(t *testing.T) {
	t.Parallel()

	req, err := ValidateListNodesRequest(ListNodesRequest{
		UserID:   " user-1 ",
		ParentID: " parent-1 ",
		Status:   " Trashed ",
		Limit:    500,
	})
	if err != nil {
		t.Fatalf("ValidateListNodesRequest returned error: %v", err)
	}
	if req.UserID != "user-1" || req.ParentID != "parent-1" || req.Status != NodeStatusTrashed {
		t.Fatalf("request = %+v, want trimmed status-normalized request", req)
	}
	if req.Limit != 200 {
		t.Fatalf("Limit = %d, want max cap 200", req.Limit)
	}

	defaulted, err := ValidateListNodesRequest(ListNodesRequest{UserID: "user-1"})
	if err != nil {
		t.Fatalf("ValidateListNodesRequest default returned error: %v", err)
	}
	if defaulted.Status != NodeStatusActive || defaulted.Limit != 50 {
		t.Fatalf("defaulted request = %+v, want active/50", defaulted)
	}
}

func TestValidateListNodesRequestRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	tests := []ListNodesRequest{
		{Status: NodeStatusActive},
		{UserID: "user\n1", Status: NodeStatusActive},
		{UserID: "user-1", ParentID: "parent\n1", Status: NodeStatusActive},
		{UserID: "user-1", Status: "archived"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.UserID+"-"+tc.Status, func(t *testing.T) {
			t.Parallel()

			if _, err := ValidateListNodesRequest(tc); err == nil {
				t.Fatalf("ValidateListNodesRequest(%+v) error = nil, want rejection", tc)
			}
		})
	}
}

func TestValidateTrashNodeRequest(t *testing.T) {
	t.Parallel()

	req, err := ValidateTrashNodeRequest(TrashNodeRequest{UserID: " user-1 ", NodeID: " node-1 "})
	if err != nil {
		t.Fatalf("ValidateTrashNodeRequest returned error: %v", err)
	}
	if req.UserID != "user-1" || req.NodeID != "node-1" {
		t.Fatalf("request = %+v, want trimmed IDs", req)
	}
}

func TestValidateTrashNodeRequestRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	tests := []TrashNodeRequest{
		{NodeID: "node-1"},
		{UserID: "user-1"},
		{UserID: "user\n1", NodeID: "node-1"},
		{UserID: "user-1", NodeID: "node\n1"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.UserID+"-"+tc.NodeID, func(t *testing.T) {
			t.Parallel()

			if _, err := ValidateTrashNodeRequest(tc); err == nil {
				t.Fatalf("ValidateTrashNodeRequest(%+v) error = nil, want rejection", tc)
			}
		})
	}
}

func TestCreateFileFromObjectRequiresStore(t *testing.T) {
	t.Parallel()

	repo := NewRepository(nil)
	_, err := repo.CreateFileFromObject(context.Background(), nil, CreateFileFromObjectRequest{})
	if err == nil || !strings.Contains(err.Error(), "database handle is required") {
		t.Fatalf("CreateFileFromObject err = %v, want database handle rejection first", err)
	}
}

type fakeStore struct {
	info storage.ObjectInfo
	err  error
}

func (s fakeStore) Put(context.Context, string, io.Reader) error {
	return nil
}

func (s fakeStore) Get(context.Context, string) (io.ReadCloser, error) {
	return nil, nil
}

func (s fakeStore) Stat(context.Context, string) (storage.ObjectInfo, error) {
	if s.err != nil {
		return storage.ObjectInfo{}, s.err
	}
	return s.info, nil
}

func (s fakeStore) Copy(context.Context, string, string) error {
	return nil
}

func (s fakeStore) Move(context.Context, string, string) error {
	return nil
}

func (s fakeStore) List(context.Context, storage.ListOptions) (storage.ObjectListPage, error) {
	return storage.ObjectListPage{}, nil
}

func (s fakeStore) Delete(context.Context, string) error {
	return nil
}
