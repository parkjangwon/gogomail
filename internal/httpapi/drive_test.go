package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gogomail/gogomail/internal/drive"
)

func TestDriveListNodesHandler(t *testing.T) {
	t.Parallel()

	service := &fakeDriveService{nodes: []drive.Node{{ID: "node-1", Name: "Reports", Type: drive.NodeTypeFolder, Status: drive.NodeStatusActive}}}
	mux := http.NewServeMux()
	RegisterDriveRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/drive/nodes?user_id=user-1&parent_id=parent-1&status=active&limit=25", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.listReq.UserID != "user-1" || service.listReq.ParentID != "parent-1" || service.listReq.Status != "active" || service.listReq.Limit != 25 {
		t.Fatalf("list request = %+v, want query-backed request", service.listReq)
	}
	var body struct {
		Nodes []drive.Node `json:"drive_nodes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if len(body.Nodes) != 1 || body.Nodes[0].ID != "node-1" {
		t.Fatalf("nodes = %+v", body.Nodes)
	}
}

func TestDriveCreateFolderHandler(t *testing.T) {
	t.Parallel()

	service := &fakeDriveService{folder: drive.Node{ID: "folder-1", Name: "Reports", Type: drive.NodeTypeFolder, Status: drive.NodeStatusActive}}
	mux := http.NewServeMux()
	RegisterDriveRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/drive/folders?user_id=user-1", strings.NewReader(`{"parent_id":"parent-1","name":"Reports"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.createReq.UserID != "user-1" || service.createReq.ParentID != "parent-1" || service.createReq.Name != "Reports" {
		t.Fatalf("create request = %+v, want request body/user", service.createReq)
	}
}

func TestDriveFinalizeFileHandler(t *testing.T) {
	t.Parallel()

	service := &fakeDriveService{file: drive.Node{ID: "file-1", Name: "report.pdf", Type: drive.NodeTypeFile, Status: drive.NodeStatusActive}}
	mux := http.NewServeMux()
	RegisterDriveRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/drive/files/finalize?user_id=user-1", strings.NewReader(`{
		"parent_id":"parent-1",
		"name":"report.pdf",
		"storage_backend":"s3",
		"storage_path":"drive/users/user-1/staging/upload-1",
		"mime_type":"application/pdf",
		"checksum_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.fileReq.UserID != "user-1" || service.fileReq.ParentID != "parent-1" || service.fileReq.StorageBackend != "s3" || service.fileReq.StoragePath != "drive/users/user-1/staging/upload-1" {
		t.Fatalf("file request = %+v, want finalize body/user", service.fileReq)
	}
}

func TestDriveLifecycleHandlers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		assert     func(*testing.T, *fakeDriveService)
	}{
		{
			name:       "trash",
			method:     http.MethodPost,
			path:       "/api/v1/drive/nodes/node-1/trash?user_id=user-1",
			wantStatus: http.StatusOK,
			assert: func(t *testing.T, service *fakeDriveService) {
				t.Helper()
				if service.trashReq.UserID != "user-1" || service.trashReq.NodeID != "node-1" {
					t.Fatalf("trash request = %+v", service.trashReq)
				}
			},
		},
		{
			name:       "restore",
			method:     http.MethodPost,
			path:       "/api/v1/drive/nodes/node-1/restore?user_id=user-1",
			wantStatus: http.StatusOK,
			assert: func(t *testing.T, service *fakeDriveService) {
				t.Helper()
				if service.restoreReq.UserID != "user-1" || service.restoreReq.NodeID != "node-1" {
					t.Fatalf("restore request = %+v", service.restoreReq)
				}
			},
		},
		{
			name:       "permanent delete",
			method:     http.MethodDelete,
			path:       "/api/v1/drive/nodes/node-1?user_id=user-1",
			wantStatus: http.StatusOK,
			assert: func(t *testing.T, service *fakeDriveService) {
				t.Helper()
				if service.deleteReq.UserID != "user-1" || service.deleteReq.NodeID != "node-1" {
					t.Fatalf("delete request = %+v", service.deleteReq)
				}
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := &fakeDriveService{node: drive.Node{ID: "node-1", UserID: "user-1"}}
			mux := http.NewServeMux()
			RegisterDriveRoutes(mux, service, nil)

			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			tt.assert(t, service)
		})
	}
}

