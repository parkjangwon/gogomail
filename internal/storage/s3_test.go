package storage

import (
	"context"
	"errors"
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
	info, err := store.Stat(context.Background(), "messages/msg-1.eml")
	if err != nil {
		t.Fatalf("Stat returned error: %v", err)
	}
	if info.Path != "messages/msg-1.eml" || info.Size != 5 || info.ContentType != "message/rfc822" || info.ETag != "etag-1" || info.LastModified.IsZero() {
		t.Fatalf("object info = %+v", info)
	}
	if err := store.Delete(context.Background(), "messages/msg-1.eml"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}

	want := []string{
		"PUT /gogomail/mail/messages/msg-1.eml",
		"GET /gogomail/mail/messages/msg-1.eml",
		"HEAD /gogomail/mail/messages/msg-1.eml",
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
	if _, err := store.Stat(ctx, "messages/msg-1.eml"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Stat err = %v, want context.Canceled", err)
	}
	if err := store.Delete(ctx, "messages/msg-1.eml"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Delete err = %v, want context.Canceled", err)
	}
}

func TestS3StoreStatRequiresValidContentLength(t *testing.T) {
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
				Header:        http.Header{"Content-Length": []string{"not-a-size"}},
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
	if err := store.Put(context.Background(), "../bad", strings.NewReader("bad")); err == nil {
		t.Fatal("Put accepted unsafe object path")
	}
	if _, err := store.Stat(context.Background(), "../bad"); err == nil {
		t.Fatal("Stat accepted unsafe object path")
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
	info, err := store.Stat(ctx, objectPath)
	if err != nil {
		t.Fatalf("Stat returned error: %v", err)
	}
	if info.Path != objectPath || info.Size != int64(len(body)) {
		t.Fatalf("object info = %+v", info)
	}
}
