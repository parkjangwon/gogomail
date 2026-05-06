package storage

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strconv"
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
			if source := r.Header.Get("x-amz-copy-source"); source != "" {
				if source != "/gogomail/mail/messages/msg-1.eml" && source != "/gogomail/mail/messages/msg-1-copy.eml" {
					t.Errorf("x-amz-copy-source = %q", source)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				w.Header().Set("Content-Type", "application/xml")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`<CopyObjectResult><ETag>"etag-copy"</ETag></CopyObjectResult>`))
				return
			}
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
			if r.URL.Query().Get("list-type") == "2" {
				if r.URL.Query().Get("prefix") != "mail/messages" || r.URL.Query().Get("max-keys") != "10" || r.URL.Query().Get("continuation-token") != "cursor-1" {
					t.Errorf("list query = %s", r.URL.RawQuery)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				w.Header().Set("Content-Type", "application/xml")
				_, _ = w.Write([]byte(`<ListBucketResult>
  <IsTruncated>true</IsTruncated>
  <NextContinuationToken>cursor-2</NextContinuationToken>
  <Contents>
    <Key>mail/messages/msg-1.eml</Key>
    <LastModified>2026-05-05T12:00:00Z</LastModified>
    <ETag>"etag-list-1"</ETag>
    <Size>5</Size>
  </Contents>
  <Contents>
    <Key>other-prefix/ignored.eml</Key>
    <Size>99</Size>
  </Contents>
</ListBucketResult>`))
				return
			}
			if gotRange := r.Header.Get("Range"); gotRange != "" {
				if gotRange != "bytes=1-3" {
					t.Errorf("Range = %q", gotRange)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				w.Header().Set("Content-Range", "bytes 1-3/5")
				w.WriteHeader(http.StatusPartialContent)
				_, _ = w.Write([]byte("ell"))
				return
			}
			_, _ = w.Write([]byte("hello"))
		case http.MethodHead:
			w.Header().Set("Content-Length", "5")
			w.Header().Set("Content-Type", "message/rfc822")
			w.Header().Set("ETag", `"etag-1"`)
			w.Header().Set("Last-Modified", "Tue, 05 May 2026 12:00:00 GMT")
			w.WriteHeader(http.StatusOK)
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
	ranged, err := store.GetRange(context.Background(), "messages/msg-1.eml", RangeRequest{Offset: 1, Length: 3})
	if err != nil {
		t.Fatalf("GetRange returned error: %v", err)
	}
	rangeBody, err := io.ReadAll(ranged)
	if err != nil {
		t.Fatalf("read range body: %v", err)
	}
	if err := ranged.Close(); err != nil {
		t.Fatalf("close range body: %v", err)
	}
	if string(rangeBody) != "ell" {
		t.Fatalf("range body = %q", rangeBody)
	}
	info, err := store.Stat(context.Background(), "messages/msg-1.eml")
	if err != nil {
		t.Fatalf("Stat returned error: %v", err)
	}
	if info.Path != "messages/msg-1.eml" || info.Size != 5 || info.ContentType != "message/rfc822" || info.ETag != "etag-1" || info.LastModified.IsZero() {
		t.Fatalf("object info = %+v", info)
	}
	list, err := store.List(context.Background(), ListOptions{Prefix: "messages", Limit: 10, Cursor: "cursor-1"})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if !list.HasMore || list.NextCursor != "cursor-2" || len(list.Objects) != 1 {
		t.Fatalf("list page = %+v", list)
	}
	if list.Objects[0].Path != "messages/msg-1.eml" || list.Objects[0].Size != 5 || list.Objects[0].ETag != "etag-list-1" || list.Objects[0].LastModified.IsZero() {
		t.Fatalf("listed object = %+v", list.Objects[0])
	}
	if err := store.Copy(context.Background(), "messages/msg-1.eml", "messages/msg-1-copy.eml"); err != nil {
		t.Fatalf("Copy returned error: %v", err)
	}
	if err := store.Move(context.Background(), "messages/msg-1-copy.eml", "messages/msg-1-moved.eml"); err != nil {
		t.Fatalf("Move returned error: %v", err)
	}
	if err := store.Delete(context.Background(), "messages/msg-1.eml"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}

	want := []string{
		"PUT /gogomail/mail/messages/msg-1.eml",
		"GET /gogomail/mail/messages/msg-1.eml",
		"GET /gogomail/mail/messages/msg-1.eml",
		"HEAD /gogomail/mail/messages/msg-1.eml",
		"GET /gogomail",
		"PUT /gogomail/mail/messages/msg-1-copy.eml",
		"PUT /gogomail/mail/messages/msg-1-moved.eml",
		"DELETE /gogomail/mail/messages/msg-1-copy.eml",
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

func TestS3StoreRejectsNilPutBody(t *testing.T) {
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
	err = store.Put(context.Background(), "messages/msg-1.eml", nil)
	if err == nil || !strings.Contains(err.Error(), "storage body is required") {
		t.Fatalf("Put err = %v, want nil body rejection", err)
	}
}

func TestS3StorePutRequiresOKStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status int
		want   string
	}{
		{name: "created", status: http.StatusCreated, want: "status 201"},
		{name: "accepted", status: http.StatusAccepted, want: "status 202"},
		{name: "no_content", status: http.StatusNoContent, want: "status 204"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			body := &trackingReadCloser{reader: strings.NewReader("provider body")}
			store, err := NewS3Store(S3Options{
				Endpoint:        "http://localhost:9000",
				Region:          "us-east-1",
				Bucket:          "gogomail",
				AccessKeyID:     "access",
				SecretAccessKey: "secret",
				ForcePathStyle:  true,
				HTTPClient: &http.Client{Transport: staticRoundTripper{
					resp: &http.Response{
						StatusCode: tc.status,
						Body:       body,
					},
				}},
			})
			if err != nil {
				t.Fatalf("NewS3Store returned error: %v", err)
			}
			err = store.Put(context.Background(), "messages/msg-1.eml", strings.NewReader("hello"))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Put err = %v, want %q", err, tc.want)
			}
			if !body.closed {
				t.Fatal("unexpected-status response body was not closed")
			}
		})
	}
}

func TestS3StoreRejectsCanceledContextBeforeRequest(t *testing.T) {
	t.Parallel()

	store, err := NewS3Store(S3Options{
		Endpoint:        "http://localhost:9000",
		Region:          "us-east-1",
		Bucket:          "gogomail",
		AccessKeyID:     "access",
		SecretAccessKey: "secret",
		ForcePathStyle:  true,
		HTTPClient:      &http.Client{Transport: failingRoundTripper{t: t}},
	})
	if err != nil {
		t.Fatalf("NewS3Store returned error: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := store.Put(ctx, "messages/msg-1.eml", strings.NewReader("hello")); !errors.Is(err, context.Canceled) {
		t.Fatalf("Put err = %v, want context.Canceled", err)
	}
	if _, err := store.Get(ctx, "messages/msg-1.eml"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Get err = %v, want context.Canceled", err)
	}
	if _, err := store.GetRange(ctx, "messages/msg-1.eml", RangeRequest{Offset: 0, Length: 1}); !errors.Is(err, context.Canceled) {
		t.Fatalf("GetRange err = %v, want context.Canceled", err)
	}
	if _, err := store.Stat(ctx, "messages/msg-1.eml"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Stat err = %v, want context.Canceled", err)
	}
	if err := store.Copy(ctx, "messages/msg-1.eml", "messages/msg-2.eml"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Copy err = %v, want context.Canceled", err)
	}
	if err := store.Move(ctx, "messages/msg-1.eml", "messages/msg-2.eml"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Move err = %v, want context.Canceled", err)
	}
	if _, err := store.List(ctx, ListOptions{Prefix: "messages"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("List err = %v, want context.Canceled", err)
	}
	if err := store.Delete(ctx, "messages/msg-1.eml"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Delete err = %v, want context.Canceled", err)
	}
}

func TestS3StoreMoveReportsCopiedObjectWhenSourceDeleteFails(t *testing.T) {
	t.Parallel()

	var requests []string
	deleteBody := &trackingReadCloser{reader: strings.NewReader("delete denied")}
	store, err := NewS3Store(S3Options{
		Endpoint:        "http://localhost:9000",
		Region:          "us-east-1",
		Bucket:          "gogomail",
		Prefix:          "mail",
		AccessKeyID:     "access",
		SecretAccessKey: "secret",
		ForcePathStyle:  true,
		HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			requests = append(requests, req.Method+" "+req.URL.EscapedPath())
			switch {
			case req.Method == http.MethodPut && req.Header.Get("x-amz-copy-source") == "/gogomail/mail/messages/source.eml":
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`<CopyObjectResult><ETag>"copied"</ETag></CopyObjectResult>`)),
					Request:    req,
				}, nil
			case req.Method == http.MethodDelete:
				return &http.Response{
					StatusCode: http.StatusForbidden,
					Body:       deleteBody,
					Request:    req,
				}, nil
			default:
				return &http.Response{
					StatusCode: http.StatusBadRequest,
					Body:       io.NopCloser(strings.NewReader("unexpected request")),
					Request:    req,
				}, nil
			}
		})},
	})
	if err != nil {
		t.Fatalf("NewS3Store returned error: %v", err)
	}

	err = store.Move(context.Background(), "messages/source.eml", "messages/dest.eml")
	var cleanupErr S3MoveCleanupError
	if !errors.As(err, &cleanupErr) {
		t.Fatalf("Move err = %v, want S3MoveCleanupError", err)
	}
	if cleanupErr.SourcePath != "messages/source.eml" || cleanupErr.DestPath != "messages/dest.eml" {
		t.Fatalf("cleanup error = %+v", cleanupErr)
	}
	if !strings.Contains(err.Error(), "copied") || !strings.Contains(err.Error(), "failed to delete source") || !strings.Contains(err.Error(), "messages/source.eml") || !strings.Contains(err.Error(), "messages/dest.eml") {
		t.Fatalf("cleanup error message = %q", err)
	}
	if !deleteBody.closed {
		t.Fatal("failed delete response body was not closed")
	}
	wantRequests := []string{
		"PUT /gogomail/mail/messages/dest.eml",
		"DELETE /gogomail/mail/messages/source.eml",
	}
	if len(requests) != len(wantRequests) {
		t.Fatalf("requests = %+v, want %+v", requests, wantRequests)
	}
	for i := range wantRequests {
		if requests[i] != wantRequests[i] {
			t.Fatalf("request[%d] = %q, want %q", i, requests[i], wantRequests[i])
		}
	}
}

func TestS3StoreReadersHonorCanceledContext(t *testing.T) {
	t.Parallel()

	t.Run("get", func(t *testing.T) {
		t.Parallel()

		store, err := NewS3Store(S3Options{
			Endpoint:        "http://localhost:9000",
			Region:          "us-east-1",
			Bucket:          "gogomail",
			AccessKeyID:     "access",
			SecretAccessKey: "secret",
			ForcePathStyle:  true,
			HTTPClient: &http.Client{Transport: staticRoundTripper{
				resp: &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader("hello")),
				},
			}},
		})
		if err != nil {
			t.Fatalf("NewS3Store returned error: %v", err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		body, err := store.Get(ctx, "messages/msg-1.eml")
		if err != nil {
			t.Fatalf("Get returned error: %v", err)
		}
		cancel()
		if _, err := io.ReadAll(body); !errors.Is(err, context.Canceled) {
			t.Fatalf("Get reader err = %v, want context.Canceled", err)
		}
		if err := body.Close(); err != nil {
			t.Fatalf("close Get reader: %v", err)
		}
	})

	t.Run("get_range", func(t *testing.T) {
		t.Parallel()

		store, err := NewS3Store(S3Options{
			Endpoint:        "http://localhost:9000",
			Region:          "us-east-1",
			Bucket:          "gogomail",
			AccessKeyID:     "access",
			SecretAccessKey: "secret",
			ForcePathStyle:  true,
			HTTPClient: &http.Client{Transport: staticRoundTripper{
				resp: &http.Response{
					StatusCode: http.StatusPartialContent,
					Header:     http.Header{"Content-Range": []string{"bytes 0-4/5"}},
					Body:       io.NopCloser(strings.NewReader("hello")),
				},
			}},
		})
		if err != nil {
			t.Fatalf("NewS3Store returned error: %v", err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		body, err := store.GetRange(ctx, "messages/msg-1.eml", RangeRequest{Offset: 0, Length: 5})
		if err != nil {
			t.Fatalf("GetRange returned error: %v", err)
		}
		cancel()
		if _, err := io.ReadAll(body); !errors.Is(err, context.Canceled) {
			t.Fatalf("GetRange reader err = %v, want context.Canceled", err)
		}
		if err := body.Close(); err != nil {
			t.Fatalf("close GetRange reader: %v", err)
		}
	})
}

func TestS3StoreGetDrainsSmallRemainderOnClose(t *testing.T) {
	t.Parallel()

	body := &trackingReadCloser{reader: strings.NewReader("hello-extra")}
	store, err := NewS3Store(S3Options{
		Endpoint:        "http://localhost:9000",
		Region:          "us-east-1",
		Bucket:          "gogomail",
		AccessKeyID:     "access",
		SecretAccessKey: "secret",
		ForcePathStyle:  true,
		HTTPClient: &http.Client{Transport: staticRoundTripper{
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Body:       body,
			},
		}},
	})
	if err != nil {
		t.Fatalf("NewS3Store returned error: %v", err)
	}
	reader, err := store.Get(context.Background(), "messages/msg-1.eml")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	prefix := make([]byte, len("hello"))
	if _, err := io.ReadFull(reader, prefix); err != nil {
		t.Fatalf("read get prefix: %v", err)
	}
	if string(prefix) != "hello" {
		t.Fatalf("prefix = %q, want hello", prefix)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("close get body: %v", err)
	}
	if !body.closed {
		t.Fatal("get body was not closed")
	}
	if body.readBytes != len("hello-extra") {
		t.Fatalf("read bytes = %d, want drained %d", body.readBytes, len("hello-extra"))
	}
}

func TestS3StoreGetRejectsInvalidContentLength(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		header        string
		contentLength int64
		want          string
	}{
		{name: "invalid", header: "not-a-size", contentLength: -1, want: "invalid content length"},
		{name: "signed", header: "+5", contentLength: -1, want: "invalid content length"},
		{name: "leading space", header: " 5", contentLength: -1, want: "invalid content length"},
		{name: "trailing space", header: "5 ", contentLength: -1, want: "invalid content length"},
		{name: "header invalid with populated content length", header: " 5", contentLength: 5, want: "invalid content length"},
		{name: "header mismatch", header: "5", contentLength: 4, want: "content-length mismatch"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			body := &trackingReadCloser{reader: strings.NewReader("hello")}
			store, err := NewS3Store(S3Options{
				Endpoint:        "http://localhost:9000",
				Region:          "us-east-1",
				Bucket:          "gogomail",
				AccessKeyID:     "access",
				SecretAccessKey: "secret",
				ForcePathStyle:  true,
				HTTPClient: &http.Client{Transport: staticRoundTripper{
					resp: &http.Response{
						StatusCode:    http.StatusOK,
						Header:        http.Header{"Content-Length": []string{tc.header}},
						ContentLength: tc.contentLength,
						Body:          body,
					},
				}},
			})
			if err != nil {
				t.Fatalf("NewS3Store returned error: %v", err)
			}
			if _, err := store.Get(context.Background(), "messages/msg-1.eml"); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Get err = %v, want %q", err, tc.want)
			}
			if !body.closed {
				t.Fatal("invalid get response body was not closed")
			}
		})
	}
}

