package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gogomail/gogomail/internal/drive"
	"github.com/gogomail/gogomail/internal/storage"
)

func TestDriveListNodesHandler(t *testing.T) {
	t.Parallel()

	service := &fakeDriveService{nodes: []drive.Node{{ID: "node-1", Name: "Reports", Type: drive.NodeTypeFolder, Status: drive.NodeStatusActive}}}
	mux := http.NewServeMux()
	RegisterDriveRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/drive/nodes?user_id=user-1&parent_id=parent-1&status=active&q=%20Report%20&limit=25", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.listReq.UserID != "user-1" || service.listReq.ParentID != "parent-1" || service.listReq.Status != "active" || service.listReq.Query != "Report" || service.listReq.Limit != 25 {
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

func TestDriveGetNodeHandler(t *testing.T) {
	t.Parallel()

	service := &fakeDriveService{node: drive.Node{ID: "node-1", UserID: "user-1", Name: "Report.pdf", Type: drive.NodeTypeFile, Status: drive.NodeStatusActive}}
	mux := http.NewServeMux()
	RegisterDriveRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/drive/nodes/node-1?user_id=user-1&status=active", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.getReq.UserID != "user-1" || service.getReq.NodeID != "node-1" || service.getReq.Status != "active" {
		t.Fatalf("get request = %+v, want query-backed request", service.getReq)
	}
	var body struct {
		Node drive.Node `json:"drive_node"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if body.Node.ID != "node-1" {
		t.Fatalf("node = %+v", body.Node)
	}
}

func TestDriveDownloadNodeHandler(t *testing.T) {
	t.Parallel()

	service := &fakeDriveService{download: drive.FileDownload{
		Node: drive.Node{ID: "node-1", UserID: "user-1", Name: "보고서.pdf", Type: drive.NodeTypeFile, MIMEType: "application/pdf", Size: 7, ChecksumSHA256: strings.Repeat("A", 64), Status: drive.NodeStatusActive},
		Body: io.NopCloser(strings.NewReader("content")),
	}}
	mux := http.NewServeMux()
	RegisterDriveRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/drive/nodes/node-1/download?user_id=user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.openReq.UserID != "user-1" || service.openReq.NodeID != "node-1" {
		t.Fatalf("open request = %+v, want user/node", service.openReq)
	}
	if rec.Body.String() != "content" {
		t.Fatalf("body = %q", rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/pdf" {
		t.Fatalf("Content-Type = %q", got)
	}
	if got := rec.Header().Get("Content-Disposition"); !strings.Contains(got, `filename*=UTF-8''%EB%B3%B4%EA%B3%A0%EC%84%9C.pdf`) {
		t.Fatalf("Content-Disposition = %q", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q", got)
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q", got)
	}
	if got := rec.Header().Get("Content-Length"); got != "7" {
		t.Fatalf("Content-Length = %q", got)
	}
	if got := rec.Header().Get("X-Gogomail-Drive-SHA256"); got != strings.Repeat("a", 64) {
		t.Fatalf("X-Gogomail-Drive-SHA256 = %q", got)
	}
}

func TestDriveDownloadNodeHandlerSupportsByteRange(t *testing.T) {
	t.Parallel()

	service := &fakeDriveService{
		metadata: drive.FileMetadata{
			Node:   drive.Node{ID: "node-1", UserID: "user-1", Name: "report.pdf", Type: drive.NodeTypeFile, MIMEType: "application/pdf", Size: 99, ChecksumSHA256: strings.Repeat("b", 64), Status: drive.NodeStatusActive},
			Object: storage.ObjectInfo{Path: "drive/users/user-1/objects/node-1", Size: 7},
		},
		rangeDownload: drive.FileDownload{
			Node: drive.Node{ID: "node-1", UserID: "user-1", Name: "report.pdf", Type: drive.NodeTypeFile, MIMEType: "application/pdf", Size: 99, ChecksumSHA256: strings.Repeat("b", 64), Status: drive.NodeStatusActive},
			Body: io.NopCloser(strings.NewReader("nte")),
		},
	}
	mux := http.NewServeMux()
	RegisterDriveRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/drive/nodes/node-1/download?user_id=user-1", nil)
	req.Header.Set("Range", "bytes=2-4")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusPartialContent {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.statReq.UserID != "user-1" || service.statReq.NodeID != "node-1" {
		t.Fatalf("stat request = %+v, want user/node", service.statReq)
	}
	if service.openRangeReq.UserID != "user-1" || service.openRangeReq.NodeID != "node-1" || service.openRangeReq.Offset != 2 || service.openRangeReq.Length != 3 {
		t.Fatalf("range request = %+v, want offset/length", service.openRangeReq)
	}
	if rec.Body.String() != "nte" {
		t.Fatalf("body = %q", rec.Body.String())
	}
	if got := rec.Header().Get("Content-Range"); got != "bytes 2-4/7" {
		t.Fatalf("Content-Range = %q", got)
	}
	if got := rec.Header().Get("Content-Length"); got != "3" {
		t.Fatalf("Content-Length = %q", got)
	}
	if got := rec.Header().Get("Accept-Ranges"); got != "bytes" {
		t.Fatalf("Accept-Ranges = %q", got)
	}
	if got := rec.Header().Get("X-Gogomail-Drive-SHA256"); got != strings.Repeat("b", 64) {
		t.Fatalf("X-Gogomail-Drive-SHA256 = %q", got)
	}
}

func TestDriveDownloadNodeHandlerRejectsUnsatisfiableByteRange(t *testing.T) {
	t.Parallel()

	service := &fakeDriveService{metadata: drive.FileMetadata{
		Node:   drive.Node{ID: "node-1", UserID: "user-1", Name: "report.pdf", Type: drive.NodeTypeFile, MIMEType: "application/pdf", Size: 7, Status: drive.NodeStatusActive},
		Object: storage.ObjectInfo{Path: "drive/users/user-1/objects/node-1", Size: 7},
	}}
	mux := http.NewServeMux()
	RegisterDriveRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/drive/nodes/node-1/download?user_id=user-1", nil)
	req.Header.Set("Range", "bytes=9-10")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestedRangeNotSatisfiable {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Range"); got != "bytes */7" {
		t.Fatalf("Content-Range = %q", got)
	}
	if service.openRangeReq.NodeID != "" {
		t.Fatalf("range open should not run for unsatisfiable range: %+v", service.openRangeReq)
	}
}

func TestParseSingleHTTPByteRange(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		header string
		total  int64
		offset int64
		length int64
	}{
		{header: "bytes=2-4", total: 7, offset: 2, length: 3},
		{header: "bytes=4-", total: 7, offset: 4, length: 3},
		{header: "bytes=-3", total: 7, offset: 4, length: 3},
		{header: "bytes=0-99", total: 7, offset: 0, length: 7},
	} {
		got, err := parseSingleHTTPByteRange(tc.header, tc.total)
		if err != nil {
			t.Fatalf("parseSingleHTTPByteRange(%q) returned error: %v", tc.header, err)
		}
		if got.Offset != tc.offset || got.Length != tc.length || got.Total != tc.total {
			t.Fatalf("parseSingleHTTPByteRange(%q) = %+v", tc.header, got)
		}
	}

	for _, header := range []string{
		"bytes=7-8",
		"bytes=4-2",
		"bytes=-0",
		"bytes=1-2,4-5",
		"items=0-1",
		"bytes=+1-2",
	} {
		if _, err := parseSingleHTTPByteRange(header, 7); err == nil {
			t.Fatalf("parseSingleHTTPByteRange(%q) succeeded, want error", header)
		}
	}
}

func TestDriveHeadDownloadNodeHandler(t *testing.T) {
	t.Parallel()

	service := &fakeDriveService{metadata: drive.FileMetadata{
		Node:   drive.Node{ID: "node-1", UserID: "user-1", Name: "report.pdf", Type: drive.NodeTypeFile, MIMEType: "application/pdf", Size: 1, ChecksumSHA256: strings.Repeat("c", 64), Status: drive.NodeStatusActive},
		Object: storage.ObjectInfo{Path: "drive/users/user-1/objects/node-1", Size: 7},
	}}
	mux := http.NewServeMux()
	RegisterDriveRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodHead, "/api/v1/drive/nodes/node-1/download?user_id=user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.statReq.UserID != "user-1" || service.statReq.NodeID != "node-1" {
		t.Fatalf("stat request = %+v, want user/node", service.statReq)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("HEAD body length = %d, want 0", rec.Body.Len())
	}
	if got := rec.Header().Get("Content-Length"); got != "7" {
		t.Fatalf("Content-Length = %q", got)
	}
	if got := rec.Header().Get("Content-Disposition"); !strings.Contains(got, `filename="report.pdf"`) {
		t.Fatalf("Content-Disposition = %q", got)
	}
	if got := rec.Header().Get("X-Gogomail-Drive-SHA256"); got != strings.Repeat("c", 64) {
		t.Fatalf("X-Gogomail-Drive-SHA256 = %q", got)
	}
}

func TestDriveDownloadHeadersOmitUnsafeChecksum(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	writeDriveFileDownloadHeaders(rec, drive.Node{
		Name:           "report.pdf",
		Type:           drive.NodeTypeFile,
		MIMEType:       "application/pdf",
		Size:           7,
		ChecksumSHA256: "bad\r\nheader",
	})
	if got := rec.Header().Get("X-Gogomail-Drive-SHA256"); got != "" {
		t.Fatalf("X-Gogomail-Drive-SHA256 = %q, want omitted", got)
	}
}

func TestDriveUsageHandler(t *testing.T) {
	t.Parallel()

	service := &fakeDriveService{usageSummary: drive.UsageSummary{UserID: "user-1", QuotaUsed: 2048, ActiveBytes: 1024, ActiveNodes: 3}}
	mux := http.NewServeMux()
	RegisterDriveRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/drive/usage?user_id=user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.usageReq.UserID != "user-1" {
		t.Fatalf("usage request = %+v, want user-backed request", service.usageReq)
	}
	var body struct {
		Summary drive.UsageSummary `json:"drive_usage_summary"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if body.Summary.UserID != "user-1" || body.Summary.ActiveBytes != 1024 {
		t.Fatalf("summary = %+v", body.Summary)
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

func TestDriveCreateUploadSessionHandler(t *testing.T) {
	t.Parallel()

	service := &fakeDriveService{uploadSession: drive.UploadSession{
		ID:             "session-1",
		UserID:         "user-1",
		ParentID:       "parent-1",
		UploadID:       "upload-1",
		Name:           "Report.pdf",
		DeclaredSize:   123,
		MIMEType:       "application/pdf",
		Status:         drive.UploadSessionStatusPending,
		StorageBackend: "s3",
	}}
	mux := http.NewServeMux()
	RegisterDriveRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/drive/upload-sessions?user_id=user-1", strings.NewReader(`{
		"parent_id":"parent-1",
		"name":"Report.pdf",
		"declared_size":123,
		"mime_type":"application/pdf",
		"storage_backend":"s3",
		"expires_at":"2026-05-07T12:00:00Z"
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.uploadSessionReq.UserID != "user-1" || service.uploadSessionReq.ParentID != "parent-1" || service.uploadSessionReq.StorageBackend != "s3" {
		t.Fatalf("upload session request = %+v, want request body/user", service.uploadSessionReq)
	}
	if service.uploadSessionReq.ExpiresAt.IsZero() {
		t.Fatalf("upload session request = %+v, want parsed expires_at", service.uploadSessionReq)
	}
	var body struct {
		Session drive.UploadSession `json:"drive_upload_session"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if body.Session.ID != "session-1" {
		t.Fatalf("session = %+v", body.Session)
	}
}

func TestDriveGetUploadSessionHandler(t *testing.T) {
	t.Parallel()

	service := &fakeDriveService{uploadSession: drive.UploadSession{
		ID:             "session-1",
		UserID:         "user-1",
		UploadID:       "upload-1",
		Name:           "Report.pdf",
		Status:         drive.UploadSessionStatusPending,
		StorageBackend: "s3",
	}}
	mux := http.NewServeMux()
	RegisterDriveRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/drive/upload-sessions/session-1?user_id=user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.getUploadSessionReq.UserID != "user-1" || service.getUploadSessionReq.SessionID != "session-1" {
		t.Fatalf("get upload session request = %+v, want user/session", service.getUploadSessionReq)
	}
	var body struct {
		Session drive.UploadSession `json:"drive_upload_session"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if body.Session.ID != "session-1" {
		t.Fatalf("session = %+v", body.Session)
	}
}

func TestDriveListUploadSessionsHandler(t *testing.T) {
	t.Parallel()

	service := &fakeDriveService{uploadSessions: []drive.UploadSession{{
		ID:             "session-1",
		UserID:         "user-1",
		UploadID:       "upload-1",
		Name:           "Report.pdf",
		Status:         drive.UploadSessionStatusUploading,
		StorageBackend: "s3",
	}}}
	mux := http.NewServeMux()
	RegisterDriveRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/drive/upload-sessions?user_id=user-1&status=uploading&limit=10", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.listUploadSessionReq.UserID != "user-1" || service.listUploadSessionReq.Status != drive.UploadSessionStatusUploading || service.listUploadSessionReq.Limit != 10 {
		t.Fatalf("list upload session request = %+v, want query-backed request", service.listUploadSessionReq)
	}
	var body struct {
		Sessions []drive.UploadSession `json:"drive_upload_sessions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if len(body.Sessions) != 1 || body.Sessions[0].ID != "session-1" {
		t.Fatalf("sessions = %+v", body.Sessions)
	}
}

func TestDriveCancelUploadSessionHandler(t *testing.T) {
	t.Parallel()

	service := &fakeDriveService{uploadSession: drive.UploadSession{
		ID:             "session-1",
		UserID:         "user-1",
		UploadID:       "upload-1",
		Name:           "Report.pdf",
		Status:         drive.UploadSessionStatusCanceled,
		StorageBackend: "s3",
	}}
	mux := http.NewServeMux()
	RegisterDriveRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/drive/upload-sessions/session-1?user_id=user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.cancelUploadSessionReq.UserID != "user-1" || service.cancelUploadSessionReq.SessionID != "session-1" {
		t.Fatalf("cancel upload session request = %+v, want user/session", service.cancelUploadSessionReq)
	}
	var body struct {
		Session drive.UploadSession `json:"drive_upload_session"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if body.Session.Status != drive.UploadSessionStatusCanceled {
		t.Fatalf("session = %+v", body.Session)
	}
}

func TestDriveStoreUploadSessionBodyHandler(t *testing.T) {
	t.Parallel()

	service := &fakeDriveService{uploadSession: drive.UploadSession{
		ID:             "session-1",
		UserID:         "user-1",
		UploadID:       "upload-1",
		Name:           "Report.pdf",
		ReceivedSize:   11,
		Status:         drive.UploadSessionStatusUploading,
		StorageBackend: "s3",
		ChecksumSHA256: strings.Repeat("a", 64),
	}}
	mux := http.NewServeMux()
	RegisterDriveRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/drive/upload-sessions/session-1/body?user_id=user-1", strings.NewReader("hello drive"))
	req.Header.Set("X-Content-SHA256", strings.Repeat("a", 64))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.storeUploadSessionBodyReq.UserID != "user-1" || service.storeUploadSessionBodyReq.SessionID != "session-1" || service.storeUploadSessionBodyReq.ExpectedChecksumSHA256 != strings.Repeat("a", 64) || service.storeUploadSessionBodyReq.Body == nil {
		t.Fatalf("store upload session body request = %+v, want user/session/checksum/body", service.storeUploadSessionBodyReq)
	}
	var body struct {
		Session drive.UploadSession `json:"drive_upload_session"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if body.Session.Status != drive.UploadSessionStatusUploading {
		t.Fatalf("session = %+v", body.Session)
	}
}

func TestDriveFinalizeUploadSessionHandler(t *testing.T) {
	t.Parallel()

	service := &fakeDriveService{file: drive.Node{ID: "file-1", UserID: "user-1", Name: "Report.pdf", Type: drive.NodeTypeFile, Status: drive.NodeStatusActive}}
	mux := http.NewServeMux()
	RegisterDriveRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/drive/upload-sessions/session-1/finalize?user_id=user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.finalizeUploadSessionReq.UserID != "user-1" || service.finalizeUploadSessionReq.SessionID != "session-1" {
		t.Fatalf("finalize upload session request = %+v, want user/session", service.finalizeUploadSessionReq)
	}
	var body struct {
		Node drive.Node `json:"drive_node"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if body.Node.ID != "file-1" {
		t.Fatalf("node = %+v", body.Node)
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

func TestDriveStoreStagedObjectHandler(t *testing.T) {
	t.Parallel()

	service := &fakeDriveService{staged: drive.StagedObject{
		UserID:         "user-1",
		UploadID:       "upload-1",
		StorageBackend: "s3",
		StoragePath:    "drive/users/user-1/staging/upload-1",
		Size:           11,
		ChecksumSHA256: strings.Repeat("a", 64),
	}}
	mux := http.NewServeMux()
	RegisterDriveRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/drive/files/staged/upload-1/body?user_id=user-1&storage_backend=s3", strings.NewReader("hello drive"))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.stagedReq.UserID != "user-1" || service.stagedReq.UploadID != "upload-1" || service.stagedReq.StorageBackend != "s3" || service.stagedReq.Body == nil {
		t.Fatalf("staged request = %+v, want upload identity/body", service.stagedReq)
	}
	var body struct {
		Staged drive.StagedObject `json:"drive_staged_object"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if body.Staged.StoragePath != "drive/users/user-1/staging/upload-1" {
		t.Fatalf("staged = %+v", body.Staged)
	}
	if got := rec.Body.String(); !strings.Contains(got, `"storage_path":"drive/users/user-1/staging/upload-1"`) || strings.Contains(got, "StoragePath") {
		t.Fatalf("response body = %s, want snake_case staged object fields", got)
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

func TestDriveRenameNodeHandler(t *testing.T) {
	t.Parallel()

	service := &fakeDriveService{node: drive.Node{ID: "node-1", UserID: "user-1", Name: "Renamed.pdf", NormalizedName: "renamed.pdf"}}
	mux := http.NewServeMux()
	RegisterDriveRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/drive/nodes/node-1/name?user_id=user-1", strings.NewReader(`{"name":"Renamed.pdf"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.renameReq.UserID != "user-1" || service.renameReq.NodeID != "node-1" || service.renameReq.Name != "Renamed.pdf" {
		t.Fatalf("rename request = %+v, want request body/user/node", service.renameReq)
	}
	var body struct {
		Node drive.Node `json:"drive_node"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if body.Node.Name != "Renamed.pdf" {
		t.Fatalf("node = %+v", body.Node)
	}
}

func TestDriveMoveNodeHandler(t *testing.T) {
	t.Parallel()

	service := &fakeDriveService{node: drive.Node{ID: "node-1", UserID: "user-1", ParentID: "parent-1", Name: "Report.pdf"}}
	mux := http.NewServeMux()
	RegisterDriveRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/drive/nodes/node-1/parent?user_id=user-1", strings.NewReader(`{"parent_id":"parent-1"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.moveReq.UserID != "user-1" || service.moveReq.NodeID != "node-1" || service.moveReq.ParentID != "parent-1" {
		t.Fatalf("move request = %+v, want request body/user/node", service.moveReq)
	}
	var body struct {
		Node drive.Node `json:"drive_node"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if body.Node.ParentID != "parent-1" {
		t.Fatalf("node = %+v", body.Node)
	}
}

func TestDriveCopyNodeHandler(t *testing.T) {
	t.Parallel()

	service := &fakeDriveService{node: drive.Node{ID: "copy-1", UserID: "user-1", ParentID: "parent-2", Name: "Report Copy.pdf", Type: drive.NodeTypeFile, Status: drive.NodeStatusActive}}
	mux := http.NewServeMux()
	RegisterDriveRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/drive/nodes/node-1/copy?user_id=user-1", strings.NewReader(`{"parent_id":"parent-2","name":"Report Copy.pdf"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.copyReq.UserID != "user-1" || service.copyReq.NodeID != "node-1" || service.copyReq.ParentID != "parent-2" || service.copyReq.Name != "Report Copy.pdf" {
		t.Fatalf("copy request = %+v, want request body/user/node", service.copyReq)
	}
	var body struct {
		Node drive.Node `json:"drive_node"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if body.Node.ID != "copy-1" || body.Node.Name != "Report Copy.pdf" {
		t.Fatalf("node = %+v", body.Node)
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
		{name: "get unknown query", req: httptest.NewRequest(http.MethodGet, "/api/v1/drive/nodes/node-1?user_id=user-1&typo=true", nil)},
		{name: "download body rejected", req: httptest.NewRequest(http.MethodGet, "/api/v1/drive/nodes/node-1/download?user_id=user-1", strings.NewReader(`{}`))},
		{name: "download unsafe id", req: httptest.NewRequest(http.MethodGet, "/api/v1/drive/nodes/node%0A1/download?user_id=user-1", nil)},
		{name: "create invalid json", req: httptest.NewRequest(http.MethodPost, "/api/v1/drive/folders?user_id=user-1", strings.NewReader(`{`))},
		{name: "upload session invalid json", req: httptest.NewRequest(http.MethodPost, "/api/v1/drive/upload-sessions?user_id=user-1", strings.NewReader(`{`))},
		{name: "upload session invalid expires", req: httptest.NewRequest(http.MethodPost, "/api/v1/drive/upload-sessions?user_id=user-1", strings.NewReader(`{"name":"Report.pdf","storage_backend":"s3","expires_at":"tomorrow"}`))},
		{name: "list upload sessions unknown query", req: httptest.NewRequest(http.MethodGet, "/api/v1/drive/upload-sessions?user_id=user-1&cursor=bad", nil)},
		{name: "get upload session unknown query", req: httptest.NewRequest(http.MethodGet, "/api/v1/drive/upload-sessions/session-1?user_id=user-1&typo=true", nil)},
		{name: "cancel upload session body rejected", req: httptest.NewRequest(http.MethodDelete, "/api/v1/drive/upload-sessions/session-1?user_id=user-1", strings.NewReader(`{}`))},
		{name: "store upload session content range rejected", req: requestWithHeader(http.MethodPut, "/api/v1/drive/upload-sessions/session-1/body?user_id=user-1", "Content-Range", "bytes 0-1/2")},
		{name: "finalize upload session body rejected", req: httptest.NewRequest(http.MethodPost, "/api/v1/drive/upload-sessions/session-1/finalize?user_id=user-1", strings.NewReader(`{}`))},
		{name: "finalize invalid json", req: httptest.NewRequest(http.MethodPost, "/api/v1/drive/files/finalize?user_id=user-1", strings.NewReader(`{`))},
		{name: "staged missing backend", req: httptest.NewRequest(http.MethodPut, "/api/v1/drive/files/staged/upload-1/body?user_id=user-1", strings.NewReader("x"))},
		{name: "rename invalid json", req: httptest.NewRequest(http.MethodPatch, "/api/v1/drive/nodes/node-1/name?user_id=user-1", strings.NewReader(`{`))},
		{name: "move invalid json", req: httptest.NewRequest(http.MethodPatch, "/api/v1/drive/nodes/node-1/parent?user_id=user-1", strings.NewReader(`{`))},
		{name: "copy invalid json", req: httptest.NewRequest(http.MethodPost, "/api/v1/drive/nodes/node-1/copy?user_id=user-1", strings.NewReader(`{`))},
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
	nodes                     []drive.Node
	node                      drive.Node
	folder                    drive.Node
	file                      drive.Node
	staged                    drive.StagedObject
	uploadSession             drive.UploadSession
	uploadSessions            []drive.UploadSession
	download                  drive.FileDownload
	rangeDownload             drive.FileDownload
	metadata                  drive.FileMetadata
	usageSummary              drive.UsageSummary
	err                       error
	getReq                    drive.GetNodeRequest
	openReq                   drive.OpenFileRequest
	openRangeReq              drive.OpenFileRangeRequest
	statReq                   drive.OpenFileRequest
	usageReq                  drive.GetUsageSummaryRequest
	getUploadSessionReq       drive.GetUploadSessionRequest
	listUploadSessionReq      drive.ListUploadSessionsRequest
	cancelUploadSessionReq    drive.CancelUploadSessionRequest
	storeUploadSessionBodyReq drive.StoreUploadSessionBodyRequest
	finalizeUploadSessionReq  drive.FinalizeUploadSessionRequest
	listReq                   drive.ListNodesRequest
	createReq                 drive.CreateFolderRequest
	fileReq                   drive.CreateFileFromObjectRequest
	stagedReq                 drive.StoreStagedObjectRequest
	uploadSessionReq          drive.CreateUploadSessionRequest
	trashReq                  drive.TrashNodeRequest
	restoreReq                drive.RestoreNodeRequest
	renameReq                 drive.RenameNodeRequest
	moveReq                   drive.MoveNodeRequest
	copyReq                   drive.CopyNodeRequest
	deleteReq                 drive.PermanentDeleteNodeRequest
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

func (f *fakeDriveService) GetNode(_ context.Context, req drive.GetNodeRequest) (drive.Node, error) {
	f.getReq = req
	if f.err != nil {
		return drive.Node{}, f.err
	}
	return f.node, nil
}

func (f *fakeDriveService) OpenFile(_ context.Context, req drive.OpenFileRequest) (drive.FileDownload, error) {
	f.openReq = req
	if f.err != nil {
		return drive.FileDownload{}, f.err
	}
	if f.download.Body != nil {
		return f.download, nil
	}
	return drive.FileDownload{
		Node: drive.Node{ID: "node-1", UserID: req.UserID, Name: "report.pdf", Type: drive.NodeTypeFile, MIMEType: "application/pdf", Size: 7, Status: drive.NodeStatusActive},
		Body: io.NopCloser(strings.NewReader("content")),
	}, nil
}

func (f *fakeDriveService) OpenFileRange(_ context.Context, req drive.OpenFileRangeRequest) (drive.FileDownload, error) {
	f.openRangeReq = req
	if f.err != nil {
		return drive.FileDownload{}, f.err
	}
	if f.rangeDownload.Body != nil {
		return f.rangeDownload, nil
	}
	return drive.FileDownload{
		Node: drive.Node{ID: "node-1", UserID: req.UserID, Name: "report.pdf", Type: drive.NodeTypeFile, MIMEType: "application/pdf", Size: req.Length, Status: drive.NodeStatusActive},
		Body: io.NopCloser(strings.NewReader("content")),
	}, nil
}

func (f *fakeDriveService) StatFile(_ context.Context, req drive.OpenFileRequest) (drive.FileMetadata, error) {
	f.statReq = req
	if f.err != nil {
		return drive.FileMetadata{}, f.err
	}
	if f.metadata.Node.ID != "" {
		return f.metadata, nil
	}
	return drive.FileMetadata{
		Node:   drive.Node{ID: "node-1", UserID: req.UserID, Name: "report.pdf", Type: drive.NodeTypeFile, MIMEType: "application/pdf", Size: 7, Status: drive.NodeStatusActive},
		Object: storage.ObjectInfo{Path: "drive/users/user-1/objects/node-1", Size: 7},
	}, nil
}

func (f *fakeDriveService) GetUsageSummary(_ context.Context, req drive.GetUsageSummaryRequest) (drive.UsageSummary, error) {
	f.usageReq = req
	if f.err != nil {
		return drive.UsageSummary{}, f.err
	}
	return f.usageSummary, nil
}

func (f *fakeDriveService) CreateFileFromObject(_ context.Context, req drive.CreateFileFromObjectRequest) (drive.Node, error) {
	f.fileReq = req
	if f.err != nil {
		return drive.Node{}, f.err
	}
	return f.file, nil
}

func (f *fakeDriveService) StoreStagedObject(_ context.Context, req drive.StoreStagedObjectRequest) (drive.StagedObject, error) {
	f.stagedReq = req
	if f.err != nil {
		return drive.StagedObject{}, f.err
	}
	return f.staged, nil
}

func (f *fakeDriveService) CreateUploadSession(_ context.Context, req drive.CreateUploadSessionRequest) (drive.UploadSession, error) {
	f.uploadSessionReq = req
	if f.err != nil {
		return drive.UploadSession{}, f.err
	}
	return f.uploadSession, nil
}

func (f *fakeDriveService) GetUploadSession(_ context.Context, req drive.GetUploadSessionRequest) (drive.UploadSession, error) {
	f.getUploadSessionReq = req
	if f.err != nil {
		return drive.UploadSession{}, f.err
	}
	return f.uploadSession, nil
}

func (f *fakeDriveService) ListUploadSessions(_ context.Context, req drive.ListUploadSessionsRequest) ([]drive.UploadSession, error) {
	f.listUploadSessionReq = req
	if f.err != nil {
		return nil, f.err
	}
	return f.uploadSessions, nil
}

func (f *fakeDriveService) CancelUploadSession(_ context.Context, req drive.CancelUploadSessionRequest) (drive.UploadSession, error) {
	f.cancelUploadSessionReq = req
	if f.err != nil {
		return drive.UploadSession{}, f.err
	}
	return f.uploadSession, nil
}

func (f *fakeDriveService) StoreUploadSessionBody(_ context.Context, req drive.StoreUploadSessionBodyRequest) (drive.UploadSession, error) {
	f.storeUploadSessionBodyReq = req
	if f.err != nil {
		return drive.UploadSession{}, f.err
	}
	return f.uploadSession, nil
}

func (f *fakeDriveService) FinalizeUploadSession(_ context.Context, req drive.FinalizeUploadSessionRequest) (drive.Node, error) {
	f.finalizeUploadSessionReq = req
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

func (f *fakeDriveService) RenameNode(_ context.Context, req drive.RenameNodeRequest) (drive.Node, error) {
	f.renameReq = req
	if f.err != nil {
		return drive.Node{}, f.err
	}
	return f.node, nil
}

func (f *fakeDriveService) MoveNode(_ context.Context, req drive.MoveNodeRequest) (drive.Node, error) {
	f.moveReq = req
	if f.err != nil {
		return drive.Node{}, f.err
	}
	return f.node, nil
}

func (f *fakeDriveService) CopyNode(_ context.Context, req drive.CopyNodeRequest) (drive.Node, error) {
	f.copyReq = req
	if f.err != nil {
		return drive.Node{}, f.err
	}
	return f.node, nil
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

func requestWithHeader(method string, target string, header string, value string) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader("body"))
	req.Header.Set(header, value)
	return req
}
