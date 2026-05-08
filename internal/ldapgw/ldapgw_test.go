package ldapgw

import (
	"testing"
)

func TestBerEncodeBindRequest(t *testing.T) {
	req := bindRequest{
		version: 3,
		name:    "cn=admin,dc=example,dc=com",
		auth:    simpleAuth("secret"),
	}

	data, err := req.encode()
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("encode returned empty data")
	}
}

func TestBerDecodeBindRequest(t *testing.T) {
	req := bindRequest{
		version: 3,
		name:    "cn=admin,dc=example,dc=com",
		auth:    simpleAuth("secret"),
	}

	data, err := req.encode()
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	decoded, err := decodeBindRequest(data)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if decoded.version != 3 {
		t.Fatalf("version = %d, want 3", decoded.version)
	}
	if decoded.name != "cn=admin,dc=example,dc=com" {
		t.Fatalf("name = %s, want cn=admin,dc=example,dc=com", decoded.name)
	}
	if string(decoded.auth) != "secret" {
		t.Fatalf("auth = %s, want secret", string(decoded.auth))
	}
}

func TestIsReadOnlyOperation(t *testing.T) {
	tests := []struct {
		op     int
		readOnly bool
	}{
		{opBindRequest, true},
		{opSearchRequest, true},
		{opUnbindRequest, true},
		{opModifyRequest, false},
		{opAddRequest, false},
		{opDeleteRequest, false},
		{opModDNRequest, false},
		{opCompareRequest, true},
		{opAbandonRequest, true},
	}

	for _, tt := range tests {
		got := isReadOnlyOperation(tt.op)
		if got != tt.readOnly {
			t.Errorf("isReadOnlyOperation(%d) = %v, want %v", tt.op, got, tt.readOnly)
		}
	}
}

func TestEscapeLDAPValue(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello", "hello"},
		{"te*st", "te\\2ast"},
		{"te(st)", "te\\28st\\29"},
		{"te\\st", "te\\5cst"},
		{"te\x00st", "te\\00st"},
	}

	for _, tt := range tests {
		got := escapeLDAPValue(tt.input)
		if got != tt.expected {
			t.Errorf("escapeLDAPValue(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