func TestS3StoreGetReportsTruncatedContentLengthBody(t *testing.T) {
	t.Parallel()

	store, err := NewS3Store(S3Options{
		Endpoint:        "http://localhost:9000",
		Region:          "us-east-1",
		Bucket:          "gogomail",
		AccessKeyID:     "access",
		SecretAccessKey: "secret",
		ForcePathStyle:  true,
		HTTPClient: &http.Client{Transport: staticRoundTripper{
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Length": []string{"5"}},
				Body:       io.NopCloser(strings.NewReader("hel")),
			},
		}},
	})
	if err != nil {
		t.Fatalf("NewS3Store returned error: %v", err)
	}
	reader, err := store.Get(context.Background(), "messages/msg-1.eml")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if _, err := io.ReadAll(reader); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("get read err = %v, want io.ErrUnexpectedEOF", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("close get reader: %v", err)
	}
}

func TestS3StoreGetRejectsUnexpectedPartialContent(t *testing.T) {
	t.Parallel()

	body := &trackingReadCloser{reader: strings.NewReader("hel")}
	store, err := NewS3Store(S3Options{
		Endpoint:        "http://localhost:9000",
		Region:          "us-east-1",
		Bucket:          "gogomail",
		AccessKeyID:     "access",
		SecretAccessKey: "secret",
		ForcePathStyle:  true,
		HTTPClient: &http.Client{Transport: staticRoundTripper{
			resp: &http.Response{
				StatusCode: http.StatusPartialContent,
				Body:       body,
			},
		}},
	})
	if err != nil {
		t.Fatalf("NewS3Store returned error: %v", err)
	}
	if _, err := store.Get(context.Background(), "messages/msg-1.eml"); err == nil || !strings.Contains(err.Error(), "status 206") {
		t.Fatalf("Get err = %v, want unexpected partial-content rejection", err)
	}
	if !body.closed {
		t.Fatal("partial get response body was not closed")
	}
}

func TestS3StoreStatRequiresValidContentLength(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		header        string
		contentLength int64
		want          string
	}{
		{name: "invalid", header: "not-a-size", contentLength: -1, want: "invalid content length"},
		{name: "negative", header: "-1", contentLength: -1, want: "invalid content length"},
		{name: "signed", header: "+5", contentLength: -1, want: "invalid content length"},
		{name: "leading space", header: " 5", contentLength: -1, want: "invalid content length"},
		{name: "trailing space", header: "5 ", contentLength: -1, want: "invalid content length"},
		{name: "leading tab", header: "\t5", contentLength: -1, want: "invalid content length"},
		{name: "header invalid with populated content length", header: " 5", contentLength: 5, want: "invalid content length"},
		{name: "header mismatch", header: "5", contentLength: 4, want: "content-length mismatch"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			store, err := NewS3Store(S3Options{
				Endpoint:        "http://localhost:9000",
				Region:          "us-east-1",
				Bucket:          "gogomail",
				AccessKeyID:     "access",
				SecretAccessKey: "secret",
				ForcePathStyle:  true,
				HTTPClient: &http.Client{Transport: staticRoundTripper{
					resp: &http.Response{
						StatusCode:    http.StatusOK,
						Header:        http.Header{"Content-Length": []string{tc.header}},
						ContentLength: tc.contentLength,
						Body:          io.NopCloser(strings.NewReader("")),
					},
				}},
			})
			if err != nil {
				t.Fatalf("NewS3Store returned error: %v", err)
			}
			if _, err := store.Stat(context.Background(), "messages/msg-1.eml"); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Stat err = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestS3StoreStatRequiresContentLength(t *testing.T) {
	t.Parallel()

	store, err := NewS3Store(S3Options{
		Endpoint:        "http://localhost:9000",
		Region:          "us-east-1",
		Bucket:          "gogomail",
		AccessKeyID:     "access",
		SecretAccessKey: "secret",
		ForcePathStyle:  true,
		HTTPClient: &http.Client{Transport: staticRoundTripper{
			resp: &http.Response{
				StatusCode:    http.StatusOK,
				ContentLength: -1,
				Body:          io.NopCloser(strings.NewReader("")),
			},
		}},
	})
	if err != nil {
		t.Fatalf("NewS3Store returned error: %v", err)
	}
	if _, err := store.Stat(context.Background(), "messages/msg-1.eml"); err == nil || !strings.Contains(err.Error(), "content length") {
		t.Fatalf("Stat err = %v, want content length rejection", err)
	}
}

func TestS3StoreStatRequiresValidLastModified(t *testing.T) {
	t.Parallel()

	store, err := NewS3Store(S3Options{
		Endpoint:        "http://localhost:9000",
		Region:          "us-east-1",
		Bucket:          "gogomail",
		AccessKeyID:     "access",
		SecretAccessKey: "secret",
		ForcePathStyle:  true,
		HTTPClient: &http.Client{Transport: staticRoundTripper{
			resp: &http.Response{
				StatusCode:    http.StatusOK,
				Header:        http.Header{"Last-Modified": []string{"not-a-time"}},
				ContentLength: 5,
				Body:          io.NopCloser(strings.NewReader("")),
			},
		}},
	})
	if err != nil {
		t.Fatalf("NewS3Store returned error: %v", err)
	}
	if _, err := store.Stat(context.Background(), "messages/msg-1.eml"); err == nil || !strings.Contains(err.Error(), "invalid last-modified") {
		t.Fatalf("Stat err = %v, want last-modified rejection", err)
	}
}

func TestS3StoreStatAllowsLastModifiedOWS(t *testing.T) {
	t.Parallel()

	store, err := NewS3Store(S3Options{
		Endpoint:        "http://localhost:9000",
		Region:          "us-east-1",
		Bucket:          "gogomail",
		AccessKeyID:     "access",
		SecretAccessKey: "secret",
		ForcePathStyle:  true,
		HTTPClient: &http.Client{Transport: staticRoundTripper{
			resp: &http.Response{
				StatusCode:    http.StatusOK,
				Header:        http.Header{"Last-Modified": []string{" Tue, 05 May 2026 12:00:00 GMT\t"}},
				ContentLength: 5,
				Body:          io.NopCloser(strings.NewReader("")),
			},
		}},
	})
	if err != nil {
		t.Fatalf("NewS3Store returned error: %v", err)
	}
	info, err := store.Stat(context.Background(), "messages/msg-1.eml")
	if err != nil {
		t.Fatalf("Stat returned error: %v", err)
	}
	if info.LastModified.IsZero() {
		t.Fatal("LastModified was not parsed")
	}
}

func TestS3StoreStatDropsUnsafeMetadata(t *testing.T) {
	t.Parallel()

	store, err := NewS3Store(S3Options{
		Endpoint:        "http://localhost:9000",
		Region:          "us-east-1",
		Bucket:          "gogomail",
		AccessKeyID:     "access",
		SecretAccessKey: "secret",
		ForcePathStyle:  true,
		HTTPClient: &http.Client{Transport: staticRoundTripper{
			resp: &http.Response{
				StatusCode:    http.StatusOK,
				Header:        http.Header{"Content-Type": []string{"message/rfc822\r\nx-bad"}, "ETag": []string{`"` + strings.Repeat("e", maxS3ETagBytes+1) + `"`}},
				ContentLength: 5,
				Body:          io.NopCloser(strings.NewReader("")),
			},
		}},
	})
	if err != nil {
		t.Fatalf("NewS3Store returned error: %v", err)
	}
	info, err := store.Stat(context.Background(), "messages/msg-1.eml")
	if err != nil {
		t.Fatalf("Stat returned error: %v", err)
	}
	if info.Path != "messages/msg-1.eml" || info.Size != 5 {
		t.Fatalf("object info identity = %+v", info)
	}
	if info.ContentType != "" || info.ETag != "" {
		t.Fatalf("unsafe metadata was not dropped: %+v", info)
	}
}

func TestS3StoreStatAndListRequireOKStatus(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		call func(*S3Store) error
	}{
		{
			name: "stat",
			call: func(store *S3Store) error {
				_, err := store.Stat(context.Background(), "messages/msg-1.eml")
				return err
			},
		},
		{
			name: "list",
			call: func(store *S3Store) error {
				_, err := store.List(context.Background(), ListOptions{Prefix: "messages"})
				return err
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			body := &trackingReadCloser{reader: strings.NewReader("")}
			store, err := NewS3Store(S3Options{
				Endpoint:        "http://localhost:9000",
				Region:          "us-east-1",
				Bucket:          "gogomail",
				AccessKeyID:     "access",
				SecretAccessKey: "secret",
				ForcePathStyle:  true,
				HTTPClient: &http.Client{Transport: staticRoundTripper{
					resp: &http.Response{
						StatusCode: http.StatusPartialContent,
						Body:       body,
					},
				}},
			})
			if err != nil {
				t.Fatalf("NewS3Store returned error: %v", err)
			}
			if err := tc.call(store); err == nil || !strings.Contains(err.Error(), "status 206") {
				t.Fatalf("%s err = %v, want unexpected partial-content rejection", tc.name, err)
			}
			if !body.closed {
				t.Fatal("unexpected-status response body was not closed")
			}
		})
	}
}

func TestS3StoreListRejectsTruncatedPageWithoutCursor(t *testing.T) {
	t.Parallel()

	store, err := NewS3Store(S3Options{
		Endpoint:        "http://localhost:9000",
		Region:          "us-east-1",
		Bucket:          "gogomail",
		AccessKeyID:     "access",
		SecretAccessKey: "secret",
		ForcePathStyle:  true,
		HTTPClient: &http.Client{Transport: staticRoundTripper{
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(strings.NewReader(`<ListBucketResult>
  <IsTruncated>true</IsTruncated>
  <Contents><Key>messages/msg-1.eml</Key><Size>5</Size></Contents>
</ListBucketResult>`)),
			},
		}},
	})
	if err != nil {
		t.Fatalf("NewS3Store returned error: %v", err)
	}
	_, err = store.List(context.Background(), ListOptions{Prefix: "messages"})
	if err == nil || !strings.Contains(err.Error(), "truncated response missing continuation token") {
		t.Fatalf("List err = %v, want missing continuation token rejection", err)
	}
}

func TestS3StoreListRejectsEmbeddedErrorInOKResponse(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		{
			name: "formatted",
			body: `<Error>
  <Code>SlowDown</Code>
  <Message>list throttled
try later</Message>
  <RequestId>req-1</RequestId>
</Error>`,
			want: "SlowDown: list throttled try later: request-id=req-1",
		},
		{
			name: "empty",
			body: `<Error/>`,
			want: "embedded error",
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			store, err := NewS3Store(S3Options{
				Endpoint:        "http://localhost:9000",
				Region:          "us-east-1",
				Bucket:          "gogomail",
				AccessKeyID:     "access",
				SecretAccessKey: "secret",
				ForcePathStyle:  true,
				HTTPClient: &http.Client{Transport: staticRoundTripper{
					resp: &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(strings.NewReader(tc.body)),
					},
				}},
			})
			if err != nil {
				t.Fatalf("NewS3Store returned error: %v", err)
			}

			_, err = store.List(context.Background(), ListOptions{Prefix: "messages"})
			if err == nil || !strings.Contains(err.Error(), "embedded error") || !strings.Contains(err.Error(), tc.want) || strings.Contains(err.Error(), "<Error>") || strings.ContainsAny(err.Error(), "\r\n") {
				t.Fatalf("List err = %q, want sanitized embedded error rejection", err)
			}
		})
	}
}

