package app

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	gosmtp "github.com/emersion/go-smtp"
	"github.com/gogomail/gogomail/internal/config"
	"github.com/gogomail/gogomail/internal/delivery"
	"github.com/gogomail/gogomail/internal/maildb"
)

func TestSMTPTLSConfigRequiresCertAndKeyTogether(t *testing.T) {
	t.Parallel()

	if _, err := smtpTLSConfig(config.Config{SMTPTLSCertFile: "cert.pem"}); err == nil {
		t.Fatal("smtpTLSConfig accepted certificate without key")
	}
	if _, err := smtpTLSConfig(config.Config{SMTPTLSKeyFile: "key.pem"}); err == nil {
		t.Fatal("smtpTLSConfig accepted key without certificate")
	}
}

func TestSMTPTLSConfigAllowsNoTLSFiles(t *testing.T) {
	t.Parallel()

	tlsConfig, err := smtpTLSConfig(config.Config{})
	if err != nil {
		t.Fatalf("smtpTLSConfig returned error: %v", err)
	}
	if tlsConfig != nil {
		t.Fatal("smtpTLSConfig returned config without TLS files")
	}
}

func TestDKIMKeyProviderMapsRepositoryKey(t *testing.T) {
	t.Parallel()

	provider := dkimKeyProvider{repository: fakeDKIMKeyRepository{key: maildb.DKIMKey{
		DomainID:      "domain-1",
		DomainName:    "example.com",
		Selector:      "s1",
		PrivateKeyPEM: "private",
	}}}
	key, err := provider.DKIMKey(context.Background(), delivery.Job{
		QueuedMessage: delivery.QueuedMessage{DomainID: "domain-1"},
	})
	if err != nil {
		t.Fatalf("DKIMKey returned error: %v", err)
	}
	if key.Domain != "example.com" || key.Selector != "s1" || key.PrivateKeyPEM != "private" {
		t.Fatalf("key = %+v", key)
	}
}

func TestSubmissionServerOptionsSelectSMTPSAddress(t *testing.T) {
	t.Parallel()

	cfg := config.Load()
	cfg.SMTPDomain = "mail.example"
	cfg.SubmissionAddr = ":2587"
	cfg.SubmissionSMTPSAddr = " :2465 "
	cfg.SMTPReadTimeout = 7 * time.Second
	cfg.SMTPWriteTimeout = 8 * time.Second
	cfg.SubmissionMaxMessageBytes = 1234
	cfg.SubmissionMaxRecipients = 12
	cfg.SubmissionAllowInsecureAuth = false
	cfg.SubmissionSupportDSN = true
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	backend := gosmtp.BackendFunc(func(*gosmtp.Conn) (gosmtp.Session, error) {
		return nil, nil
	})

	opts := submissionServerOptions(cfg, nil, backend, tlsConfig, true)

	if opts.Addr != ":2465" {
		t.Fatalf("Addr = %q, want SMTPS addr", opts.Addr)
	}
	if !opts.ImplicitTLS {
		t.Fatal("ImplicitTLS = false, want true")
	}
	if opts.TLSConfig != tlsConfig {
		t.Fatal("TLSConfig was not preserved")
	}
	if opts.AllowInsecureAuth {
		t.Fatal("AllowInsecureAuth = true, want false")
	}
	if !opts.EnableDSN {
		t.Fatal("EnableDSN = false, want true")
	}
}

func TestAPIMeteringHandlerDefaultsToOriginalHandler(t *testing.T) {
	t.Parallel()

	next := &sentinelHTTPHandler{}
	handler := apiMeteringHandler(next, config.Config{APIMeteringBackend: "none"}, nil, nil)
	if handler != next {
		t.Fatal("apiMeteringHandler wrapped handler when backend is none")
	}
}

func TestAPIMeteringHandlerWrapsSlogBackend(t *testing.T) {
	t.Parallel()

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})
	handler := apiMeteringHandler(next, config.Config{
		APIMeteringBackend: "slog",
		APIMeteringTimeout: 100 * time.Millisecond,
	}, nil, nil)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/info", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusAccepted)
	}
}

func TestAPIMeteringHandlerRequiresOutboxDB(t *testing.T) {
	t.Parallel()

	next := &sentinelHTTPHandler{}
	handler := apiMeteringHandler(next, config.Config{APIMeteringBackend: "outbox"}, nil, nil)
	if handler != next {
		t.Fatal("apiMeteringHandler wrapped outbox backend without database handle")
	}
}

type fakeDKIMKeyRepository struct {
	key          maildb.DKIMKey
	lastDomainID string
}

type sentinelHTTPHandler struct{}

func (*sentinelHTTPHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func (r fakeDKIMKeyRepository) ActiveDKIMKey(_ context.Context, domainID string) (maildb.DKIMKey, error) {
	r.lastDomainID = domainID
	return r.key, nil
}
