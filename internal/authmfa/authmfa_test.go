package authmfa

import (
	"testing"
	"time"
)

func TestGenerateSecret(t *testing.T) {
	secret, err := GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret returned error: %v", err)
	}
	if len(secret) == 0 {
		t.Fatal("GenerateSecret returned empty secret")
	}

	// Should generate different secrets each time
	secret2, err := GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret returned error: %v", err)
	}
	if secret == secret2 {
		t.Fatal("GenerateSecret should return different secrets")
	}
}

func TestGenerateTOTP(t *testing.T) {
	secret, err := GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret returned error: %v", err)
	}

	code, err := GenerateTOTP(secret, time.Now())
	if err != nil {
		t.Fatalf("GenerateTOTP returned error: %v", err)
	}
	if len(code) != 6 {
		t.Fatalf("GenerateTOTP returned %d digits, want 6", len(code))
	}

	// Same time should generate same code
	code2, err := GenerateTOTP(secret, time.Now())
	if err != nil {
		t.Fatalf("GenerateTOTP returned error: %v", err)
	}
	if code != code2 {
		t.Fatal("GenerateTOTP should return same code for same time")
	}
}

func TestVerifyTOTP(t *testing.T) {
	secret, err := GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret returned error: %v", err)
	}

	now := time.Now()
	code, err := GenerateTOTP(secret, now)
	if err != nil {
		t.Fatalf("GenerateTOTP returned error: %v", err)
	}

	// Valid code should verify
	if !VerifyTOTP(secret, code, now) {
		t.Fatal("VerifyTOTP should return true for valid code")
	}

	// Invalid code should not verify
	if VerifyTOTP(secret, "000000", now) {
		t.Fatal("VerifyTOTP should return false for invalid code")
	}
}

func TestVerifyTOTPWindow(t *testing.T) {
	secret, err := GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret returned error: %v", err)
	}

	now := time.Now()
	code, err := GenerateTOTP(secret, now)
	if err != nil {
		t.Fatalf("GenerateTOTP returned error: %v", err)
	}

	// Code should be valid within ±2 windows (±2 minutes)
	past := now.Add(-1 * time.Minute)
	if !VerifyTOTP(secret, code, past) {
		t.Fatal("VerifyTOTP should accept code from 1 minute ago")
	}

	future := now.Add(1 * time.Minute)
	if !VerifyTOTP(secret, code, future) {
		t.Fatal("VerifyTOTP should accept code from 1 minute in the future")
	}

	// Code should be invalid outside window
	farPast := now.Add(-3 * time.Minute)
	if VerifyTOTP(secret, code, farPast) {
		t.Fatal("VerifyTOTP should reject code from 3 minutes ago")
	}

	farFuture := now.Add(3 * time.Minute)
	if VerifyTOTP(secret, code, farFuture) {
		t.Fatal("VerifyTOTP should reject code from 3 minutes in the future")
	}
}

func TestGenerateRecoveryCodes(t *testing.T) {
	codes, err := GenerateRecoveryCodes(8)
	if err != nil {
		t.Fatalf("GenerateRecoveryCodes returned error: %v", err)
	}
	if len(codes) != 8 {
		t.Fatalf("GenerateRecoveryCodes returned %d codes, want 8", len(codes))
	}

	// All codes should be unique
	seen := make(map[string]bool)
	for _, code := range codes {
		if seen[code] {
			t.Fatal("GenerateRecoveryCodes returned duplicate codes")
		}
		seen[code] = true
		if len(code) != 10 {
			t.Fatalf("recovery code length = %d, want 10", len(code))
		}
	}
}

func TestVerifyRecoveryCode(t *testing.T) {
	codes, err := GenerateRecoveryCodes(8)
	if err != nil {
		t.Fatalf("GenerateRecoveryCodes returned error: %v", err)
	}

	// Valid code should verify and be consumed
	remaining, ok := VerifyRecoveryCode(codes, codes[0])
	if !ok {
		t.Fatal("VerifyRecoveryCode should return true for valid code")
	}
	if len(remaining) != 7 {
		t.Fatalf("VerifyRecoveryCode returned %d remaining codes, want 7", len(remaining))
	}

	// Same code should not verify again
	_, ok = VerifyRecoveryCode(remaining, codes[0])
	if ok {
		t.Fatal("VerifyRecoveryCode should return false for already used code")
	}

	// Invalid code should not verify
	_, ok = VerifyRecoveryCode(remaining, "invalid")
	if ok {
		t.Fatal("VerifyRecoveryCode should return false for invalid code")
	}
}