func TestS3StoreListRequiresCanonicalIsTruncated(t *testing.T) {
	t.Parallel()

	tests := []string{
		"",
		"1",
		"TRUE",
		" false ",
	}
	for _, isTruncated := range tests {
		isTruncated := isTruncated
		t.Run(isTruncated, func(t *testing.T) {
			t.Parallel()

			isTruncatedElement := ""
			if isTruncated != "" {
				isTruncatedElement = "<IsTruncated>" + isTruncated + "</IsTruncated>"
			}
			store, err := NewS3Store(S3Options{
				Endpoint:        "http://localhost:9000",
				Region:          "us-east-1",
				Bucket:          "gogomail",
				AccessKeyID:     "access",
				SecretAccessKey: "secret",
				ForcePathStyle:  true,
				HTTPClient: &http.Client{Transport: staticRoundTripper{
					resp: &http.Response{
						StatusCode: http.StatusOK,
						Body: io.NopCloser(strings.NewReader(`<ListBucketResult>` + isTruncatedElement + `
  <Contents><Key>messages/msg-1.eml</Key><Size>5</Size></Contents>
</ListBucketResult>`)),
					},
				}},
			})
			if err != nil {
				t.Fatalf("NewS3Store returned error: %v", err)
			}
			_, err = store.List(context.Background(), ListOptions{Prefix: "messages"})
			if err == nil || !strings.Contains(err.Error(), "invalid IsTruncated") {
				t.Fatalf("List err = %v, want invalid IsTruncated rejection", err)
			}
		})
	}
}

func TestS3StoreListRejectsDuplicatePaginationControls(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		{
			name: "is_truncated",
			body: `<ListBucketResult>
  <IsTruncated>false</IsTruncated>
  <IsTruncated>true</IsTruncated>
  <Contents><Key>messages/msg-1.eml</Key><Size>5</Size></Contents>
</ListBucketResult>`,
			want: "duplicate IsTruncated",
		},
		{
			name: "continuation_token",
			body: `<ListBucketResult>
  <IsTruncated>true</IsTruncated>
  <NextContinuationToken>cursor-1</NextContinuationToken>
  <NextContinuationToken>cursor-2</NextContinuationToken>
  <Contents><Key>messages/msg-1.eml</Key><Size>5</Size></Contents>
</ListBucketResult>`,
			want: "duplicate continuation token",
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			store, err := NewS3Store(S3Options{
				Endpoint:        "http://localhost:9000",
				Region:          "us-east-1",
				Bucket:          "gogomail",
				AccessKeyID:     "access",
				SecretAccessKey: "secret",
				ForcePathStyle:  true,
				HTTPClient: &http.Client{Transport: staticRoundTripper{
					resp: &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(strings.NewReader(tc.body)),
					},
				}},
			})
			if err != nil {
				t.Fatalf("NewS3Store returned error: %v", err)
			}

			_, err = store.List(context.Background(), ListOptions{Prefix: "messages"})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("List err = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestS3StoreListRejectsDuplicateObjectMetadata(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		contents string
		want     string
	}{
		{
			name:     "key",
			contents: `<Key>messages/msg-1.eml</Key><Key>messages/msg-2.eml</Key><Size>5</Size>`,
			want:     "duplicate object key",
		},
		{
			name:     "size",
			contents: `<Key>messages/msg-1.eml</Key><Size>5</Size><Size>7</Size>`,
			want:     "duplicate object size",
		},
		{
			name:     "etag",
			contents: `<Key>messages/msg-1.eml</Key><Size>5</Size><ETag>"a"</ETag><ETag>"b"</ETag>`,
			want:     "duplicate object etag",
		},
		{
			name:     "last_modified",
			contents: `<Key>messages/msg-1.eml</Key><Size>5</Size><LastModified>2026-05-05T12:00:00Z</LastModified><LastModified>2026-05-06T12:00:00Z</LastModified>`,
			want:     "duplicate object last-modified",
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			store, err := NewS3Store(S3Options{
				Endpoint:        "http://localhost:9000",
				Region:          "us-east-1",
				Bucket:          "gogomail",
				AccessKeyID:     "access",
				SecretAccessKey: "secret",
				ForcePathStyle:  true,
				HTTPClient: &http.Client{Transport: staticRoundTripper{
					resp: &http.Response{
						StatusCode: http.StatusOK,
						Body: io.NopCloser(strings.NewReader(`<ListBucketResult>
  <IsTruncated>false</IsTruncated>
  <Contents>` + tc.contents + `</Contents>
</ListBucketResult>`)),
					},
				}},
			})
			if err != nil {
				t.Fatalf("NewS3Store returned error: %v", err)
			}

			_, err = store.List(context.Background(), ListOptions{Prefix: "messages"})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("List err = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestS3StoreDeletePrefixUsesContinuationCursor(t *testing.T) {
	t.Parallel()

	var requests []string
	store, err := NewS3Store(S3Options{
		Endpoint:        "http://localhost:9000",
		Region:          "us-east-1",
		Bucket:          "gogomail",
		Prefix:          "mail",
		AccessKeyID:     "access",
		SecretAccessKey: "secret",
		ForcePathStyle:  true,
		HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			requests = append(requests, req.Method+" "+req.URL.EscapedPath()+"?"+req.URL.RawQuery)
			switch {
			case req.Method == http.MethodGet && req.URL.Query().Get("continuation-token") == "":
				return &http.Response{
					StatusCode: http.StatusOK,
					Body: io.NopCloser(strings.NewReader(`<ListBucketResult>
  <IsTruncated>true</IsTruncated>
  <NextContinuationToken>cursor-2</NextContinuationToken>
  <Contents><Key>mail/drive/user-1/a.txt</Key><Size>1</Size></Contents>
</ListBucketResult>`)),
					Request: req,
				}, nil
			case req.Method == http.MethodGet && req.URL.Query().Get("continuation-token") == "cursor-2":
				return &http.Response{
					StatusCode: http.StatusOK,
					Body: io.NopCloser(strings.NewReader(`<ListBucketResult>
  <IsTruncated>false</IsTruncated>
  <Contents><Key>mail/drive/user-1/b.txt</Key><Size>1</Size></Contents>
</ListBucketResult>`)),
					Request: req,
				}, nil
			case req.Method == http.MethodDelete:
				return &http.Response{
					StatusCode: http.StatusNoContent,
					Body:       io.NopCloser(strings.NewReader("")),
					Request:    req,
				}, nil
			default:
				return &http.Response{
					StatusCode: http.StatusBadRequest,
					Body:       io.NopCloser(strings.NewReader("unexpected request")),
					Request:    req,
				}, nil
			}
		})},
	})
	if err != nil {
		t.Fatalf("NewS3Store returned error: %v", err)
	}

	first, err := DeletePrefix(context.Background(), store, DeletePrefixOptions{Prefix: "drive/user-1", Limit: 1})
	if err != nil {
		t.Fatalf("DeletePrefix first page returned error: %v", err)
	}
	if first.Deleted != 1 || !first.HasMore || first.NextCursor != "cursor-2" {
		t.Fatalf("first result = %+v, want deleted first page and cursor", first)
	}
	second, err := DeletePrefix(context.Background(), store, DeletePrefixOptions{Prefix: "drive/user-1", Limit: 1, Cursor: first.NextCursor})
	if err != nil {
		t.Fatalf("DeletePrefix second page returned error: %v", err)
	}
	if second.Deleted != 1 || second.HasMore || second.NextCursor != "" {
		t.Fatalf("second result = %+v, want final delete", second)
	}

	if len(requests) != 4 {
		t.Fatalf("requests = %+v, want two list and two delete requests", requests)
	}
	if !strings.Contains(requests[0], "list-type=2") || strings.Contains(requests[0], "continuation-token=") {
		t.Fatalf("first list request = %q, want no continuation-token", requests[0])
	}
	if requests[1] != "DELETE /gogomail/mail/drive/user-1/a.txt?" {
		t.Fatalf("first delete request = %q", requests[1])
	}
	if !strings.Contains(requests[2], "continuation-token=cursor-2") {
		t.Fatalf("second list request = %q, want continuation cursor", requests[2])
	}
	if requests[3] != "DELETE /gogomail/mail/drive/user-1/b.txt?" {
		t.Fatalf("second delete request = %q", requests[3])
	}
}

func TestS3StoreDeletePrefixSkipsSiblingKeysAfterCanonicalMapping(t *testing.T) {
	t.Parallel()

	var requests []string
	store, err := NewS3Store(S3Options{
		Endpoint:        "http://localhost:9000",
		Region:          "us-east-1",
		Bucket:          "gogomail",
		Prefix:          "mail",
		AccessKeyID:     "access",
		SecretAccessKey: "secret",
		ForcePathStyle:  true,
		HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			requests = append(requests, req.Method+" "+req.URL.EscapedPath())
			switch req.Method {
			case http.MethodGet:
				return &http.Response{
					StatusCode: http.StatusOK,
					Body: io.NopCloser(strings.NewReader(`<ListBucketResult>
  <IsTruncated>false</IsTruncated>
  <Contents><Key>mail/drive/user-10/leak.txt</Key><Size>1</Size></Contents>
  <Contents><Key>mail/drive/user-1/docs/a.txt</Key><Size>1</Size></Contents>
</ListBucketResult>`)),
					Request: req,
				}, nil
			case http.MethodDelete:
				return &http.Response{
					StatusCode: http.StatusNoContent,
					Body:       io.NopCloser(strings.NewReader("")),
					Request:    req,
				}, nil
			default:
				return &http.Response{
					StatusCode: http.StatusBadRequest,
					Body:       io.NopCloser(strings.NewReader("unexpected request")),
					Request:    req,
				}, nil
			}
		})},
	})
	if err != nil {
		t.Fatalf("NewS3Store returned error: %v", err)
	}

	result, err := DeletePrefix(context.Background(), store, DeletePrefixOptions{Prefix: "drive/user-1", Limit: 10})
	if err != nil {
		t.Fatalf("DeletePrefix returned error: %v", err)
	}
	if result.Deleted != 1 || result.HasMore || result.NextCursor != "" {
		t.Fatalf("DeletePrefix result = %+v, want only matching object deleted", result)
	}
	want := []string{
		"GET /gogomail",
		"DELETE /gogomail/mail/drive/user-1/docs/a.txt",
	}
	if !reflect.DeepEqual(requests, want) {
		t.Fatalf("requests = %+v, want %+v", requests, want)
	}
}

func TestS3StoreListRequiresListBucketResult(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		{
			name: "unexpected_root",
			body: `<Result/>`,
			want: "expected element type <ListBucketResult>",
		},
		{
			name: "unexpected_namespace",
			body: `<ListBucketResult xmlns="urn:not-s3"><IsTruncated>false</IsTruncated></ListBucketResult>`,
			want: "unexpected response namespace",
		},
		{
			name: "unexpected_control_namespace",
			body: `<ListBucketResult><x:IsTruncated xmlns:x="urn:not-s3">false</x:IsTruncated></ListBucketResult>`,
			want: "unexpected response namespace",
		},
		{
			name: "unexpected_contents_namespace",
			body: `<ListBucketResult><IsTruncated>false</IsTruncated><x:Contents xmlns:x="urn:not-s3"><Key>messages/msg-1.eml</Key><Size>5</Size></x:Contents></ListBucketResult>`,
			want: "unexpected response namespace",
		},
		{
			name: "unexpected_object_metadata_namespace",
			body: `<ListBucketResult><IsTruncated>false</IsTruncated><Contents><x:Key xmlns:x="urn:not-s3">messages/msg-1.eml</x:Key><Size>5</Size></Contents></ListBucketResult>`,
			want: "unexpected response namespace",
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			store, err := NewS3Store(S3Options{
				Endpoint:        "http://localhost:9000",
				Region:          "us-east-1",
				Bucket:          "gogomail",
				AccessKeyID:     "access",
				SecretAccessKey: "secret",
				ForcePathStyle:  true,
				HTTPClient: &http.Client{Transport: staticRoundTripper{
					resp: &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(strings.NewReader(tc.body)),
					},
				}},
			})
			if err != nil {
				t.Fatalf("NewS3Store returned error: %v", err)
			}

			_, err = store.List(context.Background(), ListOptions{Prefix: "messages"})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("List err = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestS3StoreListAcceptsAWSNamespace(t *testing.T) {
	t.Parallel()

	store, err := NewS3Store(S3Options{
		Endpoint:        "http://localhost:9000",
		Region:          "us-east-1",
		Bucket:          "gogomail",
		AccessKeyID:     "access",
		SecretAccessKey: "secret",
		ForcePathStyle:  true,
		HTTPClient: &http.Client{Transport: staticRoundTripper{
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(strings.NewReader(`<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <IsTruncated>false</IsTruncated>
  <Contents><Key>messages/msg-1.eml</Key><Size>5</Size></Contents>
</ListBucketResult>`)),
			},
		}},
	})
	if err != nil {
		t.Fatalf("NewS3Store returned error: %v", err)
	}

	page, err := store.List(context.Background(), ListOptions{Prefix: "messages"})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(page.Objects) != 1 || page.Objects[0].Path != "messages/msg-1.eml" {
		t.Fatalf("List page = %+v, want namespaced S3 object", page)
	}
}

func TestS3StoreListFiltersLogicalPrefixAfterCanonicalMapping(t *testing.T) {
	t.Parallel()

	store, err := NewS3Store(S3Options{
		Endpoint:        "http://localhost:9000",
		Region:          "us-east-1",
		Bucket:          "gogomail",
		Prefix:          "mail",
		AccessKeyID:     "access",
		SecretAccessKey: "secret",
		ForcePathStyle:  true,
		HTTPClient: &http.Client{Transport: staticRoundTripper{
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(strings.NewReader(`<ListBucketResult>
  <IsTruncated>false</IsTruncated>
  <Contents><Key>mail/drive/user-10/leak.txt</Key><Size>1</Size></Contents>
  <Contents><Key>mail/drive/user-1/docs/a.txt</Key><Size>2</Size></Contents>
</ListBucketResult>`)),
			},
		}},
	})
	if err != nil {
		t.Fatalf("NewS3Store returned error: %v", err)
	}

	page, err := store.List(context.Background(), ListOptions{Prefix: "drive/user-1", Limit: 10})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(page.Objects) != 1 || page.Objects[0].Path != "drive/user-1/docs/a.txt" || page.Objects[0].Size != 2 {
		t.Fatalf("list objects = %+v, want only drive/user-1 object", page.Objects)
	}
}

