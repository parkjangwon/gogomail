package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/admin"
	"github.com/gogomail/gogomail/internal/auth"
	"github.com/gogomail/gogomail/internal/backpressure"
	"github.com/gogomail/gogomail/internal/configstore"
	"github.com/gogomail/gogomail/internal/davsyncretention"
	"github.com/gogomail/gogomail/internal/directory"
	"github.com/gogomail/gogomail/internal/dnscheck"
	"github.com/gogomail/gogomail/internal/drive"
	"github.com/gogomail/gogomail/internal/idprovider"
	ldapidp "github.com/gogomail/gogomail/internal/idprovider/ldap"
	rdbmsidp "github.com/gogomail/gogomail/internal/idprovider/rdbms"
	"github.com/gogomail/gogomail/internal/imapgw"
	"github.com/gogomail/gogomail/internal/maildb"
	"github.com/gogomail/gogomail/internal/storage"
	"github.com/google/uuid"
)

func TestAdminQueueHandler(t *testing.T) {
	t.Parallel()

	oldestReadyAt := time.Date(2026, 5, 4, 8, 0, 0, 0, time.UTC)
	service := &fakeAdminService{
		queueStats: []maildb.QueueStat{{
			Topic:         "mail.outbound.general",
			Status:        "pending",
			Count:         2,
			ReadyCount:    1,
			DelayedCount:  1,
			OldestReadyAt: &oldestReadyAt,
		}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/queue", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Queues []maildb.QueueStat `json:"queues"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if len(body.Queues) != 1 || body.Queues[0].Count != 2 || body.Queues[0].ReadyCount != 1 || body.Queues[0].DelayedCount != 1 || body.Queues[0].OldestReadyAt == nil {
		t.Fatalf("queues = %+v", body.Queues)
	}
}

func TestHandleListAlertEventsReturnsPaginationMetadata(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		alertEvents: []admin.AlertEvent{
			{ID: "event-1", CompanyID: "company-1", Message: "first"},
			{ID: "event-2", CompanyID: "company-1", Message: "second"},
			{ID: "event-3", CompanyID: "company-2", Message: "other"},
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/admin/v1/companies/company-1/alert-events?limit=1&offset=2&alert_rule_id=rule-1&unresolved=true", nil)
	req.SetPathValue("id", "company-1")
	rec := httptest.NewRecorder()

	handleListAlertEvents(rec, req, service)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Events  []admin.AlertEvent `json:"events"`
		Limit   int                `json:"limit"`
		Offset  int                `json:"offset"`
		HasMore bool               `json:"has_more"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Limit != 1 || body.Offset != 2 || !body.HasMore || len(body.Events) != 1 {
		t.Fatalf("pagination body = %+v", body)
	}
	filter := service.lastListAlertEventsFilter
	if filter.CompanyID != "company-1" || filter.AlertRuleID != "rule-1" || !filter.OnlyUnresolved || filter.Limit != 1 || filter.Offset != 2 {
		t.Fatalf("filter = %+v", filter)
	}
}

func TestLDAPSyncHistoryReturnsPaginationMetadata(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		ldapSyncRuns: []maildb.LDAPSyncRunView{
			{SyncType: "users", Status: "success"},
			{SyncType: "groups", Status: "success"},
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/admin/v1/domains/domain-1/ldap/sync-history?limit=1&offset=5", nil)
	req.SetPathValue("id", "domain-1")
	rec := httptest.NewRecorder()

	handleLDAPSyncHistory(rec, req, service)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Runs    []maildb.LDAPSyncRunView `json:"sync_runs"`
		Limit   int                      `json:"limit"`
		Offset  int                      `json:"offset"`
		HasMore bool                     `json:"has_more"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Limit != 1 || body.Offset != 5 || !body.HasMore || len(body.Runs) != 1 {
		t.Fatalf("body = %+v", body)
	}
	if service.lastLDAPSyncRunsReq.Limit != 2 || service.lastLDAPSyncRunsReq.Offset != 5 {
		t.Fatalf("request = %+v, want limit+1 probe", service.lastLDAPSyncRunsReq)
	}
}

func TestLDAPSyncConflictsReturnsCursorPaginationMetadata(t *testing.T) {
	t.Parallel()

	firstID := uuid.MustParse("77777777-7777-7777-7777-777777777777")
	secondID := uuid.MustParse("88888888-8888-8888-8888-888888888888")
	cursorID := uuid.MustParse("99999999-9999-9999-9999-999999999999")
	syncRunID := uuid.MustParse("aaaaaaaa-1111-1111-1111-aaaaaaaaaaaa")
	cursorTime := time.Date(2026, 5, 21, 12, 45, 0, 0, time.UTC)
	cursor, err := maildb.EncodeLDAPSyncConflictCursor(maildb.LDAPSyncConflictView{ID: cursorID, CreatedAt: cursorTime})
	if err != nil {
		t.Fatalf("EncodeLDAPSyncConflictCursor returned error: %v", err)
	}
	service := &fakeAdminService{
		ldapSyncConflicts: []maildb.LDAPSyncConflictView{
			{ID: firstID, SyncRunID: syncRunID, ConflictType: "duplicate_key", CreatedAt: cursorTime.Add(-time.Minute)},
			{ID: secondID, SyncRunID: syncRunID, ConflictType: "attr_mismatch", CreatedAt: cursorTime.Add(-2 * time.Minute)},
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/admin/v1/domains/domain-1/ldap/conflicts?limit=1&cursor="+cursor+"&sync_run_id="+syncRunID.String()+"&unresolved_only=true", nil)
	req.SetPathValue("id", "domain-1")
	rec := httptest.NewRecorder()

	handleLDAPSyncConflicts(rec, req, service)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Conflicts  []maildb.LDAPSyncConflictView `json:"conflicts"`
		Limit      int                           `json:"limit"`
		Offset     int                           `json:"offset"`
		HasMore    bool                          `json:"has_more"`
		NextCursor string                        `json:"next_cursor"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Limit != 1 || body.Offset != 0 || !body.HasMore || len(body.Conflicts) != 1 || body.NextCursor == "" {
		t.Fatalf("body = %+v", body)
	}
	reqSeen := service.lastLDAPSyncConflictsReq
	if reqSeen.Limit != 2 || reqSeen.Offset != 0 || reqSeen.SyncRunID != syncRunID.String() || !reqSeen.UnresolvedOnly {
		t.Fatalf("request = %+v, want limit+1 filtered cursor probe", reqSeen)
	}
	if reqSeen.Cursor.ID != cursorID || !reqSeen.Cursor.CreatedAt.Equal(cursorTime) {
		t.Fatalf("cursor request = %+v", reqSeen.Cursor)
	}
	next, err := maildb.DecodeLDAPSyncConflictCursor(body.NextCursor)
	if err != nil {
		t.Fatalf("DecodeLDAPSyncConflictCursor(next) returned error: %v", err)
	}
	if next.ID != firstID {
		t.Fatalf("next cursor ID = %s, want %s", next.ID, firstID)
	}
}

func TestRDBMSSyncHistoryReturnsCursorPaginationMetadata(t *testing.T) {
	t.Parallel()

	firstID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	secondID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	cursorID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	cursorTime := time.Date(2026, 5, 21, 9, 0, 0, 0, time.UTC)
	cursor, err := maildb.EncodeRDBMSSyncRunCursor(maildb.RDBMSSyncRunView{ID: cursorID, CreatedAt: cursorTime})
	if err != nil {
		t.Fatalf("EncodeRDBMSSyncRunCursor returned error: %v", err)
	}
	service := &fakeAdminService{
		rdbmsSyncRuns: []maildb.RDBMSSyncRunView{
			{ID: firstID, SyncType: "users", Status: "success", CreatedAt: cursorTime.Add(-time.Minute)},
			{ID: secondID, SyncType: "groups", Status: "success", CreatedAt: cursorTime.Add(-2 * time.Minute)},
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/admin/v1/domains/domain-1/rdbms/sync-history?limit=1&cursor="+cursor, nil)
	req.SetPathValue("id", "domain-1")
	rec := httptest.NewRecorder()

	handleRDBMSSyncHistory(rec, req, service)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Runs       []maildb.RDBMSSyncRunView `json:"sync_runs"`
		Limit      int                       `json:"limit"`
		Offset     int                       `json:"offset"`
		HasMore    bool                      `json:"has_more"`
		NextCursor string                    `json:"next_cursor"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Limit != 1 || body.Offset != 0 || !body.HasMore || len(body.Runs) != 1 || body.NextCursor == "" {
		t.Fatalf("body = %+v", body)
	}
	if service.lastRDBMSSyncRunsReq.Limit != 2 || service.lastRDBMSSyncRunsReq.Offset != 0 {
		t.Fatalf("request = %+v, want limit+1 cursor probe", service.lastRDBMSSyncRunsReq)
	}
	if service.lastRDBMSSyncRunsReq.Cursor.ID != cursorID || !service.lastRDBMSSyncRunsReq.Cursor.CreatedAt.Equal(cursorTime) {
		t.Fatalf("cursor request = %+v", service.lastRDBMSSyncRunsReq.Cursor)
	}
	next, err := maildb.DecodeRDBMSSyncRunCursor(body.NextCursor)
	if err != nil {
		t.Fatalf("DecodeRDBMSSyncRunCursor(next) returned error: %v", err)
	}
	if next.ID != firstID {
		t.Fatalf("next cursor ID = %s, want %s", next.ID, firstID)
	}
}

func TestRDBMSSyncConflictsReturnsPaginationMetadata(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 21, 10, 30, 0, 0, time.UTC)
	service := &fakeAdminService{
		rdbmsSyncConflicts: []maildb.RDBMSSyncConflictView{
			{ID: uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"), ConflictType: "duplicate_key", CreatedAt: now},
			{ID: uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"), ConflictType: "schema_mismatch", CreatedAt: now.Add(-time.Minute)},
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/admin/v1/domains/domain-1/rdbms/conflicts?limit=1&offset=3&unresolved_only=true", nil)
	req.SetPathValue("id", "domain-1")
	rec := httptest.NewRecorder()

	handleRDBMSSyncConflicts(rec, req, service)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Conflicts []maildb.RDBMSSyncConflictView `json:"conflicts"`
		Limit     int                            `json:"limit"`
		Offset    int                            `json:"offset"`
		HasMore   bool                           `json:"has_more"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Limit != 1 || body.Offset != 3 || !body.HasMore || len(body.Conflicts) != 1 {
		t.Fatalf("body = %+v", body)
	}
	reqSeen := service.lastRDBMSSyncConflictsReq
	if reqSeen.Limit != 2 || reqSeen.Offset != 3 || !reqSeen.UnresolvedOnly {
		t.Fatalf("request = %+v, want limit+1 unresolved probe", reqSeen)
	}
}

func TestRDBMSSyncConflictsReturnsCursorPaginationMetadata(t *testing.T) {
	t.Parallel()

	firstID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	secondID := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	cursorID := uuid.MustParse("66666666-6666-6666-6666-666666666666")
	cursorTime := time.Date(2026, 5, 21, 11, 0, 0, 0, time.UTC)
	cursor, err := maildb.EncodeRDBMSSyncConflictCursor(maildb.RDBMSSyncConflictView{ID: cursorID, CreatedAt: cursorTime})
	if err != nil {
		t.Fatalf("EncodeRDBMSSyncConflictCursor returned error: %v", err)
	}
	service := &fakeAdminService{
		rdbmsSyncConflicts: []maildb.RDBMSSyncConflictView{
			{ID: firstID, ConflictType: "duplicate_key", CreatedAt: cursorTime.Add(-time.Minute)},
			{ID: secondID, ConflictType: "schema_mismatch", CreatedAt: cursorTime.Add(-2 * time.Minute)},
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/admin/v1/domains/domain-1/rdbms/conflicts?limit=1&cursor="+cursor+"&unresolved_only=true", nil)
	req.SetPathValue("id", "domain-1")
	rec := httptest.NewRecorder()

	handleRDBMSSyncConflicts(rec, req, service)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Conflicts  []maildb.RDBMSSyncConflictView `json:"conflicts"`
		Limit      int                            `json:"limit"`
		Offset     int                            `json:"offset"`
		HasMore    bool                           `json:"has_more"`
		NextCursor string                         `json:"next_cursor"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Limit != 1 || body.Offset != 0 || !body.HasMore || len(body.Conflicts) != 1 || body.NextCursor == "" {
		t.Fatalf("body = %+v", body)
	}
	reqSeen := service.lastRDBMSSyncConflictsReq
	if reqSeen.Limit != 2 || reqSeen.Offset != 0 || !reqSeen.UnresolvedOnly {
		t.Fatalf("request = %+v, want limit+1 unresolved cursor probe", reqSeen)
	}
	if reqSeen.Cursor.ID != cursorID || !reqSeen.Cursor.CreatedAt.Equal(cursorTime) {
		t.Fatalf("cursor request = %+v", reqSeen.Cursor)
	}
	next, err := maildb.DecodeRDBMSSyncConflictCursor(body.NextCursor)
	if err != nil {
		t.Fatalf("DecodeRDBMSSyncConflictCursor(next) returned error: %v", err)
	}
	if next.ID != firstID {
		t.Fatalf("next cursor ID = %s, want %s", next.ID, firstID)
	}
}

func TestAdminConsoleCapabilitiesHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/console/capabilities", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body adminConsoleCapabilitiesEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	got := body.AdminConsoleCapabilities
	if got.ContractVersion != BackendContractVersion {
		t.Fatalf("contract version = %q, want %q", got.ContractVersion, BackendContractVersion)
	}
	if got.Modules["admin"] != "available" || got.Modules["mail"] != "available" || got.Modules["drive"] != "available" {
		t.Fatalf("modules = %#v", got.Modules)
	}
	if got.Limits.MaxListLimit != maildb.MessageListMaxLimit ||
		got.Limits.MaxAttachmentCleanupLimit != maildb.AttachmentCleanupMaxLimit ||
		got.Limits.MaxAPIUsageRetentionRunLimit != maildb.APIUsageLedgerRetentionMaxLimit {
		t.Fatalf("limits = %#v", got.Limits)
	}
	if !got.Tenancy.Companies || !got.Tenancy.Domains || !got.Tenancy.Users || !got.Tenancy.DNSChecks || !got.Tenancy.DKIMKeys {
		t.Fatalf("tenancy capabilities = %#v", got.Tenancy)
	}
	if !got.Operations.AuditLogs || !got.Operations.DirectoryPrincipals || !got.Operations.DirectoryAliases || !got.Operations.DirectoryDelegations || !got.Operations.DeliveryRoutes || !got.Operations.APIUsageExport || !got.Operations.DAVSyncRetention || !got.Operations.IMAPUIDBackfill || !got.Operations.DriveUploadSessions || !got.Operations.DriveNodes || !got.Operations.DriveNodeDetail || !got.Operations.DriveUsageSummary || !got.Operations.DriveUploadCleanup || !got.Operations.DriveCleanupFailures || !got.Operations.DriveCleanupFailureRetry {
		t.Fatalf("operation capabilities = %#v", got.Operations)
	}
	if !got.Security.AdminTokenHeader || !got.Security.BearerToken || !got.Security.RejectsAmbiguousAuth || !got.Security.NoStoreJSON {
		t.Fatalf("security capabilities = %#v", got.Security)
	}
	if got.Integrations.LDAPRead != "available" || got.Integrations.LDAPSync != "unavailable" || got.Integrations.OrganizationSync != "unavailable" {
		t.Fatalf("integration capabilities = %#v", got.Integrations)
	}
	if got.Storage.ConfiguredBackend != "local" || !got.Storage.LocalFilesystem || !got.Storage.SecretsRedacted || len(got.Storage.ActiveLabels) != 1 || got.Storage.ActiveLabels[0] != "local" || !got.Storage.SupportsLocalNFS || got.Storage.SupportsMinIO || got.Storage.SupportsAWSCompatible {
		t.Fatalf("storage capabilities = %#v", got.Storage)
	}
	wantStorageOperations := []string{"put", "get", "get_range", "stat", "copy", "move", "list", "delete"}
	if len(got.Storage.Operations) != len(wantStorageOperations) {
		t.Fatalf("storage operations = %#v, want %#v", got.Storage.Operations, wantStorageOperations)
	}
	for i, want := range wantStorageOperations {
		if got.Storage.Operations[i] != want {
			t.Fatalf("storage operations = %#v, want %#v", got.Storage.Operations, wantStorageOperations)
		}
	}
}

func TestAdminLDAPSyncUnavailableReturnsNotImplemented(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{ldapSyncErr: ldapidp.ErrSyncNotConfigured}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodPost, "/admin/v1/domains/domain-1/ldap/sync?sync_type=users", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), ldapidp.ErrSyncNotConfigured.Error()) {
		t.Fatalf("body = %q, leaked backend sentinel error", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "ldap sync is not configured") {
		t.Fatalf("body = %q, want public not-configured error", rec.Body.String())
	}
}

func TestAdminRDBMSSyncUnavailableReturnsNotImplemented(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{rdbmsSyncErr: rdbmsidp.ErrSyncNotConfigured}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodPost, "/admin/v1/domains/domain-1/rdbms/sync?sync_type=users", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), rdbmsidp.ErrSyncNotConfigured.Error()) {
		t.Fatalf("body = %q, leaked backend sentinel error", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "rdbms sync is not configured") {
		t.Fatalf("body = %q, want public not-configured error", rec.Body.String())
	}
}

func TestAdminCompanyAuditPolicyDefaultsToSafeValues(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/companies/company-1/security/audit-policy", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var body struct {
		Policy struct {
			CompanyID           string `json:"company_id"`
			AuditLevel          string `json:"audit_level"`
			AuditAdminActions   bool   `json:"audit_admin_actions"`
			AuditSecurityEvents bool   `json:"audit_security_events"`
			RetentionDays       int    `json:"retention_days"`
			MaskMailContent     bool   `json:"mask_mail_content"`
			MaskRecipientEmails bool   `json:"mask_recipient_emails"`
		} `json:"policy"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if body.Policy.CompanyID != "company-1" {
		t.Fatalf("company_id = %q, want company-1", body.Policy.CompanyID)
	}
	if body.Policy.AuditLevel != "level_2" || !body.Policy.AuditAdminActions || !body.Policy.AuditSecurityEvents || body.Policy.RetentionDays != 90 || !body.Policy.MaskMailContent || body.Policy.MaskRecipientEmails {
		t.Fatalf("policy = %+v", body.Policy)
	}
	if service.lastCompanyConfigID != "company-1" || service.lastCompanyConfigKey != "audit_policy" {
		t.Fatalf("config lookup = (%q, %q), want (company-1, audit_policy)", service.lastCompanyConfigID, service.lastCompanyConfigKey)
	}
}

func TestAdminCompanyAuditPolicySavePersistsCompanyConfig(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodPut, "/admin/v1/companies/company-1/security/audit-policy", strings.NewReader(`{"audit_level":"level_3","audit_admin_actions":false,"audit_security_events":true,"retention_days":180,"mask_mail_content":false,"mask_recipient_emails":true}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var body struct {
		Policy struct {
			CompanyID           string `json:"company_id"`
			AuditLevel          string `json:"audit_level"`
			AuditAdminActions   bool   `json:"audit_admin_actions"`
			AuditSecurityEvents bool   `json:"audit_security_events"`
			RetentionDays       int    `json:"retention_days"`
			MaskMailContent     bool   `json:"mask_mail_content"`
			MaskRecipientEmails bool   `json:"mask_recipient_emails"`
		} `json:"policy"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if body.Policy.CompanyID != "company-1" || body.Policy.AuditLevel != "level_3" || body.Policy.RetentionDays != 180 || body.Policy.MaskMailContent || !body.Policy.MaskRecipientEmails {
		t.Fatalf("policy = %+v", body.Policy)
	}
	if service.lastCompanyConfigID != "company-1" || service.lastCompanyConfigKey != "audit_policy" {
		t.Fatalf("config save = (%q, %q), want (company-1, audit_policy)", service.lastCompanyConfigID, service.lastCompanyConfigKey)
	}
	if len(service.companyConfig) != 0 {
		t.Fatalf("companyConfig should not be prepopulated by test")
	}
}

func TestAdminListRolesHandlerUsesService(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, 5, 14, 9, 0, 0, 0, time.UTC)
	service := &fakeAdminService{
		adminRoles: []admin.RoleSummary{{
			ID:               "role-1",
			CompanyID:        "company-1",
			Name:             "Operator",
			Description:      "Runs mail operations",
			IsBuiltin:        false,
			PermissionsCount: 7,
			AssignedUsers:    3,
			CreatedAt:        createdAt,
			UpdatedAt:        createdAt,
		}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/roles?company_id=company-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Roles []admin.RoleSummary `json:"roles"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if service.lastRoleCompanyID != "company-1" {
		t.Fatalf("lastRoleCompanyID = %q, want company-1", service.lastRoleCompanyID)
	}
	if len(body.Roles) != 1 || body.Roles[0].ID != "role-1" || body.Roles[0].PermissionsCount != 7 || body.Roles[0].AssignedUsers != 3 {
		t.Fatalf("roles = %+v", body.Roles)
	}
}

func TestAdminCreateRoleHandlerPersistsCustomRole(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, 5, 14, 9, 0, 0, 0, time.UTC)
	service := &fakeAdminService{
		createdAdminRole: admin.RoleSummary{
			ID:          "role-new",
			CompanyID:   "company-1",
			Name:        "Security Analyst",
			Description: "Investigates abuse",
			IsBuiltin:   false,
			CreatedAt:   createdAt,
			UpdatedAt:   createdAt,
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodPost, "/admin/v1/roles", strings.NewReader(`{"company_id":"company-1","name":" Security Analyst ","description":"Investigates abuse"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastCreateAdminRole.CompanyID != "company-1" || service.lastCreateAdminRole.Name != "Security Analyst" {
		t.Fatalf("lastCreateAdminRole = %+v", service.lastCreateAdminRole)
	}
	var body struct {
		Role admin.RoleSummary `json:"role"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if body.Role.ID != "role-new" || body.Role.IsBuiltin {
		t.Fatalf("role = %+v", body.Role)
	}
}

func TestAdminCreateRoleHandlerRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
	}{
		{name: "missing company", body: `{"name":"Operator"}`},
		{name: "missing name", body: `{"company_id":"company-1"}`},
		{name: "builtin rejected", body: `{"company_id":"company-1","name":"Root","is_builtin":true}`},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := &fakeAdminService{}
			mux := http.NewServeMux()
			RegisterAdminRoutes(mux, service, "")

			req := httptest.NewRequest(http.MethodPost, "/admin/v1/roles", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if service.lastCreateAdminRole.Name != "" {
				t.Fatalf("dispatched role create %+v", service.lastCreateAdminRole)
			}
		})
	}
}

func TestAdminRoleHandlersMapServiceErrors(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{roleErr: fmt.Errorf("role backend down")}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	listReq := httptest.NewRequest(http.MethodGet, "/admin/v1/roles?company_id=company-1", nil)
	listRec := httptest.NewRecorder()
	mux.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusInternalServerError {
		t.Fatalf("list status = %d, body = %s", listRec.Code, listRec.Body.String())
	}

	createReq := httptest.NewRequest(http.MethodPost, "/admin/v1/roles", strings.NewReader(`{"company_id":"company-1","name":"Operator"}`))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	mux.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusBadRequest {
		t.Fatalf("create status = %d, body = %s", createRec.Code, createRec.Body.String())
	}
}

func TestAdminConsoleCapabilitiesHandlerIsAdminBaseOnly(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/console/capabilities", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestAdminConsoleCapabilitiesHandlerAcceptsDocumentedAdminAuth(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		header   string
		value    string
		wantCode int
	}{
		{name: "admin token header", header: "X-Admin-Token", value: "secret", wantCode: http.StatusOK},
		{name: "bearer token", header: "Authorization", value: "Bearer secret", wantCode: http.StatusOK},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := &fakeAdminService{}
			mux := http.NewServeMux()
			RegisterAdminRoutes(mux, service, "secret")

			req := httptest.NewRequest(http.MethodGet, "/admin/v1/console/capabilities", nil)
			req.Header.Set(tt.header, tt.value)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != tt.wantCode {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestAdminConsoleCapabilitiesHandlerRejectsAmbiguousAdminAuth(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "secret")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/console/capabilities", nil)
	req.Header.Set("X-Admin-Token", "secret")
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestAdminConsoleCapabilitiesHandlerUsesConfiguredStorageCapabilities(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "", WithStorageCapabilities(storage.BackendCapabilities{
		ContractVersion:       BackendContractVersion,
		ConfiguredBackend:     "minio",
		BackendClass:          "s3_compatible",
		ActiveLabels:          []string{"minio", "s3"},
		Operations:            []string{"put", "get", "get_range", "stat", "copy", "move", "list", "delete"},
		S3Compatible:          true,
		PathStyleAddressing:   true,
		EndpointOrigin:        "http://localhost:19000",
		Bucket:                "gogomail",
		SecretsRedacted:       true,
		SupportsMinIO:         true,
		SupportsAWSCompatible: true,
	}))

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/console/capabilities", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body adminConsoleCapabilitiesEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	got := body.AdminConsoleCapabilities.Storage
	if got.ConfiguredBackend != "minio" || got.BackendClass != "s3_compatible" || !got.S3Compatible || !got.PathStyleAddressing || got.EndpointOrigin != "http://localhost:19000" || got.Bucket != "gogomail" || got.SupportsLocalNFS || !got.SupportsMinIO || !got.SupportsAWSCompatible {
		t.Fatalf("storage capabilities = %#v", got)
	}
}

func TestAdminBackfillIMAPMailboxUIDsHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		imapUIDBackfill: []maildb.IMAPMessageUID{{
			MessageID: "msg-1",
			MailboxID: "inbox",
			UID:       imapgw.UID(12),
			ModSeq:    2,
		}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodPost, "/admin/v1/imap/mailboxes/%20inbox%20/uid-backfill?user_id=%20user-1%20&limit=10", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastIMAPBackfillUserID != "user-1" || service.lastIMAPBackfillMailboxID != "inbox" || service.lastIMAPBackfillLimit != 10 {
		t.Fatalf("backfill request = %q/%q/%d", service.lastIMAPBackfillUserID, service.lastIMAPBackfillMailboxID, service.lastIMAPBackfillLimit)
	}
	if !strings.Contains(rec.Body.String(), `"imap_uid_backfill"`) || !strings.Contains(rec.Body.String(), `"message_id":"msg-1"`) || !strings.Contains(rec.Body.String(), `"uid":12`) {
		t.Fatalf("response missing backfill envelope/items: %s", rec.Body.String())
	}
}

func TestAdminBackfillIMAPMailboxUIDsRejectsUnsafeUserID(t *testing.T) {
	t.Parallel()

	tests := []string{
		"/admin/v1/imap/mailboxes/inbox/uid-backfill?user_id=user%0Abad",
		"/admin/v1/imap/mailboxes/inbox/uid-backfill?user_id=" + strings.Repeat("u", maxAdminQueryFilterBytes+1),
	}

	for _, path := range tests {
		path := path
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			service := &fakeAdminService{}
			mux := http.NewServeMux()
			RegisterAdminRoutes(mux, service, "")

			req := httptest.NewRequest(http.MethodPost, path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if service.lastIMAPBackfillUserID != "" || service.lastIMAPBackfillMailboxID != "" {
				t.Fatalf("backfill request = %q/%q", service.lastIMAPBackfillUserID, service.lastIMAPBackfillMailboxID)
			}
		})
	}
}

func TestAdminBackfillIMAPMailboxUIDsRejectsUnsafeMailboxID(t *testing.T) {
	t.Parallel()

	tests := []string{
		"/admin/v1/imap/mailboxes/inbox%0Abad/uid-backfill?user_id=user-1",
		"/admin/v1/imap/mailboxes/" + strings.Repeat("m", maxAdminQueryFilterBytes+1) + "/uid-backfill?user_id=user-1",
	}

	for _, path := range tests {
		path := path
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			service := &fakeAdminService{}
			mux := http.NewServeMux()
			RegisterAdminRoutes(mux, service, "")

			req := httptest.NewRequest(http.MethodPost, path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if service.lastIMAPBackfillUserID != "" || service.lastIMAPBackfillMailboxID != "" {
				t.Fatalf("backfill request = %q/%q", service.lastIMAPBackfillUserID, service.lastIMAPBackfillMailboxID)
			}
		})
	}
}

func TestAdminOutboxEventsHandler(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 4, 9, 0, 0, 0, time.UTC)
	service := &fakeAdminService{
		outboxEvents: []maildb.OutboxEventView{{
			ID:           "outbox-1",
			Topic:        "mail.event",
			PartitionKey: "msg-1",
			Status:       "pending",
			Attempts:     1,
			CreatedAt:    now,
			AvailableAt:  now,
		}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/outbox-events?limit=10&topic=%20mail.event%20&partition_key=%20msg-1%20&status=%20pending%20&since=2026-05-04T00:00:00Z", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Events []maildb.OutboxEventView `json:"outbox_events"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Events) != 1 || body.Events[0].ID != "outbox-1" {
		t.Fatalf("outbox_events = %+v", body.Events)
	}
	if service.lastOutboxEventList.Limit != 10 || service.lastOutboxEventList.Topic != "mail.event" || service.lastOutboxEventList.PartitionKey != "msg-1" || service.lastOutboxEventList.Status != "pending" || service.lastOutboxEventList.Since.IsZero() {
		t.Fatalf("lastOutboxEventList = %+v", service.lastOutboxEventList)
	}
}

func TestAdminOutboxEventsHandlerRejectsInvalidSince(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, &fakeAdminService{}, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/outbox-events?since=not-a-time", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "since must be RFC3339 timestamp") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestAdminOutboxEventsHandlerRejectsInvalidStatus(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, &fakeAdminService{}, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/outbox-events?status=stuck", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "unsupported outbox status") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestAdminOutboxEventsHandlerRejectsUnsafeFilters(t *testing.T) {
	t.Parallel()

	tests := []string{
		"/admin/v1/outbox-events?topic=mail.event%0D%0Abad",
		"/admin/v1/outbox-events?partition_key=" + strings.Repeat("x", maxAdminQueryFilterBytes+1),
		"/admin/v1/outbox-events?status=pending%0Abad",
	}
	for _, path := range tests {
		service := &fakeAdminService{}
		mux := http.NewServeMux()
		RegisterAdminRoutes(mux, service, "")

		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s status = %d, body = %s", path, rec.Code, rec.Body.String())
		}
		if service.lastOutboxEventList.Limit != 0 {
			t.Fatalf("%s dispatched request %+v", path, service.lastOutboxEventList)
		}
	}
}

func TestAdminOutboxEventDetailHandler(t *testing.T) {
	t.Parallel()

	longError := strings.Repeat("redis down ", 80)
	now := time.Date(2026, 5, 4, 9, 0, 0, 0, time.UTC)
	service := &fakeAdminService{
		outboxEvent: maildb.OutboxEventView{
			ID:           "outbox-1",
			Topic:        "mail.event",
			PartitionKey: "msg-1",
			Status:       "failed",
			Attempts:     10,
			LastError:    longError,
			CreatedAt:    now,
			AvailableAt:  now,
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/outbox-events/outbox-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Event maildb.OutboxEventView `json:"outbox_event"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Event.ID != "outbox-1" || body.Event.LastError != longError {
		t.Fatalf("outbox_event = %+v", body.Event)
	}
	if service.lastOutboxEventID != "outbox-1" {
		t.Fatalf("lastOutboxEventID = %q", service.lastOutboxEventID)
	}
}

func TestAdminOutboxEventDetailHandlerRejectsUnsafeID(t *testing.T) {
	t.Parallel()

	tests := []string{
		"/admin/v1/outbox-events/outbox%0Abad",
		"/admin/v1/outbox-events/" + strings.Repeat("o", maxAdminQueryFilterBytes+1),
	}

	for _, path := range tests {
		path := path
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			service := &fakeAdminService{}
			mux := http.NewServeMux()
			RegisterAdminRoutes(mux, service, "")

			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if service.lastOutboxEventID != "" {
				t.Fatalf("lastOutboxEventID = %q", service.lastOutboxEventID)
			}
		})
	}
}

func TestAdminAuditLogsHandler(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 4, 9, 0, 0, 0, time.UTC)
	service := &fakeAdminService{
		auditLogs: []maildb.AuditLogView{{
			ID:         "audit-1",
			Category:   "admin",
			Action:     "quota.reconciliation_correction",
			TargetType: "user",
			Result:     "applied",
			Detail:     json.RawMessage(`{"before_drift_count":1}`),
			CreatedAt:  now,
		}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/audit-logs?limit=10&category=%20admin%20&action=%20quota.reconciliation_correction%20&action_prefix=%20quota.%20&result=%20applied%20&target_type=%20user%20&company_id=%20company-1%20&domain_id=%20domain-1%20&user_id=%20user-1%20&actor_id=%20actor-1%20&target_id=%20target-1%20&since=2026-05-04T00:00:00Z", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Logs []maildb.AuditLogView `json:"audit_logs"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Logs) != 1 || body.Logs[0].ID != "audit-1" {
		t.Fatalf("audit_logs = %+v", body.Logs)
	}
	if service.lastAuditLogList.Limit != 10 ||
		service.lastAuditLogList.Category != "admin" ||
		service.lastAuditLogList.Action != "quota.reconciliation_correction" ||
		service.lastAuditLogList.ActionPrefix != "quota." ||
		service.lastAuditLogList.Result != "applied" ||
		service.lastAuditLogList.TargetType != "user" ||
		service.lastAuditLogList.CompanyID != "company-1" ||
		service.lastAuditLogList.DomainID != "domain-1" ||
		service.lastAuditLogList.UserID != "user-1" ||
		service.lastAuditLogList.ActorID != "actor-1" ||
		service.lastAuditLogList.TargetID != "target-1" ||
		service.lastAuditLogList.Since.IsZero() {
		t.Fatalf("lastAuditLogList = %+v", service.lastAuditLogList)
	}
}

func TestAdminCompanyAuditLogsExportHandler(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 4, 9, 0, 0, 0, time.UTC)
	service := &fakeAdminService{
		auditLogs: []maildb.AuditLogView{{
			ID:         "audit-1",
			CompanyID:  "company-1",
			ActorID:    "actor-1",
			Category:   "admin",
			Action:     "quota.reconciliation_correction",
			TargetType: "user",
			TargetID:   "target-1",
			Result:     "applied",
			IPAddress:  "192.0.2.10",
			CreatedAt:  now,
		}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/companies/company-1/audit-logs/export?limit=10&category=admin&action_prefix=quota.", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/csv") {
		t.Fatalf("Content-Type = %q, want text/csv", ct)
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, `audit-logs-company-1.csv`) {
		t.Fatalf("Content-Disposition = %q", cd)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "audit-1,company-1,actor-1,admin,quota.reconciliation_correction,user,target-1,applied,192.0.2.10") {
		t.Fatalf("CSV body = %q", body)
	}
	if service.lastAuditLogList.CompanyID != "company-1" || service.lastAuditLogList.Limit != 10 || service.lastAuditLogList.Category != "admin" || service.lastAuditLogList.ActionPrefix != "quota." {
		t.Fatalf("lastAuditLogList = %+v", service.lastAuditLogList)
	}
}

func TestAdminAuditLogsHandlerRejectsUnsafeFilters(t *testing.T) {
	t.Parallel()

	tests := []string{
		"/admin/v1/audit-logs?category=admin%0Abad",
		"/admin/v1/audit-logs?action=" + strings.Repeat("x", maxAdminQueryFilterBytes+1),
		"/admin/v1/audit-logs?action_prefix=share_link.%0Abad",
		"/admin/v1/audit-logs?actor_id=actor%0Dbad",
		"/admin/v1/audit-logs?target_id=" + strings.Repeat("x", maxAdminQueryFilterBytes+1),
		"/admin/v1/audit-logs?since=not-a-time",
	}
	for _, path := range tests {
		service := &fakeAdminService{}
		mux := http.NewServeMux()
		RegisterAdminRoutes(mux, service, "")

		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s status = %d, body = %s", path, rec.Code, rec.Body.String())
		}
		if service.lastAuditLogList.Limit != 0 {
			t.Fatalf("%s dispatched request %+v", path, service.lastAuditLogList)
		}
	}
}

func TestAdminDirectoryDelegationsHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		directoryDelegations: []directory.Delegation{{
			ID:           "delegation-1",
			CompanyID:    "company-1",
			OwnerKind:    directory.PrincipalKindResource,
			OwnerID:      "room-1",
			DelegateKind: directory.PrincipalKindGroup,
			DelegateID:   "team-1",
			Scope:        directory.DelegationScopeCalendar,
			Role:         directory.DelegationRoleWrite,
			Status:       "active",
		}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/directory/delegations?limit=10&company_id=%20company-1%20&owner_kind=resource&owner_id=room-1&delegate_kind=group&delegate_id=team-1&scope=calendar&role=write&active_only=false", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Delegations []directory.Delegation `json:"directory_delegations"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Delegations) != 1 || body.Delegations[0].ID != "delegation-1" {
		t.Fatalf("directory_delegations = %+v", body.Delegations)
	}
	if service.lastDirectoryDelegationList.Limit != 10 ||
		service.lastDirectoryDelegationList.CompanyID != "company-1" ||
		service.lastDirectoryDelegationList.OwnerKind != directory.PrincipalKindResource ||
		service.lastDirectoryDelegationList.OwnerID != "room-1" ||
		service.lastDirectoryDelegationList.DelegateKind != directory.PrincipalKindGroup ||
		service.lastDirectoryDelegationList.DelegateID != "team-1" ||
		service.lastDirectoryDelegationList.Scope != directory.DelegationScopeCalendar ||
		service.lastDirectoryDelegationList.Role != directory.DelegationRoleWrite ||
		service.lastDirectoryDelegationList.ActiveOnly {
		t.Fatalf("lastDirectoryDelegationList = %+v", service.lastDirectoryDelegationList)
	}
}

func TestAdminDirectoryPrincipalsHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		directoryPrincipals: []directory.Principal{{
			ID:           "user-1",
			Kind:         directory.PrincipalKindUser,
			CompanyID:    "company-1",
			DomainID:     "domain-1",
			DisplayName:  "Alice",
			PrimaryEmail: "alice@example.com",
			Status:       "active",
		}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/directory/principals?limit=10&company_id=%20company-1%20&domain_id=domain-1&organization_id=org-1&kinds=user,resource&q=%20Alice%20&active_only=false", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Principals []directory.Principal `json:"directory_principals"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Principals) != 1 || body.Principals[0].ID != "user-1" {
		t.Fatalf("directory_principals = %+v", body.Principals)
	}
	if service.lastDirectoryPrincipalSearch.Limit != 10 ||
		service.lastDirectoryPrincipalSearch.CompanyID != "company-1" ||
		service.lastDirectoryPrincipalSearch.DomainID != "domain-1" ||
		service.lastDirectoryPrincipalSearch.OrganizationID != "org-1" ||
		strings.Join(service.lastDirectoryPrincipalSearch.Kinds, ",") != "user,resource" ||
		service.lastDirectoryPrincipalSearch.Query != "Alice" ||
		service.lastDirectoryPrincipalSearch.ActiveOnly {
		t.Fatalf("lastDirectoryPrincipalSearch = %+v", service.lastDirectoryPrincipalSearch)
	}
}

func TestAdminDirectoryPrincipalsHandlerDefaultsActiveOnly(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/directory/principals?company_id=company-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !service.lastDirectoryPrincipalSearch.ActiveOnly {
		t.Fatalf("lastDirectoryPrincipalSearch.ActiveOnly = false, want default true")
	}
}

func TestAdminDirectoryPrincipalsHandlerRejectsUnsafeFilters(t *testing.T) {
	t.Parallel()

	tests := []string{
		"/admin/v1/directory/principals?company_id=company%0Abad",
		"/admin/v1/directory/principals?company_id=company-1&kinds=calendar",
		"/admin/v1/directory/principals?company_id=company-1&q=" + strings.Repeat("x", directory.MaxPrincipalSearchBytes+1),
		"/admin/v1/directory/principals?company_id=company-1&active_only=maybe",
		"/admin/v1/directory/principals?company_id=company-1&cursor=opaque",
	}
	for _, path := range tests {
		service := &fakeAdminService{}
		mux := http.NewServeMux()
		RegisterAdminRoutes(mux, service, "")

		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s status = %d, body = %s", path, rec.Code, rec.Body.String())
		}
		if service.lastDirectoryPrincipalSearch.CompanyID != "" {
			t.Fatalf("%s dispatched request %+v", path, service.lastDirectoryPrincipalSearch)
		}
	}
}

func TestAdminDirectoryAliasResolveHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		directoryAlias: directory.Alias{
			ID:         "alias-1",
			CompanyID:  "company-1",
			DomainID:   "domain-1",
			Address:    "ops@example.com",
			AddressACE: "ops@example.com",
			TargetKind: directory.PrincipalKindGroup,
			TargetID:   "group-1",
			Status:     "active",
			TargetPrincipal: directory.Principal{
				ID:          "group-1",
				Kind:        directory.PrincipalKindGroup,
				CompanyID:   "company-1",
				DisplayName: "Ops",
				Status:      "active",
			},
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/directory/aliases/resolve?address=%20Ops@Example.COM%20&active_only=false", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Alias directory.Alias `json:"directory_alias"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Alias.ID != "alias-1" || body.Alias.TargetPrincipal.ID != "group-1" {
		t.Fatalf("directory_alias = %+v", body.Alias)
	}
	if service.lastDirectoryAliasResolve.Address != "Ops@Example.COM" ||
		service.lastDirectoryAliasResolve.ActiveOnly {
		t.Fatalf("lastDirectoryAliasResolve = %+v", service.lastDirectoryAliasResolve)
	}
}

func TestAdminDirectoryAliasResolveHandlerDefaultsActiveOnly(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{directoryAlias: directory.Alias{ID: "alias-1"}}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/directory/aliases/resolve?address=ops@example.com", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !service.lastDirectoryAliasResolve.ActiveOnly {
		t.Fatalf("lastDirectoryAliasResolve.ActiveOnly = false, want default true")
	}
}

func TestAdminDirectoryAliasResolveHandlerRejectsUnsafeFilters(t *testing.T) {
	t.Parallel()

	tests := []string{
		"/admin/v1/directory/aliases/resolve",
		"/admin/v1/directory/aliases/resolve?address=not-an-address",
		"/admin/v1/directory/aliases/resolve?address=ops@example.com%0Abad",
		"/admin/v1/directory/aliases/resolve?address=ops@example.com&active_only=maybe",
		"/admin/v1/directory/aliases/resolve?address=ops@example.com&cursor=opaque",
	}
	for _, path := range tests {
		service := &fakeAdminService{}
		mux := http.NewServeMux()
		RegisterAdminRoutes(mux, service, "")

		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s status = %d, body = %s", path, rec.Code, rec.Body.String())
		}
		if service.lastDirectoryAliasResolve.Address != "" {
			t.Fatalf("%s dispatched request %+v", path, service.lastDirectoryAliasResolve)
		}
	}
}

func TestAdminDirectoryAliasesHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		directoryAliases: []directory.Alias{{
			ID:         "alias-1",
			CompanyID:  "company-1",
			DomainID:   "domain-1",
			Address:    "ops@example.com",
			AddressACE: "ops@example.com",
			TargetKind: directory.PrincipalKindGroup,
			TargetID:   "group-1",
			Status:     "active",
			TargetPrincipal: directory.Principal{
				ID:          "group-1",
				Kind:        directory.PrincipalKindGroup,
				CompanyID:   "company-1",
				DisplayName: "Ops",
				Status:      "active",
			},
		}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/directory/aliases?limit=10&company_id=%20company-1%20&domain_id=domain-1&target_kind=group&target_id=group-1&q=%20ops%20&active_only=false", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Aliases []directory.Alias `json:"directory_aliases"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Aliases) != 1 || body.Aliases[0].ID != "alias-1" || body.Aliases[0].TargetPrincipal.ID != "group-1" {
		t.Fatalf("directory_aliases = %+v", body.Aliases)
	}
	if service.lastDirectoryAliasList.Limit != 10 ||
		service.lastDirectoryAliasList.CompanyID != "company-1" ||
		service.lastDirectoryAliasList.DomainID != "domain-1" ||
		service.lastDirectoryAliasList.TargetKind != directory.PrincipalKindGroup ||
		service.lastDirectoryAliasList.TargetID != "group-1" ||
		service.lastDirectoryAliasList.Query != "ops" ||
		service.lastDirectoryAliasList.ActiveOnly {
		t.Fatalf("lastDirectoryAliasList = %+v", service.lastDirectoryAliasList)
	}
}

func TestAdminDirectoryAliasesHandlerDefaultsActiveOnly(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/directory/aliases?company_id=company-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !service.lastDirectoryAliasList.ActiveOnly {
		t.Fatalf("lastDirectoryAliasList.ActiveOnly = false, want default true")
	}
}

func TestAdminDirectoryAliasesHandlerRejectsUnsafeFilters(t *testing.T) {
	t.Parallel()

	tests := []string{
		"/admin/v1/directory/aliases?company_id=company%0Abad",
		"/admin/v1/directory/aliases?company_id=company-1&target_id=group-1",
		"/admin/v1/directory/aliases?company_id=company-1&target_kind=calendar",
		"/admin/v1/directory/aliases?company_id=company-1&q=" + strings.Repeat("x", directory.MaxAliasSearchBytes+1),
		"/admin/v1/directory/aliases?company_id=company-1&active_only=maybe",
		"/admin/v1/directory/aliases?company_id=company-1&cursor=opaque",
	}
	for _, path := range tests {
		service := &fakeAdminService{}
		mux := http.NewServeMux()
		RegisterAdminRoutes(mux, service, "")

		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s status = %d, body = %s", path, rec.Code, rec.Body.String())
		}
		if service.lastDirectoryAliasList.CompanyID != "" {
			t.Fatalf("%s dispatched request %+v", path, service.lastDirectoryAliasList)
		}
	}
}

func TestAdminDirectoryDelegationsHandlerDefaultsActiveOnly(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/directory/delegations?company_id=company-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !service.lastDirectoryDelegationList.ActiveOnly {
		t.Fatalf("lastDirectoryDelegationList.ActiveOnly = false, want default true")
	}
}

func TestAdminDirectoryDelegationsHandlerRejectsUnsafeFilters(t *testing.T) {
	t.Parallel()

	tests := []string{
		"/admin/v1/directory/delegations?company_id=company%0Abad",
		"/admin/v1/directory/delegations?company_id=company-1&owner_id=owner-1",
		"/admin/v1/directory/delegations?company_id=company-1&delegate_kind=calendar",
		"/admin/v1/directory/delegations?company_id=company-1&active_only=maybe",
		"/admin/v1/directory/delegations?company_id=company-1&cursor=opaque",
	}
	for _, path := range tests {
		service := &fakeAdminService{}
		mux := http.NewServeMux()
		RegisterAdminRoutes(mux, service, "")

		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s status = %d, body = %s", path, rec.Code, rec.Body.String())
		}
		if service.lastDirectoryDelegationList.CompanyID != "" {
			t.Fatalf("%s dispatched request %+v", path, service.lastDirectoryDelegationList)
		}
	}
}

func TestAdminDirectoryGroupMembershipsHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		directoryGroupMemberships: []directory.GroupMembership{{
			ID:         "membership-1",
			GroupID:    "group-1",
			CompanyID:  "company-1",
			MemberKind: directory.PrincipalKindUser,
			MemberID:   "user-1",
			Role:       directory.GroupMembershipRoleManager,
			Status:     "active",
		}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/directory/group-memberships?limit=10&company_id=%20company-1%20&group_id=group-1&member_kind=user&member_id=user-1&role=manager&active_only=false", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Memberships []directory.GroupMembership `json:"directory_group_memberships"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Memberships) != 1 || body.Memberships[0].ID != "membership-1" {
		t.Fatalf("directory_group_memberships = %+v", body.Memberships)
	}
	if service.lastDirectoryGroupMembershipList.Limit != 10 ||
		service.lastDirectoryGroupMembershipList.CompanyID != "company-1" ||
		service.lastDirectoryGroupMembershipList.GroupID != "group-1" ||
		service.lastDirectoryGroupMembershipList.MemberKind != directory.PrincipalKindUser ||
		service.lastDirectoryGroupMembershipList.MemberID != "user-1" ||
		service.lastDirectoryGroupMembershipList.Role != directory.GroupMembershipRoleManager ||
		service.lastDirectoryGroupMembershipList.ActiveOnly {
		t.Fatalf("lastDirectoryGroupMembershipList = %+v", service.lastDirectoryGroupMembershipList)
	}
}

func TestAdminDirectoryGroupMembershipsHandlerDefaultsActiveOnly(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/directory/group-memberships?company_id=company-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !service.lastDirectoryGroupMembershipList.ActiveOnly {
		t.Fatalf("lastDirectoryGroupMembershipList.ActiveOnly = false, want default true")
	}
}

func TestAdminDirectoryGroupMembershipsHandlerRejectsUnsafeFilters(t *testing.T) {
	t.Parallel()

	tests := []string{
		"/admin/v1/directory/group-memberships?company_id=company%0Abad",
		"/admin/v1/directory/group-memberships?company_id=company-1&member_id=user-1",
		"/admin/v1/directory/group-memberships?company_id=company-1&member_kind=calendar",
		"/admin/v1/directory/group-memberships?company_id=company-1&role=admin",
		"/admin/v1/directory/group-memberships?company_id=company-1&active_only=maybe",
		"/admin/v1/directory/group-memberships?company_id=company-1&cursor=opaque",
	}
	for _, path := range tests {
		service := &fakeAdminService{}
		mux := http.NewServeMux()
		RegisterAdminRoutes(mux, service, "")

		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s status = %d, body = %s", path, rec.Code, rec.Body.String())
		}
		if service.lastDirectoryGroupMembershipList.CompanyID != "" {
			t.Fatalf("%s dispatched request %+v", path, service.lastDirectoryGroupMembershipList)
		}
	}
}

func TestAdminCreateDirectoryDelegationHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		directoryDelegation: directory.Delegation{
			ID:           "delegation-1",
			CompanyID:    "company-1",
			OwnerKind:    directory.PrincipalKindResource,
			OwnerID:      "room-1",
			DelegateKind: directory.PrincipalKindGroup,
			DelegateID:   "team-1",
			Scope:        directory.DelegationScopeCalendar,
			Role:         directory.DelegationRoleWrite,
			Status:       "active",
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	body := []byte(`{"company_id":"company-1","owner_kind":"resource","owner_id":"room-1","delegate_kind":"group","delegate_id":"team-1","scope":"calendar","role":"write"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/v1/directory/delegations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var response struct {
		Delegation directory.Delegation `json:"directory_delegation"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Delegation.ID != "delegation-1" {
		t.Fatalf("directory_delegation = %+v", response.Delegation)
	}
	if service.lastDirectoryDelegationCreate.CompanyID != "company-1" ||
		service.lastDirectoryDelegationCreate.OwnerKind != directory.PrincipalKindResource ||
		service.lastDirectoryDelegationCreate.OwnerID != "room-1" ||
		service.lastDirectoryDelegationCreate.DelegateKind != directory.PrincipalKindGroup ||
		service.lastDirectoryDelegationCreate.DelegateID != "team-1" ||
		service.lastDirectoryDelegationCreate.Scope != directory.DelegationScopeCalendar ||
		service.lastDirectoryDelegationCreate.Role != directory.DelegationRoleWrite {
		t.Fatalf("lastDirectoryDelegationCreate = %+v", service.lastDirectoryDelegationCreate)
	}
}

func TestAdminCreateDirectoryDelegationHandlerRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		body string
		ct   string
	}{
		{name: "unknown query", path: "/admin/v1/directory/delegations?dry_run=true", body: `{}`, ct: "application/json"},
		{name: "bad content type", path: "/admin/v1/directory/delegations", body: `{}`, ct: "text/plain"},
		{name: "unknown json field", path: "/admin/v1/directory/delegations", body: `{"company_id":"company-1","owner_kind":"resource","owner_id":"room-1","delegate_kind":"group","delegate_id":"team-1","scope":"calendar","role":"write","extra":true}`, ct: "application/json"},
		{name: "self delegation", path: "/admin/v1/directory/delegations", body: `{"company_id":"company-1","owner_kind":"user","owner_id":"user-1","delegate_kind":"user","delegate_id":"user-1","scope":"calendar","role":"read"}`, ct: "application/json"},
		{name: "bad role", path: "/admin/v1/directory/delegations", body: `{"company_id":"company-1","owner_kind":"resource","owner_id":"room-1","delegate_kind":"group","delegate_id":"team-1","scope":"calendar","role":"owner"}`, ct: "application/json"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			service := &fakeAdminService{}
			mux := http.NewServeMux()
			RegisterAdminRoutes(mux, service, "")

			req := httptest.NewRequest(http.MethodPost, tc.path, bytes.NewReader([]byte(tc.body)))
			req.Header.Set("Content-Type", tc.ct)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if service.lastDirectoryDelegationCreate.CompanyID != "" {
				t.Fatalf("dispatched request %+v", service.lastDirectoryDelegationCreate)
			}
		})
	}
}

func TestAdminCreateDirectoryGroupMembershipHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		directoryGroupMembership: directory.GroupMembership{
			ID:         "membership-1",
			GroupID:    "group-1",
			CompanyID:  "company-1",
			MemberKind: directory.PrincipalKindUser,
			MemberID:   "user-1",
			Role:       directory.GroupMembershipRoleManager,
			Status:     "active",
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	body := []byte(`{"group_id":"group-1","member_kind":"user","member_id":"user-1","role":"manager"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/v1/directory/group-memberships", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var response struct {
		Membership directory.GroupMembership `json:"directory_group_membership"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Membership.ID != "membership-1" {
		t.Fatalf("directory_group_membership = %+v", response.Membership)
	}
	if service.lastDirectoryGroupMembershipCreate.GroupID != "group-1" ||
		service.lastDirectoryGroupMembershipCreate.MemberKind != directory.PrincipalKindUser ||
		service.lastDirectoryGroupMembershipCreate.MemberID != "user-1" ||
		service.lastDirectoryGroupMembershipCreate.Role != directory.GroupMembershipRoleManager {
		t.Fatalf("lastDirectoryGroupMembershipCreate = %+v", service.lastDirectoryGroupMembershipCreate)
	}
}

func TestAdminCreateDirectoryGroupMembershipHandlerRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		body string
		ct   string
	}{
		{name: "unknown query", path: "/admin/v1/directory/group-memberships?dry_run=true", body: `{}`, ct: "application/json"},
		{name: "bad content type", path: "/admin/v1/directory/group-memberships", body: `{}`, ct: "text/plain"},
		{name: "unknown json field", path: "/admin/v1/directory/group-memberships", body: `{"group_id":"group-1","member_kind":"user","member_id":"user-1","role":"member","extra":true}`, ct: "application/json"},
		{name: "self group", path: "/admin/v1/directory/group-memberships", body: `{"group_id":"group-1","member_kind":"group","member_id":"group-1"}`, ct: "application/json"},
		{name: "bad role", path: "/admin/v1/directory/group-memberships", body: `{"group_id":"group-1","member_kind":"user","member_id":"user-1","role":"admin"}`, ct: "application/json"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			service := &fakeAdminService{}
			mux := http.NewServeMux()
			RegisterAdminRoutes(mux, service, "")

			req := httptest.NewRequest(http.MethodPost, tc.path, bytes.NewReader([]byte(tc.body)))
			req.Header.Set("Content-Type", tc.ct)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if service.lastDirectoryGroupMembershipCreate.GroupID != "" {
				t.Fatalf("dispatched request %+v", service.lastDirectoryGroupMembershipCreate)
			}
		})
	}
}

func TestAdminDeleteDirectoryGroupMembershipHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		directoryGroupMembership: directory.GroupMembership{
			ID:         "membership-1",
			GroupID:    "group-1",
			CompanyID:  "company-1",
			MemberKind: directory.PrincipalKindUser,
			MemberID:   "user-1",
			Role:       directory.GroupMembershipRoleManager,
			Status:     "deleted",
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodDelete, "/admin/v1/directory/group-memberships/%20membership-1%20", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var response struct {
		Membership directory.GroupMembership `json:"directory_group_membership"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Membership.ID != "membership-1" || response.Membership.Status != "deleted" {
		t.Fatalf("directory_group_membership = %+v", response.Membership)
	}
	if service.lastDirectoryGroupMembershipDeleteID != "membership-1" {
		t.Fatalf("lastDirectoryGroupMembershipDeleteID = %q", service.lastDirectoryGroupMembershipDeleteID)
	}
}

func TestAdminDeleteDirectoryGroupMembershipHandlerRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
	}{
		{name: "bad path value", path: "/admin/v1/directory/group-memberships/membership%0Abad"},
		{name: "unknown query", path: "/admin/v1/directory/group-memberships/membership-1?dry_run=true"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			service := &fakeAdminService{}
			mux := http.NewServeMux()
			RegisterAdminRoutes(mux, service, "")

			req := httptest.NewRequest(http.MethodDelete, tc.path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if service.lastDirectoryGroupMembershipDeleteID != "" {
				t.Fatalf("dispatched id %q", service.lastDirectoryGroupMembershipDeleteID)
			}
		})
	}
}

func TestAdminDeleteDirectoryDelegationHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		directoryDelegation: directory.Delegation{
			ID:           "delegation-1",
			CompanyID:    "company-1",
			OwnerKind:    directory.PrincipalKindResource,
			OwnerID:      "room-1",
			DelegateKind: directory.PrincipalKindGroup,
			DelegateID:   "team-1",
			Scope:        directory.DelegationScopeCalendar,
			Role:         directory.DelegationRoleWrite,
			Status:       "deleted",
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodDelete, "/admin/v1/directory/delegations/%20delegation-1%20", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var response struct {
		Delegation directory.Delegation `json:"directory_delegation"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Delegation.ID != "delegation-1" || response.Delegation.Status != "deleted" {
		t.Fatalf("directory_delegation = %+v", response.Delegation)
	}
	if service.lastDirectoryDelegationDeleteID != "delegation-1" {
		t.Fatalf("lastDirectoryDelegationDeleteID = %q", service.lastDirectoryDelegationDeleteID)
	}
}

func TestAdminDeleteDirectoryDelegationHandlerRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	tests := []string{
		"/admin/v1/directory/delegations/delegation%0Abad",
		"/admin/v1/directory/delegations/delegation-1?dry_run=true",
	}
	for _, path := range tests {
		service := &fakeAdminService{}
		mux := http.NewServeMux()
		RegisterAdminRoutes(mux, service, "")

		req := httptest.NewRequest(http.MethodDelete, path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s status = %d, body = %s", path, rec.Code, rec.Body.String())
		}
		if service.lastDirectoryDelegationDeleteID != "" {
			t.Fatalf("%s dispatched delete id %q", path, service.lastDirectoryDelegationDeleteID)
		}
	}
}

func TestAdminCreateDirectoryAliasHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		directoryAlias: directory.Alias{
			ID:         "alias-1",
			CompanyID:  "company-1",
			DomainID:   "domain-1",
			Address:    "ops@example.com",
			AddressACE: "ops@example.com",
			TargetKind: directory.PrincipalKindGroup,
			TargetID:   "group-1",
			Status:     "active",
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	body := []byte(`{"company_id":"company-1","domain_id":"domain-1","address":"Ops@Example.COM","target_kind":"group","target_id":"group-1"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/v1/directory/aliases", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var response struct {
		Alias directory.Alias `json:"directory_alias"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Alias.ID != "alias-1" {
		t.Fatalf("directory_alias = %+v", response.Alias)
	}
	if service.lastDirectoryAliasCreate.CompanyID != "company-1" ||
		service.lastDirectoryAliasCreate.DomainID != "domain-1" ||
		service.lastDirectoryAliasCreate.Address != "Ops@Example.COM" ||
		service.lastDirectoryAliasCreate.TargetKind != directory.PrincipalKindGroup ||
		service.lastDirectoryAliasCreate.TargetID != "group-1" {
		t.Fatalf("lastDirectoryAliasCreate = %+v", service.lastDirectoryAliasCreate)
	}
}

func TestAdminCreateDirectoryAliasHandlerRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		body string
		ct   string
	}{
		{name: "unknown query", path: "/admin/v1/directory/aliases?dry_run=true", body: `{}`, ct: "application/json"},
		{name: "bad content type", path: "/admin/v1/directory/aliases", body: `{}`, ct: "text/plain"},
		{name: "unknown json field", path: "/admin/v1/directory/aliases", body: `{"company_id":"company-1","domain_id":"domain-1","address":"ops@example.com","target_kind":"group","target_id":"group-1","extra":true}`, ct: "application/json"},
		{name: "invalid address", path: "/admin/v1/directory/aliases", body: `{"company_id":"company-1","domain_id":"domain-1","address":"not-an-address","target_kind":"group","target_id":"group-1"}`, ct: "application/json"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			service := &fakeAdminService{}
			mux := http.NewServeMux()
			RegisterAdminRoutes(mux, service, "")

			req := httptest.NewRequest(http.MethodPost, tc.path, bytes.NewReader([]byte(tc.body)))
			req.Header.Set("Content-Type", tc.ct)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if service.lastDirectoryAliasCreate.CompanyID != "" {
				t.Fatalf("dispatched request %+v", service.lastDirectoryAliasCreate)
			}
		})
	}
}

func TestAdminDeleteDirectoryAliasHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		directoryAlias: directory.Alias{
			ID:         "alias-1",
			CompanyID:  "company-1",
			DomainID:   "domain-1",
			Address:    "ops@example.com",
			AddressACE: "ops@example.com",
			TargetKind: directory.PrincipalKindGroup,
			TargetID:   "group-1",
			Status:     "deleted",
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodDelete, "/admin/v1/directory/aliases/%20alias-1%20", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var response struct {
		Alias directory.Alias `json:"directory_alias"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Alias.ID != "alias-1" || response.Alias.Status != "deleted" {
		t.Fatalf("directory_alias = %+v", response.Alias)
	}
	if service.lastDirectoryAliasDeleteID != "alias-1" {
		t.Fatalf("lastDirectoryAliasDeleteID = %q", service.lastDirectoryAliasDeleteID)
	}
}

func TestAdminDeleteDirectoryAliasHandlerRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	tests := []string{
		"/admin/v1/directory/aliases/alias%0Abad",
		"/admin/v1/directory/aliases/alias-1?dry_run=true",
	}
	for _, path := range tests {
		service := &fakeAdminService{}
		mux := http.NewServeMux()
		RegisterAdminRoutes(mux, service, "")

		req := httptest.NewRequest(http.MethodDelete, path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s status = %d, body = %s", path, rec.Code, rec.Body.String())
		}
		if service.lastDirectoryAliasDeleteID != "" {
			t.Fatalf("%s dispatched delete id %q", path, service.lastDirectoryAliasDeleteID)
		}
	}
}

func TestAdminAuditLogIntegrityHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		auditLogIntegrity: maildb.AuditLogIntegrityView{
			CheckedCount: 2,
			Valid:        true,
			FirstID:      "audit-1",
			LastID:       "audit-2",
			Breaks:       []maildb.AuditLogIntegrityBreak{},
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/audit-logs/integrity?limit=25&since=2026-05-04T00:00:00Z", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Integrity maildb.AuditLogIntegrityView `json:"audit_log_integrity"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !body.Integrity.Valid || body.Integrity.CheckedCount != 2 {
		t.Fatalf("audit_log_integrity = %+v", body.Integrity)
	}
	if service.lastAuditLogIntegrity.Limit != 25 || service.lastAuditLogIntegrity.Since.IsZero() {
		t.Fatalf("lastAuditLogIntegrity = %+v", service.lastAuditLogIntegrity)
	}
	if service.lastAuditLogID != "" {
		t.Fatalf("integrity route dispatched detail id = %q", service.lastAuditLogID)
	}
}

func TestAdminAuditLogIntegrityHandlerRejectsInvalidSince(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/audit-logs/integrity?since=bad-time", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastAuditLogIntegrity.Limit != 0 {
		t.Fatalf("lastAuditLogIntegrity = %+v", service.lastAuditLogIntegrity)
	}
}

func TestAdminAuditLogDetailHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		auditLog: maildb.AuditLogView{
			ID:        "audit-1",
			Category:  "mail",
			Action:    "mail.received",
			Result:    "success",
			Detail:    json.RawMessage(`{"message_id":"msg-1"}`),
			CreatedAt: time.Date(2026, 5, 4, 9, 0, 0, 0, time.UTC),
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/audit-logs/audit-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Log maildb.AuditLogView `json:"audit_log"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Log.ID != "audit-1" || service.lastAuditLogID != "audit-1" {
		t.Fatalf("audit_log = %+v lastID=%q", body.Log, service.lastAuditLogID)
	}
}

func TestAdminAuditLogDetailHandlerRejectsUnsafeID(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/audit-logs/audit%0Abad", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastAuditLogID != "" {
		t.Fatalf("lastAuditLogID = %q", service.lastAuditLogID)
	}
}

func TestAdminBackpressureHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		backpressureState: backpressure.State{Level: "warning", Reason: "queue lag"},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/backpressure", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Backpressure backpressure.State `json:"backpressure"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if body.Backpressure.Level != "warning" || body.Backpressure.Reason != "queue lag" {
		t.Fatalf("backpressure = %+v", body.Backpressure)
	}
}

func TestAdminUpdateBackpressureHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodPatch, "/admin/v1/backpressure", strings.NewReader(`{
		"level": "danger",
		"reason": "queue lag above threshold"
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastBackpressureUpdate.Level != "danger" || service.lastBackpressureUpdate.Reason != "queue lag above threshold" {
		t.Fatalf("lastBackpressureUpdate = %+v", service.lastBackpressureUpdate)
	}
	if !strings.Contains(rec.Body.String(), `"backpressure"`) {
		t.Fatalf("response missing backpressure envelope: %s", rec.Body.String())
	}
}

func TestAdminQuotaUsageHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		quotaUsage: []maildb.QuotaUsageView{{
			Scope:            "domain",
			ID:               "domain-1",
			DomainID:         "domain-1",
			Name:             "example.com",
			QuotaUsed:        900,
			QuotaLimit:       1000,
			QuotaRemaining:   100,
			AllocatedQuota:   700,
			AllocatableQuota: 300,
			UsageRatio:       0.9,
			AllocationRatio:  0.7,
		}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/quota-usage?limit=5&scope=domain&domain_id=domain-1&over_limit=true&over_allocated=false", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		QuotaUsage []maildb.QuotaUsageView `json:"quota_usage"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if len(body.QuotaUsage) != 1 || body.QuotaUsage[0].Name != "example.com" {
		t.Fatalf("quota_usage = %+v", body.QuotaUsage)
	}
	if body.QuotaUsage[0].QuotaRemaining != 100 || body.QuotaUsage[0].AllocatableQuota != 300 {
		t.Fatalf("quota capacity fields = %+v", body.QuotaUsage[0])
	}
	if service.lastLimit != 5 {
		t.Fatalf("lastLimit = %d, want 5", service.lastLimit)
	}
	if service.lastQuotaUsageList.Scope != "domain" ||
		service.lastQuotaUsageList.DomainID != "domain-1" ||
		service.lastQuotaUsageList.OverLimit == nil ||
		!*service.lastQuotaUsageList.OverLimit ||
		service.lastQuotaUsageList.OverAllocated == nil ||
		*service.lastQuotaUsageList.OverAllocated {
		t.Fatalf("lastQuotaUsageList = %+v", service.lastQuotaUsageList)
	}
}

func TestAdminQuotaUsageHandlerRejectsUnsafeFilters(t *testing.T) {
	t.Parallel()

	tests := []string{
		"/admin/v1/quota-usage?scope=domain%0Abad",
		"/admin/v1/quota-usage?domain_id=" + strings.Repeat("d", maxAdminQueryFilterBytes+1),
		"/admin/v1/quota-usage?over_limit=maybe",
	}
	for _, path := range tests {
		service := &fakeAdminService{}
		mux := http.NewServeMux()
		RegisterAdminRoutes(mux, service, "")

		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s status = %d, body = %s", path, rec.Code, rec.Body.String())
		}
		if service.lastQuotaUsageList.Limit != 0 {
			t.Fatalf("%s dispatched request %+v", path, service.lastQuotaUsageList)
		}
	}
}

func TestAdminAttachmentUploadSessionsHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		attachmentUploadSessions: []maildb.AttachmentUploadSession{{
			ID:           "session-1",
			UserID:       "user-1",
			DraftID:      "draft-1",
			Filename:     "large.bin",
			DeclaredSize: 42,
			MIMEType:     "application/octet-stream",
			Status:       "uploading",
			ExpiresAt:    time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC),
			CreatedAt:    time.Date(2026, 5, 4, 11, 0, 0, 0, time.UTC),
			UpdatedAt:    time.Date(2026, 5, 4, 11, 30, 0, 0, time.UTC),
		}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/attachment-upload-sessions?limit=5&user_id=%20user-1%20&draft_id=%20draft-1%20&status=uploading", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Sessions []maildb.AttachmentUploadSession `json:"attachment_upload_sessions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if len(body.Sessions) != 1 || body.Sessions[0].ID != "session-1" {
		t.Fatalf("sessions = %+v", body.Sessions)
	}
	if service.lastAttachmentUploadSessionList.Limit != 5 ||
		service.lastAttachmentUploadSessionList.UserID != "user-1" ||
		service.lastAttachmentUploadSessionList.DraftID != "draft-1" ||
		service.lastAttachmentUploadSessionList.Status != "uploading" {
		t.Fatalf("lastAttachmentUploadSessionList = %+v", service.lastAttachmentUploadSessionList)
	}
}

func TestAdminDriveUploadSessionsHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		driveUploadSessions: []drive.UploadSession{{
			ID:             "session-1",
			UserID:         "user-1",
			UploadID:       "upload-1",
			Name:           "Report.pdf",
			Status:         drive.UploadSessionStatusUploading,
			StorageBackend: "s3",
		}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/drive-upload-sessions?limit=5&user_id=%20user-1%20&status=uploading", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Sessions []drive.UploadSession `json:"drive_upload_sessions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if len(body.Sessions) != 1 || body.Sessions[0].ID != "session-1" {
		t.Fatalf("sessions = %+v", body.Sessions)
	}
	if service.lastDriveUploadSessionList.Limit != 5 ||
		service.lastDriveUploadSessionList.UserID != "user-1" ||
		service.lastDriveUploadSessionList.Status != drive.UploadSessionStatusUploading {
		t.Fatalf("lastDriveUploadSessionList = %+v", service.lastDriveUploadSessionList)
	}
}

func TestAdminDriveNodesHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		driveNodes: []drive.Node{{
			ID:       "node-1",
			UserID:   "user-1",
			ParentID: "parent-1",
			Name:     "Reports",
			Type:     drive.NodeTypeFolder,
			Status:   drive.NodeStatusActive,
		}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/drive-nodes?limit=5&user_id=%20user-1%20&status=active&node_type=folder&q=%20Report%20&sort=updated&all_parents=true", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Nodes []drive.Node `json:"drive_nodes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if len(body.Nodes) != 1 || body.Nodes[0].ID != "node-1" {
		t.Fatalf("nodes = %+v", body.Nodes)
	}
	if service.lastDriveNodeList.Limit != 5 ||
		service.lastDriveNodeList.UserID != "user-1" ||
		service.lastDriveNodeList.ParentID != "" ||
		service.lastDriveNodeList.Status != drive.NodeStatusActive ||
		service.lastDriveNodeList.NodeType != drive.NodeTypeFolder ||
		service.lastDriveNodeList.Query != "report" ||
		service.lastDriveNodeList.Sort != drive.NodeSortUpdated ||
		!service.lastDriveNodeList.AllParents {
		t.Fatalf("lastDriveNodeList = %+v", service.lastDriveNodeList)
	}
}

func TestAdminDriveNodesHandlerRejectsUnsafeFilters(t *testing.T) {
	t.Parallel()

	tests := []string{
		"/admin/v1/drive-nodes?user_id=user%0Abad",
		"/admin/v1/drive-nodes?status=missing&user_id=user-1",
		"/admin/v1/drive-nodes?node_type=shortcut&user_id=user-1",
		"/admin/v1/drive-nodes?q=report%0Abad&user_id=user-1",
		"/admin/v1/drive-nodes?sort=owner&user_id=user-1",
		"/admin/v1/drive-nodes?all_parents=maybe&user_id=user-1",
		"/admin/v1/drive-nodes?all_parents=true&parent_id=parent-1&user_id=user-1",
		"/admin/v1/drive-nodes?cursor=opaque&user_id=user-1",
		"/admin/v1/drive-nodes",
	}
	for _, path := range tests {
		path := path
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			service := &fakeAdminService{}
			mux := http.NewServeMux()
			RegisterAdminRoutes(mux, service, "")

			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if service.lastDriveNodeList.Limit != 0 {
				t.Fatalf("list was called: %+v", service.lastDriveNodeList)
			}
		})
	}
}

func TestAdminDriveNodeHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		driveNode: drive.Node{
			ID:     "node-1",
			UserID: "user-1",
			Name:   "Report.pdf",
			Type:   drive.NodeTypeFile,
			Status: drive.NodeStatusActive,
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/drive-nodes/node-1?user_id=%20user-1%20&status=active", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Node drive.Node `json:"drive_node"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if body.Node.ID != "node-1" {
		t.Fatalf("node = %+v", body.Node)
	}
	if service.lastDriveNodeGet.UserID != "user-1" || service.lastDriveNodeGet.NodeID != "node-1" || service.lastDriveNodeGet.Status != drive.NodeStatusActive {
		t.Fatalf("lastDriveNodeGet = %+v", service.lastDriveNodeGet)
	}
}

func TestAdminDriveNodeHandlerRejectsUnsafeFilters(t *testing.T) {
	t.Parallel()

	tests := []string{
		"/admin/v1/drive-nodes/node-1?user_id=user%0Abad",
		"/admin/v1/drive-nodes/node-1?user_id=user-1&status=missing",
		"/admin/v1/drive-nodes/node%0A1?user_id=user-1",
		"/admin/v1/drive-nodes/node-1?user_id=user-1&cursor=opaque",
		"/admin/v1/drive-nodes/node-1",
	}
	for _, path := range tests {
		path := path
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			service := &fakeAdminService{}
			mux := http.NewServeMux()
			RegisterAdminRoutes(mux, service, "")

			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if service.lastDriveNodeGet.NodeID != "" {
				t.Fatalf("get was called: %+v", service.lastDriveNodeGet)
			}
		})
	}
}

func TestAdminDriveUsageHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		driveUsageSummary: drive.UsageSummary{
			UserID:                "user-1",
			QuotaUsed:             2048,
			QuotaLimit:            4096,
			ActiveNodes:           3,
			FileCount:             2,
			ActiveBytes:           2048,
			PendingUploadSessions: 1,
			PendingUploadBytes:    512,
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/drive-usage?user_id=%20user-1%20", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Summary drive.UsageSummary `json:"drive_usage_summary"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if body.Summary.UserID != "user-1" || body.Summary.ActiveBytes != 2048 || body.Summary.PendingUploadSessions != 1 {
		t.Fatalf("summary = %+v", body.Summary)
	}
	if service.lastDriveUsage.UserID != "user-1" {
		t.Fatalf("lastDriveUsage = %+v", service.lastDriveUsage)
	}
}

func TestAdminDriveUsageHandlerRejectsUnsafeFilters(t *testing.T) {
	t.Parallel()

	tests := []string{
		"/admin/v1/drive-usage?user_id=user%0Abad",
		"/admin/v1/drive-usage?user_id=user-1&cursor=opaque",
		"/admin/v1/drive-usage",
	}
	for _, path := range tests {
		path := path
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			service := &fakeAdminService{}
			mux := http.NewServeMux()
			RegisterAdminRoutes(mux, service, "")

			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if service.lastDriveUsage.UserID != "" {
				t.Fatalf("usage was called: %+v", service.lastDriveUsage)
			}
		})
	}
}

func TestAdminDriveUploadSessionsHandlerRejectsUnsafeFilters(t *testing.T) {
	t.Parallel()

	tests := []string{
		"/admin/v1/drive-upload-sessions?user_id=user%0Abad",
		"/admin/v1/drive-upload-sessions?status=ready&user_id=user-1",
		"/admin/v1/drive-upload-sessions?cursor=opaque&user_id=user-1",
		"/admin/v1/drive-upload-sessions",
	}
	for _, path := range tests {
		path := path
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			service := &fakeAdminService{}
			mux := http.NewServeMux()
			RegisterAdminRoutes(mux, service, "")

			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if service.lastDriveUploadSessionList.UserID != "" {
				t.Fatalf("list was called: %+v", service.lastDriveUploadSessionList)
			}
		})
	}
}

func TestAdminDriveUploadCleanupCandidatesHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		staleDriveUploadSessionCount: drive.StaleUploadSessionCount{TotalCount: 3, LimitedCount: 2},
		staleDriveUploadSessions: []drive.UploadSession{{
			ID:             "session-1",
			UserID:         "user-1",
			UploadID:       "upload-1",
			Name:           "Report.pdf",
			Status:         drive.UploadSessionStatusUploading,
			StorageBackend: "s3",
		}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodPost, "/admin/v1/drive-upload-cleanup/candidates", strings.NewReader(`{
		"before":"2020-05-06T12:00:00Z",
		"limit":25
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{`"drive_upload_cleanup_candidates"`, `"session_candidates"`, `"id":"session-1"`, `"session_candidate_count":3`, `"session_limited_count":2`, `"limit":25`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("body = %s, want %s", rec.Body.String(), want)
		}
	}
	if !service.lastDriveUploadCleanupBefore.Equal(time.Date(2020, 5, 6, 12, 0, 0, 0, time.UTC)) || service.lastDriveUploadCleanupLimit != 25 {
		t.Fatalf("cleanup request = %s/%d", service.lastDriveUploadCleanupBefore, service.lastDriveUploadCleanupLimit)
	}
}

func TestAdminDriveUploadCleanupRunHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		staleDriveUploadSessionCount: drive.StaleUploadSessionCount{TotalCount: 2, LimitedCount: 2},
		expiredDriveUploadSessions: []drive.UploadSession{{
			ID:             "session-1",
			UserID:         "user-1",
			UploadID:       "upload-1",
			Name:           "Report.pdf",
			Status:         drive.UploadSessionStatusExpired,
			StorageBackend: "s3",
		}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodPost, "/admin/v1/drive-upload-cleanup/runs", strings.NewReader(`{
		"before":"2020-05-06T12:00:00Z",
		"limit":25
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{`"drive_upload_cleanup_run"`, `"expired_sessions"`, `"id":"session-1"`, `"session_candidate_count":2`, `"expired_session_count":1`, `"limit":25`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("body = %s, want %s", rec.Body.String(), want)
		}
	}
	if !service.lastDriveUploadCleanupBefore.Equal(time.Date(2020, 5, 6, 12, 0, 0, 0, time.UTC)) || service.lastDriveUploadCleanupLimit != 25 {
		t.Fatalf("cleanup request = %s/%d", service.lastDriveUploadCleanupBefore, service.lastDriveUploadCleanupLimit)
	}
}

func TestAdminDriveCleanupFailuresHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		driveCleanupFailures: []drive.ObjectCleanupFailure{{
			ID:             "failure-1",
			UserID:         "user-1",
			StorageBackend: "s3",
			StoragePath:    "drive/users/user-1/files/node-1/body",
			Status:         drive.ObjectCleanupFailureStatusPending,
			Attempts:       2,
			LastError:      "delete failed",
		}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/drive-cleanup-failures?limit=5&user_id=%20user-1%20&status=pending", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Failures []drive.ObjectCleanupFailure `json:"drive_cleanup_failures"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if len(body.Failures) != 1 || body.Failures[0].ID != "failure-1" {
		t.Fatalf("failures = %+v", body.Failures)
	}
	if service.lastDriveCleanupFailureList.UserID != "user-1" ||
		service.lastDriveCleanupFailureList.Status != drive.ObjectCleanupFailureStatusPending ||
		service.lastDriveCleanupFailureList.Limit != 5 {
		t.Fatalf("lastDriveCleanupFailureList = %+v", service.lastDriveCleanupFailureList)
	}
}

func TestAdminDriveCleanupFailuresHandlerRejectsUnsafeFilters(t *testing.T) {
	t.Parallel()

	tests := []string{
		"/admin/v1/drive-cleanup-failures?user_id=user%0Abad",
		"/admin/v1/drive-cleanup-failures?status=closed",
		"/admin/v1/drive-cleanup-failures?cursor=opaque",
	}
	for _, path := range tests {
		path := path
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			service := &fakeAdminService{}
			mux := http.NewServeMux()
			RegisterAdminRoutes(mux, service, "")

			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if service.lastDriveCleanupFailureList.Limit != 0 {
				t.Fatalf("list was called: %+v", service.lastDriveCleanupFailureList)
			}
		})
	}
}

func TestAdminResolveDriveCleanupFailureHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		resolvedDriveCleanupFailure: drive.ObjectCleanupFailure{
			ID:             "failure-1",
			UserID:         "user-1",
			StorageBackend: "s3",
			StoragePath:    "drive/users/user-1/files/node-1/body",
			Status:         drive.ObjectCleanupFailureStatusResolved,
			Attempts:       2,
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodPost, "/admin/v1/drive-cleanup-failures/failure-1/resolve", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Failure drive.ObjectCleanupFailure `json:"drive_cleanup_failure"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if body.Failure.ID != "failure-1" || body.Failure.Status != drive.ObjectCleanupFailureStatusResolved {
		t.Fatalf("failure = %+v", body.Failure)
	}
	if service.lastResolveDriveCleanupFailureID != "failure-1" {
		t.Fatalf("lastResolveDriveCleanupFailureID = %q", service.lastResolveDriveCleanupFailureID)
	}
}

func TestAdminRetryDriveCleanupFailuresHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		driveCleanupRetryResult: drive.RetryObjectCleanupFailuresResult{Scanned: 3, Deleted: 2, Resolved: 2, Failed: 1},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodPost, "/admin/v1/drive-cleanup-failures/retry-runs", strings.NewReader(`{
		"user_id":" user-1 ",
		"limit":5
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{`"drive_cleanup_retry_run"`, `"user_id":"user-1"`, `"limit":5`, `"scanned":3`, `"deleted":2`, `"resolved":2`, `"failed":1`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("body = %s, want %s", rec.Body.String(), want)
		}
	}
	if service.lastDriveCleanupFailureRetry.UserID != "user-1" ||
		service.lastDriveCleanupFailureRetry.Status != drive.ObjectCleanupFailureStatusPending ||
		service.lastDriveCleanupFailureRetry.Limit != 5 {
		t.Fatalf("lastDriveCleanupFailureRetry = %+v", service.lastDriveCleanupFailureRetry)
	}
}

func TestAdminRetryDriveCleanupFailuresHandlerRejectsUnsafeRequests(t *testing.T) {
	t.Parallel()

	tests := []string{
		"{\"user_id\":\"user\\nbad\"}",
	}
	for _, body := range tests {
		body := body
		t.Run(body, func(t *testing.T) {
			t.Parallel()

			service := &fakeAdminService{}
			mux := http.NewServeMux()
			RegisterAdminRoutes(mux, service, "")

			req := httptest.NewRequest(http.MethodPost, "/admin/v1/drive-cleanup-failures/retry-runs", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if service.lastDriveCleanupFailureRetry.Limit != 0 {
				t.Fatalf("retry was called: %+v", service.lastDriveCleanupFailureRetry)
			}
		})
	}
}

func TestAdminAttachmentUploadSessionsHandlerRejectsUnsafeFilters(t *testing.T) {
	t.Parallel()

	tests := []string{
		"/admin/v1/attachment-upload-sessions?user_id=user%0Abad",
		"/admin/v1/attachment-upload-sessions?draft_id=" + strings.Repeat("d", maxAdminQueryFilterBytes+1),
		"/admin/v1/attachment-upload-sessions?status=ready",
	}
	for _, path := range tests {
		path := path
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			service := &fakeAdminService{}
			mux := http.NewServeMux()
			RegisterAdminRoutes(mux, service, "")

			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if service.lastAttachmentUploadSessionList.Limit != 0 {
				t.Fatalf("list was called: %+v", service.lastAttachmentUploadSessionList)
			}
		})
	}
}

func TestAdminAttachmentCleanupRunHandler(t *testing.T) {
	t.Parallel()

	before := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)
	service := &fakeAdminService{
		expiredAttachments: []maildb.Attachment{{ID: "att-1"}, {ID: "att-2"}},
		expiredAttachmentSessions: []maildb.AttachmentUploadSession{
			{ID: "session-1"},
			{ID: "session-2"},
			{ID: "session-3"},
		},
		staleAttachmentCount: maildb.StaleAttachmentUploadCount{
			TotalCount:   5,
			LimitedCount: 2,
		},
		staleAttachmentSessionCount: maildb.StaleAttachmentUploadSessionCount{
			TotalCount:   8,
			LimitedCount: 3,
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodPost, "/admin/v1/attachment-cleanup/runs", strings.NewReader(`{
		"before": "`+before.Format(time.RFC3339)+`",
		"limit": 25
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !service.lastAttachmentCleanupBefore.Equal(before) || service.lastAttachmentCleanupLimit != 25 {
		t.Fatalf("cleanup request = %s/%d", service.lastAttachmentCleanupBefore, service.lastAttachmentCleanupLimit)
	}
	if !service.lastAttachmentSessionCleanupBefore.Equal(before) || service.lastAttachmentSessionCleanupLimit != 25 {
		t.Fatalf("session cleanup request = %s/%d", service.lastAttachmentSessionCleanupBefore, service.lastAttachmentSessionCleanupLimit)
	}
	if !strings.Contains(rec.Body.String(), `"attachment_cleanup_run"`) || !strings.Contains(rec.Body.String(), `"expired_count":2`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"candidate_count":5`) || !strings.Contains(rec.Body.String(), `"limited_count":2`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
	for _, want := range []string{`"session_candidate_count":8`, `"session_limited_count":3`, `"expired_session_count":3`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("body missing %s: %s", want, rec.Body.String())
		}
	}
}

func TestAdminAttachmentCleanupRunHandlerSupportsDryRun(t *testing.T) {
	t.Parallel()

	before := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)
	service := &fakeAdminService{
		staleAttachmentCount: maildb.StaleAttachmentUploadCount{
			TotalCount:   7,
			LimitedCount: 3,
		},
		staleAttachmentSessionCount: maildb.StaleAttachmentUploadSessionCount{
			TotalCount:   11,
			LimitedCount: 3,
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodPost, "/admin/v1/attachment-cleanup/runs", strings.NewReader(`{
		"before": "`+before.Format(time.RFC3339)+`",
		"limit": 3,
		"dry_run": true
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !service.lastAttachmentCleanupBefore.IsZero() {
		t.Fatalf("cleanup dispatched for dry run at %s", service.lastAttachmentCleanupBefore)
	}
	if !service.lastAttachmentSessionCleanupBefore.IsZero() {
		t.Fatalf("session cleanup dispatched for dry run at %s", service.lastAttachmentSessionCleanupBefore)
	}
	if !service.lastAttachmentCleanupCountBefore.Equal(before) || service.lastAttachmentCleanupCountLimit != 3 {
		t.Fatalf("count request = %s/%d", service.lastAttachmentCleanupCountBefore, service.lastAttachmentCleanupCountLimit)
	}
	if !service.lastAttachmentSessionCleanupCountBefore.Equal(before) || service.lastAttachmentSessionCleanupCountLimit != 3 {
		t.Fatalf("session count request = %s/%d", service.lastAttachmentSessionCleanupCountBefore, service.lastAttachmentSessionCleanupCountLimit)
	}
	for _, want := range []string{`"dry_run":true`, `"candidate_count":7`, `"limited_count":3`, `"expired_count":0`, `"session_candidate_count":11`, `"session_limited_count":3`, `"expired_session_count":0`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("body missing %s: %s", want, rec.Body.String())
		}
	}
}

func TestAdminAttachmentCleanupCandidatesHandler(t *testing.T) {
	t.Parallel()

	before := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)
	service := &fakeAdminService{
		staleAttachmentCount: maildb.StaleAttachmentUploadCount{
			TotalCount:   9,
			LimitedCount: 1,
		},
		staleAttachmentSessionCount: maildb.StaleAttachmentUploadSessionCount{
			TotalCount:   4,
			LimitedCount: 1,
		},
		staleAttachmentCandidates: []maildb.StaleAttachmentUploadCandidate{{
			ID:        "att-1",
			UserID:    "user-1",
			UploadID:  "upload-1",
			Filename:  "report.pdf",
			Size:      42,
			MIMEType:  "application/pdf",
			Status:    "uploading",
			CreatedAt: before.Add(-time.Hour),
		}},
		staleAttachmentSessionCandidates: []maildb.StaleAttachmentUploadSessionCandidate{{
			ID:             "session-1",
			UserID:         "user-1",
			UploadID:       "upload-session-1",
			Filename:       "large.bin",
			DeclaredSize:   1024,
			ReceivedSize:   512,
			MIMEType:       "application/octet-stream",
			Status:         "uploading",
			StorageBackend: "local",
			StoragePath:    "upload-sessions/user-1/session-1/body",
			ExpiresAt:      before.Add(-time.Minute),
			CreatedAt:      before.Add(-time.Hour),
		}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodPost, "/admin/v1/attachment-cleanup/candidates", strings.NewReader(`{
		"before": "`+before.Format(time.RFC3339)+`",
		"limit": 25
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !service.lastAttachmentCleanupListBefore.Equal(before) || service.lastAttachmentCleanupListLimit != 25 {
		t.Fatalf("list request = %s/%d", service.lastAttachmentCleanupListBefore, service.lastAttachmentCleanupListLimit)
	}
	if !service.lastAttachmentCleanupCountBefore.Equal(before) || service.lastAttachmentCleanupCountLimit != 25 {
		t.Fatalf("count request = %s/%d", service.lastAttachmentCleanupCountBefore, service.lastAttachmentCleanupCountLimit)
	}
	if !service.lastAttachmentSessionCleanupListBefore.Equal(before) || service.lastAttachmentSessionCleanupListLimit != 25 {
		t.Fatalf("session list request = %s/%d", service.lastAttachmentSessionCleanupListBefore, service.lastAttachmentSessionCleanupListLimit)
	}
	if !service.lastAttachmentSessionCleanupCountBefore.Equal(before) || service.lastAttachmentSessionCleanupCountLimit != 25 {
		t.Fatalf("session count request = %s/%d", service.lastAttachmentSessionCleanupCountBefore, service.lastAttachmentSessionCleanupCountLimit)
	}
	for _, want := range []string{`"attachment_cleanup_candidates"`, `"candidates"`, `"id":"att-1"`, `"user_id":"user-1"`, `"candidate_count":9`, `"limited_count":1`, `"session_candidates"`, `"id":"session-1"`, `"session_candidate_count":4`, `"session_limited_count":1`, `"limit":25`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("body missing %s: %s", want, rec.Body.String())
		}
	}
}

func TestAdminAttachmentCleanupRunHandlerRejectsUnsafeRequests(t *testing.T) {
	t.Parallel()

	tests := []string{
		`{"limit":25}`,
		`{"before":"not-a-time"}`,
		`{"before":"` + time.Now().UTC().Add(time.Hour).Format(time.RFC3339) + `"}`,
		`{"before":"2026-05-04T12:00:00Z","limit":-1}`,
	}
	for _, body := range tests {
		body := body
		t.Run(body, func(t *testing.T) {
			t.Parallel()

			service := &fakeAdminService{}
			mux := http.NewServeMux()
			RegisterAdminRoutes(mux, service, "")

			req := httptest.NewRequest(http.MethodPost, "/admin/v1/attachment-cleanup/runs", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if !service.lastAttachmentCleanupBefore.IsZero() {
				t.Fatalf("cleanup dispatched for %s", body)
			}
		})
	}
}

func TestAdminAPIUsageDailyHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		apiUsageDaily: []maildb.APIUsageDailyView{{
			Day:              time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC),
			Method:           "GET",
			Route:            "GET /api/v1/messages",
			Status:           200,
			TenantID:         "tenant-1",
			CompanyID:        "company-1",
			DomainID:         "domain-1",
			UserID:           "user-1",
			APIKeyID:         "api-key-1",
			PrincipalID:      "principal-1",
			AuthSource:       "bearer",
			RequestCount:     4,
			RequestBytes:     40,
			ResponseBytes:    400,
			LatencyMSTotal:   100,
			LatencyMSMax:     40,
			LatencyMSAverage: 25,
			FirstSeenAt:      time.Date(2026, 5, 4, 1, 0, 0, 0, time.UTC),
			LastSeenAt:       time.Date(2026, 5, 4, 2, 0, 0, 0, time.UTC),
		}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/api-usage/daily?limit=5&tenant_id=%20tenant-1%20&company_id=company-1&domain_id=domain-1&user_id=user-1&api_key_id=api-key-1&principal_id=principal-1&auth_source=bearer&method=GET&route=GET%20/api/v1/messages&status=200&from=2026-05-04T00:00:00Z&to=2026-05-05T00:00:00Z", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var body struct {
		APIUsageDaily []maildb.APIUsageDailyView `json:"api_usage_daily"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.APIUsageDaily) != 1 || body.APIUsageDaily[0].LatencyMSAverage != 25 {
		t.Fatalf("api_usage_daily = %+v", body.APIUsageDaily)
	}
	if body.APIUsageDaily[0].TenantID != "tenant-1" || body.APIUsageDaily[0].PrincipalID != "principal-1" {
		t.Fatalf("api_usage_daily identity = %+v", body.APIUsageDaily[0])
	}
	if service.lastLimit != 5 {
		t.Fatalf("lastLimit = %d, want 5", service.lastLimit)
	}
	if service.lastAPIUsageDailyList.TenantID != "tenant-1" ||
		service.lastAPIUsageDailyList.CompanyID != "company-1" ||
		service.lastAPIUsageDailyList.DomainID != "domain-1" ||
		service.lastAPIUsageDailyList.UserID != "user-1" ||
		service.lastAPIUsageDailyList.APIKeyID != "api-key-1" ||
		service.lastAPIUsageDailyList.PrincipalID != "principal-1" ||
		service.lastAPIUsageDailyList.AuthSource != "bearer" ||
		service.lastAPIUsageDailyList.Method != "GET" ||
		service.lastAPIUsageDailyList.Route != "GET /api/v1/messages" ||
		service.lastAPIUsageDailyList.Status != 200 ||
		service.lastAPIUsageDailyList.From.IsZero() ||
		service.lastAPIUsageDailyList.To.IsZero() {
		t.Fatalf("lastAPIUsageDailyList = %+v", service.lastAPIUsageDailyList)
	}
}

func TestAdminAPIUsageDailyHandlerRejectsUnsafeFilters(t *testing.T) {
	t.Parallel()

	tests := []string{
		"/admin/v1/api-usage/daily?tenant_id=tenant%0Abad",
		"/admin/v1/api-usage/daily?principal_id=" + strings.Repeat("p", maxAdminQueryFilterBytes+1),
		"/admin/v1/api-usage/daily?status=9999",
		"/admin/v1/api-usage/daily?from=2026-05-05T00:00:00Z&to=2026-05-04T00:00:00Z",
	}
	for _, path := range tests {
		service := &fakeAdminService{}
		mux := http.NewServeMux()
		RegisterAdminRoutes(mux, service, "")

		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s status = %d, body = %s", path, rec.Code, rec.Body.String())
		}
		if service.lastAPIUsageDailyList.Limit != 0 {
			t.Fatalf("%s dispatched request %+v", path, service.lastAPIUsageDailyList)
		}
	}
}

func TestAdminAPIUsageMonthlyHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		apiUsageMonthly: []maildb.APIUsageMonthlyView{{
			Month:            time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
			Method:           "GET",
			Route:            "GET /api/v1/messages",
			Status:           200,
			TenantID:         "tenant-1",
			PrincipalID:      "principal-1",
			AuthSource:       "bearer",
			RequestCount:     4,
			LatencyMSTotal:   100,
			LatencyMSAverage: 25,
		}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/api-usage/monthly?limit=5&tenant_id=tenant-1&principal_id=principal-1&auth_source=bearer&method=GET&route=GET%20/api/v1/messages&status=200&from=2026-05-01T00:00:00Z&to=2026-06-01T00:00:00Z", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var body struct {
		APIUsageMonthly []maildb.APIUsageMonthlyView `json:"api_usage_monthly"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.APIUsageMonthly) != 1 || body.APIUsageMonthly[0].LatencyMSAverage != 25 {
		t.Fatalf("api_usage_monthly = %+v", body.APIUsageMonthly)
	}
	if body.APIUsageMonthly[0].TenantID != "tenant-1" || body.APIUsageMonthly[0].PrincipalID != "principal-1" {
		t.Fatalf("api_usage_monthly identity = %+v", body.APIUsageMonthly[0])
	}
	if service.lastAPIUsageMonthlyList.Limit != 5 ||
		service.lastAPIUsageMonthlyList.TenantID != "tenant-1" ||
		service.lastAPIUsageMonthlyList.PrincipalID != "principal-1" ||
		service.lastAPIUsageMonthlyList.AuthSource != "bearer" ||
		service.lastAPIUsageMonthlyList.Method != "GET" ||
		service.lastAPIUsageMonthlyList.Route != "GET /api/v1/messages" ||
		service.lastAPIUsageMonthlyList.Status != 200 ||
		service.lastAPIUsageMonthlyList.From.IsZero() ||
		service.lastAPIUsageMonthlyList.To.IsZero() {
		t.Fatalf("lastAPIUsageMonthlyList = %+v", service.lastAPIUsageMonthlyList)
	}
}

func TestAdminAPIUsageLedgerHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		apiUsageLedger: []maildb.APIUsageLedgerView{{
			EventID:       "usage-1",
			SchemaVersion: "2026-05-04.api-usage.v2",
			EventTime:     time.Date(2026, 5, 4, 1, 0, 0, 0, time.UTC),
			RecordedAt:    time.Date(2026, 5, 4, 1, 0, 1, 0, time.UTC),
			Method:        "GET",
			Route:         "GET /api/v1/messages",
			Status:        200,
			TenantID:      "tenant-1",
			PrincipalID:   "principal-1",
			AuthSource:    "bearer",
			RequestCount:  1,
			Payload:       json.RawMessage(`{"event":"api.usage"}`),
		}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/api-usage/ledger?limit=5&tenant_id=%20tenant-1%20&principal_id=%20principal-1%20&from=2026-05-04T00:00:00Z&to=2026-05-05T00:00:00Z", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var body struct {
		APIUsageLedger []maildb.APIUsageLedgerView `json:"api_usage_ledger"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.APIUsageLedger) != 1 || body.APIUsageLedger[0].EventID != "usage-1" {
		t.Fatalf("api_usage_ledger = %+v", body.APIUsageLedger)
	}
	if body.APIUsageLedger[0].TenantID != "tenant-1" || body.APIUsageLedger[0].PrincipalID != "principal-1" {
		t.Fatalf("api_usage_ledger identity = %+v", body.APIUsageLedger[0])
	}
	if service.lastLimit != 5 {
		t.Fatalf("lastLimit = %d, want 5", service.lastLimit)
	}
	if service.lastAPIUsageLedgerList.TenantID != "tenant-1" || service.lastAPIUsageLedgerList.PrincipalID != "principal-1" {
		t.Fatalf("lastAPIUsageLedgerList = %+v", service.lastAPIUsageLedgerList)
	}
	if service.lastAPIUsageLedgerList.From.IsZero() || service.lastAPIUsageLedgerList.To.IsZero() {
		t.Fatalf("lastAPIUsageLedgerList timestamps = %+v", service.lastAPIUsageLedgerList)
	}
}

func TestAdminAPIUsageLedgerRejectsInvalidTimeRange(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, &fakeAdminService{}, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/api-usage/ledger?from=2026-05-05T00:00:00Z&to=2026-05-04T00:00:00Z", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestAdminAPIUsageLedgerRejectsUnsafeIdentityFilters(t *testing.T) {
	t.Parallel()

	oversizedPrincipalID := strings.Repeat("p", maxAdminQueryFilterBytes+1)
	tests := []struct {
		name   string
		method string
		path   string
	}{
		{
			name:   "tenant crlf",
			method: http.MethodGet,
			path:   "/admin/v1/api-usage/ledger?tenant_id=tenant%0Abad",
		},
		{
			name:   "export principal oversized",
			method: http.MethodGet,
			path:   "/admin/v1/api-usage/ledger/export?principal_id=" + oversizedPrincipalID,
		},
		{
			name:   "stats tenant crlf",
			method: http.MethodGet,
			path:   "/admin/v1/api-usage/ledger/stats?tenant_id=tenant%0Dbad",
		},
		{
			name:   "export batch principal oversized",
			method: http.MethodPost,
			path:   "/admin/v1/api-usage/export-batches?principal_id=" + oversizedPrincipalID + "&from=2026-05-04T00:00:00Z&to=2026-05-05T00:00:00Z",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := &fakeAdminService{}
			mux := http.NewServeMux()
			RegisterAdminRoutes(mux, service, "")

			req := httptest.NewRequest(tt.method, tt.path, nil)
			rr := httptest.NewRecorder()
			mux.ServeHTTP(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
			}
			if service.lastAPIUsageLedgerList.TenantID != "" || service.lastAPIUsageLedgerList.PrincipalID != "" {
				t.Fatalf("lastAPIUsageLedgerList = %+v", service.lastAPIUsageLedgerList)
			}
		})
	}
}

func TestAdminAPIUsageLedgerExportHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		apiUsageLedger: []maildb.APIUsageLedgerView{{
			EventID:       "usage-1",
			SchemaVersion: "2026-05-04.api-usage.v2",
			EventTime:     time.Date(2026, 5, 4, 1, 0, 0, 0, time.UTC),
			RecordedAt:    time.Date(2026, 5, 4, 1, 0, 1, 0, time.UTC),
			Method:        "GET",
			Route:         "GET /api/v1/messages",
			Status:        200,
			RequestCount:  1,
			Payload:       json.RawMessage(`{"event":"api.usage"}`),
		}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/api-usage/ledger/export?limit=5", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Content-Type"); got != "application/x-ndjson" {
		t.Fatalf("content type = %q", got)
	}
	if got := rr.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("cache control = %q", got)
	}
	if got := rr.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("x-content-type-options = %q", got)
	}
	lines := strings.Split(strings.TrimSpace(rr.Body.String()), "\n")
	if len(lines) != 1 || !strings.Contains(lines[0], `"event_id":"usage-1"`) {
		t.Fatalf("ndjson = %q", rr.Body.String())
	}
}

func TestAdminAPIUsageLedgerStatsHandler(t *testing.T) {
	t.Parallel()

	first := time.Date(2026, 5, 4, 1, 0, 0, 0, time.UTC)
	last := time.Date(2026, 5, 4, 2, 0, 0, 0, time.UTC)
	service := &fakeAdminService{
		apiUsageLedgerStats: maildb.APIUsageLedgerStatsView{
			EventCount:       2,
			RequestCount:     4,
			RequestBytes:     40,
			ResponseBytes:    400,
			LatencyMSTotal:   100,
			LatencyMSMax:     40,
			LatencyMSAverage: 25,
			FirstEventAt:     &first,
			LastEventAt:      &last,
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/api-usage/ledger/stats?tenant_id=tenant-1&from=2026-05-04T00:00:00Z&to=2026-05-05T00:00:00Z", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var body struct {
		Stats maildb.APIUsageLedgerStatsView `json:"api_usage_ledger_stats"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Stats.EventCount != 2 || body.Stats.LatencyMSAverage != 25 {
		t.Fatalf("stats = %+v", body.Stats)
	}
	if service.lastAPIUsageLedgerList.TenantID != "tenant-1" || service.lastAPIUsageLedgerList.From.IsZero() {
		t.Fatalf("lastAPIUsageLedgerList = %+v", service.lastAPIUsageLedgerList)
	}
}

func TestAdminAPIUsageLedgerRetentionReadinessHandler(t *testing.T) {
	t.Parallel()

	cutoff := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	service := &fakeAdminService{
		apiUsageLedgerRetentionReadiness: maildb.APIUsageLedgerRetentionReadinessView{
			Cutoff:                cutoff,
			TenantID:              "tenant-1",
			PrincipalID:           "principal-1",
			CandidateEventCount:   10,
			CandidateRequestCount: 10,
			CoveringExportBatchID: "api-usage-export-1",
			Ready:                 true,
			BlockingReasons:       []string{},
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/api-usage/ledger/retention-readiness?cutoff="+cutoff.Format(time.RFC3339)+"&tenant_id=%20tenant-1%20&principal_id=%20principal-1%20", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var body struct {
		Readiness maildb.APIUsageLedgerRetentionReadinessView `json:"api_usage_ledger_retention_readiness"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !body.Readiness.Ready || body.Readiness.CoveringExportBatchID != "api-usage-export-1" {
		t.Fatalf("readiness = %+v", body.Readiness)
	}
	if service.lastAPIUsageLedgerRetention.TenantID != "tenant-1" || service.lastAPIUsageLedgerRetention.PrincipalID != "principal-1" || service.lastAPIUsageLedgerRetention.Cutoff.IsZero() {
		t.Fatalf("lastAPIUsageLedgerRetention = %+v", service.lastAPIUsageLedgerRetention)
	}
}

func TestAdminAPIUsageLedgerRetentionReadinessRequiresCutoff(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, &fakeAdminService{}, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/api-usage/ledger/retention-readiness", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestAdminAPIUsageLedgerRetentionReadinessRejectsFutureCutoff(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, &fakeAdminService{}, "")

	cutoff := time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
	req := httptest.NewRequest(http.MethodGet, "/admin/v1/api-usage/ledger/retention-readiness?cutoff="+cutoff, nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "cutoff must not be in the future") {
		t.Fatalf("body = %s", rr.Body.String())
	}
}

func TestAdminAPIUsageLedgerRetentionReadinessRejectsUnsafeIdentityFilters(t *testing.T) {
	t.Parallel()

	cutoff := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)
	tests := []struct {
		name string
		path string
	}{
		{
			name: "tenant crlf",
			path: "/admin/v1/api-usage/ledger/retention-readiness?cutoff=" + cutoff + "&tenant_id=tenant%0Abad",
		},
		{
			name: "principal oversized",
			path: "/admin/v1/api-usage/ledger/retention-readiness?cutoff=" + cutoff + "&principal_id=" + strings.Repeat("p", maxAdminQueryFilterBytes+1),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := &fakeAdminService{}
			mux := http.NewServeMux()
			RegisterAdminRoutes(mux, service, "")

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rr := httptest.NewRecorder()
			mux.ServeHTTP(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
			}
			if !service.lastAPIUsageLedgerRetention.Cutoff.IsZero() || service.lastAPIUsageLedgerRetention.TenantID != "" || service.lastAPIUsageLedgerRetention.PrincipalID != "" {
				t.Fatalf("lastAPIUsageLedgerRetention = %+v", service.lastAPIUsageLedgerRetention)
			}
		})
	}
}

func TestAdminAPIUsageLedgerRetentionRunHandler(t *testing.T) {
	t.Parallel()

	cutoff := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	service := &fakeAdminService{
		apiUsageLedgerRetentionRun: maildb.APIUsageLedgerRetentionRunView{
			ID:             "api-usage-retention-test",
			CreatedAt:      cutoff.Add(time.Minute),
			Cutoff:         cutoff,
			TenantID:       "tenant-1",
			PrincipalID:    "principal-1",
			Limit:          25,
			DryRun:         false,
			ConfirmReady:   true,
			Ready:          true,
			CandidateCount: 40,
			LimitedCount:   25,
			DeletedCount:   25,
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodPost, "/admin/v1/api-usage/ledger/retention-runs", strings.NewReader(`{
		"cutoff": "`+cutoff.Format(time.RFC3339)+`",
		"tenant_id": " tenant-1 ",
		"principal_id": " principal-1 ",
		"limit": 25,
		"dry_run": false,
		"confirm_ready": true
	}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var body struct {
		Run maildb.APIUsageLedgerRetentionRunView `json:"api_usage_ledger_retention_run"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Run.ID != "api-usage-retention-test" || body.Run.CreatedAt.IsZero() || body.Run.DeletedCount != 25 || !body.Run.Ready {
		t.Fatalf("run = %+v", body.Run)
	}
	if service.lastAPIUsageLedgerRetentionRun.TenantID != "tenant-1" ||
		service.lastAPIUsageLedgerRetentionRun.PrincipalID != "principal-1" ||
		service.lastAPIUsageLedgerRetentionRun.Limit != 25 ||
		!service.lastAPIUsageLedgerRetentionRun.ConfirmReady ||
		service.lastAPIUsageLedgerRetentionRun.Cutoff.IsZero() {
		t.Fatalf("lastAPIUsageLedgerRetentionRun = %+v", service.lastAPIUsageLedgerRetentionRun)
	}
}

func TestAdminAPIUsageLedgerRetentionRunRequiresConfirmForDestructiveRun(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	cutoff := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)
	req := httptest.NewRequest(http.MethodPost, "/admin/v1/api-usage/ledger/retention-runs", strings.NewReader(`{"cutoff":"`+cutoff+`","dry_run":false}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	if !service.lastAPIUsageLedgerRetentionRun.Cutoff.IsZero() {
		t.Fatalf("retention run dispatched: %+v", service.lastAPIUsageLedgerRetentionRun)
	}
}

func TestAdminAPIUsageLedgerRetentionRunRejectsUnsafeRequests(t *testing.T) {
	t.Parallel()

	cutoff := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)
	tests := []string{
		`{"limit":25,"dry_run":true}`,
		`{"cutoff":"not-a-time","dry_run":true}`,
		`{"cutoff":"` + time.Now().UTC().Add(time.Hour).Format(time.RFC3339) + `","dry_run":true}`,
		`{"cutoff":"` + cutoff + `","limit":-1,"dry_run":true}`,
		`{"cutoff":"` + cutoff + `","tenant_id":"tenant\nbad","dry_run":true}`,
		`{"cutoff":"` + cutoff + `","principal_id":"` + strings.Repeat("p", maxAdminQueryFilterBytes+1) + `","dry_run":true}`,
	}
	for _, body := range tests {
		body := body
		t.Run(body, func(t *testing.T) {
			t.Parallel()

			service := &fakeAdminService{}
			mux := http.NewServeMux()
			RegisterAdminRoutes(mux, service, "")

			req := httptest.NewRequest(http.MethodPost, "/admin/v1/api-usage/ledger/retention-runs", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			mux.ServeHTTP(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
			}
			if !service.lastAPIUsageLedgerRetentionRun.Cutoff.IsZero() {
				t.Fatalf("retention run dispatched: %+v", service.lastAPIUsageLedgerRetentionRun)
			}
		})
	}
}

func TestAdminListAPIUsageLedgerRetentionRunsHandler(t *testing.T) {
	t.Parallel()

	created := time.Date(2026, 5, 5, 1, 0, 0, 0, time.UTC)
	service := &fakeAdminService{
		apiUsageLedgerRetentionRuns: []maildb.APIUsageLedgerRetentionRunView{{
			ID:             "api-usage-retention-1",
			CreatedAt:      created,
			Cutoff:         created.Add(-time.Hour),
			TenantID:       "tenant-1",
			PrincipalID:    "principal-1",
			Limit:          100,
			DryRun:         true,
			ConfirmReady:   false,
			Ready:          true,
			CandidateCount: 10,
			LimitedCount:   10,
			DeletedCount:   0,
		}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/api-usage/ledger/retention-runs?limit=5&tenant_id=%20tenant-1%20&principal_id=%20principal-1%20&created_from=2026-05-05T00:00:00Z&created_to=2026-05-06T00:00:00Z", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var body struct {
		Runs []maildb.APIUsageLedgerRetentionRunView `json:"api_usage_ledger_retention_runs"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Runs) != 1 || body.Runs[0].ID != "api-usage-retention-1" {
		t.Fatalf("runs = %+v", body.Runs)
	}
	if service.lastAPIUsageLedgerRetentionRunList.Limit != 5 ||
		service.lastAPIUsageLedgerRetentionRunList.TenantID != "tenant-1" ||
		service.lastAPIUsageLedgerRetentionRunList.PrincipalID != "principal-1" ||
		service.lastAPIUsageLedgerRetentionRunList.CreatedFrom.IsZero() ||
		service.lastAPIUsageLedgerRetentionRunList.CreatedTo.IsZero() {
		t.Fatalf("lastAPIUsageLedgerRetentionRunList = %+v", service.lastAPIUsageLedgerRetentionRunList)
	}
}

func TestAdminGetAPIUsageLedgerRetentionRunHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		apiUsageLedgerRetentionRun: maildb.APIUsageLedgerRetentionRunView{
			ID:           "api-usage-retention-1",
			CreatedAt:    time.Date(2026, 5, 5, 1, 0, 0, 0, time.UTC),
			Cutoff:       time.Date(2026, 5, 5, 0, 0, 0, 0, time.UTC),
			Limit:        100,
			DryRun:       true,
			ConfirmReady: false,
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/api-usage/ledger/retention-runs/api-usage-retention-1", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var body struct {
		Run maildb.APIUsageLedgerRetentionRunView `json:"api_usage_ledger_retention_run"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Run.ID != "api-usage-retention-1" || service.lastAPIUsageLedgerRetentionRunID != "api-usage-retention-1" {
		t.Fatalf("run = %+v lastID=%q", body.Run, service.lastAPIUsageLedgerRetentionRunID)
	}
}

func TestAdminAPIUsageLedgerRetentionRunListRejectsUnsafeRequests(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	for _, path := range []string{
		"/admin/v1/api-usage/ledger/retention-runs?tenant_id=tenant%0Abad",
		"/admin/v1/api-usage/ledger/retention-runs?principal_id=" + strings.Repeat("x", maxAdminQueryFilterBytes+1),
		"/admin/v1/api-usage/ledger/retention-runs?created_from=bad-time",
		"/admin/v1/api-usage/ledger/retention-runs?created_from=2026-05-06T00:00:00Z&created_to=2026-05-05T00:00:00Z",
		"/admin/v1/api-usage/ledger/retention-runs/api-usage-retention%0Abad",
	} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rr := httptest.NewRecorder()
			mux.ServeHTTP(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
			}
			if service.lastAPIUsageLedgerRetentionRunID != "" || service.lastAPIUsageLedgerRetentionRunList.Limit != 0 {
				t.Fatalf("retention run read dispatched: id=%q list=%+v", service.lastAPIUsageLedgerRetentionRunID, service.lastAPIUsageLedgerRetentionRunList)
			}
		})
	}
}

func TestAdminDAVSyncRetentionReadinessHandler(t *testing.T) {
	t.Parallel()

	cutoff := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	service := &fakeAdminService{
		davSyncRetentionReadiness: davsyncretention.ReadinessView{
			Cutoff:             cutoff,
			Limit:              500,
			Ready:              true,
			Truncated:          false,
			CandidateCount:     18,
			CalDAVCandidates:   7,
			CardDAVCandidates:  11,
			DestructiveGuarded: true,
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/dav-sync/retention-readiness?cutoff="+cutoff.Format(time.RFC3339)+"&limit=500", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var body struct {
		Readiness davsyncretention.ReadinessView `json:"dav_sync_retention_readiness"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !body.Readiness.Ready || body.Readiness.CandidateCount != 18 || !body.Readiness.DestructiveGuarded {
		t.Fatalf("readiness = %+v", body.Readiness)
	}
	if service.lastDAVSyncRetentionReadiness.Limit != 500 || service.lastDAVSyncRetentionReadiness.Cutoff.IsZero() {
		t.Fatalf("lastDAVSyncRetentionReadiness = %+v", service.lastDAVSyncRetentionReadiness)
	}
}

func TestAdminDAVSyncRetentionReadinessRejectsUnsafeRequests(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")
	for _, path := range []string{
		"/admin/v1/dav-sync/retention-readiness",
		"/admin/v1/dav-sync/retention-readiness?cutoff=not-a-time",
		"/admin/v1/dav-sync/retention-readiness?cutoff=" + time.Now().UTC().Add(time.Hour).Format(time.RFC3339),
		"/admin/v1/dav-sync/retention-readiness?cutoff=" + time.Now().UTC().Add(-time.Hour).Format(time.RFC3339) + "&limit=0",
		"/admin/v1/dav-sync/retention-readiness?cutoff=" + time.Now().UTC().Add(-time.Hour).Format(time.RFC3339) + "&status=completed",
	} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rr := httptest.NewRecorder()
			mux.ServeHTTP(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
			}
			if !service.lastDAVSyncRetentionReadiness.Cutoff.IsZero() {
				t.Fatalf("readiness dispatched: %+v", service.lastDAVSyncRetentionReadiness)
			}
		})
	}
}

func TestAdminRunDAVSyncRetentionHandler(t *testing.T) {
	t.Parallel()

	cutoff := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	service := &fakeAdminService{
		davSyncRetentionRun: davsyncretention.RunRecord{
			ID:                "dav-sync-retention-run-1",
			Cutoff:            cutoff,
			Limit:             500,
			DryRun:            true,
			ConfirmReady:      false,
			Status:            davsyncretention.RunStatusCompleted,
			CalDAVCandidates:  7,
			CardDAVCandidates: 11,
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodPost, "/admin/v1/dav-sync/retention-runs", strings.NewReader(`{
		"cutoff":"`+cutoff.Format(time.RFC3339)+`",
		"limit":500,
		"dry_run":true
	}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var body struct {
		Run davsyncretention.RunRecord `json:"dav_sync_retention_run"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Run.ID != "dav-sync-retention-run-1" || !body.Run.DryRun || body.Run.CalDAVCandidates != 7 {
		t.Fatalf("run = %+v", body.Run)
	}
	if !service.lastDAVSyncRetentionRun.DryRun || service.lastDAVSyncRetentionRun.Limit != 500 || service.lastDAVSyncRetentionRun.Cutoff.IsZero() {
		t.Fatalf("lastDAVSyncRetentionRun = %+v", service.lastDAVSyncRetentionRun)
	}
}

func TestAdminRunDAVSyncRetentionRejectsUnsafeRequests(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")
	past := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)
	for _, tc := range []struct {
		name string
		path string
		body string
	}{
		{name: "unknown query", path: "/admin/v1/dav-sync/retention-runs?dry_run=true", body: `{"cutoff":"` + past + `","dry_run":true}`},
		{name: "missing cutoff", path: "/admin/v1/dav-sync/retention-runs", body: `{"dry_run":true}`},
		{name: "bad cutoff", path: "/admin/v1/dav-sync/retention-runs", body: `{"cutoff":"not-a-time","dry_run":true}`},
		{name: "future cutoff", path: "/admin/v1/dav-sync/retention-runs", body: `{"cutoff":"` + time.Now().UTC().Add(time.Hour).Format(time.RFC3339) + `","dry_run":true}`},
		{name: "negative limit", path: "/admin/v1/dav-sync/retention-runs", body: `{"cutoff":"` + past + `","limit":-1,"dry_run":true}`},
		{name: "too large limit", path: "/admin/v1/dav-sync/retention-runs", body: `{"cutoff":"` + past + `","limit":10001,"dry_run":true}`},
		{name: "destructive without confirm", path: "/admin/v1/dav-sync/retention-runs", body: `{"cutoff":"` + past + `","dry_run":false}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			mux.ServeHTTP(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
			}
			if !service.lastDAVSyncRetentionRun.Cutoff.IsZero() {
				t.Fatalf("run dispatched: %+v", service.lastDAVSyncRetentionRun)
			}
		})
	}
}

func TestAdminListDAVSyncRetentionRunsHandler(t *testing.T) {
	t.Parallel()

	created := time.Date(2026, 5, 5, 1, 0, 0, 0, time.UTC)
	service := &fakeAdminService{
		davSyncRetentionRuns: []davsyncretention.RunRecord{{
			ID:                "dav-sync-retention-1",
			CreatedAt:         created,
			Cutoff:            created.Add(-90 * 24 * time.Hour),
			Limit:             1000,
			DryRun:            true,
			ConfirmReady:      false,
			Status:            davsyncretention.RunStatusCompleted,
			CalDAVCandidates:  7,
			CardDAVCandidates: 11,
		}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/dav-sync/retention-runs?limit=5&status=completed&created_from=2026-05-05T00:00:00Z&created_to=2026-05-06T00:00:00Z", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var body struct {
		Runs []davsyncretention.RunRecord `json:"dav_sync_retention_runs"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Runs) != 1 || body.Runs[0].ID != "dav-sync-retention-1" || body.Runs[0].CalDAVCandidates != 7 {
		t.Fatalf("runs = %+v", body.Runs)
	}
	if service.lastDAVSyncRetentionRunList.Limit != 5 ||
		service.lastDAVSyncRetentionRunList.Status != davsyncretention.RunStatusCompleted ||
		service.lastDAVSyncRetentionRunList.CreatedFrom.IsZero() ||
		service.lastDAVSyncRetentionRunList.CreatedTo.IsZero() {
		t.Fatalf("lastDAVSyncRetentionRunList = %+v", service.lastDAVSyncRetentionRunList)
	}
}

func TestAdminGetDAVSyncRetentionRunHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		davSyncRetentionRun: davsyncretention.RunRecord{
			ID:                "dav-sync-retention-1",
			CreatedAt:         time.Date(2026, 5, 5, 1, 0, 0, 0, time.UTC),
			Cutoff:            time.Date(2026, 2, 5, 1, 0, 0, 0, time.UTC),
			Limit:             1000,
			DryRun:            false,
			ConfirmReady:      true,
			Status:            davsyncretention.RunStatusFailed,
			ErrorMessage:      "carddav failed",
			CalDAVCandidates:  7,
			CalDAVDeleted:     3,
			CardDAVCandidates: 0,
			CardDAVDeleted:    0,
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/dav-sync/retention-runs/dav-sync-retention-1", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var body struct {
		Run davsyncretention.RunRecord `json:"dav_sync_retention_run"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Run.ID != "dav-sync-retention-1" || body.Run.Status != davsyncretention.RunStatusFailed || service.lastDAVSyncRetentionRunID != "dav-sync-retention-1" {
		t.Fatalf("run = %+v lastID=%q", body.Run, service.lastDAVSyncRetentionRunID)
	}
}

func TestAdminDAVSyncRetentionRunListRejectsUnsafeRequests(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	for _, path := range []string{
		"/admin/v1/dav-sync/retention-runs?status=blocked",
		"/admin/v1/dav-sync/retention-runs?created_from=bad-time",
		"/admin/v1/dav-sync/retention-runs?created_from=2026-05-06T00:00:00Z&created_to=2026-05-05T00:00:00Z",
		"/admin/v1/dav-sync/retention-runs/dav-sync-retention%0Abad",
		"/admin/v1/dav-sync/retention-runs?tenant_id=tenant-1",
	} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rr := httptest.NewRecorder()
			mux.ServeHTTP(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
			}
			if service.lastDAVSyncRetentionRunID != "" || service.lastDAVSyncRetentionRunList.Limit != 0 {
				t.Fatalf("DAV retention run read dispatched: id=%q list=%+v", service.lastDAVSyncRetentionRunID, service.lastDAVSyncRetentionRunList)
			}
		})
	}
}

func TestAdminCreateAPIUsageExportBatchHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		apiUsageExportBatch: maildb.APIUsageExportBatchView{
			ID:           "api-usage-export-1",
			Status:       "completed",
			ExportFormat: "ndjson",
			TenantID:     "tenant-1",
			EventCount:   2,
			Manifest:     json.RawMessage(`{"version":"2026-05-04.api-usage-export.v1"}`),
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodPost, "/admin/v1/api-usage/export-batches?tenant_id=%20tenant-1%20&from=2026-05-04T00:00:00Z&to=2026-05-05T00:00:00Z", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var body struct {
		Batch maildb.APIUsageExportBatchView `json:"api_usage_export_batch"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Batch.ID != "api-usage-export-1" || body.Batch.EventCount != 2 {
		t.Fatalf("batch = %+v", body.Batch)
	}
	if service.lastAPIUsageLedgerList.TenantID != "tenant-1" || service.lastAPIUsageLedgerList.From.IsZero() {
		t.Fatalf("lastAPIUsageLedgerList = %+v", service.lastAPIUsageLedgerList)
	}
}

func TestAdminCreateAPIUsageExportBatchHandlerRejectsMissingWindow(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		path string
	}{
		{
			name: "missing from",
			path: "/admin/v1/api-usage/export-batches?tenant_id=tenant-1&to=2026-05-05T00:00:00Z",
		},
		{
			name: "missing to",
			path: "/admin/v1/api-usage/export-batches?tenant_id=tenant-1&from=2026-05-04T00:00:00Z",
		},
		{
			name: "missing both",
			path: "/admin/v1/api-usage/export-batches?tenant_id=tenant-1",
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			service := &fakeAdminService{}
			mux := http.NewServeMux()
			RegisterAdminRoutes(mux, service, "")

			req := httptest.NewRequest(http.MethodPost, tc.path, nil)
			rr := httptest.NewRecorder()
			mux.ServeHTTP(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
			}
			if !strings.Contains(rr.Body.String(), "from and to are required") {
				t.Fatalf("body = %s", rr.Body.String())
			}
			if !service.lastAPIUsageLedgerList.From.IsZero() || !service.lastAPIUsageLedgerList.To.IsZero() {
				t.Fatalf("lastAPIUsageLedgerList = %+v", service.lastAPIUsageLedgerList)
			}
		})
	}
}

func TestAdminListAPIUsageExportBatchesHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		apiUsageExportBatches: []maildb.APIUsageExportBatchView{{
			ID:           "api-usage-export-1",
			Status:       "completed",
			ExportFormat: "ndjson",
			EventCount:   2,
			Manifest:     json.RawMessage(`{}`),
		}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/api-usage/export-batches?limit=5&tenant_id=%20tenant-1%20&principal_id=%20principal-1%20&status=completed&from=2026-05-04T00:00:00Z&to=2026-05-05T00:00:00Z", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var body struct {
		Batches []maildb.APIUsageExportBatchView `json:"api_usage_export_batches"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Batches) != 1 || body.Batches[0].ID != "api-usage-export-1" {
		t.Fatalf("batches = %+v", body.Batches)
	}
	if service.lastAPIUsageExportBatchList.Limit != 5 || service.lastAPIUsageExportBatchList.TenantID != "tenant-1" || service.lastAPIUsageExportBatchList.PrincipalID != "principal-1" || service.lastAPIUsageExportBatchList.Status != "completed" || service.lastAPIUsageExportBatchList.From.IsZero() || service.lastAPIUsageExportBatchList.To.IsZero() {
		t.Fatalf("lastAPIUsageExportBatchList = %+v", service.lastAPIUsageExportBatchList)
	}
}

func TestAdminListAPIUsageExportBatchesHandlerRejectsUnsafeFilters(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	tests := []string{
		"/admin/v1/api-usage/export-batches?tenant_id=tenant%0Abad",
		"/admin/v1/api-usage/export-batches?principal_id=" + strings.Repeat("p", maxAdminQueryFilterBytes+1),
		"/admin/v1/api-usage/export-batches?status=ready",
		"/admin/v1/api-usage/export-batches?from=bad-time",
		"/admin/v1/api-usage/export-batches?from=2026-05-05T00:00:00Z&to=2026-05-04T00:00:00Z",
	}
	for _, target := range tests {
		target := target
		t.Run(target, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, target, nil)
			rr := httptest.NewRecorder()
			mux.ServeHTTP(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
			}
			if service.lastAPIUsageExportBatchList.Limit != 0 {
				t.Fatalf("export batch list was called: %+v", service.lastAPIUsageExportBatchList)
			}
		})
	}
}

func TestAdminGetAPIUsageExportBatchHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		apiUsageExportBatch: maildb.APIUsageExportBatchView{
			ID:           "api-usage-export-1",
			Status:       "completed",
			ExportFormat: "ndjson",
			EventCount:   2,
			Manifest:     json.RawMessage(`{}`),
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/api-usage/export-batches/api-usage-export-1", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var body struct {
		Batch maildb.APIUsageExportBatchView `json:"api_usage_export_batch"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Batch.ID != "api-usage-export-1" {
		t.Fatalf("batch = %+v", body.Batch)
	}
	if service.lastAPIUsageExportBatchID != "api-usage-export-1" {
		t.Fatalf("lastAPIUsageExportBatchID = %q", service.lastAPIUsageExportBatchID)
	}
}

func TestAdminGetAPIUsageExportCapabilitiesHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		apiUsageExportCapabilities: maildb.APIUsageExportCapabilityView{
			ExportFormat:                                "ndjson",
			ArtifactContentType:                         "application/x-ndjson",
			ManifestDigestAlgorithm:                     "sha256",
			SignerBackend:                               "local-hmac",
			SignerConfigured:                            true,
			SignerKeyID:                                 "key-1",
			VerifierConfigured:                          true,
			ProductionSignatureReady:                    false,
			BillingReadySupported:                       false,
			VerifiedBillingReadySupported:               false,
			RetentionRunsSupported:                      true,
			RetentionWorkerSupported:                    true,
			RetentionWorkerDestructiveRequiresRemoteKey: true,
			BlockingReasons:                             []string{"production_manifest_signer_required"},
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/api-usage/export-capabilities", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var body struct {
		Capabilities maildb.APIUsageExportCapabilityView `json:"api_usage_export_capabilities"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Capabilities.SignerBackend != "local-hmac" || body.Capabilities.ProductionSignatureReady || !body.Capabilities.RetentionWorkerDestructiveRequiresRemoteKey {
		t.Fatalf("capabilities = %+v", body.Capabilities)
	}
	if !service.lastAPIUsageExportCapabilities {
		t.Fatal("GetAPIUsageExportCapabilities was not called")
	}
}

func TestAdminGetAPIUsageExportCapabilitiesHandlerAcceptsDocumentedAdminAuth(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		header string
		value  string
	}{
		{name: "admin token header", header: "X-Admin-Token", value: "secret"},
		{name: "bearer token", header: "Authorization", value: "Bearer secret"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := &fakeAdminService{}
			mux := http.NewServeMux()
			RegisterAdminRoutes(mux, service, "secret")

			req := httptest.NewRequest(http.MethodGet, "/admin/v1/api-usage/export-capabilities", nil)
			req.Header.Set(tt.header, tt.value)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestAdminGetAPIUsageExportCapabilitiesHandlerRejectsAmbiguousAdminAuth(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "secret")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/api-usage/export-capabilities", nil)
	req.Header.Set("X-Admin-Token", "secret")
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestAdminGetAPIUsageExportHandoffReadinessHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		apiUsageExportHandoff: maildb.APIUsageExportHandoffView{
			BatchID:                    "api-usage-export-1",
			BatchStatus:                "completed",
			BatchCompleted:             true,
			EventCount:                 2,
			ArtifactCount:              1,
			ArtifactEventCount:         2,
			ManifestDigestCount:        1,
			LatestManifestDigestID:     "api-usage-manifest-1",
			LatestDigestSignatureCount: 1,
			LatestSignatureID:          "api-usage-signature-1",
			LatestSignatureSigner:      "local-hmac",
			Ready:                      true,
			ReadinessGrade:             "operational",
			BillingReady:               false,
			BillingBlockingReasons:     []string{"production_manifest_signer_required"},
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/api-usage/export-batches/api-usage-export-1/handoff-readiness", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var body struct {
		Handoff maildb.APIUsageExportHandoffView `json:"api_usage_export_handoff_readiness"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !body.Handoff.Ready || body.Handoff.BillingReady || body.Handoff.ReadinessGrade != "operational" {
		t.Fatalf("handoff = %+v", body.Handoff)
	}
	if service.lastAPIUsageExportBatchID != "api-usage-export-1" {
		t.Fatalf("lastAPIUsageExportBatchID = %q", service.lastAPIUsageExportBatchID)
	}
	if service.lastAPIUsageExportHandoffDeep {
		t.Fatal("lastAPIUsageExportHandoffDeep = true, want false")
	}
}

func TestAdminGetAPIUsageExportHandoffReadinessHandlerDeep(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		apiUsageExportHandoff: maildb.APIUsageExportHandoffView{
			BatchID:                    "api-usage-export-1",
			BatchStatus:                "completed",
			BatchCompleted:             true,
			EventCount:                 2,
			ArtifactCount:              1,
			ArtifactEventCount:         2,
			ManifestDigestCount:        1,
			LatestManifestDigestID:     "api-usage-manifest-1",
			LatestDigestSignatureCount: 1,
			LatestSignatureID:          "api-usage-signature-1",
			Ready:                      true,
			ReadinessGrade:             "billing_candidate",
			BillingReady:               true,
			DeepVerification:           true,
			DeepReady:                  true,
			ArtifactVerifications: []maildb.APIUsageExportArtifactVerificationView{{
				BatchID:    "api-usage-export-1",
				ArtifactID: "api-usage-artifact-1",
				Valid:      true,
			}},
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/api-usage/export-batches/api-usage-export-1/handoff-readiness?deep=true", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var body struct {
		Handoff maildb.APIUsageExportHandoffView `json:"api_usage_export_handoff_readiness"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !body.Handoff.DeepVerification || !body.Handoff.DeepReady || len(body.Handoff.ArtifactVerifications) != 1 {
		t.Fatalf("handoff = %+v", body.Handoff)
	}
	if !service.lastAPIUsageExportHandoffDeep {
		t.Fatal("lastAPIUsageExportHandoffDeep = false, want true")
	}
}

func TestAdminGetAPIUsageExportHandoffReadinessHandlerRejectsBadDeepQuery(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/api-usage/export-batches/api-usage-export-1/handoff-readiness?deep=sure", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	if service.lastAPIUsageExportBatchID != "" {
		t.Fatalf("lastAPIUsageExportBatchID = %q", service.lastAPIUsageExportBatchID)
	}
}

func TestAdminAPIUsageExportPathIDsRejectUnsafeValues(t *testing.T) {
	t.Parallel()

	oversizedBatchID := strings.Repeat("b", maxAdminQueryFilterBytes+1)
	tests := []struct {
		name   string
		method string
		path   string
	}{
		{
			name:   "batch crlf",
			method: http.MethodGet,
			path:   "/admin/v1/api-usage/export-batches/api-usage-export%0Abad",
		},
		{
			name:   "batch oversized",
			method: http.MethodGet,
			path:   "/admin/v1/api-usage/export-batches/" + oversizedBatchID + "/handoff-readiness",
		},
		{
			name:   "create artifact batch crlf",
			method: http.MethodPost,
			path:   "/admin/v1/api-usage/export-batches/api-usage-export%0Dbad/artifacts",
		},
		{
			name:   "artifact crlf",
			method: http.MethodGet,
			path:   "/admin/v1/api-usage/export-batches/api-usage-export-1/artifacts/api-usage-artifact%0Abad",
		},
		{
			name:   "digest crlf",
			method: http.MethodGet,
			path:   "/admin/v1/api-usage/export-batches/api-usage-export-1/manifest-digests/api-usage-manifest%0Abad",
		},
		{
			name:   "signature crlf",
			method: http.MethodGet,
			path:   "/admin/v1/api-usage/export-batches/api-usage-export-1/manifest-digests/api-usage-manifest-1/signatures/api-usage-signature%0Abad",
		},
		{
			name:   "signature verification oversized batch",
			method: http.MethodGet,
			path:   "/admin/v1/api-usage/export-batches/" + oversizedBatchID + "/manifest-digests/api-usage-manifest-1/signatures/api-usage-signature-1/verification",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := &fakeAdminService{}
			mux := http.NewServeMux()
			RegisterAdminRoutes(mux, service, "")

			req := httptest.NewRequest(tt.method, tt.path, nil)
			rr := httptest.NewRecorder()
			mux.ServeHTTP(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
			}
			if service.lastAPIUsageExportBatchID != "" || service.lastAPIUsageExportArtifactID != "" || service.lastAPIUsageExportManifestDigestID != "" || service.lastAPIUsageExportManifestSignatureID != "" {
				t.Fatalf("last ids = %q/%q/%q/%q", service.lastAPIUsageExportBatchID, service.lastAPIUsageExportArtifactID, service.lastAPIUsageExportManifestDigestID, service.lastAPIUsageExportManifestSignatureID)
			}
			if service.lastCreateAPIUsageExportArtifact.BatchID != "" {
				t.Fatalf("lastCreateAPIUsageExportArtifact = %+v", service.lastCreateAPIUsageExportArtifact)
			}
		})
	}
}

func TestAdminAPIUsageRoutesRejectUnknownQueryParameters(t *testing.T) {
	t.Parallel()

	cutoff := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)
	tests := []struct {
		name       string
		method     string
		path       string
		dispatched func(*fakeAdminService) bool
	}{
		{
			name:   "daily aggregate",
			method: http.MethodGet,
			path:   "/admin/v1/api-usage/daily?tenant_id=tenant-1&bucket=hour",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastAPIUsageDailyList.TenantID != ""
			},
		},
		{
			name:   "monthly aggregate",
			method: http.MethodGet,
			path:   "/admin/v1/api-usage/monthly?group_by=route",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastAPIUsageMonthlyList.Limit != 0
			},
		},
		{
			name:   "ledger list",
			method: http.MethodGet,
			path:   "/admin/v1/api-usage/ledger?cursor=opaque",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastAPIUsageLedgerList.Limit != 0
			},
		},
		{
			name:   "ledger export",
			method: http.MethodGet,
			path:   "/admin/v1/api-usage/ledger/export?format=csv",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastAPIUsageLedgerList.Limit != 0
			},
		},
		{
			name:   "ledger stats",
			method: http.MethodGet,
			path:   "/admin/v1/api-usage/ledger/stats?limit=5",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastAPIUsageLedgerList.Limit != 0
			},
		},
		{
			name:   "retention readiness",
			method: http.MethodGet,
			path:   "/admin/v1/api-usage/ledger/retention-readiness?cutoff=" + cutoff + "&dry_run=true",
			dispatched: func(service *fakeAdminService) bool {
				return !service.lastAPIUsageLedgerRetention.Cutoff.IsZero()
			},
		},
		{
			name:   "retention run create",
			method: http.MethodPost,
			path:   "/admin/v1/api-usage/ledger/retention-runs?dry_run=true",
			dispatched: func(service *fakeAdminService) bool {
				return !service.lastAPIUsageLedgerRetentionRun.Cutoff.IsZero()
			},
		},
		{
			name:   "dav sync retention run create",
			method: http.MethodPost,
			path:   "/admin/v1/dav-sync/retention-runs?dry_run=true",
			dispatched: func(service *fakeAdminService) bool {
				return !service.lastDAVSyncRetentionRun.Cutoff.IsZero()
			},
		},
		{
			name:   "retention run list",
			method: http.MethodGet,
			path:   "/admin/v1/api-usage/ledger/retention-runs?cutoff=" + cutoff,
			dispatched: func(service *fakeAdminService) bool {
				return service.lastAPIUsageLedgerRetentionRunList.Limit != 0
			},
		},
		{
			name:   "retention run detail",
			method: http.MethodGet,
			path:   "/admin/v1/api-usage/ledger/retention-runs/api-usage-retention-1?expand=true",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastAPIUsageLedgerRetentionRunID != ""
			},
		},
		{
			name:   "export capabilities",
			method: http.MethodGet,
			path:   "/admin/v1/api-usage/export-capabilities?deep=true",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastAPIUsageExportCapabilities
			},
		},
		{
			name:   "export batch create",
			method: http.MethodPost,
			path:   "/admin/v1/api-usage/export-batches?from=2026-05-04T00:00:00Z&to=2026-05-05T00:00:00Z&status=completed",
			dispatched: func(service *fakeAdminService) bool {
				return !service.lastAPIUsageLedgerList.From.IsZero()
			},
		},
		{
			name:   "export batch list",
			method: http.MethodGet,
			path:   "/admin/v1/api-usage/export-batches?cursor=opaque",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastAPIUsageExportBatchList.Limit != 0
			},
		},
		{
			name:   "export batch detail",
			method: http.MethodGet,
			path:   "/admin/v1/api-usage/export-batches/api-usage-export-1?include_manifest=true",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastAPIUsageExportBatchID != ""
			},
		},
		{
			name:   "handoff readiness",
			method: http.MethodGet,
			path:   "/admin/v1/api-usage/export-batches/api-usage-export-1/handoff-readiness?id=api-usage-export-1",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastAPIUsageExportBatchID != ""
			},
		},
		{
			name:   "batch export",
			method: http.MethodGet,
			path:   "/admin/v1/api-usage/export-batches/api-usage-export-1/export?from=2026-05-04T00:00:00Z",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastAPIUsageExportBatchID != "" || !service.lastAPIUsageLedgerList.From.IsZero()
			},
		},
		{
			name:   "artifact create",
			method: http.MethodPost,
			path:   "/admin/v1/api-usage/export-batches/api-usage-export-1/artifacts?include_payload=true",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastCreateAPIUsageExportArtifact.BatchID != ""
			},
		},
		{
			name:   "artifact list",
			method: http.MethodGet,
			path:   "/admin/v1/api-usage/export-batches/api-usage-export-1/artifacts?cursor=opaque",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastAPIUsageExportBatchID != "" || service.lastLimit != 0
			},
		},
		{
			name:   "artifact write",
			method: http.MethodPost,
			path:   "/admin/v1/api-usage/export-batches/api-usage-export-1/artifacts/write?dry_run=true",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastAPIUsageExportBatchID != ""
			},
		},
		{
			name:   "artifact detail",
			method: http.MethodGet,
			path:   "/admin/v1/api-usage/export-batches/api-usage-export-1/artifacts/api-usage-artifact-1?deep=true",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastAPIUsageExportArtifactID != ""
			},
		},
		{
			name:   "artifact download",
			method: http.MethodGet,
			path:   "/admin/v1/api-usage/export-batches/api-usage-export-1/artifacts/api-usage-artifact-1/download?filename=usage.ndjson",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastAPIUsageExportArtifactID != ""
			},
		},
		{
			name:   "artifact verification",
			method: http.MethodGet,
			path:   "/admin/v1/api-usage/export-batches/api-usage-export-1/artifacts/api-usage-artifact-1/verification?deep=true",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastAPIUsageExportArtifactID != ""
			},
		},
		{
			name:   "manifest digest create",
			method: http.MethodPost,
			path:   "/admin/v1/api-usage/export-batches/api-usage-export-1/manifest-digests?schema_version=latest",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastAPIUsageExportBatchID != ""
			},
		},
		{
			name:   "manifest digest list",
			method: http.MethodGet,
			path:   "/admin/v1/api-usage/export-batches/api-usage-export-1/manifest-digests?cursor=opaque",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastAPIUsageExportBatchID != "" || service.lastLimit != 0
			},
		},
		{
			name:   "manifest digest detail",
			method: http.MethodGet,
			path:   "/admin/v1/api-usage/export-batches/api-usage-export-1/manifest-digests/api-usage-manifest-1?include_manifest=true",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastAPIUsageExportManifestDigestID != ""
			},
		},
		{
			name:   "manifest digest verification",
			method: http.MethodGet,
			path:   "/admin/v1/api-usage/export-batches/api-usage-export-1/manifest-digests/api-usage-manifest-1/verification?deep=true",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastAPIUsageExportManifestDigestID != ""
			},
		},
		{
			name:   "manifest signature create",
			method: http.MethodPost,
			path:   "/admin/v1/api-usage/export-batches/api-usage-export-1/manifest-digests/api-usage-manifest-1/signatures?key_id=key-1",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastAPIUsageExportManifestDigestID != ""
			},
		},
		{
			name:   "manifest signature list",
			method: http.MethodGet,
			path:   "/admin/v1/api-usage/export-batches/api-usage-export-1/manifest-digests/api-usage-manifest-1/signatures?cursor=opaque",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastAPIUsageExportManifestDigestID != "" || service.lastLimit != 0
			},
		},
		{
			name:   "manifest signature detail",
			method: http.MethodGet,
			path:   "/admin/v1/api-usage/export-batches/api-usage-export-1/manifest-digests/api-usage-manifest-1/signatures/api-usage-signature-1?include_digest=true",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastAPIUsageExportManifestSignatureID != ""
			},
		},
		{
			name:   "manifest signature verification",
			method: http.MethodGet,
			path:   "/admin/v1/api-usage/export-batches/api-usage-export-1/manifest-digests/api-usage-manifest-1/signatures/api-usage-signature-1/verification?deep=true",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastAPIUsageExportManifestSignatureID != ""
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := &fakeAdminService{}
			mux := http.NewServeMux()
			RegisterAdminRoutes(mux, service, "")

			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if tt.dispatched(service) {
				t.Fatalf("handler dispatched for unknown query key: %+v", service)
			}
		})
	}
}

func TestAdminExportAPIUsageExportBatchHandler(t *testing.T) {
	t.Parallel()

	windowStart := time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC)
	windowEnd := time.Date(2026, 5, 5, 0, 0, 0, 0, time.UTC)
	service := &fakeAdminService{
		apiUsageExportBatch: maildb.APIUsageExportBatchView{
			ID:           "api-usage-export-1",
			Status:       "completed",
			ExportFormat: "ndjson",
			TenantID:     "tenant-1",
			PrincipalID:  "principal-1",
			WindowStart:  &windowStart,
			WindowEnd:    &windowEnd,
			EventCount:   1,
			Manifest:     json.RawMessage(`{}`),
		},
		apiUsageLedger: []maildb.APIUsageLedgerView{{
			EventID:       "usage-1",
			SchemaVersion: "2026-05-04.api-usage.v2",
			EventTime:     windowStart,
			RecordedAt:    windowStart,
			Method:        "GET",
			Route:         "GET /api/v1/messages",
			Status:        200,
			RequestCount:  1,
			Payload:       json.RawMessage(`{"event":"api.usage"}`),
		}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/api-usage/export-batches/api-usage-export-1/export?limit=5", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Content-Type"); got != "application/x-ndjson" {
		t.Fatalf("content type = %q", got)
	}
	if got := rr.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("cache control = %q", got)
	}
	if got := rr.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("x-content-type-options = %q", got)
	}
	if !strings.Contains(rr.Body.String(), `"event_id":"usage-1"`) {
		t.Fatalf("ndjson = %q", rr.Body.String())
	}
	if service.lastAPIUsageLedgerList.TenantID != "tenant-1" || service.lastAPIUsageLedgerList.PrincipalID != "principal-1" {
		t.Fatalf("lastAPIUsageLedgerList = %+v", service.lastAPIUsageLedgerList)
	}
	if !service.lastAPIUsageLedgerList.From.Equal(windowStart) || !service.lastAPIUsageLedgerList.To.Equal(windowEnd) {
		t.Fatalf("lastAPIUsageLedgerList timestamps = %+v", service.lastAPIUsageLedgerList)
	}
}

func TestAdminCreateAPIUsageExportArtifactHandler(t *testing.T) {
	t.Parallel()

	hash := strings.Repeat("a", 64)
	service := &fakeAdminService{
		apiUsageExportArtifact: maildb.APIUsageExportArtifactView{
			ID:             "api-usage-artifact-1",
			BatchID:        "api-usage-export-1",
			StorageBackend: "s3",
			ObjectKey:      "exports/api-usage-export-1.ndjson",
			ContentType:    "application/x-ndjson",
			ByteCount:      12,
			SHA256Hex:      hash,
			EventCount:     2,
			Metadata:       json.RawMessage(`{"bucket":"billing"}`),
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	body := strings.NewReader(`{"storage_backend":"s3","object_key":"exports/api-usage-export-1.ndjson","byte_count":12,"sha256_hex":"` + hash + `","event_count":2,"metadata":{"bucket":"billing"}}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/v1/api-usage/export-batches/api-usage-export-1/artifacts", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var response struct {
		Artifact maildb.APIUsageExportArtifactView `json:"api_usage_export_artifact"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Artifact.ID != "api-usage-artifact-1" || service.lastCreateAPIUsageExportArtifact.BatchID != "api-usage-export-1" {
		t.Fatalf("artifact = %+v last=%+v", response.Artifact, service.lastCreateAPIUsageExportArtifact)
	}
}

func TestAdminWriteAPIUsageExportArtifactHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		apiUsageExportArtifact: maildb.APIUsageExportArtifactView{
			ID:             "api-usage-artifact-1",
			BatchID:        "api-usage-export-1",
			StorageBackend: "local",
			ObjectKey:      "exports/api-usage/api-usage-export-1.ndjson",
			ContentType:    "application/x-ndjson",
			ByteCount:      12,
			SHA256Hex:      strings.Repeat("a", 64),
			EventCount:     2,
			Metadata:       json.RawMessage(`{"writer":"gogomail-admin-api"}`),
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	body := strings.NewReader(`{"object_key":"exports/api-usage/api-usage-export-1.ndjson","metadata":{"purpose":"billing"}}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/v1/api-usage/export-batches/api-usage-export-1/artifacts/write", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var response struct {
		Artifact maildb.APIUsageExportArtifactView `json:"api_usage_export_artifact"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Artifact.ID != "api-usage-artifact-1" || service.lastAPIUsageExportBatchID != "api-usage-export-1" {
		t.Fatalf("artifact = %+v lastBatch=%q", response.Artifact, service.lastAPIUsageExportBatchID)
	}
	if service.lastWriteAPIUsageExportArtifact.ObjectKey != "exports/api-usage/api-usage-export-1.ndjson" {
		t.Fatalf("last write request = %+v", service.lastWriteAPIUsageExportArtifact)
	}
}

func TestAdminWriteAPIUsageExportArtifactHandlerAcceptsEmptyBody(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		apiUsageExportArtifact: maildb.APIUsageExportArtifactView{
			ID:          "api-usage-artifact-1",
			BatchID:     "api-usage-export-1",
			ContentType: "application/x-ndjson",
			SHA256Hex:   strings.Repeat("a", 64),
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodPost, "/admin/v1/api-usage/export-batches/api-usage-export-1/artifacts/write", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	if service.lastAPIUsageExportBatchID != "api-usage-export-1" {
		t.Fatalf("last batch = %q", service.lastAPIUsageExportBatchID)
	}
	if service.lastWriteAPIUsageExportArtifact.ObjectKey != "" || len(service.lastWriteAPIUsageExportArtifact.Metadata) != 0 {
		t.Fatalf("last write request = %+v", service.lastWriteAPIUsageExportArtifact)
	}
}

func TestAdminListAPIUsageExportArtifactsHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		apiUsageExportArtifacts: []maildb.APIUsageExportArtifactView{{
			ID:             "api-usage-artifact-1",
			BatchID:        "api-usage-export-1",
			StorageBackend: "s3",
			ObjectKey:      "exports/api-usage-export-1.ndjson",
			ContentType:    "application/x-ndjson",
			SHA256Hex:      strings.Repeat("a", 64),
			Metadata:       json.RawMessage(`{}`),
		}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/api-usage/export-batches/api-usage-export-1/artifacts?limit=5", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var response struct {
		Artifacts []maildb.APIUsageExportArtifactView `json:"api_usage_export_artifacts"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Artifacts) != 1 || response.Artifacts[0].ID != "api-usage-artifact-1" {
		t.Fatalf("artifacts = %+v", response.Artifacts)
	}
	if service.lastAPIUsageExportBatchID != "api-usage-export-1" || service.lastLimit != 5 {
		t.Fatalf("last batch/limit = %q/%d", service.lastAPIUsageExportBatchID, service.lastLimit)
	}
}

func TestAdminGetAPIUsageExportArtifactHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		apiUsageExportArtifact: maildb.APIUsageExportArtifactView{
			ID:             "api-usage-artifact-1",
			BatchID:        "api-usage-export-1",
			StorageBackend: "s3",
			ObjectKey:      "exports/api-usage-export-1.ndjson",
			ContentType:    "application/x-ndjson",
			SHA256Hex:      strings.Repeat("a", 64),
			Metadata:       json.RawMessage(`{}`),
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/api-usage/export-batches/api-usage-export-1/artifacts/api-usage-artifact-1", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var response struct {
		Artifact maildb.APIUsageExportArtifactView `json:"api_usage_export_artifact"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Artifact.ID != "api-usage-artifact-1" {
		t.Fatalf("artifact = %+v", response.Artifact)
	}
	if service.lastAPIUsageExportBatchID != "api-usage-export-1" || service.lastAPIUsageExportArtifactID != "api-usage-artifact-1" {
		t.Fatalf("last ids = %q/%q", service.lastAPIUsageExportBatchID, service.lastAPIUsageExportArtifactID)
	}
}

func TestAdminDownloadAPIUsageExportArtifactHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		apiUsageExportArtifact: maildb.APIUsageExportArtifactView{
			ID:          "api-usage-artifact-1",
			BatchID:     "api-usage-export-1",
			ContentType: "application/x-ndjson",
			SHA256Hex:   strings.Repeat("a", 64),
		},
		apiUsageExportArtifactBody: " {\"event_id\":\"usage-1\"}\n",
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/api-usage/export-batches/api-usage-export-1/artifacts/api-usage-artifact-1/download", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Content-Type"); got != "application/x-ndjson" {
		t.Fatalf("content type = %q", got)
	}
	if got := rr.Header().Get("X-Gogomail-Artifact-SHA256"); got != strings.Repeat("a", 64) {
		t.Fatalf("sha header = %q", got)
	}
	if got := rr.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("cache control = %q", got)
	}
	if got := rr.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("x-content-type-options = %q", got)
	}
	if !strings.Contains(rr.Body.String(), `"event_id":"usage-1"`) {
		t.Fatalf("body = %q", rr.Body.String())
	}
	if service.lastAPIUsageExportBatchID != "api-usage-export-1" || service.lastAPIUsageExportArtifactID != "api-usage-artifact-1" {
		t.Fatalf("last ids = %q/%q", service.lastAPIUsageExportBatchID, service.lastAPIUsageExportArtifactID)
	}
}

func TestAdminDownloadAPIUsageExportArtifactHandlerSanitizesHeaders(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		apiUsageExportArtifact: maildb.APIUsageExportArtifactView{
			ID:          "api-usage-artifact-1",
			BatchID:     "api-usage-export-1",
			ContentType: "application/x-ndjson\r\nX-Bad: yes",
			SHA256Hex:   strings.Repeat("a", 63) + "\n",
		},
		apiUsageExportArtifactBody: "{}\n",
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/api-usage/export-batches/api-usage-export-1/artifacts/api-usage-artifact-1/download", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Content-Type"); got != "application/x-ndjson" {
		t.Fatalf("content type = %q", got)
	}
	if got := rr.Header().Get("X-Gogomail-Artifact-SHA256"); got != "" {
		t.Fatalf("sha header = %q", got)
	}
}

func TestAdminVerifyAPIUsageExportArtifactHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		apiUsageExportArtifactVerification: maildb.APIUsageExportArtifactVerificationView{
			BatchID:           "api-usage-export-1",
			ArtifactID:        "api-usage-artifact-1",
			ObjectKey:         "exports/api-usage/api-usage-export-1.ndjson",
			ExpectedByteCount: 12,
			ActualByteCount:   12,
			ExpectedSHA256Hex: strings.Repeat("a", 64),
			ActualSHA256Hex:   strings.Repeat("a", 64),
			Valid:             true,
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/api-usage/export-batches/api-usage-export-1/artifacts/api-usage-artifact-1/verification", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var response struct {
		Verification maildb.APIUsageExportArtifactVerificationView `json:"api_usage_export_artifact_verification"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.Verification.Valid {
		t.Fatalf("verification = %+v", response.Verification)
	}
	if service.lastAPIUsageExportBatchID != "api-usage-export-1" || service.lastAPIUsageExportArtifactID != "api-usage-artifact-1" {
		t.Fatalf("last ids = %q/%q", service.lastAPIUsageExportBatchID, service.lastAPIUsageExportArtifactID)
	}
}

func TestAdminCreateAPIUsageExportManifestDigestHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		apiUsageExportManifestDigest: maildb.APIUsageExportManifestDigestView{
			ID:              "api-usage-manifest-1",
			BatchID:         "api-usage-export-1",
			SchemaVersion:   "2026-05-04.api-usage-export-manifest.v1",
			DigestAlgorithm: "sha256",
			DigestHex:       strings.Repeat("a", 64),
			Manifest:        json.RawMessage(`{"schema_version":"2026-05-04.api-usage-export-manifest.v1"}`),
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodPost, "/admin/v1/api-usage/export-batches/api-usage-export-1/manifest-digests", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var response struct {
		Digest maildb.APIUsageExportManifestDigestView `json:"api_usage_export_manifest_digest"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Digest.ID != "api-usage-manifest-1" || service.lastAPIUsageExportBatchID != "api-usage-export-1" {
		t.Fatalf("digest = %+v lastBatch=%q", response.Digest, service.lastAPIUsageExportBatchID)
	}
}

func TestAdminListAPIUsageExportManifestDigestsHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		apiUsageExportManifestDigests: []maildb.APIUsageExportManifestDigestView{{
			ID:              "api-usage-manifest-1",
			BatchID:         "api-usage-export-1",
			SchemaVersion:   "2026-05-04.api-usage-export-manifest.v1",
			DigestAlgorithm: "sha256",
			DigestHex:       strings.Repeat("a", 64),
			Manifest:        json.RawMessage(`{}`),
		}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/api-usage/export-batches/api-usage-export-1/manifest-digests?limit=5", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var response struct {
		Digests []maildb.APIUsageExportManifestDigestView `json:"api_usage_export_manifest_digests"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Digests) != 1 || response.Digests[0].ID != "api-usage-manifest-1" {
		t.Fatalf("digests = %+v", response.Digests)
	}
	if service.lastAPIUsageExportBatchID != "api-usage-export-1" || service.lastLimit != 5 {
		t.Fatalf("last batch/limit = %q/%d", service.lastAPIUsageExportBatchID, service.lastLimit)
	}
}

func TestAdminGetAPIUsageExportManifestDigestHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		apiUsageExportManifestDigest: maildb.APIUsageExportManifestDigestView{
			ID:              "api-usage-manifest-1",
			BatchID:         "api-usage-export-1",
			SchemaVersion:   "2026-05-04.api-usage-export-manifest.v1",
			DigestAlgorithm: "sha256",
			DigestHex:       strings.Repeat("a", 64),
			Manifest:        json.RawMessage(`{}`),
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/api-usage/export-batches/api-usage-export-1/manifest-digests/api-usage-manifest-1", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var response struct {
		Digest maildb.APIUsageExportManifestDigestView `json:"api_usage_export_manifest_digest"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Digest.ID != "api-usage-manifest-1" {
		t.Fatalf("digest = %+v", response.Digest)
	}
	if service.lastAPIUsageExportBatchID != "api-usage-export-1" || service.lastAPIUsageExportManifestDigestID != "api-usage-manifest-1" {
		t.Fatalf("last ids = %q/%q", service.lastAPIUsageExportBatchID, service.lastAPIUsageExportManifestDigestID)
	}
}

func TestAdminVerifyAPIUsageExportManifestDigestHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		apiUsageExportManifestDigestVerification: maildb.APIUsageExportManifestDigestVerificationView{
			BatchID:           "api-usage-export-1",
			DigestID:          "api-usage-manifest-1",
			SchemaVersion:     "2026-05-04.api-usage-export-manifest.v1",
			DigestAlgorithm:   "sha256",
			ExpectedDigestHex: strings.Repeat("a", 64),
			ActualDigestHex:   strings.Repeat("a", 64),
			Valid:             true,
			CanonicalManifest: json.RawMessage(`{}`),
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/api-usage/export-batches/api-usage-export-1/manifest-digests/api-usage-manifest-1/verification", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var response struct {
		Verification maildb.APIUsageExportManifestDigestVerificationView `json:"api_usage_export_manifest_digest_verification"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.Verification.Valid {
		t.Fatalf("verification = %+v", response.Verification)
	}
	if service.lastAPIUsageExportBatchID != "api-usage-export-1" || service.lastAPIUsageExportManifestDigestID != "api-usage-manifest-1" {
		t.Fatalf("last ids = %q/%q", service.lastAPIUsageExportBatchID, service.lastAPIUsageExportManifestDigestID)
	}
}

func TestAdminCreateAPIUsageExportManifestSignatureHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		apiUsageExportManifestSignature: maildb.APIUsageExportManifestSignatureView{
			ID:                 "api-usage-signature-1",
			DigestID:           "api-usage-manifest-1",
			BatchID:            "api-usage-export-1",
			SignerBackend:      "local-hmac",
			KeyID:              "key-1",
			SignatureAlgorithm: "hmac-sha256",
			SignedDigestHex:    strings.Repeat("a", 64),
			SignatureHex:       strings.Repeat("b", 64),
			Metadata:           json.RawMessage(`{}`),
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodPost, "/admin/v1/api-usage/export-batches/api-usage-export-1/manifest-digests/api-usage-manifest-1/signatures", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var response struct {
		Signature maildb.APIUsageExportManifestSignatureView `json:"api_usage_export_manifest_signature"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Signature.ID != "api-usage-signature-1" {
		t.Fatalf("signature = %+v", response.Signature)
	}
	if service.lastAPIUsageExportBatchID != "api-usage-export-1" || service.lastAPIUsageExportManifestDigestID != "api-usage-manifest-1" {
		t.Fatalf("last ids = %q/%q", service.lastAPIUsageExportBatchID, service.lastAPIUsageExportManifestDigestID)
	}
}

func TestAdminListAPIUsageExportManifestSignaturesHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		apiUsageExportManifestSignatures: []maildb.APIUsageExportManifestSignatureView{{
			ID:                 "api-usage-signature-1",
			DigestID:           "api-usage-manifest-1",
			BatchID:            "api-usage-export-1",
			SignerBackend:      "local-hmac",
			KeyID:              "key-1",
			SignatureAlgorithm: "hmac-sha256",
			SignedDigestHex:    strings.Repeat("a", 64),
			SignatureHex:       strings.Repeat("b", 64),
			Metadata:           json.RawMessage(`{}`),
		}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/api-usage/export-batches/api-usage-export-1/manifest-digests/api-usage-manifest-1/signatures?limit=5", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var response struct {
		Signatures []maildb.APIUsageExportManifestSignatureView `json:"api_usage_export_manifest_signatures"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Signatures) != 1 || response.Signatures[0].ID != "api-usage-signature-1" {
		t.Fatalf("signatures = %+v", response.Signatures)
	}
	if service.lastAPIUsageExportBatchID != "api-usage-export-1" || service.lastAPIUsageExportManifestDigestID != "api-usage-manifest-1" || service.lastLimit != 5 {
		t.Fatalf("last = %q/%q/%d", service.lastAPIUsageExportBatchID, service.lastAPIUsageExportManifestDigestID, service.lastLimit)
	}
}

func TestAdminGetAPIUsageExportManifestSignatureHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		apiUsageExportManifestSignature: maildb.APIUsageExportManifestSignatureView{
			ID:                 "api-usage-signature-1",
			DigestID:           "api-usage-manifest-1",
			BatchID:            "api-usage-export-1",
			SignerBackend:      "local-hmac",
			KeyID:              "key-1",
			SignatureAlgorithm: "hmac-sha256",
			SignedDigestHex:    strings.Repeat("a", 64),
			SignatureHex:       strings.Repeat("b", 64),
			Metadata:           json.RawMessage(`{}`),
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/api-usage/export-batches/api-usage-export-1/manifest-digests/api-usage-manifest-1/signatures/api-usage-signature-1", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var response struct {
		Signature maildb.APIUsageExportManifestSignatureView `json:"api_usage_export_manifest_signature"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Signature.ID != "api-usage-signature-1" {
		t.Fatalf("signature = %+v", response.Signature)
	}
	if service.lastAPIUsageExportBatchID != "api-usage-export-1" || service.lastAPIUsageExportManifestDigestID != "api-usage-manifest-1" || service.lastAPIUsageExportManifestSignatureID != "api-usage-signature-1" {
		t.Fatalf("last ids = %q/%q/%q", service.lastAPIUsageExportBatchID, service.lastAPIUsageExportManifestDigestID, service.lastAPIUsageExportManifestSignatureID)
	}
}

func TestAdminVerifyAPIUsageExportManifestSignatureHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		apiUsageExportManifestSignatureVerification: maildb.APIUsageExportManifestSignatureVerificationView{
			BatchID:            "api-usage-export-1",
			DigestID:           "api-usage-manifest-1",
			SignatureID:        "api-usage-signature-1",
			SignerBackend:      "local-hmac",
			KeyID:              "key-1",
			SignatureAlgorithm: "hmac-sha256",
			SignedDigestHex:    strings.Repeat("a", 64),
			ExpectedDigestHex:  strings.Repeat("a", 64),
			Valid:              true,
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/api-usage/export-batches/api-usage-export-1/manifest-digests/api-usage-manifest-1/signatures/api-usage-signature-1/verification", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var response struct {
		Verification maildb.APIUsageExportManifestSignatureVerificationView `json:"api_usage_export_manifest_signature_verification"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.Verification.Valid {
		t.Fatalf("verification = %+v", response.Verification)
	}
	if service.lastAPIUsageExportBatchID != "api-usage-export-1" || service.lastAPIUsageExportManifestDigestID != "api-usage-manifest-1" || service.lastAPIUsageExportManifestSignatureID != "api-usage-signature-1" {
		t.Fatalf("last ids = %q/%q/%q", service.lastAPIUsageExportBatchID, service.lastAPIUsageExportManifestDigestID, service.lastAPIUsageExportManifestSignatureID)
	}
}

func TestAdminQuotaReconciliationHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		quotaReconciliation: []maildb.QuotaReconciliationView{{
			Scope:      "user",
			ID:         "user-1",
			DomainID:   "domain-1",
			Name:       "admin@example.com",
			LedgerUsed: 1200,
			ActualUsed: 1000,
			Delta:      200,
			InSync:     false,
		}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/quota-reconciliation?limit=7", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		QuotaReconciliation []maildb.QuotaReconciliationView `json:"quota_reconciliation"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if len(body.QuotaReconciliation) != 1 || body.QuotaReconciliation[0].Delta != 200 {
		t.Fatalf("quota_reconciliation = %+v", body.QuotaReconciliation)
	}
	if service.lastLimit != 7 {
		t.Fatalf("lastLimit = %d, want 7", service.lastLimit)
	}
}

func TestAdminQuotaCorrectionHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		quotaCorrection: maildb.QuotaCorrectionResult{
			DryRun: true,
			Corrected: []maildb.QuotaReconciliationView{{
				Scope:      "domain",
				ID:         "domain-1",
				LedgerUsed: 10,
				ActualUsed: 20,
				Delta:      -10,
			}},
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodPost, "/admin/v1/quota-reconciliation/corrections", strings.NewReader(`{"scope":"domain","id":"domain-1","dry_run":true}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastQuotaCorrection.Scope != "domain" || service.lastQuotaCorrection.ID != "domain-1" || !service.lastQuotaCorrection.DryRun {
		t.Fatalf("lastQuotaCorrection = %+v", service.lastQuotaCorrection)
	}
	var body struct {
		QuotaCorrection maildb.QuotaCorrectionResult `json:"quota_correction"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if len(body.QuotaCorrection.Corrected) != 1 || body.QuotaCorrection.Corrected[0].Delta != -10 {
		t.Fatalf("quota_correction = %+v", body.QuotaCorrection)
	}
}

func TestAdminCompaniesHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		companies: []maildb.CompanyView{{ID: "company-1", Name: "Gogo Co", Status: "active", QuotaLimit: 10_000}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/companies?limit=10&status=suspended", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Companies []maildb.CompanyView `json:"companies"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if len(body.Companies) != 1 || body.Companies[0].Name != "Gogo Co" {
		t.Fatalf("companies = %+v", body.Companies)
	}
	if service.lastLimit != 10 {
		t.Fatalf("lastLimit = %d, want 10", service.lastLimit)
	}
	if service.lastCompanyList.Status != "suspended" {
		t.Fatalf("lastCompanyList = %+v, want status filter", service.lastCompanyList)
	}
}

func TestAdminCompaniesHandlerRejectsUnsafeStatusFilter(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/companies?status=active%0Abad", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if service.lastCompanyList.Status != "" {
		t.Fatalf("lastCompanyList = %+v, want no dispatch", service.lastCompanyList)
	}
}

func TestAdminGetCompanyHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		companies: []maildb.CompanyView{{ID: "company-1", Name: "Gogo Co", Status: "active"}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/companies/company-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Company maildb.CompanyView `json:"company"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if body.Company.ID != "company-1" || service.lastCompanyID != "company-1" {
		t.Fatalf("company = %+v lastCompanyID=%q", body.Company, service.lastCompanyID)
	}
}

func TestAdminUpdateCompanyQuotaHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodPatch, "/admin/v1/companies/%20company-1%20/quota", bytes.NewReader([]byte(`{"quota_limit":8192}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastCompanyQuota.ID != "company-1" || service.lastCompanyQuota.QuotaLimit != 8192 {
		t.Fatalf("lastCompanyQuota = %+v", service.lastCompanyQuota)
	}
}

func TestAdminCompanyPathIDsRejectUnsafeValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{
			name:   "get crlf",
			method: http.MethodGet,
			path:   "/admin/v1/companies/company%0Abad",
		},
		{
			name:   "quota oversized",
			method: http.MethodPatch,
			path:   "/admin/v1/companies/" + strings.Repeat("c", maxAdminQueryFilterBytes+1) + "/quota",
			body:   `{"quota_limit":8192}`,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := &fakeAdminService{}
			mux := http.NewServeMux()
			RegisterAdminRoutes(mux, service, "")

			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if service.lastCompanyID != "" || service.lastCompanyQuota.ID != "" {
				t.Fatalf("last company ids = %q/%q", service.lastCompanyID, service.lastCompanyQuota.ID)
			}
		})
	}
}

func TestAdminJSONHandlersRejectTrailingTokens(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodPatch, "/admin/v1/companies/company-1/quota", bytes.NewReader([]byte(`{"quota_limit":8192} {"quota_limit":1}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastCompanyQuota.ID != "" {
		t.Fatalf("handler should not dispatch trailing-token body: %+v", service.lastCompanyQuota)
	}
}

func TestAdminJSONHandlersRejectUnknownFields(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodPatch, "/admin/v1/companies/company-1/quota", bytes.NewReader([]byte(`{"quota_limit":8192,"unexpected":true}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastCompanyQuota.ID != "" {
		t.Fatalf("handler should not dispatch unknown-field body: %+v", service.lastCompanyQuota)
	}
}

func TestAdminJSONHandlersRejectMissingOrNonJSONContentType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		contentType string
		extra       string
	}{
		{name: "missing"},
		{name: "text plain", contentType: "text/plain"},
		{name: "duplicate", contentType: "application/json", extra: "application/json"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := &fakeAdminService{}
			mux := http.NewServeMux()
			RegisterAdminRoutes(mux, service, "")

			req := httptest.NewRequest(http.MethodPatch, "/admin/v1/companies/company-1/quota", bytes.NewReader([]byte(`{"quota_limit":8192}`)))
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}
			if tt.extra != "" {
				req.Header.Add("Content-Type", tt.extra)
			}
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if service.lastCompanyQuota.ID != "" {
				t.Fatalf("handler should not dispatch non-json content type: %+v", service.lastCompanyQuota)
			}
		})
	}
}

func TestAdminAuthAcceptsBearerToken(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		queueStats: []maildb.QueueStat{{Topic: "mail.outbound.general", Status: "pending", Count: 1}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "secret")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/queue", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestAdminAuthRejectsWrongLengthToken(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "secret")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/queue", nil)
	req.Header.Set("Authorization", "Bearer secrets")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestAdminWebhooksRejectPrivateURL(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodPost, "/admin/v1/companies/company-1/webhooks", strings.NewReader(`{"name":"local","url":"http://127.0.0.1:8080/hook","events":["mail.received"],"enabled":true}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestAdminWebhooksAllowPrivateURLWhenCompanyGovernanceAllows(t *testing.T) {
	t.Parallel()

	raw := json.RawMessage(`{"security_profile":"enterprise","webhook_private_network_access":"allow"}`)
	service := &fakeAdminService{companyConfig: []configstore.ConfigEntry{{Value: raw}}}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodPost, "/admin/v1/companies/company-1/webhooks", strings.NewReader(`{"name":"local","url":"http://127.0.0.1:8080/hook","events":["mail.received"],"enabled":true}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastCompanyConfigKey != webhooksConfigKey {
		t.Fatalf("lastCompanyConfigKey = %q, want %q", service.lastCompanyConfigKey, webhooksConfigKey)
	}
}

func TestAdminCompanySecurityGovernancePolicyRejectsInvalidRelaxation(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodPut, "/admin/v1/companies/company-1/security/governance", strings.NewReader(`{"security_profile":"enterprise","webhook_private_network_access":"private"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestAdminWebhooksListRedactsSecret(t *testing.T) {
	t.Parallel()

	cfg := webhooksConfig{Webhooks: []webhook{{
		ID:      "wh-1",
		Name:    "hook",
		URL:     "https://hooks.example.test/mail",
		Secret:  "0123456789abcdef",
		Events:  []string{"mail.received"},
		Enabled: true,
	}}}
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}
	service := &fakeAdminService{companyConfig: []configstore.ConfigEntry{{Value: raw}}}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/companies/company-1/webhooks", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "0123456789abcdef") {
		t.Fatalf("secret leaked in response: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"secret_suffix":"89abcdef"`) {
		t.Fatalf("secret suffix missing: %s", rec.Body.String())
	}
}

func TestAdminLoginIssuesSignedAccessAndRefreshTokens(t *testing.T) {
	t.Parallel()

	manager, err := auth.NewTokenManager("admin-auth-secret")
	if err != nil {
		t.Fatalf("NewTokenManager returned error: %v", err)
	}
	service := &fakeAdminService{
		authenticatedUser: maildb.AuthenticatedUser{UserID: "user-1", DomainID: "domain-1", SessionVersion: 4},
		users:             []maildb.UserView{{ID: "user-1", DomainID: "domain-1", Role: "company_admin"}},
		domains:           []maildb.DomainView{{ID: "domain-1", CompanyID: "company-1"}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "", WithTokenManager(manager))

	req := httptest.NewRequest(http.MethodPost, "/admin/v1/auth/login", strings.NewReader(`{"email":"admin@example.com","password":"secret"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		User         struct {
			ID        string `json:"id"`
			Role      string `json:"role"`
			CompanyID string `json:"company_id"`
		} `json:"user"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	accessClaims, err := manager.Verify(body.AccessToken)
	if err != nil {
		t.Fatalf("verify access token: %v", err)
	}
	refreshClaims, err := manager.Verify(body.RefreshToken)
	if err != nil {
		t.Fatalf("verify refresh token: %v", err)
	}
	if accessClaims.UserID != "user-1" || accessClaims.CompanyID != "company-1" || accessClaims.Role != "company_admin" || accessClaims.SessionVersion != 4 {
		t.Fatalf("access claims = %+v", accessClaims)
	}
	if accessClaims.TokenType != "access" || refreshClaims.TokenType != "refresh" || refreshClaims.UserID != accessClaims.UserID || refreshClaims.SessionVersion != accessClaims.SessionVersion || !refreshClaims.Expires.After(accessClaims.Expires) {
		t.Fatalf("refresh claims = %+v, access = %+v", refreshClaims, accessClaims)
	}
	if body.User.ID != "user-1" || body.User.CompanyID != "company-1" {
		t.Fatalf("user = %+v", body.User)
	}
}

func TestAdminLoginRejectsHardcodedBootstrapCredentials(t *testing.T) {
	// Ensure env vars are not set — hardcoded credentials must never work.
	t.Setenv("GOGOMAIL_ADMIN_BOOTSTRAP_EMAIL", "")
	t.Setenv("GOGOMAIL_ADMIN_BOOTSTRAP_PASSWORD", "")

	manager, err := auth.NewTokenManager("admin-auth-secret")
	if err != nil {
		t.Fatalf("NewTokenManager returned error: %v", err)
	}
	for _, env := range []string{"production", "test", "development", ""} {
		mux := http.NewServeMux()
		RegisterAdminRoutes(mux, &fakeAdminService{}, "", WithTokenManager(manager), WithEnvironment(env))

		req := httptest.NewRequest(http.MethodPost, "/admin/v1/auth/login", strings.NewReader(`{"email":"admin@system","password":"admin1234"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("env=%q: expected 401, got status=%d body=%s", env, rec.Code, rec.Body.String())
		}
	}
}

func TestAdminLoginAllowsBootstrapViaEnvVars(t *testing.T) {
	t.Setenv("GOGOMAIL_ADMIN_BOOTSTRAP_EMAIL", "bootstrap@example.com")
	t.Setenv("GOGOMAIL_ADMIN_BOOTSTRAP_PASSWORD", "s3cr3t-b00tstrap!")

	manager, err := auth.NewTokenManager("admin-auth-secret")
	if err != nil {
		t.Fatalf("NewTokenManager returned error: %v", err)
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, &fakeAdminService{}, "", WithTokenManager(manager))

	// Correct credentials should succeed.
	req := httptest.NewRequest(http.MethodPost, "/admin/v1/auth/login", strings.NewReader(`{"email":"bootstrap@example.com","password":"s3cr3t-b00tstrap!"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("correct credentials: status=%d body=%s", rec.Code, rec.Body.String())
	}

	// Wrong password should fail.
	req2 := httptest.NewRequest(http.MethodPost, "/admin/v1/auth/login", strings.NewReader(`{"email":"bootstrap@example.com","password":"wrong"}`))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password: status=%d body=%s", rec2.Code, rec2.Body.String())
	}
}

func TestAdminVerifyRequiresValidBearerJWT(t *testing.T) {
	t.Parallel()

	manager, err := auth.NewTokenManager("admin-auth-secret")
	if err != nil {
		t.Fatalf("NewTokenManager returned error: %v", err)
	}
	service := &fakeAdminService{sessionVersions: map[string]int64{"user-1": 2}}
	manager.SetRevocationChecker(service)
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "", WithTokenManager(manager))

	missingReq := httptest.NewRequest(http.MethodGet, "/admin/v1/auth/verify", nil)
	missingRec := httptest.NewRecorder()
	mux.ServeHTTP(missingRec, missingReq)
	if missingRec.Code != http.StatusUnauthorized {
		t.Fatalf("missing status = %d, body = %s", missingRec.Code, missingRec.Body.String())
	}

	revoked, err := manager.Sign(auth.Claims{UserID: "user-1", DomainID: "domain-1", CompanyID: "company-1", Role: "company_admin", SessionVersion: 1}, time.Minute)
	if err != nil {
		t.Fatalf("Sign revoked returned error: %v", err)
	}
	revokedReq := httptest.NewRequest(http.MethodGet, "/admin/v1/auth/verify", nil)
	revokedReq.Header.Set("Authorization", "Bearer "+revoked)
	revokedRec := httptest.NewRecorder()
	mux.ServeHTTP(revokedRec, revokedReq)
	if revokedRec.Code != http.StatusUnauthorized {
		t.Fatalf("revoked status = %d, body = %s", revokedRec.Code, revokedRec.Body.String())
	}

	valid, err := manager.Sign(auth.Claims{UserID: "user-1", DomainID: "domain-1", CompanyID: "company-1", Role: "company_admin", SessionVersion: 2}, time.Minute)
	if err != nil {
		t.Fatalf("Sign valid returned error: %v", err)
	}
	validReq := httptest.NewRequest(http.MethodGet, "/admin/v1/auth/verify", nil)
	validReq.Header.Set("Authorization", "Bearer "+valid)
	validRec := httptest.NewRecorder()
	mux.ServeHTTP(validRec, validReq)
	if validRec.Code != http.StatusOK {
		t.Fatalf("valid status = %d, body = %s", validRec.Code, validRec.Body.String())
	}
	var body struct {
		Authenticated bool   `json:"authenticated"`
		UserID        string `json:"user_id"`
		CompanyID     string `json:"company_id"`
	}
	if err := json.Unmarshal(validRec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if !body.Authenticated || body.UserID != "user-1" || body.CompanyID != "company-1" {
		t.Fatalf("verify body = %+v", body)
	}
}

func TestAdminRefreshIssuesNewAccessToken(t *testing.T) {
	t.Parallel()

	manager, err := auth.NewTokenManager("admin-auth-secret")
	if err != nil {
		t.Fatalf("NewTokenManager returned error: %v", err)
	}
	service := &fakeAdminService{sessionVersions: map[string]int64{"user-1": 3}}
	manager.SetRevocationChecker(service)
	refreshToken, err := manager.Sign(auth.Claims{UserID: "user-1", DomainID: "domain-1", CompanyID: "company-1", Role: "company_admin", SessionVersion: 3, TokenType: "refresh"}, 24*time.Hour)
	if err != nil {
		t.Fatalf("Sign returned error: %v", err)
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "", WithTokenManager(manager))

	req := httptest.NewRequest(http.MethodPost, "/admin/v1/auth/refresh", strings.NewReader(`{"refresh_token":"`+refreshToken+`"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	claims, err := manager.VerifyFull(context.Background(), body.AccessToken)
	if err != nil {
		t.Fatalf("VerifyFull access token returned error: %v", err)
	}
	if claims.UserID != "user-1" || claims.Role != "company_admin" || claims.SessionVersion != 3 {
		t.Fatalf("claims = %+v", claims)
	}
}

func TestAdminLogoutRevokesBearerSession(t *testing.T) {
	t.Parallel()

	manager, err := auth.NewTokenManager("admin-auth-secret")
	if err != nil {
		t.Fatalf("NewTokenManager returned error: %v", err)
	}
	service := &fakeAdminService{sessionVersions: map[string]int64{"user-1": 7}}
	manager.SetRevocationChecker(service)
	token, err := manager.Sign(auth.Claims{UserID: "user-1", DomainID: "domain-1", CompanyID: "company-1", Role: "company_admin", SessionVersion: 7}, time.Hour)
	if err != nil {
		t.Fatalf("Sign returned error: %v", err)
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "", WithTokenManager(manager))

	req := httptest.NewRequest(http.MethodPost, "/admin/v1/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.sessionVersions["user-1"] != 8 || service.lastSessionRevokedUserID != "user-1" {
		t.Fatalf("session versions = %+v revoked = %q", service.sessionVersions, service.lastSessionRevokedUserID)
	}
	if _, err := manager.VerifyFull(context.Background(), token); err == nil {
		t.Fatal("VerifyFull accepted token after logout revocation")
	}
}

func TestAdminAuthRejectsOversizedAuthorizationHeaders(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		header string
		value  string
	}{
		{
			name:   "bearer",
			header: "Authorization",
			value:  strings.Repeat("a", maxHTTPAuthHeaderBytes+1),
		},
		{
			name:   "admin_token",
			header: "X-Admin-Token",
			value:  strings.Repeat("a", maxHTTPAuthHeaderBytes+1),
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			service := &fakeAdminService{
				queueStats: []maildb.QueueStat{{Topic: "mail.outbound.general", Status: "pending", Count: 1}},
			}
			mux := http.NewServeMux()
			RegisterAdminRoutes(mux, service, "secret")

			req := httptest.NewRequest(http.MethodGet, "/admin/v1/queue", nil)
			req.Header.Set(tc.header, tc.value)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestAdminAuthRejectsAmbiguousAuthHeaders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		build func(http.Header)
	}{
		{
			name: "duplicate authorization",
			build: func(header http.Header) {
				header.Add("Authorization", "Bearer one")
				header.Add("Authorization", "Bearer two")
			},
		},
		{
			name: "duplicate admin token",
			build: func(header http.Header) {
				header.Add("X-Admin-Token", "one")
				header.Add("X-Admin-Token", "two")
			},
		},
		{
			name: "admin token and authorization",
			build: func(header http.Header) {
				header.Set("X-Admin-Token", "secret")
				header.Set("Authorization", "Bearer secret")
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := &fakeAdminService{
				queueStats: []maildb.QueueStat{{Topic: "mail.outbound.general", Status: "pending", Count: 1}},
			}
			mux := http.NewServeMux()
			RegisterAdminRoutes(mux, service, "secret")

			req := httptest.NewRequest(http.MethodGet, "/admin/v1/queue", nil)
			tt.build(req.Header)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestAdminDomainsHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		domains: []maildb.DomainView{{ID: "domain-1", Name: "example.com", Status: "active"}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/domains?company_id=%20company-1%20&status=active&dns_status=ok&limit=10", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Domains []maildb.DomainView `json:"domains"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if len(body.Domains) != 1 || body.Domains[0].Name != "example.com" {
		t.Fatalf("domains = %+v", body.Domains)
	}
	if service.lastLimit != 10 {
		t.Fatalf("lastLimit = %d", service.lastLimit)
	}
	if service.lastDomainList.CompanyID != "company-1" || service.lastDomainList.Status != "active" || service.lastDomainList.DNSStatus != "ok" {
		t.Fatalf("lastDomainList = %+v", service.lastDomainList)
	}
}

func TestAdminCoreListHandlersRejectUnknownQueryParameters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		path       string
		dispatched func(*fakeAdminService) bool
	}{
		{
			name: "companies",
			path: "/admin/v1/companies?limit=10&state=active",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastLimit != 0
			},
		},
		{
			name: "domains",
			path: "/admin/v1/domains?company_id=company-1&dns_state=ok",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastDomainList.CompanyID != ""
			},
		},
		{
			name: "domain dns checks",
			path: "/admin/v1/domains/domain-1/dns-checks?status=missing&from=2026-05-04T00:00:00Z",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastDomainDNSCheckList.DomainID != ""
			},
		},
		{
			name: "users",
			path: "/admin/v1/users?domain_id=domain-1&passwordReady=true",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastUserList.DomainID != ""
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := &fakeAdminService{}
			mux := http.NewServeMux()
			RegisterAdminRoutes(mux, service, "")

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if tt.dispatched(service) {
				t.Fatalf("handler dispatched for unknown query parameter: %+v", service)
			}
		})
	}
}

func TestAdminListHandlerRejectsInvalidLimit(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/domains?limit=bad", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestAdminListHandlerRejectsTooLargeLimit(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/domains?limit=201", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "limit must be at most 200") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestAdminDomainsHandlerRejectsUnsafeFilters(t *testing.T) {
	t.Parallel()

	tests := []string{
		"/admin/v1/domains?company_id=company%0Abad",
		"/admin/v1/domains?status=archived",
		"/admin/v1/domains?dns_status=stale",
		"/admin/v1/domains?dns_status=ok%0Abad",
	}

	for _, path := range tests {
		path := path
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			service := &fakeAdminService{}
			mux := http.NewServeMux()
			RegisterAdminRoutes(mux, service, "")

			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if service.lastDomainList.Limit != 0 {
				t.Fatalf("lastDomainList = %+v", service.lastDomainList)
			}
		})
	}
}

func TestAdminGetDomainHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		domains: []maildb.DomainView{{ID: "domain-1", Name: "example.com", Status: "active"}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/domains/domain-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Domain maildb.DomainView `json:"domain"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if body.Domain.ID != "domain-1" || service.lastDomainID != "domain-1" {
		t.Fatalf("domain = %+v lastDomainID=%q", body.Domain, service.lastDomainID)
	}
}

func TestAdminDomainDNSCheckHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		dnsReport: dnscheck.DomainReport{
			Domain: "example.com",
			MX:     dnscheck.RecordCheck{Name: "mx", Host: "example.com", Status: dnscheck.StatusOK, Found: []string{"mx.example.com"}},
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/domains/domain-1/dns-check", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		DNSCheck dnscheck.DomainReport `json:"dns_check"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if body.DNSCheck.Domain != "example.com" || service.lastDomainID != "domain-1" {
		t.Fatalf("dns_check = %+v lastDomainID=%q", body.DNSCheck, service.lastDomainID)
	}
}

func TestAdminDomainDNSCheckHistoryHandler(t *testing.T) {
	t.Parallel()

	checkedAt := time.Date(2026, 5, 4, 10, 0, 0, 0, time.UTC)
	service := &fakeAdminService{
		dnsChecks: []maildb.DomainDNSCheckView{{
			ID:        "check-1",
			DomainID:  "domain-1",
			Status:    "ok",
			CheckedAt: checkedAt,
			Report: dnscheck.DomainReport{
				Domain: "example.com",
				MX:     dnscheck.RecordCheck{Name: "mx", Host: "example.com", Status: dnscheck.StatusOK},
			},
		}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/domains/domain-1/dns-checks?limit=5&status=missing&since=2026-05-04T00:00:00Z", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		DNSChecks []maildb.DomainDNSCheckView `json:"dns_checks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if len(body.DNSChecks) != 1 || body.DNSChecks[0].ID != "check-1" {
		t.Fatalf("dns_checks = %+v", body.DNSChecks)
	}
	if service.lastDomainDNSCheckList.DomainID != "domain-1" || service.lastDomainDNSCheckList.Limit != 5 || service.lastDomainDNSCheckList.Status != "missing" || service.lastDomainDNSCheckList.Since.IsZero() {
		t.Fatalf("lastDomainDNSCheckList=%+v", service.lastDomainDNSCheckList)
	}
}

func TestAdminDomainDNSCheckHistoryHandlerRejectsUnsafeFilters(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	tests := []string{
		"/admin/v1/domains/domain-1/dns-checks?status=pass",
		"/admin/v1/domains/domain-1/dns-checks?status=missing%0Abad",
		"/admin/v1/domains/domain-1/dns-checks?since=not-a-time",
	}
	for _, target := range tests {
		target := target
		t.Run(target, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, target, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if service.lastDomainDNSCheckList.DomainID != "" {
				t.Fatalf("dns check list was called: %+v", service.lastDomainDNSCheckList)
			}
		})
	}
}

func TestAdminDomainPathIDsRejectUnsafeValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{
			name:   "get crlf",
			method: http.MethodGet,
			path:   "/admin/v1/domains/domain%0Abad",
		},
		{
			name:   "stats oversized",
			method: http.MethodGet,
			path:   "/admin/v1/domains/" + strings.Repeat("d", maxAdminQueryFilterBytes+1) + "/stats",
		},
		{
			name:   "dns check crlf",
			method: http.MethodGet,
			path:   "/admin/v1/domains/domain%0Dbad/dns-check",
		},
		{
			name:   "dns checks oversized",
			method: http.MethodGet,
			path:   "/admin/v1/domains/" + strings.Repeat("d", maxAdminQueryFilterBytes+1) + "/dns-checks",
		},
		{
			name:   "status crlf",
			method: http.MethodPatch,
			path:   "/admin/v1/domains/domain%0Abad/status",
			body:   `{"status":"suspended"}`,
		},
		{
			name:   "quota oversized",
			method: http.MethodPatch,
			path:   "/admin/v1/domains/" + strings.Repeat("d", maxAdminQueryFilterBytes+1) + "/quota",
			body:   `{"quota_limit":1024}`,
		},
		{
			name:   "policy crlf",
			method: http.MethodPatch,
			path:   "/admin/v1/domains/domain%0Abad/policy",
			body:   `{"inbound_mode":"monitor"}`,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := &fakeAdminService{}
			mux := http.NewServeMux()
			RegisterAdminRoutes(mux, service, "")

			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if service.lastDomainID != "" || service.lastDomainStatus.ID != "" || service.lastDomainQuota.ID != "" || service.lastDomainPolicy.ID != "" {
				t.Fatalf("last domain ids = %q/%q/%q/%q", service.lastDomainID, service.lastDomainStatus.ID, service.lastDomainQuota.ID, service.lastDomainPolicy.ID)
			}
		})
	}
}

func TestAdminCreateDomainHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	body := []byte(`{"company_id":"company-1","name":"Example.COM","quota_limit":1024}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/v1/domains", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastCreateDomain.CompanyID != "company-1" || service.lastCreateDomain.Name != "Example.COM" {
		t.Fatalf("lastCreateDomain = %+v", service.lastCreateDomain)
	}
}

func TestAdminCreateDomainHandlerInheritsCompanyDomainSettings(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		companyConfig: []configstore.ConfigEntry{{
			ScopeID: "company-1",
			Key:     companyDomainSettingsDefaultsKey,
			Value: json.RawMessage(`{
				"tls_policy":"require",
				"quota_per_user":536870912,
				"ip_whitelist_enabled":true,
				"ip_whitelist":["10.0.0.0/8"],
				"require_2fa":true,
				"session_timeout_minutes":120,
				"password_min_length":12,
				"password_require_uppercase":true,
				"password_require_numbers":true,
				"password_require_special_chars":true,
				"password_expiry_days":90,
				"user_registration_mode":"email_invite",
				"password_reset_token_ttl_minutes":30
			}`),
		}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	body := []byte(`{"company_id":"company-1","name":"Example.COM","quota_limit":1024}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/v1/domains", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastCompanyConfigKey != companyDomainSettingsDefaultsKey {
		t.Fatalf("lastCompanyConfigKey = %q", service.lastCompanyConfigKey)
	}
	if service.lastDomainSettings.DomainID != "domain-new" || service.lastDomainSettings.TLSPolicy != "require" || service.lastDomainSettings.QuotaPerUser != 536870912 {
		t.Fatalf("lastDomainSettings = %+v", service.lastDomainSettings)
	}
	if service.lastDomainSettings.UserRegistrationMode != "email_invite" || service.lastDomainSettings.PasswordResetTokenTTLMinutes != 30 {
		t.Fatalf("lastDomainSettings registration/reset = %+v", service.lastDomainSettings)
	}
}

func TestAdminUpdateDomainStatusHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodPatch, "/admin/v1/domains/%20domain-1%20/status", bytes.NewReader([]byte(`{"status":"suspended"}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastDomainStatus.ID != "domain-1" || service.lastDomainStatus.Status != "suspended" {
		t.Fatalf("lastDomainStatus = %+v", service.lastDomainStatus)
	}
}

func TestAdminUpdateDomainQuotaHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodPatch, "/admin/v1/domains/%20domain-1%20/quota", bytes.NewReader([]byte(`{"quota_limit":2048}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastDomainQuota.ID != "domain-1" || service.lastDomainQuota.QuotaLimit != 2048 {
		t.Fatalf("lastDomainQuota = %+v", service.lastDomainQuota)
	}
}

func TestAdminUpdateDomainPolicyHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodPatch, "/admin/v1/domains/%20domain-1%20/policy", bytes.NewReader([]byte(`{
		"inbound_mode": "monitor",
		"outbound_mode": "enforce",
		"max_recipients_per_message": 50,
		"max_message_bytes": 1048576,
		"max_attachment_bytes": 524288
	}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastDomainPolicy.ID != "domain-1" || service.lastDomainPolicy.InboundMode != "monitor" {
		t.Fatalf("lastDomainPolicy = %+v", service.lastDomainPolicy)
	}
	if service.lastDomainPolicy.MaxAttachmentBytes != 524288 {
		t.Fatalf("MaxAttachmentBytes = %d, want 524288", service.lastDomainPolicy.MaxAttachmentBytes)
	}
	if !strings.Contains(rec.Body.String(), `"domain_policy"`) {
		t.Fatalf("response missing domain_policy envelope: %s", rec.Body.String())
	}
}

func TestAdminUsersHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		users: []maildb.UserView{{ID: "user-1", DomainID: "domain-1", Username: "admin", Status: "active"}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/users?domain_id=%20domain-1%20&status=active&password_configured=true&limit=10", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Users []maildb.UserView `json:"users"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if len(body.Users) != 1 || body.Users[0].Username != "admin" {
		t.Fatalf("users = %+v", body.Users)
	}
	if service.lastDomainID != "domain-1" || service.lastLimit != 10 {
		t.Fatalf("domain/limit = %q/%d", service.lastDomainID, service.lastLimit)
	}
	if service.lastUserList.Status != "active" {
		t.Fatalf("status filter = %q, want active", service.lastUserList.Status)
	}
	if service.lastUserList.PasswordConfigured == nil || !*service.lastUserList.PasswordConfigured {
		t.Fatalf("password_configured filter = %v, want true", service.lastUserList.PasswordConfigured)
	}
}

func TestAdminUsersHandlerRejectsUnsafeFilters(t *testing.T) {
	t.Parallel()

	tests := []string{
		"/admin/v1/users?domain_id=domain%0Abad",
		"/admin/v1/users?domain_id=" + strings.Repeat("d", maxAdminQueryFilterBytes+1),
		"/admin/v1/users?status=archived",
		"/admin/v1/users?status=active%0Abad",
		"/admin/v1/users?password_configured=maybe",
		"/admin/v1/users?password_configured=true%0Abad",
	}

	for _, path := range tests {
		path := path
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			service := &fakeAdminService{}
			mux := http.NewServeMux()
			RegisterAdminRoutes(mux, service, "")

			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if service.lastDomainID != "" || service.lastUserList.PasswordConfigured != nil {
				t.Fatalf("last user list = %+v", service.lastUserList)
			}
		})
	}
}

func TestAdminCompanyUsersBulkExportScopesUsersByCompanyDomains(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		domains: []maildb.DomainView{
			{ID: "domain-1", CompanyID: "company-1", Name: "example.com"},
			{ID: "domain-2", CompanyID: "company-2", Name: "other.test"},
		},
		users: []maildb.UserView{
			{ID: "user-1", DomainID: "domain-1", Username: "alice", DisplayName: "Alice", Status: "active"},
			{ID: "user-2", DomainID: "domain-2", Username: "bob", DisplayName: "Bob", Status: "active"},
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/companies/company-1/users/bulk-export", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "alice") {
		t.Fatalf("export missing company user: %s", body)
	}
	if strings.Contains(body, "bob") {
		t.Fatalf("export leaked another company user: %s", body)
	}
	if service.lastDomainList.CompanyID != "company-1" || service.lastUserList.CompanyID != "company-1" || service.lastUserList.DomainID != "" {
		t.Fatalf("domain/user list = %+v/%+v", service.lastDomainList, service.lastUserList)
	}
}

func TestAdminOperationalGetHandlersRejectUnknownQueryParameters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		path       string
		dispatched func(*fakeAdminService) bool
	}{
		{
			name: "company detail",
			path: "/admin/v1/companies/company-1?expand=domains",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastCompanyID != ""
			},
		},
		{
			name: "domain detail",
			path: "/admin/v1/domains/domain-1?include_stats=true",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastDomainID != ""
			},
		},
		{
			name: "queue",
			path: "/admin/v1/queue?limit=5",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastLimit != 0
			},
		},
		{
			name: "outbox list",
			path: "/admin/v1/outbox-events?topic=mail.event&cursor=opaque",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastOutboxEventList.Topic != ""
			},
		},
		{
			name: "outbox detail",
			path: "/admin/v1/outbox-events/outbox-1?include_payload=true",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastOutboxEventID != ""
			},
		},
		{
			name: "audit list",
			path: "/admin/v1/audit-logs?category=admin&cursor=opaque",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastAuditLogList.Category != ""
			},
		},
		{
			name: "audit integrity",
			path: "/admin/v1/audit-logs/integrity?deep=true",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastAuditLogIntegrity.Limit != 0
			},
		},
		{
			name: "audit detail",
			path: "/admin/v1/audit-logs/audit-1?include_detail=true",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastAuditLogID != ""
			},
		},
		{
			name: "quota usage",
			path: "/admin/v1/quota-usage?scope=domain&company_id=company-1",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastQuotaUsageList.Scope != ""
			},
		},
		{
			name: "attachment upload sessions",
			path: "/admin/v1/attachment-upload-sessions?user_id=user-1&cursor=opaque",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastAttachmentUploadSessionList.UserID != ""
			},
		},
		{
			name: "quota reconciliation",
			path: "/admin/v1/quota-reconciliation?scope=user",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastLimit != 0
			},
		},
		{
			name: "delivery attempts",
			path: "/admin/v1/delivery-attempts?message_id=msg-1&cursor=opaque",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastDeliveryAttemptList.MessageID != ""
			},
		},
		{
			name: "delivery attempt stats",
			path: "/admin/v1/delivery-attempts/stats?message_id=msg-1&limit=5",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastDeliveryAttemptStats.MessageID != ""
			},
		},
		{
			name: "exhausted attempts",
			path: "/admin/v1/delivery-attempts/exhausted?message_id=msg-1&status=failed",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastExhaustedAttemptList.MessageID != ""
			},
		},
		{
			name: "push attempts",
			path: "/admin/v1/push-notification-attempts?message_id=msg-1&cursor=opaque",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastPushAttemptList.MessageID != ""
			},
		},
		{
			name: "push attempt detail",
			path: "/admin/v1/push-notification-attempts/attempt-1?include_device=true",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastPushOutcome.AttemptID != ""
			},
		},
		{
			name: "push stats",
			path: "/admin/v1/push-notification-stats?message_id=msg-1&limit=5",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastPushNotificationStats.MessageID != ""
			},
		},
		{
			name: "suppression list",
			path: "/admin/v1/suppression-list?domain_id=domain-1&cursor=opaque",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastSuppressionList.DomainID != ""
			},
		},
		{
			name: "trusted relays",
			path: "/admin/v1/trusted-relays?cidr=192.0.2.0/24&status=active",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastTrustedRelayList.CIDR != ""
			},
		},
		{
			name: "delivery routes",
			path: "/admin/v1/delivery-routes?status=active&cursor=opaque",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastDeliveryRouteList.Status != ""
			},
		},
		{
			name: "delivery route resolve",
			path: "/admin/v1/delivery-routes/resolve?domain=example.net&dry_run=true",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastResolveDeliveryRouteDomain != ""
			},
		},
		{
			name: "dkim keys",
			path: "/admin/v1/dkim-keys?domain_id=domain-1&selector=mail",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastDKIMKeyList.DomainID != ""
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := &fakeAdminService{}
			mux := http.NewServeMux()
			RegisterAdminRoutes(mux, service, "")

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if tt.dispatched(service) {
				t.Fatalf("handler dispatched for unknown query parameter: %+v", service)
			}
		})
	}
}

func TestAdminBodylessHandlersRejectPayloadMetadata(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		method      string
		path        string
		body        string
		contentType string
		dispatched  func(*fakeAdminService) bool
	}{
		{
			name:   "get outbox body",
			method: http.MethodGet,
			path:   "/admin/v1/outbox-events/outbox-1",
			body:   `{}`,
			dispatched: func(service *fakeAdminService) bool {
				return service.lastOutboxEventID != ""
			},
		},
		{
			name:        "get audit content type",
			method:      http.MethodGet,
			path:        "/admin/v1/audit-logs/audit-1",
			contentType: "application/json",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastAuditLogID != ""
			},
		},
		{
			name:   "delete dkim body",
			method: http.MethodDelete,
			path:   "/admin/v1/dkim-keys/dkim-1",
			body:   `{}`,
			dispatched: func(service *fakeAdminService) bool {
				return service.lastDeactivateDKIMKeyID != ""
			},
		},
		{
			name:        "delete suppression content type",
			method:      http.MethodDelete,
			path:        "/admin/v1/suppression-list/suppression-1",
			contentType: "application/json",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastDeleteSuppressionID != ""
			},
		},
		{
			name:   "imap backfill body",
			method: http.MethodPost,
			path:   "/admin/v1/imap/mailboxes/inbox/uid-backfill?user_id=user-1&limit=10",
			body:   `{}`,
			dispatched: func(service *fakeAdminService) bool {
				return service.lastIMAPBackfillUserID != ""
			},
		},
		{
			name:        "export batch create content type",
			method:      http.MethodPost,
			path:        "/admin/v1/api-usage/export-batches?tenant_id=tenant-1&from=2026-05-04T00:00:00Z&to=2026-05-05T00:00:00Z",
			contentType: "application/json",
			dispatched: func(service *fakeAdminService) bool {
				return !service.lastAPIUsageLedgerList.From.IsZero()
			},
		},
		{
			name:   "manifest digest body",
			method: http.MethodPost,
			path:   "/admin/v1/api-usage/export-batches/api-usage-export-1/manifest-digests",
			body:   `{}`,
			dispatched: func(service *fakeAdminService) bool {
				return service.lastAPIUsageExportBatchID != ""
			},
		},
		{
			name:   "manifest signature body",
			method: http.MethodPost,
			path:   "/admin/v1/api-usage/export-batches/api-usage-export-1/manifest-digests/api-usage-manifest-1/signatures",
			body:   `{}`,
			dispatched: func(service *fakeAdminService) bool {
				return service.lastAPIUsageExportManifestDigestID != ""
			},
		},
		{
			name:   "dkim verify body",
			method: http.MethodPost,
			path:   "/admin/v1/dkim-keys/dkim-1/verify-dns",
			body:   `{}`,
			dispatched: func(service *fakeAdminService) bool {
				return service.lastVerifyDKIMKeyID != ""
			},
		},
		{
			name:   "outbox retry body",
			method: http.MethodPost,
			path:   "/admin/v1/outbox/outbox-1/retry",
			body:   `{}`,
			dispatched: func(service *fakeAdminService) bool {
				return service.lastRetryOutboxID != ""
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := &fakeAdminService{}
			mux := http.NewServeMux()
			RegisterAdminRoutes(mux, service, "")

			var body io.Reader
			if tt.body != "" {
				body = strings.NewReader(tt.body)
			}
			req := httptest.NewRequest(tt.method, tt.path, body)
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if tt.dispatched(service) {
				t.Fatalf("handler dispatched for bodyless payload metadata: %+v", service)
			}
		})
	}
}

func TestAdminBodylessCommandHandlersRejectUnknownQueryParameters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		method     string
		path       string
		dispatched func(*fakeAdminService) bool
	}{
		{
			name:   "imap backfill",
			method: http.MethodPost,
			path:   "/admin/v1/imap/mailboxes/inbox/uid-backfill?user_id=user-1&limit=10&dry_run=true",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastIMAPBackfillUserID != ""
			},
		},
		{
			name:   "delete dkim",
			method: http.MethodDelete,
			path:   "/admin/v1/dkim-keys/dkim-1?force=true",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastDeactivateDKIMKeyID != ""
			},
		},
		{
			name:   "verify dkim dns",
			method: http.MethodPost,
			path:   "/admin/v1/dkim-keys/dkim-1/verify-dns?refresh=true",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastVerifyDKIMKeyID != ""
			},
		},
		{
			name:   "retry outbox",
			method: http.MethodPost,
			path:   "/admin/v1/outbox/outbox-1/retry?dry_run=true",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastRetryOutboxID != ""
			},
		},
		{
			name:   "delete suppression",
			method: http.MethodDelete,
			path:   "/admin/v1/suppression-list/suppression-1?reason=manual",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastDeleteSuppressionID != ""
			},
		},
		{
			name:   "delete trusted relay",
			method: http.MethodDelete,
			path:   "/admin/v1/trusted-relays/relay-1?cascade=true",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastDeleteTrustedRelayID != ""
			},
		},
		{
			name:   "delete delivery route",
			method: http.MethodDelete,
			path:   "/admin/v1/delivery-routes/route-1?force=true",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastDeleteDeliveryRouteID != ""
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := &fakeAdminService{}
			mux := http.NewServeMux()
			RegisterAdminRoutes(mux, service, "")

			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if tt.dispatched(service) {
				t.Fatalf("handler dispatched for unknown query parameter: %+v", service)
			}
		})
	}
}

func TestAdminJSONMutationHandlersRejectUnknownQueryParameters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		method     string
		path       string
		dispatched func(*fakeAdminService) bool
	}{
		{
			name:   "company quota",
			method: http.MethodPatch,
			path:   "/admin/v1/companies/company-1/quota?dry_run=true",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastCompanyQuota.ID != ""
			},
		},
		{
			name:   "create domain",
			method: http.MethodPost,
			path:   "/admin/v1/domains?validate_only=true",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastCreateDomain.Name != ""
			},
		},
		{
			name:   "domain status",
			method: http.MethodPatch,
			path:   "/admin/v1/domains/domain-1/status?force=true",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastDomainStatus.ID != ""
			},
		},
		{
			name:   "create user",
			method: http.MethodPost,
			path:   "/admin/v1/users?send_invite=true",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastCreateUser.Username != ""
			},
		},
		{
			name:   "backpressure",
			method: http.MethodPatch,
			path:   "/admin/v1/backpressure?ttl=60",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastBackpressureUpdate.Level != ""
			},
		},
		{
			name:   "cleanup candidates",
			method: http.MethodPost,
			path:   "/admin/v1/attachment-cleanup/candidates?dry_run=true",
			dispatched: func(service *fakeAdminService) bool {
				return !service.lastAttachmentCleanupCountBefore.IsZero()
			},
		},
		{
			name:   "quota correction",
			method: http.MethodPost,
			path:   "/admin/v1/quota-reconciliation/corrections?dry_run=true",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastQuotaCorrection.Scope != ""
			},
		},
		{
			name:   "push outcome",
			method: http.MethodPatch,
			path:   "/admin/v1/push-notification-attempts/attempt-1/outcome?provider=fcm",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastPushOutcome.AttemptID != ""
			},
		},
		{
			name:   "trusted relay create",
			method: http.MethodPost,
			path:   "/admin/v1/trusted-relays?validate_only=true",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastTrustedRelayList.CIDR != ""
			},
		},
		{
			name:   "delivery route create",
			method: http.MethodPost,
			path:   "/admin/v1/delivery-routes?dry_run=true",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastDeliveryRouteList.DomainPattern != ""
			},
		},
		{
			name:   "delivery route status",
			method: http.MethodPatch,
			path:   "/admin/v1/delivery-routes/route-1/status?force=true",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastDeliveryRouteStatus.ID != ""
			},
		},
		{
			name:   "dkim create",
			method: http.MethodPost,
			path:   "/admin/v1/dkim-keys?verify=true",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastDomainID != ""
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := &fakeAdminService{}
			mux := http.NewServeMux()
			RegisterAdminRoutes(mux, service, "")

			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(`{}`))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if tt.dispatched(service) {
				t.Fatalf("handler dispatched for unknown query parameter: %+v", service)
			}
		})
	}
}

func TestAdminGetUserHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		users: []maildb.UserView{{ID: "user-1", DomainID: "domain-1", Username: "admin", Status: "active"}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/users/user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		User maildb.UserView `json:"user"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if body.User.ID != "user-1" || service.lastUserID != "user-1" {
		t.Fatalf("user = %+v lastUserID=%q", body.User, service.lastUserID)
	}
}

func TestAdminHandlersRejectDuplicateScalarQuery(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		path       string
		dispatched func(*fakeAdminService) bool
	}{
		{
			name: "duplicate limit",
			path: "/admin/v1/domains?limit=5&limit=10",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastLimit != 0
			},
		},
		{
			name: "duplicate bool",
			path: "/admin/v1/users?password_configured=true&password_configured=false",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastUserList.PasswordConfigured != nil
			},
		},
		{
			name: "duplicate text filter",
			path: "/admin/v1/users?domain_id=domain-1&domain_id=domain-2",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastDomainID != ""
			},
		},
		{
			name: "duplicate deep",
			path: "/admin/v1/api-usage/export-batches/api-usage-export-1/handoff-readiness?deep=true&deep=false",
			dispatched: func(service *fakeAdminService) bool {
				return service.lastAPIUsageExportBatchID != ""
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := &fakeAdminService{}
			mux := http.NewServeMux()
			RegisterAdminRoutes(mux, service, "")

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if tt.dispatched(service) {
				t.Fatalf("handler dispatched for duplicate scalar query: %+v", service)
			}
		})
	}
}

func TestAdminCreateUserHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	body := []byte(`{"domain_id":"domain-1","username":"admin","display_name":"Admin","address":"admin@example.com","password_hash":"plain:dev-password"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/v1/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastCreateUser.Username != "admin" || service.lastCreateUser.Address != "admin@example.com" || service.lastCreateUser.PasswordHash != "plain:dev-password" {
		t.Fatalf("lastCreateUser = %+v", service.lastCreateUser)
	}
}

func TestAdminUpdateUserStatusHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodPatch, "/admin/v1/users/%20user-1%20/status", bytes.NewReader([]byte(`{"status":"disabled"}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastUserStatus.ID != "user-1" || service.lastUserStatus.Status != "disabled" {
		t.Fatalf("lastUserStatus = %+v", service.lastUserStatus)
	}
}

func TestAdminDeleteUserHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodDelete, "/admin/v1/users/%20user-1%20", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastDeleteUserID != "user-1" {
		t.Fatalf("lastDeleteUserID = %q, want user-1", service.lastDeleteUserID)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if body["status"] != "ok" || body["id"] != "user-1" {
		t.Fatalf("body = %+v", body)
	}
}

func TestAdminUpdateUserQuotaHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodPatch, "/admin/v1/users/%20user-1%20/quota", bytes.NewReader([]byte(`{"quota_limit":4096}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastUserQuota.ID != "user-1" || service.lastUserQuota.QuotaLimit != 4096 {
		t.Fatalf("lastUserQuota = %+v", service.lastUserQuota)
	}
}

func TestAdminUpdateUserPasswordHashHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodPatch, "/admin/v1/users/%20user-1%20/password-hash", bytes.NewReader([]byte(`{"password_hash":"plain:dev-password"}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastUserPasswordHash.ID != "user-1" || service.lastUserPasswordHash.PasswordHash != "plain:dev-password" {
		t.Fatalf("lastUserPasswordHash = %+v", service.lastUserPasswordHash)
	}
}

func TestAdminUserPathIDsRejectUnsafeValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{
			name:   "get crlf",
			method: http.MethodGet,
			path:   "/admin/v1/users/user%0Abad",
		},
		{
			name:   "status oversized",
			method: http.MethodPatch,
			path:   "/admin/v1/users/" + strings.Repeat("u", maxAdminQueryFilterBytes+1) + "/status",
			body:   `{"status":"disabled"}`,
		},
		{
			name:   "delete crlf",
			method: http.MethodDelete,
			path:   "/admin/v1/users/user%0Abad",
		},
		{
			name:   "quota crlf",
			method: http.MethodPatch,
			path:   "/admin/v1/users/user%0Dbad/quota",
			body:   `{"quota_limit":4096}`,
		},
		{
			name:   "password hash crlf",
			method: http.MethodPatch,
			path:   "/admin/v1/users/user%0Abad/password-hash",
			body:   `{"password_hash":"plain:dev-password"}`,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := &fakeAdminService{}
			mux := http.NewServeMux()
			RegisterAdminRoutes(mux, service, "")

			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if service.lastUserID != "" || service.lastUserStatus.ID != "" || service.lastUserQuota.ID != "" {
				t.Fatalf("last user ids = %q/%q/%q", service.lastUserID, service.lastUserStatus.ID, service.lastUserQuota.ID)
			}
		})
	}
}

func TestAdminDeliveryAttemptsHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		attempts: []maildb.DeliveryAttemptView{{
			ID:                "attempt-1",
			MessageID:         "msg-1",
			Sender:            "sender@example.com",
			Recipient:         "user@example.net",
			Status:            "bounced",
			EnhancedStatus:    "5.1.1",
			DSNReturn:         "HDRS",
			DSNEnvelopeID:     "env+2D1",
			DSNNotify:         []string{"FAILURE"},
			OriginalRecipient: "rfc822;alias+40example.net",
			AttemptedAt:       time.Date(2026, 5, 3, 9, 0, 0, 0, time.UTC),
		}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/delivery-attempts?limit=10&status=%20bounced%20&recipient_domain=%20example.net%20&message_id=%20msg-1%20&farm=%20general%20&sender=%20Sender@Example.COM%20&since=2026-05-04T00:00:00Z", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastDeliveryAttemptList.Limit != 10 ||
		service.lastDeliveryAttemptList.Status != "bounced" ||
		service.lastDeliveryAttemptList.RecipientDomain != "example.net" ||
		service.lastDeliveryAttemptList.MessageID != "msg-1" ||
		service.lastDeliveryAttemptList.Farm != "general" ||
		service.lastDeliveryAttemptList.Sender != "Sender@Example.COM" ||
		service.lastDeliveryAttemptList.Since.IsZero() {
		t.Fatalf("lastDeliveryAttemptList = %+v", service.lastDeliveryAttemptList)
	}
	if !strings.Contains(rec.Body.String(), `"enhanced_status":"5.1.1"`) || !strings.Contains(rec.Body.String(), `"dsn_notify":["FAILURE"]`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestAdminDeliveryAttemptsHandlerRejectsInvalidSince(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, &fakeAdminService{}, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/delivery-attempts?since=not-a-time", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "since must be RFC3339 timestamp") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestAdminDeliveryAttemptsHandlerRejectsInvalidStatus(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, &fakeAdminService{}, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/delivery-attempts?status=retrying", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "unsupported delivery attempt status") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestAdminDeliveryAttemptsHandlerRejectsUnsafeFilters(t *testing.T) {
	t.Parallel()

	tests := []string{
		"/admin/v1/delivery-attempts?status=failed%0Abad",
		"/admin/v1/delivery-attempts?recipient_domain=" + strings.Repeat("x", maxAdminQueryFilterBytes+1),
		"/admin/v1/delivery-attempts?message_id=msg%0Abad",
		"/admin/v1/delivery-attempts?farm=" + strings.Repeat("f", maxAdminQueryFilterBytes+1),
		"/admin/v1/delivery-attempts?sender=sender%0Dbad",
	}
	for _, path := range tests {
		service := &fakeAdminService{}
		mux := http.NewServeMux()
		RegisterAdminRoutes(mux, service, "")

		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s status = %d, body = %s", path, rec.Code, rec.Body.String())
		}
		if service.lastDeliveryAttemptList.Limit != 0 {
			t.Fatalf("%s dispatched request %+v", path, service.lastDeliveryAttemptList)
		}
	}
}

func TestAdminDeliveryAttemptStatsHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		deliveryAttemptStats: maildb.DeliveryAttemptStatsView{
			TotalAttempts:    4,
			UniqueMessages:   2,
			UniqueRecipients: 3,
			Delivered:        1,
			Failed:           1,
			Bounced:          1,
			Exhausted:        1,
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/delivery-attempts/stats?status=%20failed%20&recipient_domain=%20example.net%20&message_id=%20msg-1%20&farm=%20general%20&sender=%20sender@example.com%20&since=2026-05-04T00:00:00Z", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Stats maildb.DeliveryAttemptStatsView `json:"delivery_attempt_stats"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Stats.TotalAttempts != 4 || body.Stats.UniqueRecipients != 3 || body.Stats.Exhausted != 1 {
		t.Fatalf("delivery_attempt_stats = %+v", body.Stats)
	}
	if service.lastDeliveryAttemptStats.Status != "failed" ||
		service.lastDeliveryAttemptStats.RecipientDomain != "example.net" ||
		service.lastDeliveryAttemptStats.MessageID != "msg-1" ||
		service.lastDeliveryAttemptStats.Farm != "general" ||
		service.lastDeliveryAttemptStats.Sender != "sender@example.com" ||
		service.lastDeliveryAttemptStats.Since.IsZero() {
		t.Fatalf("lastDeliveryAttemptStats = %+v", service.lastDeliveryAttemptStats)
	}
}

func TestAdminDeliveryAttemptStatsHandlerRejectsInvalidSince(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, &fakeAdminService{}, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/delivery-attempts/stats?since=not-a-time", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "since must be RFC3339 timestamp") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestAdminDeliveryAttemptStatsHandlerRejectsInvalidStatus(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, &fakeAdminService{}, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/delivery-attempts/stats?status=retrying", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "unsupported delivery attempt status") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestAdminDeliveryAttemptStatsHandlerRejectsUnsafeFilters(t *testing.T) {
	t.Parallel()

	tests := []string{
		"/admin/v1/delivery-attempts/stats?status=failed%0Dbad",
		"/admin/v1/delivery-attempts/stats?recipient_domain=" + strings.Repeat("x", maxAdminQueryFilterBytes+1),
		"/admin/v1/delivery-attempts/stats?message_id=msg%0Abad",
		"/admin/v1/delivery-attempts/stats?farm=" + strings.Repeat("f", maxAdminQueryFilterBytes+1),
		"/admin/v1/delivery-attempts/stats?sender=sender%0Dbad",
	}
	for _, path := range tests {
		service := &fakeAdminService{}
		mux := http.NewServeMux()
		RegisterAdminRoutes(mux, service, "")

		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s status = %d, body = %s", path, rec.Code, rec.Body.String())
		}
		if service.lastDeliveryAttemptStats.Status != "" || service.lastDeliveryAttemptStats.RecipientDomain != "" {
			t.Fatalf("%s dispatched request %+v", path, service.lastDeliveryAttemptStats)
		}
	}
}

func TestAdminExhaustedAttemptsHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		attempts: []maildb.DeliveryAttemptView{{
			ID:              "attempt-1",
			MessageID:       "msg-1",
			Sender:          "sender@example.com",
			Recipient:       "user@example.net",
			RecipientDomain: "example.net",
			Status:          "exhausted",
			EnhancedStatus:  "4.0.0",
			AttemptedAt:     time.Date(2026, 5, 3, 9, 0, 0, 0, time.UTC),
		}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/delivery-attempts/exhausted?limit=10&recipient_domain=%20example.net%20&message_id=%20msg-1%20&farm=%20general%20&sender=%20sender@example.com%20&since=2026-05-04T00:00:00Z", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastExhaustedAttemptList.Limit != 10 ||
		service.lastExhaustedAttemptList.RecipientDomain != "example.net" ||
		service.lastExhaustedAttemptList.MessageID != "msg-1" ||
		service.lastExhaustedAttemptList.Farm != "general" ||
		service.lastExhaustedAttemptList.Sender != "sender@example.com" ||
		service.lastExhaustedAttemptList.Since.IsZero() {
		t.Fatalf("lastExhaustedAttemptList = %+v", service.lastExhaustedAttemptList)
	}
	if !strings.Contains(rec.Body.String(), `"enhanced_status":"4.0.0"`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestAdminExhaustedAttemptsHandlerRejectsUnsafeFilters(t *testing.T) {
	t.Parallel()

	tests := []string{
		"/admin/v1/delivery-attempts/exhausted?recipient_domain=example.net%0Abad",
		"/admin/v1/delivery-attempts/exhausted?message_id=msg%0Abad",
		"/admin/v1/delivery-attempts/exhausted?farm=" + strings.Repeat("f", maxAdminQueryFilterBytes+1),
		"/admin/v1/delivery-attempts/exhausted?sender=sender%0Dbad",
	}
	for _, path := range tests {
		service := &fakeAdminService{}
		mux := http.NewServeMux()
		RegisterAdminRoutes(mux, service, "")

		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s status = %d, body = %s", path, rec.Code, rec.Body.String())
		}
		if service.lastExhaustedAttemptList.RecipientDomain != "" ||
			service.lastExhaustedAttemptList.MessageID != "" ||
			service.lastExhaustedAttemptList.Farm != "" ||
			service.lastExhaustedAttemptList.Sender != "" {
			t.Fatalf("%s dispatched request %+v", path, service.lastExhaustedAttemptList)
		}
	}
}

func TestAdminExhaustedAttemptsHandlerRejectsInvalidSince(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, &fakeAdminService{}, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/delivery-attempts/exhausted?since=not-a-time", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "since must be RFC3339 timestamp") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestAdminPushNotificationAttemptsHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		pushNotificationAttempts: []maildb.PushNotificationAttemptView{{
			ID:                "push-attempt-1",
			MessageID:         "msg-1",
			UserID:            "user-1",
			DeviceID:          "device-1",
			Platform:          "fcm",
			TokenSuffix:       "token-1",
			Status:            "candidate",
			ProviderMessageID: "provider-message-1",
			ProviderStatus:    "accepted",
			AttemptedAt:       time.Date(2026, 5, 3, 9, 0, 0, 0, time.UTC),
		}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/push-notification-attempts?limit=10&message_id=%20msg-1%20&status=%20candidate%20&user_id=%20user-1%20&platform=%20fcm%20&device_id=%20device-1%20&provider_status=%20accepted%20&provider_message_id=%20provider-message-1%20&since=2026-05-04T00:00:00Z", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastPushAttemptList.Limit != 10 ||
		service.lastPushAttemptList.MessageID != "msg-1" ||
		service.lastPushAttemptList.Status != "candidate" ||
		service.lastPushAttemptList.UserID != "user-1" ||
		service.lastPushAttemptList.Platform != "fcm" ||
		service.lastPushAttemptList.DeviceID != "device-1" ||
		service.lastPushAttemptList.ProviderStatus != "accepted" ||
		service.lastPushAttemptList.ProviderMessageID != "provider-message-1" ||
		service.lastPushAttemptList.Since.IsZero() {
		t.Fatalf("lastPushAttemptList = %+v", service.lastPushAttemptList)
	}
	if !strings.Contains(rec.Body.String(), "push_notification_attempts") || !strings.Contains(rec.Body.String(), "provider-message-1") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestAdminPushNotificationAttemptsHandlerRejectsInvalidSince(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, &fakeAdminService{}, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/push-notification-attempts?since=not-a-time", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "since must be RFC3339 timestamp") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestAdminPushNotificationAttemptsHandlerRejectsUnsafeFilters(t *testing.T) {
	t.Parallel()

	tests := []string{
		"/admin/v1/push-notification-attempts?status=candidate%0Abad",
		"/admin/v1/push-notification-attempts?message_id=msg-1%0Abad",
		"/admin/v1/push-notification-attempts?user_id=user-1%0Dbad",
		"/admin/v1/push-notification-attempts?platform=fcm%0Abad",
		"/admin/v1/push-notification-attempts?device_id=" + strings.Repeat("x", maxAdminQueryFilterBytes+1),
		"/admin/v1/push-notification-attempts?provider_status=accepted%0Abad",
		"/admin/v1/push-notification-attempts?provider_message_id=" + strings.Repeat("x", maxAdminQueryFilterBytes+1),
	}
	for _, path := range tests {
		service := &fakeAdminService{}
		mux := http.NewServeMux()
		RegisterAdminRoutes(mux, service, "")

		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s status = %d, body = %s", path, rec.Code, rec.Body.String())
		}
		if service.lastPushAttemptList.Limit != 0 {
			t.Fatalf("%s dispatched request %+v", path, service.lastPushAttemptList)
		}
	}
}

func TestAdminPushNotificationAttemptDetailHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		pushNotificationAttempts: []maildb.PushNotificationAttemptView{{
			ID:                "attempt-1",
			MessageID:         "msg-1",
			UserID:            "user-1",
			DeviceID:          "device-1",
			Platform:          "fcm",
			Status:            "delivered",
			ProviderMessageID: "provider-message-1",
			AttemptedAt:       time.Date(2026, 5, 4, 1, 0, 0, 0, time.UTC),
		}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/push-notification-attempts/attempt-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastPushOutcome.AttemptID != "attempt-1" {
		t.Fatalf("attempt id = %q", service.lastPushOutcome.AttemptID)
	}
	if !strings.Contains(rec.Body.String(), "push_notification_attempt") || !strings.Contains(rec.Body.String(), "provider-message-1") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestAdminPushNotificationAttemptDetailHandlerRejectsUnsafeID(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/push-notification-attempts/bad%0Aid", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastPushOutcome.AttemptID != "" {
		t.Fatalf("dispatched attempt id %q", service.lastPushOutcome.AttemptID)
	}
}

func TestAdminPushNotificationOutcomeHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	body := strings.NewReader(`{"status":"delivered","provider_message_id":"provider-message-1","provider_status":"accepted","error_message":"ok"}`)
	req := httptest.NewRequest(http.MethodPatch, "/admin/v1/push-notification-attempts/attempt-1/outcome", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastPushOutcome.AttemptID != "attempt-1" ||
		service.lastPushOutcome.Status != "delivered" ||
		service.lastPushOutcome.ProviderMessageID != "provider-message-1" ||
		service.lastPushOutcome.ProviderStatus != "accepted" ||
		service.lastPushOutcome.ErrorMessage != "ok" {
		t.Fatalf("lastPushOutcome = %+v", service.lastPushOutcome)
	}
	if !strings.Contains(rec.Body.String(), `"id":"attempt-1"`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestAdminPushNotificationOutcomeHandlerRejectsUnsafeID(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodPatch, "/admin/v1/push-notification-attempts/bad%0Aid/outcome", strings.NewReader(`{"status":"failed"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastPushOutcome.AttemptID != "" {
		t.Fatalf("dispatched outcome %+v", service.lastPushOutcome)
	}
}

func TestAdminPushNotificationOutcomeHandlerRejectsInvalidBody(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodPatch, "/admin/v1/push-notification-attempts/attempt-1/outcome", strings.NewReader(`{"status":"candidate"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastPushOutcome.Status != "candidate" {
		t.Fatalf("lastPushOutcome = %+v", service.lastPushOutcome)
	}
}

func TestAdminPushNotificationStatsHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		pushNotificationStats: maildb.PushNotificationStatsView{
			ActiveDevices: 3,
			TotalAttempts: 9,
			Candidate:     4,
			Delivered:     2,
			Failed:        1,
			InvalidToken:  2,
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/push-notification-stats?message_id=%20message-1%20&user_id=%20user-1%20&platform=%20fcm%20&device_id=%20device-1%20&since=2026-05-04T00:00:00Z", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastPushNotificationStats.MessageID != "message-1" || service.lastPushNotificationStats.UserID != "user-1" || service.lastPushNotificationStats.Platform != "fcm" || service.lastPushNotificationStats.DeviceID != "device-1" || service.lastPushNotificationStats.Since.IsZero() {
		t.Fatalf("lastPushNotificationStats = %+v", service.lastPushNotificationStats)
	}
	if !strings.Contains(rec.Body.String(), "push_notification_stats") || !strings.Contains(rec.Body.String(), `"active_devices":3`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestAdminPushNotificationStatsHandlerRejectsInvalidSince(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, &fakeAdminService{}, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/push-notification-stats?since=not-a-time", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "since must be RFC3339 timestamp") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestAdminPushNotificationStatsHandlerRejectsUnsafeFilters(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	for _, target := range []string{
		"/admin/v1/push-notification-stats?message_id=message-1%0Abad",
		"/admin/v1/push-notification-stats?user_id=user-1%0Abad",
		"/admin/v1/push-notification-stats?platform=fcm%0Abad",
		"/admin/v1/push-notification-stats?device_id=device-1%0Abad",
	} {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s status = %d, body = %s", target, rec.Code, rec.Body.String())
		}
	}
	if service.lastPushNotificationStats.MessageID != "" || service.lastPushNotificationStats.UserID != "" || service.lastPushNotificationStats.Platform != "" || service.lastPushNotificationStats.DeviceID != "" {
		t.Fatalf("dispatched request %+v", service.lastPushNotificationStats)
	}
}

func TestAdminSuppressionListHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		suppression: []maildb.SuppressionEntry{{
			ID:        "suppression-1",
			Email:     "user@example.net",
			Reason:    "hard_bounce",
			CreatedAt: time.Date(2026, 5, 3, 9, 0, 0, 0, time.UTC),
		}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/suppression-list?domain_id=%20domain-1%20&email=%20user@example.net%20&reason=%20hard_bounce%20&limit=5", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastLimit != 5 {
		t.Fatalf("lastLimit = %d, want 5", service.lastLimit)
	}
	if service.lastSuppressionList.DomainID != "domain-1" || service.lastSuppressionList.Email != "user@example.net" || service.lastSuppressionList.Reason != "hard_bounce" {
		t.Fatalf("lastSuppressionList = %+v", service.lastSuppressionList)
	}
}

func TestAdminSuppressionListHandlerRejectsUnsafeFilters(t *testing.T) {
	t.Parallel()

	tests := []string{
		"/admin/v1/suppression-list?domain_id=domain%0Abad",
		"/admin/v1/suppression-list?email=user%0D@example.net",
		"/admin/v1/suppression-list?reason=" + strings.Repeat("x", maxAdminQueryFilterBytes+1),
	}

	for _, path := range tests {
		path := path
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			service := &fakeAdminService{}
			mux := http.NewServeMux()
			RegisterAdminRoutes(mux, service, "")

			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if service.lastSuppressionList.Limit != 0 {
				t.Fatalf("lastSuppressionList = %+v", service.lastSuppressionList)
			}
		})
	}
}

func TestAdminDKIMKeysHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		dkimKeys: []maildb.DKIMKeyView{{
			ID:       "dkim-1",
			DomainID: "domain-1",
			Selector: "s1",
			Status:   "active",
		}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/dkim-keys?domain_id=%20domain-1%20&status=active&limit=5", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastDomainID != "domain-1" {
		t.Fatalf("lastDomainID = %q, want domain-1", service.lastDomainID)
	}
	if service.lastLimit != 5 {
		t.Fatalf("lastLimit = %d, want 5", service.lastLimit)
	}
	if service.lastDKIMKeyList.Status != "active" {
		t.Fatalf("lastDKIMKeyList.Status = %q, want active", service.lastDKIMKeyList.Status)
	}
}

func TestAdminDKIMKeysHandlerRejectsUnsafeFilters(t *testing.T) {
	t.Parallel()

	tests := []string{
		"/admin/v1/dkim-keys?domain_id=domain%0Abad",
		"/admin/v1/dkim-keys?domain_id=" + strings.Repeat("d", maxAdminQueryFilterBytes+1),
		"/admin/v1/dkim-keys?status=revoked",
		"/admin/v1/dkim-keys?status=active%0Abad",
	}

	for _, path := range tests {
		path := path
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			service := &fakeAdminService{}
			mux := http.NewServeMux()
			RegisterAdminRoutes(mux, service, "")

			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if service.lastDomainID != "" || service.lastDKIMKeyList.Status != "" {
				t.Fatalf("lastDKIMKeyList = %+v", service.lastDKIMKeyList)
			}
		})
	}
}

func TestAdminCreateDKIMKeyHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{createdDKIMKeyID: "dkim-1"}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	body := []byte(`{"domain_id":"domain-1","selector":"s1","private_key_pem":"private","public_key_dns":"v=DKIM1; p=public"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/v1/dkim-keys", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastCreateDKIMKey.Selector != "s1" {
		t.Fatalf("lastCreateDKIMKey = %+v", service.lastCreateDKIMKey)
	}
}

func TestAdminDeactivateDKIMKeyHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodDelete, "/admin/v1/dkim-keys/%20dkim-1%20", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastDeactivateDKIMKeyID != "dkim-1" {
		t.Fatalf("lastDeactivateDKIMKeyID = %q", service.lastDeactivateDKIMKeyID)
	}
}

func TestAdminVerifyDKIMKeyDNSHandlerTrimsID(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodPost, "/admin/v1/dkim-keys/%20dkim-1%20/verify-dns", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastVerifyDKIMKeyID != "dkim-1" {
		t.Fatalf("lastVerifyDKIMKeyID = %q", service.lastVerifyDKIMKeyID)
	}
}

func TestAdminDKIMKeyPathIDsRejectUnsafeValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{
			name:   "delete crlf",
			method: http.MethodDelete,
			path:   "/admin/v1/dkim-keys/dkim%0Abad",
		},
		{
			name:   "verify oversized",
			method: http.MethodPost,
			path:   "/admin/v1/dkim-keys/" + strings.Repeat("d", maxAdminQueryFilterBytes+1) + "/verify-dns",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := &fakeAdminService{}
			mux := http.NewServeMux()
			RegisterAdminRoutes(mux, service, "")

			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if service.lastDeactivateDKIMKeyID != "" || service.lastVerifyDKIMKeyID != "" {
				t.Fatalf("last dkim ids = %q/%q", service.lastDeactivateDKIMKeyID, service.lastVerifyDKIMKeyID)
			}
		})
	}
}

func TestAdminRetryOutboxHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodPost, "/admin/v1/outbox/outbox-1/retry", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastRetryOutboxID != "outbox-1" {
		t.Fatalf("lastRetryOutboxID = %q", service.lastRetryOutboxID)
	}
}

func TestAdminRetryOutboxHandlerRejectsBlankID(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodPost, "/admin/v1/outbox/%20/retry", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastRetryOutboxID != "" {
		t.Fatalf("lastRetryOutboxID = %q", service.lastRetryOutboxID)
	}
}

func TestAdminRetryOutboxHandlerRejectsUnsafeID(t *testing.T) {
	t.Parallel()

	tests := []string{
		"/admin/v1/outbox/outbox%0Abad/retry",
		"/admin/v1/outbox/" + strings.Repeat("o", maxAdminQueryFilterBytes+1) + "/retry",
	}

	for _, path := range tests {
		path := path
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			service := &fakeAdminService{}
			mux := http.NewServeMux()
			RegisterAdminRoutes(mux, service, "")

			req := httptest.NewRequest(http.MethodPost, path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if service.lastRetryOutboxID != "" {
				t.Fatalf("lastRetryOutboxID = %q", service.lastRetryOutboxID)
			}
		})
	}
}

func TestAdminDeleteSuppressionHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodDelete, "/admin/v1/suppression-list/%20suppression-1%20", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastDeleteSuppressionID != "suppression-1" {
		t.Fatalf("lastDeleteSuppressionID = %q", service.lastDeleteSuppressionID)
	}
}

func TestAdminDeleteSuppressionHandlerRejectsUnsafeID(t *testing.T) {
	t.Parallel()

	tests := []string{
		"/admin/v1/suppression-list/suppression%0Abad",
		"/admin/v1/suppression-list/" + strings.Repeat("s", maxAdminQueryFilterBytes+1),
	}

	for _, path := range tests {
		path := path
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			service := &fakeAdminService{}
			mux := http.NewServeMux()
			RegisterAdminRoutes(mux, service, "")

			req := httptest.NewRequest(http.MethodDelete, path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if service.lastDeleteSuppressionID != "" {
				t.Fatalf("lastDeleteSuppressionID = %q", service.lastDeleteSuppressionID)
			}
		})
	}
}

func TestAdminTrustedRelaysHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		trustedRelays: []maildb.TrustedRelayView{{
			ID:          "relay-1",
			CIDR:        "192.0.2.0/24",
			Description: "spam relay",
			CreatedAt:   time.Date(2026, 5, 4, 9, 0, 0, 0, time.UTC),
		}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/trusted-relays?limit=5&cidr=192.0.2.0/24&description=spam", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		TrustedRelays []maildb.TrustedRelayView `json:"trusted_relays"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if len(body.TrustedRelays) != 1 || body.TrustedRelays[0].CIDR != "192.0.2.0/24" {
		t.Fatalf("trusted_relays = %+v", body.TrustedRelays)
	}
	if service.lastTrustedRelayList.Limit != 5 || service.lastTrustedRelayList.CIDR != "192.0.2.0/24" || service.lastTrustedRelayList.Description != "spam" {
		t.Fatalf("lastTrustedRelayList = %+v", service.lastTrustedRelayList)
	}
}

func TestAdminTrustedRelaysHandlerRejectsUnsafeFilters(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	tests := []string{
		"/admin/v1/trusted-relays?cidr=not-a-cidr",
		"/admin/v1/trusted-relays?description=edge%0Abad",
		"/admin/v1/trusted-relays?description=" + strings.Repeat("x", maxAdminQueryFilterBytes+1),
	}
	for _, target := range tests {
		target := target
		t.Run(target, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, target, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if service.lastTrustedRelayList.Limit != 0 {
				t.Fatalf("trusted relay list was called: %+v", service.lastTrustedRelayList)
			}
		})
	}
}

func TestAdminCreateTrustedRelayHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodPost, "/admin/v1/trusted-relays", bytes.NewReader([]byte(`{
		"cidr": "192.0.2.1",
		"description": "edge relay"
	}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastCreateTrustedRelay.CIDR != "192.0.2.1" || service.lastCreateTrustedRelay.Description != "edge relay" {
		t.Fatalf("lastCreateTrustedRelay = %+v", service.lastCreateTrustedRelay)
	}
}

func TestAdminDeliveryRoutesHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		deliveryRoutes: []maildb.DeliveryRouteView{{
			ID:            "route-1",
			DomainPattern: "*.example.net",
			Hosts:         []string{"relay.example.net"},
			Port:          587,
			TLSMode:       "require",
			Status:        "active",
		}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/delivery-routes?status=active&farm=%20transactional%20&domain_pattern=%20*.example.net%20&limit=5", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		DeliveryRoutes []maildb.DeliveryRouteView `json:"delivery_routes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if len(body.DeliveryRoutes) != 1 || body.DeliveryRoutes[0].DomainPattern != "*.example.net" {
		t.Fatalf("delivery_routes = %+v", body.DeliveryRoutes)
	}
	if service.lastLimit != 5 {
		t.Fatalf("lastLimit = %d, want 5", service.lastLimit)
	}
	if service.lastDeliveryRouteList.Status != "active" || service.lastDeliveryRouteList.Farm != "transactional" || service.lastDeliveryRouteList.DomainPattern != "*.example.net" {
		t.Fatalf("lastDeliveryRouteList = %+v", service.lastDeliveryRouteList)
	}
}

func TestAdminDeliveryRoutesHandlerRejectsUnsafeFilters(t *testing.T) {
	t.Parallel()

	tests := []string{
		"/admin/v1/delivery-routes?status=paused",
		"/admin/v1/delivery-routes?farm=pool%0Abad",
		"/admin/v1/delivery-routes?domain_pattern=" + strings.Repeat("d", maxAdminQueryFilterBytes+1),
	}

	for _, path := range tests {
		path := path
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			service := &fakeAdminService{}
			mux := http.NewServeMux()
			RegisterAdminRoutes(mux, service, "")

			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if service.lastDeliveryRouteList.Limit != 0 {
				t.Fatalf("lastDeliveryRouteList = %+v", service.lastDeliveryRouteList)
			}
		})
	}
}

func TestAdminCreateDeliveryRouteHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodPost, "/admin/v1/delivery-routes", bytes.NewReader([]byte(`{
		"domain_pattern": "*.example.net",
		"farm": "transactional",
		"hosts": ["relay.example.net"],
		"port": 587,
		"tls_mode": "require",
		"auth_username": "relay-user",
		"auth_password": "secret"
	}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastCreateDeliveryRoute.DomainPattern != "*.example.net" || service.lastCreateDeliveryRoute.AuthPassword != "secret" {
		t.Fatalf("lastCreateDeliveryRoute = %+v", service.lastCreateDeliveryRoute)
	}
	if !strings.Contains(rec.Body.String(), `"delivery_route"`) {
		t.Fatalf("response missing delivery_route envelope: %s", rec.Body.String())
	}
}

func TestAdminResolveDeliveryRouteHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		deliveryRouteResolution: maildb.DeliveryRouteResolveView{
			Domain:  "mail.example.net",
			Matched: true,
			Route:   &maildb.DeliveryRouteView{ID: "route-1", DomainPattern: "*.example.net"},
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/delivery-routes/resolve?domain=%20mail.example.net%20", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastResolveDeliveryRouteDomain != "mail.example.net" {
		t.Fatalf("lastResolveDeliveryRouteDomain = %q", service.lastResolveDeliveryRouteDomain)
	}
	if !strings.Contains(rec.Body.String(), `"delivery_route_resolution"`) {
		t.Fatalf("response missing delivery_route_resolution envelope: %s", rec.Body.String())
	}
}

func TestAdminResolveDeliveryRouteRejectsUnsafeDomain(t *testing.T) {
	t.Parallel()

	tests := []string{
		"/admin/v1/delivery-routes/resolve?domain=mail%0Aexample.net",
		"/admin/v1/delivery-routes/resolve?domain=" + strings.Repeat("d", maxAdminQueryFilterBytes+1),
	}

	for _, path := range tests {
		path := path
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			service := &fakeAdminService{}
			mux := http.NewServeMux()
			RegisterAdminRoutes(mux, service, "")

			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if service.lastResolveDeliveryRouteDomain != "" {
				t.Fatalf("lastResolveDeliveryRouteDomain = %q", service.lastResolveDeliveryRouteDomain)
			}
		})
	}
}

func TestAdminUpdateDeliveryRouteStatusHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodPatch, "/admin/v1/delivery-routes/%20route-1%20/status", bytes.NewReader([]byte(`{"status":"disabled"}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastDeliveryRouteStatus.ID != "route-1" || service.lastDeliveryRouteStatus.Status != "disabled" {
		t.Fatalf("lastDeliveryRouteStatus = %+v", service.lastDeliveryRouteStatus)
	}
}

func TestAdminDeleteDeliveryRouteHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodDelete, "/admin/v1/delivery-routes/%20route-1%20", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastDeleteDeliveryRouteID != "route-1" {
		t.Fatalf("lastDeleteDeliveryRouteID = %q", service.lastDeleteDeliveryRouteID)
	}
}

func TestAdminDeleteTrustedRelayHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodDelete, "/admin/v1/trusted-relays/%20relay-1%20", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastDeleteTrustedRelayID != "relay-1" {
		t.Fatalf("lastDeleteTrustedRelayID = %q", service.lastDeleteTrustedRelayID)
	}
}

func TestAdminDeliveryRouteAndTrustedRelayPathIDsRejectUnsafeValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{
			name:   "delivery route status crlf",
			method: http.MethodPatch,
			path:   "/admin/v1/delivery-routes/route%0Abad/status",
			body:   `{"status":"disabled"}`,
		},
		{
			name:   "delivery route delete oversized",
			method: http.MethodDelete,
			path:   "/admin/v1/delivery-routes/" + strings.Repeat("r", maxAdminQueryFilterBytes+1),
		},
		{
			name:   "trusted relay delete crlf",
			method: http.MethodDelete,
			path:   "/admin/v1/trusted-relays/relay%0Abad",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := &fakeAdminService{}
			mux := http.NewServeMux()
			RegisterAdminRoutes(mux, service, "")

			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if service.lastDeliveryRouteStatus.ID != "" || service.lastDeleteDeliveryRouteID != "" || service.lastDeleteTrustedRelayID != "" {
				t.Fatalf("last ids = %+v/%q/%q", service.lastDeliveryRouteStatus, service.lastDeleteDeliveryRouteID, service.lastDeleteTrustedRelayID)
			}
		})
	}
}

func TestAdminRoutesRequireTokenWhenConfigured(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "secret")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/queue", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/admin/v1/queue", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestAdminRoutesRejectMissingAuthConfigOutsideDev(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "", WithEnvironment("production"))

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/queue", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "admin authentication is not configured") {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestAdminRoutesAllowMissingAuthOnlyInDevelopment(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		queueStats: []maildb.QueueStat{{Topic: "mail.outbound.general", Status: "pending", Count: 1}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "", WithEnvironment("development"))

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/queue", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestAdminUpdateDirectoryDelegationRoleHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		directoryDelegation: directory.Delegation{
			ID:           "delegation-1",
			CompanyID:    "company-1",
			OwnerKind:    directory.PrincipalKindResource,
			OwnerID:      "room-1",
			DelegateKind: directory.PrincipalKindGroup,
			DelegateID:   "team-1",
			Scope:        directory.DelegationScopeCalendar,
			Role:         directory.DelegationRoleManage,
			Status:       "active",
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodPatch, "/admin/v1/directory/delegations/%20delegation-1%20/role", bytes.NewReader([]byte(`{"role":"manage"}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var response struct {
		Delegation directory.Delegation `json:"directory_delegation"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Delegation.ID != "delegation-1" || response.Delegation.Role != directory.DelegationRoleManage {
		t.Fatalf("directory_delegation = %+v", response.Delegation)
	}
	if service.lastDirectoryDelegationRoleUpdate.ID != "delegation-1" ||
		service.lastDirectoryDelegationRoleUpdate.Role != directory.DelegationRoleManage {
		t.Fatalf("lastDirectoryDelegationRoleUpdate = %+v", service.lastDirectoryDelegationRoleUpdate)
	}
}

func TestAdminUpdateDirectoryDelegationRoleHandlerRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		body string
		ct   string
	}{
		{name: "bad path value", path: "/admin/v1/directory/delegations/delegation%0Abad/role", body: `{"role":"manage"}`, ct: "application/json"},
		{name: "unknown query", path: "/admin/v1/directory/delegations/delegation-1/role?dry_run=true", body: `{"role":"manage"}`, ct: "application/json"},
		{name: "bad content type", path: "/admin/v1/directory/delegations/delegation-1/role", body: `{"role":"manage"}`, ct: "text/plain"},
		{name: "unknown json field", path: "/admin/v1/directory/delegations/delegation-1/role", body: `{"role":"manage","extra":true}`, ct: "application/json"},
		{name: "bad role", path: "/admin/v1/directory/delegations/delegation-1/role", body: `{"role":"owner"}`, ct: "application/json"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			service := &fakeAdminService{}
			mux := http.NewServeMux()
			RegisterAdminRoutes(mux, service, "")

			req := httptest.NewRequest(http.MethodPatch, tc.path, bytes.NewReader([]byte(tc.body)))
			req.Header.Set("Content-Type", tc.ct)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if service.lastDirectoryDelegationRoleUpdate.ID != "" {
				t.Fatalf("dispatched request %+v", service.lastDirectoryDelegationRoleUpdate)
			}
		})
	}
}

func TestAdminReassignDirectoryDelegationHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		directoryDelegation: directory.Delegation{
			ID:           "delegation-1",
			CompanyID:    "company-1",
			OwnerKind:    directory.PrincipalKindResource,
			OwnerID:      "room-2",
			DelegateKind: directory.PrincipalKindUser,
			DelegateID:   "user-1",
			Scope:        directory.DelegationScopeDrive,
			Role:         directory.DelegationRoleWrite,
			Status:       "active",
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodPatch, "/admin/v1/directory/delegations/%20delegation-1%20/assignment", bytes.NewReader([]byte(`{"owner_kind":"resource","owner_id":"room-2","delegate_kind":"user","delegate_id":"user-1","scope":"drive"}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var response struct {
		Delegation directory.Delegation `json:"directory_delegation"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Delegation.ID != "delegation-1" ||
		response.Delegation.OwnerID != "room-2" ||
		response.Delegation.DelegateID != "user-1" ||
		response.Delegation.Scope != directory.DelegationScopeDrive {
		t.Fatalf("directory_delegation = %+v", response.Delegation)
	}
	if service.lastDirectoryDelegationReassign.ID != "delegation-1" ||
		service.lastDirectoryDelegationReassign.OwnerKind != directory.PrincipalKindResource ||
		service.lastDirectoryDelegationReassign.OwnerID != "room-2" ||
		service.lastDirectoryDelegationReassign.DelegateKind != directory.PrincipalKindUser ||
		service.lastDirectoryDelegationReassign.DelegateID != "user-1" ||
		service.lastDirectoryDelegationReassign.Scope != directory.DelegationScopeDrive {
		t.Fatalf("lastDirectoryDelegationReassign = %+v", service.lastDirectoryDelegationReassign)
	}
}

func TestAdminReassignDirectoryDelegationHandlerRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		body string
		ct   string
	}{
		{name: "bad path value", path: "/admin/v1/directory/delegations/delegation%0Abad/assignment", body: `{"owner_kind":"resource","owner_id":"room-2","delegate_kind":"user","delegate_id":"user-1","scope":"drive"}`, ct: "application/json"},
		{name: "unknown query", path: "/admin/v1/directory/delegations/delegation-1/assignment?dry_run=true", body: `{"owner_kind":"resource","owner_id":"room-2","delegate_kind":"user","delegate_id":"user-1","scope":"drive"}`, ct: "application/json"},
		{name: "bad content type", path: "/admin/v1/directory/delegations/delegation-1/assignment", body: `{"owner_kind":"resource","owner_id":"room-2","delegate_kind":"user","delegate_id":"user-1","scope":"drive"}`, ct: "text/plain"},
		{name: "unknown json field", path: "/admin/v1/directory/delegations/delegation-1/assignment", body: `{"owner_kind":"resource","owner_id":"room-2","delegate_kind":"user","delegate_id":"user-1","scope":"drive","extra":true}`, ct: "application/json"},
		{name: "self delegation", path: "/admin/v1/directory/delegations/delegation-1/assignment", body: `{"owner_kind":"user","owner_id":"user-1","delegate_kind":"user","delegate_id":"user-1","scope":"drive"}`, ct: "application/json"},
		{name: "bad scope", path: "/admin/v1/directory/delegations/delegation-1/assignment", body: `{"owner_kind":"resource","owner_id":"room-2","delegate_kind":"user","delegate_id":"user-1","scope":"files"}`, ct: "application/json"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			service := &fakeAdminService{}
			mux := http.NewServeMux()
			RegisterAdminRoutes(mux, service, "")

			req := httptest.NewRequest(http.MethodPatch, tc.path, bytes.NewReader([]byte(tc.body)))
			req.Header.Set("Content-Type", tc.ct)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if service.lastDirectoryDelegationReassign.ID != "" {
				t.Fatalf("dispatched request %+v", service.lastDirectoryDelegationReassign)
			}
		})
	}
}

func TestAdminUpdateDirectoryGroupMembershipRoleHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		directoryGroupMembership: directory.GroupMembership{
			ID:         "membership-1",
			GroupID:    "group-1",
			CompanyID:  "company-1",
			MemberKind: directory.PrincipalKindUser,
			MemberID:   "user-1",
			Role:       directory.GroupMembershipRoleOwner,
			Status:     "active",
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodPatch, "/admin/v1/directory/group-memberships/%20membership-1%20/role", bytes.NewReader([]byte(`{"role":"owner"}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var response struct {
		Membership directory.GroupMembership `json:"directory_group_membership"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Membership.ID != "membership-1" || response.Membership.Role != directory.GroupMembershipRoleOwner {
		t.Fatalf("directory_group_membership = %+v", response.Membership)
	}
	if service.lastDirectoryGroupMembershipRoleUpdate.ID != "membership-1" ||
		service.lastDirectoryGroupMembershipRoleUpdate.Role != directory.GroupMembershipRoleOwner {
		t.Fatalf("lastDirectoryGroupMembershipRoleUpdate = %+v", service.lastDirectoryGroupMembershipRoleUpdate)
	}
}

func TestAdminUpdateDirectoryGroupMembershipRoleHandlerRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		body string
		ct   string
	}{
		{name: "bad path value", path: "/admin/v1/directory/group-memberships/membership%0Abad/role", body: `{"role":"owner"}`, ct: "application/json"},
		{name: "unknown query", path: "/admin/v1/directory/group-memberships/membership-1/role?dry_run=true", body: `{"role":"owner"}`, ct: "application/json"},
		{name: "bad content type", path: "/admin/v1/directory/group-memberships/membership-1/role", body: `{"role":"owner"}`, ct: "text/plain"},
		{name: "unknown json field", path: "/admin/v1/directory/group-memberships/membership-1/role", body: `{"role":"owner","extra":true}`, ct: "application/json"},
		{name: "bad role", path: "/admin/v1/directory/group-memberships/membership-1/role", body: `{"role":"admin"}`, ct: "application/json"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			service := &fakeAdminService{}
			mux := http.NewServeMux()
			RegisterAdminRoutes(mux, service, "")

			req := httptest.NewRequest(http.MethodPatch, tc.path, bytes.NewReader([]byte(tc.body)))
			req.Header.Set("Content-Type", tc.ct)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if service.lastDirectoryGroupMembershipRoleUpdate.ID != "" {
				t.Fatalf("dispatched request %+v", service.lastDirectoryGroupMembershipRoleUpdate)
			}
		})
	}
}

func TestAdminReassignDirectoryGroupMembershipHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		directoryGroupMembership: directory.GroupMembership{
			ID:         "membership-1",
			GroupID:    "group-2",
			CompanyID:  "company-1",
			MemberKind: directory.PrincipalKindUser,
			MemberID:   "user-2",
			Role:       directory.GroupMembershipRoleOwner,
			Status:     "active",
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodPatch, "/admin/v1/directory/group-memberships/%20membership-1%20/assignment", bytes.NewReader([]byte(`{"group_id":"group-2","member_kind":"user","member_id":"user-2"}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var response struct {
		Membership directory.GroupMembership `json:"directory_group_membership"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Membership.ID != "membership-1" || response.Membership.GroupID != "group-2" || response.Membership.MemberID != "user-2" {
		t.Fatalf("directory_group_membership = %+v", response.Membership)
	}
	if service.lastDirectoryGroupMembershipReassign.ID != "membership-1" ||
		service.lastDirectoryGroupMembershipReassign.GroupID != "group-2" ||
		service.lastDirectoryGroupMembershipReassign.MemberKind != directory.PrincipalKindUser ||
		service.lastDirectoryGroupMembershipReassign.MemberID != "user-2" {
		t.Fatalf("lastDirectoryGroupMembershipReassign = %+v", service.lastDirectoryGroupMembershipReassign)
	}
}

func TestAdminReassignDirectoryGroupMembershipHandlerRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		body string
		ct   string
	}{
		{name: "bad path value", path: "/admin/v1/directory/group-memberships/membership%0Abad/assignment", body: `{"group_id":"group-2","member_kind":"user","member_id":"user-2"}`, ct: "application/json"},
		{name: "unknown query", path: "/admin/v1/directory/group-memberships/membership-1/assignment?dry_run=true", body: `{"group_id":"group-2","member_kind":"user","member_id":"user-2"}`, ct: "application/json"},
		{name: "bad content type", path: "/admin/v1/directory/group-memberships/membership-1/assignment", body: `{"group_id":"group-2","member_kind":"user","member_id":"user-2"}`, ct: "text/plain"},
		{name: "unknown json field", path: "/admin/v1/directory/group-memberships/membership-1/assignment", body: `{"group_id":"group-2","member_kind":"user","member_id":"user-2","extra":true}`, ct: "application/json"},
		{name: "self group", path: "/admin/v1/directory/group-memberships/membership-1/assignment", body: `{"group_id":"group-2","member_kind":"group","member_id":"group-2"}`, ct: "application/json"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			service := &fakeAdminService{}
			mux := http.NewServeMux()
			RegisterAdminRoutes(mux, service, "")

			req := httptest.NewRequest(http.MethodPatch, tc.path, bytes.NewReader([]byte(tc.body)))
			req.Header.Set("Content-Type", tc.ct)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if service.lastDirectoryGroupMembershipReassign.ID != "" {
				t.Fatalf("dispatched request %+v", service.lastDirectoryGroupMembershipReassign)
			}
		})
	}
}

type fakeAdminService struct {
	companies                                   []maildb.CompanyView
	domains                                     []maildb.DomainView
	dnsReport                                   dnscheck.DomainReport
	dnsChecks                                   []maildb.DomainDNSCheckView
	users                                       []maildb.UserView
	queueStats                                  []maildb.QueueStat
	outboxEvents                                []maildb.OutboxEventView
	outboxEvent                                 maildb.OutboxEventView
	auditLogs                                   []maildb.AuditLogView
	auditLog                                    maildb.AuditLogView
	auditLogIntegrity                           maildb.AuditLogIntegrityView
	quotaUsage                                  []maildb.QuotaUsageView
	apiUsageDaily                               []maildb.APIUsageDailyView
	apiUsageMonthly                             []maildb.APIUsageMonthlyView
	apiUsageLedger                              []maildb.APIUsageLedgerView
	apiUsageLedgerStats                         maildb.APIUsageLedgerStatsView
	apiUsageLedgerRetentionReadiness            maildb.APIUsageLedgerRetentionReadinessView
	apiUsageLedgerRetentionRun                  maildb.APIUsageLedgerRetentionRunView
	apiUsageLedgerRetentionRuns                 []maildb.APIUsageLedgerRetentionRunView
	davSyncRetentionRun                         davsyncretention.RunRecord
	davSyncRetentionRuns                        []davsyncretention.RunRecord
	davSyncRetentionReadiness                   davsyncretention.ReadinessView
	apiUsageExportCapabilities                  maildb.APIUsageExportCapabilityView
	apiUsageExportBatch                         maildb.APIUsageExportBatchView
	apiUsageExportBatches                       []maildb.APIUsageExportBatchView
	attachmentUploadSessions                    []maildb.AttachmentUploadSession
	directoryPrincipals                         []directory.Principal
	directoryAlias                              directory.Alias
	directoryAliases                            []directory.Alias
	directoryDelegation                         directory.Delegation
	directoryDelegations                        []directory.Delegation
	directoryGroupMembership                    directory.GroupMembership
	directoryGroupMemberships                   []directory.GroupMembership
	driveNode                                   drive.Node
	driveNodes                                  []drive.Node
	driveUsageSummary                           drive.UsageSummary
	driveUploadSessions                         []drive.UploadSession
	apiUsageExportHandoff                       maildb.APIUsageExportHandoffView
	apiUsageExportArtifact                      maildb.APIUsageExportArtifactView
	apiUsageExportArtifacts                     []maildb.APIUsageExportArtifactView
	apiUsageExportArtifactBody                  string
	apiUsageExportArtifactVerification          maildb.APIUsageExportArtifactVerificationView
	apiUsageExportManifestDigest                maildb.APIUsageExportManifestDigestView
	apiUsageExportManifestDigests               []maildb.APIUsageExportManifestDigestView
	apiUsageExportManifestDigestVerification    maildb.APIUsageExportManifestDigestVerificationView
	apiUsageExportManifestSignature             maildb.APIUsageExportManifestSignatureView
	apiUsageExportManifestSignatures            []maildb.APIUsageExportManifestSignatureView
	apiUsageExportManifestSignatureVerification maildb.APIUsageExportManifestSignatureVerificationView
	quotaReconciliation                         []maildb.QuotaReconciliationView
	quotaCorrection                             maildb.QuotaCorrectionResult
	expiredAttachments                          []maildb.Attachment
	expiredAttachmentSessions                   []maildb.AttachmentUploadSession
	staleAttachmentCount                        maildb.StaleAttachmentUploadCount
	staleAttachmentSessionCount                 maildb.StaleAttachmentUploadSessionCount
	staleAttachmentCandidates                   []maildb.StaleAttachmentUploadCandidate
	staleAttachmentSessionCandidates            []maildb.StaleAttachmentUploadSessionCandidate
	staleDriveUploadSessionCount                drive.StaleUploadSessionCount
	staleDriveUploadSessions                    []drive.UploadSession
	expiredDriveUploadSessions                  []drive.UploadSession
	driveCleanupFailures                        []drive.ObjectCleanupFailure
	resolvedDriveCleanupFailure                 drive.ObjectCleanupFailure
	driveCleanupRetryResult                     drive.RetryObjectCleanupFailuresResult
	attempts                                    []maildb.DeliveryAttemptView
	deliveryAttemptStats                        maildb.DeliveryAttemptStatsView
	ldapSyncErr                                 error
	rdbmsSyncErr                                error
	lastDeliveryAttemptList                     maildb.DeliveryAttemptListRequest
	lastDeliveryAttemptStats                    maildb.DeliveryAttemptStatsRequest
	lastExhaustedAttemptList                    maildb.ExhaustedAttemptListRequest
	pushNotificationAttempts                    []maildb.PushNotificationAttemptView
	pushNotificationStats                       maildb.PushNotificationStatsView
	suppression                                 []maildb.SuppressionEntry
	trustedRelays                               []maildb.TrustedRelayView
	deliveryRoutes                              []maildb.DeliveryRouteView
	deliveryRouteResolution                     maildb.DeliveryRouteResolveView
	imapUIDBackfill                             []maildb.IMAPMessageUID
	dkimKeys                                    []maildb.DKIMKeyView
	backpressureState                           backpressure.State
	createdDKIMKeyID                            string
	lastLimit                                   int
	lastOutboxEventList                         maildb.OutboxEventListRequest
	lastOutboxEventID                           string
	lastAuditLogList                            maildb.AuditLogListRequest
	lastAuditLogID                              string
	lastAuditLogIntegrity                       maildb.AuditLogIntegrityRequest
	lastSuppressionList                         maildb.SuppressionEntryListRequest
	lastDomainList                              maildb.DomainListRequest
	lastCompanyID                               string
	lastDomainID                                string
	lastUserID                                  string
	lastUserList                                maildb.UserListRequest
	lastDomainStatus                            maildb.UpdateDomainStatusRequest
	lastDomainDNSCheckList                      maildb.DomainDNSCheckListRequest
	lastCompanyQuota                            maildb.UpdateCompanyQuotaRequest
	lastDomainQuota                             maildb.UpdateDomainQuotaRequest
	lastDomainPolicy                            maildb.UpdateDomainPolicyRequest
	lastDomainSettings                          admin.DomainSettings
	lastCreateDomain                            maildb.CreateDomainRequest
	lastUserStatus                              maildb.UpdateUserStatusRequest
	lastBulkUserStatus                          maildb.BulkUpdateUserStatusRequest
	lastUserQuota                               maildb.UpdateUserQuotaRequest
	lastUserPasswordHash                        maildb.UpdateUserPasswordHashRequest
	lastUserRecoveryEmail                       maildb.UpdateUserRecoveryEmailRequest
	lastDeleteUserID                            string
	lastQuotaCorrection                         maildb.CorrectQuotaReconciliationRequest
	lastAttachmentCleanupBefore                 time.Time
	lastAttachmentCleanupLimit                  int
	lastAttachmentSessionCleanupBefore          time.Time
	lastAttachmentSessionCleanupLimit           int
	lastAttachmentCleanupCountBefore            time.Time
	lastAttachmentCleanupCountLimit             int
	lastAttachmentSessionCleanupCountBefore     time.Time
	lastAttachmentSessionCleanupCountLimit      int
	lastQuotaUsageList                          maildb.QuotaUsageListRequest
	lastAttachmentCleanupListBefore             time.Time
	lastAttachmentCleanupListLimit              int
	lastAttachmentSessionCleanupListBefore      time.Time
	lastAttachmentSessionCleanupListLimit       int
	lastAttachmentUploadSessionList             maildb.AttachmentUploadSessionListRequest
	lastDirectoryPrincipalSearch                directory.SearchPrincipalsRequest
	lastDirectoryAliasCreate                    directory.CreateAliasRequest
	lastDirectoryAliasDeleteID                  string
	lastDirectoryAliasResolve                   directory.ResolveAliasRequest
	lastDirectoryAliasList                      directory.ListAliasesRequest
	lastDirectoryDelegationCreate               directory.CreateDelegationRequest
	lastDirectoryDelegationDeleteID             string
	lastDirectoryDelegationList                 directory.ListDelegationsRequest
	lastDirectoryDelegationRoleUpdate           directory.UpdateDelegationRoleRequest
	lastDirectoryDelegationReassign             directory.ReassignDelegationRequest
	lastDirectoryGroupMembershipCreate          directory.CreateGroupMembershipRequest
	lastDirectoryGroupMembershipList            directory.ListGroupMembershipsRequest
	lastDirectoryGroupMembershipDeleteID        string
	lastDirectoryGroupMembershipRoleUpdate      directory.UpdateGroupMembershipRoleRequest
	lastDirectoryGroupMembershipReassign        directory.ReassignGroupMembershipRequest
	lastDriveNodeGet                            drive.GetNodeRequest
	lastDriveNodeList                           drive.ListNodesRequest
	lastDriveUsage                              drive.GetUsageSummaryRequest
	lastDriveUploadSessionList                  drive.ListUploadSessionsRequest
	lastDriveUploadCleanupBefore                time.Time
	lastDriveUploadCleanupLimit                 int
	lastDriveCleanupFailureList                 drive.ListObjectCleanupFailuresRequest
	lastResolveDriveCleanupFailureID            string
	lastDriveCleanupFailureRetry                drive.ListObjectCleanupFailuresRequest
	lastAPIUsageDailyList                       maildb.APIUsageAggregateListRequest
	lastAPIUsageMonthlyList                     maildb.APIUsageAggregateListRequest
	lastAPIUsageLedgerList                      maildb.APIUsageLedgerListRequest
	lastAPIUsageLedgerRetention                 maildb.APIUsageLedgerRetentionRequest
	lastAPIUsageLedgerRetentionRun              maildb.APIUsageLedgerRetentionRunRequest
	lastAPIUsageLedgerRetentionRunList          maildb.APIUsageLedgerRetentionRunListRequest
	lastAPIUsageLedgerRetentionRunID            string
	lastDAVSyncRetentionRun                     davsyncretention.RunRequest
	lastDAVSyncRetentionRunList                 davsyncretention.RunListRequest
	lastDAVSyncRetentionRunID                   string
	lastDAVSyncRetentionReadiness               davsyncretention.ReadinessRequest
	lastAPIUsageExportCapabilities              bool
	lastAPIUsageExportBatchID                   string
	lastAPIUsageExportBatchList                 maildb.APIUsageExportBatchListRequest
	lastAPIUsageExportHandoffDeep               bool
	lastAPIUsageExportArtifactID                string
	lastAPIUsageExportManifestDigestID          string
	lastAPIUsageExportManifestSignatureID       string
	lastCreateAPIUsageExportArtifact            maildb.CreateAPIUsageExportArtifactRequest
	lastWriteAPIUsageExportArtifact             maildb.WriteAPIUsageExportArtifactRequest
	lastPushAttemptList                         maildb.PushNotificationAttemptListRequest
	lastPushOutcome                             maildb.UpdatePushNotificationOutcomeRequest
	lastPushNotificationStats                   maildb.PushNotificationStatsRequest
	lastCreateUser                              maildb.CreateUserRequest
	lastDKIMKeyList                             maildb.DKIMKeyListRequest
	lastCreateDKIMKey                           maildb.CreateDKIMKeyInput
	lastTrustedRelayList                        maildb.TrustedRelayListRequest
	lastCreateTrustedRelay                      maildb.CreateTrustedRelayRequest
	lastDeliveryRouteList                       maildb.DeliveryRouteListRequest
	lastCreateDeliveryRoute                     maildb.CreateDeliveryRouteRequest
	lastResolveDeliveryRouteDomain              string
	lastDeliveryRouteStatus                     maildb.UpdateDeliveryRouteStatusRequest
	lastIMAPBackfillUserID                      string
	lastIMAPBackfillMailboxID                   string
	lastIMAPBackfillLimit                       int
	lastBackpressureUpdate                      backpressure.StateUpdate
	lastDeactivateDKIMKeyID                     string
	lastVerifyDKIMKeyID                         string
	lastRetryOutboxID                           string
	lastDeleteSuppressionID                     string
	lastDeleteTrustedRelayID                    string
	lastDeleteDeliveryRouteID                   string
	lastCompanyList                             maildb.CompanyListRequest
	mailFlowLogs                                []maildb.MailFlowLogView
	mailFlowLog                                 maildb.MailFlowLogView
	mailFlowLogStats                            maildb.MailFlowLogStatsView
	mailFlowLogDailyStats                       []maildb.MailFlowLogDailyStatsView
	lastMailFlowLogList                         maildb.MailFlowLogListRequest
	lastMailFlowLogID                           string
	lastMailFlowLogStats                        maildb.MailFlowLogStatsRequest
	lastMailFlowLogDailyStats                   maildb.MailFlowLogDailyStatsRequest
	quotaAlertThresholds                        []maildb.QuotaAlertThresholdView
	quotaAlertThreshold                         maildb.QuotaAlertThresholdView
	quotaAlerts                                 []maildb.QuotaAlertView
	quotaAlert                                  maildb.QuotaAlertView
	lastQuotaAlertThresholdList                 maildb.QuotaAlertThresholdListRequest
	lastQuotaAlertThresholdID                   string
	lastCreateQuotaAlertThreshold               maildb.CreateQuotaAlertThresholdRequest
	lastUpdateQuotaAlertThreshold               maildb.UpdateQuotaAlertThresholdRequest
	lastDeleteQuotaAlertThresholdID             string
	lastQuotaAlertList                          maildb.QuotaAlertListRequest
	lastQuotaAlertID                            string
	companyConfig                               []configstore.ConfigEntry
	domainConfig                                []configstore.ConfigEntry
	userConfig                                  []configstore.ConfigEntry
	lastCompanyConfigID                         string
	lastCompanyConfigKey                        string
	lastDomainConfigID                          string
	lastDomainConfigKey                         string
	lastUserConfigID                            string
	lastUserConfigKey                           string
	lastPropagateCompanyID                      string
	lastPropagateScope                          configstore.PropagateScope
	pushDevices                                 []maildb.PushDevice
	lastListDevicesUserID                       string
	lastDeleteDeviceUserID                      string
	lastDeleteDeviceID                          string
	lastDeleteAllDevicesUserID                  string
	deleteAllDevicesCount                       int
	alertRules                                  []admin.AlertRule
	alertChannels                               []admin.AlertChannel
	alertEvents                                 []admin.AlertEvent
	adminRoles                                  []admin.RoleSummary
	createdAdminRole                            admin.RoleSummary
	roleErr                                     error
	lastCreateAlertRule                         *admin.AlertRule
	lastGetAlertRuleID                          string
	lastListAlertRulesCompanyID                 string
	lastUpdateAlertRule                         *admin.AlertRule
	lastDeleteAlertRuleID                       string
	lastCreateAlertChannel                      *admin.AlertChannel
	lastGetAlertChannelID                       string
	lastListAlertChannelsCompanyID              string
	lastUpdateAlertChannel                      *admin.AlertChannel
	lastDeleteAlertChannelID                    string
	lastListAlertEventsFilter                   admin.AlertEventFilter
	lastLogAlertEvent                           *admin.AlertEvent
	lastRoleCompanyID                           string
	lastCreateAdminRole                         admin.CreateRoleRequest
	authenticatedUser                           maildb.AuthenticatedUser
	authErr                                     error
	sessionVersions                             map[string]int64
	lastSessionRevokedUserID                    string
	ldapSyncRuns                                []maildb.LDAPSyncRunView
	ldapSyncConflicts                           []maildb.LDAPSyncConflictView
	rdbmsSyncRuns                               []maildb.RDBMSSyncRunView
	rdbmsSyncConflicts                          []maildb.RDBMSSyncConflictView
	lastLDAPSyncRunsReq                         maildb.LDAPSyncRunListRequest
	lastLDAPSyncConflictsReq                    maildb.LDAPSyncConflictListRequest
	lastRDBMSSyncRunsReq                        maildb.RDBMSSyncRunListRequest
	lastRDBMSSyncConflictsReq                   maildb.RDBMSSyncConflictListRequest
}

func (f *fakeAdminService) ListCompanies(_ context.Context, req maildb.CompanyListRequest) ([]maildb.CompanyView, bool, error) {
	f.lastLimit = req.Limit
	f.lastCompanyList = req
	companies := f.companies
	hasMore := false
	if req.ProbeMore {
		limit := req.Limit
		if limit <= 0 {
			limit = maildb.MessageListDefaultLimit
		}
		if len(companies) > limit {
			hasMore = true
			companies = companies[:limit]
		}
	}
	return companies, hasMore, nil
}

func (f *fakeAdminService) ListAdminRoles(_ context.Context, companyID string) ([]admin.RoleSummary, error) {
	f.lastRoleCompanyID = companyID
	if f.roleErr != nil {
		return nil, f.roleErr
	}
	return f.adminRoles, nil
}

func (f *fakeAdminService) CreateAdminRole(_ context.Context, req admin.CreateRoleRequest) (admin.RoleSummary, error) {
	f.lastCreateAdminRole = req
	if f.roleErr != nil {
		return admin.RoleSummary{}, f.roleErr
	}
	if f.createdAdminRole.ID != "" {
		return f.createdAdminRole, nil
	}
	return admin.RoleSummary{
		ID:          "role-new",
		CompanyID:   req.CompanyID,
		Name:        req.Name,
		Description: req.Description,
		IsBuiltin:   false,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}, nil
}

func (f *fakeAdminService) GetCompany(_ context.Context, id string) (maildb.CompanyView, error) {
	f.lastCompanyID = id
	for _, company := range f.companies {
		if company.ID == id {
			return company, nil
		}
	}
	return maildb.CompanyView{}, nil
}

func (f *fakeAdminService) CreateCompany(_ context.Context, req maildb.CreateCompanyRequest) (maildb.CompanyView, error) {
	return maildb.CompanyView{ID: "company-new", Name: req.Name, Status: "active", QuotaLimit: req.QuotaLimit}, nil
}

func (f *fakeAdminService) UpdateCompanyQuota(_ context.Context, req maildb.UpdateCompanyQuotaRequest) error {
	f.lastCompanyQuota = req
	return nil
}

func (f *fakeAdminService) UpdateCompany(_ context.Context, req maildb.UpdateCompanyRequest) (maildb.CompanyView, error) {
	return maildb.CompanyView{ID: req.ID, Name: req.Name, Status: "active", QuotaLimit: req.QuotaLimit}, nil
}

func (f *fakeAdminService) DeleteCompany(_ context.Context, id string) error {
	return nil
}

func (f *fakeAdminService) ListDomains(_ context.Context, req maildb.DomainListRequest) ([]maildb.DomainView, bool, error) {
	f.lastDomainList = req
	f.lastLimit = req.Limit
	if req.CompanyID == "" {
		return f.domains, false, nil
	}
	hasCompanyIDs := false
	for _, domain := range f.domains {
		if domain.CompanyID != "" {
			hasCompanyIDs = true
			break
		}
	}
	if !hasCompanyIDs {
		return f.domains, false, nil
	}
	domains := make([]maildb.DomainView, 0, len(f.domains))
	for _, domain := range f.domains {
		if domain.CompanyID == req.CompanyID {
			domains = append(domains, domain)
		}
	}
	return domains, false, nil
}

func (f *fakeAdminService) CreateDomain(_ context.Context, req maildb.CreateDomainRequest) (maildb.DomainView, error) {
	f.lastCreateDomain = req
	return maildb.DomainView{ID: "domain-new", CompanyID: req.CompanyID, Name: req.Name, NameACE: req.NameACE, Status: "active"}, nil
}

func (f *fakeAdminService) GetDomain(_ context.Context, id string) (maildb.DomainView, error) {
	f.lastDomainID = id
	for _, domain := range f.domains {
		if domain.ID == id {
			return domain, nil
		}
	}
	return maildb.DomainView{}, nil
}

func (f *fakeAdminService) GetDomainStats(_ context.Context, id string) (maildb.DomainStatsView, error) {
	f.lastDomainID = id
	return maildb.DomainStatsView{DomainID: id}, nil
}

func (f *fakeAdminService) VerifyDomainDNS(_ context.Context, id string) (dnscheck.DomainReport, error) {
	f.lastDomainID = id
	return f.dnsReport, nil
}

func (f *fakeAdminService) ListDomainDNSChecks(_ context.Context, req maildb.DomainDNSCheckListRequest) ([]maildb.DomainDNSCheckView, error) {
	f.lastDomainID = req.DomainID
	f.lastLimit = req.Limit
	f.lastDomainDNSCheckList = req
	return f.dnsChecks, nil
}

func (f *fakeAdminService) UpdateDomainStatus(_ context.Context, req maildb.UpdateDomainStatusRequest) error {
	f.lastDomainStatus = req
	return nil
}

func (f *fakeAdminService) DeleteDomain(_ context.Context, id string) error {
	return nil
}

func (f *fakeAdminService) UpdateDomainQuota(_ context.Context, req maildb.UpdateDomainQuotaRequest) error {
	f.lastDomainQuota = req
	return nil
}

func (f *fakeAdminService) UpdateDomainPolicy(_ context.Context, req maildb.UpdateDomainPolicyRequest) (maildb.DomainPolicyView, error) {
	f.lastDomainPolicy = req
	return maildb.DomainPolicyView{
		DomainID:                req.ID,
		InboundMode:             req.InboundMode,
		OutboundMode:            req.OutboundMode,
		MaxRecipientsPerMessage: req.MaxRecipientsPerMessage,
		MaxMessageBytes:         req.MaxMessageBytes,
		MaxAttachmentBytes:      req.MaxAttachmentBytes,
	}, nil
}

func (f *fakeAdminService) ListUsers(_ context.Context, req maildb.UserListRequest) ([]maildb.UserView, bool, error) {
	f.lastUserList = req
	f.lastDomainID = req.DomainID
	f.lastLimit = req.Limit
	if req.CompanyID != "" {
		companyDomainIDs := make(map[string]struct{})
		for _, domain := range f.domains {
			if domain.CompanyID == req.CompanyID {
				companyDomainIDs[domain.ID] = struct{}{}
			}
		}
		if len(companyDomainIDs) > 0 {
			users := make([]maildb.UserView, 0, len(f.users))
			for _, user := range f.users {
				if _, ok := companyDomainIDs[user.DomainID]; ok {
					users = append(users, user)
				}
			}
			return users, false, nil
		}
	}
	if req.DomainID == "" {
		return f.users, false, nil
	}
	hasDomainIDs := false
	for _, user := range f.users {
		if user.DomainID != "" {
			hasDomainIDs = true
			break
		}
	}
	if !hasDomainIDs {
		return f.users, false, nil
	}
	users := make([]maildb.UserView, 0, len(f.users))
	for _, user := range f.users {
		if user.DomainID == req.DomainID {
			users = append(users, user)
		}
	}
	return users, false, nil
}

func (f *fakeAdminService) CreateUser(_ context.Context, req maildb.CreateUserRequest) (maildb.UserView, error) {
	f.lastCreateUser = req
	return maildb.UserView{ID: "user-new", DomainID: req.DomainID, Username: req.Username, DisplayName: req.DisplayName, Status: "active"}, nil
}

func (f *fakeAdminService) GetUser(_ context.Context, id string) (maildb.UserView, error) {
	f.lastUserID = id
	for _, user := range f.users {
		if user.ID == id {
			return user, nil
		}
	}
	return maildb.UserView{}, nil
}

func (f *fakeAdminService) UpdateUserStatus(_ context.Context, req maildb.UpdateUserStatusRequest) error {
	f.lastUserStatus = req
	return nil
}

func (f *fakeAdminService) BulkUpdateUserStatus(_ context.Context, req maildb.BulkUpdateUserStatusRequest) (maildb.BulkUpdateUserStatusResult, error) {
	f.lastBulkUserStatus = req
	return maildb.BulkUpdateUserStatusResult{Updated: req.IDs}, nil
}

func (f *fakeAdminService) UpdateUserQuota(_ context.Context, req maildb.UpdateUserQuotaRequest) error {
	f.lastUserQuota = req
	return nil
}

func (f *fakeAdminService) UpdateUserPasswordHash(_ context.Context, req maildb.UpdateUserPasswordHashRequest) error {
	f.lastUserPasswordHash = req
	return nil
}

func (f *fakeAdminService) UpdateUserRole(_ context.Context, req maildb.UpdateUserRoleRequest) error {
	return nil
}

func (f *fakeAdminService) UpdateUserRecoveryEmail(_ context.Context, req maildb.UpdateUserRecoveryEmailRequest) error {
	f.lastUserRecoveryEmail = req
	return nil
}

func (f *fakeAdminService) DeleteUser(_ context.Context, id string) error {
	f.lastDeleteUserID = id
	return nil
}

func (f *fakeAdminService) AuthenticateUser(_ context.Context, email, password string) (maildb.AuthenticatedUser, error) {
	if f.authErr != nil {
		return maildb.AuthenticatedUser{}, f.authErr
	}
	if f.authenticatedUser.UserID == "" {
		return maildb.AuthenticatedUser{}, fmt.Errorf("invalid credentials")
	}
	return f.authenticatedUser, nil
}

func (f *fakeAdminService) SessionVersionFor(_ context.Context, userID string) (int64, error) {
	if f.sessionVersions == nil {
		return 0, nil
	}
	return f.sessionVersions[userID], nil
}

func (f *fakeAdminService) IncrementSessionVersion(_ context.Context, userID string) (int64, error) {
	if f.sessionVersions == nil {
		f.sessionVersions = make(map[string]int64)
	}
	f.sessionVersions[userID]++
	f.lastSessionRevokedUserID = userID
	return f.sessionVersions[userID], nil
}

func (f *fakeAdminService) ListQueueStats(context.Context) ([]maildb.QueueStat, error) {
	return f.queueStats, nil
}

func (f *fakeAdminService) ListOutboxEvents(_ context.Context, req maildb.OutboxEventListRequest) ([]maildb.OutboxEventView, bool, error) {
	f.lastOutboxEventList = req
	if req.Status != "" && req.Status != "pending" && req.Status != "processing" && req.Status != "done" && req.Status != "failed" {
		return nil, false, fmt.Errorf("unsupported outbox status")
	}
	return f.outboxEvents, false, nil
}

func (f *fakeAdminService) GetOutboxEvent(_ context.Context, id string) (maildb.OutboxEventView, error) {
	f.lastOutboxEventID = id
	if f.outboxEvent.ID == "" {
		return maildb.OutboxEventView{}, fmt.Errorf("outbox event %q not found", id)
	}
	return f.outboxEvent, nil
}

func (f *fakeAdminService) ListAuditLogs(_ context.Context, req maildb.AuditLogListRequest) ([]maildb.AuditLogView, bool, error) {
	f.lastAuditLogList = req
	return f.auditLogs, false, nil
}

func (f *fakeAdminService) GetAuditLog(_ context.Context, id string) (maildb.AuditLogView, error) {
	f.lastAuditLogID = id
	if f.auditLog.ID == "" {
		return maildb.AuditLogView{}, fmt.Errorf("audit log %q not found", id)
	}
	return f.auditLog, nil
}

func (f *fakeAdminService) CheckAuditLogIntegrity(_ context.Context, req maildb.AuditLogIntegrityRequest) (maildb.AuditLogIntegrityView, error) {
	f.lastAuditLogIntegrity = req
	return f.auditLogIntegrity, nil
}

func (f *fakeAdminService) ListMailFlowLogs(_ context.Context, req maildb.MailFlowLogListRequest) ([]maildb.MailFlowLogView, error) {
	f.lastMailFlowLogList = req
	return f.mailFlowLogs, nil
}

func (f *fakeAdminService) GetMailFlowLog(_ context.Context, id string) (maildb.MailFlowLogView, error) {
	f.lastMailFlowLogID = id
	if f.mailFlowLog.ID == "" {
		return maildb.MailFlowLogView{}, fmt.Errorf("mail flow log %q not found", id)
	}
	return f.mailFlowLog, nil
}

func (f *fakeAdminService) GetMailFlowLogStats(_ context.Context, req maildb.MailFlowLogStatsRequest) (maildb.MailFlowLogStatsView, error) {
	f.lastMailFlowLogStats = req
	return f.mailFlowLogStats, nil
}

func (f *fakeAdminService) GetMailFlowLogDailyStats(_ context.Context, req maildb.MailFlowLogDailyStatsRequest) ([]maildb.MailFlowLogDailyStatsView, error) {
	f.lastMailFlowLogDailyStats = req
	return f.mailFlowLogDailyStats, nil
}

func (f *fakeAdminService) SearchDirectoryPrincipals(_ context.Context, req directory.SearchPrincipalsRequest) ([]directory.Principal, error) {
	if _, err := directory.NormalizeSearchPrincipalsRequest(req); err != nil {
		return nil, err
	}
	f.lastDirectoryPrincipalSearch = req
	return f.directoryPrincipals, nil
}

func (f *fakeAdminService) CreateDirectoryAlias(_ context.Context, req directory.CreateAliasRequest) (directory.Alias, error) {
	if _, err := directory.NormalizeCreateAliasRequest(req); err != nil {
		return directory.Alias{}, err
	}
	f.lastDirectoryAliasCreate = req
	return f.directoryAlias, nil
}

func (f *fakeAdminService) DeleteDirectoryAlias(_ context.Context, id string) (directory.Alias, error) {
	if _, err := directory.NormalizePrincipalID(id); err != nil {
		return directory.Alias{}, err
	}
	f.lastDirectoryAliasDeleteID = id
	return f.directoryAlias, nil
}

func (f *fakeAdminService) ResolveDirectoryAlias(_ context.Context, req directory.ResolveAliasRequest) (directory.Alias, error) {
	if _, err := directory.NormalizeResolveAliasRequest(req); err != nil {
		return directory.Alias{}, err
	}
	f.lastDirectoryAliasResolve = req
	return f.directoryAlias, nil
}

func (f *fakeAdminService) ListDirectoryAliases(_ context.Context, req directory.ListAliasesRequest) ([]directory.Alias, error) {
	if _, err := directory.NormalizeListAliasesRequest(req); err != nil {
		return nil, err
	}
	f.lastDirectoryAliasList = req
	return f.directoryAliases, nil
}

func (f *fakeAdminService) CreateDirectoryDelegation(_ context.Context, req directory.CreateDelegationRequest) (directory.Delegation, error) {
	if _, err := directory.NormalizeCreateDelegationRequest(req); err != nil {
		return directory.Delegation{}, err
	}
	f.lastDirectoryDelegationCreate = req
	return f.directoryDelegation, nil
}

func (f *fakeAdminService) CreateDirectoryGroupMembership(_ context.Context, req directory.CreateGroupMembershipRequest) (directory.GroupMembership, error) {
	if _, err := directory.NormalizeCreateGroupMembershipRequest(req); err != nil {
		return directory.GroupMembership{}, err
	}
	f.lastDirectoryGroupMembershipCreate = req
	return f.directoryGroupMembership, nil
}

func (f *fakeAdminService) ListDirectoryGroupMemberships(_ context.Context, req directory.ListGroupMembershipsRequest) ([]directory.GroupMembership, error) {
	if _, err := directory.NormalizeListGroupMembershipsRequest(req); err != nil {
		return nil, err
	}
	f.lastDirectoryGroupMembershipList = req
	return f.directoryGroupMemberships, nil
}

func (f *fakeAdminService) DeleteDirectoryGroupMembership(_ context.Context, id string) (directory.GroupMembership, error) {
	normalized, err := directory.NormalizePrincipalID(id)
	if err != nil {
		return directory.GroupMembership{}, err
	}
	f.lastDirectoryGroupMembershipDeleteID = normalized
	return f.directoryGroupMembership, nil
}

func (f *fakeAdminService) UpdateDirectoryGroupMembershipRole(_ context.Context, req directory.UpdateGroupMembershipRoleRequest) (directory.GroupMembership, error) {
	normalized, err := directory.NormalizeUpdateGroupMembershipRoleRequest(req)
	if err != nil {
		return directory.GroupMembership{}, err
	}
	f.lastDirectoryGroupMembershipRoleUpdate = normalized
	return f.directoryGroupMembership, nil
}

func (f *fakeAdminService) ReassignDirectoryGroupMembership(_ context.Context, req directory.ReassignGroupMembershipRequest) (directory.GroupMembership, error) {
	normalized, err := directory.NormalizeReassignGroupMembershipRequest(req)
	if err != nil {
		return directory.GroupMembership{}, err
	}
	f.lastDirectoryGroupMembershipReassign = normalized
	return f.directoryGroupMembership, nil
}

func (f *fakeAdminService) ReassignDirectoryDelegation(_ context.Context, req directory.ReassignDelegationRequest) (directory.Delegation, error) {
	normalized, err := directory.NormalizeReassignDelegationRequest(req)
	if err != nil {
		return directory.Delegation{}, err
	}
	f.lastDirectoryDelegationReassign = normalized
	return f.directoryDelegation, nil
}

func (f *fakeAdminService) DeleteDirectoryDelegation(_ context.Context, id string) (directory.Delegation, error) {
	if _, err := directory.NormalizePrincipalID(id); err != nil {
		return directory.Delegation{}, err
	}
	f.lastDirectoryDelegationDeleteID = id
	return f.directoryDelegation, nil
}

func (f *fakeAdminService) ListDirectoryDelegations(_ context.Context, req directory.ListDelegationsRequest) ([]directory.Delegation, error) {
	if _, err := directory.NormalizeListDelegationsRequest(req); err != nil {
		return nil, err
	}
	f.lastDirectoryDelegationList = req
	return f.directoryDelegations, nil
}

func (f *fakeAdminService) UpdateDirectoryDelegationRole(_ context.Context, req directory.UpdateDelegationRoleRequest) (directory.Delegation, error) {
	normalized, err := directory.NormalizeUpdateDelegationRoleRequest(req)
	if err != nil {
		return directory.Delegation{}, err
	}
	f.lastDirectoryDelegationRoleUpdate = normalized
	return f.directoryDelegation, nil
}

func (f *fakeAdminService) GetBackpressure(context.Context) (backpressure.State, error) {
	if f.backpressureState.Level == "" {
		return backpressure.State{Level: "normal"}, nil
	}
	return f.backpressureState, nil
}

func (f *fakeAdminService) UpdateBackpressure(_ context.Context, req backpressure.StateUpdate) (backpressure.State, error) {
	f.lastBackpressureUpdate = req
	return backpressure.State{Level: req.Level, Reason: req.Reason}, nil
}

func (f *fakeAdminService) ListQuotaUsage(_ context.Context, req maildb.QuotaUsageListRequest) ([]maildb.QuotaUsageView, error) {
	f.lastLimit = req.Limit
	f.lastQuotaUsageList = req
	return f.quotaUsage, nil
}

func (f *fakeAdminService) RunAttachmentCleanup(_ context.Context, before time.Time, limit int) ([]maildb.Attachment, error) {
	f.lastAttachmentCleanupBefore = before
	f.lastAttachmentCleanupLimit = limit
	return f.expiredAttachments, nil
}

func (f *fakeAdminService) RunAttachmentUploadSessionCleanup(_ context.Context, before time.Time, limit int) ([]maildb.AttachmentUploadSession, error) {
	f.lastAttachmentSessionCleanupBefore = before
	f.lastAttachmentSessionCleanupLimit = limit
	return f.expiredAttachmentSessions, nil
}

func (f *fakeAdminService) CountStaleAttachmentUploads(_ context.Context, before time.Time, limit int) (maildb.StaleAttachmentUploadCount, error) {
	f.lastAttachmentCleanupCountBefore = before
	f.lastAttachmentCleanupCountLimit = limit
	return f.staleAttachmentCount, nil
}

func (f *fakeAdminService) CountStaleAttachmentUploadSessions(_ context.Context, before time.Time, limit int) (maildb.StaleAttachmentUploadSessionCount, error) {
	f.lastAttachmentSessionCleanupCountBefore = before
	f.lastAttachmentSessionCleanupCountLimit = limit
	return f.staleAttachmentSessionCount, nil
}

func (f *fakeAdminService) ListStaleAttachmentUploads(_ context.Context, before time.Time, limit int) ([]maildb.StaleAttachmentUploadCandidate, error) {
	f.lastAttachmentCleanupListBefore = before
	f.lastAttachmentCleanupListLimit = limit
	return f.staleAttachmentCandidates, nil
}

func (f *fakeAdminService) ListStaleAttachmentUploadSessions(_ context.Context, before time.Time, limit int) ([]maildb.StaleAttachmentUploadSessionCandidate, error) {
	f.lastAttachmentSessionCleanupListBefore = before
	f.lastAttachmentSessionCleanupListLimit = limit
	return f.staleAttachmentSessionCandidates, nil
}

func (f *fakeAdminService) ListAttachmentUploadSessions(_ context.Context, req maildb.AttachmentUploadSessionListRequest) ([]maildb.AttachmentUploadSession, error) {
	f.lastAttachmentUploadSessionList = req
	f.lastLimit = req.Limit
	return f.attachmentUploadSessions, nil
}

func (f *fakeAdminService) ListDriveUploadSessions(_ context.Context, req drive.ListUploadSessionsRequest) ([]drive.UploadSession, error) {
	f.lastDriveUploadSessionList = req
	f.lastLimit = req.Limit
	return f.driveUploadSessions, nil
}

func (f *fakeAdminService) ListDriveNodes(_ context.Context, req drive.ListNodesRequest) ([]drive.Node, error) {
	f.lastDriveNodeList = req
	f.lastLimit = req.Limit
	return f.driveNodes, nil
}

func (f *fakeAdminService) GetDriveNode(_ context.Context, req drive.GetNodeRequest) (drive.Node, error) {
	f.lastDriveNodeGet = req
	return f.driveNode, nil
}

func (f *fakeAdminService) GetDriveUsageSummary(_ context.Context, req drive.GetUsageSummaryRequest) (drive.UsageSummary, error) {
	f.lastDriveUsage = req
	return f.driveUsageSummary, nil
}

func (f *fakeAdminService) CountStaleDriveUploadSessions(_ context.Context, before time.Time, limit int) (drive.StaleUploadSessionCount, error) {
	f.lastDriveUploadCleanupBefore = before
	f.lastDriveUploadCleanupLimit = limit
	return f.staleDriveUploadSessionCount, nil
}

func (f *fakeAdminService) ListStaleDriveUploadSessions(_ context.Context, before time.Time, limit int) ([]drive.UploadSession, error) {
	f.lastDriveUploadCleanupBefore = before
	f.lastDriveUploadCleanupLimit = limit
	return f.staleDriveUploadSessions, nil
}

func (f *fakeAdminService) RunDriveUploadSessionCleanup(_ context.Context, before time.Time, limit int) ([]drive.UploadSession, error) {
	f.lastDriveUploadCleanupBefore = before
	f.lastDriveUploadCleanupLimit = limit
	return f.expiredDriveUploadSessions, nil
}

func (f *fakeAdminService) ListDriveObjectCleanupFailures(_ context.Context, req drive.ListObjectCleanupFailuresRequest) ([]drive.ObjectCleanupFailure, error) {
	f.lastDriveCleanupFailureList = req
	f.lastLimit = req.Limit
	return f.driveCleanupFailures, nil
}

func (f *fakeAdminService) ResolveDriveObjectCleanupFailure(_ context.Context, id string) (drive.ObjectCleanupFailure, error) {
	f.lastResolveDriveCleanupFailureID = id
	return f.resolvedDriveCleanupFailure, nil
}

func (f *fakeAdminService) RetryDriveObjectCleanupFailures(_ context.Context, req drive.ListObjectCleanupFailuresRequest) (drive.RetryObjectCleanupFailuresResult, error) {
	f.lastDriveCleanupFailureRetry = req
	return f.driveCleanupRetryResult, nil
}

func (f *fakeAdminService) ListAPIUsageDaily(_ context.Context, req maildb.APIUsageAggregateListRequest) ([]maildb.APIUsageDailyView, error) {
	f.lastLimit = req.Limit
	f.lastAPIUsageDailyList = req
	return f.apiUsageDaily, nil
}

func (f *fakeAdminService) ListAPIUsageMonthly(_ context.Context, req maildb.APIUsageAggregateListRequest) ([]maildb.APIUsageMonthlyView, error) {
	f.lastLimit = req.Limit
	f.lastAPIUsageMonthlyList = req
	return f.apiUsageMonthly, nil
}

func (f *fakeAdminService) ListAPIUsageLedger(_ context.Context, req maildb.APIUsageLedgerListRequest) ([]maildb.APIUsageLedgerView, error) {
	f.lastLimit = req.Limit
	f.lastAPIUsageLedgerList = req
	return f.apiUsageLedger, nil
}

func (f *fakeAdminService) GetAPIUsageLedgerStats(_ context.Context, req maildb.APIUsageLedgerListRequest) (maildb.APIUsageLedgerStatsView, error) {
	f.lastAPIUsageLedgerList = req
	return f.apiUsageLedgerStats, nil
}

func (f *fakeAdminService) GetAPIUsageLedgerRetentionReadiness(_ context.Context, req maildb.APIUsageLedgerRetentionRequest) (maildb.APIUsageLedgerRetentionReadinessView, error) {
	f.lastAPIUsageLedgerRetention = req
	return f.apiUsageLedgerRetentionReadiness, nil
}

func (f *fakeAdminService) RunAPIUsageLedgerRetention(_ context.Context, req maildb.APIUsageLedgerRetentionRunRequest) (maildb.APIUsageLedgerRetentionRunView, error) {
	f.lastAPIUsageLedgerRetentionRun = req
	return f.apiUsageLedgerRetentionRun, nil
}

func (f *fakeAdminService) ListAPIUsageLedgerRetentionRuns(_ context.Context, req maildb.APIUsageLedgerRetentionRunListRequest) ([]maildb.APIUsageLedgerRetentionRunView, error) {
	f.lastAPIUsageLedgerRetentionRunList = req
	return f.apiUsageLedgerRetentionRuns, nil
}

func (f *fakeAdminService) GetAPIUsageLedgerRetentionRun(_ context.Context, id string) (maildb.APIUsageLedgerRetentionRunView, error) {
	f.lastAPIUsageLedgerRetentionRunID = id
	return f.apiUsageLedgerRetentionRun, nil
}

func (f *fakeAdminService) ListDAVSyncRetentionRuns(_ context.Context, req davsyncretention.RunListRequest) ([]davsyncretention.RunRecord, error) {
	f.lastDAVSyncRetentionRunList = req
	return f.davSyncRetentionRuns, nil
}

func (f *fakeAdminService) GetDAVSyncRetentionRun(_ context.Context, id string) (davsyncretention.RunRecord, error) {
	f.lastDAVSyncRetentionRunID = id
	return f.davSyncRetentionRun, nil
}

func (f *fakeAdminService) RunDAVSyncRetention(_ context.Context, req davsyncretention.RunRequest) (davsyncretention.RunRecord, error) {
	f.lastDAVSyncRetentionRun = req
	return f.davSyncRetentionRun, nil
}

func (f *fakeAdminService) GetDAVSyncRetentionReadiness(_ context.Context, req davsyncretention.ReadinessRequest) (davsyncretention.ReadinessView, error) {
	f.lastDAVSyncRetentionReadiness = req
	return f.davSyncRetentionReadiness, nil
}

func (f *fakeAdminService) GetAPIUsageExportCapabilities(context.Context) (maildb.APIUsageExportCapabilityView, error) {
	f.lastAPIUsageExportCapabilities = true
	return f.apiUsageExportCapabilities, nil
}

func (f *fakeAdminService) CreateAPIUsageExportBatch(_ context.Context, req maildb.APIUsageLedgerListRequest) (maildb.APIUsageExportBatchView, error) {
	f.lastAPIUsageLedgerList = req
	return f.apiUsageExportBatch, nil
}

func (f *fakeAdminService) ListAPIUsageExportBatches(_ context.Context, req maildb.APIUsageExportBatchListRequest) ([]maildb.APIUsageExportBatchView, error) {
	f.lastLimit = req.Limit
	f.lastAPIUsageExportBatchList = req
	return f.apiUsageExportBatches, nil
}

func (f *fakeAdminService) GetAPIUsageExportBatch(_ context.Context, id string) (maildb.APIUsageExportBatchView, error) {
	f.lastAPIUsageExportBatchID = id
	return f.apiUsageExportBatch, nil
}

func (f *fakeAdminService) GetAPIUsageExportHandoff(_ context.Context, batchID string, deep bool) (maildb.APIUsageExportHandoffView, error) {
	f.lastAPIUsageExportBatchID = batchID
	f.lastAPIUsageExportHandoffDeep = deep
	return f.apiUsageExportHandoff, nil
}

func (f *fakeAdminService) CreateAPIUsageExportArtifact(_ context.Context, req maildb.CreateAPIUsageExportArtifactRequest) (maildb.APIUsageExportArtifactView, error) {
	f.lastCreateAPIUsageExportArtifact = req
	return f.apiUsageExportArtifact, nil
}

func (f *fakeAdminService) WriteAPIUsageExportArtifact(_ context.Context, batchID string, req maildb.WriteAPIUsageExportArtifactRequest) (maildb.APIUsageExportArtifactView, error) {
	f.lastAPIUsageExportBatchID = batchID
	f.lastWriteAPIUsageExportArtifact = req
	return f.apiUsageExportArtifact, nil
}

func (f *fakeAdminService) ListAPIUsageExportArtifacts(_ context.Context, batchID string, limit int) ([]maildb.APIUsageExportArtifactView, error) {
	f.lastAPIUsageExportBatchID = batchID
	f.lastLimit = limit
	return f.apiUsageExportArtifacts, nil
}

func (f *fakeAdminService) GetAPIUsageExportArtifact(_ context.Context, batchID string, artifactID string) (maildb.APIUsageExportArtifactView, error) {
	f.lastAPIUsageExportBatchID = batchID
	f.lastAPIUsageExportArtifactID = artifactID
	return f.apiUsageExportArtifact, nil
}

func (f *fakeAdminService) OpenAPIUsageExportArtifact(_ context.Context, batchID string, artifactID string) (maildb.APIUsageExportArtifactView, io.ReadCloser, error) {
	f.lastAPIUsageExportBatchID = batchID
	f.lastAPIUsageExportArtifactID = artifactID
	return f.apiUsageExportArtifact, io.NopCloser(strings.NewReader(f.apiUsageExportArtifactBody)), nil
}

func (f *fakeAdminService) VerifyAPIUsageExportArtifact(_ context.Context, batchID string, artifactID string) (maildb.APIUsageExportArtifactVerificationView, error) {
	f.lastAPIUsageExportBatchID = batchID
	f.lastAPIUsageExportArtifactID = artifactID
	return f.apiUsageExportArtifactVerification, nil
}

func (f *fakeAdminService) CreateAPIUsageExportManifestDigest(_ context.Context, batchID string) (maildb.APIUsageExportManifestDigestView, error) {
	f.lastAPIUsageExportBatchID = batchID
	return f.apiUsageExportManifestDigest, nil
}

func (f *fakeAdminService) ListAPIUsageExportManifestDigests(_ context.Context, batchID string, limit int) ([]maildb.APIUsageExportManifestDigestView, error) {
	f.lastAPIUsageExportBatchID = batchID
	f.lastLimit = limit
	return f.apiUsageExportManifestDigests, nil
}

func (f *fakeAdminService) GetAPIUsageExportManifestDigest(_ context.Context, batchID string, digestID string) (maildb.APIUsageExportManifestDigestView, error) {
	f.lastAPIUsageExportBatchID = batchID
	f.lastAPIUsageExportManifestDigestID = digestID
	return f.apiUsageExportManifestDigest, nil
}

func (f *fakeAdminService) VerifyAPIUsageExportManifestDigest(_ context.Context, batchID string, digestID string) (maildb.APIUsageExportManifestDigestVerificationView, error) {
	f.lastAPIUsageExportBatchID = batchID
	f.lastAPIUsageExportManifestDigestID = digestID
	return f.apiUsageExportManifestDigestVerification, nil
}

func (f *fakeAdminService) CreateAPIUsageExportManifestSignature(_ context.Context, batchID string, digestID string) (maildb.APIUsageExportManifestSignatureView, error) {
	f.lastAPIUsageExportBatchID = batchID
	f.lastAPIUsageExportManifestDigestID = digestID
	return f.apiUsageExportManifestSignature, nil
}

func (f *fakeAdminService) ListAPIUsageExportManifestSignatures(_ context.Context, batchID string, digestID string, limit int) ([]maildb.APIUsageExportManifestSignatureView, error) {
	f.lastAPIUsageExportBatchID = batchID
	f.lastAPIUsageExportManifestDigestID = digestID
	f.lastLimit = limit
	return f.apiUsageExportManifestSignatures, nil
}

func (f *fakeAdminService) GetAPIUsageExportManifestSignature(_ context.Context, batchID string, digestID string, signatureID string) (maildb.APIUsageExportManifestSignatureView, error) {
	f.lastAPIUsageExportBatchID = batchID
	f.lastAPIUsageExportManifestDigestID = digestID
	f.lastAPIUsageExportManifestSignatureID = signatureID
	return f.apiUsageExportManifestSignature, nil
}

func (f *fakeAdminService) VerifyAPIUsageExportManifestSignature(_ context.Context, batchID string, digestID string, signatureID string) (maildb.APIUsageExportManifestSignatureVerificationView, error) {
	f.lastAPIUsageExportBatchID = batchID
	f.lastAPIUsageExportManifestDigestID = digestID
	f.lastAPIUsageExportManifestSignatureID = signatureID
	return f.apiUsageExportManifestSignatureVerification, nil
}

func (f *fakeAdminService) ListQuotaReconciliation(_ context.Context, limit int) ([]maildb.QuotaReconciliationView, error) {
	f.lastLimit = limit
	return f.quotaReconciliation, nil
}

func (f *fakeAdminService) CorrectQuotaReconciliation(_ context.Context, req maildb.CorrectQuotaReconciliationRequest) (maildb.QuotaCorrectionResult, error) {
	f.lastQuotaCorrection = req
	return f.quotaCorrection, nil
}

func (f *fakeAdminService) ListDeliveryAttempts(_ context.Context, req maildb.DeliveryAttemptListRequest) ([]maildb.DeliveryAttemptView, bool, error) {
	f.lastLimit = req.Limit
	f.lastDeliveryAttemptList = req
	if req.Status != "" && req.Status != "delivered" && req.Status != "failed" && req.Status != "bounced" && req.Status != "exhausted" {
		return nil, false, fmt.Errorf("unsupported delivery attempt status")
	}
	return f.attempts, false, nil
}

func (f *fakeAdminService) GetDeliveryAttemptStats(_ context.Context, req maildb.DeliveryAttemptStatsRequest) (maildb.DeliveryAttemptStatsView, error) {
	f.lastDeliveryAttemptStats = req
	if req.Status != "" && req.Status != "delivered" && req.Status != "failed" && req.Status != "bounced" && req.Status != "exhausted" {
		return maildb.DeliveryAttemptStatsView{}, fmt.Errorf("unsupported delivery attempt status")
	}
	return f.deliveryAttemptStats, nil
}

func (f *fakeAdminService) ListExhaustedAttempts(_ context.Context, req maildb.ExhaustedAttemptListRequest) ([]maildb.DeliveryAttemptView, error) {
	f.lastLimit = req.Limit
	f.lastExhaustedAttemptList = req
	return f.attempts, nil
}

func (f *fakeAdminService) ListPushNotificationAttempts(_ context.Context, req maildb.PushNotificationAttemptListRequest) ([]maildb.PushNotificationAttemptView, error) {
	f.lastPushAttemptList = req
	return f.pushNotificationAttempts, nil
}

func (f *fakeAdminService) GetPushNotificationAttempt(_ context.Context, id string) (maildb.PushNotificationAttemptView, error) {
	f.lastPushOutcome.AttemptID = id
	for _, attempt := range f.pushNotificationAttempts {
		if attempt.ID == id {
			return attempt, nil
		}
	}
	return maildb.PushNotificationAttemptView{}, fmt.Errorf("push notification attempt %q not found", id)
}

func (f *fakeAdminService) UpdatePushNotificationOutcome(_ context.Context, req maildb.UpdatePushNotificationOutcomeRequest) error {
	f.lastPushOutcome = req
	if req.Status != "queued" && req.Status != "delivered" && req.Status != "failed" && req.Status != "invalid_token" {
		return fmt.Errorf("unsupported push notification outcome status")
	}
	return nil
}

func (f *fakeAdminService) GetPushNotificationStats(_ context.Context, req maildb.PushNotificationStatsRequest) (maildb.PushNotificationStatsView, error) {
	f.lastPushNotificationStats = req
	return f.pushNotificationStats, nil
}

func (f *fakeAdminService) ListPushDevices(_ context.Context, userID string, _ int) ([]maildb.PushDevice, error) {
	f.lastListDevicesUserID = userID
	return f.pushDevices, nil
}

func (f *fakeAdminService) DeletePushDevice(_ context.Context, userID string, id string) error {
	f.lastDeleteDeviceUserID = userID
	f.lastDeleteDeviceID = id
	return nil
}

func (f *fakeAdminService) DeleteAllPushDevices(_ context.Context, userID string) (int, error) {
	f.lastDeleteAllDevicesUserID = userID
	return f.deleteAllDevicesCount, nil
}

func (f *fakeAdminService) ListSuppressionEntries(_ context.Context, req maildb.SuppressionEntryListRequest) ([]maildb.SuppressionEntry, error) {
	f.lastSuppressionList = req
	f.lastLimit = req.Limit
	return f.suppression, nil
}

func (f *fakeAdminService) ListTrustedRelays(_ context.Context, req maildb.TrustedRelayListRequest) ([]maildb.TrustedRelayView, error) {
	f.lastTrustedRelayList = req
	f.lastLimit = req.Limit
	return f.trustedRelays, nil
}

func (f *fakeAdminService) CreateTrustedRelay(_ context.Context, req maildb.CreateTrustedRelayRequest) (maildb.TrustedRelayView, error) {
	f.lastCreateTrustedRelay = req
	return maildb.TrustedRelayView{ID: "relay-new", CIDR: req.CIDR, Description: req.Description}, nil
}

func (f *fakeAdminService) DeleteTrustedRelay(_ context.Context, id string) error {
	f.lastDeleteTrustedRelayID = id
	return nil
}

func (f *fakeAdminService) ListDeliveryRoutes(_ context.Context, req maildb.DeliveryRouteListRequest) ([]maildb.DeliveryRouteView, error) {
	f.lastDeliveryRouteList = req
	f.lastLimit = req.Limit
	return f.deliveryRoutes, nil
}

func (f *fakeAdminService) CreateDeliveryRoute(_ context.Context, req maildb.CreateDeliveryRouteRequest) (maildb.DeliveryRouteView, error) {
	f.lastCreateDeliveryRoute = req
	return maildb.DeliveryRouteView{
		ID:            "route-new",
		DomainPattern: req.DomainPattern,
		Farm:          req.Farm,
		Hosts:         req.Hosts,
		Port:          req.Port,
		TLSMode:       req.TLSMode,
		Status:        "active",
	}, nil
}

func (f *fakeAdminService) ResolveDeliveryRoute(_ context.Context, domain string) (maildb.DeliveryRouteResolveView, error) {
	f.lastResolveDeliveryRouteDomain = domain
	return f.deliveryRouteResolution, nil
}

func (f *fakeAdminService) UpdateDeliveryRouteStatus(_ context.Context, req maildb.UpdateDeliveryRouteStatusRequest) error {
	f.lastDeliveryRouteStatus = req
	return nil
}

func (f *fakeAdminService) DeleteDeliveryRoute(_ context.Context, id string) error {
	f.lastDeleteDeliveryRouteID = id
	return nil
}

func (f *fakeAdminService) BackfillIMAPMailboxUIDs(_ context.Context, userID string, mailboxID string, limit int) ([]maildb.IMAPMessageUID, error) {
	f.lastIMAPBackfillUserID = userID
	f.lastIMAPBackfillMailboxID = mailboxID
	f.lastIMAPBackfillLimit = limit
	return f.imapUIDBackfill, nil
}

func (f *fakeAdminService) ListDKIMKeys(_ context.Context, req maildb.DKIMKeyListRequest) ([]maildb.DKIMKeyView, error) {
	f.lastDKIMKeyList = req
	f.lastDomainID = req.DomainID
	f.lastLimit = req.Limit
	return f.dkimKeys, nil
}

func (f *fakeAdminService) CreateDKIMKey(_ context.Context, input maildb.CreateDKIMKeyInput) (string, error) {
	f.lastCreateDKIMKey = input
	if f.createdDKIMKeyID != "" {
		return f.createdDKIMKeyID, nil
	}
	return "dkim-1", nil
}

func (f *fakeAdminService) DeactivateDKIMKey(_ context.Context, id string) error {
	f.lastDeactivateDKIMKeyID = id
	return nil
}

func (f *fakeAdminService) VerifyDKIMKeyDNS(_ context.Context, keyID string) (maildb.DKIMKeyDNSVerificationResult, error) {
	f.lastVerifyDKIMKeyID = keyID
	return maildb.DKIMKeyDNSVerificationResult{KeyID: keyID, Selector: "default"}, nil
}

func (f *fakeAdminService) RetryOutbox(_ context.Context, id string) error {
	f.lastRetryOutboxID = id
	return nil
}

func (f *fakeAdminService) DeleteSuppressionEntry(_ context.Context, id string) error {
	f.lastDeleteSuppressionID = id
	return nil
}

func (f *fakeAdminService) ListQuotaAlertThresholds(_ context.Context, req maildb.QuotaAlertThresholdListRequest) ([]maildb.QuotaAlertThresholdView, error) {
	f.lastQuotaAlertThresholdList = req
	f.lastLimit = req.Limit
	return f.quotaAlertThresholds, nil
}

func (f *fakeAdminService) GetQuotaAlertThreshold(_ context.Context, id string) (maildb.QuotaAlertThresholdView, error) {
	f.lastQuotaAlertThresholdID = id
	return f.quotaAlertThreshold, nil
}

func (f *fakeAdminService) CreateQuotaAlertThreshold(_ context.Context, req maildb.CreateQuotaAlertThresholdRequest) (maildb.QuotaAlertThresholdView, error) {
	f.lastCreateQuotaAlertThreshold = req
	return f.quotaAlertThreshold, nil
}

func (f *fakeAdminService) UpdateQuotaAlertThreshold(_ context.Context, req maildb.UpdateQuotaAlertThresholdRequest) (maildb.QuotaAlertThresholdView, error) {
	f.lastUpdateQuotaAlertThreshold = req
	return f.quotaAlertThreshold, nil
}

func (f *fakeAdminService) DeleteQuotaAlertThreshold(_ context.Context, id string) error {
	f.lastDeleteQuotaAlertThresholdID = id
	return nil
}

func (f *fakeAdminService) ListQuotaAlerts(_ context.Context, req maildb.QuotaAlertListRequest) ([]maildb.QuotaAlertView, error) {
	f.lastQuotaAlertList = req
	f.lastLimit = req.Limit
	return f.quotaAlerts, nil
}

func (f *fakeAdminService) GetQuotaAlert(_ context.Context, id string) (maildb.QuotaAlertView, error) {
	f.lastQuotaAlertID = id
	return f.quotaAlert, nil
}

func (f *fakeAdminService) GetCompanyConfig(_ context.Context, companyID, key string) (configstore.ConfigEntry, error) {
	f.lastCompanyConfigID = companyID
	f.lastCompanyConfigKey = key
	if len(f.companyConfig) == 0 {
		return configstore.ConfigEntry{}, configstore.ErrConfigNotFound
	}
	return f.companyConfig[0], nil
}

func (f *fakeAdminService) SetCompanyConfig(_ context.Context, companyID, key string, value json.RawMessage, locked bool, expectedVersion int64) (configstore.ConfigEntry, error) {
	f.lastCompanyConfigID = companyID
	f.lastCompanyConfigKey = key
	return configstore.ConfigEntry{
		ScopeID: companyID,
		Key:     key,
		Value:   value,
		Locked:  locked,
		Version: 1,
	}, nil
}

func (f *fakeAdminService) DeleteCompanyConfig(_ context.Context, companyID, key string, expectedVersion int64) error {
	f.lastCompanyConfigID = companyID
	f.lastCompanyConfigKey = key
	return nil
}

func (f *fakeAdminService) ListCompanyConfig(_ context.Context, companyID string) ([]configstore.ConfigEntry, error) {
	f.lastCompanyConfigID = companyID
	return f.companyConfig, nil
}

func (f *fakeAdminService) GetDomainConfig(_ context.Context, domainID, key string) (configstore.ConfigEntry, error) {
	f.lastDomainConfigID = domainID
	f.lastDomainConfigKey = key
	if len(f.domainConfig) == 0 {
		return configstore.ConfigEntry{}, configstore.ErrConfigNotFound
	}
	return f.domainConfig[0], nil
}

func (f *fakeAdminService) SetDomainConfig(_ context.Context, domainID, key string, value json.RawMessage, locked bool, expectedVersion int64) (configstore.ConfigEntry, error) {
	f.lastDomainConfigID = domainID
	f.lastDomainConfigKey = key
	return configstore.ConfigEntry{
		ScopeID: domainID,
		Key:     key,
		Value:   value,
		Locked:  locked,
		Version: 1,
	}, nil
}

func (f *fakeAdminService) DeleteDomainConfig(_ context.Context, domainID, key string, expectedVersion int64) error {
	f.lastDomainConfigID = domainID
	f.lastDomainConfigKey = key
	return nil
}

func (f *fakeAdminService) ListDomainConfig(_ context.Context, domainID string) ([]configstore.ConfigEntry, error) {
	f.lastDomainConfigID = domainID
	return f.domainConfig, nil
}

func (f *fakeAdminService) GetUserConfig(_ context.Context, userID, key string) (configstore.ConfigEntry, error) {
	f.lastUserConfigID = userID
	f.lastUserConfigKey = key
	if len(f.userConfig) == 0 {
		return configstore.ConfigEntry{}, configstore.ErrConfigNotFound
	}
	return f.userConfig[0], nil
}

func (f *fakeAdminService) SetUserConfig(_ context.Context, userID, key string, value json.RawMessage, locked bool, expectedVersion int64) (configstore.ConfigEntry, error) {
	f.lastUserConfigID = userID
	f.lastUserConfigKey = key
	return configstore.ConfigEntry{
		ScopeID: userID,
		Key:     key,
		Value:   value,
		Locked:  locked,
		Version: 1,
	}, nil
}

func (f *fakeAdminService) DeleteUserConfig(_ context.Context, userID, key string, expectedVersion int64) error {
	f.lastUserConfigID = userID
	f.lastUserConfigKey = key
	return nil
}

func (f *fakeAdminService) ListUserConfig(_ context.Context, userID string) ([]configstore.ConfigEntry, error) {
	f.lastUserConfigID = userID
	return f.userConfig, nil
}

func (f *fakeAdminService) PropagateCompanyConfig(_ context.Context, companyID string, scope configstore.PropagateScope, key string, value json.RawMessage, locked bool) error {
	f.lastPropagateCompanyID = companyID
	f.lastPropagateScope = scope
	return nil
}

func (f *fakeAdminService) GetDomainSettings(_ context.Context, domainID string) (*admin.DomainSettings, error) {
	f.lastDomainID = domainID
	return &admin.DomainSettings{
		DomainID:                    domainID,
		TLSPolicy:                   "opportunistic",
		QuotaPerUser:                10737418240,
		IPWhitelistEnabled:          false,
		IPWhitelist:                 []string{},
		Require2FA:                  false,
		SessionTimeoutMinutes:       480,
		PasswordMinLength:           8,
		PasswordRequireUppercase:    true,
		PasswordRequireNumbers:      true,
		PasswordRequireSpecialChars: false,
		PasswordExpiryDays:          0,
		UpdatedAt:                   time.Now(),
		UpdatedBy:                   "admin-1",
	}, nil
}

func (f *fakeAdminService) UpdateDomainSettings(_ context.Context, settings *admin.DomainSettings) error {
	f.lastDomainID = settings.DomainID
	f.lastDomainSettings = *settings
	return nil
}

func (f *fakeAdminService) GetAPISettings(_ context.Context, domainID string) (*admin.APISettings, error) {
	f.lastDomainID = domainID
	return &admin.APISettings{
		DomainID:             domainID,
		RateLimitRPS:         100,
		RateLimitBPS:         0,
		CIDRAllowlistEnabled: false,
		CIDRAllowlist:        []string{},
		RequireAPIKey:        true,
		UpdatedAt:            time.Now(),
		UpdatedBy:            "admin-1",
	}, nil
}

func (f *fakeAdminService) UpdateAPISettings(_ context.Context, settings *admin.APISettings) error {
	f.lastDomainID = settings.DomainID
	return nil
}

func (f *fakeAdminService) CreateAPIKey(_ context.Context, key *admin.APIKey) (secret string, err error) {
	f.lastDomainID = key.DomainID
	key.ID = "key-" + key.DomainID
	return "test-secret-" + key.ID, nil
}

func (f *fakeAdminService) ListAPIKeys(_ context.Context, domainID string) ([]admin.APIKey, error) {
	f.lastDomainID = domainID
	return []admin.APIKey{
		{
			ID:        "key-1",
			DomainID:  domainID,
			Name:      "test-key",
			CreatedBy: "admin-1",
			CreatedAt: time.Now(),
			IsActive:  true,
		},
	}, nil
}

func (f *fakeAdminService) DeleteAPIKey(_ context.Context, keyID string) error {
	return nil
}

func (f *fakeAdminService) RotateAPIKey(_ context.Context, keyID string) (newSecret string, err error) {
	return "new-secret-" + keyID, nil
}

func (f *fakeAdminService) CreateAlertRule(_ context.Context, rule *admin.AlertRule) error {
	f.lastCreateAlertRule = rule
	rule.ID = "rule-123"
	return nil
}

func (f *fakeAdminService) GetAlertRule(_ context.Context, ruleID string) (*admin.AlertRule, error) {
	f.lastGetAlertRuleID = ruleID
	for _, rule := range f.alertRules {
		if rule.ID == ruleID {
			return &rule, nil
		}
	}
	return nil, fmt.Errorf("rule not found")
}

func (f *fakeAdminService) ListAlertRules(_ context.Context, companyID string) ([]admin.AlertRule, error) {
	f.lastListAlertRulesCompanyID = companyID
	var rules []admin.AlertRule
	for _, rule := range f.alertRules {
		if rule.CompanyID == companyID {
			rules = append(rules, rule)
		}
	}
	return rules, nil
}

func (f *fakeAdminService) UpdateAlertRule(_ context.Context, rule *admin.AlertRule) error {
	f.lastUpdateAlertRule = rule
	return nil
}

func (f *fakeAdminService) DeleteAlertRule(_ context.Context, ruleID string) error {
	f.lastDeleteAlertRuleID = ruleID
	return nil
}

func (f *fakeAdminService) CreateAlertChannel(_ context.Context, channel *admin.AlertChannel) error {
	f.lastCreateAlertChannel = channel
	channel.ID = "channel-123"
	return nil
}

func (f *fakeAdminService) GetAlertChannel(_ context.Context, channelID string) (*admin.AlertChannel, error) {
	f.lastGetAlertChannelID = channelID
	for _, ch := range f.alertChannels {
		if ch.ID == channelID {
			return &ch, nil
		}
	}
	return nil, fmt.Errorf("channel not found")
}

func (f *fakeAdminService) ListAlertChannels(_ context.Context, companyID string) ([]admin.AlertChannel, error) {
	f.lastListAlertChannelsCompanyID = companyID
	var channels []admin.AlertChannel
	for _, ch := range f.alertChannels {
		if ch.CompanyID == companyID {
			channels = append(channels, ch)
		}
	}
	return channels, nil
}

func (f *fakeAdminService) UpdateAlertChannel(_ context.Context, channel *admin.AlertChannel) error {
	f.lastUpdateAlertChannel = channel
	return nil
}

func (f *fakeAdminService) DeleteAlertChannel(_ context.Context, channelID string) error {
	f.lastDeleteAlertChannelID = channelID
	return nil
}

func (f *fakeAdminService) ListAlertEvents(_ context.Context, filter admin.AlertEventFilter) ([]admin.AlertEvent, bool, error) {
	f.lastListAlertEventsFilter = filter
	var events []admin.AlertEvent
	for _, event := range f.alertEvents {
		if event.CompanyID == filter.CompanyID {
			events = append(events, event)
		}
	}
	hasMore := filter.Limit > 0 && len(events) > filter.Limit
	if hasMore {
		events = events[:filter.Limit]
	}
	return events, hasMore, nil
}

func (f *fakeAdminService) LogAlertEvent(_ context.Context, event *admin.AlertEvent) error {
	f.lastLogAlertEvent = event
	event.ID = "event-123"
	return nil
}

func TestAdminUserConfigWriteBlocked(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "secret")

	req := httptest.NewRequest(http.MethodPut, "/admin/v1/users/user-1/config/some.key", bytes.NewReader([]byte(`{"value":"test"}`)))
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("PUT status = %d, want 403", rec.Code)
	}

	req = httptest.NewRequest(http.MethodDelete, "/admin/v1/users/user-1/config/some.key", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("DELETE status = %d, want 403", rec.Code)
	}
}

func (f *fakeAdminService) GetUserMFAStatus(ctx context.Context, userID string) (maildb.UserMFAStatus, error) {
	return maildb.UserMFAStatus{}, nil
}

func (f *fakeAdminService) ResetUserMFA(ctx context.Context, userID string) error {
	return nil
}

func (f *fakeAdminService) GetMFAStats(ctx context.Context, companyID string) (maildb.MFAStats, error) {
	return maildb.MFAStats{}, nil
}

func (f *fakeAdminService) ListLoginAttempts(ctx context.Context, filter admin.LoginAuditFilter) ([]admin.LoginAuditLog, error) {
	return []admin.LoginAuditLog{}, nil
}

func (f *fakeAdminService) CreateInviteToken(ctx context.Context, userID, createdBy string) (maildb.InviteToken, error) {
	return maildb.InviteToken{}, nil
}

func (f *fakeAdminService) GetInviteToken(ctx context.Context, token string) (maildb.InviteToken, error) {
	return maildb.InviteToken{}, nil
}

func (f *fakeAdminService) AcceptInviteToken(ctx context.Context, token, passwordHash string) (maildb.UserView, error) {
	return maildb.UserView{}, nil
}

func (f *fakeAdminService) ListAdminUsers(_ context.Context, req maildb.AdminUserListRequest) ([]maildb.AdminUserView, bool, error) {
	return []maildb.AdminUserView{}, false, nil
}

func (f *fakeAdminService) SetUserRole(_ context.Context, userID, role string) error {
	return nil
}

func (f *fakeAdminService) ClearUserAdminRole(_ context.Context, userID string) error {
	return nil
}

func (f *fakeAdminService) TriggerLDAPSync(ctx context.Context, domainID, syncType string) (map[string]interface{}, error) {
	if f.ldapSyncErr != nil {
		return nil, f.ldapSyncErr
	}
	return map[string]interface{}{
		"sync_run_id": "test-sync-run-id",
		"status":      "running",
	}, nil
}

func (f *fakeAdminService) GetLDAPSyncRuns(ctx context.Context, req maildb.LDAPSyncRunListRequest) ([]maildb.LDAPSyncRunView, error) {
	f.lastLDAPSyncRunsReq = req
	return f.ldapSyncRuns, nil
}

func (f *fakeAdminService) GetLDAPSyncRun(ctx context.Context, runID string) (*maildb.LDAPSyncRunView, error) {
	return nil, nil
}

func (f *fakeAdminService) GetLDAPSyncConflicts(ctx context.Context, req maildb.LDAPSyncConflictListRequest) ([]maildb.LDAPSyncConflictView, error) {
	f.lastLDAPSyncConflictsReq = req
	return f.ldapSyncConflicts, nil
}

func (f *fakeAdminService) GetLDAPSyncConflict(ctx context.Context, conflictID string) (*maildb.LDAPSyncConflictView, error) {
	return nil, nil
}

func (f *fakeAdminService) ResolveLDAPSyncConflict(ctx context.Context, conflictID, resolution string) error {
	return nil
}

func (f *fakeAdminService) TriggerRDBMSSync(ctx context.Context, domainID, syncType string) (map[string]interface{}, error) {
	if f.rdbmsSyncErr != nil {
		return nil, f.rdbmsSyncErr
	}
	return map[string]interface{}{
		"sync_run_id": "test-sync-run-id",
		"status":      "running",
	}, nil
}

func (f *fakeAdminService) GetRDBMSSyncRuns(ctx context.Context, req maildb.RDBMSSyncRunListRequest) ([]maildb.RDBMSSyncRunView, error) {
	f.lastRDBMSSyncRunsReq = req
	return f.rdbmsSyncRuns, nil
}

func (f *fakeAdminService) GetRDBMSSyncRun(ctx context.Context, runID string) (*maildb.RDBMSSyncRunView, error) {
	return nil, nil
}

func (f *fakeAdminService) GetRDBMSSyncConflicts(ctx context.Context, req maildb.RDBMSSyncConflictListRequest) ([]maildb.RDBMSSyncConflictView, error) {
	f.lastRDBMSSyncConflictsReq = req
	return f.rdbmsSyncConflicts, nil
}

func (f *fakeAdminService) GetRDBMSSyncConflict(ctx context.Context, conflictID string) (*maildb.RDBMSSyncConflictView, error) {
	return nil, nil
}

func (f *fakeAdminService) ResolveRDBMSSyncConflict(ctx context.Context, conflictID, resolution string) error {
	return nil
}

func (f *fakeAdminService) GetDomainIdPConfig(_ context.Context, domainID string) (*idprovider.Config, error) {
	return &idprovider.Config{DomainID: domainID, ProviderType: "database", Settings: map[string]interface{}{}}, nil
}

func (f *fakeAdminService) SetDomainIdPConfig(_ context.Context, cfg *idprovider.Config) error {
	return nil
}

func (f *fakeAdminService) DeleteDomainIdPConfig(_ context.Context, domainID string) error {
	return nil
}
