package dane_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"math/big"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/dane"
)

// mockDNSResolver returns predefined TLSA records for testing.
type mockDNSResolver struct {
	records map[string][]dane.TLSARecord
}

func (m *mockDNSResolver) LookupTLSA(ctx context.Context, domain string) ([]dane.TLSARecord, error) {
	recs := m.records[domain]
	return recs, nil
}

func TestValidatorAcceptsPinnedPublicKey(t *testing.T) {
	cert := mustGenerateTestCert(t, "mx.example.com")

	// Extract public key SHA-256
	pubKey := cert.PublicKey.(*rsa.PublicKey)
	pubKeyDER, err := x509.MarshalPKIXPublicKey(pubKey)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}
	pubKeyHash := sha256.Sum256(pubKeyDER)
	pubKeyHashHex := hex.EncodeToString(pubKeyHash[:])

	// Create TLSA record: DANE-EE (usage=3), public-key (selector=1), SHA-256 (matching=1)
	tlsaRec := dane.TLSARecord{
		Usage:        3, // DANE-EE
		Selector:     1, // public key
		MatchingType: 1, // SHA-256
		Association:  pubKeyHashHex,
	}

	resolver := &mockDNSResolver{
		records: map[string][]dane.TLSARecord{
			"mx.example.com": {tlsaRec},
		},
	}

	validator := dane.NewValidator(resolver)
	tlsCert := &tls.Certificate{Certificate: [][]byte{cert.Raw}}

	result, err := validator.Validate(context.Background(), "mx.example.com", 25, []*tls.Certificate{tlsCert})
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !result.Valid {
		t.Fatalf("expected valid, got invalid: %s", result.Reason)
	}
}

func TestValidatorRejectsMismatchedPublicKey(t *testing.T) {
	cert := mustGenerateTestCert(t, "mx.example.com")

	// Use wrong hash
	wrongHash := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	tlsaRec := dane.TLSARecord{
		Usage:        3,
		Selector:     1,
		MatchingType: 1,
		Association:  wrongHash,
	}

	resolver := &mockDNSResolver{
		records: map[string][]dane.TLSARecord{
			"mx.example.com": {tlsaRec},
		},
	}

	validator := dane.NewValidator(resolver)
	tlsCert := &tls.Certificate{Certificate: [][]byte{cert.Raw}}

	result, err := validator.Validate(context.Background(), "mx.example.com", 25, []*tls.Certificate{tlsCert})
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if result.Valid {
		t.Fatalf("expected invalid, got valid")
	}
}

func TestValidatorNoRecords(t *testing.T) {
	cert := mustGenerateTestCert(t, "mx.example.com")

	resolver := &mockDNSResolver{
		records: map[string][]dane.TLSARecord{},
	}

	validator := dane.NewValidator(resolver)
	tlsCert := &tls.Certificate{Certificate: [][]byte{cert.Raw}}

	result, err := validator.Validate(context.Background(), "mx.example.com", 25, []*tls.Certificate{tlsCert})
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if result.Present {
		t.Fatalf("expected no DANE records, but marked as present")
	}
}

func TestValidatorAcceptsFullCertificateMatch(t *testing.T) {
	cert := mustGenerateTestCert(t, "mx.example.com")

	// SHA-256 hash of full certificate
	certHash := sha256.Sum256(cert.Raw)
	certHashHex := hex.EncodeToString(certHash[:])

	tlsaRec := dane.TLSARecord{
		Usage:        3, // DANE-EE
		Selector:     0, // full certificate
		MatchingType: 1, // SHA-256
		Association:  certHashHex,
	}

	resolver := &mockDNSResolver{
		records: map[string][]dane.TLSARecord{
			"mx.example.com": {tlsaRec},
		},
	}

	validator := dane.NewValidator(resolver)
	tlsCert := &tls.Certificate{Certificate: [][]byte{cert.Raw}}

	result, err := validator.Validate(context.Background(), "mx.example.com", 25, []*tls.Certificate{tlsCert})
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !result.Valid {
		t.Fatalf("expected valid certificate match, got invalid")
	}
}

func TestValidatorNoCertificates(t *testing.T) {
	resolver := &mockDNSResolver{
		records: map[string][]dane.TLSARecord{},
	}

	validator := dane.NewValidator(resolver)
	result, err := validator.Validate(context.Background(), "mx.example.com", 25, []*tls.Certificate{})
	if err == nil {
		t.Fatal("expected error for empty certificate list")
	}
	if result.Valid {
		t.Fatal("expected invalid result for no certificates")
	}
}

// mustGenerateTestCert creates a self-signed certificate for testing.
func mustGenerateTestCert(t *testing.T, domain string) *x509.Certificate {
	t.Helper()

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	notBefore := time.Now()
	notAfter := notBefore.Add(365 * 24 * time.Hour)

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		t.Fatalf("generate serial: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: domain,
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{domain},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatalf("parse certificate: %v", err)
	}

	return cert
}