func TestS3StoreListRejectsUnsafeETagMetadata(t *testing.T) {
	t.Parallel()

	store, err := NewS3Store(S3Options{
		Endpoint:        "http://localhost:9000",
		Region:          "us-east-1",
		Bucket:          "gogomail",
		Prefix:          "mail",
		AccessKeyID:     "access",
		SecretAccessKey: "secret",
		ForcePathStyle:  true,
		HTTPClient: &http.Client{Transport: staticRoundTripper{
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(strings.NewReader(`<ListBucketResult>
  <IsTruncated>false</IsTruncated>
  <Contents><Key>mail/messages/msg-1.eml</Key><Size>5</Size><ETag>` + strings.Repeat("e", maxS3ETagBytes+1) + `</ETag></Contents>
</ListBucketResult>`)),
			},
		}},
	})
	if err != nil {
		t.Fatalf("NewS3Store returned error: %v", err)
	}
	_, err = store.List(context.Background(), ListOptions{Prefix: "messages"})
	if err == nil || !strings.Contains(err.Error(), "invalid object etag") {
		t.Fatalf("List err = %v, want invalid object etag", err)
	}
}

func TestS3StoreListRejectsProviderOverLimitPage(t *testing.T) {
	t.Parallel()

	store, err := NewS3Store(S3Options{
		Endpoint:        "http://localhost:9000",
		Region:          "us-east-1",
		Bucket:          "gogomail",
		Prefix:          "mail",
		AccessKeyID:     "access",
		SecretAccessKey: "secret",
		ForcePathStyle:  true,
		HTTPClient: &http.Client{Transport: staticRoundTripper{
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(strings.NewReader(`<ListBucketResult>
  <IsTruncated>false</IsTruncated>
  <Contents><Key>mail/messages/msg-1.eml</Key><Size>5</Size></Contents>
  <Contents><Key>mail/messages/msg-2.eml</Key><Size>7</Size></Contents>
</ListBucketResult>`)),
			},
		}},
	})
	if err != nil {
		t.Fatalf("NewS3Store returned error: %v", err)
	}

	_, err = store.List(context.Background(), ListOptions{Prefix: "messages", Limit: 1})
	if err == nil || !strings.Contains(err.Error(), "more objects than requested limit") {
		t.Fatalf("List err = %v, want provider over-limit rejection", err)
	}
}

func TestS3StoreListValidatesSizeAfterCanonicalPrefix(t *testing.T) {
	t.Parallel()

	store, err := NewS3Store(S3Options{
		Endpoint:        "http://localhost:9000",
		Region:          "us-east-1",
		Bucket:          "gogomail",
		Prefix:          "mail",
		AccessKeyID:     "access",
		SecretAccessKey: "secret",
		ForcePathStyle:  true,
		HTTPClient: &http.Client{Transport: staticRoundTripper{
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(strings.NewReader(`<ListBucketResult>
  <IsTruncated>false</IsTruncated>
  <Contents><Key>other-prefix/ignored.eml</Key><Size>-1</Size></Contents>
  <Contents><Key>mail/messages/msg-1.eml</Key><Size>5</Size></Contents>
</ListBucketResult>`)),
			},
		}},
	})
	if err != nil {
		t.Fatalf("NewS3Store returned error: %v", err)
	}

	page, err := store.List(context.Background(), ListOptions{Prefix: "messages"})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(page.Objects) != 1 || page.Objects[0].Path != "messages/msg-1.eml" || page.Objects[0].Size != 5 {
		t.Fatalf("list objects = %+v, want canonical object only", page.Objects)
	}
}

func TestS3StoreListRejectsSignedObjectSize(t *testing.T) {
	t.Parallel()

	tests := []string{"+5", "-1", " 5 "}
	for _, size := range tests {
		size := size
		t.Run(size, func(t *testing.T) {
			t.Parallel()

			store, err := NewS3Store(S3Options{
				Endpoint:        "http://localhost:9000",
				Region:          "us-east-1",
				Bucket:          "gogomail",
				Prefix:          "mail",
				AccessKeyID:     "access",
				SecretAccessKey: "secret",
				ForcePathStyle:  true,
				HTTPClient: &http.Client{Transport: staticRoundTripper{
					resp: &http.Response{
						StatusCode: http.StatusOK,
						Body: io.NopCloser(strings.NewReader(`<ListBucketResult>
  <IsTruncated>false</IsTruncated>
  <Contents><Key>mail/messages/msg-1.eml</Key><Size>` + size + `</Size></Contents>
</ListBucketResult>`)),
					},
				}},
			})
			if err != nil {
				t.Fatalf("NewS3Store returned error: %v", err)
			}

			_, err = store.List(context.Background(), ListOptions{Prefix: "messages"})
			if err == nil || !strings.Contains(err.Error(), "invalid object size") {
				t.Fatalf("List err = %v, want invalid object size", err)
			}
		})
	}
}

func TestS3StoreListRejectsInvalidLastModified(t *testing.T) {
	t.Parallel()

	tests := []string{
		"not-a-time",
		" 2026-05-05T12:00:00Z ",
	}
	for _, lastModified := range tests {
		lastModified := lastModified
		t.Run(lastModified, func(t *testing.T) {
			t.Parallel()

			store, err := NewS3Store(S3Options{
				Endpoint:        "http://localhost:9000",
				Region:          "us-east-1",
				Bucket:          "gogomail",
				Prefix:          "mail",
				AccessKeyID:     "access",
				SecretAccessKey: "secret",
				ForcePathStyle:  true,
				HTTPClient: &http.Client{Transport: staticRoundTripper{
					resp: &http.Response{
						StatusCode: http.StatusOK,
						Body: io.NopCloser(strings.NewReader(`<ListBucketResult>
  <IsTruncated>false</IsTruncated>
  <Contents><Key>mail/messages/msg-1.eml</Key><Size>5</Size><LastModified>` + lastModified + `</LastModified></Contents>
</ListBucketResult>`)),
					},
				}},
			})
			if err != nil {
				t.Fatalf("NewS3Store returned error: %v", err)
			}

			_, err = store.List(context.Background(), ListOptions{Prefix: "messages"})
			if err == nil || !strings.Contains(err.Error(), "invalid last-modified") {
				t.Fatalf("List err = %v, want invalid last-modified", err)
			}
		})
	}
}

func TestS3StoreListRejectsInvalidETag(t *testing.T) {
	t.Parallel()

	tests := []string{
		`"bad&#xA;etag"`,
		`"` + strings.Repeat("e", maxS3ETagBytes+1) + `"`,
		`""`,
	}
	for _, etag := range tests {
		etag := etag
		t.Run(strconv.Itoa(len(etag)), func(t *testing.T) {
			t.Parallel()

			store, err := NewS3Store(S3Options{
				Endpoint:        "http://localhost:9000",
				Region:          "us-east-1",
				Bucket:          "gogomail",
				Prefix:          "mail",
				AccessKeyID:     "access",
				SecretAccessKey: "secret",
				ForcePathStyle:  true,
				HTTPClient: &http.Client{Transport: staticRoundTripper{
					resp: &http.Response{
						StatusCode: http.StatusOK,
						Body: io.NopCloser(strings.NewReader(`<ListBucketResult>
  <IsTruncated>false</IsTruncated>
  <Contents><Key>mail/messages/msg-1.eml</Key><Size>5</Size><ETag>` + etag + `</ETag></Contents>
</ListBucketResult>`)),
					},
				}},
			})
			if err != nil {
				t.Fatalf("NewS3Store returned error: %v", err)
			}

			_, err = store.List(context.Background(), ListOptions{Prefix: "messages"})
			if err == nil || !strings.Contains(err.Error(), "invalid object etag") {
				t.Fatalf("List err = %v, want invalid object etag", err)
			}
		})
	}
}

func TestS3StoreListRejectsMissingObjectKey(t *testing.T) {
	t.Parallel()

	tests := []string{
		`<Contents><Size>5</Size></Contents>`,
		`<Contents><Key> </Key><Size>5</Size></Contents>`,
	}
	for _, contents := range tests {
		contents := contents
		t.Run(contents, func(t *testing.T) {
			t.Parallel()

			store, err := NewS3Store(S3Options{
				Endpoint:        "http://localhost:9000",
				Region:          "us-east-1",
				Bucket:          "gogomail",
				Prefix:          "mail",
				AccessKeyID:     "access",
				SecretAccessKey: "secret",
				ForcePathStyle:  true,
				HTTPClient: &http.Client{Transport: staticRoundTripper{
					resp: &http.Response{
						StatusCode: http.StatusOK,
						Body: io.NopCloser(strings.NewReader(`<ListBucketResult>
  <IsTruncated>false</IsTruncated>` + contents + `
</ListBucketResult>`)),
					},
				}},
			})
			if err != nil {
				t.Fatalf("NewS3Store returned error: %v", err)
			}

			_, err = store.List(context.Background(), ListOptions{Prefix: "messages"})
			if err == nil || !strings.Contains(err.Error(), "object key is required") {
				t.Fatalf("List err = %v, want object key rejection", err)
			}
		})
	}
}

func TestS3StoreListRejectsWhitespacePaddedContinuationToken(t *testing.T) {
	t.Parallel()

	store, err := NewS3Store(S3Options{
		Endpoint:        "http://localhost:9000",
		Region:          "us-east-1",
		Bucket:          "gogomail",
		Prefix:          "mail",
		AccessKeyID:     "access",
		SecretAccessKey: "secret",
		ForcePathStyle:  true,
		HTTPClient: &http.Client{Transport: staticRoundTripper{
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(strings.NewReader(`<ListBucketResult>
  <IsTruncated>true</IsTruncated>
  <NextContinuationToken> cursor-2 </NextContinuationToken>
  <Contents><Key>mail/messages/msg-1.eml</Key><Size>5</Size></Contents>
</ListBucketResult>`)),
			},
		}},
	})
	if err != nil {
		t.Fatalf("NewS3Store returned error: %v", err)
	}

	_, err = store.List(context.Background(), ListOptions{Prefix: "messages"})
	if err == nil || !strings.Contains(err.Error(), "leading or trailing whitespace") {
		t.Fatalf("List err = %v, want whitespace-padded continuation token rejection", err)
	}
}

func TestS3StoreListIgnoresContinuationTokenOnFinalPage(t *testing.T) {
	t.Parallel()

	store, err := NewS3Store(S3Options{
		Endpoint:        "http://localhost:9000",
		Region:          "us-east-1",
		Bucket:          "gogomail",
		Prefix:          "mail",
		AccessKeyID:     "access",
		SecretAccessKey: "secret",
		ForcePathStyle:  true,
		HTTPClient: &http.Client{Transport: staticRoundTripper{
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(strings.NewReader(`<ListBucketResult>
  <IsTruncated>false</IsTruncated>
  <NextContinuationToken> cursor-ignored </NextContinuationToken>
  <Contents><Key>mail/messages/msg-1.eml</Key><Size>5</Size></Contents>
</ListBucketResult>`)),
			},
		}},
	})
	if err != nil {
		t.Fatalf("NewS3Store returned error: %v", err)
	}

	page, err := store.List(context.Background(), ListOptions{Prefix: "messages"})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if page.HasMore || page.NextCursor != "" {
		t.Fatalf("page = %+v, want final page with empty cursor", page)
	}
	if len(page.Objects) != 1 || page.Objects[0].Path != "messages/msg-1.eml" {
		t.Fatalf("objects = %+v, want listed object", page.Objects)
	}
}

func TestS3StoreListDoesNotTrimReturnedKeys(t *testing.T) {
	t.Parallel()

	store, err := NewS3Store(S3Options{
		Endpoint:        "http://localhost:9000",
		Region:          "us-east-1",
		Bucket:          "gogomail",
		Prefix:          "mail",
		AccessKeyID:     "access",
		SecretAccessKey: "secret",
		ForcePathStyle:  true,
		HTTPClient: &http.Client{Transport: staticRoundTripper{
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(strings.NewReader(`<ListBucketResult>
  <IsTruncated>false</IsTruncated>
  <Contents><Key>mail/messages/msg-1.eml </Key><Size>5</Size></Contents>
  <Contents><Key> mail/messages/msg-2.eml</Key><Size>7</Size></Contents>
  <Contents><Key>mail/messages/%2Fsecret.eml</Key><Size>11</Size></Contents>
  <Contents><Key>mail/messages/%5Csecret.eml</Key><Size>13</Size></Contents>
  <Contents><Key>mail/messages/msg-3.eml</Key><Size>9</Size></Contents>
</ListBucketResult>`)),
			},
		}},
	})
	if err != nil {
		t.Fatalf("NewS3Store returned error: %v", err)
	}
	page, err := store.List(context.Background(), ListOptions{Prefix: "messages"})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(page.Objects) != 1 || page.Objects[0].Path != "messages/msg-3.eml" {
		t.Fatalf("list objects = %+v, want only exact canonical key", page.Objects)
	}
}

func TestNewS3StoreRejectsEncodedSeparatorPrefix(t *testing.T) {
	t.Parallel()

	for _, prefix := range []string{"mail/%2Ftenant", "mail/%5ctenant", "mail/%252Ftenant", "mail/%255ctenant"} {
		prefix := prefix
		t.Run(prefix, func(t *testing.T) {
			t.Parallel()

			_, err := NewS3Store(S3Options{
				Endpoint:        "http://localhost:9000",
				Region:          "us-east-1",
				Bucket:          "gogomail",
				Prefix:          prefix,
				AccessKeyID:     "access",
				SecretAccessKey: "secret",
				ForcePathStyle:  true,
			})
			if err == nil {
				t.Fatalf("NewS3Store accepted encoded-separator prefix %q", prefix)
			}
		})
	}
}

