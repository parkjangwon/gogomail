package maildb

import (
	"strings"
	"testing"
)

func TestValidateUpsertPushDeviceRequest(t *testing.T) {
	t.Parallel()

	err := ValidateUpsertPushDeviceRequest(UpsertPushDeviceRequest{
		UserID:   "user-1",
		Platform: " FCM ",
		Token:    "token-1",
		Label:    "phone",
	})
	if err != nil {
		t.Fatalf("ValidateUpsertPushDeviceRequest returned error: %v", err)
	}
}

func TestValidateUpsertPushDeviceRequestRejectsUnsafeValues(t *testing.T) {
	t.Parallel()

	tests := []UpsertPushDeviceRequest{
		{UserID: "", Platform: "fcm", Token: "token"},
		{UserID: "user-1\nbad", Platform: "fcm", Token: "token"},
		{UserID: strings.Repeat("u", maxPushDeviceUserIDBytes+1), Platform: "fcm", Token: "token"},
		{UserID: "user-1", Platform: "sms", Token: "token"},
		{UserID: "user-1", Platform: "fcm", Token: ""},
		{UserID: "user-1", Platform: "fcm", Token: strings.Repeat("t", maxPushDeviceTokenBytes+1)},
		{UserID: "user-1", Platform: "fcm", Token: "line\nbreak"},
		{UserID: "user-1", Platform: "fcm", Token: "token", Label: strings.Repeat("x", maxPushDeviceLabelBytes+1)},
	}
	for _, req := range tests {
		req := req
		t.Run(req.Platform+"/"+req.Token, func(t *testing.T) {
			t.Parallel()
			if err := ValidateUpsertPushDeviceRequest(req); err == nil {
				t.Fatalf("ValidateUpsertPushDeviceRequest accepted %+v", req)
			}
		})
	}
}
