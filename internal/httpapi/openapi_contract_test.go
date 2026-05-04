package httpapi

import (
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/gogomail/gogomail/internal/maildb"
	"github.com/gogomail/gogomail/internal/mailservice"
)

func TestOpenAPIDraftUsesBackendContractVersion(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("../../docs/openapi.yaml")
	if err != nil {
		t.Fatalf("read OpenAPI draft: %v", err)
	}
	if !strings.Contains(string(raw), "version: "+BackendContractVersion) {
		t.Fatalf("OpenAPI draft does not contain backend contract version %q", BackendContractVersion)
	}
}

func TestOpenAPIDraftCoversRegisteredHTTPRoutes(t *testing.T) {
	t.Parallel()

	registered := extractRegisteredRoutes(t, "mail.go", "admin.go", "health.go")
	documented := extractOpenAPIRoutes(t, "../../docs/openapi.yaml")

	var missing []string
	for _, route := range registered {
		if !documented[route] {
			missing = append(missing, route)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Fatalf("OpenAPI draft is missing registered routes:\n%s", strings.Join(missing, "\n"))
	}
}

func TestOpenAPIDraftDocumentsRequestBodies(t *testing.T) {
	t.Parallel()

	operations := extractOpenAPIOperationBlocks(t, "../../docs/openapi.yaml")
	for _, route := range []string{
		"POST /folders",
		"PATCH /folders/{id}",
		"PATCH /messages/{id}/flags",
		"PATCH /messages/{id}/folder",
		"PATCH /messages/bulk/flags",
		"PATCH /messages/bulk/folder",
		"POST /messages/bulk/delete",
		"POST /messages/send",
		"POST /drafts",
		"PATCH /drafts/{id}",
		"POST /attachments",
		"POST /attachments/upload",
		"POST /attachments/upload-sessions",
		"PUT /attachments/upload-sessions/{id}/body",
		"POST /push-devices",
		"PATCH /companies/{id}/quota",
		"POST /domains",
		"PATCH /domains/{id}/status",
		"PATCH /domains/{id}/quota",
		"PATCH /domains/{id}/policy",
		"POST /users",
		"PATCH /users/{id}/status",
		"PATCH /users/{id}/quota",
		"POST /trusted-relays",
		"POST /delivery-routes",
		"PATCH /delivery-routes/{id}/status",
		"PATCH /push-notification-attempts/{id}/outcome",
		"PATCH /backpressure",
		"POST /attachment-cleanup/candidates",
		"POST /attachment-cleanup/runs",
		"POST /api-usage/export-batches/{id}/artifacts",
		"POST /api-usage/export-batches/{id}/artifacts/write",
		"POST /dkim-keys",
		"POST /quota-reconciliation/corrections",
	} {
		block, ok := operations[route]
		if !ok {
			t.Fatalf("OpenAPI operation %s is missing", route)
		}
		if !strings.Contains(block, "requestBody:") {
			t.Fatalf("OpenAPI operation %s must document its requestBody", route)
		}
	}
}

func TestOpenAPIDraftDocumentsSupportedMessageFlags(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("../../docs/openapi.yaml")
	if err != nil {
		t.Fatalf("read OpenAPI draft: %v", err)
	}
	draft := string(raw)
	for _, flag := range []string{"read", "starred", "answered", "forwarded"} {
		err := maildb.ValidateBulkMessageFlagRequest(maildb.BulkMessageFlagRequest{
			UserID:     "user-1",
			MessageIDs: []string{"11111111-1111-1111-1111-111111111111"},
			Flag:       flag,
			Value:      true,
		})
		if err != nil {
			t.Fatalf("test fixture flag %q is not accepted by the HTTP API", flag)
		}
		if !strings.Contains(draft, flag) {
			t.Fatalf("OpenAPI draft does not document supported message flag %q", flag)
		}
	}
}

func TestOpenAPIDraftDocumentsAttachmentStatuses(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("../../docs/openapi.yaml")
	if err != nil {
		t.Fatalf("read OpenAPI draft: %v", err)
	}
	block := extractOpenAPIComponentBlock(t, string(raw), "schemas", "Attachment")
	for _, status := range []string{"uploading", "stored", "deleted"} {
		if !strings.Contains(block, status) {
			t.Fatalf("Attachment schema must document status %q", status)
		}
	}
	if strings.Contains(block, "active") {
		t.Fatal("Attachment schema must not document obsolete status active")
	}
}

func TestOpenAPIDraftDocumentsAttachmentUploadLimits(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("../../docs/openapi.yaml")
	if err != nil {
		t.Fatalf("read OpenAPI draft: %v", err)
	}
	block := extractOpenAPIComponentBlock(t, string(raw), "schemas", "AttachmentUploadCapabilities")
	for _, want := range []string{
		"maximum: " + strconv.FormatInt(mailservice.MaxAttachmentUploadBytes, 10),
		"maximum: " + strconv.Itoa(mailservice.MaxAttachmentFilenameBytes),
		"maximum: " + strconv.FormatInt(int64(mailservice.MaxAttachmentUploadSessionTTL.Seconds()), 10),
		"max_session_ttl_seconds",
		"upload_sessions",
		"cancel_upload_sessions",
		"upload_session_body",
		"upload_session_checksum",
		"finalize_upload_sessions",
		"resumable_chunked_uploads",
	} {
		if !strings.Contains(block, want) {
			t.Fatalf("AttachmentUploadCapabilities schema must document %q", want)
		}
	}
}

func TestOpenAPIDraftDocumentsStableResponseEnvelopes(t *testing.T) {
	t.Parallel()

	operations := extractOpenAPIOperationBlocks(t, "../../docs/openapi.yaml")
	for route, responseRef := range map[string]string{
		"GET /folders":                                               "#/components/responses/FolderList",
		"POST /folders":                                              "#/components/responses/Folder",
		"PATCH /folders/{id}":                                        "#/components/responses/Folder",
		"DELETE /folders/{id}":                                       "#/components/responses/Status",
		"GET /messages":                                              "#/components/responses/MessageListPage",
		"GET /search":                                                "#/components/responses/MessageList",
		"GET /messages/{id}":                                         "#/components/responses/Message",
		"GET /messages/{id}/delivery-status":                         "#/components/responses/MessageDeliveryStatus",
		"GET /threads":                                               "#/components/responses/ThreadList",
		"GET /threads/{id}/messages":                                 "#/components/responses/MessageList",
		"PATCH /messages/{id}/flags":                                 "#/components/responses/Status",
		"PATCH /messages/{id}/folder":                                "#/components/responses/Status",
		"PATCH /messages/bulk/flags":                                 "#/components/responses/BulkUpdate",
		"PATCH /messages/bulk/folder":                                "#/components/responses/BulkUpdate",
		"POST /messages/bulk/delete":                                 "#/components/responses/BulkUpdate",
		"DELETE /messages/{id}":                                      "#/components/responses/Status",
		"POST /messages/send":                                        "#/components/responses/SendQueued",
		"POST /drafts":                                               "#/components/responses/Draft",
		"PATCH /drafts/{id}":                                         "#/components/responses/Draft",
		"DELETE /drafts/{id}":                                        "#/components/responses/Status",
		"POST /drafts/{id}/send":                                     "#/components/responses/SendQueued",
		"POST /attachments":                                          "#/components/responses/Attachment",
		"POST /attachments/upload":                                   "#/components/responses/Attachment",
		"POST /attachments/upload-sessions":                          "#/components/responses/AttachmentUploadSession",
		"GET /attachments/capabilities":                              "#/components/responses/AttachmentUploadCapabilities",
		"DELETE /attachments/{id}":                                   "#/components/responses/Attachment",
		"GET /attachments/upload-sessions/{id}":                      "#/components/responses/AttachmentUploadSession",
		"DELETE /attachments/upload-sessions/{id}":                   "#/components/responses/AttachmentUploadSession",
		"PUT /attachments/upload-sessions/{id}/body":                 "#/components/responses/AttachmentUploadSession",
		"POST /attachments/upload-sessions/{id}/finalize":            "#/components/responses/Attachment",
		"GET /messages/{id}/attachments":                             "#/components/responses/AttachmentList",
		"GET /push-devices":                                          "#/components/responses/PushDeviceList",
		"POST /push-devices":                                         "#/components/responses/PushDevice",
		"DELETE /push-devices/{id}":                                  "#/components/responses/IDStatus",
		"POST /imap/mailboxes/{id}/uid-backfill":                     "#/components/responses/IMAPUIDBackfill",
		"GET /companies":                                             "#/components/responses/CompanyList",
		"GET /companies/{id}":                                        "#/components/responses/Company",
		"GET /domains":                                               "#/components/responses/DomainList",
		"GET /domains/{id}":                                          "#/components/responses/Domain",
		"GET /domains/{id}/stats":                                    "#/components/responses/DomainStats",
		"GET /domains/{id}/dns-check":                                "#/components/responses/DomainDNSCheck",
		"GET /domains/{id}/dns-checks":                               "#/components/responses/DomainDNSCheckHistory",
		"POST /domains":                                              "#/components/responses/Domain",
		"PATCH /domains/{id}/status":                                 "#/components/responses/IDStatus",
		"PATCH /domains/{id}/quota":                                  "#/components/responses/IDStatus",
		"PATCH /domains/{id}/policy":                                 "#/components/responses/DomainPolicy",
		"GET /users":                                                 "#/components/responses/UserList",
		"GET /users/{id}":                                            "#/components/responses/User",
		"POST /users":                                                "#/components/responses/User",
		"PATCH /users/{id}/status":                                   "#/components/responses/IDStatus",
		"PATCH /users/{id}/quota":                                    "#/components/responses/IDStatus",
		"GET /queue":                                                 "#/components/responses/QueueStats",
		"GET /outbox-events":                                         "#/components/responses/OutboxEventList",
		"GET /outbox-events/{id}":                                    "#/components/responses/OutboxEvent",
		"GET /backpressure":                                          "#/components/responses/Backpressure",
		"PATCH /backpressure":                                        "#/components/responses/Backpressure",
		"GET /quota-usage":                                           "#/components/responses/QuotaUsageList",
		"GET /api-usage/daily":                                       "#/components/responses/APIUsageDailyList",
		"GET /api-usage/monthly":                                     "#/components/responses/APIUsageMonthlyList",
		"GET /api-usage/ledger":                                      "#/components/responses/APIUsageLedgerList",
		"GET /api-usage/ledger/export":                               "#/components/responses/APIUsageLedgerExport",
		"GET /api-usage/ledger/stats":                                "#/components/responses/APIUsageLedgerStats",
		"GET /api-usage/ledger/retention-readiness":                  "#/components/responses/APIUsageLedgerRetentionReadiness",
		"GET /api-usage/export-capabilities":                         "#/components/responses/APIUsageExportCapabilities",
		"GET /api-usage/export-batches":                              "#/components/responses/APIUsageExportBatchList",
		"POST /api-usage/export-batches":                             "#/components/responses/APIUsageExportBatch",
		"GET /api-usage/export-batches/{id}":                         "#/components/responses/APIUsageExportBatch",
		"GET /api-usage/export-batches/{id}/handoff-readiness":       "#/components/responses/APIUsageExportHandoffReadiness",
		"GET /api-usage/export-batches/{id}/export":                  "#/components/responses/APIUsageLedgerExport",
		"GET /api-usage/export-batches/{id}/artifacts":               "#/components/responses/APIUsageExportArtifactList",
		"POST /api-usage/export-batches/{id}/artifacts":              "#/components/responses/APIUsageExportArtifact",
		"GET /api-usage/export-batches/{id}/artifacts/{artifact_id}": "#/components/responses/APIUsageExportArtifact",
		"POST /api-usage/export-batches/{id}/artifacts/write":        "#/components/responses/APIUsageExportArtifact",
		"GET /api-usage/export-batches/{id}/artifacts/{artifact_id}/download":                                    "#/components/responses/APIUsageExportArtifactDownload",
		"GET /api-usage/export-batches/{id}/artifacts/{artifact_id}/verification":                                "#/components/responses/APIUsageExportArtifactVerification",
		"GET /api-usage/export-batches/{id}/manifest-digests":                                                    "#/components/responses/APIUsageExportManifestDigestList",
		"POST /api-usage/export-batches/{id}/manifest-digests":                                                   "#/components/responses/APIUsageExportManifestDigest",
		"GET /api-usage/export-batches/{id}/manifest-digests/{digest_id}":                                        "#/components/responses/APIUsageExportManifestDigest",
		"GET /api-usage/export-batches/{id}/manifest-digests/{digest_id}/verification":                           "#/components/responses/APIUsageExportManifestDigestVerification",
		"GET /api-usage/export-batches/{id}/manifest-digests/{digest_id}/signatures":                             "#/components/responses/APIUsageExportManifestSignatureList",
		"POST /api-usage/export-batches/{id}/manifest-digests/{digest_id}/signatures":                            "#/components/responses/APIUsageExportManifestSignature",
		"GET /api-usage/export-batches/{id}/manifest-digests/{digest_id}/signatures/{signature_id}":              "#/components/responses/APIUsageExportManifestSignature",
		"GET /api-usage/export-batches/{id}/manifest-digests/{digest_id}/signatures/{signature_id}/verification": "#/components/responses/APIUsageExportManifestSignatureVerification",
		"POST /attachment-cleanup/candidates":                                                                    "#/components/responses/AttachmentCleanupCandidates",
		"POST /attachment-cleanup/runs":                                                                          "#/components/responses/AttachmentCleanupRun",
		"GET /quota-reconciliation":                                                                              "#/components/responses/QuotaReconciliationList",
		"POST /quota-reconciliation/corrections":                                                                 "#/components/responses/QuotaCorrection",
		"GET /delivery-attempts":                                                                                 "#/components/responses/DeliveryAttempts",
		"GET /delivery-attempts/stats":                                                                           "#/components/responses/DeliveryAttemptStats",
		"GET /delivery-attempts/exhausted":                                                                       "#/components/responses/ExhaustedAttempts",
		"GET /push-notification-attempts":                                                                        "#/components/responses/PushNotificationAttempts",
		"GET /push-notification-attempts/{id}":                                                                   "#/components/responses/PushNotificationAttempt",
		"PATCH /push-notification-attempts/{id}/outcome":                                                         "#/components/responses/IDStatus",
		"GET /push-notification-stats":                                                                           "#/components/responses/PushNotificationStats",
		"GET /suppression-list":                                                                                  "#/components/responses/SuppressionList",
		"DELETE /suppression-list/{id}":                                                                          "#/components/responses/IDStatus",
		"GET /trusted-relays":                                                                                    "#/components/responses/TrustedRelayList",
		"POST /trusted-relays":                                                                                   "#/components/responses/TrustedRelay",
		"DELETE /trusted-relays/{id}":                                                                            "#/components/responses/IDStatus",
		"GET /delivery-routes":                                                                                   "#/components/responses/DeliveryRouteList",
		"POST /delivery-routes":                                                                                  "#/components/responses/DeliveryRoute",
		"GET /delivery-routes/resolve":                                                                           "#/components/responses/DeliveryRouteResolution",
		"GET /delivery-routes/counters":                                                                          "#/components/responses/DeliveryRouteCounters",
		"PATCH /delivery-routes/{id}/status":                                                                     "#/components/responses/IDStatus",
		"DELETE /delivery-routes/{id}":                                                                           "#/components/responses/IDStatus",
		"GET /dkim-keys":                                                                                         "#/components/responses/DKIMKeyList",
		"POST /dkim-keys":                                                                                        "#/components/responses/IDStatus",
		"DELETE /dkim-keys/{id}":                                                                                 "#/components/responses/IDStatus",
		"POST /dkim-keys/{id}/verify-dns":                                                                        "#/components/responses/DKIMKeyDNSVerification",
		"POST /outbox/{id}/retry":                                                                                "#/components/responses/IDStatus",
		"GET /messages/{id}/attachments/{attachment_id}/download":                                                "",
	} {
		block, ok := operations[route]
		if !ok {
			t.Fatalf("OpenAPI operation %s is missing", route)
		}
		if responseRef == "" {
			if strings.Contains(block, "application/json:") {
				t.Fatalf("OpenAPI operation %s downloads bytes and must not declare a JSON envelope", route)
			}
			continue
		}
		if !strings.Contains(block, `$ref: "`+responseRef+`"`) {
			t.Fatalf("OpenAPI operation %s must use response ref %s", route, responseRef)
		}
	}
}

func TestOpenAPIDraftDocumentsAttachmentUploadSizeErrors(t *testing.T) {
	t.Parallel()

	operations := extractOpenAPIOperationBlocks(t, "../../docs/openapi.yaml")
	for _, route := range []string{"POST /attachments", "POST /attachments/upload", "POST /attachments/upload-sessions"} {
		block, ok := operations[route]
		if !ok {
			t.Fatalf("OpenAPI operation %s is missing", route)
		}
		if !strings.Contains(block, `"413":`) {
			t.Fatalf("OpenAPI operation %s must document HTTP 413 for attachment size caps", route)
		}
		if !strings.Contains(block, `$ref: "#/components/responses/Error"`) {
			t.Fatalf("OpenAPI operation %s must map HTTP 413 to the reusable Error response", route)
		}
	}
}

func TestOpenAPIDraftDocumentsUploadSessionChecksumHeader(t *testing.T) {
	t.Parallel()

	operations := extractOpenAPIOperationBlocks(t, "../../docs/openapi.yaml")
	block, ok := operations["PUT /attachments/upload-sessions/{id}/body"]
	if !ok {
		t.Fatal("OpenAPI operation PUT /attachments/upload-sessions/{id}/body is missing")
	}
	for _, want := range []string{"X-Content-SHA256", "in: header", "^[0-9a-f]{64}$"} {
		if !strings.Contains(block, want) {
			t.Fatalf("upload session body operation must document checksum header detail %q", want)
		}
	}
}

func TestOpenAPIDraftDocumentsCleanupSessionCounts(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("../../docs/openapi.yaml")
	if err != nil {
		t.Fatalf("read OpenAPI draft: %v", err)
	}
	draft := string(raw)
	for _, want := range []string{"AttachmentCleanupSessionCandidate", "session_candidates", "session_candidate_count", "session_limited_count", "expired_session_count"} {
		if !strings.Contains(draft, want) {
			t.Fatalf("OpenAPI cleanup run schema must document %q", want)
		}
	}
}

func TestOpenAPIDraftDocumentsNonJSONDownloadResponses(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("../../docs/openapi.yaml")
	if err != nil {
		t.Fatalf("read OpenAPI draft: %v", err)
	}
	draft := string(raw)
	for response, mediaType := range map[string]string{
		"APIUsageLedgerExport":           "application/x-ndjson",
		"APIUsageExportArtifactDownload": "application/x-ndjson",
	} {
		block := extractOpenAPIComponentBlock(t, draft, "responses", response)
		if strings.Contains(block, "application/json:") {
			t.Fatalf("OpenAPI response %s streams bytes and must not declare application/json", response)
		}
		if !strings.Contains(block, mediaType+":") {
			t.Fatalf("OpenAPI response %s must document media type %s", response, mediaType)
		}
		if !strings.Contains(block, "type: string") {
			t.Fatalf("OpenAPI response %s must document a string stream schema", response)
		}
		if !strings.Contains(block, "Cache-Control:") || !strings.Contains(block, "enum: [no-store]") {
			t.Fatalf("OpenAPI response %s must document Cache-Control: no-store", response)
		}
		if !strings.Contains(block, "X-Content-Type-Options:") || !strings.Contains(block, "enum: [nosniff]") {
			t.Fatalf("OpenAPI response %s must document X-Content-Type-Options: nosniff", response)
		}
	}

	operations := extractOpenAPIOperationBlocks(t, "../../docs/openapi.yaml")
	block, ok := operations["GET /messages/{id}/attachments/{attachment_id}/download"]
	if !ok {
		t.Fatal("OpenAPI operation GET /messages/{id}/attachments/{attachment_id}/download is missing")
	}
	if strings.Contains(block, "application/json:") {
		t.Fatal("attachment download must not declare application/json")
	}
	for _, want := range []string{"application/octet-stream:", "type: string", "format: binary", "Content-Disposition:", "Cache-Control:", "enum: [no-store]", "X-Content-Type-Options:", "enum: [nosniff]"} {
		if !strings.Contains(block, want) {
			t.Fatalf("attachment download must document %q", want)
		}
	}
}

func TestOpenAPIDraftDocumentsAPIUsageLedgerFilters(t *testing.T) {
	t.Parallel()

	operations := extractOpenAPIOperationBlocks(t, "../../docs/openapi.yaml")
	block, ok := operations["GET /api-usage/ledger"]
	if !ok {
		t.Fatal("OpenAPI operation GET /api-usage/ledger is missing")
	}
	for _, param := range []string{"tenant_id", "principal_id", "from", "to"} {
		if !strings.Contains(block, "name: "+param) {
			t.Fatalf("GET /api-usage/ledger must document query parameter %q", param)
		}
	}
}

func TestOpenAPIDraftDocumentsOperationalTriageFilters(t *testing.T) {
	t.Parallel()

	operations := extractOpenAPIOperationBlocks(t, "../../docs/openapi.yaml")
	for route, params := range map[string][]string{
		"GET /search":                                          {"limit", "q", "folder_id", "from", "subject", "has_attachment", "sort", "include_rank", "include_highlights"},
		"GET /companies":                                       {"limit"},
		"GET /domains":                                         {"limit"},
		"GET /domains/{id}/dns-checks":                         {"id", "limit"},
		"GET /users":                                           {"limit", "domain_id"},
		"GET /quota-usage":                                     {"limit"},
		"GET /delivery-attempts":                               {"limit", "status", "recipient_domain", "since"},
		"GET /delivery-attempts/stats":                         {"status", "recipient_domain", "since"},
		"GET /delivery-attempts/exhausted":                     {"limit", "recipient_domain", "since"},
		"GET /push-notification-attempts":                      {"limit", "message_id", "status", "user_id", "platform", "device_id", "provider_status", "provider_message_id", "since"},
		"GET /push-notification-stats":                         {"message_id", "user_id", "platform", "device_id", "since"},
		"GET /push-devices":                                    {"limit"},
		"GET /outbox-events":                                   {"limit", "topic", "partition_key", "status", "since"},
		"GET /api-usage/ledger/retention-readiness":            {"cutoff", "tenant_id", "principal_id"},
		"POST /api-usage/export-batches":                       {"tenant_id", "principal_id", "from", "to"},
		"GET /api-usage/export-batches/{id}/handoff-readiness": {"id", "deep"},
		"GET /suppression-list":                                {"limit"},
		"GET /trusted-relays":                                  {"limit"},
		"GET /delivery-routes":                                 {"limit"},
		"GET /delivery-routes/resolve":                         {"domain"},
		"POST /imap/mailboxes/{id}/uid-backfill":               {"id", "user_id", "limit"},
		"GET /dkim-keys":                                       {"limit", "domain_id"},
	} {
		block, ok := operations[route]
		if !ok {
			t.Fatalf("OpenAPI operation %s is missing", route)
		}
		for _, param := range params {
			if !openAPIOperationDocumentsParameter(block, param) {
				t.Fatalf("OpenAPI operation %s must document parameter %q", route, param)
			}
		}
	}
}

func TestOpenAPIDraftDocumentsRetentionCutoffGuardrail(t *testing.T) {
	t.Parallel()

	operations := extractOpenAPIOperationBlocks(t, "../../docs/openapi.yaml")
	block, ok := operations["GET /api-usage/ledger/retention-readiness"]
	if !ok {
		t.Fatal("OpenAPI operation GET /api-usage/ledger/retention-readiness is missing")
	}
	if !strings.Contains(block, "future cutoffs are rejected") {
		t.Fatalf("retention-readiness cutoff parameter must document future-cutoff rejection, got:\n%s", block)
	}
}

func TestOpenAPIDraftDocumentsAPIUsageExportBatchRequiredWindow(t *testing.T) {
	t.Parallel()

	operations := extractOpenAPIOperationBlocks(t, "../../docs/openapi.yaml")
	block, ok := operations["POST /api-usage/export-batches"]
	if !ok {
		t.Fatal("OpenAPI operation POST /api-usage/export-batches is missing")
	}
	for _, param := range []string{"from", "to"} {
		if !openAPIOperationDocumentsRequiredQueryParameter(block, param) {
			t.Fatalf("POST /api-usage/export-batches must document required query parameter %q", param)
		}
	}
}

func TestOpenAPIDraftKeepsThreadListParametersScoped(t *testing.T) {
	t.Parallel()

	operations := extractOpenAPIOperationBlocks(t, "../../docs/openapi.yaml")
	block, ok := operations["GET /threads"]
	if !ok {
		t.Fatal("OpenAPI operation GET /threads is missing")
	}
	if !strings.Contains(block, "#/components/parameters/Limit") {
		t.Fatalf("GET /threads must document the limit parameter, got:\n%s", block)
	}
	for _, param := range []string{"tenant_id", "principal_id", "from", "to"} {
		if openAPIOperationDocumentsParameter(block, param) {
			t.Fatalf("GET /threads must not document API usage filter parameter %q", param)
		}
	}
}

func TestOpenAPIDraftDocumentsPathParameters(t *testing.T) {
	t.Parallel()

	operations := extractOpenAPIOperationBlocks(t, "../../docs/openapi.yaml")
	var missing []string
	for route, block := range operations {
		for _, param := range extractOpenAPIPathParameters(route) {
			if !openAPIOperationDocumentsParameter(block, param) {
				missing = append(missing, route+" -> "+param)
			}
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Fatalf("OpenAPI operations missing path parameters:\n%s", strings.Join(missing, "\n"))
	}
}

func TestOpenAPIDraftUsesReusableNestedPathParameters(t *testing.T) {
	t.Parallel()

	operations := extractOpenAPIOperationBlocks(t, "../../docs/openapi.yaml")
	for route, refs := range map[string][]string{
		"GET /messages/{id}/attachments/{attachment_id}/download":                                                {"#/components/parameters/PathID", "#/components/parameters/AttachmentID"},
		"GET /api-usage/export-batches/{id}/manifest-digests/{digest_id}":                                        {"#/components/parameters/DigestID"},
		"GET /api-usage/export-batches/{id}/manifest-digests/{digest_id}/verification":                           {"#/components/parameters/DigestID"},
		"GET /api-usage/export-batches/{id}/manifest-digests/{digest_id}/signatures":                             {"#/components/parameters/DigestID"},
		"POST /api-usage/export-batches/{id}/manifest-digests/{digest_id}/signatures":                            {"#/components/parameters/DigestID"},
		"GET /api-usage/export-batches/{id}/manifest-digests/{digest_id}/signatures/{signature_id}":              {"#/components/parameters/DigestID", "#/components/parameters/SignatureID"},
		"GET /api-usage/export-batches/{id}/manifest-digests/{digest_id}/signatures/{signature_id}/verification": {"#/components/parameters/DigestID", "#/components/parameters/SignatureID"},
	} {
		block, ok := operations[route]
		if !ok {
			t.Fatalf("OpenAPI operation %s is missing", route)
		}
		for _, ref := range refs {
			if !strings.Contains(block, `$ref: "`+ref+`"`) {
				t.Fatalf("OpenAPI operation %s must use reusable path parameter %s", route, ref)
			}
		}
	}
}

func TestOpenAPIDraftDocumentsDeliveryAttemptDiagnostics(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("../../docs/openapi.yaml")
	if err != nil {
		t.Fatalf("read OpenAPI draft: %v", err)
	}
	draft := string(raw)
	block := extractOpenAPIComponentBlock(t, draft, "schemas", "DeliveryAttempt")
	for _, field := range []string{"sender", "enhanced_status", "dsn_return", "dsn_envelope_id", "dsn_notify", "original_recipient"} {
		if !strings.Contains(block, "        "+field+":") {
			t.Fatalf("DeliveryAttempt schema must document diagnostic field %q", field)
		}
	}
}

func TestOpenAPIDraftDocumentsDKIMCreateInput(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("../../docs/openapi.yaml")
	if err != nil {
		t.Fatalf("read OpenAPI draft: %v", err)
	}
	block := extractOpenAPIComponentBlock(t, string(raw), "schemas", "DKIMKeyCreateRequest")
	for _, field := range []string{"domain_id", "selector", "private_key_pem", "public_key_dns"} {
		if !strings.Contains(block, "        "+field+":") {
			t.Fatalf("DKIMKeyCreateRequest schema must document CreateDKIMKeyInput field %q", field)
		}
	}
	if strings.Contains(block, "        active:") {
		t.Fatal("DKIMKeyCreateRequest schema must not document unsupported active input")
	}
}

func TestOpenAPIDraftDocumentsQuotaUpdateInputs(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("../../docs/openapi.yaml")
	if err != nil {
		t.Fatalf("read OpenAPI draft: %v", err)
	}
	draft := string(raw)
	operations := extractOpenAPIOperationBlocks(t, "../../docs/openapi.yaml")
	for route, requestBodyRef := range map[string]string{
		"PATCH /companies/{id}/quota": "#/components/requestBodies/CompanyQuotaUpdate",
		"PATCH /domains/{id}/quota":   "#/components/requestBodies/DomainQuotaUpdate",
		"PATCH /users/{id}/quota":     "#/components/requestBodies/UserQuotaUpdate",
	} {
		block, ok := operations[route]
		if !ok {
			t.Fatalf("OpenAPI operation %s is missing", route)
		}
		if !strings.Contains(block, `$ref: "`+requestBodyRef+`"`) {
			t.Fatalf("OpenAPI operation %s must use request body %s", route, requestBodyRef)
		}
	}

	for schema, fields := range map[string][]string{
		"CompanyQuotaUpdateRequest": {"quota_limit"},
		"DomainQuotaUpdateRequest":  {"quota_limit", "default_user_quota"},
		"UserQuotaUpdateRequest":    {"quota_limit", "quota_source"},
	} {
		block := extractOpenAPIComponentBlock(t, draft, "schemas", schema)
		for _, field := range fields {
			if !strings.Contains(block, "        "+field+":") {
				t.Fatalf("%s schema must document field %q", schema, field)
			}
		}
	}
}

func TestOpenAPIDraftDocumentsAdminStatusEnums(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("../../docs/openapi.yaml")
	if err != nil {
		t.Fatalf("read OpenAPI draft: %v", err)
	}
	draft := string(raw)
	for _, status := range []string{"active", "suspended", "disabled"} {
		if err := maildb.ValidateUpdateDomainStatusRequest(maildb.UpdateDomainStatusRequest{ID: "domain-1", Status: status}); err != nil {
			t.Fatalf("domain status fixture %q is invalid: %v", status, err)
		}
		if err := maildb.ValidateUpdateUserStatusRequest(maildb.UpdateUserStatusRequest{ID: "user-1", Status: status}); err != nil {
			t.Fatalf("user status fixture %q is invalid: %v", status, err)
		}
	}
	statusBlock := extractOpenAPIComponentBlock(t, draft, "schemas", "StatusUpdateRequest")
	if !strings.Contains(statusBlock, "enum: [active, suspended, disabled]") {
		t.Fatalf("StatusUpdateRequest must document domain/user status enum, got:\n%s", statusBlock)
	}

	for _, status := range []string{"active", "disabled"} {
		if err := maildb.ValidateUpdateDeliveryRouteStatusRequest(maildb.UpdateDeliveryRouteStatusRequest{ID: "route-1", Status: status}); err != nil {
			t.Fatalf("delivery route status fixture %q is invalid: %v", status, err)
		}
	}
	routeStatusBlock := extractOpenAPIComponentBlock(t, draft, "schemas", "DeliveryRouteStatusUpdateRequest")
	if !strings.Contains(routeStatusBlock, "enum: [active, disabled]") {
		t.Fatalf("DeliveryRouteStatusUpdateRequest must document delivery route status enum, got:\n%s", routeStatusBlock)
	}
}

func TestOpenAPIDraftOperationsHaveStableOperationIDs(t *testing.T) {
	t.Parallel()

	operations := extractOpenAPIOperationBlocks(t, "../../docs/openapi.yaml")
	seen := make(map[string]string)
	pattern := regexp.MustCompile(`(?m)^\s+operationId: ([a-z][A-Za-z0-9]*)$`)
	for route, block := range operations {
		match := pattern.FindStringSubmatch(block)
		if match == nil {
			t.Fatalf("OpenAPI operation %s must declare a lower-camel operationId", route)
		}
		id := match[1]
		if previous := seen[id]; previous != "" {
			t.Fatalf("OpenAPI operationId %q is duplicated by %s and %s", id, previous, route)
		}
		seen[id] = route
	}
}

func TestOpenAPIDraftDocumentsReusableErrorResponseForMutableAndProtectedOperations(t *testing.T) {
	t.Parallel()

	operations := extractOpenAPIOperationBlocks(t, "../../docs/openapi.yaml")
	for route, block := range operations {
		if strings.HasPrefix(route, "GET /health/") || route == "GET /info" {
			continue
		}
		if !strings.Contains(block, `"default":`) && !strings.Contains(block, "default:") {
			t.Fatalf("OpenAPI operation %s must document the reusable default Error response", route)
		}
		if !strings.Contains(block, `$ref: "#/components/responses/Error"`) {
			t.Fatalf("OpenAPI operation %s must reuse components.responses.Error", route)
		}
	}
}

func TestOpenAPIDraftHasNoDuplicateAdjacentRefs(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("../../docs/openapi.yaml")
	if err != nil {
		t.Fatalf("read OpenAPI draft: %v", err)
	}
	var previous string
	for lineNumber, line := range strings.Split(string(raw), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, `$ref: `) && trimmed == previous {
			t.Fatalf("OpenAPI draft has duplicate adjacent %s at line %d", trimmed, lineNumber+1)
		}
		previous = trimmed
	}
}

func TestOpenAPIDraftResponseSchemasExposeEnvelopeKeys(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("../../docs/openapi.yaml")
	if err != nil {
		t.Fatalf("read OpenAPI draft: %v", err)
	}
	draft := string(raw)
	for schema, key := range map[string]string{
		"FolderListEnvelope":                                  "folders",
		"FolderEnvelope":                                      "folder",
		"MessageListPageEnvelope":                             "messages",
		"MessageListEnvelope":                                 "messages",
		"MessageEnvelope":                                     "message",
		"MessageDeliveryStatusEnvelope":                       "delivery_status",
		"ThreadListEnvelope":                                  "threads",
		"DraftEnvelope":                                       "draft",
		"SendQueuedEnvelope":                                  "message",
		"AttachmentListEnvelope":                              "attachments",
		"AttachmentEnvelope":                                  "attachment",
		"AttachmentUploadSessionEnvelope":                     "attachment_upload_session",
		"AttachmentUploadCapabilitiesEnvelope":                "attachment_upload_capabilities",
		"PushDeviceListEnvelope":                              "push_devices",
		"PushDeviceEnvelope":                                  "push_device",
		"QueueStatsEnvelope":                                  "queues",
		"IMAPUIDBackfillEnvelope":                             "imap_uid_backfill",
		"OutboxEventListEnvelope":                             "outbox_events",
		"OutboxEventEnvelope":                                 "outbox_event",
		"BackpressureEnvelope":                                "backpressure",
		"QuotaUsageListEnvelope":                              "quota_usage",
		"APIUsageDailyListEnvelope":                           "api_usage_daily",
		"APIUsageMonthlyListEnvelope":                         "api_usage_monthly",
		"APIUsageLedgerListEnvelope":                          "api_usage_ledger",
		"APIUsageLedgerStatsEnvelope":                         "api_usage_ledger_stats",
		"APIUsageLedgerRetentionReadinessEnvelope":            "api_usage_ledger_retention_readiness",
		"APIUsageExportCapabilitiesEnvelope":                  "api_usage_export_capabilities",
		"APIUsageExportBatchEnvelope":                         "api_usage_export_batch",
		"APIUsageExportBatchListEnvelope":                     "api_usage_export_batches",
		"APIUsageExportHandoffReadinessEnvelope":              "api_usage_export_handoff_readiness",
		"AttachmentCleanupCandidatesEnvelope":                 "attachment_cleanup_candidates",
		"AttachmentCleanupRunEnvelope":                        "attachment_cleanup_run",
		"APIUsageExportArtifactEnvelope":                      "api_usage_export_artifact",
		"APIUsageExportArtifactListEnvelope":                  "api_usage_export_artifacts",
		"APIUsageExportArtifactVerificationEnvelope":          "api_usage_export_artifact_verification",
		"APIUsageExportManifestDigestEnvelope":                "api_usage_export_manifest_digest",
		"APIUsageExportManifestDigestListEnvelope":            "api_usage_export_manifest_digests",
		"APIUsageExportManifestDigestVerificationEnvelope":    "api_usage_export_manifest_digest_verification",
		"APIUsageExportManifestSignatureEnvelope":             "api_usage_export_manifest_signature",
		"APIUsageExportManifestSignatureListEnvelope":         "api_usage_export_manifest_signatures",
		"APIUsageExportManifestSignatureVerificationEnvelope": "api_usage_export_manifest_signature_verification",
		"QuotaReconciliationListEnvelope":                     "quota_reconciliation",
		"QuotaCorrectionEnvelope":                             "quota_correction",
		"DeliveryAttemptsEnvelope":                            "delivery_attempts",
		"DeliveryAttemptStatsEnvelope":                        "delivery_attempt_stats",
		"ExhaustedAttemptsEnvelope":                           "exhausted_attempts",
		"PushNotificationAttemptEnvelope":                     "push_notification_attempt",
		"PushNotificationAttemptsEnvelope":                    "push_notification_attempts",
		"PushNotificationStatsEnvelope":                       "push_notification_stats",
		"SuppressionListEnvelope":                             "suppression_list",
		"DKIMKeyListEnvelope":                                 "dkim_keys",
		"DKIMKeyDNSVerificationEnvelope":                      "dkim_verification",
		"TrustedRelayListEnvelope":                            "trusted_relays",
		"TrustedRelayEnvelope":                                "trusted_relay",
		"DeliveryRouteListEnvelope":                           "delivery_routes",
		"DeliveryRouteEnvelope":                               "delivery_route",
		"DeliveryRouteResolutionEnvelope":                     "delivery_route_resolution",
		"DeliveryRouteCountersEnvelope":                       "route_counters",
		"CompanyListEnvelope":                                 "companies",
		"CompanyEnvelope":                                     "company",
		"DomainListEnvelope":                                  "domains",
		"DomainEnvelope":                                      "domain",
		"DomainStatsEnvelope":                                 "stats",
		"DomainDNSCheckEnvelope":                              "dns_check",
		"DomainDNSCheckHistoryEnvelope":                       "dns_checks",
		"DomainPolicyEnvelope":                                "domain_policy",
		"UserListEnvelope":                                    "users",
		"UserEnvelope":                                        "user",
	} {
		block := extractOpenAPIComponentBlock(t, draft, "schemas", schema)
		if !openAPIRequiredListContains(block, key) {
			t.Fatalf("OpenAPI schema %s must require envelope key %q", schema, key)
		}
		if !strings.Contains(block, "        "+key+":") {
			t.Fatalf("OpenAPI schema %s must expose envelope key %q", schema, key)
		}
	}
}

func TestOpenAPIDraftDocumentsReusableErrorEnvelopeResponse(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("../../docs/openapi.yaml")
	if err != nil {
		t.Fatalf("read OpenAPI draft: %v", err)
	}
	draft := string(raw)
	response := extractOpenAPIComponentBlock(t, draft, "responses", "Error")
	if !strings.Contains(response, `description: Stable structured error envelope`) {
		t.Fatalf("OpenAPI Error response must describe the stable structured error envelope")
	}
	if !strings.Contains(response, `$ref: "#/components/schemas/ErrorEnvelope"`) {
		t.Fatalf("OpenAPI Error response must point at ErrorEnvelope")
	}
	schema := extractOpenAPIComponentBlock(t, draft, "schemas", "ErrorEnvelope")
	for _, required := range []string{"error", "code", "message", "status", "status_text", "error_message"} {
		if !strings.Contains(schema, required) {
			t.Fatalf("OpenAPI ErrorEnvelope must document %q", required)
		}
	}
}

func TestOpenAPIDraftDocumentsDevelopmentUserIDFallbackParameter(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("../../docs/openapi.yaml")
	if err != nil {
		t.Fatalf("read OpenAPI draft: %v", err)
	}
	param := extractOpenAPIComponentBlock(t, string(raw), "parameters", "UserIDFallback")
	for _, want := range []string{
		"name: user_id",
		"in: query",
		"Development fallback required only when JWT auth is disabled.",
	} {
		if !strings.Contains(param, want) {
			t.Fatalf("OpenAPI UserIDFallback parameter must document %q", want)
		}
	}
}

func openAPIRequiredListContains(block string, key string) bool {
	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "required: [") {
			continue
		}
		required := strings.TrimPrefix(line, "required: [")
		required = strings.TrimSuffix(required, "]")
		for _, item := range strings.Split(required, ",") {
			if strings.TrimSpace(item) == key {
				return true
			}
		}
	}
	return false
}

