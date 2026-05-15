package httpapi

import (
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/gogomail/gogomail/internal/drive"
	"github.com/gogomail/gogomail/internal/maildb"
	"github.com/gogomail/gogomail/internal/mailservice"
	"go.yaml.in/yaml/v3"
)

func TestOpenAPIDraftIsParseableYAML(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("../../docs/openapi.yaml")
	if err != nil {
		t.Fatalf("read OpenAPI draft: %v", err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("OpenAPI draft must be parseable YAML: %v", err)
	}
	if doc["openapi"] != "3.1.0" {
		t.Fatalf("OpenAPI draft version = %v, want 3.1.0", doc["openapi"])
	}
	if _, ok := doc["paths"].(map[string]any); !ok {
		t.Fatal("OpenAPI draft must contain a paths object")
	}
}

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

func TestOpenAPIDraftPinsAdminBootstrapOperationsToAdminBase(t *testing.T) {
	t.Parallel()

	operations := extractOpenAPIOperationBlocks(t, "../../docs/openapi.yaml")
	for _, route := range []string{
		"GET /console/capabilities",
		"GET /delivery-routes/counters",
		"GET /queue",
		"POST /imap/mailboxes/{id}/uid-backfill",
		"GET /companies",
		"GET /companies/{id}",
		"PATCH /companies/{id}/quota",
		"GET /domains",
		"POST /domains",
		"GET /domains/{id}",
		"GET /domains/{id}/stats",
		"GET /domains/{id}/dns-check",
		"GET /domains/{id}/dns-checks",
		"PATCH /domains/{id}/status",
		"PATCH /domains/{id}/quota",
		"PATCH /domains/{id}/policy",
		"GET /users",
		"POST /users",
		"GET /users/{id}",
		"PATCH /users/{id}/status",
		"PATCH /users/{id}/quota",
		"PATCH /users/{id}/password-hash",
		"GET /outbox-events",
		"GET /outbox-events/{id}",
		"GET /audit-logs",
		"GET /audit-logs/integrity",
		"GET /audit-logs/{id}",
		"GET /companies/{id}/security/login-audits",
		"GET /directory/principals",
		"GET /directory/aliases/resolve",
		"GET /directory/aliases",
		"POST /directory/aliases",
		"DELETE /directory/aliases/{id}",
		"GET /directory/delegations",
		"POST /directory/delegations",
		"DELETE /directory/delegations/{id}",
		"PATCH /directory/delegations/{id}/role",
		"PATCH /directory/delegations/{id}/assignment",
		"GET /directory/group-memberships",
		"POST /directory/group-memberships",
		"DELETE /directory/group-memberships/{id}",
		"PATCH /directory/group-memberships/{id}/role",
		"PATCH /directory/group-memberships/{id}/assignment",
		"GET /backpressure",
		"PATCH /backpressure",
		"GET /quota-usage",
		"POST /attachment-cleanup/candidates",
		"POST /attachment-cleanup/runs",
		"GET /attachment-upload-sessions",
		"GET /drive-upload-sessions",
		"GET /drive-nodes",
		"GET /drive-nodes/{id}",
		"GET /drive-usage",
		"POST /drive-upload-cleanup/candidates",
		"POST /drive-upload-cleanup/runs",
		"GET /drive-cleanup-failures",
		"POST /drive-cleanup-failures/{id}/resolve",
		"POST /drive-cleanup-failures/retry-runs",
		"GET /quota-reconciliation",
		"POST /quota-reconciliation/corrections",
		"GET /delivery-attempts",
		"GET /delivery-attempts/stats",
		"GET /delivery-attempts/exhausted",
		"GET /push-notification-attempts",
		"GET /push-notification-attempts/{id}",
		"PATCH /push-notification-attempts/{id}/outcome",
		"GET /push-notification-stats",
		"GET /suppression-list",
		"DELETE /suppression-list/{id}",
		"GET /trusted-relays",
		"POST /trusted-relays",
		"DELETE /trusted-relays/{id}",
		"GET /delivery-routes",
		"POST /delivery-routes",
		"GET /delivery-routes/resolve",
		"PATCH /delivery-routes/{id}/status",
		"DELETE /delivery-routes/{id}",
		"GET /dkim-keys",
		"POST /dkim-keys",
		"DELETE /dkim-keys/{id}",
		"POST /dkim-keys/{id}/verify-dns",
		"POST /outbox/{id}/retry",
		"GET /api-usage/daily",
		"GET /api-usage/monthly",
		"GET /api-usage/ledger",
		"GET /api-usage/ledger/export",
		"GET /api-usage/ledger/stats",
		"GET /api-usage/ledger/retention-readiness",
		"GET /api-usage/ledger/retention-runs",
		"POST /api-usage/ledger/retention-runs",
		"GET /api-usage/ledger/retention-runs/{id}",
		"GET /dav-sync/retention-readiness",
		"GET /dav-sync/retention-runs",
		"POST /dav-sync/retention-runs",
		"GET /dav-sync/retention-runs/{id}",
		"GET /api-usage/export-capabilities",
		"GET /api-usage/export-batches",
		"POST /api-usage/export-batches",
		"GET /api-usage/export-batches/{id}",
		"GET /api-usage/export-batches/{id}/handoff-readiness",
		"GET /api-usage/export-batches/{id}/export",
		"GET /api-usage/export-batches/{id}/artifacts",
		"POST /api-usage/export-batches/{id}/artifacts",
		"GET /api-usage/export-batches/{id}/artifacts/{artifact_id}",
		"POST /api-usage/export-batches/{id}/artifacts/write",
		"GET /api-usage/export-batches/{id}/artifacts/{artifact_id}/download",
		"GET /api-usage/export-batches/{id}/artifacts/{artifact_id}/verification",
		"GET /api-usage/export-batches/{id}/manifest-digests",
		"POST /api-usage/export-batches/{id}/manifest-digests",
		"GET /api-usage/export-batches/{id}/manifest-digests/{digest_id}",
		"GET /api-usage/export-batches/{id}/manifest-digests/{digest_id}/verification",
		"GET /api-usage/export-batches/{id}/manifest-digests/{digest_id}/signatures",
		"POST /api-usage/export-batches/{id}/manifest-digests/{digest_id}/signatures",
		"GET /api-usage/export-batches/{id}/manifest-digests/{digest_id}/signatures/{signature_id}",
		"GET /api-usage/export-batches/{id}/manifest-digests/{digest_id}/signatures/{signature_id}/verification",
	} {
		block, ok := operations[route]
		if !ok {
			t.Fatalf("OpenAPI operation %s is missing", route)
		}
		for _, want := range []string{"servers:", "url: /admin/v1", "description: Admin API"} {
			if !strings.Contains(block, want) {
				t.Fatalf("OpenAPI operation %s must pin the admin server with %q:\n%s", route, want, block)
			}
		}
		for _, want := range []string{"security:", "adminToken: []", "bearerAuth: []"} {
			if !strings.Contains(block, want) {
				t.Fatalf("OpenAPI operation %s must document admin auth with %q:\n%s", route, want, block)
			}
		}
	}
}

func TestOpenAPIDraftPinsAllRegisteredAdminOperationsToAdminBase(t *testing.T) {
	t.Parallel()

	operations := extractOpenAPIOperationBlocks(t, "../../docs/openapi.yaml")
	for _, route := range extractRegisteredAdminRoutes(t, "admin.go") {
		block, ok := operations[route]
		if !ok {
			t.Fatalf("OpenAPI operation %s is missing", route)
		}
		for _, want := range []string{"servers:", "url: /admin/v1", "description: Admin API"} {
			if !strings.Contains(block, want) {
				t.Fatalf("OpenAPI operation %s must pin the admin server with %q:\n%s", route, want, block)
			}
		}
		for _, want := range []string{"security:", "adminToken: []", "bearerAuth: []"} {
			if !strings.Contains(block, want) {
				t.Fatalf("OpenAPI operation %s must document admin auth with %q:\n%s", route, want, block)
			}
		}
	}
}

func TestOpenAPIDraftPinsAllRegisteredMailOperationsToMailBase(t *testing.T) {
	t.Parallel()

	operations := extractOpenAPIOperationBlocks(t, "../../docs/openapi.yaml")
	for _, route := range extractRegisteredMailRoutes(t, "mail.go", "drive.go") {
		block, ok := operations[route]
		if !ok {
			t.Fatalf("OpenAPI operation %s is missing", route)
		}
		for _, want := range []string{"servers:", "url: /api/v1", "description: Mail API"} {
			if !strings.Contains(block, want) {
				t.Fatalf("OpenAPI operation %s must pin the Mail API server with %q:\n%s", route, want, block)
			}
		}
	}
}

func TestOpenAPIDraftPinsHealthAndInfoServers(t *testing.T) {
	t.Parallel()

	operations := extractOpenAPIOperationBlocks(t, "../../docs/openapi.yaml")
	for _, route := range []string{"GET /health/live", "GET /health/ready"} {
		block, ok := operations[route]
		if !ok {
			t.Fatalf("OpenAPI operation %s is missing", route)
		}
		for _, want := range []string{"servers:", "url: /", "description: Service root"} {
			if !strings.Contains(block, want) {
				t.Fatalf("OpenAPI operation %s must pin the service root server with %q:\n%s", route, want, block)
			}
		}
	}
	block, ok := operations["GET /info"]
	if !ok {
		t.Fatal("OpenAPI operation GET /info is missing")
	}
	for _, want := range []string{"servers:", "url: /api/v1", "description: Mail API"} {
		if !strings.Contains(block, want) {
			t.Fatalf("OpenAPI operation GET /info must pin the Mail API server with %q:\n%s", want, block)
		}
	}
}

func TestOpenAPIDraftDocumentsPublicShareLinkRoutesAsUnauthenticated(t *testing.T) {
	t.Parallel()

	operations := extractOpenAPIOperationBlocks(t, "../../docs/openapi.yaml")
	for _, route := range []string{
		"GET /drive/share-links/{id}",
		"HEAD /drive/share-links/{id}/download",
		"GET /drive/share-links/{id}/download",
	} {
		block, ok := operations[route]
		if !ok {
			t.Fatalf("OpenAPI operation %s is missing", route)
		}
		if !strings.Contains(block, "security: []") {
			t.Fatalf("OpenAPI operation %s must opt out of global bearer auth:\n%s", route, block)
		}
	}
}

func TestOpenAPIDraftCoversRegisteredHTTPRoutes(t *testing.T) {
	t.Parallel()

	registered := extractRegisteredRoutes(t, "mail.go", "drive.go", "admin.go", "admin_helpers.go", "orgchart.go", "health.go")
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

func TestOpenAPIDraftDoesNotExposeUnregisteredRoutes(t *testing.T) {
	t.Parallel()

	registered := make(map[string]bool)
	for _, route := range extractRegisteredRoutes(t, "mail.go", "drive.go", "admin.go", "admin_helpers.go", "orgchart.go", "health.go") {
		registered[route] = true
	}
	documented := extractOpenAPIRoutes(t, "../../docs/openapi.yaml")

	var stale []string
	for route := range documented {
		if !registered[route] {
			stale = append(stale, route)
		}
	}
	if len(stale) > 0 {
		sort.Strings(stale)
		t.Fatalf("OpenAPI draft exposes unregistered routes:\n%s", strings.Join(stale, "\n"))
	}
}

func TestOpenAPIDraftDocumentsRequestBodies(t *testing.T) {
	t.Parallel()

	operations := extractOpenAPIOperationBlocks(t, "../../docs/openapi.yaml")
	for _, route := range []string{
		"POST /folders",
		"POST /drive/folders",
		"POST /drive/upload-sessions",
		"PUT /drive/upload-sessions/{id}/body",
		"POST /drive/files/finalize",
		"PUT /drive/files/staged/{upload_id}/body",
		"PATCH /drive/nodes/{id}/name",
		"PATCH /drive/nodes/{id}/parent",
		"POST /drive/nodes/{id}/share-links",
		"PATCH /folders/{id}",
		"PATCH /messages/{id}/flags",
		"PATCH /messages/{id}/folder",
		"PATCH /messages/bulk/flags",
		"PATCH /threads/bulk/flags",
		"PATCH /messages/bulk/folder",
		"PATCH /threads/bulk/folder",
		"POST /messages/bulk/delete",
		"POST /threads/bulk/delete",
		"POST /messages/bulk/restore",
		"POST /threads/bulk/restore",
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
		"PATCH /users/{id}/password-hash",
		"POST /trusted-relays",
		"POST /delivery-routes",
		"PATCH /delivery-routes/{id}/status",
		"PATCH /push-notification-attempts/{id}/outcome",
		"PATCH /backpressure",
		"POST /attachment-cleanup/candidates",
		"POST /drive-upload-cleanup/candidates",
		"POST /drive-upload-cleanup/runs",
		"POST /drive-cleanup-failures/retry-runs",
		"POST /attachment-cleanup/runs",
		"POST /api-usage/ledger/retention-runs",
		"POST /dav-sync/retention-runs",
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

func TestOpenAPIDraftDocumentsDriveCapabilityLimits(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("../../docs/openapi.yaml")
	if err != nil {
		t.Fatalf("read OpenAPI draft: %v", err)
	}
	block := extractOpenAPIComponentBlock(t, string(raw), "schemas", "WebmailCapabilities")
	for _, want := range []string{
		"maximum: " + strconv.FormatInt(drive.MaxUploadSessionBytes, 10),
		"maximum: " + strconv.FormatInt(int64(drive.MaxUploadSessionTTL.Seconds()), 10),
		"maximum: " + strconv.FormatInt(int64(drive.DefaultUploadSessionTTL.Seconds()), 10),
		"upload_sessions",
		"node_download",
		"node_range_download",
		"node_type_filter",
		"node_all_parents_search",
		"supported_node_types",
		"enum: [folder, file]",
		"node_sort_controls",
		"supported_node_sorts",
		"enum: [name, updated, created, size]",
		"copy_nodes",
		"share_links",
		"share_link_permissions",
		"enum: [view, download]",
		"max_share_link_ttl_seconds",
		"maximum: " + strconv.FormatInt(int64(drive.MaxShareLinkTTL.Seconds()), 10),
		"list_upload_sessions",
		"upload_session_body",
		"upload_session_checksum",
		"finalize_upload_sessions",
		"cancel_upload_sessions",
		"resumable_chunked_uploads",
	} {
		if !strings.Contains(block, want) {
			t.Fatalf("WebmailCapabilities Drive schema must document %q", want)
		}
	}
}

func TestOpenAPIDraftDocumentsWebmailCapabilityLimits(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("../../docs/openapi.yaml")
	if err != nil {
		t.Fatalf("read OpenAPI draft: %v", err)
	}
	block := extractOpenAPIComponentBlock(t, string(raw), "schemas", "WebmailCapabilities")
	for _, want := range []string{
		"maximum: " + strconv.Itoa(maildb.MessageListMaxLimit),
		"maximum: " + strconv.Itoa(maildb.BulkMessageMaxIDs),
		"maximum: " + strconv.Itoa(mailservice.MaxComposeRecipients),
		"maximum: " + strconv.Itoa(mailservice.MaxComposeSubjectBytes),
		"maximum: " + strconv.Itoa(mailservice.MaxComposeTextBodyBytes),
		"maximum: " + strconv.Itoa(mailservice.MaxComposeAttachments),
		"enum: [new, reply, forward]",
		"enum: [available]",
	} {
		if !strings.Contains(block, want) {
			t.Fatalf("WebmailCapabilities schema must document %q", want)
		}
	}
}

func TestOpenAPIDraftDocumentsAdminConsoleCapabilityLimits(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("../../docs/openapi.yaml")
	if err != nil {
		t.Fatalf("read OpenAPI draft: %v", err)
	}
	block := extractOpenAPIComponentBlock(t, string(raw), "schemas", "AdminConsoleCapabilities")
	for _, want := range []string{
		"maximum: " + strconv.Itoa(maildb.MessageListMaxLimit),
		"maximum: " + strconv.Itoa(maildb.AttachmentCleanupMaxLimit),
		"maximum: " + strconv.Itoa(maildb.APIUsageLedgerRetentionMaxLimit),
		"enum: [available]",
		"drive_upload_sessions",
		"drive_upload_cleanup",
		"drive_cleanup_failures",
		"api_usage_export",
		"imap_uid_backfill",
		"rejects_ambiguous_auth",
		"storage",
		"StorageBackendCapabilities",
	} {
		if !strings.Contains(block, want) {
			t.Fatalf("AdminConsoleCapabilities schema must document %q", want)
		}
	}
	storageBlock := extractOpenAPIComponentBlock(t, string(raw), "schemas", "StorageBackendCapabilities")
	for _, want := range []string{
		"configured_backend",
		"active_labels",
		"operations",
		"path_style_addressing",
		"endpoint_origin",
		"secrets_redacted",
		"supports_backend_switch",
	} {
		if !strings.Contains(storageBlock, want) {
			t.Fatalf("StorageBackendCapabilities schema must document %q", want)
		}
	}
	activeLabelsBlock := extractOpenAPIPropertyBlock(t, storageBlock, "active_labels")
	if strings.Contains(activeLabelsBlock, "enum: [local, nfs, s3, minio]") {
		t.Fatal("StorageBackendCapabilities.active_labels must remain an extensible token list, not a closed backend enum")
	}
	for _, want := range []string{"minItems: 1", "uniqueItems: true", "pattern: \"^[a-z0-9._-]{1,64}$\"", "sorted and de-duplicated"} {
		if !strings.Contains(activeLabelsBlock, want) {
			t.Fatalf("StorageBackendCapabilities.active_labels must document %q", want)
		}
	}
	operationsBlock := extractOpenAPIPropertyBlock(t, storageBlock, "operations")
	for _, want := range []string{"uniqueItems: true", "enum: [put, get, get_range, stat, copy, move, list, delete]"} {
		if !strings.Contains(operationsBlock, want) {
			t.Fatalf("StorageBackendCapabilities.operations must document %q", want)
		}
	}
}

func TestOpenAPIDraftDocumentsStableResponseEnvelopes(t *testing.T) {
	t.Parallel()

	operations := extractOpenAPIOperationBlocks(t, "../../docs/openapi.yaml")
	for route, responseRef := range map[string]string{
		"GET /webmail/capabilities":                                           "#/components/responses/WebmailCapabilities",
		"GET /console/capabilities":                                           "#/components/responses/AdminConsoleCapabilities",
		"GET /folders":                                                        "#/components/responses/FolderList",
		"POST /folders":                                                       "#/components/responses/Folder",
		"PATCH /folders/{id}":                                                 "#/components/responses/Folder",
		"DELETE /folders/{id}":                                                "#/components/responses/Status",
		"GET /drive/nodes":                                                    "#/components/responses/DriveNodeList",
		"GET /drive/nodes/{id}":                                               "#/components/responses/DriveNode",
		"HEAD /drive/nodes/{id}/download":                                     "",
		"GET /drive/nodes/{id}/download":                                      "",
		"GET /drive/usage":                                                    "#/components/responses/DriveUsageSummary",
		"POST /drive/folders":                                                 "#/components/responses/DriveNode",
		"GET /drive/upload-sessions":                                          "#/components/responses/DriveUploadSessionList",
		"POST /drive/upload-sessions":                                         "#/components/responses/DriveUploadSession",
		"GET /drive/upload-sessions/{id}":                                     "#/components/responses/DriveUploadSession",
		"DELETE /drive/upload-sessions/{id}":                                  "#/components/responses/DriveUploadSession",
		"PUT /drive/upload-sessions/{id}/body":                                "#/components/responses/DriveUploadSession",
		"POST /drive/upload-sessions/{id}/finalize":                           "#/components/responses/DriveNode",
		"POST /drive/files/finalize":                                          "#/components/responses/DriveNode",
		"PUT /drive/files/staged/{upload_id}/body":                            "#/components/responses/DriveStagedObject",
		"DELETE /drive/nodes/{id}":                                            "#/components/responses/DriveDelete",
		"POST /drive/nodes/{id}/trash":                                        "#/components/responses/DriveNodeUpdate",
		"POST /drive/nodes/{id}/restore":                                      "#/components/responses/DriveNodeUpdate",
		"PATCH /drive/nodes/{id}/name":                                        "#/components/responses/DriveNode",
		"PATCH /drive/nodes/{id}/parent":                                      "#/components/responses/DriveNode",
		"POST /drive/nodes/{id}/copy":                                         "#/components/responses/DriveNode",
		"POST /drive/nodes/{id}/share-links":                                  "#/components/responses/DriveShareLink",
		"GET /drive/share-links":                                              "#/components/responses/DriveShareLinkList",
		"GET /drive/share-links/{id}":                                         "#/components/responses/DriveSharedFile",
		"DELETE /drive/share-links/{id}":                                      "#/components/responses/DriveShareLink",
		"HEAD /drive/share-links/{id}/download":                               "",
		"GET /drive/share-links/{id}/download":                                "",
		"GET /mailbox/overview":                                               "#/components/responses/MailboxOverview",
		"GET /messages":                                                       "#/components/responses/MessageListPage",
		"GET /search":                                                         "#/components/responses/MessageListPage",
		"GET /messages/{id}":                                                  "#/components/responses/Message",
		"GET /messages/{id}/delivery-status":                                  "#/components/responses/MessageDeliveryStatus",
		"GET /threads":                                                        "#/components/responses/ThreadList",
		"GET /threads/{id}/messages":                                          "#/components/responses/MessageListPage",
		"PATCH /messages/{id}/flags":                                          "#/components/responses/Status",
		"PATCH /messages/{id}/folder":                                         "#/components/responses/Status",
		"PATCH /messages/bulk/flags":                                          "#/components/responses/BulkUpdate",
		"PATCH /threads/bulk/flags":                                           "#/components/responses/BulkUpdate",
		"PATCH /messages/bulk/folder":                                         "#/components/responses/BulkUpdate",
		"PATCH /threads/bulk/folder":                                          "#/components/responses/BulkUpdate",
		"POST /messages/bulk/delete":                                          "#/components/responses/BulkUpdate",
		"POST /threads/bulk/delete":                                           "#/components/responses/BulkUpdate",
		"DELETE /messages/{id}":                                               "#/components/responses/Status",
		"POST /messages/{id}/restore":                                         "#/components/responses/Status",
		"POST /messages/bulk/restore":                                         "#/components/responses/BulkUpdate",
		"POST /threads/bulk/restore":                                          "#/components/responses/BulkUpdate",
		"POST /messages/send":                                                 "#/components/responses/SendQueued",
		"POST /drafts":                                                        "#/components/responses/Draft",
		"GET /drafts/search":                                                  "#/components/responses/DraftList",
		"PATCH /drafts/{id}":                                                  "#/components/responses/Draft",
		"DELETE /drafts/{id}":                                                 "#/components/responses/Status",
		"POST /drafts/{id}/send":                                              "#/components/responses/SendQueued",
		"POST /attachments":                                                   "#/components/responses/Attachment",
		"POST /attachments/upload":                                            "#/components/responses/Attachment",
		"POST /attachments/upload-sessions":                                   "#/components/responses/AttachmentUploadSession",
		"GET /attachments/capabilities":                                       "#/components/responses/AttachmentUploadCapabilities",
		"DELETE /attachments/{id}":                                            "#/components/responses/Attachment",
		"GET /attachments/upload-sessions/{id}":                               "#/components/responses/AttachmentUploadSession",
		"DELETE /attachments/upload-sessions/{id}":                            "#/components/responses/AttachmentUploadSession",
		"PUT /attachments/upload-sessions/{id}/body":                          "#/components/responses/AttachmentUploadSession",
		"POST /attachments/upload-sessions/{id}/finalize":                     "#/components/responses/Attachment",
		"GET /messages/{id}/attachments":                                      "#/components/responses/AttachmentList",
		"HEAD /messages/{id}/attachments/{attachment_id}/download":            "",
		"GET /push-devices":                                                   "#/components/responses/PushDeviceList",
		"POST /push-devices":                                                  "#/components/responses/PushDevice",
		"DELETE /push-devices/{id}":                                           "#/components/responses/IDStatus",
		"POST /imap/mailboxes/{id}/uid-backfill":                              "#/components/responses/IMAPUIDBackfill",
		"GET /companies":                                                      "#/components/responses/CompanyList",
		"GET /companies/{id}":                                                 "#/components/responses/Company",
		"GET /domains":                                                        "#/components/responses/DomainList",
		"GET /domains/{id}":                                                   "#/components/responses/Domain",
		"GET /domains/{id}/stats":                                             "#/components/responses/DomainStats",
		"GET /domains/{id}/dns-check":                                         "#/components/responses/DomainDNSCheck",
		"GET /domains/{id}/dns-checks":                                        "#/components/responses/DomainDNSCheckHistory",
		"POST /domains":                                                       "#/components/responses/Domain",
		"PATCH /domains/{id}/status":                                          "#/components/responses/IDStatus",
		"PATCH /domains/{id}/quota":                                           "#/components/responses/IDStatus",
		"PATCH /domains/{id}/policy":                                          "#/components/responses/DomainPolicy",
		"GET /users":                                                          "#/components/responses/UserList",
		"GET /users/{id}":                                                     "#/components/responses/User",
		"POST /users":                                                         "#/components/responses/User",
		"PATCH /users/{id}/status":                                            "#/components/responses/IDStatus",
		"PATCH /users/{id}/quota":                                             "#/components/responses/IDStatus",
		"PATCH /users/{id}/password-hash":                                     "#/components/responses/IDStatus",
		"GET /queue":                                                          "#/components/responses/QueueStats",
		"GET /outbox-events":                                                  "#/components/responses/OutboxEventList",
		"GET /outbox-events/{id}":                                             "#/components/responses/OutboxEvent",
		"GET /audit-logs":                                                     "#/components/responses/AuditLogList",
		"GET /audit-logs/integrity":                                           "#/components/responses/AuditLogIntegrity",
		"GET /audit-logs/{id}":                                                "#/components/responses/AuditLog",
		"GET /companies/{id}/security/login-audits":                           "#/components/responses/LoginAuditList",
		"GET /directory/principals":                                           "#/components/responses/DirectoryPrincipalList",
		"GET /directory/aliases/resolve":                                      "#/components/responses/DirectoryAlias",
		"GET /directory/aliases":                                              "#/components/responses/DirectoryAliasList",
		"POST /directory/aliases":                                             "#/components/responses/DirectoryAlias",
		"DELETE /directory/aliases/{id}":                                      "#/components/responses/DirectoryAlias",
		"GET /directory/delegations":                                          "#/components/responses/DirectoryDelegationList",
		"POST /directory/delegations":                                         "#/components/responses/DirectoryDelegation",
		"DELETE /directory/delegations/{id}":                                  "#/components/responses/DirectoryDelegation",
		"PATCH /directory/delegations/{id}/role":                              "#/components/responses/DirectoryDelegation",
		"PATCH /directory/delegations/{id}/assignment":                        "#/components/responses/DirectoryDelegation",
		"GET /directory/group-memberships":                                    "#/components/responses/DirectoryGroupMembershipList",
		"POST /directory/group-memberships":                                   "#/components/responses/DirectoryGroupMembership",
		"DELETE /directory/group-memberships/{id}":                            "#/components/responses/DirectoryGroupMembership",
		"PATCH /directory/group-memberships/{id}/role":                        "#/components/responses/DirectoryGroupMembership",
		"PATCH /directory/group-memberships/{id}/assignment":                  "#/components/responses/DirectoryGroupMembership",
		"GET /backpressure":                                                   "#/components/responses/Backpressure",
		"PATCH /backpressure":                                                 "#/components/responses/Backpressure",
		"GET /quota-usage":                                                    "#/components/responses/QuotaUsageList",
		"GET /api-usage/daily":                                                "#/components/responses/APIUsageDailyList",
		"GET /api-usage/monthly":                                              "#/components/responses/APIUsageMonthlyList",
		"GET /api-usage/ledger":                                               "#/components/responses/APIUsageLedgerList",
		"GET /api-usage/ledger/export":                                        "#/components/responses/APIUsageLedgerExport",
		"GET /api-usage/ledger/stats":                                         "#/components/responses/APIUsageLedgerStats",
		"GET /api-usage/ledger/retention-readiness":                           "#/components/responses/APIUsageLedgerRetentionReadiness",
		"GET /api-usage/ledger/retention-runs":                                "#/components/responses/APIUsageLedgerRetentionRunList",
		"POST /api-usage/ledger/retention-runs":                               "#/components/responses/APIUsageLedgerRetentionRun",
		"GET /api-usage/ledger/retention-runs/{id}":                           "#/components/responses/APIUsageLedgerRetentionRun",
		"GET /dav-sync/retention-runs":                                        "#/components/responses/DAVSyncRetentionRunList",
		"POST /dav-sync/retention-runs":                                       "#/components/responses/DAVSyncRetentionRun",
		"GET /dav-sync/retention-readiness":                                   "#/components/responses/DAVSyncRetentionReadiness",
		"GET /dav-sync/retention-runs/{id}":                                   "#/components/responses/DAVSyncRetentionRun",
		"GET /api-usage/export-capabilities":                                  "#/components/responses/APIUsageExportCapabilities",
		"GET /api-usage/export-batches":                                       "#/components/responses/APIUsageExportBatchList",
		"POST /api-usage/export-batches":                                      "#/components/responses/APIUsageExportBatch",
		"GET /api-usage/export-batches/{id}":                                  "#/components/responses/APIUsageExportBatch",
		"GET /api-usage/export-batches/{id}/handoff-readiness":                "#/components/responses/APIUsageExportHandoffReadiness",
		"GET /api-usage/export-batches/{id}/export":                           "#/components/responses/APIUsageLedgerExport",
		"GET /api-usage/export-batches/{id}/artifacts":                        "#/components/responses/APIUsageExportArtifactList",
		"POST /api-usage/export-batches/{id}/artifacts":                       "#/components/responses/APIUsageExportArtifact",
		"GET /api-usage/export-batches/{id}/artifacts/{artifact_id}":          "#/components/responses/APIUsageExportArtifact",
		"POST /api-usage/export-batches/{id}/artifacts/write":                 "#/components/responses/APIUsageExportArtifact",
		"GET /api-usage/export-batches/{id}/artifacts/{artifact_id}/download": "#/components/responses/APIUsageExportArtifactDownload",
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
		"GET /attachment-upload-sessions":                                                                        "#/components/responses/AttachmentUploadSessionList",
		"GET /drive-upload-sessions":                                                                             "#/components/responses/DriveUploadSessionList",
		"GET /drive-nodes":                                                                                       "#/components/responses/DriveNodeList",
		"GET /drive-nodes/{id}":                                                                                  "#/components/responses/DriveNode",
		"GET /drive-usage":                                                                                       "#/components/responses/DriveUsageSummary",
		"POST /drive-upload-cleanup/candidates":                                                                  "#/components/responses/DriveUploadCleanupCandidates",
		"POST /drive-upload-cleanup/runs":                                                                        "#/components/responses/DriveUploadCleanupRun",
		"GET /drive-cleanup-failures":                                                                            "#/components/responses/DriveCleanupFailureList",
		"POST /drive-cleanup-failures/retry-runs":                                                                "#/components/responses/DriveCleanupRetryRun",
		"POST /drive-cleanup-failures/{id}/resolve":                                                              "#/components/responses/DriveCleanupFailure",
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

func TestOpenAPIDraftDocumentsDriveQuotaErrors(t *testing.T) {
	t.Parallel()

	operations := extractOpenAPIOperationBlocks(t, "../../docs/openapi.yaml")
	for _, route := range []string{
		"POST /drive/upload-sessions/{id}/finalize",
		"POST /drive/files/finalize",
		"POST /drive/nodes/{id}/copy",
	} {
		block, ok := operations[route]
		if !ok {
			t.Fatalf("OpenAPI operation %s is missing", route)
		}
		if !strings.Contains(block, `"507":`) {
			t.Fatalf("OpenAPI operation %s must document HTTP 507 for quota exhaustion", route)
		}
		if !strings.Contains(block, `$ref: "#/components/responses/Error"`) {
			t.Fatalf("OpenAPI operation %s must map HTTP 507 to the reusable Error response", route)
		}
	}
}

func TestOpenAPIDraftDocumentsUploadSessionChecksumHeader(t *testing.T) {
	t.Parallel()

	operations := extractOpenAPIOperationBlocks(t, "../../docs/openapi.yaml")
	for _, route := range []string{
		"PUT /attachments/upload-sessions/{id}/body",
		"PUT /drive/upload-sessions/{id}/body",
	} {
		block, ok := operations[route]
		if !ok {
			t.Fatalf("OpenAPI operation %s is missing", route)
		}
		for _, want := range []string{"X-Content-SHA256", "in: header", "^[0-9a-f]{64}$"} {
			if !strings.Contains(block, want) {
				t.Fatalf("upload session body operation %s must document checksum header detail %q", route, want)
			}
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

func TestOpenAPIDraftSaveDocumentsSendOptions(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("../../docs/openapi.yaml")
	if err != nil {
		t.Fatalf("read OpenAPI draft: %v", err)
	}
	draft := string(raw)
	compose := extractOpenAPIComponentBlock(t, draft, "schemas", "ComposeRequest")
	for _, want := range []string{
		"track_opens:",
		"scheduled_at:",
		"Draft saves preserve this option for draft-send.",
	} {
		if !strings.Contains(compose, want) {
			t.Fatalf("ComposeRequest must document draft-send option %q", want)
		}
	}

	draftSave := extractOpenAPIComponentBlock(t, draft, "requestBodies", "DraftSave")
	if !strings.Contains(draftSave, "#/components/schemas/ComposeRequest") {
		t.Fatalf("DraftSave must reuse ComposeRequest so draft-save options stay aligned")
	}

	operations := extractOpenAPIOperationBlocks(t, "../../docs/openapi.yaml")
	sendDraft := operations["POST /drafts/{id}/send"]
	if strings.Contains(sendDraft, "requestBody:") {
		t.Fatalf("POST /drafts/{id}/send must remain bodyless")
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
	for _, route := range []string{
		"HEAD /messages/{id}/attachments/{attachment_id}/download",
		"GET /messages/{id}/attachments/{attachment_id}/download",
		"HEAD /drive/nodes/{id}/download",
		"GET /drive/nodes/{id}/download",
		"HEAD /drive/share-links/{id}/download",
		"GET /drive/share-links/{id}/download",
	} {
		block, ok := operations[route]
		if !ok {
			t.Fatalf("OpenAPI operation %s is missing", route)
		}
		if strings.Contains(block, "application/json:") {
			t.Fatalf("%s must not declare application/json", route)
		}
		wants := []string{"Content-Disposition:", "Cache-Control:", "enum: [no-store]", "X-Content-Type-Options:", "enum: [nosniff]"}
		if !strings.HasPrefix(route, "HEAD ") {
			wants = append(wants, "application/octet-stream:", "type: string", "format: binary")
		}
		if route == "GET /drive/nodes/{id}/download" || route == "GET /drive/share-links/{id}/download" {
			wants = append(wants, "name: Range", "\"206\":", "Content-Range:", "Accept-Ranges:", "X-Gogomail-Drive-SHA256:", "^[0-9a-f]{64}$")
		}
		if route == "GET /drive/share-links/{id}/download" {
			wants = append(wants, "\"416\":", "Unsatisfied shared Drive file byte range", "^bytes \\\\*/[0-9]+$")
		}
		if route == "HEAD /drive/nodes/{id}/download" || route == "HEAD /drive/share-links/{id}/download" {
			wants = append(wants, "X-Gogomail-Drive-SHA256:", "^[0-9a-f]{64}$")
		}
		for _, want := range wants {
			if !strings.Contains(block, want) {
				t.Fatalf("%s must document %q", route, want)
			}
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
		"GET /messages":                                        {"limit", "cursor", "folder_id", "read", "starred", "has_attachment", "sort"},
		"GET /search":                                          {"limit", "cursor", "q", "folder_id", "from", "to", "cc", "bcc", "subject", "has_attachment", "sort", "include_rank", "include_highlights"},
		"GET /drafts/search":                                   {"limit", "cursor", "q", "from", "subject", "has_attachment"},
		"GET /drive/nodes":                                     {"limit", "parent_id", "status", "node_type", "q", "sort", "all_parents"},
		"GET /companies":                                       {"limit", "status"},
		"GET /domains":                                         {"limit", "company_id", "status", "dns_status"},
		"GET /domains/{id}/dns-checks":                         {"id", "limit", "status", "since"},
		"GET /users":                                           {"limit", "domain_id", "status", "password_configured"},
		"GET /quota-usage":                                     {"limit", "scope", "domain_id", "over_limit", "over_allocated"},
		"GET /drive/upload-sessions":                           {"limit", "status"},
		"GET /drive/share-links":                               {"limit", "node_id", "status"},
		"GET /attachment-upload-sessions":                      {"limit", "user_id", "draft_id", "status"},
		"GET /drive-upload-sessions":                           {"limit", "user_id", "status"},
		"GET /drive-nodes":                                     {"limit", "user_id", "parent_id", "status", "node_type", "q", "sort", "all_parents"},
		"GET /drive-nodes/{id}":                                {"id", "user_id", "status"},
		"GET /drive-usage":                                     {"user_id"},
		"GET /drive-cleanup-failures":                          {"limit", "user_id", "status"},
		"GET /delivery-attempts":                               {"limit", "status", "recipient_domain", "message_id", "farm", "sender", "since"},
		"GET /delivery-attempts/stats":                         {"status", "recipient_domain", "message_id", "farm", "sender", "since"},
		"GET /delivery-attempts/exhausted":                     {"limit", "recipient_domain", "message_id", "farm", "sender", "since"},
		"GET /push-notification-attempts":                      {"limit", "message_id", "status", "user_id", "platform", "device_id", "provider_status", "provider_message_id", "since"},
		"GET /push-notification-stats":                         {"message_id", "user_id", "platform", "device_id", "since"},
		"GET /push-devices":                                    {"limit"},
		"GET /outbox-events":                                   {"limit", "topic", "partition_key", "status", "since"},
		"GET /audit-logs":                                      {"limit", "category", "action", "action_prefix", "result", "target_type", "company_id", "domain_id", "user_id", "actor_id", "target_id", "since"},
		"GET /audit-logs/integrity":                            {"limit", "since"},
		"GET /companies/{id}/security/login-audits":           {"user_id", "success", "from_date", "to_date", "limit", "offset"},
		"GET /directory/principals":                            {"limit", "company_id", "domain_id", "organization_id", "kinds", "q", "active_only"},
		"GET /directory/aliases/resolve":                       {"address", "active_only"},
		"GET /directory/aliases":                               {"limit", "company_id", "domain_id", "target_kind", "target_id", "q", "active_only"},
		"GET /directory/delegations":                           {"limit", "company_id", "owner_kind", "owner_id", "delegate_kind", "delegate_id", "scope", "role", "active_only"},
		"GET /api-usage/daily":                                 {"limit", "tenant_id", "company_id", "domain_id", "user_id", "api_key_id", "principal_id", "auth_source", "method", "route", "status", "from", "to"},
		"GET /api-usage/monthly":                               {"limit", "tenant_id", "company_id", "domain_id", "user_id", "api_key_id", "principal_id", "auth_source", "method", "route", "status", "from", "to"},
		"GET /api-usage/ledger/retention-readiness":            {"cutoff", "tenant_id", "principal_id"},
		"GET /api-usage/ledger/retention-runs":                 {"limit", "tenant_id", "principal_id", "created_from", "created_to"},
		"GET /dav-sync/retention-runs":                         {"limit", "status", "created_from", "created_to"},
		"GET /dav-sync/retention-readiness":                    {"cutoff", "limit"},
		"GET /api-usage/export-batches":                        {"limit", "tenant_id", "principal_id", "status", "from", "to"},
		"POST /api-usage/export-batches":                       {"tenant_id", "principal_id", "from", "to"},
		"GET /api-usage/export-batches/{id}/handoff-readiness": {"id", "deep"},
		"GET /suppression-list":                                {"limit", "domain_id", "email", "reason"},
		"GET /trusted-relays":                                  {"limit", "cidr", "description"},
		"GET /delivery-routes":                                 {"limit", "status", "farm", "domain_pattern"},
		"GET /delivery-routes/resolve":                         {"domain"},
		"POST /imap/mailboxes/{id}/uid-backfill":               {"id", "user_id", "limit"},
		"GET /dkim-keys":                                       {"limit", "domain_id", "status"},
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
	block, ok = operations["GET /dav-sync/retention-readiness"]
	if !ok {
		t.Fatal("OpenAPI operation GET /dav-sync/retention-readiness is missing")
	}
	if !strings.Contains(block, "future cutoffs are rejected") {
		t.Fatalf("DAV sync retention-readiness cutoff parameter must document future-cutoff rejection, got:\n%s", block)
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
	for _, want := range []string{"#/components/parameters/Limit", "name: folder_id", "name: cursor", "name: read", "name: starred", "name: has_attachment", "name: sort"} {
		if !strings.Contains(block, want) {
			t.Fatalf("GET /threads must document %q, got:\n%s", want, block)
		}
	}
	for _, want := range []string{"newest", "oldest"} {
		if !strings.Contains(block, want) {
			t.Fatalf("GET /threads sort parameter must document %q, got:\n%s", want, block)
		}
	}
	for _, param := range []string{"tenant_id", "principal_id", "from", "to"} {
		if openAPIOperationDocumentsParameter(block, param) {
			t.Fatalf("GET /threads must not document API usage filter parameter %q", param)
		}
	}
}

func TestOpenAPIDraftDocumentsMessageListSortEnum(t *testing.T) {
	t.Parallel()

	operations := extractOpenAPIOperationBlocks(t, "../../docs/openapi.yaml")
	block, ok := operations["GET /messages"]
	if !ok {
		t.Fatal("OpenAPI operation GET /messages is missing")
	}
	for _, want := range []string{"name: sort", "newest", "oldest"} {
		if !strings.Contains(block, want) {
			t.Fatalf("GET /messages sort parameter must document %q, got:\n%s", want, block)
		}
	}
}

func TestOpenAPIDraftDocumentsMailboxPreviewFields(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("../../docs/openapi.yaml")
	if err != nil {
		t.Fatalf("read OpenAPI draft: %v", err)
	}
	draft := string(raw)
	for schema, wants := range map[string][]string{
		"MessageSummary": {"preview", "maxLength: 280", "Whitespace-normalized body preview"},
		"ThreadSummary":  {"preview", "maxLength: 280", "Preview for the latest message"},
	} {
		block := extractOpenAPIComponentBlock(t, draft, "schemas", schema)
		for _, want := range wants {
			if !strings.Contains(block, want) {
				t.Fatalf("OpenAPI %s must document %q, got:\n%s", schema, want, block)
			}
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
		"HEAD /messages/{id}/attachments/{attachment_id}/download":                                               {"#/components/parameters/PathID", "#/components/parameters/AttachmentID"},
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
		"PATCH /companies/{id}/quota":                        "#/components/requestBodies/CompanyQuotaUpdate",
		"PATCH /domains/{id}/quota":                          "#/components/requestBodies/DomainQuotaUpdate",
		"PATCH /users/{id}/quota":                            "#/components/requestBodies/UserQuotaUpdate",
		"POST /directory/aliases":                            "#/components/requestBodies/DirectoryAliasCreate",
		"POST /directory/delegations":                        "#/components/requestBodies/DirectoryDelegationCreate",
		"PATCH /directory/delegations/{id}/role":             "#/components/requestBodies/DirectoryDelegationRoleUpdate",
		"PATCH /directory/delegations/{id}/assignment":       "#/components/requestBodies/DirectoryDelegationReassign",
		"POST /directory/group-memberships":                  "#/components/requestBodies/DirectoryGroupMembershipCreate",
		"PATCH /directory/group-memberships/{id}/role":       "#/components/requestBodies/DirectoryGroupMembershipRoleUpdate",
		"PATCH /directory/group-memberships/{id}/assignment": "#/components/requestBodies/DirectoryGroupMembershipReassign",
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

func TestOpenAPIDraftDocumentsUserPasswordHashInput(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("../../docs/openapi.yaml")
	if err != nil {
		t.Fatalf("read OpenAPI draft: %v", err)
	}
	draft := string(raw)
	block := extractOpenAPIComponentBlock(t, draft, "schemas", "UserCreateRequest")
	for _, want := range []string{"password_hash:", "maxLength: 4096", "pbkdf2-sha256"} {
		if !strings.Contains(block, want) {
			t.Fatalf("UserCreateRequest does not document %q:\n%s", want, block)
		}
	}
	updateBlock := extractOpenAPIComponentBlock(t, draft, "schemas", "UserPasswordHashUpdateRequest")
	for _, want := range []string{"required: [password_hash]", "password_hash:", "maxLength: 4096"} {
		if !strings.Contains(updateBlock, want) {
			t.Fatalf("UserPasswordHashUpdateRequest does not document %q:\n%s", want, updateBlock)
		}
	}
}

func TestOpenAPIDraftDocumentsUserPasswordConfiguredReadModel(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("../../docs/openapi.yaml")
	if err != nil {
		t.Fatalf("read OpenAPI draft: %v", err)
	}
	draft := string(raw)
	block := extractOpenAPIComponentBlock(t, draft, "schemas", "User")
	for _, want := range []string{"password_configured", "type: boolean"} {
		if !strings.Contains(block, want) {
			t.Fatalf("User schema does not document %q:\n%s", want, block)
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
		"WebmailCapabilitiesEnvelope":                         "webmail_capabilities",
		"AdminConsoleCapabilitiesEnvelope":                    "admin_console_capabilities",
		"FolderListEnvelope":                                  "folders",
		"FolderEnvelope":                                      "folder",
		"MailboxOverviewEnvelope":                             "mailbox_overview",
		"MessageListPageEnvelope":                             "messages",
		"MessageListEnvelope":                                 "messages",
		"MessageEnvelope":                                     "message",
		"MessageDeliveryStatusEnvelope":                       "delivery_status",
		"ThreadListEnvelope":                                  "threads",
		"DraftEnvelope":                                       "draft",
		"DraftListEnvelope":                                   "drafts",
		"SendQueuedEnvelope":                                  "message",
		"AttachmentListEnvelope":                              "attachments",
		"AttachmentEnvelope":                                  "attachment",
		"AttachmentUploadSessionEnvelope":                     "attachment_upload_session",
		"AttachmentUploadSessionListEnvelope":                 "attachment_upload_sessions",
		"DriveUploadSessionListEnvelope":                      "drive_upload_sessions",
		"DriveUploadCleanupCandidatesEnvelope":                "drive_upload_cleanup_candidates",
		"DriveUploadCleanupRunEnvelope":                       "drive_upload_cleanup_run",
		"DriveUsageSummaryEnvelope":                           "drive_usage_summary",
		"DriveCleanupFailureListEnvelope":                     "drive_cleanup_failures",
		"DriveCleanupFailureEnvelope":                         "drive_cleanup_failure",
		"DriveCleanupRetryRunEnvelope":                        "drive_cleanup_retry_run",
		"AttachmentUploadCapabilitiesEnvelope":                "attachment_upload_capabilities",
		"PushDeviceListEnvelope":                              "push_devices",
		"PushDeviceEnvelope":                                  "push_device",
		"QueueStatsEnvelope":                                  "queues",
		"IMAPUIDBackfillEnvelope":                             "imap_uid_backfill",
		"OutboxEventListEnvelope":                             "outbox_events",
		"OutboxEventEnvelope":                                 "outbox_event",
		"AuditLogListEnvelope":                                "audit_logs",
		"AuditLogEnvelope":                                    "audit_log",
		"AuditLogIntegrityEnvelope":                           "audit_log_integrity",
		"DirectoryPrincipalListEnvelope":                      "directory_principals",
		"DirectoryAliasEnvelope":                              "directory_alias",
		"DirectoryAliasListEnvelope":                          "directory_aliases",
		"DirectoryDelegationEnvelope":                         "directory_delegation",
		"DirectoryGroupMembershipEnvelope":                    "directory_group_membership",
		"DirectoryDelegationListEnvelope":                     "directory_delegations",
		"DirectoryGroupMembershipListEnvelope":                "directory_group_memberships",
		"BackpressureEnvelope":                                "backpressure",
		"QuotaUsageListEnvelope":                              "quota_usage",
		"APIUsageDailyListEnvelope":                           "api_usage_daily",
		"APIUsageMonthlyListEnvelope":                         "api_usage_monthly",
		"APIUsageLedgerListEnvelope":                          "api_usage_ledger",
		"APIUsageLedgerStatsEnvelope":                         "api_usage_ledger_stats",
		"APIUsageLedgerRetentionReadinessEnvelope":            "api_usage_ledger_retention_readiness",
		"APIUsageLedgerRetentionRunEnvelope":                  "api_usage_ledger_retention_run",
		"APIUsageLedgerRetentionRunListEnvelope":              "api_usage_ledger_retention_runs",
		"DAVSyncRetentionRunEnvelope":                         "dav_sync_retention_run",
		"DAVSyncRetentionRunListEnvelope":                     "dav_sync_retention_runs",
		"DAVSyncRetentionReadinessEnvelope":                   "dav_sync_retention_readiness",
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
	for _, want := range []string{"Cache-Control:", "enum: [no-store]", "X-Content-Type-Options:", "enum: [nosniff]"} {
		if !strings.Contains(response, want) {
			t.Fatalf("OpenAPI Error response must document %q:\n%s", want, response)
		}
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

func TestOpenAPIDraftWiresMailUserIDFallbackParameter(t *testing.T) {
	t.Parallel()

	operations := extractOpenAPIOperationBlocks(t, "../../docs/openapi.yaml")
	for _, route := range []string{
		"GET /webmail/capabilities",
		"GET /folders",
		"POST /folders",
		"PATCH /folders/{id}",
		"DELETE /folders/{id}",
		"GET /mailbox/overview",
		"GET /messages",
		"GET /search",
		"GET /messages/{id}",
		"DELETE /messages/{id}",
		"GET /messages/{id}/delivery-status",
		"GET /threads",
		"GET /threads/{id}/messages",
		"PATCH /messages/{id}/flags",
		"PATCH /messages/{id}/folder",
		"PATCH /messages/bulk/flags",
		"PATCH /threads/bulk/flags",
		"PATCH /messages/bulk/folder",
		"PATCH /threads/bulk/folder",
		"POST /messages/bulk/delete",
		"POST /threads/bulk/delete",
		"POST /messages/{id}/restore",
		"POST /messages/bulk/restore",
		"POST /threads/bulk/restore",
		"GET /messages/{id}/attachments",
		"HEAD /messages/{id}/attachments/{attachment_id}/download",
		"GET /messages/{id}/attachments/{attachment_id}/download",
		"POST /messages/send",
		"POST /drafts",
		"GET /drafts/search",
		"PATCH /drafts/{id}",
		"DELETE /drafts/{id}",
		"POST /drafts/{id}/send",
		"POST /attachments",
		"POST /attachments/upload",
		"POST /attachments/upload-sessions",
		"GET /attachments/capabilities",
		"DELETE /attachments/{id}",
		"GET /attachments/upload-sessions/{id}",
		"DELETE /attachments/upload-sessions/{id}",
		"PUT /attachments/upload-sessions/{id}/body",
		"POST /attachments/upload-sessions/{id}/finalize",
		"GET /push-devices",
		"POST /push-devices",
		"DELETE /push-devices/{id}",
		"GET /drive/nodes",
		"GET /drive/nodes/{id}",
		"HEAD /drive/nodes/{id}/download",
		"GET /drive/nodes/{id}/download",
		"POST /drive/nodes/{id}/copy",
		"GET /drive/usage",
	} {
		block, ok := operations[route]
		if !ok {
			t.Fatalf("OpenAPI operation %s is missing", route)
		}
		if !strings.Contains(block, "#/components/parameters/UserIDFallback") {
			t.Fatalf("OpenAPI operation %s must document UserIDFallback:\n%s", route, block)
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

func extractOpenAPIPropertyBlock(t *testing.T, schemaBlock string, name string) string {
	t.Helper()

	marker := "        " + name + ":"
	var block strings.Builder
	inProperty := false
	for _, line := range strings.Split(schemaBlock, "\n") {
		if strings.HasPrefix(line, "        ") && !strings.HasPrefix(line, "          ") {
			if inProperty {
				return block.String()
			}
			inProperty = strings.TrimSuffix(strings.TrimSpace(line), ":") == name
		}
		if inProperty {
			block.WriteString(line)
			block.WriteByte('\n')
		}
	}
	if inProperty {
		return block.String()
	}
	t.Fatalf("OpenAPI property %s is missing", marker)
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

func extractRegisteredAdminRoutes(t *testing.T, filenames ...string) []string {
	t.Helper()

	pattern := regexp.MustCompile(`mux\.HandleFunc\("([A-Z]+) (/admin/v1/[^"]+)"`)
	var routes []string
	for _, filename := range filenames {
		raw, err := os.ReadFile(filename)
		if err != nil {
			t.Fatalf("read registered admin route source %s: %v", filename, err)
		}
		for _, match := range pattern.FindAllStringSubmatch(string(raw), -1) {
			routes = append(routes, match[1]+" "+normalizeOpenAPIPath(match[2]))
		}
	}
	sort.Strings(routes)
	return routes
}

func extractRegisteredMailRoutes(t *testing.T, filenames ...string) []string {
	t.Helper()

	pattern := regexp.MustCompile(`mux\.HandleFunc\("([A-Z]+) (/api/v1/[^"]+)"`)
	var routes []string
	for _, filename := range filenames {
		raw, err := os.ReadFile(filename)
		if err != nil {
			t.Fatalf("read registered mail route source %s: %v", filename, err)
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
	methodPattern := regexp.MustCompile(`^    (get|head|put|post|patch|delete):\s*$`)
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
	methodPattern := regexp.MustCompile(`^    (get|head|put|post|patch|delete):\s*$`)
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
