package ldapgw

import (
	"bytes"
	"testing"
)

func TestPDUParsing(t *testing.T) {
	// Build a simple bind request PDU
	msgID := 1
	opTag := opBindRequest
	bindData := []byte{0x02, 0x01, 0x03, 0x04, 0x15, 'a', 'd', 'm', 'i', 'n', '@', 'e', 'x', 0x80, 0x06, 's', 'e', 'c', 'r', 'e', 't'}
	
	// Build full PDU using buildLDAPPacket logic
	var opContent []byte
	opContent = append([]byte{byte(opTag)}, encodeLength(len(bindData))...)
	opContent = append(opContent, bindData...)
	
	var msgIDContent []byte
	msgIDContent = append(msgIDContent, tagInteger)
	msgIDContent = append(msgIDContent, encodeLength(1)...)
	msgIDContent = append(msgIDContent, byte(msgID))
	
	var seqContent []byte
	seqContent = append(seqContent, msgIDContent...)
	seqContent = append(seqContent, opContent...)
	
	pdu := make([]byte, 0, 2+len(seqContent))
	pdu = append(pdu, tagSequence)
	pdu = append(pdu, encodeLength(len(seqContent))...)
	pdu = append(pdu, seqContent...)
	
	t.Logf("PDU bytes: %x", pdu)
	t.Logf("PDU len: %d", len(pdu))
	
	msgID2, opTag2, opData2, err := decodeLDAPPacket(pdu)
	if err != nil {
		t.Fatalf("decodeLDAPPacket failed: %v", err)
	}
	t.Logf("msgID=%d, opTag=%d, opData len=%d", msgID2, opTag2, len(opData2))
	
	// Test parsePDULength
	pduLen, headerLen := parsePDULength(pdu)
	t.Logf("parsePDULength: pduLen=%d, headerLen=%d", pduLen, headerLen)
	
	if pduLen == 0 {
		t.Error("pduLen should not be 0")
	}
	
	// Test full encode/decode roundtrip
	pdu2 := buildLDAPPacket(1, opBindRequest, bindData)
	t.Logf("buildLDAPPacket: %x", pdu2)
	
	msgID3, opTag3, _, err := decodeLDAPPacket(pdu2)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if msgID3 != 1 || opTag3 != opBindRequest {
		t.Errorf("msgID=%d, opTag=%d, want 1, %d", msgID3, opTag3, opBindRequest)
	}
}

func TestParsePDULength(t *testing.T) {
	// Minimal SEQUENCE with one byte content
	data := []byte{0x30, 0x01, 0x01}
	pduLen, headerLen := parsePDULength(data)
	t.Logf("data=%x, pduLen=%d, headerLen=%d", data, pduLen, headerLen)
	
	// Full bind request
	bindData := []byte{0x02, 0x01, 0x03, 0x04, 0x15, 'a', 'd', 'm', 'i', 'n', '@', 'e', 'x', 0x80, 0x06, 's', 'e', 'c', 'r', 'e', 't'}
	var opContent []byte
	opContent = append([]byte{byte(opBindRequest)}, encodeLength(len(bindData))...)
	opContent = append(opContent, bindData...)
	
	var msgIDContent []byte
	msgIDContent = append(msgIDContent, tagInteger, 0x01, 0x01)
	
	var seqContent bytes.Buffer
	seqContent.Write(msgIDContent)
	seqContent.Write(opContent)
	
	fullPDU := []byte{0x30, byte(seqContent.Len())}
	fullPDU = append(fullPDU, seqContent.Bytes()...)
	
	t.Logf("fullPDU=%x", fullPDU)
	pduLen2, headerLen2 := parsePDULength(fullPDU)
	t.Logf("fullPDU: pduLen=%d, headerLen=%d, actual len=%d", pduLen2, headerLen2, len(fullPDU))
	
	if pduLen2 == 0 || pduLen2+headerLen2 != len(fullPDU) {
		t.Errorf("parsePDULength mismatch: pduLen=%d, headerLen=%d, total=%d, actual=%d", pduLen2, headerLen2, pduLen2+headerLen2, len(fullPDU))
	}
}