func TestOpenAPIDraftHasNoDanglingComponentRefs(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("../../docs/openapi.yaml")
	if err != nil {
		t.Fatalf("read OpenAPI draft: %v", err)
	}
	draft := string(raw)
	for _, match := range regexp.MustCompile(`\$ref: "#/components/([^/]+)/([^"]+)"`).FindAllStringSubmatch(draft, -1) {
		section, name := match[1], match[2]
		if !openAPIComponentExists(draft, section, name) {
			t.Fatalf("OpenAPI draft has dangling component ref %q", match[0])
		}
	}
}

func extractOpenAPIComponentBlock(t *testing.T, draft string, section string, name string) string {
	t.Helper()

	inSection := false
	inComponent := false
	var block strings.Builder
	for _, line := range strings.Split(draft, "\n") {
		if line == "components:" {
			continue
		}
		if strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "    ") {
			if inComponent {
				return block.String()
			}
			inSection = strings.TrimSuffix(strings.TrimSpace(line), ":") == section
			inComponent = false
			continue
		}
		if !inSection {
			continue
		}
		if strings.HasPrefix(line, "    ") && !strings.HasPrefix(line, "      ") {
			componentName := strings.TrimSuffix(strings.TrimSpace(line), ":")
			if inComponent {
				return block.String()
			}
			inComponent = componentName == name
		}
		if inComponent {
			block.WriteString(line)
			block.WriteByte('\n')
		}
	}
	if inComponent {
		return block.String()
	}
	t.Fatalf("OpenAPI component %s/%s is missing", section, name)
	return ""
}