func TestS3StoreGetRangeRequiresMatchingContentRange(t *testing.T) {
	t.Parallel()

	body := &trackingReadCloser{reader: strings.NewReader("hello")}
	store, err := NewS3Store(S3Options{
		Endpoint:        "http://localhost:9000",
		Region:          "us-east-1",
		Bucket:          "gogomail",
		AccessKeyID:     "access",
		SecretAccessKey: "secret",
		ForcePathStyle:  true,
		HTTPClient: &http.Client{Transport: staticRoundTripper{
			resp: &http.Response{
				StatusCode: http.StatusPartialContent,
				Header:     http.Header{"Content-Range": []string{"bytes 0-0/5"}},
				Body:       body,
			},
		}},
	})
	if err != nil {
		t.Fatalf("NewS3Store returned error: %v", err)
	}
	if _, err := store.GetRange(context.Background(), "messages/msg-1.eml", RangeRequest{Offset: 1, Length: 3}); err == nil || !strings.Contains(err.Error(), "content-range mismatch") {
		t.Fatalf("GetRange err = %v, want content-range mismatch", err)
	}
	if !body.closed {
		t.Fatal("mismatched range response body was not closed")
	}
}

func TestS3StoreGetRangeRejectsWhitespaceInsideContentRange(t *testing.T) {
	t.Parallel()

	tests := []string{
		"bytes 1 -3/5",
		"bytes 1- 3/5",
		"bytes 1-3/ 5",
	}
	for _, contentRange := range tests {
		contentRange := contentRange
		t.Run(contentRange, func(t *testing.T) {
			t.Parallel()

			body := &trackingReadCloser{reader: strings.NewReader("ell")}
			store, err := NewS3Store(S3Options{
				Endpoint:        "http://localhost:9000",
				Region:          "us-east-1",
				Bucket:          "gogomail",
				AccessKeyID:     "access",
				SecretAccessKey: "secret",
				ForcePathStyle:  true,
				HTTPClient: &http.Client{Transport: staticRoundTripper{
					resp: &http.Response{
						StatusCode: http.StatusPartialContent,
						Header:     http.Header{"Content-Range": []string{contentRange}},
						Body:       body,
					},
				}},
			})
			if err != nil {
				t.Fatalf("NewS3Store returned error: %v", err)
			}
			if _, err := store.GetRange(context.Background(), "messages/msg-1.eml", RangeRequest{Offset: 1, Length: 3}); err == nil || !strings.Contains(err.Error(), "invalid content-range") {
				t.Fatalf("GetRange err = %v, want invalid content-range", err)
			}
			if !body.closed {
				t.Fatal("invalid content-range response body was not closed")
			}
		})
	}
}

func TestS3StoreGetRangeRejectsSignedContentRangeNumbers(t *testing.T) {
	t.Parallel()

	tests := []string{
		"bytes +1-3/5",
		"bytes 1-+3/5",
		"bytes 1-3/+5",
		"bytes -1-3/5",
	}
	for _, contentRange := range tests {
		contentRange := contentRange
		t.Run(contentRange, func(t *testing.T) {
			t.Parallel()

			body := &trackingReadCloser{reader: strings.NewReader("ell")}
			store, err := NewS3Store(S3Options{
				Endpoint:        "http://localhost:9000",
				Region:          "us-east-1",
				Bucket:          "gogomail",
				AccessKeyID:     "access",
				SecretAccessKey: "secret",
				ForcePathStyle:  true,
				HTTPClient: &http.Client{Transport: staticRoundTripper{
					resp: &http.Response{
						StatusCode: http.StatusPartialContent,
						Header:     http.Header{"Content-Range": []string{contentRange}},
						Body:       body,
					},
				}},
			})
			if err != nil {
				t.Fatalf("NewS3Store returned error: %v", err)
			}
			if _, err := store.GetRange(context.Background(), "messages/msg-1.eml", RangeRequest{Offset: 1, Length: 3}); err == nil || !strings.Contains(err.Error(), "invalid content-range") {
				t.Fatalf("GetRange err = %v, want invalid content-range", err)
			}
			if !body.closed {
				t.Fatal("invalid content-range response body was not closed")
			}
		})
	}
}

func TestS3StoreGetRangeAcceptsHTTP200ForFullRangeCompatibility(t *testing.T) {
	t.Parallel()

	body := &trackingReadCloser{reader: strings.NewReader("hello")}
	store, err := NewS3Store(S3Options{
		Endpoint:        "http://localhost:9000",
		Region:          "us-east-1",
		Bucket:          "gogomail",
		AccessKeyID:     "access",
		SecretAccessKey: "secret",
		ForcePathStyle:  true,
		HTTPClient: &http.Client{Transport: staticRoundTripper{
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Length": []string{"5"}},
				Body:       body,
			},
		}},
	})
	if err != nil {
		t.Fatalf("NewS3Store returned error: %v", err)
	}
	ranged, err := store.GetRange(context.Background(), "messages/msg-1.eml", RangeRequest{Offset: 0, Length: 5})
	if err != nil {
		t.Fatalf("GetRange returned error: %v", err)
	}
	got, err := io.ReadAll(ranged)
	if err != nil {
		t.Fatalf("read range body: %v", err)
	}
	if err := ranged.Close(); err != nil {
		t.Fatalf("close range body: %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("range body = %q", got)
	}
}

func TestS3StoreGetRangeAcceptsHTTP200WithMatchingContentRangeCompatibility(t *testing.T) {
	t.Parallel()

	body := &trackingReadCloser{reader: strings.NewReader("ell")}
	store, err := NewS3Store(S3Options{
		Endpoint:        "http://localhost:9000",
		Region:          "us-east-1",
		Bucket:          "gogomail",
		AccessKeyID:     "access",
		SecretAccessKey: "secret",
		ForcePathStyle:  true,
		HTTPClient: &http.Client{Transport: staticRoundTripper{
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Range": []string{"bytes 1-3/5"}},
				Body:       body,
			},
		}},
	})
	if err != nil {
		t.Fatalf("NewS3Store returned error: %v", err)
	}
	ranged, err := store.GetRange(context.Background(), "messages/msg-1.eml", RangeRequest{Offset: 1, Length: 3})
	if err != nil {
		t.Fatalf("GetRange returned error: %v", err)
	}
	got, err := io.ReadAll(ranged)
	if err != nil {
		t.Fatalf("read range body: %v", err)
	}
	if err := ranged.Close(); err != nil {
		t.Fatalf("close range body: %v", err)
	}
	if string(got) != "ell" {
		t.Fatalf("range body = %q", got)
	}
}

func TestS3StoreGetRangeRejectsHTTP200ContentRangeLengthMismatch(t *testing.T) {
	t.Parallel()

	body := &trackingReadCloser{reader: strings.NewReader("ell")}
	store, err := NewS3Store(S3Options{
		Endpoint:        "http://localhost:9000",
		Region:          "us-east-1",
		Bucket:          "gogomail",
		AccessKeyID:     "access",
		SecretAccessKey: "secret",
		ForcePathStyle:  true,
		HTTPClient: &http.Client{Transport: staticRoundTripper{
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Header: http.Header{
					"Content-Range":  []string{"bytes 1-3/5"},
					"Content-Length": []string{"4"},
				},
				Body: body,
			},
		}},
	})
	if err != nil {
		t.Fatalf("NewS3Store returned error: %v", err)
	}
	if _, err := store.GetRange(context.Background(), "messages/msg-1.eml", RangeRequest{Offset: 1, Length: 3}); err == nil || !strings.Contains(err.Error(), "content-length mismatch") {
		t.Fatalf("GetRange err = %v, want content-length mismatch", err)
	}
	if !body.closed {
		t.Fatal("unsafe HTTP 200 content-range response body was not closed")
	}
}

func TestS3StoreGetRangeRejectsUnsafeHTTP200CompatibilityResponses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		req    RangeRequest
		header http.Header
		want   string
	}{
		{
			name:   "content length mismatch",
			req:    RangeRequest{Offset: 0, Length: 5},
			header: http.Header{"Content-Length": []string{"4"}},
			want:   "content-length mismatch",
		},
		{
			name:   "padded content length",
			req:    RangeRequest{Offset: 0, Length: 5},
			header: http.Header{"Content-Length": []string{" 5"}},
			want:   "invalid content length",
		},
		{
			name:   "non zero offset without content range",
			req:    RangeRequest{Offset: 1, Length: 3},
			header: http.Header{"Content-Length": []string{"3"}},
			want:   "without content-range",
		},
		{
			name:   "content range mismatch",
			req:    RangeRequest{Offset: 1, Length: 3},
			header: http.Header{"Content-Range": []string{"bytes 0-2/5"}},
			want:   "content-range mismatch",
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			body := &trackingReadCloser{reader: strings.NewReader("hello")}
			store, err := NewS3Store(S3Options{
				Endpoint:        "http://localhost:9000",
				Region:          "us-east-1",
				Bucket:          "gogomail",
				AccessKeyID:     "access",
				SecretAccessKey: "secret",
				ForcePathStyle:  true,
				HTTPClient: &http.Client{Transport: staticRoundTripper{
					resp: &http.Response{
						StatusCode: http.StatusOK,
						Header:     tc.header,
						Body:       body,
					},
				}},
			})
			if err != nil {
				t.Fatalf("NewS3Store returned error: %v", err)
			}
			if _, err := store.GetRange(context.Background(), "messages/msg-1.eml", tc.req); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("GetRange err = %v, want %q", err, tc.want)
			}
			if !body.closed {
				t.Fatal("unsafe HTTP 200 response body was not closed")
			}
		})
	}
}

func TestS3StoreGetRangeRejectsMismatchedPartialContentLength(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		contentLength string
		want          string
	}{
		{name: "mismatch", contentLength: "4", want: "content-length mismatch"},
		{name: "invalid", contentLength: "not-a-size", want: "invalid content length"},
		{name: "signed", contentLength: "+3", want: "invalid content length"},
		{name: "leading space", contentLength: " 3", want: "invalid content length"},
		{name: "trailing space", contentLength: "3 ", want: "invalid content length"},
		{name: "leading tab", contentLength: "\t3", want: "invalid content length"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			body := &trackingReadCloser{reader: strings.NewReader("ell")}
			store, err := NewS3Store(S3Options{
				Endpoint:        "http://localhost:9000",
				Region:          "us-east-1",
				Bucket:          "gogomail",
				AccessKeyID:     "access",
				SecretAccessKey: "secret",
				ForcePathStyle:  true,
				HTTPClient: &http.Client{Transport: staticRoundTripper{
					resp: &http.Response{
						StatusCode: http.StatusPartialContent,
						Header: http.Header{
							"Content-Range":  []string{"bytes 1-3/5"},
							"Content-Length": []string{tc.contentLength},
						},
						Body: body,
					},
				}},
			})
			if err != nil {
				t.Fatalf("NewS3Store returned error: %v", err)
			}
			if _, err := store.GetRange(context.Background(), "messages/msg-1.eml", RangeRequest{Offset: 1, Length: 3}); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("GetRange err = %v, want %q", err, tc.want)
			}
			if !body.closed {
				t.Fatal("unsafe partial range response body was not closed")
			}
		})
	}
}

func TestS3StoreGetRangeReportsTruncatedBody(t *testing.T) {
	t.Parallel()

	store, err := NewS3Store(S3Options{
		Endpoint:        "http://localhost:9000",
		Region:          "us-east-1",
		Bucket:          "gogomail",
		AccessKeyID:     "access",
		SecretAccessKey: "secret",
		ForcePathStyle:  true,
		HTTPClient: &http.Client{Transport: staticRoundTripper{
			resp: &http.Response{
				StatusCode: http.StatusPartialContent,
				Header:     http.Header{"Content-Range": []string{"bytes 1-3/5"}},
				Body:       io.NopCloser(strings.NewReader("el")),
			},
		}},
	})
	if err != nil {
		t.Fatalf("NewS3Store returned error: %v", err)
	}
	ranged, err := store.GetRange(context.Background(), "messages/msg-1.eml", RangeRequest{Offset: 1, Length: 3})
	if err != nil {
		t.Fatalf("GetRange returned error: %v", err)
	}
	if _, err := io.ReadAll(ranged); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("range read err = %v, want io.ErrUnexpectedEOF", err)
	}
	if err := ranged.Close(); err != nil {
		t.Fatalf("close range body: %v", err)
	}
}

func TestS3StoreGetRangeDrainsExtraPartialBytesOnClose(t *testing.T) {
	t.Parallel()

	body := &trackingReadCloser{reader: strings.NewReader("ell-overrun")}
	store, err := NewS3Store(S3Options{
		Endpoint:        "http://localhost:9000",
		Region:          "us-east-1",
		Bucket:          "gogomail",
		AccessKeyID:     "access",
		SecretAccessKey: "secret",
		ForcePathStyle:  true,
		HTTPClient: &http.Client{Transport: staticRoundTripper{
			resp: &http.Response{
				StatusCode: http.StatusPartialContent,
				Header:     http.Header{"Content-Range": []string{"bytes 1-3/5"}},
				Body:       body,
			},
		}},
	})
	if err != nil {
		t.Fatalf("NewS3Store returned error: %v", err)
	}
	ranged, err := store.GetRange(context.Background(), "messages/msg-1.eml", RangeRequest{Offset: 1, Length: 3})
	if err != nil {
		t.Fatalf("GetRange returned error: %v", err)
	}
	got, err := io.ReadAll(ranged)
	if err != nil {
		t.Fatalf("read range body: %v", err)
	}
	if string(got) != "ell" {
		t.Fatalf("range body = %q", got)
	}
	if err := ranged.Close(); err != nil {
		t.Fatalf("close range body: %v", err)
	}
	if !body.closed {
		t.Fatal("range body was not closed")
	}
	if body.readBytes != len("ell-overrun") {
		t.Fatalf("read bytes = %d, want drained %d", body.readBytes, len("ell-overrun"))
	}
}

