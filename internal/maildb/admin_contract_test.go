package maildb

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
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

func TestValidateDomainListRequestRejectsUnknownFilters(t *testing.T) {
	t.Parallel()

	tests := []DomainListRequest{
		{Status: "archived"},
		{DNSStatus: "stale"},
		{CompanyID: "company\nbad"},
	}
	for _, req := range tests {
		req := req
		t.Run(req.Status+req.DNSStatus+req.CompanyID, func(t *testing.T) {
			t.Parallel()
			if err := ValidateDomainListRequest(req); err == nil {
				t.Fatalf("ValidateDomainListRequest accepted %+v", req)
			}
		})
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

func TestValidateCompanyListRequestRejectsUnknownStatus(t *testing.T) {
	t.Parallel()

	if err := ValidateCompanyListRequest(CompanyListRequest{Status: "archived"}); err == nil {
		t.Fatal("ValidateCompanyListRequest accepted unknown status")
	}
}

func TestValidateCompanyListRequestAcceptsLifecycleStatus(t *testing.T) {
	t.Parallel()

	if err := ValidateCompanyListRequest(CompanyListRequest{Status: " suspended "}); err != nil {
		t.Fatalf("ValidateCompanyListRequest returned error: %v", err)
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

func TestNormalizeDeliveryAttemptFiltersRejectsUnsafeValues(t *testing.T) {
	t.Parallel()

	tests := []deliveryAttemptFilters{
		{Status: "retrying"},
		{RecipientDomain: "example.net\nbad"},
		{MessageID: strings.Repeat("m", maxPushNotificationFilterBytes+1)},
		{Farm: "general\rbad"},
		{Sender: "sender@example.com\nbad"},
	}
	for _, filters := range tests {
		filters := filters
		t.Run(filters.Status+filters.RecipientDomain+filters.MessageID+filters.Farm+filters.Sender, func(t *testing.T) {
			t.Parallel()
			if _, err := normalizeDeliveryAttemptFilters(filters); err == nil {
				t.Fatalf("normalizeDeliveryAttemptFilters accepted %+v", filters)
			}
		})
	}
}

func TestNormalizeDeliveryAttemptFiltersTrimsOperationalFilters(t *testing.T) {
	t.Parallel()

	got, err := normalizeDeliveryAttemptFilters(deliveryAttemptFilters{
		Status:          " BOUNCED ",
		RecipientDomain: " Example.NET. ",
		MessageID:       " msg-1 ",
		Farm:            " General ",
		Sender:          " Sender@Example.COM ",
	})
	if err != nil {
		t.Fatalf("normalizeDeliveryAttemptFilters returned error: %v", err)
	}
	if got.Status != "bounced" ||
		got.RecipientDomain != "example.net" ||
		got.MessageID != "msg-1" ||
		got.Farm != "general" ||
		got.Sender != "sender@example.com" {
		t.Fatalf("filters = %+v, want normalized filters", got)
	}
}

func TestNormalizeAPIUsageAggregateListRequestRejectsUnsafeFilters(t *testing.T) {
	t.Parallel()

	tests := []APIUsageAggregateListRequest{
		{TenantID: "tenant\nbad"},
		{CompanyID: strings.Repeat("c", maxPushNotificationFilterBytes+1)},
		{Route: "GET /api/v1/messages\rbad"},
		{Status: -1},
		{Status: 1000},
		{From: time.Unix(2, 0), To: time.Unix(1, 0)},
	}
	for _, req := range tests {
		req := req
		t.Run(req.TenantID+req.CompanyID+req.Route, func(t *testing.T) {
			t.Parallel()
			if _, err := normalizeAPIUsageAggregateListRequest(req); err == nil {
				t.Fatalf("normalizeAPIUsageAggregateListRequest accepted %+v", req)
			}
		})
	}
}

func TestNormalizeAPIUsageAggregateListRequestNormalizesFilters(t *testing.T) {
	t.Parallel()

	got, err := normalizeAPIUsageAggregateListRequest(APIUsageAggregateListRequest{
		Limit:       5,
		TenantID:    " tenant-1 ",
		PrincipalID: " principal-1 ",
		AuthSource:  " Bearer ",
		Method:      " get ",
		Route:       " GET /api/v1/messages ",
		Status:      200,
	})
	if err != nil {
		t.Fatalf("normalizeAPIUsageAggregateListRequest returned error: %v", err)
	}
	if got.Limit != 5 ||
		got.TenantID != "tenant-1" ||
		got.PrincipalID != "principal-1" ||
		got.AuthSource != "bearer" ||
		got.Method != "get" ||
		got.Route != "GET /api/v1/messages" ||
		got.Status != 200 {
		t.Fatalf("request = %+v, want normalized filters", got)
	}
}

func TestQuotaCorrectionAuditDetailIsBounded(t *testing.T) {
	t.Parallel()

	var before []QuotaReconciliationView
	for i := 0; i < 25; i++ {
		before = append(before, QuotaReconciliationView{
			Scope:      "user",
			ID:         "user-1",
			DomainID:   "domain-1",
			Name:       "user@example.com",
			LedgerUsed: int64(100 + i),
			ActualUsed: 50,
			Delta:      int64(50 + i),
		})
	}
	raw, err := quotaCorrectionAuditDetail(CorrectQuotaReconciliationRequest{
		Scope:  "domain",
		ID:     "domain-1",
		DryRun: true,
	}, before, nil)
	if err != nil {
		t.Fatalf("quotaCorrectionAuditDetail returned error: %v", err)
	}
	var detail struct {
		Scope             string                     `json:"scope"`
		ID                string                     `json:"id"`
		DryRun            bool                       `json:"dry_run"`
		BeforeDriftCount  int                        `json:"before_drift_count"`
		AfterDriftCount   int                        `json:"after_drift_count"`
		BeforeAbsDeltaSum int64                      `json:"before_abs_delta_sum"`
		SampleLimit       int                        `json:"sample_limit"`
		BeforeSample      []quotaCorrectionAuditView `json:"before_sample"`
	}
	if err := json.Unmarshal(raw, &detail); err != nil {
		t.Fatalf("unmarshal audit detail: %v", err)
	}
	if detail.Scope != "domain" || detail.ID != "domain-1" || !detail.DryRun {
		t.Fatalf("audit detail identity = %+v", detail)
	}
	if detail.BeforeDriftCount != 25 || detail.AfterDriftCount != 0 || detail.SampleLimit != 20 || len(detail.BeforeSample) != 20 {
		t.Fatalf("audit detail bounds = %+v", detail)
	}
	if detail.BeforeAbsDeltaSum == 0 {
		t.Fatalf("before_abs_delta_sum = %d, want positive", detail.BeforeAbsDeltaSum)
	}
}

func TestNormalizePushNotificationAttemptListRequestRejectsUnsafeFilters(t *testing.T) {
	t.Parallel()

	tests := []PushNotificationAttemptListRequest{
		{MessageID: "message-1\nbad"},
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
		MessageID:         " message-1 ",
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
	if got.Limit <= 0 || got.MessageID != "message-1" || got.Status != "queued" || got.UserID != "user-1" || got.Platform != "fcm" || got.DeviceID != "device-1" || got.ProviderStatus != "accepted" || got.ProviderMessageID != "provider-message-1" {
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

func TestNormalizePushNotificationStatsRequest(t *testing.T) {
	t.Parallel()

	req, err := normalizePushNotificationStatsRequest(PushNotificationStatsRequest{
		MessageID: " message-1 ",
		UserID:    " user-1 ",
		Platform:  " FCM ",
		DeviceID:  " device-1 ",
	})
	if err != nil {
		t.Fatalf("normalizePushNotificationStatsRequest returned error: %v", err)
	}
	if req.MessageID != "message-1" || req.UserID != "user-1" || req.Platform != "fcm" || req.DeviceID != "device-1" {
		t.Fatalf("normalized request = %+v", req)
	}
}

func TestNormalizePushNotificationStatsRequestRejectsUnsafeFilters(t *testing.T) {
	t.Parallel()

	for _, req := range []PushNotificationStatsRequest{
		{MessageID: "message-1\nbad"},
		{MessageID: strings.Repeat("m", maxPushNotificationFilterBytes+1)},
		{MessageID: string([]byte{0xff})},
		{UserID: "user-1\nbad"},
		{UserID: strings.Repeat("u", maxPushNotificationFilterBytes+1)},
		{UserID: string([]byte{0xff})},
		{Platform: "pager"},
		{Platform: "fcm\nbad"},
		{DeviceID: "device-1\nbad"},
		{DeviceID: strings.Repeat("d", maxPushNotificationFilterBytes+1)},
	} {
		req := req
		t.Run(req.MessageID+req.UserID, func(t *testing.T) {
			t.Parallel()
			if _, err := normalizePushNotificationStatsRequest(req); err == nil {
				t.Fatalf("normalizePushNotificationStatsRequest accepted %+v", req)
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

func TestValidateUserListRequestRejectsUnknownStatus(t *testing.T) {
	t.Parallel()

	if err := ValidateUserListRequest(UserListRequest{Status: "paused"}); err == nil {
		t.Fatal("ValidateUserListRequest accepted unknown status")
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

func TestValidateCreateUserRequestAcceptsPasswordHash(t *testing.T) {
	t.Parallel()

	err := ValidateCreateUserRequest(CreateUserRequest{
		DomainID:     "domain-1",
		Username:     "admin",
		DisplayName:  "Admin",
		Address:      "admin@example.com",
		PasswordHash: "plain:dev-password",
	})
	if err != nil {
		t.Fatalf("ValidateCreateUserRequest returned error: %v", err)
	}
}

func TestValidateCreateUserRequestRejectsUnsafePasswordHash(t *testing.T) {
	t.Parallel()

	err := ValidateCreateUserRequest(CreateUserRequest{
		DomainID:     "domain-1",
		Username:     "admin",
		DisplayName:  "Admin",
		Address:      "admin@example.com",
		PasswordHash: "plain:dev\nbad",
	})
	if err == nil {
		t.Fatal("ValidateCreateUserRequest accepted unsafe password hash")
	}
}

func TestValidateUpdateUserPasswordHashRequest(t *testing.T) {
	t.Parallel()

	if err := ValidateUpdateUserPasswordHashRequest(UpdateUserPasswordHashRequest{
		ID:           "user-1",
		PasswordHash: "plain:dev-password",
	}); err != nil {
		t.Fatalf("ValidateUpdateUserPasswordHashRequest returned error: %v", err)
	}

	tests := []UpdateUserPasswordHashRequest{
		{ID: "", PasswordHash: "plain:dev-password"},
		{ID: "user-1", PasswordHash: ""},
		{ID: "user-1", PasswordHash: "plain:dev\nbad"},
	}
	for _, req := range tests {
		req := req
		t.Run(req.ID+"/"+req.PasswordHash, func(t *testing.T) {
			t.Parallel()

			if err := ValidateUpdateUserPasswordHashRequest(req); err == nil {
				t.Fatalf("ValidateUpdateUserPasswordHashRequest accepted %+v", req)
			}
		})
	}
}

func TestNormalizeAdminStatusTrimsAndLowers(t *testing.T) {
	t.Parallel()

	if got := normalizeAdminStatus(" Suspended "); got != "suspended" {
		t.Fatalf("status = %q, want suspended", got)
	}
}

func TestValidateDeliveryRouteListRequestRejectsUnknownFilters(t *testing.T) {
	t.Parallel()

	tests := []DeliveryRouteListRequest{
		{Status: "paused"},
		{Farm: "pool\nbad"},
		{DomainPattern: strings.Repeat("d", maxPushNotificationFilterBytes+1)},
	}
	for _, req := range tests {
		req := req
		t.Run(req.Status+req.Farm+req.DomainPattern, func(t *testing.T) {
			t.Parallel()
			if err := ValidateDeliveryRouteListRequest(req); err == nil {
				t.Fatalf("ValidateDeliveryRouteListRequest accepted %+v", req)
			}
		})
	}
}
