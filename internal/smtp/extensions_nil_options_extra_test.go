package smtpd

import "testing"

func TestValidateOptionsAcceptNilOptionPointers(t *testing.T) {
	if err := validateMailOptions(nil, extensionSupport{}); err != nil {
		t.Fatalf("validateMailOptions(nil) returned error: %v", err)
	}
	if err := validateRcptOptions(nil, extensionSupport{}); err != nil {
		t.Fatalf("validateRcptOptions(nil) returned error: %v", err)
	}
}
