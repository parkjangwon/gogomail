package dane

import (
	"context"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/miekg/dns"
)

// TLSARecord represents a DNS TLSA record (RFC 6698).
// Format: _port._proto.name TTL IN TLSA usage selector matching-type association
type TLSARecord struct {
	Usage        int    // 0=CA, 1=Service, 2=Trust Anchor, 3=Domain-issued EE
	Selector     int    // 0=full cert, 1=public key only
	MatchingType int    // 0=exact, 1=SHA-256, 2=SHA-512
	Association  string // hex-encoded hash or full data
}

// ValidationResult indicates whether a certificate matches DANE policy.
type ValidationResult struct {
	Present bool   // TLSA records exist for this domain
	Valid   bool   // Certificate matches at least one record
	Records int    // Number of TLSA records checked
	Reason  string // Explanation if invalid
}

// Resolver looks up TLSA records via DNS.
type Resolver interface {
	// LookupTLSA returns TLSA records for _port._proto.domain
	LookupTLSA(ctx context.Context, domain string) ([]TLSARecord, error)
}

// Validator validates TLS certificates against DANE policy.
type Validator struct {
	resolver Resolver
}

// NewValidator creates a new DANE validator.
func NewValidator(resolver Resolver) *Validator {
	return &Validator{resolver: resolver}
}

// Validate checks if the given certificate matches DANE policy for the domain.
// Returns a ValidationResult indicating match status.
func (v *Validator) Validate(ctx context.Context, domain string, port int, certs []*tls.Certificate) (ValidationResult, error) {
	if len(certs) == 0 {
		return ValidationResult{Reason: "no certificates provided"}, fmt.Errorf("no certificates to validate")
	}

	records, err := v.resolver.LookupTLSA(ctx, domain)
	if err != nil {
		// Treat lookup errors as "no DANE policy"
		return ValidationResult{Present: false, Reason: fmt.Sprintf("TLSA lookup failed: %v", err)}, nil
	}

	if len(records) == 0 {
		return ValidationResult{Present: false}, nil
	}

	// At least one record exists; check if cert matches any of them
	for _, record := range records {
		if v.matches(&record, certs[0]) {
			return ValidationResult{Present: true, Valid: true, Records: len(records)}, nil
		}
	}

	return ValidationResult{
		Present: true,
		Valid:   false,
		Records: len(records),
		Reason:  "certificate does not match any TLSA record",
	}, nil
}

// matches checks if a certificate matches a TLSA record.
func (v *Validator) matches(record *TLSARecord, cert *tls.Certificate) bool {
	if len(cert.Certificate) == 0 {
		return false
	}

	// Parse the first certificate (end-entity)
	parsed, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return false
	}

	// DANE-EE (usage=3): validate end-entity cert directly
	if record.Usage == 3 {
		return v.matchesEECert(record, cert, parsed)
	}

	// PKIX-TA (usage=2): validate against trust anchor
	if record.Usage == 2 {
		// Simplified: just match the cert for now
		return v.matchesCertData(record, cert.Certificate[0])
	}

	return false
}

// matchesEECert validates end-entity certificate.
func (v *Validator) matchesEECert(record *TLSARecord, cert *tls.Certificate, parsed *x509.Certificate) bool {
	switch record.Selector {
	case 0: // Full certificate
		return v.matchesCertData(record, cert.Certificate[0])
	case 1: // Public key only
		return v.matchesPubKey(record, parsed.PublicKey)
	default:
		return false
	}
}

// matchesCertData checks if the certificate bytes match the record.
func (v *Validator) matchesCertData(record *TLSARecord, certDER []byte) bool {
	expected := v.hashData(record.MatchingType, certDER)
	return strings.EqualFold(expected, record.Association)
}

// matchesPubKey checks if the public key matches the record.
func (v *Validator) matchesPubKey(record *TLSARecord, pubKey interface{}) bool {
	pubKeyDER, err := x509.MarshalPKIXPublicKey(pubKey)
	if err != nil {
		return false
	}
	expected := v.hashData(record.MatchingType, pubKeyDER)
	return strings.EqualFold(expected, record.Association)
}

// hashData applies the matching-type hash function.
func (v *Validator) hashData(matchingType int, data []byte) string {
	switch matchingType {
	case 0: // Exact match
		return hex.EncodeToString(data)
	case 1: // SHA-256
		h := sha256.Sum256(data)
		return hex.EncodeToString(h[:])
	case 2: // SHA-512
		h := sha512.Sum512(data)
		return hex.EncodeToString(h[:])
	default:
		return ""
	}
}

// NetResolver implements Resolver using net.Resolver for TLSA records.
type NetResolver struct {
	*net.Resolver
}

// LookupTLSA queries DNS for TLSA records at _port._proto.domain (RFC 6698 §3.1).
// Returns parsed TLSA records from DNS wire format.
func (r *NetResolver) LookupTLSA(ctx context.Context, domain string) ([]TLSARecord, error) {
	// Query _25._tcp.domain for SMTP (RFC 6698)
	tlsaDomain := "_25._tcp." + domain

	client := &dns.Client{
		Timeout: 5 * time.Second,
	}

	msg := &dns.Msg{}
	msg.SetQuestion(dns.Fqdn(tlsaDomain), dns.TypeTLSA)
	msg.RecursionDesired = true

	// Query default nameservers
	resp, _, err := client.ExchangeContext(ctx, msg, "")
	if err != nil {
		return nil, fmt.Errorf("dns query: %w", err)
	}

	if resp.Rcode != dns.RcodeSuccess {
		return nil, fmt.Errorf("dns query failed: rcode=%d", resp.Rcode)
	}

	var records []TLSARecord
	for _, answer := range resp.Answer {
		tlsaRR, ok := answer.(*dns.TLSA)
		if !ok {
			continue
		}

		record := TLSARecord{
			Usage:        int(tlsaRR.Usage),
			Selector:     int(tlsaRR.Selector),
			MatchingType: int(tlsaRR.MatchingType),
			Association:  tlsaRR.Certificate, // Already hex-encoded by miekg/dns
		}
		records = append(records, record)
	}

	if len(records) == 0 {
		return nil, fmt.Errorf("no TLSA records found")
	}

	return records, nil
}
