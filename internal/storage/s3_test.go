package storage

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestS3StoreUsesPathStyleEndpointAndSignsRequests(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	requests := make([]string, 0, 3)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests = append(requests, r.Method+" "+r.URL.Path)
		mu.Unlock()
		if !strings.HasPrefix(r.Header.Get("Authorization"), "AWS4-HMAC-SHA256 Credential=access/20260505/us-east-1/s3/aws4_request") {
			t.Errorf("Authorization = %q, want SigV4 credential scope", r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if r.Header.Get("x-amz-content-sha256") != "UNSIGNED-PAYLOAD" {
			t.Errorf("x-amz-content-sha256 = %q", r.Header.Get("x-amz-content-sha256"))
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		switch r.Method {
		case http.MethodPut:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("read put body: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if string(body) != "hello" {
				t.Errorf("put body = %q", body)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusOK)
		case http.MethodGet:
			_, _ = w.Write([]byte("hello"))
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("method = %s", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	store, err := NewS3Store(S3Options{
		Endpoint:        server.URL,
		Region:          "us-east-1",
		Bucket:          "gogomail",
		Prefix:          "mail",
		AccessKeyID:     "access",
		SecretAccessKey: "secret",
		ForcePathStyle:  true,
		HTTPClient:      server.Client(),
	})
	if err != nil {
		t.Fatalf("NewS3Store returned error: %v", err)
	}
	store.now = func() time.Time { return time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC) }

	if err := store.Put(context.Background(), "messages/msg-1.eml", strings.NewReader("hello")); err != nil {
		t.Fatalf("Put returned error: %v", err)
	}
	body, err := store.Get(context.Background(), "messages/msg-1.eml")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	got, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("read get body: %v", err)
	}
	if err := body.Close(); err != nil {
		t.Fatalf("close get body: %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("get body = %q", got)
	}
	if err := store.Delete(context.Background(), "messages/msg-1.eml"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}

	want := []string{
		"PUT /gogomail/mail/messages/msg-1.eml",
		"GET /gogomail/mail/messages/msg-1.eml",
		"DELETE /gogomail/mail/messages/msg-1.eml",
	}
	mu.Lock()
	defer mu.Unlock()
	for i := range want {
		if requests[i] != want[i] {
			t.Fatalf("request[%d] = %q, want %q", i, requests[i], want[i])
		}
	}
}

func TestS3StoreRejectsUnsafeObjectPath(t *testing.T) {
	t.Parallel()

	store, err := NewS3Store(S3Options{
		Endpoint:        "http://localhost:9000",
		Region:          "us-east-1",
		Bucket:          "gogomail",
		AccessKeyID:     "access",
		SecretAccessKey: "secret",
		ForcePathStyle:  true,
	})
	if err != nil {
		t.Fatalf("NewS3Store returned error: %v", err)
	}
	if err := store.Put(context.Background(), "../bad", strings.NewReader("bad")); err == nil {
		t.Fatal("Put accepted unsafe object path")
	}
}

func TestS3StoreSanitizesStatusErrorPreview(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("bad bucket\r\nrequest-id: 123"))
	}))
	defer server.Close()

	store, err := NewS3Store(S3Options{
		Endpoint:        server.URL,
		Region:          "us-east-1",
		Bucket:          "gogomail",
		AccessKeyID:     "access",
		SecretAccessKey: "secret",
		ForcePathStyle:  true,
		HTTPClient:      server.Client(),
	})
	if err != nil {
		t.Fatalf("NewS3Store returned error: %v", err)
	}
	err = store.Put(context.Background(), "messages/msg-1.eml", strings.NewReader("hello"))
	if err == nil || !strings.Contains(err.Error(), "403") || !strings.Contains(err.Error(), "bad bucket request-id: 123") || strings.ContainsAny(err.Error(), "\r\n") {
		t.Fatalf("error = %q, want sanitized status preview", err)
	}
}

func TestS3StoreIntegrationRoundTrip(t *testing.T) {
	endpoint := strings.TrimSpace(os.Getenv("GOGOMAIL_TEST_S3_ENDPOINT"))
	bucket := strings.TrimSpace(os.Getenv("GOGOMAIL_TEST_S3_BUCKET"))
	accessKeyID := strings.TrimSpace(os.Getenv("GOGOMAIL_TEST_S3_ACCESS_KEY_ID"))
	secretAccessKey := os.Getenv("GOGOMAIL_TEST_S3_SECRET_ACCESS_KEY")
	if endpoint == "" || bucket == "" || accessKeyID == "" || secretAccessKey == "" {
		t.Skip("set GOGOMAIL_TEST_S3_ENDPOINT, GOGOMAIL_TEST_S3_BUCKET, GOGOMAIL_TEST_S3_ACCESS_KEY_ID, and GOGOMAIL_TEST_S3_SECRET_ACCESS_KEY to run S3-compatible storage integration coverage")
	}

	region := strings.TrimSpace(os.Getenv("GOGOMAIL_TEST_S3_REGION"))
	if region == "" {
		region = "us-east-1"
	}
	prefix := strings.Trim(strings.TrimSpace(os.Getenv("GOGOMAIL_TEST_S3_PREFIX")), "/")
	if prefix == "" {
		prefix = "gogomail-test"
	}
	forcePathStyle := true
	if strings.EqualFold(strings.TrimSpace(os.Getenv("GOGOMAIL_TEST_S3_FORCE_PATH_STYLE")), "false") {
		forcePathStyle = false
	}

	store, err := NewS3Store(S3Options{
		Endpoint:        endpoint,
		Region:          region,
		Bucket:          bucket,
		Prefix:          prefix,
		AccessKeyID:     accessKeyID,
		SecretAccessKey: secretAccessKey,
		SessionToken:    os.Getenv("GOGOMAIL_TEST_S3_SESSION_TOKEN"),
		ForcePathStyle:  forcePathStyle,
	})
	if err != nil {
		t.Fatalf("NewS3Store returned error: %v", err)
	}

	ctx := context.Background()
	objectPath := "integration/" + strings.ReplaceAll(t.Name(), "/", "-") + "-" + time.Now().UTC().Format("20060102150405.000000000") + ".txt"
	body := "gogomail s3 integration\n"
	if err := store.Put(ctx, objectPath, strings.NewReader(body)); err != nil {
		t.Fatalf("Put returned error: %v", err)
	}
	defer func() {
		if err := store.Delete(ctx, objectPath); err != nil {
			t.Fatalf("Delete cleanup returned error: %v", err)
		}
	}()

	readCloser, err := store.Get(ctx, objectPath)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	got, err := io.ReadAll(readCloser)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if err := readCloser.Close(); err != nil {
		t.Fatalf("close body: %v", err)
	}
	if string(got) != body {
		t.Fatalf("body = %q, want %q", got, body)
	}
}