func openAPIComponentExists(draft string, section string, name string) bool {
	inComponents := false
	currentSection := ""
	for _, line := range strings.Split(draft, "\n") {
		if line == "components:" {
			inComponents = true
			continue
		}
		if !inComponents {
			continue
		}
		if strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "    ") {
			currentSection = strings.TrimSuffix(strings.TrimSpace(line), ":")
			continue
		}
		if currentSection == section && strings.HasPrefix(line, "    ") && !strings.HasPrefix(line, "      ") {
			if strings.TrimSuffix(strings.TrimSpace(line), ":") == name {
				return true
			}
		}
	}
	return false
}

func extractRegisteredRoutes(t *testing.T, filenames ...string) []string {
	t.Helper()

	pattern := regexp.MustCompile(`mux\.HandleFunc\("([A-Z]+) (/(?:api|admin)/v1/[^"]+|/health/(?:live|ready))"`)
	var routes []string
	for _, filename := range filenames {
		raw, err := os.ReadFile(filename)
		if err != nil {
			t.Fatalf("read registered route source %s: %v", filename, err)
		}
		for _, match := range pattern.FindAllStringSubmatch(string(raw), -1) {
			routes = append(routes, match[1]+" "+normalizeOpenAPIPath(match[2]))
		}
	}
	sort.Strings(routes)
	return routes
}

