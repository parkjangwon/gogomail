package app

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	gosmtp "github.com/emersion/go-smtp"
	"github.com/gogomail/gogomail/internal/apimeter"
	"github.com/gogomail/gogomail/internal/auth"
	"github.com/gogomail/gogomail/internal/config"
	"github.com/gogomail/gogomail/internal/delivery"
	"github.com/gogomail/gogomail/internal/imapgw"
	"github.com/gogomail/gogomail/internal/maildb"
	"github.com/gogomail/gogomail/internal/storage"
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

func TestIMAPTLSConfigRequiresCertAndKeyTogether(t *testing.T) {
	t.Parallel()

	if _, err := imapTLSConfig(config.Config{IMAPTLSCertFile: "cert.pem"}); err == nil {
		t.Fatal("imapTLSConfig accepted certificate without key")
	}
	if _, err := imapTLSConfig(config.Config{IMAPTLSKeyFile: "key.pem"}); err == nil {
		t.Fatal("imapTLSConfig accepted key without certificate")
	}
}

func TestIMAPTLSServerNameUsesListenerHost(t *testing.T) {
	t.Parallel()

	if got := imapTLSServerName(config.Config{IMAPAddr: "imap.example.com:993", SMTPDomain: "smtp.example.com"}); got != "imap.example.com" {
		t.Fatalf("server name = %q, want imap listener host", got)
	}
	if got := imapTLSServerName(config.Config{IMAPAddr: ":1143", SMTPDomain: "smtp.example.com"}); got != "smtp.example.com" {
		t.Fatalf("server name = %q, want SMTP domain fallback", got)
	}
}

type fakeStorageChecker struct {
	err error
}

func (f fakeStorageChecker) Check(context.Context) error {
	return f.err
}

func TestStorageReadinessCheckReportsProbeStatus(t *testing.T) {
	t.Parallel()

	ok := storageReadinessCheck("mail_storage", fakeStorageChecker{})(context.Background())
	if ok.Name != "mail_storage" || ok.Status != "ok" || ok.Detail == "" {
		t.Fatalf("ok check = %+v", ok)
	}
	failed := storageReadinessCheck("mail_storage", fakeStorageChecker{err: context.Canceled})(context.Background())
	if failed.Name != "mail_storage" || failed.Status != "error" || !strings.Contains(failed.Detail, "context canceled") {
		t.Fatalf("failed check = %+v", failed)
	}
}

func TestNewHTTPServerUsesConfiguredOperationalGuardrails(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		HTTPAddr:              ":18080",
		HTTPReadTimeout:       45 * time.Second,
		HTTPWriteTimeout:      90 * time.Second,
		HTTPIdleTimeout:       75 * time.Second,
		HTTPReadHeaderTimeout: 3 * time.Second,
		HTTPMaxHeaderBytes:    32 << 10,
	}
	handler := http.NewServeMux()
	server := newHTTPServer(cfg, handler)
	if server.Addr != ":18080" || server.Handler != handler {
		t.Fatalf("server identity = addr:%q handler:%T", server.Addr, server.Handler)
	}
	if server.ReadTimeout != 45*time.Second ||
		server.WriteTimeout != 90*time.Second ||
		server.IdleTimeout != 75*time.Second ||
		server.ReadHeaderTimeout != 3*time.Second ||
		server.MaxHeaderBytes != 32<<10 {
		t.Fatalf("server guardrails = read:%s write:%s idle:%s readHeader:%s maxHeader:%d",
			server.ReadTimeout,
			server.WriteTimeout,
			server.IdleTimeout,
			server.ReadHeaderTimeout,
			server.MaxHeaderBytes,
		)
	}
}

func TestObjectStoreForConfigRejectsUnsupportedBackend(t *testing.T) {
	t.Parallel()

	store, err := objectStoreForConfig(config.Config{StorageBackend: "minio", MailstoreRoot: t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), "unsupported storage backend") {
		t.Fatalf("store = %+v err = %v", store, err)
	}
}