func TestDriveHandlersRejectBadRequests(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		req  *http.Request
	}{
		{name: "list unknown query", req: httptest.NewRequest(http.MethodGet, "/api/v1/drive/nodes?user_id=user-1&typo=true", nil)},
		{name: "list duplicate parent", req: httptest.NewRequest(http.MethodGet, "/api/v1/drive/nodes?user_id=user-1&parent_id=a&parent_id=b", nil)},
		{name: "create invalid json", req: httptest.NewRequest(http.MethodPost, "/api/v1/drive/folders?user_id=user-1", strings.NewReader(`{`))},
		{name: "finalize invalid json", req: httptest.NewRequest(http.MethodPost, "/api/v1/drive/files/finalize?user_id=user-1", strings.NewReader(`{`))},
		{name: "trash body rejected", req: httptest.NewRequest(http.MethodPost, "/api/v1/drive/nodes/node-1/trash?user_id=user-1", strings.NewReader(`{}`))},
		{name: "delete unsafe id", req: httptest.NewRequest(http.MethodDelete, "/api/v1/drive/nodes/node%0A1?user_id=user-1", nil)},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := &fakeDriveService{}
			mux := http.NewServeMux()
			RegisterDriveRoutes(mux, service, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, tt.req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
		})
	}
}

type fakeDriveService struct {
	nodes      []drive.Node
	node       drive.Node
	folder     drive.Node
	file       drive.Node
	err        error
	listReq    drive.ListNodesRequest
	createReq  drive.CreateFolderRequest
	fileReq    drive.CreateFileFromObjectRequest
	trashReq   drive.TrashNodeRequest
	restoreReq drive.RestoreNodeRequest
	deleteReq  drive.PermanentDeleteNodeRequest
}

func (f *fakeDriveService) CreateFolder(_ context.Context, req drive.CreateFolderRequest) (drive.Node, error) {
	f.createReq = req
	if f.err != nil {
		return drive.Node{}, f.err
	}
	return f.folder, nil
}

func (f *fakeDriveService) ListNodes(_ context.Context, req drive.ListNodesRequest) ([]drive.Node, error) {
	f.listReq = req
	if f.err != nil {
		return nil, f.err
	}
	return f.nodes, nil
}

func (f *fakeDriveService) CreateFileFromObject(_ context.Context, req drive.CreateFileFromObjectRequest) (drive.Node, error) {
	f.fileReq = req
	if f.err != nil {
		return drive.Node{}, f.err
	}
	return f.file, nil
}

func (f *fakeDriveService) TrashNode(_ context.Context, req drive.TrashNodeRequest) (drive.Node, int64, error) {
	f.trashReq = req
	if f.err != nil {
		return drive.Node{}, 0, f.err
	}
	return f.node, 1, nil
}

func (f *fakeDriveService) RestoreNode(_ context.Context, req drive.RestoreNodeRequest) (drive.Node, int64, error) {
	f.restoreReq = req
	if f.err != nil {
		return drive.Node{}, 0, f.err
	}
	return f.node, 1, nil
}

func (f *fakeDriveService) PermanentDeleteNode(_ context.Context, req drive.PermanentDeleteNodeRequest) (drive.PermanentDeleteServiceResult, error) {
	f.deleteReq = req
	if f.err != nil {
		return drive.PermanentDeleteServiceResult{}, f.err
	}
	return drive.PermanentDeleteServiceResult{PermanentDelete: drive.PermanentDeleteResult{Root: f.node, DeletedNodes: 1}}, nil
}

var _ DriveService = (*fakeDriveService)(nil)
var errDriveTest = errors.New("drive test error")