func extractOpenAPIRoutes(t *testing.T, filename string) map[string]bool {
	t.Helper()

	raw, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("read OpenAPI draft: %v", err)
	}

	routes := make(map[string]bool)
	var currentPath string
	pathPattern := regexp.MustCompile(`^  (/[^:]+):\s*$`)
	methodPattern := regexp.MustCompile(`^    (get|put|post|patch|delete):\s*$`)
	for _, line := range strings.Split(string(raw), "\n") {
		if match := pathPattern.FindStringSubmatch(line); match != nil {
			currentPath = match[1]
			continue
		}
		if currentPath == "" {
			continue
		}
		if match := methodPattern.FindStringSubmatch(line); match != nil {
			routes[strings.ToUpper(match[1])+" "+normalizeOpenAPIPath(currentPath)] = true
		}
	}
	return routes
}

func extractOpenAPIOperationBlocks(t *testing.T, filename string) map[string]string {
	t.Helper()

	raw, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("read OpenAPI draft: %v", err)
	}

	operations := make(map[string]string)
	var currentPath string
	var currentRoute string
	var currentBlock strings.Builder
	pathPattern := regexp.MustCompile(`^  (/[^:]+):\s*$`)
	methodPattern := regexp.MustCompile(`^    (get|put|post|patch|delete):\s*$`)
	flush := func() {
		if currentRoute != "" {
			operations[currentRoute] = currentBlock.String()
		}
		currentRoute = ""
		currentBlock.Reset()
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if match := pathPattern.FindStringSubmatch(line); match != nil {
			flush()
			currentPath = normalizeOpenAPIPath(match[1])
			continue
		}
		if match := methodPattern.FindStringSubmatch(line); match != nil {
			flush()
			currentRoute = strings.ToUpper(match[1]) + " " + currentPath
			currentBlock.WriteString(line)
			currentBlock.WriteByte('\n')
			continue
		}
		if currentRoute != "" {
			currentBlock.WriteString(line)
			currentBlock.WriteByte('\n')
		}
	}
	flush()
	return operations
}