func TestObjectStoreForConfigBuildsS3Backend(t *testing.T) {
	t.Parallel()

	store, err := objectStoreForConfig(config.Config{
		StorageBackend:           "s3",
		StorageS3Endpoint:        "http://localhost:9000",
		StorageS3Region:          "us-east-1",
		StorageS3Bucket:          "gogomail",
		StorageS3AccessKeyID:     "access",
		StorageS3SecretAccessKey: "secret",
		StorageS3ForcePathStyle:  true,
	})
	if err != nil {
		t.Fatalf("objectStoreForConfig returned error: %v", err)
	}
	if _, ok := store.(*storage.S3Store); !ok {
		t.Fatalf("store = %T, want *storage.S3Store", store)
	}
}

func TestObjectStoreForConfigBuildsMinIOBackend(t *testing.T) {
	t.Parallel()

	store, err := objectStoreForConfig(config.Config{
		StorageBackend:           "minio",
		StorageS3Endpoint:        "http://localhost:9000",
		StorageS3Region:          "us-east-1",
		StorageS3Bucket:          "gogomail",
		StorageS3AccessKeyID:     "access",
		StorageS3SecretAccessKey: "secret",
	})
	if err != nil {
		t.Fatalf("objectStoreForConfig returned error: %v", err)
	}
	if _, ok := store.(*storage.S3Store); !ok {
		t.Fatalf("store = %T, want *storage.S3Store", store)
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

func TestNewIMAPGatewayRuntimeWiresMailboxEventBroker(t *testing.T) {
	t.Parallel()

	runtime := newIMAPGatewayRuntime(nil, nil, nil)
	if runtime.service == nil {
		t.Fatal("service is nil")
	}
	if runtime.events == nil {
		t.Fatal("mailbox event broker is nil")
	}
	if _, err := runtime.store.ListMailboxes(context.Background(), imapgw.ListMailboxesRequest{UserID: "user-1"}); err == nil || !strings.Contains(err.Error(), "imap mailbox repository is required") {
		t.Fatalf("store adapter was not wired through service boundary: %v", err)
	}
	if _, err := runtime.backend.Authenticate(context.Background(), "user@example.com", "secret"); err == nil || !strings.Contains(err.Error(), "imap authenticator is required") {
		t.Fatalf("backend authenticator was not wired through IMAP boundary: %v", err)
	}

	events, cancel, err := runtime.service.SubscribeIMAPMailbox(context.Background(), "user-1", "inbox")
	if err != nil {
		t.Fatalf("SubscribeIMAPMailbox returned error: %v", err)
	}
	defer cancel()

	if err := runtime.events.Publish(context.Background(), imapgw.MailboxEvent{
		Type:      imapgw.MailboxEventExists,
		UserID:    "user-1",
		MailboxID: "inbox",
		Messages:  1,
	}); err != nil {
		t.Fatalf("Publish returned error: %v", err)
	}

	select {
	case event := <-events:
		if event.Type != imapgw.MailboxEventExists || event.Messages != 1 {
			t.Fatalf("event = %+v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for mailbox event")
	}
}

func TestIMAPServerOptionsForConfigUsesRuntimeBackend(t *testing.T) {
	t.Parallel()

	runtime := newIMAPGatewayRuntime(nil, nil, nil)
	opts, err := imapServerOptionsForConfig(config.Config{
		IMAPAddr:              " :1143 ",
		IMAPAllowInsecureAuth: true,
		SMTPDomain:            "localhost",
	}, runtime.backend)
	if err != nil {
		t.Fatalf("imapServerOptionsForConfig returned error: %v", err)
	}
	if opts.Addr != ":1143" || opts.Backend == nil || opts.TLSConfig != nil || !opts.AllowInsecureAuth {
		t.Fatalf("options = %+v", opts)
	}
}

func TestAllInOneHTTPModeIncludesMailAndAdminAPIs(t *testing.T) {
	t.Parallel()

	if !modeIncludesMailAPI(ModeAllInOne) {
		t.Fatal("all-in-one did not include Mail API routes")
	}
	if !modeIncludesAdminAPI(ModeAllInOne) {
		t.Fatal("all-in-one did not include Admin API routes")
	}
	if modeIncludesAdminAPI(ModeMailAPI) {
		t.Fatal("mail-api should not include Admin API routes")
	}
	if modeIncludesMailAPI(ModeAdminAPI) {
		t.Fatal("admin-api should not include Mail API routes")
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

func TestAttachmentScanHooksForConfigDisabledByDefault(t *testing.T) {
	t.Parallel()

	hooks, err := attachmentScanHooksForConfig(config.Config{AttachmentScanBackend: "none"}, nil, "test")
	if err != nil {
		t.Fatalf("attachmentScanHooksForConfig returned error: %v", err)
	}
	if len(hooks) != 0 {
		t.Fatalf("hooks = %d, want none", len(hooks))
	}
}

func TestAttachmentScanHooksForConfigWebhook(t *testing.T) {
	t.Parallel()

	hooks, err := attachmentScanHooksForConfig(config.Config{
		AttachmentScanBackend:    "webhook",
		AttachmentScanWebhookURL: "http://scanner.example.test/scan",
		AttachmentScanTimeout:    time.Second,
	}, nil, "test")
	if err != nil {
		t.Fatalf("attachmentScanHooksForConfig returned error: %v", err)
	}
	if len(hooks) != 1 {
		t.Fatalf("hooks = %d, want one webhook scanner hook", len(hooks))
	}
}

func TestPushNotificationSinkForConfigWebhook(t *testing.T) {
	t.Parallel()

	sink, err := pushNotificationSinkForConfig(config.Config{
		PushNotifyBackend:        "webhook",
		PushNotifyWebhookURL:     "http://push.example.test/send",
		PushNotifyWebhookTimeout: time.Second,
	}, nil)
	if err != nil {
		t.Fatalf("pushNotificationSinkForConfig returned error: %v", err)
	}
	if sink == nil {
		t.Fatal("sink is nil")
	}
}

func TestPushNotificationSinkForConfigRejectsUnknownBackend(t *testing.T) {
	t.Parallel()

	if _, err := pushNotificationSinkForConfig(config.Config{PushNotifyBackend: "direct"}, nil); err == nil {
		t.Fatal("pushNotificationSinkForConfig accepted unknown backend")
	}
}

func TestAPIMeteringHandlerDefaultsToOriginalHandler(t *testing.T) {
	t.Parallel()

	next := &sentinelHTTPHandler{}
	handler := apiMeteringHandler(next, config.Config{APIMeteringBackend: "none"}, nil, nil, nil, "")
	if handler != next {
		t.Fatal("apiMeteringHandler wrapped handler when backend is none")
	}
}

func TestAPIUsageExportManifestSignerConfig(t *testing.T) {
	disabled := config.Config{APIUsageExportManifestSignerBackend: "disabled"}
	if signer := apiUsageExportManifestSigner(disabled); signer != nil {
		t.Fatalf("disabled signer = %#v", signer)
	}
	if verifier := apiUsageExportManifestVerifier(disabled); verifier != nil {
		t.Fatalf("disabled verifier = %#v", verifier)
	}

	enabled := config.Config{
		APIUsageExportManifestSignerBackend: "local-hmac",
		APIUsageExportManifestSignerKeyID:   "key-1",
		APIUsageExportManifestSignerSecret:  "secret",
	}
	if signer := apiUsageExportManifestSigner(enabled); signer == nil {
		t.Fatal("local-hmac signer is nil")
	}
	if verifier := apiUsageExportManifestVerifier(enabled); verifier == nil {
		t.Fatal("local-hmac verifier is nil")
	}

	privateKey := ed25519.NewKeyFromSeed([]byte(strings.Repeat("s", ed25519.SeedSize)))
	publicKey := privateKey.Public().(ed25519.PublicKey)
	ed25519Enabled := config.Config{
		APIUsageExportManifestSignerBackend: "local-ed25519",
		APIUsageExportManifestSignerKeyID:   "key-2",
		APIUsageExportSignerPrivateKey:      base64.StdEncoding.EncodeToString(privateKey),
		APIUsageExportSignerPublicKey:       base64.StdEncoding.EncodeToString(publicKey),
	}
	if signer := apiUsageExportManifestSigner(ed25519Enabled); signer == nil {
		t.Fatal("local-ed25519 signer is nil")
	}
	if verifier := apiUsageExportManifestVerifier(ed25519Enabled); verifier == nil {
		t.Fatal("local-ed25519 verifier is nil")
	}

	remoteEd25519Enabled := config.Config{
		APIUsageExportManifestSignerBackend: "remote-ed25519",
		APIUsageExportManifestSignerKeyID:   "key-3",
		APIUsageExportSignerURL:             "https://signer.example.test/sign",
		APIUsageExportSignerPublicKey:       base64.StdEncoding.EncodeToString(publicKey),
	}
	if signer := apiUsageExportManifestSigner(remoteEd25519Enabled); signer == nil {
		t.Fatal("remote-ed25519 signer is nil")
	}
	if verifier := apiUsageExportManifestVerifier(remoteEd25519Enabled); verifier == nil {
		t.Fatal("remote-ed25519 verifier is nil")
	}
}

func TestAPIUsageExportManifestSignerRejectsOversizedKeysBeforeDecode(t *testing.T) {
	t.Parallel()

	oversizedPrivate := config.Config{
		APIUsageExportManifestSignerBackend: "local-ed25519",
		APIUsageExportManifestSignerKeyID:   "key-1",
		APIUsageExportSignerPrivateKey:      strings.Repeat("a", base64.StdEncoding.EncodedLen(ed25519.PrivateKeySize)+1),
		APIUsageExportSignerPublicKey:       base64.StdEncoding.EncodeToString([]byte(strings.Repeat("p", ed25519.PublicKeySize))),
	}
	if signer := apiUsageExportManifestSigner(oversizedPrivate); signer != nil {
		t.Fatal("apiUsageExportManifestSigner accepted oversized private key")
	}

	oversizedPublic := config.Config{
		APIUsageExportManifestSignerBackend: "remote-ed25519",
		APIUsageExportManifestSignerKeyID:   "key-2",
		APIUsageExportSignerURL:             "https://signer.example.test/sign",
		APIUsageExportSignerPublicKey:       strings.Repeat("a", base64.StdEncoding.EncodedLen(ed25519.PublicKeySize)+1),
	}
	if signer := apiUsageExportManifestSigner(oversizedPublic); signer != nil {
		t.Fatal("apiUsageExportManifestSigner accepted oversized public key")
	}
	if verifier := apiUsageExportManifestVerifier(oversizedPublic); verifier != nil {
		t.Fatal("apiUsageExportManifestVerifier accepted oversized public key")
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
	}, nil, nil, nil, "")

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
	handler := apiMeteringHandler(next, config.Config{APIMeteringBackend: "outbox"}, nil, nil, nil, "")
	if handler != next {
		t.Fatal("apiMeteringHandler wrapped outbox backend without database handle")
	}
}

func TestMeteringIdentityResolverUsesJWTClaims(t *testing.T) {
	t.Parallel()

	manager, err := auth.NewTokenManager("secret")
	if err != nil {
		t.Fatalf("NewTokenManager returned error: %v", err)
	}
	token, err := manager.Sign(auth.Claims{UserID: "user-1", DomainID: "domain-1"}, time.Minute)
	if err != nil {
		t.Fatalf("Sign returned error: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/messages", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	id := meteringIdentityResolver(manager, "")(req)
	if id.UserID != "user-1" || id.DomainID != "domain-1" {
		t.Fatalf("identity = %+v", id)
	}
	if id.AuthSource != apimeter.AuthSourceBearer || id.PrincipalID != "user-1" {
		t.Fatalf("identity principal = %+v", id)
	}
}

func TestMeteringIdentityResolverUsesAdminToken(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/api-usage/daily", nil)
	req.Header.Set("X-Admin-Token", "secret")

	id := meteringIdentityResolver(nil, "secret")(req)
	if id.AuthSource != apimeter.AuthSourceAdminToken || id.PrincipalID != apimeter.AuthSourceAdminToken {
		t.Fatalf("identity = %+v", id)
	}
}

func TestMeteringAdminTokenMatchesTrimsAndRejectsMismatches(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/api-usage/daily", nil)
	req.Header.Set("X-Admin-Token", " secret ")
	if !meteringAdminTokenMatches(req, "secret") {
		t.Fatal("meteringAdminTokenMatches rejected matching trimmed token")
	}

	for _, got := range []string{"", "secre", "secret-longer"} {
		req := httptest.NewRequest(http.MethodGet, "/admin/v1/api-usage/daily", nil)
		req.Header.Set("X-Admin-Token", got)
		if meteringAdminTokenMatches(req, "secret") {
			t.Fatalf("meteringAdminTokenMatches(%q) = true, want false", got)
		}
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
