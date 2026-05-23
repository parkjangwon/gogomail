package pushnotify_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gogomail/gogomail/internal/pushnotify"
)

// Valid keys for tests (generated with webpush.GenerateVAPIDKeys and crypto/ecdsa for P256DH).
const (
	testVAPIDPublicKey  = "BIFo_B1iElvT9990ASQW-JIsBDIM8ydzYauX1bT6ZAFOMxKJ-W1XGjsclaIIZframJWIDe-ZX1SmnfRP7Lu_Iqs"
	testVAPIDPrivateKey = "wQ73AGXEif7PXLItveCGlTryGnOTB8vlb-ESldP6R2Y"
	// 65-byte uncompressed P-256 point, base64url-encoded (no padding).
	testP256DH = "BDoCP0AejNQ6QfwpxYRDua7q9TvDDehrsFRpPaYSoYdu2uzGISGCGtm3vDB2H47JcnIucARzuYbDXgYiCLC_gsQ"
	testAuth   = "KyD20_flTp6ZArVev8Eiew"
)

type fakeWebPushSubReader struct {
	subs            []pushnotify.WebPushSubData
	deletedEndpoint string
}

func (f *fakeWebPushSubReader) ListActiveWebPushSubscriptions(_ context.Context, _ string) ([]pushnotify.WebPushSubData, error) {
	return f.subs, nil
}

func (f *fakeWebPushSubReader) SoftDeleteWebPushSubscriptionByEndpoint(_ context.Context, endpoint string) error {
	f.deletedEndpoint = endpoint
	return nil
}

func TestWebPushSink_EnqueuePush_NoSubscriptions(t *testing.T) {
	sink, err := pushnotify.NewWebPushSink(pushnotify.WebPushSinkOptions{
		VAPIDPublicKey:  testVAPIDPublicKey,
		VAPIDPrivateKey: testVAPIDPrivateKey,
		ContactEmail:    "admin@example.com",
		DB:              &fakeWebPushSubReader{},
	})
	if err != nil {
		t.Fatalf("NewWebPushSink: %v", err)
	}
	err = sink.EnqueuePush(context.Background(), pushnotify.Notification{UserID: "u1"})
	if err != nil {
		t.Fatalf("EnqueuePush with no subs should not error: %v", err)
	}
}

func TestWebPushSink_NewWebPushSink_MissingVAPIDPublicKey(t *testing.T) {
	_, err := pushnotify.NewWebPushSink(pushnotify.WebPushSinkOptions{
		VAPIDPrivateKey: testVAPIDPrivateKey,
		DB:              &fakeWebPushSubReader{},
	})
	if err == nil {
		t.Fatal("expected error for missing VAPID public key")
	}
}

func TestWebPushSink_NewWebPushSink_MissingVAPIDPrivateKey(t *testing.T) {
	_, err := pushnotify.NewWebPushSink(pushnotify.WebPushSinkOptions{
		VAPIDPublicKey: testVAPIDPublicKey,
		DB:             &fakeWebPushSubReader{},
	})
	if err == nil {
		t.Fatal("expected error for missing VAPID private key")
	}
}

func TestWebPushSink_NewWebPushSink_MissingDB(t *testing.T) {
	_, err := pushnotify.NewWebPushSink(pushnotify.WebPushSinkOptions{
		VAPIDPublicKey:  testVAPIDPublicKey,
		VAPIDPrivateKey: testVAPIDPrivateKey,
	})
	if err == nil {
		t.Fatal("expected error for missing DB")
	}
}

func TestWebPushSink_EnqueuePush_GoneDeletesSubscription(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusGone)
	}))
	defer server.Close()

	reader := &fakeWebPushSubReader{
		subs: []pushnotify.WebPushSubData{{
			ID:       "sub1",
			Endpoint: server.URL,
			P256DH:   testP256DH,
			Auth:     testAuth,
		}},
	}
	sink, err := pushnotify.NewWebPushSink(pushnotify.WebPushSinkOptions{
		VAPIDPublicKey:  testVAPIDPublicKey,
		VAPIDPrivateKey: testVAPIDPrivateKey,
		ContactEmail:    "admin@example.com",
		DB:              reader,
		HTTPClient:      server.Client(),
	})
	if err != nil {
		t.Fatalf("NewWebPushSink: %v", err)
	}
	_ = sink.EnqueuePush(context.Background(), pushnotify.Notification{UserID: "u1", Subject: "Test"})
	if reader.deletedEndpoint != server.URL {
		t.Errorf("expected endpoint %q to be soft-deleted, got %q", server.URL, reader.deletedEndpoint)
	}
}
