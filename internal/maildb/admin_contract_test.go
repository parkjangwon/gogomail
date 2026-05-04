package maildb

import (
	"strings"
	"testing"
)

func TestValidateUpdateDomainStatusRequestRejectsUnknownStatus(t *testing.T) {
	t.Parallel()

	if err := ValidateUpdateDomainStatusRequest(UpdateDomainStatusRequest{ID: "domain-1", Status: "paused"}); err == nil {
		t.Fatal("ValidateUpdateDomainStatusRequest accepted unknown status")
	}
}

func TestValidateUpdateDomainStatusRequestAcceptsSuspended(t *testing.T) {
	t.Parallel()

	if err := ValidateUpdateDomainStatusRequest(UpdateDomainStatusRequest{ID: "domain-1", Status: "suspended"}); err != nil {
		t.Fatalf("ValidateUpdateDomainStatusRequest returned error: %v", err)
	}
}

func TestValidateUpdateDomainQuotaRequestRejectsNegativeQuota(t *testing.T) {
	t.Parallel()

	err := ValidateUpdateDomainQuotaRequest(UpdateDomainQuotaRequest{ID: "domain-1", QuotaLimit: -1})
	if err == nil {
		t.Fatal("ValidateUpdateDomainQuotaRequest accepted negative quota")
	}
}

func TestValidateUpdateDomainQuotaRequestRejectsNegativeDefaultUserQuota(t *testing.T) {
	t.Parallel()

	defaultQuota := int64(-1)
	err := ValidateUpdateDomainQuotaRequest(UpdateDomainQuotaRequest{
		ID:               "domain-1",
		DefaultUserQuota: &defaultQuota,
	})
	if err == nil {
		t.Fatal("ValidateUpdateDomainQuotaRequest accepted negative default_user_quota")
	}
}

func TestValidateUpdateCompanyQuotaRequestRejectsNegativeQuota(t *testing.T) {
	t.Parallel()

	err := ValidateUpdateCompanyQuotaRequest(UpdateCompanyQuotaRequest{ID: "company-1", QuotaLimit: -1})
	if err == nil {
		t.Fatal("ValidateUpdateCompanyQuotaRequest accepted negative quota")
	}
}

func TestValidateCorrectQuotaReconciliationRequestDefaultsAll(t *testing.T) {
	t.Parallel()

	got, err := ValidateCorrectQuotaReconciliationRequest(CorrectQuotaReconciliationRequest{})
	if err != nil {
		t.Fatalf("ValidateCorrectQuotaReconciliationRequest returned error: %v", err)
	}
	if got.Scope != "all" {
		t.Fatalf("scope = %q, want all", got.Scope)
	}
}

func TestValidateCorrectQuotaReconciliationRequestRejectsInvalidScope(t *testing.T) {
	t.Parallel()

	if _, err := ValidateCorrectQuotaReconciliationRequest(CorrectQuotaReconciliationRequest{Scope: "folder"}); err == nil {
		t.Fatal("ValidateCorrectQuotaReconciliationRequest accepted invalid scope")
	}
}

func TestValidateCorrectQuotaReconciliationRequestRequiresIDForScopedCorrection(t *testing.T) {
	t.Parallel()

	if _, err := ValidateCorrectQuotaReconciliationRequest(CorrectQuotaReconciliationRequest{Scope: "domain"}); err == nil {
		t.Fatal("ValidateCorrectQuotaReconciliationRequest accepted missing scoped id")
	}
}

func TestValidateCorrectQuotaReconciliationRequestRejectsIDForAll(t *testing.T) {
	t.Parallel()

	if _, err := ValidateCorrectQuotaReconciliationRequest(CorrectQuotaReconciliationRequest{Scope: "all", ID: "domain-1"}); err == nil {
		t.Fatal("ValidateCorrectQuotaReconciliationRequest accepted id with all scope")
	}
}

func TestNormalizePushNotificationAttemptListRequestRejectsUnsafeFilters(t *testing.T) {
	t.Parallel()

	tests := []PushNotificationAttemptListRequest{
		{Status: "queued\nbad"},
		{UserID: strings.Repeat("u", maxPushNotificationFilterBytes+1)},
		{DeviceID: string([]byte{0xff})},
		{Platform: "pager"},
		{ProviderStatus: "accepted\rbad"},
		{ProviderMessageID: strings.Repeat("m", maxPushNotificationFilterBytes+1)},
	}
	for _, req := range tests {
		req := req
		t.Run(req.Status+req.UserID+req.DeviceID+req.Platform+req.ProviderStatus, func(t *testing.T) {
			t.Parallel()
			if _, err := normalizePushNotificationAttemptListRequest(req); err == nil {
				t.Fatalf("normalizePushNotificationAttemptListRequest accepted %+v", req)
			}
		})
	}
}

