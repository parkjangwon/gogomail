package apimeter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
)

func TestWriteExportArtifactStreamsNDJSONAndMetadata(t *testing.T) {
	t.Parallel()

	store := &memoryExportArtifactStore{}
	result, err := WriteExportArtifact(context.Background(), store, ExportArtifactWriteRequest{
		ObjectKey: "exports/api-usage/batch-1.ndjson",
		Metadata:  json.RawMessage(`{"batch_id":"batch-1"}`),
		Encode: func(w io.Writer) error {
			return EncodeNDJSON(w, []map[string]any{
				{"event_id": "usage-1", "request_count": 1},
				{"event_id": "usage-2", "request_count": 1},
			})
		},
	})
	if err != nil {
		t.Fatalf("WriteExportArtifact returned error: %v", err)
	}
	if result.ObjectKey != "exports/api-usage/batch-1.ndjson" {
		t.Fatalf("ObjectKey = %q", result.ObjectKey)
	}
	if result.ContentType != ExportArtifactContentTypeNDJSON {
		t.Fatalf("ContentType = %q", result.ContentType)
	}
	if result.ByteCount != int64(len(store.body)) || len(result.SHA256Hex) != 64 {
		t.Fatalf("result = %+v body=%q", result, store.body)
	}
	if !strings.Contains(string(store.body), `"event_id":"usage-1"`) || !strings.HasSuffix(string(store.body), "\n") {
		t.Fatalf("body = %q", store.body)
	}
}

func TestDefaultExportArtifactObjectKey(t *testing.T) {
	t.Parallel()

	got, err := DefaultExportArtifactObjectKey("api-usage-export-1")
	if err != nil {
		t.Fatalf("DefaultExportArtifactObjectKey returned error: %v", err)
	}
	if got != "exports/api-usage/api-usage-export-1.ndjson" {
		t.Fatalf("object key = %q", got)
	}

	got, err = DefaultExportArtifactObjectKey("batch/tenant 1")
	if err != nil {
		t.Fatalf("DefaultExportArtifactObjectKey returned error: %v", err)
	}
	if got != "exports/api-usage/batch_tenant_1.ndjson" {
		t.Fatalf("sanitized object key = %q", got)
	}
}

func TestWriteExportArtifactRejectsUnsafeRequests(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		req  ExportArtifactWriteRequest
	}{
		{name: "missing key", req: ExportArtifactWriteRequest{Encode: func(io.Writer) error { return nil }}},
		{name: "path traversal", req: ExportArtifactWriteRequest{ObjectKey: "../x.ndjson", Encode: func(io.Writer) error { return nil }}},
		{name: "missing encoder", req: ExportArtifactWriteRequest{ObjectKey: "exports/x.ndjson"}},
		{name: "array metadata", req: ExportArtifactWriteRequest{ObjectKey: "exports/x.ndjson", Metadata: json.RawMessage(`[]`), Encode: func(io.Writer) error { return nil }}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := WriteExportArtifact(context.Background(), &memoryExportArtifactStore{}, tc.req)
			if err == nil {
				t.Fatal("WriteExportArtifact returned nil error")
			}
		})
	}
}

func TestWriteExportArtifactReturnsEncodeErrors(t *testing.T) {
	t.Parallel()

	_, err := WriteExportArtifact(context.Background(), &memoryExportArtifactStore{}, ExportArtifactWriteRequest{
		ObjectKey: "exports/x.ndjson",
		Encode: func(io.Writer) error {
			return fmt.Errorf("boom")
		},
	})
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("err = %v", err)
	}
}

func TestWriteExportArtifactReturnsStoreErrors(t *testing.T) {
	t.Parallel()

	_, err := WriteExportArtifact(context.Background(), failingExportArtifactStore{}, ExportArtifactWriteRequest{
		ObjectKey: "exports/x.ndjson",
		Encode: func(w io.Writer) error {
			_, err := w.Write([]byte("x\n"))
			return err
		},
	})
	if err == nil || !strings.Contains(err.Error(), "write export artifact object") {
		t.Fatalf("err = %v", err)
	}
}

type memoryExportArtifactStore struct {
	path string
	body []byte
}

func (s *memoryExportArtifactStore) Put(_ context.Context, path string, body io.Reader) error {
	s.path = path
	raw, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	s.body = raw
	return nil
}

type failingExportArtifactStore struct{}

func (failingExportArtifactStore) Put(context.Context, string, io.Reader) error {
	return fmt.Errorf("store failed")
}
