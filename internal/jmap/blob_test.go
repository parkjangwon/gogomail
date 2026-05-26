package jmap

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gogomail/gogomail/internal/storage"
)

// fakeStore is a minimal in-memory store for testing.
type fakeStore struct {
	data map[string][]byte
}

func newFakeStore() *fakeStore { return &fakeStore{data: make(map[string][]byte)} }

func (s *fakeStore) Put(_ context.Context, path string, body io.Reader) error {
	b, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	s.data[path] = b
	return nil
}

func (s *fakeStore) Get(_ context.Context, path string) (io.ReadCloser, error) {
	b, ok := s.data[path]
	if !ok {
		return nil, fmt.Errorf("not found: %s", path)
	}
	return io.NopCloser(strings.NewReader(string(b))), nil
}

func (s *fakeStore) Stat(_ context.Context, path string) (storage.ObjectInfo, error) {
	b, ok := s.data[path]
	if !ok {
		return storage.ObjectInfo{}, fmt.Errorf("not found: %s", path)
	}
	return storage.ObjectInfo{Size: int64(len(b))}, nil
}

func (s *fakeStore) GetRange(_ context.Context, _ string, _ storage.RangeRequest) (io.ReadCloser, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *fakeStore) Copy(_ context.Context, _, _ string) error { return nil }
func (s *fakeStore) Move(_ context.Context, _, _ string) error { return nil }
func (s *fakeStore) List(_ context.Context, _ storage.ListOptions) (storage.ObjectListPage, error) {
	return storage.ObjectListPage{}, nil
}
func (s *fakeStore) Delete(_ context.Context, _ string) error { return nil }

func TestBlobUploadWrongAccountReturns403(t *testing.T) {
	store := newFakeStore()
	h := NewHandler(Deps{Store: store}, nil)

	req := httptest.NewRequest(http.MethodPost, "/jmap/upload/other-user/", strings.NewReader("hello"))
	req.SetPathValue("accountId", "other-user")
	// X-Test-UserID sets the authenticated user to "actual-user" (different from accountId).
	req.Header.Set("X-Test-UserID", "actual-user")

	w := httptest.NewRecorder()
	h.ServeUpload(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestBlobUploadHappyPath(t *testing.T) {
	store := newFakeStore()
	// nil Repo: DB insert skipped, upload succeeds with just the store
	h := NewHandler(Deps{Store: store}, nil)

	body := "test content for blob"
	req := httptest.NewRequest(http.MethodPost, "/jmap/upload/user1/", strings.NewReader(body))
	req.SetPathValue("accountId", "user1")
	req.Header.Set("X-Test-UserID", "user1")
	req.Header.Set("Content-Type", "text/plain")

	w := httptest.NewRecorder()
	h.ServeUpload(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("upload want 201, got %d: %s", w.Code, w.Body.String())
	}

	var info blobInfo
	if err := json.NewDecoder(w.Body).Decode(&info); err != nil {
		t.Fatalf("decode upload response: %v", err)
	}
	if info.BlobID == "" {
		t.Error("blobId should not be empty")
	}
	if info.AccountID != "user1" {
		t.Errorf("accountId want user1, got %s", info.AccountID)
	}
	if info.Type != "text/plain" {
		t.Errorf("type want text/plain, got %s", info.Type)
	}
	if info.Size != int64(len(body)) {
		t.Errorf("size want %d, got %d", len(body), info.Size)
	}

	// Verify blob was actually stored.
	storagePath := "jmap-blobs/user1/" + info.BlobID
	if _, ok := store.data[storagePath]; !ok {
		t.Errorf("blob not found in store at path %s", storagePath)
	}
}

func TestBlobDownloadWrongAccountReturns403(t *testing.T) {
	store := newFakeStore()
	h := NewHandler(Deps{Store: store}, nil)

	req := httptest.NewRequest(http.MethodGet, "/jmap/download/other-user/some-blob/file.txt", nil)
	req.SetPathValue("accountId", "other-user")
	req.SetPathValue("blobId", "some-blob")
	req.SetPathValue("name", "file.txt")
	req.Header.Set("X-Test-UserID", "actual-user")

	w := httptest.NewRecorder()
	h.ServeDownload(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d: %s", w.Code, w.Body.String())
	}
}

// TestBlobDownloadRoundTrip uploads a blob then downloads it using the nil-Repo
// fallback path (blobId = storage path). This verifies upload→download works
// end-to-end without a database.
func TestBlobDownloadRoundTrip(t *testing.T) {
	store := newFakeStore()
	h := NewHandler(Deps{Store: store}, nil)

	// Upload
	content := "round-trip content"
	upReq := httptest.NewRequest(http.MethodPost, "/jmap/upload/user1/", strings.NewReader(content))
	upReq.SetPathValue("accountId", "user1")
	upReq.Header.Set("X-Test-UserID", "user1")
	upReq.Header.Set("Content-Type", "text/plain; charset=utf-8")
	upW := httptest.NewRecorder()
	h.ServeUpload(upW, upReq)

	if upW.Code != http.StatusCreated {
		t.Fatalf("upload want 201, got %d: %s", upW.Code, upW.Body.String())
	}

	var info blobInfo
	if err := json.NewDecoder(upW.Body).Decode(&info); err != nil {
		t.Fatalf("decode upload response: %v", err)
	}

	// Download using the storage path as blobId (nil-Repo fallback)
	storagePath := "jmap-blobs/user1/" + info.BlobID
	dlReq := httptest.NewRequest(http.MethodGet, "/jmap/download/user1/"+storagePath+"/file.txt", nil)
	dlReq.SetPathValue("accountId", "user1")
	dlReq.SetPathValue("blobId", storagePath)
	dlReq.SetPathValue("name", "file.txt")
	dlReq.Header.Set("X-Test-UserID", "user1")
	dlW := httptest.NewRecorder()
	h.ServeDownload(dlW, dlReq)

	if dlW.Code != http.StatusOK {
		t.Fatalf("download want 200, got %d: %s", dlW.Code, dlW.Body.String())
	}
	if got := dlW.Body.String(); got != content {
		t.Errorf("download body want %q, got %q", content, got)
	}
	if cd := dlW.Header().Get("Content-Disposition"); !strings.Contains(cd, "file.txt") {
		t.Errorf("Content-Disposition want filename=file.txt, got %q", cd)
	}
}
