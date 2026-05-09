package dnslookup

import (
	"context"
	"encoding/hex"
	"fmt"
	"net"
)

// TLSARecord represents a parsed TLSA record from DNS wire format (RFC 1035, RFC 6698).
type TLSARecord struct {
	Usage        int    // 0=CA, 1=Service, 2=Trust Anchor, 3=Domain-issued EE
	Selector     int    // 0=full cert, 1=public key only
	MatchingType int    // 0=exact, 1=SHA-256, 2=SHA-512
	Association  string // hex-encoded hash or full data
}

// ParseTLSARecord parses a TLSA record from RFC 1035 wire format.
// Wire format: 1 byte usage, 1 byte selector, 1 byte matching-type, N bytes association.
func ParseTLSARecord(wire []byte) (TLSARecord, error) {
	if len(wire) < 3 {
		return TLSARecord{}, fmt.Errorf("tlsa record too short: %d bytes", len(wire))
	}

	usage := int(wire[0])
	selector := int(wire[1])
	matchingType := int(wire[2])

	// RFC 6698 §2.1.1: validate usage
	if usage > 3 {
		return TLSARecord{}, fmt.Errorf("invalid tlsa usage: %d", usage)
	}

	// RFC 6698 §2.1.2: validate selector
	if selector > 1 {
		return TLSARecord{}, fmt.Errorf("invalid tlsa selector: %d", selector)
	}

	// RFC 6698 §2.1.3: validate matching-type
	if matchingType > 2 {
		return TLSARecord{}, fmt.Errorf("invalid tlsa matching-type: %d", matchingType)
	}

	association := hex.EncodeToString(wire[3:])

	return TLSARecord{
		Usage:        usage,
		Selector:     selector,
		MatchingType: matchingType,
		Association:  association,
	}, nil
}

// Resolver looks up DNS TLSA records.
type Resolver struct {
	net.Resolver
}

// NewResolver creates a new DNS resolver.
func NewResolver() *Resolver {
	return &Resolver{
		Resolver: net.Resolver{},
	}
}

// LookupTLSA queries DNS for TLSA records (type 52) at _port._proto.domain.
// This is a simplified implementation; production would use a DNS library (miekg/dns)
// to properly decode wire format responses.
func (r *Resolver) LookupTLSA(ctx context.Context, domain string) ([]TLSARecord, error) {
	// Go's net package doesn't directly support TLSA (type 52) lookups.
	// Production implementation would:
	// 1. Use miekg/dns or similar DNS library
	// 2. Query authoritative nameservers
	// 3. Parse wire format response
	// 4. Validate DNSSEC (if present)
	//
	// For now, return empty — TLSA lookups require external DNS library.
	return nil, nil
}