func normalizeOpenAPIPath(path string) string {
	path = strings.TrimSpace(path)
	path = strings.TrimPrefix(path, "/api/v1")
	path = strings.TrimPrefix(path, "/admin/v1")
	return path
}

func openAPIOperationDocumentsParameter(block string, name string) bool {
	if name == "limit" && strings.Contains(block, "#/components/parameters/Limit") {
		return true
	}
	if name == "id" && strings.Contains(block, "#/components/parameters/PathID") {
		return true
	}
	for _, namedRef := range []struct {
		name string
		ref  string
	}{
		{name: "attachment_id", ref: "#/components/parameters/AttachmentID"},
		{name: "digest_id", ref: "#/components/parameters/DigestID"},
		{name: "signature_id", ref: "#/components/parameters/SignatureID"},
	} {
		if name == namedRef.name && strings.Contains(block, namedRef.ref) {
			return true
		}
	}
	return strings.Contains(block, "name: "+name)
}

func openAPIOperationDocumentsRequiredQueryParameter(block string, name string) bool {
	start := strings.Index(block, "- name: "+name)
	if start < 0 {
		return false
	}
	segment := block[start:]
	if next := strings.Index(segment[len("- name: ")+len(name):], "\n        - name: "); next >= 0 {
		segment = segment[:len("- name: ")+len(name)+next]
	}
	return strings.Contains(segment, "in: query") && strings.Contains(segment, "required: true")
}

func extractOpenAPIPathParameters(route string) []string {
	matches := regexp.MustCompile(`\{([A-Za-z_][A-Za-z0-9_]*)\}`).FindAllStringSubmatch(route, -1)
	params := make([]string, 0, len(matches))
	for _, match := range matches {
		params = append(params, match[1])
	}
	return params
}
