package app

import (
	"context"
	"testing"

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

type fakeDKIMKeyRepository struct {
	key          maildb.DKIMKey
	lastDomainID string
}

func (r fakeDKIMKeyRepository) ActiveDKIMKey(_ context.Context, domainID string) (maildb.DKIMKey, error) {
	r.lastDomainID = domainID
	return r.key, nil
}
