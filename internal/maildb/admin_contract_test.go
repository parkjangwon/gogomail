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

func TestDomainStatusAuditDetail(t *testing.T) {
	t.Parallel()

	detail, err := domainStatusAuditDetail(domainStatusAuditView{
		ID:        "domain-1",
		CompanyID: "company-1",
		Name:      "example.com",
		Status:    "suspended",
	})
	if err != nil {
		t.Fatalf("domainStatusAuditDetail returned error: %v", err)
	}
	var body map[string]string
	if err := json.Unmarshal(detail, &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if body["domain_id"] != "domain-1" ||
		body["company_id"] != "company-1" ||
		body["name"] != "example.com" ||
		body["status"] != "suspended" {
		t.Fatalf("detail = %+v", body)
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

func TestQuotaAuditDetails(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		detail json.RawMessage
		want   map[string]any
	}{
		{
			name: "company",
			detail: mustAuditDetail(t, func() (json.RawMessage, error) {
				return companyQuotaAuditDetail(companyQuotaAuditView{
					ID:         "company-1",
					Name:       "Acme",
					Status:     "active",
					QuotaLimit: 1024,
				})
			}),
			want: map[string]any{
				"company_id":  "company-1",
				"name":        "Acme",
				"status":      "active",
				"quota_limit": float64(1024),
			},
		},
		{
			name: "domain",
			detail: mustAuditDetail(t, func() (json.RawMessage, error) {
				return domainQuotaAuditDetail(domainQuotaAuditView{
					ID:                          "domain-1",
					CompanyID:                   "company-1",
					Name:                        "example.com",
					QuotaLimit:                  512,
					DefaultUserQuotaSet:         true,
					DefaultUserQuota:            128,
					DefaultUserQuotaUserUpdates: 7,
				})
			}),
			want: map[string]any{
				"domain_id":                       "domain-1",
				"company_id":                      "company-1",
				"name":                            "example.com",
				"quota_limit":                     float64(512),
				"default_user_quota_set":          true,
				"default_user_quota":              float64(128),
				"default_user_quota_user_updates": float64(7),
			},
		},
		{
			name: "user",
			detail: mustAuditDetail(t, func() (json.RawMessage, error) {
				return userQuotaAuditDetail(userQuotaAuditView{
					ID:          "user-1",
					DomainID:    "domain-1",
					CompanyID:   "company-1",
					Username:    "alex",
					QuotaLimit:  64,
					QuotaSource: "custom",
				})
			}),
			want: map[string]any{
				"user_id":      "user-1",
				"domain_id":    "domain-1",
				"company_id":   "company-1",
				"username":     "alex",
				"quota_limit":  float64(64),
				"quota_source": "custom",
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var body map[string]any
			if err := json.Unmarshal(tt.detail, &body); err != nil {
				t.Fatalf("json.Unmarshal returned error: %v", err)
			}
			for key, want := range tt.want {
				if body[key] != want {
					t.Fatalf("%s detail[%q] = %#v, want %#v; detail=%+v", tt.name, key, body[key], want, body)
				}
			}
		})
	}
}

func mustAuditDetail(t *testing.T, build func() (json.RawMessage, error)) json.RawMessage {
	t.Helper()
	detail, err := build()
	if err != nil {
		t.Fatalf("audit detail returned error: %v", err)
	}
	return detail
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

func TestValidateAPIUsageExportBatchListRequestRejectsUnsafeFilters(t *testing.T) {
	t.Parallel()

	tests := []APIUsageExportBatchListRequest{
		{TenantID: "tenant\nbad"},
		{PrincipalID: strings.Repeat("p", maxPushNotificationFilterBytes+1)},
		{Status: "ready"},
		{From: time.Date(2026, 5, 5, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC)},
	}
	for _, req := range tests {
		req := req
		t.Run(req.TenantID+req.PrincipalID+req.Status, func(t *testing.T) {
			t.Parallel()
			if err := ValidateAPIUsageExportBatchListRequest(req); err == nil {
				t.Fatalf("ValidateAPIUsageExportBatchListRequest accepted %+v", req)
			}
		})
	}
}

func TestNormalizeQuotaUsageListRequestRejectsUnsafeFilters(t *testing.T) {
	t.Parallel()

	tests := []QuotaUsageListRequest{
		{Scope: "mailbox"},
		{Scope: "domain\nbad"},
		{DomainID: strings.Repeat("d", maxPushNotificationFilterBytes+1)},
	}
	for _, req := range tests {
		req := req
		t.Run(req.Scope+req.DomainID, func(t *testing.T) {
			t.Parallel()
			if _, err := normalizeQuotaUsageListRequest(req); err == nil {
				t.Fatalf("normalizeQuotaUsageListRequest accepted %+v", req)
			}
		})
	}
}

func TestNormalizeQuotaUsageListRequestNormalizesFilters(t *testing.T) {
	t.Parallel()

	overLimit := true
	overAllocated := false
	got, err := normalizeQuotaUsageListRequest(QuotaUsageListRequest{
		Limit:         5,
		Scope:         " Domain ",
		DomainID:      " domain-1 ",
		OverLimit:     &overLimit,
		OverAllocated: &overAllocated,
	})
	if err != nil {
		t.Fatalf("normalizeQuotaUsageListRequest returned error: %v", err)
	}
	if got.Limit != 5 ||
		got.Scope != "domain" ||
		got.DomainID != "domain-1" ||
		got.OverLimit == nil ||
		!*got.OverLimit ||
		got.OverAllocated == nil ||
		*got.OverAllocated {
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

func TestDomainPolicyAuditDetail(t *testing.T) {
	t.Parallel()

	detail, err := domainPolicyAuditDetail(domainPolicyAuditView{
		ID:        "domain-1",
		CompanyID: "company-1",
		Name:      "example.com",
		Policy: DomainPolicyView{
			InboundMode:             "monitor",
			OutboundMode:            "enforce",
			MaxRecipientsPerMessage: 100,
			MaxMessageBytes:         1024,
			MaxAttachmentBytes:      512,
		},
	})
	if err != nil {
		t.Fatalf("domainPolicyAuditDetail returned error: %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal(detail, &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	for key, want := range map[string]any{
		"domain_id":                  "domain-1",
		"company_id":                 "company-1",
		"name":                       "example.com",
		"inbound_mode":               "monitor",
		"outbound_mode":              "enforce",
		"max_recipients_per_message": float64(100),
		"max_message_bytes":          float64(1024),
		"max_attachment_bytes":       float64(512),
	} {
		if body[key] != want {
			t.Fatalf("detail[%q] = %#v, want %#v; detail=%+v", key, body[key], want, body)
		}
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

func TestSuppressionEntryAuditDetail(t *testing.T) {
	t.Parallel()

	detail, err := suppressionEntryAuditDetail(SuppressionEntry{
		ID:              "11111111-1111-1111-1111-111111111111",
		DomainID:        "22222222-2222-2222-2222-222222222222",
		Email:           "user@example.net",
		Reason:          "hard_bounce",
		SourceMessageID: "33333333-3333-3333-3333-333333333333",
	})
	if err != nil {
		t.Fatalf("suppressionEntryAuditDetail returned error: %v", err)
	}
	var got struct {
		ID              string `json:"suppression_entry_id"`
		DomainID        string `json:"domain_id"`
		Email           string `json:"email"`
		Reason          string `json:"reason"`
		SourceMessageID string `json:"source_message_id"`
	}
	if err := json.Unmarshal(detail, &got); err != nil {
		t.Fatalf("unmarshal audit detail: %v", err)
	}
	if got.ID == "" || got.DomainID == "" || got.Email != "user@example.net" || got.Reason != "hard_bounce" || got.SourceMessageID == "" {
		t.Fatalf("audit detail = %+v", got)
	}
}

func TestOutboxRetryAuditDetail(t *testing.T) {
	t.Parallel()

	detail, err := outboxRetryAuditDetail(OutboxEventView{
		ID:           "11111111-1111-1111-1111-111111111111",
		Topic:        "mail.bounced",
		PartitionKey: "message-1",
		Status:       "failed",
		Attempts:     3,
		LastError:    strings.Repeat("x", outboxEventListErrorPreviewBytes+10),
	})
	if err != nil {
		t.Fatalf("outboxRetryAuditDetail returned error: %v", err)
	}
	var got struct {
		ID                string `json:"outbox_event_id"`
		Topic             string `json:"topic"`
		PartitionKey      string `json:"partition_key"`
		PreviousStatus    string `json:"previous_status"`
		PreviousAttempts  int    `json:"previous_attempts"`
		PreviousLastError string `json:"previous_last_error"`
	}
	if err := json.Unmarshal(detail, &got); err != nil {
		t.Fatalf("unmarshal audit detail: %v", err)
	}
	if got.ID == "" || got.Topic != "mail.bounced" || got.PreviousStatus != "failed" || got.PreviousAttempts != 3 {
		t.Fatalf("audit detail = %+v", got)
	}
	if len(got.PreviousLastError) != outboxEventListErrorPreviewBytes {
		t.Fatalf("previous_last_error length = %d, want %d", len(got.PreviousLastError), outboxEventListErrorPreviewBytes)
	}
}

func TestDomainCreateAuditDetail(t *testing.T) {
	t.Parallel()

	detail, err := domainCreateAuditDetail(DomainView{
		ID:         "domain-1",
		CompanyID:  "company-1",
		Name:       "example.com",
		NameACE:    "example.com",
		Status:     "active",
		QuotaLimit: 1024,
	})
	if err != nil {
		t.Fatalf("domainCreateAuditDetail returned error: %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal(detail, &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	for key, want := range map[string]any{
		"domain_id":   "domain-1",
		"company_id":  "company-1",
		"name":        "example.com",
		"name_ace":    "example.com",
		"status":      "active",
		"quota_limit": float64(1024),
	} {
		if body[key] != want {
			t.Fatalf("detail[%q] = %#v, want %#v; detail=%+v", key, body[key], want, body)
		}
	}
}

func TestValidateUpdateUserStatusRequestRejectsUnknownStatus(t *testing.T) {
	t.Parallel()

	if err := ValidateUpdateUserStatusRequest(UpdateUserStatusRequest{ID: "user-1", Status: "paused"}); err == nil {
		t.Fatal("ValidateUpdateUserStatusRequest accepted unknown status")
	}
}

func TestUserStatusAuditDetail(t *testing.T) {
	t.Parallel()

	detail, err := userStatusAuditDetail(userStatusAuditView{
		ID:        "user-1",
		DomainID:  "domain-1",
		CompanyID: "company-1",
		Username:  "alex",
		Status:    "disabled",
	})
	if err != nil {
		t.Fatalf("userStatusAuditDetail returned error: %v", err)
	}
	var body map[string]string
	if err := json.Unmarshal(detail, &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if body["user_id"] != "user-1" ||
		body["domain_id"] != "domain-1" ||
		body["company_id"] != "company-1" ||
		body["username"] != "alex" ||
		body["status"] != "disabled" {
		t.Fatalf("detail = %+v", body)
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

func TestUserCreateAuditDetailDoesNotIncludePasswordHash(t *testing.T) {
	t.Parallel()

	detail, err := userCreateAuditDetail(userCreateAuditView{
		User: UserView{
			ID:                 "user-1",
			DomainID:           "domain-1",
			Username:           "alex",
			DisplayName:        "Alex",
			Role:               "user",
			Status:             "active",
			PasswordConfigured: true,
			QuotaLimit:         512,
			QuotaSource:        "custom",
		},
		CompanyID: "company-1",
		Address:   "alex@example.com",
	})
	if err != nil {
		t.Fatalf("userCreateAuditDetail returned error: %v", err)
	}
	if strings.Contains(string(detail), "plain:") || strings.Contains(string(detail), "password_hash") {
		t.Fatalf("audit detail leaked password hash material: %s", detail)
	}
	var body map[string]any
	if err := json.Unmarshal(detail, &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	for key, want := range map[string]any{
		"user_id":             "user-1",
		"domain_id":           "domain-1",
		"company_id":          "company-1",
		"username":            "alex",
		"display_name":        "Alex",
		"address":             "alex@example.com",
		"role":                "user",
		"status":              "active",
		"password_configured": true,
		"quota_limit":         float64(512),
		"quota_source":        "custom",
	} {
		if body[key] != want {
			t.Fatalf("detail[%q] = %#v, want %#v; detail=%+v", key, body[key], want, body)
		}
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

func TestUserCredentialAuditDetailDoesNotIncludePasswordHash(t *testing.T) {
	t.Parallel()

	detail, err := userCredentialAuditDetail(userCredentialAuditView{
		ID:                 "user-1",
		DomainID:           "domain-1",
		CompanyID:          "company-1",
		Username:           "alex",
		PasswordConfigured: true,
	})
	if err != nil {
		t.Fatalf("userCredentialAuditDetail returned error: %v", err)
	}
	if strings.Contains(string(detail), "plain:") || strings.Contains(string(detail), "password_hash") {
		t.Fatalf("audit detail leaked password hash material: %s", detail)
	}
	var body map[string]any
	if err := json.Unmarshal(detail, &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	for key, want := range map[string]any{
		"user_id":             "user-1",
		"domain_id":           "domain-1",
		"company_id":          "company-1",
		"username":            "alex",
		"password_configured": true,
	} {
		if body[key] != want {
			t.Fatalf("detail[%q] = %#v, want %#v; detail=%+v", key, body[key], want, body)
		}
	}
}

func TestNormalizeAdminStatusTrimsAndLowers(t *testing.T) {
	t.Parallel()

	if got := normalizeAdminStatus(" Suspended "); got != "suspended" {
		t.Fatalf("status = %q, want suspended", got)
	}
}

func TestValidateDomainDNSCheckListRequestRejectsUnsafeFilters(t *testing.T) {
	t.Parallel()

	tests := []DomainDNSCheckListRequest{
		{DomainID: "", Status: "ok"},
		{DomainID: "domain\nbad", Status: "ok"},
		{DomainID: "domain-1", Status: "pass"},
		{DomainID: "domain-1", Status: "missing\nbad"},
	}
	for _, req := range tests {
		req := req
		t.Run(req.DomainID+req.Status, func(t *testing.T) {
			t.Parallel()
			if err := ValidateDomainDNSCheckListRequest(req); err == nil {
				t.Fatalf("ValidateDomainDNSCheckListRequest accepted %+v", req)
			}
		})
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

func TestValidateTrustedRelayListRequestRejectsUnsafeFilters(t *testing.T) {
	t.Parallel()

	tests := []TrustedRelayListRequest{
		{CIDR: "not-a-cidr"},
		{Description: "edge\nbad"},
		{Description: strings.Repeat("d", maxPushNotificationFilterBytes+1)},
	}
	for _, req := range tests {
		req := req
		t.Run(req.CIDR+req.Description, func(t *testing.T) {
			t.Parallel()
			if err := ValidateTrustedRelayListRequest(req); err == nil {
				t.Fatalf("ValidateTrustedRelayListRequest accepted %+v", req)
			}
		})
	}
}
