package milter

import (
	"testing"
)

func TestCommandConstants(t *testing.T) {
	if cmdConnect != 'C' {
		t.Fatalf("cmdConnect = %d, want %d", cmdConnect, 'C')
	}
	if cmdHelo != 'H' {
		t.Fatalf("cmdHelo = %d, want %d", cmdHelo, 'H')
	}
	if cmdMail != 'M' {
		t.Fatalf("cmdMail = %d, want %d", cmdMail, 'M')
	}
	if cmdRcpt != 'R' {
		t.Fatalf("cmdRcpt = %d, want %d", cmdRcpt, 'R')
	}
	if cmdData != 'T' {
		t.Fatalf("cmdData = %d, want %d", cmdData, 'T')
	}
	if cmdEOB != 'E' {
		t.Fatalf("cmdEOB = %d, want %d", cmdEOB, 'E')
	}
}

func TestResponseConstants(t *testing.T) {
	if respContinue != 'c' {
		t.Fatalf("respContinue = %d, want %d", respContinue, 'c')
	}
	if respReject != 'r' {
		t.Fatalf("respReject = %d, want %d", respReject, 'r')
	}
	if respTempfail != 't' {
		t.Fatalf("respTempfail = %d, want %d", respTempfail, 't')
	}
}

func TestEncodePacket(t *testing.T) {
	pkt := &Packet{
		Command: cmdConnect,
		Data:    []byte("192.168.1.1\x00test.example.com"),
	}

	data, err := pkt.Encode()
	if err != nil {
		t.Fatalf("Encode error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("Encode returned empty data")
	}

	if len(data) < 5 {
		t.Fatalf("data too short: %d", len(data))
	}
}

func TestDecodePacket(t *testing.T) {
	original := &Packet{
		Command: cmdHelo,
		Data:    []byte("mail.example.com"),
	}

	encoded, err := original.Encode()
	if err != nil {
		t.Fatalf("Encode error: %v", err)
	}

	decoded, err := DecodePacket(encoded)
	if err != nil {
		t.Fatalf("DecodePacket error: %v", err)
	}
	if decoded.Command != cmdHelo {
		t.Fatalf("Command = %d, want %d", decoded.Command, cmdHelo)
	}
	if string(decoded.Data) != "mail.example.com" {
		t.Fatalf("Data = %s, want mail.example.com", string(decoded.Data))
	}
}

func TestDecodePacketTooShort(t *testing.T) {
	_, err := DecodePacket([]byte{0, 0, 0})
	if err == nil {
		t.Fatal("DecodePacket should error for short data")
	}
}

func TestActionFromCode(t *testing.T) {
	tests := []struct {
		code     byte
		expected Action
	}{
		{respContinue, ActionContinue},
		{respReject, ActionReject},
		{respTempfail, ActionTempfail},
		{'X', ActionUnknown},
	}

	for _, tt := range tests {
		got := ActionFromCode(tt.code)
		if got != tt.expected {
			t.Errorf("ActionFromCode(%d) = %d, want %d", tt.code, got, tt.expected)
		}
	}
}