func TestS3StoreGetRangeDrainsUnreadRangeBytesOnClose(t *testing.T) {
	t.Parallel()

	body := &trackingReadCloser{reader: strings.NewReader("ell")}
	store, err := NewS3Store(S3Options{
		Endpoint:        "http://localhost:9000",
		Region:          "us-east-1",
		Bucket:          "gogomail",
		AccessKeyID:     "access",
		SecretAccessKey: "secret",
		ForcePathStyle:  true,
		HTTPClient: &http.Client{Transport: staticRoundTripper{
			resp: &http.Response{
				StatusCode: http.StatusPartialContent,
				Header:     http.Header{"Content-Range": []string{"bytes 1-3/5"}},
				Body:       body,
			},
		}},
	})
	if err != nil {
		t.Fatalf("NewS3Store returned error: %v", err)
	}
	ranged, err := store.GetRange(context.Background(), "messages/msg-1.eml", RangeRequest{Offset: 1, Length: 3})
	if err != nil {
		t.Fatalf("GetRange returned error: %v", err)
	}
	one := make([]byte, 1)
	if n, err := ranged.Read(one); n != 1 || err != nil {
		t.Fatalf("initial range read = %d, %v", n, err)
	}
	if err := ranged.Close(); err != nil {
		t.Fatalf("close range body: %v", err)
	}
	if !body.closed {
		t.Fatal("range body was not closed")
	}
	if body.readBytes != len("ell") {
		t.Fatalf("read bytes = %d, want drained %d", body.readBytes, len("ell"))
	}
}

func TestS3StoreCheckBoundsReadinessBody(t *testing.T) {
	t.Parallel()

	var deletes int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			w.WriteHeader(http.StatusOK)
		case http.MethodGet:
			_, _ = w.Write([]byte("gogomail storage readiness\nextra"))
		case http.MethodDelete:
			deletes++
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
		AccessKeyID:     "access",
		SecretAccessKey: "secret",
		ForcePathStyle:  true,
		HTTPClient:      server.Client(),
	})
	if err != nil {
		t.Fatalf("NewS3Store returned error: %v", err)
	}
	if err := store.Check(context.Background()); err == nil || !strings.Contains(err.Error(), "readiness probe body mismatch") {
		t.Fatalf("Check err = %v, want bounded mismatch", err)
	}
	if deletes != 1 {
		t.Fatalf("delete calls = %d, want cleanup after mismatch", deletes)
	}
}

func TestS3StoreCheckProbesReadinessMetadata(t *testing.T) {
	t.Parallel()

	var sawHead bool
	var deletes int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			w.WriteHeader(http.StatusOK)
		case http.MethodGet:
			_, _ = w.Write([]byte("gogomail storage readiness\n"))
		case http.MethodHead:
			sawHead = true
			w.Header().Set("Content-Length", strconv.Itoa(len("gogomail storage readiness\n")+1))
			w.WriteHeader(http.StatusOK)
		case http.MethodDelete:
			deletes++
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
		AccessKeyID:     "access",
		SecretAccessKey: "secret",
		ForcePathStyle:  true,
		HTTPClient:      server.Client(),
	})
	if err != nil {
		t.Fatalf("NewS3Store returned error: %v", err)
	}
	if err := store.Check(context.Background()); err == nil || !strings.Contains(err.Error(), "readiness probe metadata mismatch") {
		t.Fatalf("Check err = %v, want metadata mismatch", err)
	}
	if !sawHead {
		t.Fatal("Check did not issue HEAD metadata probe")
	}
	if deletes != 1 {
		t.Fatalf("delete calls = %d, want cleanup after metadata mismatch", deletes)
	}
}

func TestS3StoreCheckProbesReadinessRange(t *testing.T) {
	t.Parallel()

	var sawRange bool
	var deletes int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			w.WriteHeader(http.StatusOK)
		case http.MethodGet:
			if r.Header.Get("Range") == "bytes=0-7" {
				sawRange = true
				w.Header().Set("Content-Range", "bytes 0-7/27")
				w.WriteHeader(http.StatusPartialContent)
				_, _ = w.Write([]byte("gogomail"))
				return
			}
			_, _ = w.Write([]byte("gogomail storage readiness\n"))
		case http.MethodHead:
			w.Header().Set("Content-Length", strconv.Itoa(len("gogomail storage readiness\n")))
			w.WriteHeader(http.StatusOK)
		case http.MethodDelete:
			deletes++
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
		AccessKeyID:     "access",
		SecretAccessKey: "secret",
		ForcePathStyle:  true,
		HTTPClient:      server.Client(),
	})
	if err != nil {
		t.Fatalf("NewS3Store returned error: %v", err)
	}
	if err := store.Check(context.Background()); err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if !sawRange {
		t.Fatal("Check did not issue range readiness probe")
	}
	if deletes != 1 {
		t.Fatalf("delete calls = %d, want cleanup after successful probe", deletes)
	}
}

func TestDrainAndCloseS3BodyIsBounded(t *testing.T) {
	t.Parallel()

	body := &trackingReadCloser{reader: strings.NewReader(strings.Repeat("x", maxS3ResponseDrainBytes+128))}
	drainAndCloseS3Body(body)
	if !body.closed {
		t.Fatal("drainAndCloseS3Body did not close body")
	}
	if body.readBytes != maxS3ResponseDrainBytes {
		t.Fatalf("drained bytes = %d, want %d", body.readBytes, maxS3ResponseDrainBytes)
	}
}

type trackingReadCloser struct {
	reader    *strings.Reader
	readBytes int
	closed    bool
}

func (r *trackingReadCloser) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	r.readBytes += n
	return n, err
}

func (r *trackingReadCloser) Close() error {
	r.closed = true
	return nil
}

type failingRoundTripper struct {
	t *testing.T
}

func (rt failingRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	rt.t.Fatal("RoundTrip called for canceled S3 request")
	return nil, nil
}

type staticRoundTripper struct {
	resp *http.Response
}

func (rt staticRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	resp := *rt.resp
	resp.Request = req
	return &resp, nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestS3StoreUsesVirtualHostedStyleByDefault(t *testing.T) {
	t.Parallel()

	store, err := NewS3Store(S3Options{
		Endpoint:        "https://s3.us-east-1.amazonaws.com/base",
		Region:          "us-east-1",
		Bucket:          "gogomail",
		Prefix:          "mail",
		AccessKeyID:     "access",
		SecretAccessKey: "secret",
		SessionToken:    "session-token",
	})
	if err != nil {
		t.Fatalf("NewS3Store returned error: %v", err)
	}
	store.now = func() time.Time { return time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC) }

	req, err := store.newRequest(context.Background(), http.MethodPut, "messages/msg 1.eml", strings.NewReader("hello"))
	if err != nil {
		t.Fatalf("newRequest returned error: %v", err)
	}
	if req.URL.Scheme != "https" || req.URL.Host != "gogomail.s3.us-east-1.amazonaws.com" {
		t.Fatalf("request URL host = %s://%s", req.URL.Scheme, req.URL.Host)
	}
	if req.URL.EscapedPath() != "/base/mail/messages/msg%201.eml" {
		t.Fatalf("request path = %q", req.URL.EscapedPath())
	}
	if req.Header.Get("x-amz-security-token") != "session-token" {
		t.Fatalf("session token header = %q", req.Header.Get("x-amz-security-token"))
	}
	if !strings.Contains(req.Header.Get("Authorization"), "SignedHeaders=host;x-amz-content-sha256;x-amz-date;x-amz-security-token") {
		t.Fatalf("Authorization = %q, want session-token signed header", req.Header.Get("Authorization"))
	}
}

func TestS3StoreAutoPathStyleForDottedHTTPSBucket(t *testing.T) {
	t.Parallel()

	store, err := NewS3Store(S3Options{
		Endpoint:        "https://s3.us-east-1.amazonaws.com/base",
		Region:          "us-east-1",
		Bucket:          "mail.example.com",
		Prefix:          "mail",
		AccessKeyID:     "access",
		SecretAccessKey: "secret",
	})
	if err != nil {
		t.Fatalf("NewS3Store returned error: %v", err)
	}
	store.now = func() time.Time { return time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC) }

	req, err := store.newRequest(context.Background(), http.MethodGet, "messages/msg 1.eml", nil)
	if err != nil {
		t.Fatalf("newRequest returned error: %v", err)
	}
	if req.URL.Host != "s3.us-east-1.amazonaws.com" {
		t.Fatalf("request host = %q, want path-style endpoint host", req.URL.Host)
	}
	if got, want := req.URL.EscapedPath(), "/base/mail.example.com/mail/messages/msg%201.eml"; got != want {
		t.Fatalf("request path = %q, want %q", got, want)
	}
}

func TestS3StoreAutoPathStyleForLocalEndpoints(t *testing.T) {
	t.Parallel()

	for _, endpoint := range []string{"http://localhost:19000", "http://127.0.0.1:19000", "http://[::1]:19000"} {
		endpoint := endpoint
		t.Run(endpoint, func(t *testing.T) {
			t.Parallel()

			store, err := NewS3Store(S3Options{
				Endpoint:        endpoint,
				Region:          "us-east-1",
				Bucket:          "gogomail",
				Prefix:          "mail",
				AccessKeyID:     "access",
				SecretAccessKey: "secret",
			})
			if err != nil {
				t.Fatalf("NewS3Store returned error: %v", err)
			}

			req, err := store.newRequest(context.Background(), http.MethodHead, "messages/msg.eml", nil)
			if err != nil {
				t.Fatalf("newRequest returned error: %v", err)
			}
			if strings.HasPrefix(req.URL.Host, "gogomail.") {
				t.Fatalf("request host = %q, want local path-style host", req.URL.Host)
			}
			if !strings.Contains(req.URL.EscapedPath(), "/gogomail/mail/messages/msg.eml") {
				t.Fatalf("request path = %q, want bucket in path", req.URL.EscapedPath())
			}
		})
	}
}

func TestS3StoreEscapesPlusInObjectKeys(t *testing.T) {
	t.Parallel()

	store, err := NewS3Store(S3Options{
		Endpoint:        "http://localhost:9000/base+proxy",
		Region:          "us-east-1",
		Bucket:          "gogomail",
		Prefix:          "mail+archive",
		AccessKeyID:     "access",
		SecretAccessKey: "secret",
		ForcePathStyle:  true,
	})
	if err != nil {
		t.Fatalf("NewS3Store returned error: %v", err)
	}
	store.now = func() time.Time { return time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC) }

	req, err := store.newRequest(context.Background(), http.MethodPut, "messages/msg+1.eml", strings.NewReader("hello"))
	if err != nil {
		t.Fatalf("newRequest returned error: %v", err)
	}
	if got, want := req.URL.EscapedPath(), "/base%2Bproxy/gogomail/mail%2Barchive/messages/msg%2B1.eml"; got != want {
		t.Fatalf("request path = %q, want %q", got, want)
	}
	if strings.Contains(req.URL.EscapedPath(), "%20") {
		t.Fatalf("request path encoded plus as space: %q", req.URL.EscapedPath())
	}
	copyReq, err := store.newRequestWithHeaders(context.Background(), http.MethodPut, "messages/msg+2.eml", nil, map[string]string{
		"x-amz-copy-source": store.copySource("messages/msg+1.eml"),
	})
	if err != nil {
		t.Fatalf("copy request returned error: %v", err)
	}
	if got, want := copyReq.URL.EscapedPath(), "/base%2Bproxy/gogomail/mail%2Barchive/messages/msg%2B2.eml"; got != want {
		t.Fatalf("copy destination path = %q, want %q", got, want)
	}
	if got, want := copyReq.Header.Get("x-amz-copy-source"), "/gogomail/mail%2Barchive/messages/msg%2B1.eml"; got != want {
		t.Fatalf("copy source = %q, want %q", got, want)
	}
	if !strings.Contains(copyReq.Header.Get("Authorization"), "x-amz-copy-source") {
		t.Fatalf("Authorization = %q, want copy source signed", copyReq.Header.Get("Authorization"))
	}
}

func TestS3StoreListUsesSigV4CanonicalQueryEncoding(t *testing.T) {
	t.Parallel()

	store, err := NewS3Store(S3Options{
		Endpoint:        "http://localhost:9000/base+proxy",
		Region:          "us-east-1",
		Bucket:          "gogomail",
		Prefix:          "mail",
		AccessKeyID:     "access",
		SecretAccessKey: "secret",
		ForcePathStyle:  true,
	})
	if err != nil {
		t.Fatalf("NewS3Store returned error: %v", err)
	}
	store.now = func() time.Time { return time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC) }

	req, err := store.newListRequest(context.Background(), "tenant+1/user@example.com/Inbox and Projects", 25, "token+with spaces/and=equals")
	if err != nil {
		t.Fatalf("newListRequest returned error: %v", err)
	}
	want := "continuation-token=token%2Bwith%20spaces%2Fand%3Dequals&list-type=2&max-keys=25&prefix=mail%2Ftenant%2B1%2Fuser%40example.com%2FInbox%20and%20Projects"
	if req.URL.RawQuery != want {
		t.Fatalf("RawQuery = %q, want %q", req.URL.RawQuery, want)
	}
	if strings.Contains(req.URL.RawQuery, "+") {
		t.Fatalf("RawQuery used form-style space encoding: %q", req.URL.RawQuery)
	}
	if got := req.URL.Query().Get("prefix"); got != "mail/tenant+1/user@example.com/Inbox and Projects" {
		t.Fatalf("decoded prefix = %q", got)
	}
	if got := req.URL.Query().Get("continuation-token"); got != "token+with spaces/and=equals" {
		t.Fatalf("decoded continuation-token = %q", got)
	}
	if !strings.Contains(req.Header.Get("Authorization"), "SignedHeaders=host;x-amz-content-sha256;x-amz-date") {
		t.Fatalf("Authorization = %q, want signed list request", req.Header.Get("Authorization"))
	}
}

