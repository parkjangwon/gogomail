package httpapi

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/gogomail/gogomail/internal/maildb"
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
		"POST /push-devices",
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
		"PATCH /backpressure",
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

func TestOpenAPIDraftDocumentsStableResponseEnvelopes(t *testing.T) {
	t.Parallel()

	operations := extractOpenAPIOperationBlocks(t, "../../docs/openapi.yaml")
	for route, responseRef := range map[string]string{
		"GET /folders":                                  "#/components/responses/FolderList",
		"POST /folders":                                 "#/components/responses/Folder",
		"PATCH /folders/{id}":                           "#/components/responses/Folder",
		"DELETE /folders/{id}":                          "#/components/responses/Status",
		"GET /messages":                                 "#/components/responses/MessageListPage",
		"GET /search":                                   "#/components/responses/MessageList",
		"GET /messages/{id}":                            "#/components/responses/Message",
		"GET /messages/{id}/delivery-status":            "#/components/responses/MessageDeliveryStatus",
		"GET /threads":                                  "#/components/responses/ThreadList",
		"GET /threads/{id}/messages":                    "#/components/responses/MessageList",
		"PATCH /messages/{id}/flags":                    "#/components/responses/Status",
		"PATCH /messages/{id}/folder":                   "#/components/responses/Status",
		"PATCH /messages/bulk/flags":                    "#/components/responses/BulkUpdate",
		"PATCH /messages/bulk/folder":                   "#/components/responses/BulkUpdate",
		"POST /messages/bulk/delete":                    "#/components/responses/BulkUpdate",
		"DELETE /messages/{id}":                         "#/components/responses/Status",
		"POST /messages/send":                           "#/components/responses/SendQueued",
		"POST /drafts":                                  "#/components/responses/Draft",
		"PATCH /drafts/{id}":                            "#/components/responses/Draft",
		"DELETE /drafts/{id}":                           "#/components/responses/Status",
		"POST /drafts/{id}/send":                        "#/components/responses/SendQueued",
		"POST /attachments":                             "#/components/responses/Attachment",
		"POST /attachments/upload":                      "#/components/responses/Attachment",
		"GET /messages/{id}/attachments":                "#/components/responses/AttachmentList",
		"GET /push-devices":                             "#/components/responses/PushDeviceList",
		"POST /push-devices":                            "#/components/responses/PushDevice",
		"DELETE /push-devices/{id}":                     "#/components/responses/IDStatus",
		"GET /domains":                                  "#/components/responses/DomainList",
		"GET /domains/{id}":                             "#/components/responses/Domain",
		"GET /domains/{id}/dns-check":                   "#/components/responses/DomainDNSCheck",
		"GET /domains/{id}/dns-checks":                  "#/components/responses/DomainDNSCheckHistory",
		"POST /domains":                                 "#/components/responses/Domain",
		"PATCH /domains/{id}/status":                    "#/components/responses/IDStatus",
		"PATCH /domains/{id}/quota":                     "#/components/responses/IDStatus",
		"PATCH /domains/{id}/policy":                    "#/components/responses/DomainPolicy",
		"GET /users":                                    "#/components/responses/UserList",
		"GET /users/{id}":                               "#/components/responses/User",
		"POST /users":                                   "#/components/responses/User",
		"PATCH /users/{id}/status":                      "#/components/responses/IDStatus",
		"PATCH /users/{id}/quota":                       "#/components/responses/IDStatus",
		"GET /queue":                                    "#/components/responses/QueueStats",
		"GET /backpressure":                             "#/components/responses/Backpressure",
		"PATCH /backpressure":                           "#/components/responses/Backpressure",
		"GET /quota-usage":                              "#/components/responses/QuotaUsageList",
		"GET /api-usage/daily":                          "#/components/responses/APIUsageDailyList",
		"GET /api-usage/monthly":                        "#/components/responses/APIUsageMonthlyList",
		"GET /api-usage/ledger":                         "#/components/responses/APIUsageLedgerList",
		"GET /api-usage/ledger/export":                  "#/components/responses/APIUsageLedgerExport",
		"GET /api-usage/ledger/stats":                   "#/components/responses/APIUsageLedgerStats",
		"GET /api-usage/export-batches":                 "#/components/responses/APIUsageExportBatchList",
		"POST /api-usage/export-batches":                "#/components/responses/APIUsageExportBatch",
		"GET /api-usage/export-batches/{id}":            "#/components/responses/APIUsageExportBatch",
		"GET /api-usage/export-batches/{id}/export":     "#/components/responses/APIUsageLedgerExport",
		"GET /api-usage/export-batches/{id}/artifacts":  "#/components/responses/APIUsageExportArtifactList",
		"POST /api-usage/export-batches/{id}/artifacts": "#/components/responses/APIUsageExportArtifact",
		"GET /api-usage/export-batches/{id}/artifacts/{artifact_id}":                   "#/components/responses/APIUsageExportArtifact",
		"GET /api-usage/export-batches/{id}/manifest-digests":                          "#/components/responses/APIUsageExportManifestDigestList",
		"POST /api-usage/export-batches/{id}/manifest-digests":                         "#/components/responses/APIUsageExportManifestDigest",
		"GET /api-usage/export-batches/{id}/manifest-digests/{digest_id}":              "#/components/responses/APIUsageExportManifestDigest",
		"GET /api-usage/export-batches/{id}/manifest-digests/{digest_id}/verification": "#/components/responses/APIUsageExportManifestDigestVerification",
		"GET /quota-reconciliation":                                                    "#/components/responses/QuotaReconciliationList",
		"POST /quota-reconciliation/corrections":                                       "#/components/responses/QuotaCorrection",
		"GET /delivery-attempts":                                                       "#/components/responses/DeliveryAttempts",
		"GET /push-notification-attempts":                                              "#/components/responses/PushNotificationAttempts",
		"GET /push-notification-stats":                                                 "#/components/responses/PushNotificationStats",
		"GET /suppression-list":                                                        "#/components/responses/SuppressionList",
		"DELETE /suppression-list/{id}":                                                "#/components/responses/IDStatus",
		"GET /trusted-relays":                                                          "#/components/responses/TrustedRelayList",
		"POST /trusted-relays":                                                         "#/components/responses/TrustedRelay",
		"DELETE /trusted-relays/{id}":                                                  "#/components/responses/IDStatus",
		"GET /delivery-routes":                                                         "#/components/responses/DeliveryRouteList",
		"POST /delivery-routes":                                                        "#/components/responses/DeliveryRoute",
		"GET /delivery-routes/resolve":                                                 "#/components/responses/DeliveryRouteResolution",
		"PATCH /delivery-routes/{id}/status":                                           "#/components/responses/IDStatus",
		"DELETE /delivery-routes/{id}":                                                 "#/components/responses/IDStatus",
		"GET /dkim-keys":                                                               "#/components/responses/DKIMKeyList",
		"POST /dkim-keys":                                                              "#/components/responses/IDStatus",
		"DELETE /dkim-keys/{id}":                                                       "#/components/responses/IDStatus",
		"POST /outbox/{id}/retry":                                                      "#/components/responses/IDStatus",
		"GET /messages/{id}/attachments/{attachment_id}/download":                      "",
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
		"FolderListEnvelope":                               "folders",
		"FolderEnvelope":                                   "folder",
		"MessageListPageEnvelope":                          "messages",
		"MessageListEnvelope":                              "messages",
		"MessageEnvelope":                                  "message",
		"MessageDeliveryStatusEnvelope":                    "delivery_status",
		"ThreadListEnvelope":                               "threads",
		"DraftEnvelope":                                    "draft",
		"SendQueuedEnvelope":                               "message",
		"AttachmentListEnvelope":                           "attachments",
		"AttachmentEnvelope":                               "attachment",
		"PushDeviceListEnvelope":                           "push_devices",
		"PushDeviceEnvelope":                               "push_device",
		"QueueStatsEnvelope":                               "queues",
		"BackpressureEnvelope":                             "backpressure",
		"QuotaUsageListEnvelope":                           "quota_usage",
		"APIUsageDailyListEnvelope":                        "api_usage_daily",
		"APIUsageMonthlyListEnvelope":                      "api_usage_monthly",
		"APIUsageLedgerListEnvelope":                       "api_usage_ledger",
		"APIUsageLedgerStatsEnvelope":                      "api_usage_ledger_stats",
		"APIUsageExportBatchEnvelope":                      "api_usage_export_batch",
		"APIUsageExportBatchListEnvelope":                  "api_usage_export_batches",
		"APIUsageExportArtifactEnvelope":                   "api_usage_export_artifact",
		"APIUsageExportArtifactListEnvelope":               "api_usage_export_artifacts",
		"APIUsageExportManifestDigestEnvelope":             "api_usage_export_manifest_digest",
		"APIUsageExportManifestDigestListEnvelope":         "api_usage_export_manifest_digests",
		"APIUsageExportManifestDigestVerificationEnvelope": "api_usage_export_manifest_digest_verification",
		"QuotaReconciliationListEnvelope":                  "quota_reconciliation",
		"QuotaCorrectionEnvelope":                          "quota_correction",
		"DeliveryAttemptsEnvelope":                         "delivery_attempts",
		"PushNotificationAttemptsEnvelope":                 "push_notification_attempts",
		"PushNotificationStatsEnvelope":                    "push_notification_stats",
		"SuppressionListEnvelope":                          "suppression_list",
		"DKIMKeyListEnvelope":                              "dkim_keys",
		"TrustedRelayListEnvelope":                         "trusted_relays",
		"TrustedRelayEnvelope":                             "trusted_relay",
		"DeliveryRouteListEnvelope":                        "delivery_routes",
		"DeliveryRouteEnvelope":                            "delivery_route",
		"DeliveryRouteResolutionEnvelope":                  "delivery_route_resolution",
		"DomainListEnvelope":                               "domains",
		"DomainEnvelope":                                   "domain",
		"DomainDNSCheckEnvelope":                           "dns_check",
		"DomainDNSCheckHistoryEnvelope":                    "dns_checks",
		"DomainPolicyEnvelope":                             "domain_policy",
		"UserListEnvelope":                                 "users",
		"UserEnvelope":                                     "user",
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
	methodPattern := regexp.MustCompile(`^    (get|post|patch|delete):\s*$`)
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
	methodPattern := regexp.MustCompile(`^    (get|post|patch|delete):\s*$`)
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
