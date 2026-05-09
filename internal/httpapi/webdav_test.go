package httpapi

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/drive"
)

func TestWebDAVPropfindListsNodes(t *testing.T) {
	t.Parallel()

	service := &fakeWebDAVService{
		nodes: []drive.Node{
			{ID: "folder-1", Name: "Reports", Type: drive.NodeTypeFolder, Status: drive.NodeStatusActive, UpdatedAt: timeNow()},
			{ID: "file-1", Name: "doc.pdf", Type: drive.NodeTypeFile, Status: drive.NodeStatusActive, Size: 2048, MIMEType: "application/pdf", UpdatedAt: timeNow()},
		},
	}
	mux := http.NewServeMux()
	RegisterWebDAVRoutes(mux, service, WebDAVRouteOptions{})

	req := httptest.NewRequest(	"PROPFIND", "/dav/?user_id=user-1", nil)
	req.Header.Set("Depth", "1")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/xml; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want application/xml", ct)
	}

	body := rec.Body.String()
	if !contains(body, "folder-1") || !contains(body, "Reports") {
		t.Fatalf("response missing folder entry: %s", body)
	}
	if !contains(body, "file-1") || !contains(body, "doc.pdf") {
		t.Fatalf("response missing file entry: %s", body)
	}
}

func TestWebDAVPropfindRejectsUnauthenticated(t *testing.T) {
	t.Parallel()

	service := &fakeWebDAVService{}
	mux := http.NewServeMux()
	RegisterWebDAVRoutes(mux, service, WebDAVRouteOptions{})

	req := httptest.NewRequest(	"PROPFIND", "/dav/", nil)
	req.Header.Set("Depth", "1")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestWebDAVOptionsReturnsDavHeader(t *testing.T) {
	t.Parallel()

	service := &fakeWebDAVService{}
	mux := http.NewServeMux()
	RegisterWebDAVRoutes(mux, service, WebDAVRouteOptions{})

	req := httptest.NewRequest(http.MethodOptions, "/dav/", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if dav := rec.Header().Get("DAV"); dav != "1, 3" {
		t.Fatalf("DAV = %q, want %q", dav, "1, 3")
	}
	allow := rec.Header().Get("Allow")
	if !contains(allow, "PROPFIND") || !contains(allow, "MKCOL") || !contains(allow, "DELETE") {
		t.Fatalf("Allow = %q, missing WebDAV methods", allow)
	}
}

func TestWebDAVLockCreatesLock(t *testing.T) {
	t.Parallel()

	service := &fakeWebDAVService{}
	mux := http.NewServeMux()
	RegisterWebDAVRoutes(mux, service, WebDAVRouteOptions{})

	req := httptest.NewRequest("LOCK", "/dav/node-1?user_id=user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	lockToken := rec.Header().Get("Lock-Token")
	if lockToken == "" {
		t.Fatal("Lock-Token header missing")
	}
	if !strings.Contains(lockToken, "urn:uuid:") {
		t.Fatalf("Lock-Token = %q, want to contain urn:uuid:", lockToken)
	}
}

func TestWebDAVUnlockReleasesLock(t *testing.T) {
	t.Parallel()

	service := &fakeWebDAVService{}
	mux := http.NewServeMux()
	RegisterWebDAVRoutes(mux, service, WebDAVRouteOptions{})

	lockReq := httptest.NewRequest("LOCK", "/dav/node-1?user_id=user-1", nil)
	lockRec := httptest.NewRecorder()
	mux.ServeHTTP(lockRec, lockReq)
	if lockRec.Code != http.StatusOK {
		t.Fatalf("LOCK status = %d, want %d", lockRec.Code, http.StatusOK)
	}
	lockToken := lockRec.Header().Get("Lock-Token")

	unlockReq := httptest.NewRequest("UNLOCK", "/dav/node-1?user_id=user-1", nil)
	unlockReq.Header.Set("Lock-Token", lockToken)
	unlockRec := httptest.NewRecorder()
	mux.ServeHTTP(unlockRec, unlockReq)

	if unlockRec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d, body = %s", unlockRec.Code, http.StatusNoContent, unlockRec.Body.String())
	}
}

func TestWebDAVDepthInfinityRejected(t *testing.T) {
	t.Parallel()

	service := &fakeWebDAVService{}
	mux := http.NewServeMux()
	RegisterWebDAVRoutes(mux, service, WebDAVRouteOptions{DepthInfinityEnabled: false})

	req := httptest.NewRequest("PROPFIND", "/dav/?user_id=user-1", nil)
	req.Header.Set("Depth", "infinity")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestWebDAVDepthInfinityAllowed(t *testing.T) {
	t.Parallel()

	service := &fakeWebDAVService{
		nodes: []drive.Node{
			{ID: "folder-1", Name: "Reports", Type: drive.NodeTypeFolder, Status: drive.NodeStatusActive, UpdatedAt: timeNow()},
		},
	}
	mux := http.NewServeMux()
	RegisterWebDAVRoutes(mux, service, WebDAVRouteOptions{DepthInfinityEnabled: true})

	req := httptest.NewRequest("PROPFIND", "/dav/?user_id=user-1", nil)
	req.Header.Set("Depth", "infinity")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMultiStatus)
	}
}

func TestWebDAVMkcolCreatesFolder(t *testing.T) {
	t.Parallel()

	service := &fakeWebDAVService{
		folder: drive.Node{ID: "new-folder-1", Name: "NewFolder", Type: drive.NodeTypeFolder, Status: drive.NodeStatusActive},
	}
	mux := http.NewServeMux()
	RegisterWebDAVRoutes(mux, service, WebDAVRouteOptions{})

	req := httptest.NewRequest("MKCOL", "/dav/NewFolder?user_id=user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); loc != "/dav/new-folder-1/" {
		t.Fatalf("Location = %q, want %q", loc, "/dav/new-folder-1/")
	}
	if service.createReq.UserID != "user-1" || service.createReq.Name != "NewFolder" {
		t.Fatalf("createReq = %+v, want user_id=user-1, name=NewFolder", service.createReq)
	}
}

func TestWebDAVMkcolWithParentAndName(t *testing.T) {
	t.Parallel()

	service := &fakeWebDAVService{
		folder: drive.Node{ID: "folder-2", Name: "SubFolder", Type: drive.NodeTypeFolder, Status: drive.NodeStatusActive},
	}
	mux := http.NewServeMux()
	RegisterWebDAVRoutes(mux, service, WebDAVRouteOptions{})

	// /dav/parent-id/subfolder-name
	req := httptest.NewRequest(	"MKCOL", "/dav/parent-1/SubFolder?user_id=user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.createReq.ParentID != "parent-1" || service.createReq.Name != "SubFolder" {
		t.Fatalf("createReq = %+v, want parent_id=parent-1, name=SubFolder", service.createReq)
	}
}

func TestWebDAVDeleteTrashesNode(t *testing.T) {
	t.Parallel()

	service := &fakeWebDAVService{}
	mux := http.NewServeMux()
	RegisterWebDAVRoutes(mux, service, WebDAVRouteOptions{})

	// /dav/node-id
	req := httptest.NewRequest(http.MethodDelete, "/dav/node-1?user_id=user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.trashReq.UserID != "user-1" || service.trashReq.NodeID != "node-1" {
		t.Fatalf("trashReq = %+v, want user_id=user-1, node_id=node-1", service.trashReq)
	}
}

func TestWebDAVMoveMovesNode(t *testing.T) {
	t.Parallel()

	service := &fakeWebDAVService{}
	mux := http.NewServeMux()
	RegisterWebDAVRoutes(mux, service, WebDAVRouteOptions{})

	// /dav/node-id -> /dav/new-parent/
	req := httptest.NewRequest(	"MOVE", "/dav/node-1?user_id=user-1", nil)
	req.Header.Set("Destination", "http://localhost/dav/new-parent/")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.moveReq.UserID != "user-1" || service.moveReq.NodeID != "node-1" || service.moveReq.ParentID != "new-parent" {
		t.Fatalf("moveReq = %+v", service.moveReq)
	}
}

func TestWebDAVMoveRequiresDestination(t *testing.T) {
	t.Parallel()

	service := &fakeWebDAVService{}
	mux := http.NewServeMux()
	RegisterWebDAVRoutes(mux, service, WebDAVRouteOptions{})

	req := httptest.NewRequest(	"MOVE", "/dav/node-1?user_id=user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestWebDAVCopyCopiesNode(t *testing.T) {
	t.Parallel()

	service := &fakeWebDAVService{}
	mux := http.NewServeMux()
	RegisterWebDAVRoutes(mux, service, WebDAVRouteOptions{})

	// /dav/node-id -> /dav/target-parent/copy-name
	req := httptest.NewRequest(	"COPY", "/dav/node-1?user_id=user-1", nil)
	req.Header.Set("Destination", "http://localhost/dav/target-parent/copy-name")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.copyReq.UserID != "user-1" || service.copyReq.NodeID != "node-1" || service.copyReq.ParentID != "target-parent" || service.copyReq.Name != "copy-name" {
		t.Fatalf("copyReq = %+v", service.copyReq)
	}
}

func TestWebDAVPutReturnsNotImplemented(t *testing.T) {
	t.Parallel()

	service := &fakeWebDAVService{}
	mux := http.NewServeMux()
	RegisterWebDAVRoutes(mux, service, WebDAVRouteOptions{})

	req := httptest.NewRequest(http.MethodPut, "/dav/file.txt?user_id=user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotImplemented)
	}
}

func TestWebDAVGetDownloadsFile(t *testing.T) {
	t.Parallel()

	service := &fakeWebDAVService{
		download: drive.FileDownload{
			Node: drive.Node{ID: "file-1", Name: "doc.pdf", MIMEType: "application/pdf", Size: 2048},
			Body: io.NopCloser(strings.NewReader("PDF content")),
		},
	}
	mux := http.NewServeMux()
	RegisterWebDAVRoutes(mux, service, WebDAVRouteOptions{})

	// /dav/file-1/doc.pdf
	req := httptest.NewRequest(http.MethodGet, "/dav/file-1/doc.pdf?user_id=user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/pdf" {
		t.Fatalf("Content-Type = %q, want %q", ct, "application/pdf")
	}
	if service.openReq.UserID != "user-1" || service.openReq.NodeID != "file-1" {
		t.Fatalf("openReq = %+v", service.openReq)
	}
}

type fakeWebDAVService struct {
	nodes       []drive.Node
	folder      drive.Node
	download    drive.FileDownload
	openReq     drive.OpenFileRequest
	listReq     drive.ListNodesRequest
	createReq   drive.CreateFolderRequest
	trashReq    drive.TrashNodeRequest
	moveReq     drive.MoveNodeRequest
	copyReq     drive.CopyNodeRequest
	renameReq   drive.RenameNodeRequest
	getNodeReq  drive.GetNodeRequest
	lockReq     drive.LockNodeRequest
	unlockReq   drive.UnlockNodeRequest
	lockNode    drive.Node
	unlocked    bool
	err         error
}

func (f *fakeWebDAVService) ListNodes(_ context.Context, req drive.ListNodesRequest) ([]drive.Node, error) {
	f.listReq = req
	if f.err != nil {
		return nil, f.err
	}
	return f.nodes, nil
}

func (f *fakeWebDAVService) GetNode(_ context.Context, req drive.GetNodeRequest) (drive.Node, error) {
	f.getNodeReq = req
	if f.err != nil {
		return drive.Node{}, f.err
	}
	return drive.Node{}, nil
}

func (f *fakeWebDAVService) OpenFile(_ context.Context, req drive.OpenFileRequest) (drive.FileDownload, error) {
	f.openReq = req
	if f.err != nil {
		return drive.FileDownload{}, f.err
	}
	return f.download, nil
}

func (f *fakeWebDAVService) CreateFolder(_ context.Context, req drive.CreateFolderRequest) (drive.Node, error) {
	f.createReq = req
	if f.err != nil {
		return drive.Node{}, f.err
	}
	return f.folder, nil
}

func (f *fakeWebDAVService) TrashNode(_ context.Context, req drive.TrashNodeRequest) error {
	f.trashReq = req
	return f.err
}

func (f *fakeWebDAVService) RenameNode(_ context.Context, req drive.RenameNodeRequest) (drive.Node, error) {
	f.renameReq = req
	if f.err != nil {
		return drive.Node{}, f.err
	}
	return f.folder, nil
}

func (f *fakeWebDAVService) MoveNode(_ context.Context, req drive.MoveNodeRequest) (drive.Node, error) {
	f.moveReq = req
	if f.err != nil {
		return drive.Node{}, f.err
	}
	return f.folder, nil
}

func (f *fakeWebDAVService) CopyNode(_ context.Context, req drive.CopyNodeRequest) (drive.Node, error) {
	f.copyReq = req
	if f.err != nil {
		return drive.Node{}, f.err
	}
	return f.folder, nil
}

func (f *fakeWebDAVService) GetUsageSummary(_ context.Context, req drive.GetUsageSummaryRequest) (drive.UsageSummary, error) {
	return drive.UsageSummary{}, nil
}

func (f *fakeWebDAVService) LockNode(_ context.Context, req drive.LockNodeRequest) (drive.LockToken, error) {
	f.lockReq = req
	if f.err != nil {
		return drive.LockToken{}, f.err
	}
	return drive.LockToken{Token: "test-lock-token"}, nil
}

func (f *fakeWebDAVService) UnlockNode(_ context.Context, req drive.UnlockNodeRequest) error {
	f.unlockReq = req
	return f.err
}

func timeNow() time.Time {
	return time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