func TestS3StoreSetsContentLengthForSeekablePutBody(t *testing.T) {
	t.Parallel()

	store, err := NewS3Store(S3Options{
		Endpoint:        "http://localhost:9000",
		Region:          "us-east-1",
		Bucket:          "gogomail",
		Prefix:          "mail",
		AccessKeyID:     "access",
		SecretAccessKey: "secret",
		ForcePathStyle:  true,
	})
	if err != nil {
		t.Fatalf("NewS3Store returned error: %v", err)
	}
	store.now = func() time.Time { return time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC) }

	body := &seekableTestReader{reader: strings.NewReader("hello world")}
	if _, err := body.Seek(6, io.SeekStart); err != nil {
		t.Fatalf("seek body: %v", err)
	}
	req, err := store.newRequest(context.Background(), http.MethodPut, "messages/msg-1.eml", body)
	if err != nil {
		t.Fatalf("newRequest returned error: %v", err)
	}
	if req.ContentLength != 5 {
		t.Fatalf("ContentLength = %d, want 5", req.ContentLength)
	}
	pos, err := body.Seek(0, io.SeekCurrent)
	if err != nil {
		t.Fatalf("seek current: %v", err)
	}
	if pos != 6 {
		t.Fatalf("body position = %d, want 6", pos)
	}
	if !strings.Contains(req.Header.Get("Authorization"), "SignedHeaders=host;x-amz-content-sha256;x-amz-date") {
		t.Fatalf("Authorization = %q, want signed request", req.Header.Get("Authorization"))
	}
}

type seekableTestReader struct {
	reader *strings.Reader
}

func (r *seekableTestReader) Read(p []byte) (int, error) {
	return r.reader.Read(p)
}

func (r *seekableTestReader) Seek(offset int64, whence int) (int64, error) {
	return r.reader.Seek(offset, whence)
}

func TestS3StoreUsesPathStyleForDottedHTTPSBucket(t *testing.T) {
	t.Parallel()

	store, err := NewS3Store(S3Options{
		Endpoint:        "https://s3.us-east-1.amazonaws.com/base",
		Region:          "us-east-1",
		Bucket:          "mail.example.com",
		Prefix:          "mail",
		AccessKeyID:     "access",
		SecretAccessKey: "secret",
	})
	if err != nil {
		t.Fatalf("NewS3Store returned error: %v", err)
	}
	store.now = func() time.Time { return time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC) }

	req, err := store.newRequest(context.Background(), http.MethodPut, "messages/msg-1.eml", strings.NewReader("hello"))
	if err != nil {
		t.Fatalf("newRequest returned error: %v", err)
	}
	if req.URL.Scheme != "https" || req.URL.Host != "s3.us-east-1.amazonaws.com" {
		t.Fatalf("request URL host = %s://%s", req.URL.Scheme, req.URL.Host)
	}
	if req.URL.EscapedPath() != "/base/mail.example.com/mail/messages/msg-1.eml" {
		t.Fatalf("request path = %q", req.URL.EscapedPath())
	}
	if !strings.Contains(req.Header.Get("Authorization"), "SignedHeaders=host;x-amz-content-sha256;x-amz-date") {
		t.Fatalf("Authorization = %q, want host signed header", req.Header.Get("Authorization"))
	}
}

func TestS3StoreUsesPathStyleForLocalEndpoints(t *testing.T) {
	t.Parallel()

	for _, endpoint := range []string{
		"http://localhost:9000/base",
		"http://127.0.0.1:9000/base",
		"http://[::1]:9000/base",
	} {
		endpoint := endpoint
		t.Run(endpoint, func(t *testing.T) {
			t.Parallel()

			store, err := NewS3Store(S3Options{
				Endpoint:        endpoint,
				Region:          "us-east-1",
				Bucket:          "gogomail",
				Prefix:          "mail",
				AccessKeyID:     "access",
				SecretAccessKey: "secret",
			})
			if err != nil {
				t.Fatalf("NewS3Store returned error: %v", err)
			}
			store.now = func() time.Time { return time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC) }

			req, err := store.newRequest(context.Background(), http.MethodPut, "messages/msg-1.eml", strings.NewReader("hello"))
			if err != nil {
				t.Fatalf("newRequest returned error: %v", err)
			}
			if req.URL.Host != store.endpoint.Host {
				t.Fatalf("request host = %q, want endpoint host %q", req.URL.Host, store.endpoint.Host)
			}
			if req.URL.EscapedPath() != "/base/gogomail/mail/messages/msg-1.eml" {
				t.Fatalf("request path = %q", req.URL.EscapedPath())
			}
		})
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

	for _, unsafePath := range []string{"../bad", "messages/%2Fsecret.eml", "messages/%5csecret.eml"} {
		unsafePath := unsafePath
		t.Run(unsafePath, func(t *testing.T) {
			t.Parallel()

			if err := store.Put(context.Background(), unsafePath, strings.NewReader("bad")); err == nil {
				t.Fatal("Put accepted unsafe object path")
			}
			if _, err := store.Get(context.Background(), unsafePath); err == nil {
				t.Fatal("Get accepted unsafe object path")
			}
			if _, err := store.Stat(context.Background(), unsafePath); err == nil {
				t.Fatal("Stat accepted unsafe object path")
			}
			if _, err := store.GetRange(context.Background(), unsafePath, RangeRequest{Offset: 0, Length: 1}); err == nil {
				t.Fatal("GetRange accepted unsafe object path")
			}
			if err := store.Copy(context.Background(), unsafePath, "messages/good.eml"); err == nil {
				t.Fatal("Copy accepted unsafe source object path")
			}
			if err := store.Copy(context.Background(), "messages/good.eml", unsafePath); err == nil {
				t.Fatal("Copy accepted unsafe destination object path")
			}
			if err := store.Move(context.Background(), unsafePath, "messages/good.eml"); err == nil {
				t.Fatal("Move accepted unsafe source object path")
			}
			if err := store.Move(context.Background(), "messages/good.eml", unsafePath); err == nil {
				t.Fatal("Move accepted unsafe destination object path")
			}
			if _, err := store.List(context.Background(), ListOptions{Prefix: unsafePath}); err == nil {
				t.Fatal("List accepted unsafe object prefix")
			}
		})
	}
}

func TestS3StoreDeleteTreatsMissingObjectAsSuccess(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %s, want DELETE", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("missing"))
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
	if err := store.Delete(context.Background(), "messages/missing.eml"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
}

func TestS3StoreDeleteRejectsAmbiguousSuccessStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status int
		want   string
	}{
		{name: "created", status: http.StatusCreated, want: "status 201"},
		{name: "accepted", status: http.StatusAccepted, want: "status 202"},
		{name: "partial_content", status: http.StatusPartialContent, want: "status 206"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			body := &trackingReadCloser{reader: strings.NewReader("provider body")}
			store, err := NewS3Store(S3Options{
				Endpoint:        "http://localhost:9000",
				Region:          "us-east-1",
				Bucket:          "gogomail",
				AccessKeyID:     "access",
				SecretAccessKey: "secret",
				ForcePathStyle:  true,
				HTTPClient: &http.Client{Transport: staticRoundTripper{
					resp: &http.Response{
						StatusCode: tc.status,
						Body:       body,
					},
				}},
			})
			if err != nil {
				t.Fatalf("NewS3Store returned error: %v", err)
			}
			err = store.Delete(context.Background(), "messages/msg-1.eml")
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Delete err = %v, want %q", err, tc.want)
			}
			if !body.closed {
				t.Fatal("unexpected-status response body was not closed")
			}
		})
	}
}

func TestS3StoreDeleteAcceptsCompatibleSuccessStatus(t *testing.T) {
	t.Parallel()

	for _, status := range []int{http.StatusOK, http.StatusNoContent} {
		status := status
		t.Run(http.StatusText(status), func(t *testing.T) {
			t.Parallel()

			body := &trackingReadCloser{reader: strings.NewReader("")}
			store, err := NewS3Store(S3Options{
				Endpoint:        "http://localhost:9000",
				Region:          "us-east-1",
				Bucket:          "gogomail",
				AccessKeyID:     "access",
				SecretAccessKey: "secret",
				ForcePathStyle:  true,
				HTTPClient: &http.Client{Transport: staticRoundTripper{
					resp: &http.Response{
						StatusCode: status,
						Body:       body,
					},
				}},
			})
			if err != nil {
				t.Fatalf("NewS3Store returned error: %v", err)
			}
			if err := store.Delete(context.Background(), "messages/msg-1.eml"); err != nil {
				t.Fatalf("Delete returned error: %v", err)
			}
			if !body.closed {
				t.Fatal("success response body was not closed")
			}
		})
	}
}

func TestS3StoreMissingObjectErrorsWrapNotExist(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		run  func(*S3Store) error
	}{
		{
			name: "get",
			run: func(store *S3Store) error {
				_, err := store.Get(context.Background(), "messages/missing.eml")
				return err
			},
		},
		{
			name: "get range",
			run: func(store *S3Store) error {
				_, err := store.GetRange(context.Background(), "messages/missing.eml", RangeRequest{Offset: 0, Length: 1})
				return err
			},
		},
		{
			name: "stat",
			run: func(store *S3Store) error {
				_, err := store.Stat(context.Background(), "messages/missing.eml")
				return err
			},
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			store, err := NewS3Store(S3Options{
				Endpoint:        "http://localhost:9000",
				Region:          "us-east-1",
				Bucket:          "gogomail",
				AccessKeyID:     "access",
				SecretAccessKey: "secret",
				ForcePathStyle:  true,
				HTTPClient: &http.Client{Transport: staticRoundTripper{
					resp: &http.Response{
						StatusCode: http.StatusNotFound,
						Body:       io.NopCloser(strings.NewReader("<Error><Code>NoSuchKey</Code></Error>")),
					},
				}},
			})
			if err != nil {
				t.Fatalf("NewS3Store returned error: %v", err)
			}
			err = tc.run(store)
			if !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("%s err = %v, want os.ErrNotExist", tc.name, err)
			}
		})
	}
}

func TestS3StoreCopyRejectsEmbeddedErrorInOKResponse(t *testing.T) {
	t.Parallel()

	store, err := NewS3Store(S3Options{
		Endpoint:        "http://localhost:9000",
		Region:          "us-east-1",
		Bucket:          "gogomail",
		AccessKeyID:     "access",
		SecretAccessKey: "secret",
		ForcePathStyle:  true,
		HTTPClient: &http.Client{Transport: staticRoundTripper{
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(strings.NewReader(`<Error>
  <Code>SlowDown</Code>
  <Message>copy throttled
try later</Message>
</Error>`)),
			},
		}},
	})
	if err != nil {
		t.Fatalf("NewS3Store returned error: %v", err)
	}
	err = store.Copy(context.Background(), "messages/msg-1.eml", "messages/msg-2.eml")
	if err == nil || !strings.Contains(err.Error(), "embedded error") || !strings.Contains(err.Error(), "SlowDown: copy throttled try later") || strings.ContainsAny(err.Error(), "\r\n") {
		t.Fatalf("Copy err = %q, want sanitized embedded error rejection", err)
	}
}

func TestS3StoreCopyAcceptsCopyObjectResult(t *testing.T) {
	t.Parallel()

	for _, body := range []string{
		`<CopyObjectResult><ETag>"etag-1"</ETag></CopyObjectResult>`,
		`<CopyObjectResult><ETag>"etag-1"</ETag><LastModified>2026-05-05T12:00:00Z</LastModified></CopyObjectResult>`,
		`<CopyObjectResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><ETag>"etag-1"</ETag></CopyObjectResult>`,
	} {
		body := body
		t.Run(body, func(t *testing.T) {
			t.Parallel()

			store, err := NewS3Store(S3Options{
				Endpoint:        "http://localhost:9000",
				Region:          "us-east-1",
				Bucket:          "gogomail",
				AccessKeyID:     "access",
				SecretAccessKey: "secret",
				ForcePathStyle:  true,
				HTTPClient: &http.Client{Transport: staticRoundTripper{
					resp: &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(strings.NewReader(body)),
					},
				}},
			})
			if err != nil {
				t.Fatalf("NewS3Store returned error: %v", err)
			}
			if err := store.Copy(context.Background(), "messages/msg-1.eml", "messages/msg-2.eml"); err != nil {
				t.Fatalf("Copy returned error: %v", err)
			}
		})
	}
}

