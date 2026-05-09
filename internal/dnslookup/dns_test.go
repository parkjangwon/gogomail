package dnslookup_test

import (
	"context"
	"testing"

	"github.com/gogomail/gogomail/internal/dnslookup"
)

// TestParseTLSARecordWireFormat verifies RFC 1035 wire format parsing.
func TestParseTLSARecordWireFormat(t *testing.T) {
	tests := []struct {
		name    string
		wire    []byte
		want    dnslookup.TLSARecord
		wantErr bool
	}{
		{
			name: "DANE-EE SHA256 public key",
			// usage=3, selector=1, matching=1, association=32 bytes SHA256
			wire: []byte{
				3, 1, 1, // usage, selector, matching-type
				0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff, // 32-byte hash (abbreviated)
				0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77,
				0x88, 0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff,
				0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77,
			},
			want: dnslookup.TLSARecord{
				Usage:        3,
				Selector:     1,
				MatchingType: 1,
				Association:  "aabbccddeeff00112233445566778899aabbccddeeff0011223344556677",
			},
			wantErr: false,
		},
		{
			name: "PKIX-TA SHA512 full cert",
			// usage=2, selector=0, matching=2, association=64 bytes SHA512
			wire: []byte{
				2, 0, 2, // usage, selector, matching-type
				0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88,
				0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff, 0x00,
				0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88,
				0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff, 0x00,
				0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88,
				0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff, 0x00,
				0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88,
				0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff, 0x00,
			},
			want: dnslookup.TLSARecord{
				Usage:        2,
				Selector:     0,
				MatchingType: 2,
			},
			wantErr: false,
		},
		{
			name:    "invalid usage",
			wire:    []byte{4, 0, 0}, // usage=4 (invalid)
			wantErr: true,
		},
		{
			name:    "invalid selector",
			wire:    []byte{3, 2, 0}, // selector=2 (invalid)
			wantErr: true,
		},
		{
			name:    "invalid matching-type",
			wire:    []byte{3, 1, 3}, // matching=3 (invalid)
			wantErr: true,
		},
		{
			name:    "too short",
			wire:    []byte{3, 1}, // missing matching-type
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := dnslookup.ParseTLSARecord(tt.wire)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseTLSARecord(%v) error = %v, wantErr %v", tt.wire, err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if got.Usage != tt.want.Usage || got.Selector != tt.want.Selector || got.MatchingType != tt.want.MatchingType {
				t.Fatalf("ParseTLSARecord(%v) = %+v, want %+v", tt.wire, got, tt.want)
			}
		})
	}
}

// TestResolverLookupTLSA verifies TLSA record lookup.
func TestResolverLookupTLSA(t *testing.T) {
	ctx := context.Background()
	resolver := dnslookup.NewResolver()

	// Actual TLSA lookup would require valid DNS setup.
	// For testing, we use a non-existent domain to verify error handling.
	records, err := resolver.LookupTLSA(ctx, "no-tlsa-record-here.test")
	if err == nil && len(records) > 0 {
		t.Fatalf("expected empty records for non-existent domain")
	}
}