func TestNormalizePushNotificationAttemptListRequestNormalizesValues(t *testing.T) {
	t.Parallel()

	got, err := normalizePushNotificationAttemptListRequest(PushNotificationAttemptListRequest{
		Limit:             -1,
		Status:            " QUEUED ",
		UserID:            " user-1 ",
		Platform:          " FCM ",
		DeviceID:          " device-1 ",
		ProviderStatus:    " accepted ",
		ProviderMessageID: " provider-message-1 ",
	})
	if err != nil {
		t.Fatalf("normalizePushNotificationAttemptListRequest returned error: %v", err)
	}
	if got.Limit <= 0 || got.Status != "queued" || got.UserID != "user-1" || got.Platform != "fcm" || got.DeviceID != "device-1" || got.ProviderStatus != "accepted" || got.ProviderMessageID != "provider-message-1" {
		t.Fatalf("normalized request = %+v", got)
	}
}

func TestValidatePushNotificationAttemptIDRejectsUnsafeValues(t *testing.T) {
	t.Parallel()

	for _, id := range []string{
		"attempt-1\nbad",
		strings.Repeat("a", maxPushNotificationFilterBytes+1),
		string([]byte{0xff}),
	} {
		id := id
		t.Run(id, func(t *testing.T) {
			t.Parallel()
			if err := validatePushNotificationFilter("attempt_id", id); err == nil {
				t.Fatalf("validatePushNotificationFilter accepted %q", id)
			}
		})
	}
}

func TestNormalizePushNotificationStatsRequestRejectsUnsafeUserID(t *testing.T) {
	t.Parallel()

	for _, userID := range []string{
		"user-1\nbad",
		strings.Repeat("u", maxPushNotificationFilterBytes+1),
		string([]byte{0xff}),
	} {
		userID := userID
		t.Run(userID, func(t *testing.T) {
			t.Parallel()
			if _, err := normalizePushNotificationStatsRequest(PushNotificationStatsRequest{UserID: userID}); err == nil {
				t.Fatalf("normalizePushNotificationStatsRequest accepted %q", userID)
			}
		})
	}
}

func TestNormalizeUpdatePushNotificationOutcomeRequest(t *testing.T) {
	t.Parallel()

	req, err := normalizeUpdatePushNotificationOutcomeRequest(UpdatePushNotificationOutcomeRequest{
		AttemptID:         " attempt-1 ",
		Status:            " DELIVERED ",
		ErrorMessage:      strings.Repeat("e", 2100),
		ProviderMessageID: strings.Repeat("m", 600),
		ProviderStatus:    strings.Repeat("s", 600),
	})
	if err != nil {
		t.Fatalf("normalizeUpdatePushNotificationOutcomeRequest returned error: %v", err)
	}
	if req.AttemptID != "attempt-1" || req.Status != "delivered" {
		t.Fatalf("normalized request = %+v", req)
	}
	if len(req.ErrorMessage) != 2000 || len(req.ProviderMessageID) != 500 || len(req.ProviderStatus) != 500 {
		t.Fatalf("bounded lengths = %d/%d/%d", len(req.ErrorMessage), len(req.ProviderMessageID), len(req.ProviderStatus))
	}
}

func TestNormalizeUpdatePushNotificationOutcomeRequestRejectsUnsafeValues(t *testing.T) {
	t.Parallel()

	tests := []UpdatePushNotificationOutcomeRequest{
		{AttemptID: "", Status: "failed"},
		{AttemptID: "attempt-1\nbad", Status: "failed"},
		{AttemptID: strings.Repeat("a", maxPushNotificationFilterBytes+1), Status: "failed"},
		{AttemptID: string([]byte{0xff}), Status: "failed"},
		{AttemptID: "attempt-1", Status: "candidate"},
		{AttemptID: "attempt-1", Status: ""},
	}
	for _, req := range tests {
		req := req
		t.Run(req.AttemptID+req.Status, func(t *testing.T) {
			t.Parallel()
			if _, err := normalizeUpdatePushNotificationOutcomeRequest(req); err == nil {
				t.Fatalf("normalizeUpdatePushNotificationOutcomeRequest accepted %+v", req)
			}
		})
	}
}

func TestValidateUpdateDomainPolicyRequestNormalizesBlankModes(t *testing.T) {
	t.Parallel()

	err := ValidateUpdateDomainPolicyRequest(UpdateDomainPolicyRequest{ID: "domain-1"})
	if err != nil {
		t.Fatalf("ValidateUpdateDomainPolicyRequest returned error: %v", err)
	}
}

