package app

import (
	"testing"

	"github.com/gogomail/gogomail/internal/config"
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