func TestS3StoreCopyRequiresOKCopyObjectResult(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{name: "no_content", status: http.StatusNoContent, body: `<CopyObjectResult/>`, want: "status 204"},
		{name: "empty_ok", status: http.StatusOK, body: "", want: "response body is required"},
		{name: "unexpected_xml", status: http.StatusOK, body: `<Result/>`, want: `unexpected response "Result"`},
		{name: "unexpected_namespace", status: http.StatusOK, body: `<CopyObjectResult xmlns="urn:not-s3"/>`, want: "unexpected response namespace"},
		{name: "unexpected_child_namespace", status: http.StatusOK, body: `<CopyObjectResult><x:ETag xmlns:x="urn:not-s3">"a"</x:ETag></CopyObjectResult>`, want: "unexpected response namespace"},
		{name: "duplicate_etag", status: http.StatusOK, body: `<CopyObjectResult><ETag>"a"</ETag><ETag>"b"</ETag></CopyObjectResult>`, want: "duplicate etag"},
		{name: "invalid_etag", status: http.StatusOK, body: `<CopyObjectResult><ETag>"bad&#xA;etag"</ETag></CopyObjectResult>`, want: "invalid etag"},
		{name: "duplicate_last_modified", status: http.StatusOK, body: `<CopyObjectResult><LastModified>2026-05-05T12:00:00Z</LastModified><LastModified>2026-05-06T12:00:00Z</LastModified></CopyObjectResult>`, want: "duplicate last-modified"},
		{name: "invalid_last_modified", status: http.StatusOK, body: `<CopyObjectResult><LastModified>not-a-time</LastModified></CopyObjectResult>`, want: "invalid last-modified"},
		{name: "padded_last_modified", status: http.StatusOK, body: `<CopyObjectResult><LastModified> 2026-05-05T12:00:00Z </LastModified></CopyObjectResult>`, want: "invalid last-modified"},
		{name: "nested_error", status: http.StatusOK, body: `<CopyObjectResult><Error><Code>SlowDown</Code></Error></CopyObjectResult>`, want: "embedded error"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			store, err := NewS3Store(S3Options{
				Endpoint:        "http://localhost:9000",
				Region:          "us-east-1",
				Bucket:          "gogomail",
				AccessKeyID:     "access",
				SecretAccessKey: "secret",
				ForcePathStyle:  true,
				HTTPClient: &http.Client{Transport: staticRoundTripper{
					resp: &http.Response{
						StatusCode: tc.status,
						Body:       io.NopCloser(strings.NewReader(tc.body)),
					},
				}},
			})
			if err != nil {
				t.Fatalf("NewS3Store returned error: %v", err)
			}
			err = store.Copy(context.Background(), "messages/msg-1.eml", "messages/msg-2.eml")
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Copy err = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestS3StoreCopyRejectsOversizedOKResponse(t *testing.T) {
	t.Parallel()

	store, err := NewS3Store(S3Options{
		Endpoint:        "http://localhost:9000",
		Region:          "us-east-1",
		Bucket:          "gogomail",
		AccessKeyID:     "access",
		SecretAccessKey: "secret",
		ForcePathStyle:  true,
		HTTPClient: &http.Client{Transport: staticRoundTripper{
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(strings.Repeat("x", maxS3CopyResponseBytes+1))),
			},
		}},
	})
	if err != nil {
		t.Fatalf("NewS3Store returned error: %v", err)
	}
	err = store.Copy(context.Background(), "messages/msg-1.eml", "messages/msg-2.eml")
	if err == nil || !strings.Contains(err.Error(), "response body is too large") {
		t.Fatalf("Copy err = %v, want oversized body rejection", err)
	}
}

func TestValidateS3BucketNameRejectsUnsafeNames(t *testing.T) {
	t.Parallel()

	for _, bucket := range []string{
		"GoGoMail",
		"ab",
		"-gogomail",
		"gogomail-",
		".gogomail",
		"gogomail.",
		"gogo..mail",
		"gogo.-mail",
		"gogo_mail",
		"gogo/mail",
		"192.168.5.4",
		"xn--gogomail",
		"sthree-gogomail",
		"amzn-s3-demo-gogomail",
		"gogomail-s3alias",
		"gogomail--ol-s3",
		"gogomail.mrap",
		"gogomail--x-s3",
		"gogomail--table-s3",
	} {
		bucket := bucket
		t.Run(bucket, func(t *testing.T) {
			t.Parallel()

			if err := ValidateS3BucketName(bucket); err == nil {
				t.Fatalf("ValidateS3BucketName(%q) error = nil, want rejection", bucket)
			}
		})
	}
}

func TestValidateS3EndpointRejectsAmbiguousTargets(t *testing.T) {
	t.Parallel()

	for _, endpoint := range []string{
		"ftp://localhost:9000",
		"http://access:secret@localhost:9000",
		"http://localhost:9000?region=us-east-1",
		"http://localhost:9000#bucket",
		"http://localhost:9000//proxy",
		"http://localhost:9000/proxy//s3",
		"http://localhost:9000/proxy/../s3",
		"http://localhost:9000/proxy/./s3",
		"http://localhost:9000/proxy%2Fs3",
		"http://localhost:9000/proxy%5Cs3",
		"http://localhost:9000\nx",
	} {
		endpoint := endpoint
		t.Run(endpoint, func(t *testing.T) {
			t.Parallel()

			if _, err := ValidateS3Endpoint(endpoint); err == nil {
				t.Fatalf("ValidateS3Endpoint(%q) error = nil, want rejection", endpoint)
			}
		})
	}
}

func TestValidateS3EndpointAcceptsCanonicalBasePath(t *testing.T) {
	t.Parallel()

	endpoint, err := ValidateS3Endpoint(" http://localhost:9000/proxy/s3/ ")
	if err != nil {
		t.Fatalf("ValidateS3Endpoint returned error: %v", err)
	}
	if endpoint.Scheme != "http" || endpoint.Host != "localhost:9000" || endpoint.Path != "/proxy/s3/" {
		t.Fatalf("endpoint = %s", endpoint.String())
	}
}

func TestValidateS3RegionRejectsUnsafeValues(t *testing.T) {
	t.Parallel()

	for _, region := range []string{"", "us east 1", "us/east/1", "US-EAST-1", "us-east-1\nbad"} {
		region := region
		t.Run(region, func(t *testing.T) {
			t.Parallel()

			if err := ValidateS3Region(region); err == nil {
				t.Fatalf("ValidateS3Region(%q) error = nil, want rejection", region)
			}
		})
	}
}

func TestNewS3StoreRejectsSecretAccessKeyWhitespace(t *testing.T) {
	t.Parallel()

	for _, secretAccessKey := range []string{" secret", "secret ", "secret\tvalue", "secret\nvalue"} {
		secretAccessKey := secretAccessKey
		t.Run(secretAccessKey, func(t *testing.T) {
			t.Parallel()

			_, err := NewS3Store(S3Options{
				Endpoint:        "http://localhost:9000",
				Region:          "us-east-1",
				Bucket:          "gogomail",
				AccessKeyID:     "access",
				SecretAccessKey: secretAccessKey,
				ForcePathStyle:  true,
			})
			if err == nil {
				t.Fatalf("NewS3Store accepted secret access key %q", secretAccessKey)
			}
			if !strings.Contains(err.Error(), "s3 secret access key") || !strings.Contains(err.Error(), "whitespace") {
				t.Fatalf("error = %q, want secret access key whitespace rejection", err)
			}
		})
	}
}

func TestNewS3StoreRejectsAccessKeyIDWhitespace(t *testing.T) {
	t.Parallel()

	for _, accessKeyID := range []string{" access", "access ", "access\tkey", "access\nkey"} {
		accessKeyID := accessKeyID
		t.Run(accessKeyID, func(t *testing.T) {
			t.Parallel()

			_, err := NewS3Store(S3Options{
				Endpoint:        "http://localhost:9000",
				Region:          "us-east-1",
				Bucket:          "gogomail",
				AccessKeyID:     accessKeyID,
				SecretAccessKey: "secret",
				ForcePathStyle:  true,
			})
			if err == nil {
				t.Fatalf("NewS3Store accepted access key id %q", accessKeyID)
			}
			if !strings.Contains(err.Error(), "s3 access key id") || !strings.Contains(err.Error(), "whitespace") {
				t.Fatalf("error = %q, want access key id whitespace rejection", err)
			}
		})
	}
}

func TestNewS3StoreRejectsSessionTokenWhitespace(t *testing.T) {
	t.Parallel()

	for _, sessionToken := range []string{" token", "token ", "token\tvalue", "token\nvalue"} {
		sessionToken := sessionToken
		t.Run(sessionToken, func(t *testing.T) {
			t.Parallel()

			_, err := NewS3Store(S3Options{
				Endpoint:        "http://localhost:9000",
				Region:          "us-east-1",
				Bucket:          "gogomail",
				AccessKeyID:     "access",
				SecretAccessKey: "secret",
				SessionToken:    sessionToken,
				ForcePathStyle:  true,
			})
			if err == nil {
				t.Fatalf("NewS3Store accepted session token %q", sessionToken)
			}
			if !strings.Contains(err.Error(), "s3 session token") || !strings.Contains(err.Error(), "whitespace") {
				t.Fatalf("error = %q, want session token whitespace rejection", err)
			}
		})
	}
}

func TestNewS3StoreRejectsOversizedCredentials(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mutate  func(*S3Options)
		wantErr string
	}{
		{
			name: "access key id",
			mutate: func(opts *S3Options) {
				opts.AccessKeyID = strings.Repeat("a", maxS3AccessKeyIDBytes+1)
			},
			wantErr: "s3 access key id",
		},
		{
			name: "secret access key",
			mutate: func(opts *S3Options) {
				opts.SecretAccessKey = strings.Repeat("s", maxS3SecretAccessKeyBytes+1)
			},
			wantErr: "s3 secret access key",
		},
		{
			name: "session token",
			mutate: func(opts *S3Options) {
				opts.SessionToken = strings.Repeat("t", maxS3SessionTokenBytes+1)
			},
			wantErr: "s3 session token",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			opts := S3Options{
				Endpoint:        "http://localhost:9000",
				Region:          "us-east-1",
				Bucket:          "gogomail",
				AccessKeyID:     "access",
				SecretAccessKey: "secret",
				ForcePathStyle:  true,
			}
			tt.mutate(&opts)
			_, err := NewS3Store(opts)
			if err == nil {
				t.Fatal("NewS3Store accepted oversized credential")
			}
			if !strings.Contains(err.Error(), tt.wantErr) || !strings.Contains(err.Error(), "too long") {
				t.Fatalf("error = %q, want %s too long rejection", err, tt.wantErr)
			}
		})
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

func TestS3StoreFormatsXMLStatusErrorPreview(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`<Error>
  <Code>SlowDown</Code>
  <Message>retry
later</Message>
  <RequestId>req-
123</RequestId>
</Error>`))
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
	_, err = store.Get(context.Background(), "messages/msg-1.eml")
	if err == nil || !strings.Contains(err.Error(), "403") || !strings.Contains(err.Error(), "SlowDown: retry later: request-id=req- 123") || strings.Contains(err.Error(), "<Error>") || strings.ContainsAny(err.Error(), "\r\n") {
		t.Fatalf("error = %q, want formatted XML status preview", err)
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
		HTTPClient:      s3IntegrationHTTPClientFromEnv(t),
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
	info, err := store.Stat(ctx, objectPath)
	if err != nil {
		t.Fatalf("Stat returned error: %v", err)
	}
	if info.Path != objectPath || info.Size != int64(len(body)) {
		t.Fatalf("object info = %+v", info)
	}
	list, err := store.List(ctx, ListOptions{Prefix: "integration", Limit: 100})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	found := false
	for _, object := range list.Objects {
		if object.Path == objectPath {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("List did not include %q: %+v", objectPath, list)
	}
	copyPath := objectPath + ".copy"
	if err := store.Copy(ctx, objectPath, copyPath); err != nil {
		t.Fatalf("Copy returned error: %v", err)
	}
	defer func() {
		if err := store.Delete(ctx, copyPath); err != nil {
			t.Fatalf("Delete copy cleanup returned error: %v", err)
		}
	}()
	copied, err := store.Get(ctx, copyPath)
	if err != nil {
		t.Fatalf("Get copied object returned error: %v", err)
	}
	copiedBody, err := io.ReadAll(copied)
	if err != nil {
		t.Fatalf("read copied body: %v", err)
	}
	if err := copied.Close(); err != nil {
		t.Fatalf("close copied body: %v", err)
	}
	if string(copiedBody) != body {
		t.Fatalf("copied body = %q, want %q", copiedBody, body)
	}

	contractPrefix := "integration/contract-" + strings.ReplaceAll(t.Name(), "/", "-") + "-" + time.Now().UTC().Format("20060102150405.000000000")
	assertStorePortabilityContract(t, store, contractPrefix)
}

func TestS3IntegrationHTTPClientFromEnvWiresTLSOverrides(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()
	certFile := t.TempDir() + "/s3-test-ca.pem"
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
	if err := os.WriteFile(certFile, certPEM, 0o600); err != nil {
		t.Fatalf("write CA file: %v", err)
	}
	t.Setenv("GOGOMAIL_TEST_S3_CA_CERT_FILE", certFile)
	t.Setenv("GOGOMAIL_TEST_S3_INSECURE_SKIP_VERIFY", "true")

	client := s3IntegrationHTTPClientFromEnv(t)
	if client == nil {
		t.Fatal("client = nil, want custom integration HTTP client")
	}
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport = %T, want *http.Transport", client.Transport)
	}
	if transport.TLSClientConfig == nil {
		t.Fatal("TLSClientConfig = nil, want S3 integration TLS overrides")
	}
	if !transport.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("InsecureSkipVerify = false, want test override")
	}
	if transport.TLSClientConfig.MinVersion != tls.VersionTLS12 {
		t.Fatalf("MinVersion = %x, want TLS 1.2", transport.TLSClientConfig.MinVersion)
	}
	if len(transport.TLSClientConfig.RootCAs.Subjects()) == 0 {
		t.Fatal("RootCAs is empty, want custom CA bundle appended")
	}
}

func s3IntegrationHTTPClientFromEnv(t *testing.T) *http.Client {
	t.Helper()
	caCertFile := strings.TrimSpace(os.Getenv("GOGOMAIL_TEST_S3_CA_CERT_FILE"))
	insecureSkipVerify := strings.EqualFold(strings.TrimSpace(os.Getenv("GOGOMAIL_TEST_S3_INSECURE_SKIP_VERIFY")), "true")
	if caCertFile == "" && !insecureSkipVerify {
		return nil
	}
	rootCAs, err := x509.SystemCertPool()
	if err != nil || rootCAs == nil {
		rootCAs = x509.NewCertPool()
	}
	if caCertFile != "" {
		data, err := os.ReadFile(caCertFile)
		if err != nil {
			t.Fatalf("read GOGOMAIL_TEST_S3_CA_CERT_FILE: %v", err)
		}
		if !rootCAs.AppendCertsFromPEM(data) {
			t.Fatalf("GOGOMAIL_TEST_S3_CA_CERT_FILE must contain at least one PEM-encoded certificate")
		}
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{
		MinVersion:         tls.VersionTLS12,
		RootCAs:            rootCAs,
		InsecureSkipVerify: insecureSkipVerify,
	}
	return &http.Client{
		Timeout:   30 * time.Second,
		Transport: transport,
	}
}