func TestValidateUpdateDomainPolicyRequestRejectsUnsafeValues(t *testing.T) {
	t.Parallel()

	for _, req := range []UpdateDomainPolicyRequest{
		{ID: "", InboundMode: "inherit", OutboundMode: "inherit"},
		{ID: "domain-1", InboundMode: "block", OutboundMode: "inherit"},
		{ID: "domain-1", InboundMode: "inherit", OutboundMode: "block"},
		{ID: "domain-1", MaxRecipientsPerMessage: -1},
		{ID: "domain-1", MaxMessageBytes: -1},
		{ID: "domain-1", MaxAttachmentBytes: -1},
	} {
		if err := ValidateUpdateDomainPolicyRequest(req); err == nil {
			t.Fatalf("ValidateUpdateDomainPolicyRequest(%+v) returned nil", req)
		}
	}
}

func TestValidateCreateDomainRequestRejectsInvalidName(t *testing.T) {
	t.Parallel()

	err := ValidateCreateDomainRequest(CreateDomainRequest{
		CompanyID: "company-1",
		Name:      "bad domain/example.com",
	})
	if err == nil {
		t.Fatal("ValidateCreateDomainRequest accepted invalid domain name")
	}
}

func TestValidateCreateDomainRequestRejectsEmptyLabels(t *testing.T) {
	t.Parallel()

	err := ValidateCreateDomainRequest(CreateDomainRequest{
		CompanyID: "company-1",
		Name:      "example..com",
	})
	if err == nil {
		t.Fatal("ValidateCreateDomainRequest accepted domain with empty label")
	}
}

func TestValidateCreateDomainRequestRejectsInvalidACEName(t *testing.T) {
	t.Parallel()

	err := ValidateCreateDomainRequest(CreateDomainRequest{
		CompanyID: "company-1",
		Name:      "example.com",
		NameACE:   "-bad.example.com",
	})
	if err == nil {
		t.Fatal("ValidateCreateDomainRequest accepted invalid ACE domain name")
	}
}

func TestValidateUpdateUserStatusRequestRejectsUnknownStatus(t *testing.T) {
	t.Parallel()

	if err := ValidateUpdateUserStatusRequest(UpdateUserStatusRequest{ID: "user-1", Status: "paused"}); err == nil {
		t.Fatal("ValidateUpdateUserStatusRequest accepted unknown status")
	}
}

func TestValidateUpdateUserQuotaRequestRejectsNegativeQuota(t *testing.T) {
	t.Parallel()

	err := ValidateUpdateUserQuotaRequest(UpdateUserQuotaRequest{ID: "user-1", QuotaLimit: -1})
	if err == nil {
		t.Fatal("ValidateUpdateUserQuotaRequest accepted negative quota")
	}
}

func TestValidateUpdateUserQuotaRequestRejectsInvalidQuotaSource(t *testing.T) {
	t.Parallel()

	err := ValidateUpdateUserQuotaRequest(UpdateUserQuotaRequest{ID: "user-1", QuotaSource: "inherited"})
	if err == nil {
		t.Fatal("ValidateUpdateUserQuotaRequest accepted invalid quota_source")
	}
}

func TestValidateCreateUserRequestRejectsInvalidUsername(t *testing.T) {
	t.Parallel()

	err := ValidateCreateUserRequest(CreateUserRequest{
		DomainID:    "domain-1",
		Username:    "admin@example.com",
		DisplayName: "Admin",
		Address:     "admin@example.com",
	})
	if err == nil {
		t.Fatal("ValidateCreateUserRequest accepted invalid username")
	}
}

func TestValidateCreateUserRequestRejectsDottyUsername(t *testing.T) {
	t.Parallel()

	err := ValidateCreateUserRequest(CreateUserRequest{
		DomainID:    "domain-1",
		Username:    "admin..ops",
		DisplayName: "Admin",
		Address:     "admin@example.com",
	})
	if err == nil {
		t.Fatal("ValidateCreateUserRequest accepted dotty username")
	}
}

func TestValidateCreateUserRequestRejectsInvalidAddress(t *testing.T) {
	t.Parallel()

	err := ValidateCreateUserRequest(CreateUserRequest{
		DomainID:    "domain-1",
		Username:    "admin",
		DisplayName: "Admin",
		Address:     "not an address",
	})
	if err == nil {
		t.Fatal("ValidateCreateUserRequest accepted invalid address")
	}
}

func TestValidateCreateUserRequestRejectsMismatchedPrimaryAddress(t *testing.T) {
	t.Parallel()

	err := ValidateCreateUserRequest(CreateUserRequest{
		DomainID:    "domain-1",
		Username:    "admin",
		DisplayName: "Admin",
		Address:     "ops@example.com",
	})
	if err == nil {
		t.Fatal("ValidateCreateUserRequest accepted mismatched primary address")
	}
}

func TestNormalizeAdminStatusTrimsAndLowers(t *testing.T) {
	t.Parallel()

	if got := normalizeAdminStatus(" Suspended "); got != "suspended" {
		t.Fatalf("status = %q, want suspended", got)
	}
}
