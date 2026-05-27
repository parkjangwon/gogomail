package config

import "testing"

func TestValidateRejectsUnsafeSMTPDomain(t *testing.T) {
	for _, domain := range []string{"", " ", "mx example.com", "mx.example.com\nInjected"} {
		t.Setenv("GOGOMAIL_ENV", "development")
		cfg := Load()
		cfg.SMTPDomain = domain
		if err := cfg.Validate(); err == nil {
			t.Fatalf("Validate() accepted unsafe SMTP domain %q", domain)
		}
	}
}

func TestValidateAcceptsSMTPDomainHostname(t *testing.T) {
	t.Setenv("GOGOMAIL_ENV", "development")
	cfg := Load()
	cfg.SMTPDomain = "mx.example.com"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateRejectsUnsafeDeliverySMTPHello(t *testing.T) {
	for _, hello := range []string{"", " ", "mx example.com", "mx.example.com\nInjected"} {
		t.Setenv("GOGOMAIL_ENV", "development")
		cfg := Load()
		cfg.DeliverySMTPHello = hello
		if err := cfg.Validate(); err == nil {
			t.Fatalf("Validate() accepted unsafe delivery SMTP hello %q", hello)
		}
	}
}

func TestValidateAcceptsDeliverySMTPHelloHostname(t *testing.T) {
	t.Setenv("GOGOMAIL_ENV", "development")
	cfg := Load()
	cfg.DeliverySMTPHello = "mx.example.com"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}
