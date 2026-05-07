package httpapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/auth"
	"github.com/gogomail/gogomail/internal/drive"
	"github.com/gogomail/gogomail/internal/mail"
	"github.com/gogomail/gogomail/internal/maildb"
	"github.com/gogomail/gogomail/internal/mailservice"
	"github.com/gogomail/gogomail/internal/outbound"
	"github.com/gogomail/gogomail/internal/storage"
)

func TestListMessagesHandler(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{
		list: []maildb.MessageSummary{
			{
				ID:            "msg-1",
				Subject:       "hello",
				Preview:       "short body preview",
				FromAddr:      "sender@example.net",
				FromName:      "Sender",
				ReceivedAt:    time.Date(2026, 5, 3, 9, 0, 0, 0, time.UTC),
				Size:          123,
				HasAttachment: true,
				Read:          false,
				Starred:       true,
			},
		},
	}

	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/messages?user_id=user-1&limit=10", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var body struct {
		Messages []maildb.MessageSummary `json:"messages"`
		Limit    int                     `json:"limit"`
		HasMore  bool                    `json:"has_more"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if len(body.Messages) != 1 || body.Messages[0].Subject != "hello" {
		t.Fatalf("messages = %+v", body.Messages)
	}
	if body.Messages[0].Preview != "short body preview" {
		t.Fatalf("preview = %q", body.Messages[0].Preview)
	}
	if body.Limit != 10 || body.HasMore {
		t.Fatalf("page metadata = limit:%d has_more:%v", body.Limit, body.HasMore)
	}
	if service.lastUserID != "user-1" {
		t.Fatalf("lastUserID = %q", service.lastUserID)
	}
	if service.lastLimit != 10 {
		t.Fatalf("lastLimit = %d", service.lastLimit)
	}
}

func TestListMessagesHandlerUsesContractDefaultLimit(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{
		list: []maildb.MessageSummary{{ID: "msg-1", Subject: "hello"}},
	}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/messages?user_id=user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Limit int `json:"limit"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if body.Limit != maildb.MessageListDefaultLimit {
		t.Fatalf("response limit = %d, want %d", body.Limit, maildb.MessageListDefaultLimit)
	}
	if service.lastLimit != maildb.MessageListDefaultLimit {
		t.Fatalf("lastLimit = %d, want %d", service.lastLimit, maildb.MessageListDefaultLimit)
	}
}

func TestListMessagesHandlerSupportsReadAndStarredFilters(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/messages?user_id=user-1&read=false&starred=true&has_attachment=true&sort=oldest", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastListFilter.Read == nil || *service.lastListFilter.Read {
		t.Fatalf("read filter = %#v", service.lastListFilter.Read)
	}
	if service.lastListFilter.Starred == nil || !*service.lastListFilter.Starred {
		t.Fatalf("starred filter = %#v", service.lastListFilter.Starred)
	}
	if service.lastListFilter.HasAttachment == nil || !*service.lastListFilter.HasAttachment {
		t.Fatalf("has_attachment filter = %#v", service.lastListFilter.HasAttachment)
	}
	if service.lastListFilter.Sort != maildb.ListSortOldest {
		t.Fatalf("sort = %q", service.lastListFilter.Sort)
	}
}

func TestListMessagesHandlerRejectsInvalidReadAndStarredFilters(t *testing.T) {
	t.Parallel()

	tests := []string{
		"/api/v1/messages?user_id=user-1&read=maybe",
		"/api/v1/messages?user_id=user-1&starred=maybe",
		"/api/v1/messages?user_id=user-1&has_attachment=maybe",
		"/api/v1/messages?user_id=user-1&sort=sideways",
		"/api/v1/messages?user_id=user-1&sort=" + strings.Repeat("s", maxHTTPControlBytes+1),
		"/api/v1/messages?user_id=user-1&read=true&read=false",
	}
	for _, path := range tests {
		path := path
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			service := &fakeMessageService{}
			mux := http.NewServeMux()
			RegisterMailRoutes(mux, service, nil)

			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if service.lastUserID != "" {
				t.Fatalf("handler dispatched despite invalid filters: %+v", service.lastListFilter)
			}
		})
	}
}

func TestMailHandlersRejectDuplicateScalarQuery(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		path       string
		dispatched func(*fakeMessageService) bool
	}{
		{
			name: "duplicate user id",
			path: "/api/v1/folders?user_id=user-1&user_id=user-2",
			dispatched: func(service *fakeMessageService) bool {
				return service.lastUserID != ""
			},
		},
		{
			name: "duplicate limit",
			path: "/api/v1/search?user_id=user-1&limit=10&limit=20",
			dispatched: func(service *fakeMessageService) bool {
				return service.lastSearch.UserID != ""
			},
		},
		{
			name: "duplicate bool",
			path: "/api/v1/search?user_id=user-1&has_attachment=true&has_attachment=false",
			dispatched: func(service *fakeMessageService) bool {
				return service.lastSearch.UserID != ""
			},
		},
		{
			name: "duplicate cursor",
			path: "/api/v1/messages?user_id=user-1&cursor=a&cursor=b",
			dispatched: func(service *fakeMessageService) bool {
				return service.lastUserID != ""
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := &fakeMessageService{}
			mux := http.NewServeMux()
			RegisterMailRoutes(mux, service, nil)

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

func TestMailReadHandlersRejectUnknownQueryParameters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		path       string
		dispatched func(*fakeMessageService) bool
	}{
		{
			name: "webmail capabilities",
			path: "/api/v1/webmail/capabilities?user_id=user-1&deep=true",
			dispatched: func(service *fakeMessageService) bool {
				return service.lastUserID != ""
			},
		},
		{
			name: "folders",
			path: "/api/v1/folders?user_id=user-1&typo=true",
			dispatched: func(service *fakeMessageService) bool {
				return service.lastUserID != ""
			},
		},
		{
			name: "mailbox overview",
			path: "/api/v1/mailbox/overview?user_id=user-1&include_folders=true",
			dispatched: func(service *fakeMessageService) bool {
				return service.lastUserID != ""
			},
		},
		{
			name: "messages",
			path: "/api/v1/messages?user_id=user-1&folder_id=folder-1&unexpected=true",
			dispatched: func(service *fakeMessageService) bool {
				return service.lastUserID != ""
			},
		},
		{
			name: "search",
			path: "/api/v1/search?user_id=user-1&q=hello&includeRanks=true",
			dispatched: func(service *fakeMessageService) bool {
				return service.lastSearch.UserID != ""
			},
		},
		{
			name: "threads",
			path: "/api/v1/threads?user_id=user-1&page=2",
			dispatched: func(service *fakeMessageService) bool {
				return service.lastUserID != ""
			},
		},
		{
			name: "thread messages",
			path: "/api/v1/threads/thread-1/messages?user_id=user-1&page=2",
			dispatched: func(service *fakeMessageService) bool {
				return service.lastThreadID != ""
			},
		},
		{
			name: "delivery status",
			path: "/api/v1/messages/msg-1/delivery-status?user_id=user-1&expand=true",
			dispatched: func(service *fakeMessageService) bool {
				return service.lastMessageID != ""
			},
		},
		{
			name: "draft search",
			path: "/api/v1/drafts/search?user_id=user-1&q=hello&include_highlights=true",
			dispatched: func(service *fakeMessageService) bool {
				return service.lastDraftSearch.UserID != ""
			},
		},
		{
			name: "message attachments",
			path: "/api/v1/messages/msg-1/attachments?user_id=user-1&limit=5",
			dispatched: func(service *fakeMessageService) bool {
				return service.lastMessageID != ""
			},
		},
		{
			name: "attachment capabilities",
			path: "/api/v1/attachments/capabilities?user_id=user-1&deep=true",
			dispatched: func(service *fakeMessageService) bool {
				return service.lastUserID != ""
			},
		},
		{
			name: "upload session read",
			path: "/api/v1/attachments/upload-sessions/session-1?user_id=user-1&include_body=true",
			dispatched: func(service *fakeMessageService) bool {
				return service.lastGetUploadSessionID != ""
			},
		},
		{
			name: "attachment download",
			path: "/api/v1/messages/msg-1/attachments/att-1/download?user_id=user-1&filename=report.pdf",
			dispatched: func(service *fakeMessageService) bool {
				return service.lastMessageID != ""
			},
		},
		{
			name: "push devices",
			path: "/api/v1/push-devices?user_id=user-1&cursor=opaque",
			dispatched: func(service *fakeMessageService) bool {
				return service.lastUserID != "" || service.lastLimit != 0
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := &fakeMessageService{}
			mux := http.NewServeMux()
			RegisterMailRoutes(mux, service, nil)

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

func TestWebmailCapabilitiesHandler(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/webmail/capabilities?user_id=user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body webmailCapabilitiesEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	got := body.WebmailCapabilities
	if got.ContractVersion != BackendContractVersion {
		t.Fatalf("contract version = %q, want %q", got.ContractVersion, BackendContractVersion)
	}
	if got.Modules["mail"] != "available" || got.Modules["drive"] != "available" {
		t.Fatalf("modules = %#v", got.Modules)
	}
	if got.MaxListLimit != maildb.MessageListMaxLimit {
		t.Fatalf("max list limit = %d, want %d", got.MaxListLimit, maildb.MessageListMaxLimit)
	}
	if got.BulkActions.MaxMessageIDs != maildb.BulkMessageMaxIDs || !got.BulkActions.Flags || !got.BulkActions.ThreadFlags || !got.BulkActions.Move || !got.BulkActions.ThreadMove || !got.BulkActions.Delete || !got.BulkActions.ThreadDelete || !got.BulkActions.Restore || !got.BulkActions.ThreadRestore {
		t.Fatalf("bulk actions = %#v", got.BulkActions)
	}
	if got.Compose.MaxRecipients != mailservice.MaxComposeRecipients || got.Compose.MaxAttachmentIDs != mailservice.MaxComposeAttachments {
		t.Fatalf("compose caps = %#v", got.Compose)
	}
	wantSearchFilters := []string{"q", "folder_id", "from", "subject", "has_attachment"}
	if !slices.Equal(got.Search.Filters, wantSearchFilters) {
		t.Fatalf("search filters = %#v, want %#v", got.Search.Filters, wantSearchFilters)
	}
	if !got.Attachments.UploadSessions || got.Attachments.MaxAttachmentBytes != mailservice.MaxAttachmentUploadBytes {
		t.Fatalf("attachment caps = %#v", got.Attachments)
	}
	if !got.Drive.UploadSessions || !got.Drive.ListUploadSessions || !got.Drive.NodeNameSearch || !got.Drive.NodeAllParentsSearch || !got.Drive.NodeTypeFilter || len(got.Drive.SupportedNodeTypes) != 2 || got.Drive.SupportedNodeTypes[0] != drive.NodeTypeFolder || !got.Drive.NodeSortControls || len(got.Drive.SupportedNodeSorts) != 4 || got.Drive.SupportedNodeSorts[0] != drive.NodeSortName || !got.Drive.NodeDownload || !got.Drive.NodeRangeDownload || !got.Drive.CopyNodes || !got.Drive.ShareLinks || len(got.Drive.ShareLinkPermissions) != 2 || got.Drive.MaxShareLinkTTLSeconds != int64(drive.MaxShareLinkTTL.Seconds()) || !got.Drive.UsageSummary || !got.Drive.FinalizeUploadSessions || got.Drive.MaxUploadSessionBytes != drive.MaxUploadSessionBytes {
		t.Fatalf("drive caps = %#v", got.Drive)
	}
	if service.lastUserID != "" {
		t.Fatalf("capability read should not dispatch service calls, lastUserID = %q", service.lastUserID)
	}
}

func TestMailboxOverviewHandler(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{
		folders: []maildb.Folder{
			{ID: "inbox-id", SystemType: "inbox", Total: 7, Unread: 3, Starred: 1, TotalSize: 700},
			{ID: "sent-id", SystemType: "sent", Total: 2, Unread: 0, Starred: 1, TotalSize: 200},
			{ID: "project-id", Total: 5, Unread: 2, Starred: 0, TotalSize: 500},
		},
	}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/mailbox/overview?user_id=user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body mailboxOverviewEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	got := body.MailboxOverview
	if got.TotalMessages != 14 || got.UnreadMessages != 5 || got.StarredMessages != 2 || got.TotalSizeBytes != 1400 {
		t.Fatalf("overview = %#v", got)
	}
	if got.SystemFolders["inbox"] != "inbox-id" || got.SystemFolders["sent"] != "sent-id" {
		t.Fatalf("system folders = %#v", got.SystemFolders)
	}
	if service.lastUserID != "user-1" {
		t.Fatalf("lastUserID = %q", service.lastUserID)
	}
}

func TestMailMutationHandlersRejectUnknownQueryParameters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		dispatched func(*fakeMessageService) bool
	}{
		{
			name:   "create folder",
			method: http.MethodPost,
			path:   "/api/v1/folders?user_id=user-1&dry_run=true",
			body:   `{"name":"Projects"}`,
			dispatched: func(service *fakeMessageService) bool {
				return service.lastUserID != ""
			},
		},
		{
			name:   "flag message",
			method: http.MethodPatch,
			path:   "/api/v1/messages/msg-1/flags?user_id=user-1&expand=true",
			body:   `{"flag":"read","value":true}`,
			dispatched: func(service *fakeMessageService) bool {
				return service.lastMessageID != ""
			},
		},
		{
			name:   "delete message",
			method: http.MethodDelete,
			path:   "/api/v1/messages/msg-1?user_id=user-1&force=true",
			dispatched: func(service *fakeMessageService) bool {
				return service.lastDeletedID != ""
			},
		},
		{
			name:   "save draft",
			method: http.MethodPost,
			path:   "/api/v1/drafts?user_id=user-1&draft=true",
			body:   `{"subject":"draft","text_body":"body"}`,
			dispatched: func(service *fakeMessageService) bool {
				return service.lastDraft.UserID != ""
			},
		},
		{
			name:   "reserve attachment",
			method: http.MethodPost,
			path:   "/api/v1/attachments?user_id=user-1&filename=report.pdf",
			body:   `{"filename":"report.pdf","size":42,"mime_type":"application/pdf"}`,
			dispatched: func(service *fakeMessageService) bool {
				return service.lastAttachmentUpload.UserID != ""
			},
		},
		{
			name:   "send message",
			method: http.MethodPost,
			path:   "/api/v1/messages/send?user_id=user-1&preview=true",
			body:   `{"to":[{"email":"recipient@example.net"}],"subject":"hello","text_body":"body"}`,
			dispatched: func(service *fakeMessageService) bool {
				return service.lastSend.UserID != ""
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := &fakeMessageService{}
			mux := http.NewServeMux()
			RegisterMailRoutes(mux, service, nil)

			var body io.Reader
			if tt.body != "" {
				body = strings.NewReader(tt.body)
			}
			req := httptest.NewRequest(tt.method, tt.path, body)
			if tt.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
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

func TestMailBodylessHandlersRejectPayloadMetadata(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		method      string
		path        string
		body        string
		contentType string
		dispatched  func(*fakeMessageService) bool
	}{
		{
			name:   "folder list body",
			method: http.MethodGet,
			path:   "/api/v1/folders?user_id=user-1",
			body:   `{}`,
			dispatched: func(service *fakeMessageService) bool {
				return service.lastUserID != ""
			},
		},
		{
			name:        "message list content type",
			method:      http.MethodGet,
			path:        "/api/v1/messages?user_id=user-1",
			contentType: "application/json",
			dispatched: func(service *fakeMessageService) bool {
				return service.lastUserID != ""
			},
		},
		{
			name:   "message detail body",
			method: http.MethodGet,
			path:   "/api/v1/messages/msg-1?user_id=user-1",
			body:   `{}`,
			dispatched: func(service *fakeMessageService) bool {
				return service.lastMessageID != ""
			},
		},
		{
			name:   "delete message body",
			method: http.MethodDelete,
			path:   "/api/v1/messages/msg-1?user_id=user-1",
			body:   `{}`,
			dispatched: func(service *fakeMessageService) bool {
				return service.lastDeletedID != ""
			},
		},
		{
			name:        "draft send content type",
			method:      http.MethodPost,
			path:        "/api/v1/drafts/draft-1/send?user_id=user-1",
			contentType: "application/json",
			dispatched: func(service *fakeMessageService) bool {
				return service.lastDeletedDraftID != ""
			},
		},
		{
			name:   "attachment capabilities body",
			method: http.MethodGet,
			path:   "/api/v1/attachments/capabilities?user_id=user-1",
			body:   `{}`,
			dispatched: func(service *fakeMessageService) bool {
				return service.lastUserID != ""
			},
		},
		{
			name:   "upload session read body",
			method: http.MethodGet,
			path:   "/api/v1/attachments/upload-sessions/session-1?user_id=user-1",
			body:   `{}`,
			dispatched: func(service *fakeMessageService) bool {
				return service.lastGetUploadSessionID != ""
			},
		},
		{
			name:   "upload session finalize body",
			method: http.MethodPost,
			path:   "/api/v1/attachments/upload-sessions/session-1/finalize?user_id=user-1",
			body:   `{}`,
			dispatched: func(service *fakeMessageService) bool {
				return service.lastFinalizeUploadSessionID != ""
			},
		},
		{
			name:   "attachment download body",
			method: http.MethodGet,
			path:   "/api/v1/messages/msg-1/attachments/att-1/download?user_id=user-1",
			body:   `{}`,
			dispatched: func(service *fakeMessageService) bool {
				return service.lastMessageID != ""
			},
		},
		{
			name:        "push device list content type",
			method:      http.MethodGet,
			path:        "/api/v1/push-devices?user_id=user-1",
			contentType: "application/json",
			dispatched: func(service *fakeMessageService) bool {
				return service.lastUserID != ""
			},
		},
		{
			name:   "push device delete body",
			method: http.MethodDelete,
			path:   "/api/v1/push-devices/device-1?user_id=user-1",
			body:   `{}`,
			dispatched: func(service *fakeMessageService) bool {
				return service.lastDeletePushDeviceID != ""
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := &fakeMessageService{}
			mux := http.NewServeMux()
			RegisterMailRoutes(mux, service, nil)

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

func TestListFoldersHandler(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{
		folders: []maildb.Folder{
			{ID: "folder-1", Name: "Inbox", FullPath: "Inbox", Type: "system", SystemType: "inbox", Starred: 2},
		},
	}

	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/folders?user_id=user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var body struct {
		Folders []maildb.Folder `json:"folders"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if len(body.Folders) != 1 || body.Folders[0].SystemType != "inbox" {
		t.Fatalf("folders = %+v", body.Folders)
	}
	if body.Folders[0].Starred != 2 {
		t.Fatalf("starred = %d", body.Folders[0].Starred)
	}
	if service.lastUserID != "user-1" {
		t.Fatalf("lastUserID = %q", service.lastUserID)
	}
}

func TestMailJSONHandlersRejectTrailingTokens(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/folders?user_id=user-1", strings.NewReader(`{"name":"Projects"} {"name":"Ignored"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastFolderName != "" {
		t.Fatalf("handler should not dispatch trailing-token body, created folder %q", service.lastFolderName)
	}
}

func TestMailJSONHandlersRejectUnknownFields(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/folders?user_id=user-1", strings.NewReader(`{"name":"Projects","unexpected":true}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastFolderName != "" {
		t.Fatalf("handler should not dispatch unknown-field body, created folder %q", service.lastFolderName)
	}
}

func TestMailJSONHandlersRejectMissingOrNonJSONContentType(t *testing.T) {
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

			service := &fakeMessageService{}
			mux := http.NewServeMux()
			RegisterMailRoutes(mux, service, nil)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/folders?user_id=user-1", strings.NewReader(`{"name":"Projects"}`))
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
			if service.lastFolderName != "" {
				t.Fatalf("handler should not dispatch non-json content type, created folder %q", service.lastFolderName)
			}
		})
	}
}

func TestListThreadsHandler(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{
		threads: []maildb.ThreadSummary{{
			ID:              "thread-1",
			Subject:         "hello",
			Preview:         "latest body preview",
			MessageCount:    2,
			UnreadCount:     1,
			LatestMessageID: "msg-2",
			LatestFromAddr:  "sender@example.net",
		}},
	}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/threads?user_id=user-1&limit=10", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Threads []maildb.ThreadSummary `json:"threads"`
		Limit   int                    `json:"limit"`
		HasMore bool                   `json:"has_more"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if len(body.Threads) != 1 || body.Threads[0].ID != "thread-1" {
		t.Fatalf("threads = %+v", body.Threads)
	}
	if body.Threads[0].Preview != "latest body preview" {
		t.Fatalf("thread preview = %q", body.Threads[0].Preview)
	}
	if body.Limit != 10 || body.HasMore {
		t.Fatalf("thread page metadata = limit:%d has_more:%v", body.Limit, body.HasMore)
	}
	if service.lastUserID != "user-1" || service.lastLimit != 10 {
		t.Fatalf("lastUserID=%q lastLimit=%d", service.lastUserID, service.lastLimit)
	}
}

func TestListThreadsHandlerUsesContractDefaultLimit(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{
		threads: []maildb.ThreadSummary{{ID: "thread-1", Subject: "hello"}},
	}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/threads?user_id=user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Limit int `json:"limit"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if body.Limit != maildb.MessageListDefaultLimit {
		t.Fatalf("response limit = %d, want %d", body.Limit, maildb.MessageListDefaultLimit)
	}
	if service.lastLimit != maildb.MessageListDefaultLimit {
		t.Fatalf("lastLimit = %d, want %d", service.lastLimit, maildb.MessageListDefaultLimit)
	}
}

func TestListThreadsHandlerSupportsReadAndStarredFilters(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/threads?user_id=user-1&folder_id=folder-1&read=false&starred=true&has_attachment=true&sort=oldest", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastThreadFilter.FolderID != "folder-1" {
		t.Fatalf("folder filter = %q", service.lastThreadFilter.FolderID)
	}
	if service.lastThreadFilter.Read == nil || *service.lastThreadFilter.Read {
		t.Fatalf("read filter = %#v", service.lastThreadFilter.Read)
	}
	if service.lastThreadFilter.Starred == nil || !*service.lastThreadFilter.Starred {
		t.Fatalf("starred filter = %#v", service.lastThreadFilter.Starred)
	}
	if service.lastThreadFilter.HasAttachment == nil || !*service.lastThreadFilter.HasAttachment {
		t.Fatalf("has_attachment filter = %#v", service.lastThreadFilter.HasAttachment)
	}
	if service.lastThreadFilter.Sort != maildb.ListSortOldest {
		t.Fatalf("sort = %q", service.lastThreadFilter.Sort)
	}
}

func TestListThreadsHandlerRejectsInvalidReadAndStarredFilters(t *testing.T) {
	t.Parallel()

	tests := []string{
		"/api/v1/threads?user_id=user-1&read=maybe",
		"/api/v1/threads?user_id=user-1&starred=maybe",
		"/api/v1/threads?user_id=user-1&has_attachment=maybe",
		"/api/v1/threads?user_id=user-1&folder_id=folder%0Abad",
		"/api/v1/threads?user_id=user-1&sort=sideways",
		"/api/v1/threads?user_id=user-1&sort=" + strings.Repeat("s", maxHTTPControlBytes+1),
		"/api/v1/threads?user_id=user-1&starred=true&starred=false",
	}
	for _, path := range tests {
		path := path
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			service := &fakeMessageService{}
			mux := http.NewServeMux()
			RegisterMailRoutes(mux, service, nil)

			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if service.lastUserID != "" {
				t.Fatalf("handler dispatched despite invalid filters: %+v", service.lastThreadFilter)
			}
		})
	}
}

func TestSearchMessagesHandler(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{
		list: []maildb.MessageSummary{{ID: "msg-1", Subject: "hello search"}},
	}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/search?user_id=user-1&q=%20hello%20&folder_id=%20folder-1%20&from=%20sender%20&subject=%20search%20&has_attachment=true&limit=10", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Messages []maildb.MessageSummary `json:"messages"`
		Limit    int                     `json:"limit"`
		HasMore  bool                    `json:"has_more"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if len(body.Messages) != 1 || body.Messages[0].ID != "msg-1" {
		t.Fatalf("messages = %+v", body.Messages)
	}
	if service.lastSearch.UserID != "user-1" || service.lastSearch.Query != "hello" || service.lastSearch.FolderID != "folder-1" || service.lastSearch.From != "sender" || service.lastSearch.Subject != "search" {
		t.Fatalf("lastSearch = %+v", service.lastSearch)
	}
	if service.lastSearch.HasAttachment == nil || !*service.lastSearch.HasAttachment {
		t.Fatalf("HasAttachment = %+v", service.lastSearch.HasAttachment)
	}
}

func TestSearchMessagesHandlerUsesContractDefaultLimit(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{
		list: []maildb.MessageSummary{{ID: "msg-1", Subject: "hello search"}},
	}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/search?user_id=user-1&q=hello", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastSearch.Limit != maildb.MessageListDefaultLimit {
		t.Fatalf("search limit = %d, want %d", service.lastSearch.Limit, maildb.MessageListDefaultLimit)
	}
}

func TestSearchMessagesHandlerPassesRankingOptions(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{
		list: []maildb.MessageSummary{{
			ID:         "msg-1",
			Subject:    "hello search",
			SearchRank: ptrFloat64(0.42),
			SearchHighlights: &maildb.MessageSearchHighlights{
				Subject: []string{"<mark>hello</mark> search"},
			},
		}},
	}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/search?user_id=user-1&q=hello&sort=relevance&include_rank=true&include_highlights=true", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastSearch.Sort != maildb.MessageSearchSortRelevance || !service.lastSearch.IncludeRank || !service.lastSearch.IncludeHighlights {
		t.Fatalf("lastSearch = %+v", service.lastSearch)
	}
	if !strings.Contains(rec.Body.String(), "search_rank") || !strings.Contains(rec.Body.String(), "search_highlights") {
		t.Fatalf("response did not include search metadata: %s", rec.Body.String())
	}
}

func TestSearchMessagesHandlerRejectsInvalidHasAttachment(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/search?user_id=user-1&has_attachment=maybe", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestSearchMessagesHandlerRejectsUnsafeFilters(t *testing.T) {
	t.Parallel()

	tests := []string{
		"/api/v1/search?user_id=user-1&q=hello%0Abad",
		"/api/v1/search?user_id=user-1&folder_id=" + strings.Repeat("f", maxHTTPResourceIDBytes+1),
		"/api/v1/search?user_id=user-1&from=" + strings.Repeat("s", maxHTTPQueryBytes+1),
		"/api/v1/search?user_id=user-1&subject=receipt%0Dbad",
	}
	for _, target := range tests {
		target := target
		t.Run(target, func(t *testing.T) {
			t.Parallel()

			service := &fakeMessageService{}
			mux := http.NewServeMux()
			RegisterMailRoutes(mux, service, nil)

			req := httptest.NewRequest(http.MethodGet, target, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if service.lastSearch.UserID != "" {
				t.Fatalf("lastSearch = %+v", service.lastSearch)
			}
		})
	}
}

func TestSearchMessagesHandlerRejectsInvalidRankingOptions(t *testing.T) {
	t.Parallel()

	tests := []string{
		"/api/v1/search?user_id=user-1&sort=popular",
		"/api/v1/search?user_id=user-1&include_rank=maybe",
		"/api/v1/search?user_id=user-1&include_highlights=maybe",
		"/api/v1/search?user_id=user-1&sort=" + strings.Repeat("s", maxHTTPControlBytes+1),
		"/api/v1/search?user_id=user-1&include_rank=true%0Abad",
	}
	for _, target := range tests {
		target := target
		t.Run(target, func(t *testing.T) {
			t.Parallel()

			service := &fakeMessageService{}
			mux := http.NewServeMux()
			RegisterMailRoutes(mux, service, nil)

			req := httptest.NewRequest(http.MethodGet, target, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestMailRoutesRejectOversizedLimit(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/messages?user_id=user-1&limit="+strings.Repeat("9", maxHTTPControlBytes+1), nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastUserID != "" {
		t.Fatalf("handler should not dispatch oversized limit, lastUserID = %q", service.lastUserID)
	}
}

func ptrFloat64(v float64) *float64 {
	return &v
}

func TestListThreadMessagesHandler(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{
		list: []maildb.MessageSummary{{ID: "msg-1", Subject: "hello"}},
	}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/threads/thread-1/messages?user_id=user-1&limit=20", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Messages []maildb.MessageSummary `json:"messages"`
		Limit    int                     `json:"limit"`
		HasMore  bool                    `json:"has_more"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if len(body.Messages) != 1 || service.lastThreadID != "thread-1" {
		t.Fatalf("messages = %+v lastThreadID=%q", body.Messages, service.lastThreadID)
	}
	if body.Limit != 20 || body.HasMore {
		t.Fatalf("thread message page metadata = limit:%d has_more:%v", body.Limit, body.HasMore)
	}
	if service.lastUserID != "user-1" || service.lastLimit != 20 {
		t.Fatalf("lastUserID=%q lastLimit=%d", service.lastUserID, service.lastLimit)
	}
}

func TestListThreadMessagesHandlerUsesContractDefaultLimit(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{
		list: []maildb.MessageSummary{{ID: "msg-1", Subject: "hello"}},
	}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/threads/thread-1/messages?user_id=user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Limit int `json:"limit"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if body.Limit != maildb.MessageListDefaultLimit {
		t.Fatalf("response limit = %d, want %d", body.Limit, maildb.MessageListDefaultLimit)
	}
	if service.lastThreadID != "thread-1" {
		t.Fatalf("lastThreadID = %q", service.lastThreadID)
	}
	if service.lastLimit != maildb.MessageListDefaultLimit {
		t.Fatalf("lastLimit = %d, want %d", service.lastLimit, maildb.MessageListDefaultLimit)
	}
}

func TestThreadHandlersRejectInvalidCursor(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	for _, path := range []string{
		"/api/v1/threads?user_id=user-1&cursor=not-base64",
		"/api/v1/threads/thread-1/messages?user_id=user-1&cursor=not-base64",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s status = %d, body = %s", path, rec.Code, rec.Body.String())
		}
	}
}

func TestCreateFolderHandler(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/folders?user_id=user-1", strings.NewReader(`{"name":"Projects"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Folder maildb.Folder `json:"folder"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if body.Folder.Name != "Projects" || body.Folder.Type != "user" {
		t.Fatalf("folder = %+v", body.Folder)
	}
}

func TestCreateFolderHandlerRejectsOversizedJSONBody(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	body := `{"name":"` + strings.Repeat("a", maxJSONBodyBytes) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/folders?user_id=user-1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastFolderName != "" {
		t.Fatalf("handler should not dispatch oversized body, created folder %q", service.lastFolderName)
	}
}

func TestRenameFolderHandler(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/folders/folder-1?user_id=user-1", strings.NewReader(`{"name":"Renamed"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastFolderID != "folder-1" || service.lastFolderName != "Renamed" {
		t.Fatalf("rename = folder:%q name:%q", service.lastFolderID, service.lastFolderName)
	}
}

func TestBulkSetMessageFlagsHandler(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/messages/bulk/flags?user_id=user-1", strings.NewReader(`{
		"message_ids":["msg-1","msg-2"],
		"flag":"read",
		"value":true
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastBulkFlag.UserID != "user-1" || service.lastBulkFlag.Flag != "read" || !service.lastBulkFlag.Value {
		t.Fatalf("lastBulkFlag = %+v", service.lastBulkFlag)
	}
	if len(service.lastBulkFlag.MessageIDs) != 2 {
		t.Fatalf("message ids = %+v", service.lastBulkFlag.MessageIDs)
	}
}

func TestBulkSetThreadFlagsHandler(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/threads/bulk/flags?user_id=user-1", strings.NewReader(`{
		"thread_ids":["thread-1","thread-2"],
		"flag":"read",
		"value":true
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastBulkThreadFlag.UserID != "user-1" || service.lastBulkThreadFlag.Flag != "read" || !service.lastBulkThreadFlag.Value {
		t.Fatalf("lastBulkThreadFlag = %+v", service.lastBulkThreadFlag)
	}
	if len(service.lastBulkThreadFlag.ThreadIDs) != 2 {
		t.Fatalf("thread ids = %+v", service.lastBulkThreadFlag.ThreadIDs)
	}
}

func TestBulkMoveMessagesHandler(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/messages/bulk/folder?user_id=user-1", strings.NewReader(`{
		"message_ids":["msg-1","msg-2"],
		"folder_id":"folder-archive"
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastBulkMove.UserID != "user-1" || service.lastBulkMove.FolderID != "folder-archive" {
		t.Fatalf("lastBulkMove = %+v", service.lastBulkMove)
	}
	if len(service.lastBulkMove.MessageIDs) != 2 {
		t.Fatalf("message ids = %+v", service.lastBulkMove.MessageIDs)
	}
}

func TestBulkMoveThreadsHandler(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/threads/bulk/folder?user_id=user-1", strings.NewReader(`{
		"thread_ids":["thread-1","thread-2"],
		"folder_id":"folder-archive"
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastBulkThreadMove.UserID != "user-1" || service.lastBulkThreadMove.FolderID != "folder-archive" {
		t.Fatalf("lastBulkThreadMove = %+v", service.lastBulkThreadMove)
	}
	if len(service.lastBulkThreadMove.ThreadIDs) != 2 {
		t.Fatalf("thread ids = %+v", service.lastBulkThreadMove.ThreadIDs)
	}
}

func TestBulkDeleteMessagesHandler(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/messages/bulk/delete?user_id=user-1", strings.NewReader(`{
		"message_ids":["msg-1","msg-2"]
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastBulkDelete.UserID != "user-1" || len(service.lastBulkDelete.MessageIDs) != 2 {
		t.Fatalf("lastBulkDelete = %+v", service.lastBulkDelete)
	}
}

func TestBulkDeleteThreadsHandler(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/threads/bulk/delete?user_id=user-1", strings.NewReader(`{
		"thread_ids":["thread-1","thread-2"]
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastBulkThreadDelete.UserID != "user-1" || len(service.lastBulkThreadDelete.ThreadIDs) != 2 {
		t.Fatalf("lastBulkThreadDelete = %+v", service.lastBulkThreadDelete)
	}
}

func TestDeleteFolderHandler(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/folders/folder-1?user_id=user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastDeletedFolderID != "folder-1" {
		t.Fatalf("lastDeletedFolderID = %q", service.lastDeletedFolderID)
	}
}

func TestListMessagesHandlerFiltersByFolder(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/messages?user_id=user-1&folder_id=folder-1&limit=25", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastFolderID != "folder-1" {
		t.Fatalf("lastFolderID = %q", service.lastFolderID)
	}
	if service.lastLimit != 25 {
		t.Fatalf("lastLimit = %d", service.lastLimit)
	}
}

func TestListMessagesHandlerRejectsUnsafeFolderID(t *testing.T) {
	t.Parallel()

	tests := []string{
		"/api/v1/messages?user_id=user-1&folder_id=folder%0Abad",
		"/api/v1/messages?user_id=user-1&folder_id=" + strings.Repeat("f", maxHTTPResourceIDBytes+1),
	}

	for _, target := range tests {
		target := target
		t.Run(target, func(t *testing.T) {
			t.Parallel()

			service := &fakeMessageService{}
			mux := http.NewServeMux()
			RegisterMailRoutes(mux, service, nil)

			req := httptest.NewRequest(http.MethodGet, target, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if service.lastFolderID != "" {
				t.Fatalf("lastFolderID = %q", service.lastFolderID)
			}
		})
	}
}

func TestListMessagesHandlerRejectsInvalidCursor(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/messages?user_id=user-1&cursor=bad", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
			Status  int    `json:"status"`
		} `json:"error"`
		ErrorMessage string `json:"error_message"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if body.Error.Code != "bad_request" || body.Error.Status != http.StatusBadRequest || body.Error.Message == "" {
		t.Fatalf("error envelope = %+v", body.Error)
	}
	if body.ErrorMessage != body.Error.Message {
		t.Fatalf("error_message = %q, want %q", body.ErrorMessage, body.Error.Message)
	}
}

func TestListMessagesHandlerRejectsInvalidLimit(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/messages?user_id=user-1&limit=abc", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "limit must be an integer") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestListMessagesHandlerRejectsNegativeLimit(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/messages?user_id=user-1&limit=-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "limit must be positive") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestListMessagesHandlerRejectsTooLargeLimit(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/messages?user_id=user-1&limit=201", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "limit must be at most 200") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestGetMessageHandler(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{
		detail: maildb.MessageDetail{
			ID:          "msg-1",
			Subject:     "hello",
			FromAddr:    "sender@example.net",
			FromName:    "Sender",
			ToAddrs:     json.RawMessage(`[{"name":"Admin","address":"admin@example.com"}]`),
			StoragePath: "mailstore/example.eml",
			TextBody:    "body",
		},
	}

	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/messages/msg-1?user_id=user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var body struct {
		Message maildb.MessageDetail `json:"message"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if body.Message.ID != "msg-1" || body.Message.TextBody != "body" {
		t.Fatalf("message = %+v", body.Message)
	}
	if service.lastMessageID != "msg-1" {
		t.Fatalf("lastMessageID = %q", service.lastMessageID)
	}
}

func TestSetMessageFlagHandler(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/messages/msg-1/flags?user_id=user-1", strings.NewReader(`{"flag":"read","value":true}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastMessageID != "msg-1" || service.lastFlag != "read" || !service.lastFlagValue {
		t.Fatalf("flag update = id:%q flag:%q value:%v", service.lastMessageID, service.lastFlag, service.lastFlagValue)
	}
}

func TestMoveMessageHandler(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/messages/msg-1/folder?user_id=user-1", strings.NewReader(`{"folder_id":"folder-2"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastMessageID != "msg-1" || service.lastMoveFolderID != "folder-2" {
		t.Fatalf("move = id:%q folder:%q", service.lastMessageID, service.lastMoveFolderID)
	}
}

func TestDeleteMessageHandler(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/messages/msg-1?user_id=user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastDeletedID != "msg-1" {
		t.Fatalf("lastDeletedID = %q", service.lastDeletedID)
	}
}

func TestRestoreMessageHandler(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/messages/msg-1/restore?user_id=user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastRestoredID != "msg-1" {
		t.Fatalf("lastRestoredID = %q", service.lastRestoredID)
	}
}

func TestBulkRestoreMessagesHandler(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/messages/bulk/restore?user_id=user-1", strings.NewReader(`{
		"message_ids":["msg-1","msg-2"]
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastBulkRestore.UserID != "user-1" || len(service.lastBulkRestore.MessageIDs) != 2 {
		t.Fatalf("lastBulkRestore = %+v", service.lastBulkRestore)
	}
}

func TestBulkRestoreThreadsHandler(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/threads/bulk/restore?user_id=user-1", strings.NewReader(`{
		"thread_ids":["thread-1","thread-2"]
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastBulkThreadRestore.UserID != "user-1" || len(service.lastBulkThreadRestore.ThreadIDs) != 2 {
		t.Fatalf("lastBulkThreadRestore = %+v", service.lastBulkThreadRestore)
	}
}

func TestSaveDraftHandler(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/drafts", strings.NewReader(`{
		"user_id":"user-1",
		"subject":"draft",
		"text_body":"body"
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastDraft.UserID != "user-1" || service.lastDraft.Subject != "draft" {
		t.Fatalf("lastDraft = %+v", service.lastDraft)
	}
}

func TestSearchDraftsHandler(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{
		drafts: []maildb.MessageDetail{{ID: "draft-1", Subject: "hello draft", TextBody: "body"}},
	}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/drafts/search?user_id=user-1&q=%20hello%20&from=%20sender%20&to=%20alice%20&cc=%20bob%20&bcc=%20carol%20&subject=%20draft%20&has_attachment=false&limit=10", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Drafts  []maildb.MessageDetail `json:"drafts"`
		Limit   int                    `json:"limit"`
		HasMore bool                   `json:"has_more"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if len(body.Drafts) != 1 || body.Drafts[0].ID != "draft-1" {
		t.Fatalf("drafts = %+v", body.Drafts)
	}
	if body.Limit != 10 || body.HasMore {
		t.Fatalf("draft page metadata = limit:%d has_more:%v", body.Limit, body.HasMore)
	}
	if service.lastDraftSearch.UserID != "user-1" || service.lastDraftSearch.Query != "hello" || service.lastDraftSearch.From != "sender" || service.lastDraftSearch.Subject != "draft" {
		t.Fatalf("lastDraftSearch = %+v", service.lastDraftSearch)
	}
	if service.lastDraftSearch.To != "alice" || service.lastDraftSearch.Cc != "bob" || service.lastDraftSearch.Bcc != "carol" {
		t.Fatalf("lastDraftSearch to/cc/bcc = %q/%q/%q", service.lastDraftSearch.To, service.lastDraftSearch.Cc, service.lastDraftSearch.Bcc)
	}
	if service.lastDraftSearch.HasAttachment == nil || *service.lastDraftSearch.HasAttachment {
		t.Fatalf("HasAttachment = %+v", service.lastDraftSearch.HasAttachment)
	}
}

func TestSearchDraftsHandlerUsesContractDefaultLimit(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{
		drafts: []maildb.MessageDetail{{ID: "draft-1", Subject: "hello draft", TextBody: "body"}},
	}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/drafts/search?user_id=user-1&q=hello", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Limit int `json:"limit"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if body.Limit != maildb.MessageListDefaultLimit {
		t.Fatalf("response limit = %d, want %d", body.Limit, maildb.MessageListDefaultLimit)
	}
	if service.lastDraftSearch.Limit != maildb.MessageListDefaultLimit {
		t.Fatalf("draft search limit = %d, want %d", service.lastDraftSearch.Limit, maildb.MessageListDefaultLimit)
	}
}

func TestSearchDraftsHandlerRejectsInvalidCursor(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/drafts/search?user_id=user-1&cursor=bad", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastDraftSearch.UserID != "" {
		t.Fatalf("lastDraftSearch = %+v", service.lastDraftSearch)
	}
}

func TestSearchDraftsHandlerRejectsUnsafeFilters(t *testing.T) {
	t.Parallel()

	tests := []string{
		"/api/v1/drafts/search?user_id=user-1&q=hello%0Abad",
		"/api/v1/drafts/search?user_id=user-1&from=" + strings.Repeat("s", maxHTTPQueryBytes+1),
		"/api/v1/drafts/search?user_id=user-1&to=alice%0Dbad",
		"/api/v1/drafts/search?user_id=user-1&cc=bob%0Abad",
		"/api/v1/drafts/search?user_id=user-1&bcc=" + strings.Repeat("s", maxHTTPQueryBytes+1),
		"/api/v1/drafts/search?user_id=user-1&subject=receipt%0Dbad",
		"/api/v1/drafts/search?user_id=user-1&has_attachment=maybe",
	}
	for _, target := range tests {
		target := target
		t.Run(target, func(t *testing.T) {
			t.Parallel()

			service := &fakeMessageService{}
			mux := http.NewServeMux()
			RegisterMailRoutes(mux, service, nil)

			req := httptest.NewRequest(http.MethodGet, target, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if service.lastDraftSearch.UserID != "" {
				t.Fatalf("lastDraftSearch = %+v", service.lastDraftSearch)
			}
		})
	}
}

func TestDeleteDraftHandler(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/drafts/draft-1?user_id=user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastDeletedDraftID != "draft-1" {
		t.Fatalf("lastDeletedDraftID = %q", service.lastDeletedDraftID)
	}
}

func TestSendDraftHandler(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{
		sendResult: mailservice.SendTextResult{ID: "msg-1", RFCMessageID: "<msg-1@example.com>", Farm: outbound.FarmGeneral},
	}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/drafts/draft-1/send?user_id=user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastDeletedDraftID != "draft-1" || service.lastUserID != "user-1" {
		t.Fatalf("send draft = user:%q draft:%q", service.lastUserID, service.lastDeletedDraftID)
	}
}

func TestUpdateDraftHandlerUsesPathID(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/drafts/draft-1", strings.NewReader(`{
		"user_id":"user-1",
		"draft_id":"ignored",
		"subject":"updated"
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastDraft.DraftID != "draft-1" || service.lastDraft.Subject != "updated" {
		t.Fatalf("lastDraft = %+v", service.lastDraft)
	}
}

func TestListAttachmentsHandler(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{
		attachments: []maildb.Attachment{
			{ID: "att-1", MessageID: "msg-1", Filename: "report.pdf", MIMEType: "application/pdf", Size: 42, Status: "stored"},
		},
	}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/messages/msg-1/attachments?user_id=user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Attachments []maildb.Attachment `json:"attachments"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if len(body.Attachments) != 1 || body.Attachments[0].Filename != "report.pdf" {
		t.Fatalf("attachments = %+v", body.Attachments)
	}
	if service.lastMessageID != "msg-1" {
		t.Fatalf("lastMessageID = %q", service.lastMessageID)
	}
}

func TestCreateAttachmentUploadHandler(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/attachments", strings.NewReader(`{
		"user_id":"user-1",
		"draft_id":"draft-1",
		"filename":"report.pdf",
		"size":42,
		"mime_type":"application/pdf"
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastAttachmentUpload.DraftID != "draft-1" || service.lastAttachmentUpload.Filename != "report.pdf" {
		t.Fatalf("lastAttachmentUpload = %+v", service.lastAttachmentUpload)
	}
}

func TestPushDeviceHandlers(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{
		pushDevices: []maildb.PushDevice{{ID: "device-1", Platform: "fcm", Token: "token-1", Status: "active"}},
	}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/push-devices?user_id=user-1", strings.NewReader(`{"platform":"fcm","token":"token-1","label":"phone"}`))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	mux.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", createRec.Code, createRec.Body.String())
	}
	if service.lastPushDevice.UserID != "user-1" || service.lastPushDevice.Platform != "fcm" {
		t.Fatalf("lastPushDevice = %+v", service.lastPushDevice)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/push-devices?user_id=user-1&limit=5", nil)
	listRec := httptest.NewRecorder()
	mux.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d body=%s", listRec.Code, listRec.Body.String())
	}
	if !strings.Contains(listRec.Body.String(), "push_devices") {
		t.Fatalf("list body = %s", listRec.Body.String())
	}
	if strings.Contains(listRec.Body.String(), "token-1") {
		t.Fatalf("list body leaked raw token: %s", listRec.Body.String())
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/v1/push-devices/device-1?user_id=user-1", nil)
	deleteRec := httptest.NewRecorder()
	mux.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("delete status = %d body=%s", deleteRec.Code, deleteRec.Body.String())
	}
	if service.lastDeletePushDeviceID != "device-1" {
		t.Fatalf("lastDeletePushDeviceID = %q", service.lastDeletePushDeviceID)
	}
}

func TestCreateAttachmentUploadHandlerMapsQuotaFull(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{
		attachmentErr: fmt.Errorf("%w: user used 900, limit 1000, write 200 bytes", mail.ErrMailboxFull),
	}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/attachments", strings.NewReader(`{
		"user_id":"user-1",
		"filename":"report.pdf",
		"size":200,
		"mime_type":"application/pdf"
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusInsufficientStorage {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"code":"insufficient_storage"`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestUploadAttachmentHandler(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("draft_id", " draft-1 "); err != nil {
		t.Fatalf("WriteField returned error: %v", err)
	}
	part, err := writer.CreateFormFile("file", "report.pdf")
	if err != nil {
		t.Fatalf("CreateFormFile returned error: %v", err)
	}
	if _, err := part.Write([]byte("content")); err != nil {
		t.Fatalf("part.Write returned error: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/attachments/upload?user_id=user-1", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastAttachmentBody != "content" || service.lastAttachmentUpload.DraftID != "draft-1" {
		t.Fatalf("upload = body:%q req:%+v", service.lastAttachmentBody, service.lastAttachmentUpload)
	}
}

func TestUploadAttachmentHandlerRejectsAmbiguousMultipartScalars(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		build func(*testing.T, *multipart.Writer)
	}{
		{
			name: "duplicate draft id",
			build: func(t *testing.T, writer *multipart.Writer) {
				t.Helper()
				if err := writer.WriteField("draft_id", "draft-1"); err != nil {
					t.Fatalf("WriteField returned error: %v", err)
				}
				if err := writer.WriteField("draft_id", "draft-2"); err != nil {
					t.Fatalf("WriteField returned error: %v", err)
				}
				part, err := writer.CreateFormFile("file", "report.pdf")
				if err != nil {
					t.Fatalf("CreateFormFile returned error: %v", err)
				}
				if _, err := part.Write([]byte("content")); err != nil {
					t.Fatalf("part.Write returned error: %v", err)
				}
			},
		},
		{
			name: "duplicate file",
			build: func(t *testing.T, writer *multipart.Writer) {
				t.Helper()
				for _, name := range []string{"one.txt", "two.txt"} {
					part, err := writer.CreateFormFile("file", name)
					if err != nil {
						t.Fatalf("CreateFormFile returned error: %v", err)
					}
					if _, err := part.Write([]byte(name)); err != nil {
						t.Fatalf("part.Write returned error: %v", err)
					}
				}
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := &fakeMessageService{}
			mux := http.NewServeMux()
			RegisterMailRoutes(mux, service, nil)

			var body bytes.Buffer
			writer := multipart.NewWriter(&body)
			tt.build(t, writer)
			if err := writer.Close(); err != nil {
				t.Fatalf("writer.Close returned error: %v", err)
			}

			req := httptest.NewRequest(http.MethodPost, "/api/v1/attachments/upload?user_id=user-1", &body)
			req.Header.Set("Content-Type", writer.FormDataContentType())
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if service.lastAttachmentBody != "" || service.lastAttachmentUpload.UserID != "" {
				t.Fatalf("handler dispatched for ambiguous multipart upload: body=%q req=%+v", service.lastAttachmentBody, service.lastAttachmentUpload)
			}
		})
	}
}

func TestUploadAttachmentHandlerRejectsDraftIDQueryFallback(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "report.pdf")
	if err != nil {
		t.Fatalf("CreateFormFile returned error: %v", err)
	}
	if _, err := part.Write([]byte("content")); err != nil {
		t.Fatalf("part.Write returned error: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/attachments/upload?user_id=user-1&draft_id=query-draft", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastAttachmentBody != "" || service.lastAttachmentUpload.UserID != "" {
		t.Fatalf("handler dispatched for draft_id query fallback: body=%q req=%+v", service.lastAttachmentBody, service.lastAttachmentUpload)
	}
}

func TestCreateAttachmentUploadSessionHandler(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	expiresAt := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/attachments/upload-sessions?user_id=user-1", strings.NewReader(`{
		"draft_id":" draft-1 ",
		"filename":"large.bin",
		"declared_size":42,
		"mime_type":"application/octet-stream",
		"expires_at":"`+expiresAt.Format(time.RFC3339)+`"
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastUploadSession.UserID != "user-1" ||
		service.lastUploadSession.DraftID != " draft-1 " ||
		service.lastUploadSession.Filename != "large.bin" ||
		service.lastUploadSession.DeclaredSize != 42 ||
		service.lastUploadSession.MIMEType != "application/octet-stream" ||
		!service.lastUploadSession.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("lastUploadSession = %+v", service.lastUploadSession)
	}
	if !strings.Contains(rec.Body.String(), `"attachment_upload_session"`) || !strings.Contains(rec.Body.String(), `"status":"pending"`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestCancelAttachmentUploadHandler(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/attachments/att-1?user_id=user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastUserID != "user-1" || service.lastCancelAttachmentID != "att-1" {
		t.Fatalf("cancel request = user:%q attachment:%q", service.lastUserID, service.lastCancelAttachmentID)
	}
	if !strings.Contains(rec.Body.String(), `"attachment"`) || !strings.Contains(rec.Body.String(), `"status":"deleted"`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestCancelAttachmentUploadSessionHandler(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/attachments/upload-sessions/session-1?user_id=user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastUserID != "user-1" || service.lastCancelUploadSessionID != "session-1" {
		t.Fatalf("cancel session request = user:%q session:%q", service.lastUserID, service.lastCancelUploadSessionID)
	}
	if !strings.Contains(rec.Body.String(), `"attachment_upload_session"`) || !strings.Contains(rec.Body.String(), `"status":"canceled"`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestGetAttachmentUploadSessionHandler(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/attachments/upload-sessions/session-1?user_id=user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastUserID != "user-1" || service.lastGetUploadSessionID != "session-1" {
		t.Fatalf("get session request = user:%q session:%q", service.lastUserID, service.lastGetUploadSessionID)
	}
	if !strings.Contains(rec.Body.String(), `"attachment_upload_session"`) || !strings.Contains(rec.Body.String(), `"status":"pending"`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestStoreAttachmentUploadSessionBodyHandler(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/attachments/upload-sessions/session-1/body?user_id=user-1", strings.NewReader("content"))
	req.Header.Set("Content-Type", "application/json")
	checksum := sha256.Sum256([]byte("content"))
	req.Header.Set("X-Content-SHA256", hex.EncodeToString(checksum[:]))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastUserID != "user-1" || service.lastStoreUploadSessionID != "session-1" || service.lastUploadSessionChecksum != hex.EncodeToString(checksum[:]) || service.lastUploadSessionBody != "content" {
		t.Fatalf("store session request = user:%q session:%q checksum:%q body:%q", service.lastUserID, service.lastStoreUploadSessionID, service.lastUploadSessionChecksum, service.lastUploadSessionBody)
	}
	if !strings.Contains(rec.Body.String(), `"attachment_upload_session"`) || !strings.Contains(rec.Body.String(), `"status":"uploading"`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestStoreAttachmentUploadSessionBodyHandlerRejectsOversizedBody(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/attachments/upload-sessions/session-1/body?user_id=user-1", bytes.NewReader(bytes.Repeat([]byte("x"), int(mailservice.MaxAttachmentUploadBytes+2))))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"code":"payload_too_large"`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestStoreAttachmentUploadSessionBodyHandlerRejectsContentRange(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/attachments/upload-sessions/session-1/body?user_id=user-1", strings.NewReader("content"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Range", "bytes 0-6/7")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastStoreUploadSessionID != "" {
		t.Fatalf("service should not be called for range upload: session=%q", service.lastStoreUploadSessionID)
	}
}

func TestStoreAttachmentUploadSessionBodyHandlerRejectsDuplicateControlHeaders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		build func(http.Header)
	}{
		{
			name: "duplicate content range",
			build: func(header http.Header) {
				header.Add("Content-Range", "bytes 0-6/7")
				header.Add("Content-Range", "bytes 7-13/14")
			},
		},
		{
			name: "duplicate checksum",
			build: func(header http.Header) {
				header.Add("X-Content-SHA256", strings.Repeat("0", 64))
				header.Add("X-Content-SHA256", strings.Repeat("1", 64))
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := &fakeMessageService{}
			mux := http.NewServeMux()
			RegisterMailRoutes(mux, service, nil)

			req := httptest.NewRequest(http.MethodPut, "/api/v1/attachments/upload-sessions/session-1/body?user_id=user-1", strings.NewReader("content"))
			req.Header.Set("Content-Type", "application/json")
			tt.build(req.Header)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if service.lastStoreUploadSessionID != "" {
				t.Fatalf("service should not be called for duplicate control header: session=%q", service.lastStoreUploadSessionID)
			}
		})
	}
}

func TestFinalizeAttachmentUploadSessionHandler(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/attachments/upload-sessions/session-1/finalize?user_id=user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastUserID != "user-1" || service.lastFinalizeUploadSessionID != "session-1" {
		t.Fatalf("finalize session request = user:%q session:%q", service.lastUserID, service.lastFinalizeUploadSessionID)
	}
	if !strings.Contains(rec.Body.String(), `"attachment"`) || !strings.Contains(rec.Body.String(), `"status":"uploading"`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestAttachmentUploadCapabilitiesHandler(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/attachments/capabilities?user_id=user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Capabilities struct {
			MaxAttachmentBytes int64 `json:"max_attachment_bytes"`
			MaxFilenameBytes   int   `json:"max_filename_bytes"`
			MaxSessionTTL      int64 `json:"max_session_ttl_seconds"`
		} `json:"attachment_upload_capabilities"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if body.Capabilities.MaxAttachmentBytes != mailservice.MaxAttachmentUploadBytes {
		t.Fatalf("max_attachment_bytes = %d, want %d", body.Capabilities.MaxAttachmentBytes, mailservice.MaxAttachmentUploadBytes)
	}
	if body.Capabilities.MaxFilenameBytes != mailservice.MaxAttachmentFilenameBytes {
		t.Fatalf("max_filename_bytes = %d, want %d", body.Capabilities.MaxFilenameBytes, mailservice.MaxAttachmentFilenameBytes)
	}
	if body.Capabilities.MaxSessionTTL != int64(mailservice.MaxAttachmentUploadSessionTTL.Seconds()) {
		t.Fatalf("max_session_ttl_seconds = %d, want %d", body.Capabilities.MaxSessionTTL, int64(mailservice.MaxAttachmentUploadSessionTTL.Seconds()))
	}
	for _, want := range []string{
		`"attachment_upload_capabilities"`,
		`"metadata_reservation":true`,
		`"direct_multipart_upload":true`,
		`"cancel_pending_uploads":true`,
		`"upload_sessions":true`,
		`"cancel_upload_sessions":true`,
		`"upload_session_body":true`,
		`"upload_session_checksum":true`,
		`"finalize_upload_sessions":true`,
		`"resumable_chunked_uploads":false`,
		`"requires_declared_size":true`,
	} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("body missing %s: %s", want, rec.Body.String())
		}
	}
}

func TestUploadAttachmentHandlerRejectsOversizedRequestBody(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "too-large-envelope.bin")
	if err != nil {
		t.Fatalf("CreateFormFile returned error: %v", err)
	}
	if _, err := part.Write(bytes.Repeat([]byte("x"), int(mailservice.MaxAttachmentUploadBytes+(1<<20)+1))); err != nil {
		t.Fatalf("part.Write returned error: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/attachments/upload?user_id=user-1", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "attachment upload request is too large") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestUploadAttachmentHandlerRejectsUnsafeDraftID(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		draftID string
	}{
		{
			name:    "crlf",
			draftID: "draft\nbad",
		},
		{
			name:    "oversized",
			draftID: strings.Repeat("d", maxHTTPResourceIDBytes+1),
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			service := &fakeMessageService{}
			mux := http.NewServeMux()
			RegisterMailRoutes(mux, service, nil)

			var body bytes.Buffer
			writer := multipart.NewWriter(&body)
			if err := writer.WriteField("draft_id", tc.draftID); err != nil {
				t.Fatalf("WriteField returned error: %v", err)
			}
			part, err := writer.CreateFormFile("file", "report.pdf")
			if err != nil {
				t.Fatalf("CreateFormFile returned error: %v", err)
			}
			if _, err := part.Write([]byte("content")); err != nil {
				t.Fatalf("part.Write returned error: %v", err)
			}
			if err := writer.Close(); err != nil {
				t.Fatalf("writer.Close returned error: %v", err)
			}

			req := httptest.NewRequest(http.MethodPost, "/api/v1/attachments/upload?user_id=user-1", &body)
			req.Header.Set("Content-Type", writer.FormDataContentType())
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if service.lastAttachmentUpload.DraftID != "" || service.lastAttachmentBody != "" {
				t.Fatalf("handler should not dispatch unsafe draft_id: body=%q req=%+v", service.lastAttachmentBody, service.lastAttachmentUpload)
			}
		})
	}
}

func TestUploadAttachmentHandlerRejectsOversize(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "large.bin")
	if err != nil {
		t.Fatalf("CreateFormFile returned error: %v", err)
	}
	if _, err := part.Write(bytes.Repeat([]byte("x"), int(mailservice.MaxAttachmentUploadBytes)+1)); err != nil {
		t.Fatalf("part.Write returned error: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/attachments/upload?user_id=user-1", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastAttachmentUpload.Filename != "" {
		t.Fatalf("upload should not reach service: %+v", service.lastAttachmentUpload)
	}
}

func TestDownloadAttachmentHandler(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/messages/msg-1/attachments/att-1/download?user_id=user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != "content" {
		t.Fatalf("body = %q", rec.Body.String())
	}
	if got := rec.Header().Get("Content-Disposition"); !strings.Contains(got, `filename="report.pdf"`) {
		t.Fatalf("Content-Disposition = %q", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q", got)
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q", got)
	}
	if got := rec.Header().Get("Content-Length"); got != "7" {
		t.Fatalf("Content-Length = %q", got)
	}
}

func TestHeadDownloadAttachmentHandler(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{
		attachmentMetadata: mailservice.AttachmentMetadata{
			Attachment: maildb.Attachment{ID: "att-1", Filename: "report.pdf", MIMEType: "application/pdf", Size: 1},
			Object:     storage.ObjectInfo{Path: "attachments/att-1", Size: 7},
		},
	}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodHead, "/api/v1/messages/msg-1/attachments/att-1/download?user_id=user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastUserID != "user-1" || service.lastMessageID != "msg-1" || service.lastAttachmentID != "att-1" {
		t.Fatalf("stat request = user %q message %q attachment %q", service.lastUserID, service.lastMessageID, service.lastAttachmentID)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("HEAD body length = %d, want 0", rec.Body.Len())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/pdf" {
		t.Fatalf("Content-Type = %q", got)
	}
	if got := rec.Header().Get("Content-Disposition"); !strings.Contains(got, `filename="report.pdf"`) {
		t.Fatalf("Content-Disposition = %q", got)
	}
	if got := rec.Header().Get("Content-Length"); got != "7" {
		t.Fatalf("Content-Length = %q", got)
	}
}

func TestDownloadAttachmentHandlerUsesUTF8FilenameParameter(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{
		download: mailservice.AttachmentDownload{
			Attachment: maildb.Attachment{ID: "att-1", Filename: "보고서 1.pdf", MIMEType: "application/pdf", Size: 7},
			Body:       io.NopCloser(strings.NewReader("content")),
		},
	}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/messages/msg-1/attachments/att-1/download?user_id=user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	got := rec.Header().Get("Content-Disposition")
	if !strings.Contains(got, `filename="___ 1.pdf"`) {
		t.Fatalf("Content-Disposition ASCII fallback = %q", got)
	}
	if !strings.Contains(got, `filename*=UTF-8''%EB%B3%B4%EA%B3%A0%EC%84%9C%201.pdf`) {
		t.Fatalf("Content-Disposition UTF-8 parameter = %q", got)
	}
}

func TestDownloadAttachmentHandlerBoundsFilenameHeader(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{
		download: mailservice.AttachmentDownload{
			Attachment: maildb.Attachment{ID: "att-1", Filename: strings.Repeat("a", 220) + ".pdf", MIMEType: "application/pdf", Size: 7},
			Body:       io.NopCloser(strings.NewReader("content")),
		},
	}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/messages/msg-1/attachments/att-1/download?user_id=user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	got := rec.Header().Get("Content-Disposition")
	if strings.Contains(got, strings.Repeat("a", 181)) {
		t.Fatalf("Content-Disposition was not bounded: %q", got)
	}
	if !strings.Contains(got, `filename="`+strings.Repeat("a", 180)+`"`) {
		t.Fatalf("Content-Disposition = %q", got)
	}
}

func TestDownloadAttachmentHandlerFallsBackForUnsafeMIMEType(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{
		download: mailservice.AttachmentDownload{
			Attachment: maildb.Attachment{ID: "att-1", Filename: "report.pdf", MIMEType: "application/pdf\r\nX-Bad: yes", Size: 7},
			Body:       io.NopCloser(strings.NewReader("content")),
		},
	}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/messages/msg-1/attachments/att-1/download?user_id=user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/octet-stream" {
		t.Fatalf("Content-Type = %q", got)
	}
}

func TestDownloadAttachmentHandlerFallsBackForInvalidMIMEType(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{
		download: mailservice.AttachmentDownload{
			Attachment: maildb.Attachment{ID: "att-1", Filename: "report.pdf", MIMEType: "not-a-content-type", Size: 7},
			Body:       io.NopCloser(strings.NewReader("content")),
		},
	}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/messages/msg-1/attachments/att-1/download?user_id=user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/octet-stream" {
		t.Fatalf("Content-Type = %q", got)
	}
}

func TestSendMessageHandler(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{
		sendResult: mailservice.SendTextResult{
			ID:           "msg-1",
			RFCMessageID: "<msg-1@example.com>",
			Farm:         outbound.FarmGeneral,
		},
	}

	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/messages/send", strings.NewReader(`{
		"user_id":"user-1",
		"to":[{"email":"recipient@example.net"}],
		"subject":"hello",
		"text_body":"body"
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastSend.UserID != "user-1" {
		t.Fatalf("lastSend.UserID = %q", service.lastSend.UserID)
	}
	if len(service.lastSend.To) != 1 || service.lastSend.To[0].Email != "recipient@example.net" {
		t.Fatalf("lastSend.To = %+v", service.lastSend.To)
	}
	if !strings.Contains(rec.Body.String(), `"send_status":"queued"`) ||
		!strings.Contains(rec.Body.String(), `"delivery_status":"pending"`) ||
		!strings.Contains(rec.Body.String(), `"bounce_status":"none"`) {
		t.Fatalf("send response missing status contract: %s", rec.Body.String())
	}
}

func TestMessageDeliveryStatusHandler(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{
		deliveryStatus: maildb.MessageDeliveryStatusView{
			MessageID:      "msg-1",
			RFCMessageID:   "<msg-1@example.com>",
			DeliveryStatus: "delivered",
			BounceStatus:   "none",
			Attempts: []maildb.DeliveryAttemptView{{
				ID:        "attempt-1",
				MessageID: "msg-1",
				Recipient: "recipient@example.net",
				Status:    "delivered",
			}},
		},
	}

	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/messages/%20msg-1%20/delivery-status?user_id=user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		DeliveryStatus maildb.MessageDeliveryStatusView `json:"delivery_status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if body.DeliveryStatus.MessageID != "msg-1" || body.DeliveryStatus.DeliveryStatus != "delivered" {
		t.Fatalf("delivery_status = %+v", body.DeliveryStatus)
	}
	if service.lastUserID != "user-1" || service.lastMessageID != "msg-1" {
		t.Fatalf("lastUserID=%q lastMessageID=%q", service.lastUserID, service.lastMessageID)
	}
}

func TestSendReplyHandlerPassesSourceMessageID(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/messages/send", strings.NewReader(`{
		"user_id":"user-1",
		"intent":"reply",
		"source_message_id":"msg-original",
		"to":[{"email":"sender@example.net"}],
		"subject":"Re: hello",
		"text_body":"body"
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastSend.Intent != mailservice.ComposeIntentReply || service.lastSend.SourceMessageID != "msg-original" {
		t.Fatalf("lastSend = %+v", service.lastSend)
	}
}

func TestListMessagesHandlerUsesJWTUser(t *testing.T) {
	t.Parallel()

	manager, err := auth.NewTokenManager("secret")
	if err != nil {
		t.Fatalf("NewTokenManager returned error: %v", err)
	}
	token, err := manager.Sign(auth.Claims{UserID: "jwt-user"}, time.Minute)
	if err != nil {
		t.Fatalf("Sign returned error: %v", err)
	}
	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, manager)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/messages", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastUserID != "jwt-user" {
		t.Fatalf("lastUserID = %q, want jwt-user", service.lastUserID)
	}
}

func TestMailAuthRejectsOversizedAuthorizationHeader(t *testing.T) {
	t.Parallel()

	manager, err := auth.NewTokenManager("secret")
	if err != nil {
		t.Fatalf("NewTokenManager returned error: %v", err)
	}
	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, manager)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/messages", nil)
	req.Header.Set("Authorization", strings.Repeat("a", maxHTTPAuthHeaderBytes+1))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastUserID != "" {
		t.Fatalf("handler should not dispatch oversized auth header, lastUserID = %q", service.lastUserID)
	}
}

func TestMailAuthRejectsDuplicateAuthorizationHeaders(t *testing.T) {
	t.Parallel()

	manager, err := auth.NewTokenManager("secret")
	if err != nil {
		t.Fatalf("NewTokenManager returned error: %v", err)
	}
	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, manager)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/messages", nil)
	req.Header.Add("Authorization", "Bearer one")
	req.Header.Add("Authorization", "Bearer two")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastUserID != "" {
		t.Fatalf("handler should not dispatch duplicate auth headers, lastUserID = %q", service.lastUserID)
	}
}

func TestMailRoutesTrimQueryUserID(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/messages?user_id=%20user-1%20", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastUserID != "user-1" {
		t.Fatalf("lastUserID = %q", service.lastUserID)
	}
}

func TestMailRoutesRejectUnsafeQueryUserID(t *testing.T) {
	t.Parallel()

	tests := []string{
		"/api/v1/messages?user_id=user%0Abad",
		"/api/v1/messages?user_id=" + strings.Repeat("u", maxHTTPResourceIDBytes+1),
	}

	for _, path := range tests {
		path := path
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			service := &fakeMessageService{}
			mux := http.NewServeMux()
			RegisterMailRoutes(mux, service, nil)

			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if service.lastUserID != "" {
				t.Fatalf("lastUserID = %q", service.lastUserID)
			}
		})
	}
}

func TestMailRoutesRejectUnsafePathIDs(t *testing.T) {
	t.Parallel()

	oversized := strings.Repeat("x", maxHTTPResourceIDBytes+1)
	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{
			name:   "folder crlf",
			method: http.MethodPatch,
			path:   "/api/v1/folders/folder%0Abad?user_id=user-1",
			body:   `{"name":"Renamed"}`,
		},
		{
			name:   "thread oversized",
			method: http.MethodGet,
			path:   "/api/v1/threads/" + oversized + "/messages?user_id=user-1",
		},
		{
			name:   "message crlf",
			method: http.MethodGet,
			path:   "/api/v1/messages/msg%0Abad?user_id=user-1",
		},
		{
			name:   "message attachment oversized",
			method: http.MethodGet,
			path:   "/api/v1/messages/" + oversized + "/attachments/att-1/download?user_id=user-1",
		},
		{
			name:   "attachment crlf",
			method: http.MethodGet,
			path:   "/api/v1/messages/msg-1/attachments/att%0Abad/download?user_id=user-1",
		},
		{
			name:   "attachment cancel crlf",
			method: http.MethodDelete,
			path:   "/api/v1/attachments/att%0Abad?user_id=user-1",
		},
		{
			name:   "upload session cancel crlf",
			method: http.MethodDelete,
			path:   "/api/v1/attachments/upload-sessions/session%0Abad?user_id=user-1",
		},
		{
			name:   "upload session get crlf",
			method: http.MethodGet,
			path:   "/api/v1/attachments/upload-sessions/session%0Abad?user_id=user-1",
		},
		{
			name:   "upload session body crlf",
			method: http.MethodPut,
			path:   "/api/v1/attachments/upload-sessions/session%0Abad/body?user_id=user-1",
		},
		{
			name:   "upload session finalize crlf",
			method: http.MethodPost,
			path:   "/api/v1/attachments/upload-sessions/session%0Abad/finalize?user_id=user-1",
		},
		{
			name:   "draft crlf",
			method: http.MethodPatch,
			path:   "/api/v1/drafts/draft%0Abad",
			body:   `{"user_id":"user-1","to":[{"email":"user@example.net"}],"subject":"draft"}`,
		},
		{
			name:   "push device oversized",
			method: http.MethodDelete,
			path:   "/api/v1/push-devices/" + oversized + "?user_id=user-1",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := &fakeMessageService{}
			mux := http.NewServeMux()
			RegisterMailRoutes(mux, service, nil)

			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if service.lastFolderID != "" || service.lastThreadID != "" || service.lastMessageID != "" || service.lastDraft.DraftID != "" || service.lastDeletedDraftID != "" || service.lastDeletePushDeviceID != "" || service.lastCancelAttachmentID != "" || service.lastCancelUploadSessionID != "" || service.lastGetUploadSessionID != "" || service.lastStoreUploadSessionID != "" || service.lastFinalizeUploadSessionID != "" {
				t.Fatalf("service dispatched: folder=%q thread=%q message=%q draft=%q deletedDraft=%q push=%q cancelAttachment=%q cancelUploadSession=%q getUploadSession=%q storeUploadSession=%q finalizeUploadSession=%q", service.lastFolderID, service.lastThreadID, service.lastMessageID, service.lastDraft.DraftID, service.lastDeletedDraftID, service.lastDeletePushDeviceID, service.lastCancelAttachmentID, service.lastCancelUploadSessionID, service.lastGetUploadSessionID, service.lastStoreUploadSessionID, service.lastFinalizeUploadSessionID)
			}
		})
	}
}

func TestMailRoutesRequireJWTWhenConfigured(t *testing.T) {
	t.Parallel()

	manager, err := auth.NewTokenManager("secret")
	if err != nil {
		t.Fatalf("NewTokenManager returned error: %v", err)
	}
	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, manager)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/messages", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

type fakeMessageService struct {
	folders                     []maildb.Folder
	createdFolder               maildb.Folder
	list                        []maildb.MessageSummary
	drafts                      []maildb.MessageDetail
	threads                     []maildb.ThreadSummary
	attachments                 []maildb.Attachment
	pushDevices                 []maildb.PushDevice
	download                    mailservice.AttachmentDownload
	attachmentMetadata          mailservice.AttachmentMetadata
	detail                      maildb.MessageDetail
	sendResult                  mailservice.SendTextResult
	deliveryStatus              maildb.MessageDeliveryStatusView
	lastSend                    mailservice.SendTextRequest
	lastDraft                   mailservice.SaveDraftRequest
	lastAttachmentUpload        mailservice.CreateAttachmentUploadRequest
	lastAttachmentID            string
	lastUploadSession           mailservice.CreateAttachmentUploadSessionRequest
	lastCancelAttachmentID      string
	lastCancelUploadSessionID   string
	lastGetUploadSessionID      string
	lastStoreUploadSessionID    string
	lastFinalizeUploadSessionID string
	lastPushDevice              maildb.UpsertPushDeviceRequest
	lastAttachmentBody          string
	lastUploadSessionBody       string
	lastUploadSessionChecksum   string
	attachmentErr               error
	lastUserID                  string
	lastFolderName              string
	lastDeletedFolderID         string
	lastMessageID               string
	lastFolderID                string
	lastThreadID                string
	lastMoveFolderID            string
	lastDeletedID               string
	lastRestoredID              string
	lastDeletedDraftID          string
	lastDeletePushDeviceID      string
	lastFlag                    string
	lastFlagValue               bool
	lastBulkFlag                maildb.BulkMessageFlagRequest
	lastBulkThreadFlag          maildb.BulkThreadFlagRequest
	lastBulkMove                maildb.BulkMessageMoveRequest
	lastBulkThreadMove          maildb.BulkThreadMoveRequest
	lastBulkDelete              maildb.BulkMessageDeleteRequest
	lastBulkThreadDelete        maildb.BulkThreadDeleteRequest
	lastBulkRestore             maildb.BulkMessageRestoreRequest
	lastBulkThreadRestore       maildb.BulkThreadRestoreRequest
	lastListFilter              maildb.MessageListFilter
	lastThreadFilter            maildb.ThreadListFilter
	lastSearch                  maildb.MessageSearchQuery
	lastDraftSearch             maildb.DraftSearchQuery
	lastLimit                   int
}

func (f *fakeMessageService) ListFolders(_ context.Context, userID string) ([]maildb.Folder, error) {
	f.lastUserID = userID
	return f.folders, nil
}

func (f *fakeMessageService) CreateFolder(_ context.Context, req maildb.CreateFolderRequest) (maildb.Folder, error) {
	f.lastUserID = req.UserID
	f.lastFolderName = req.Name
	if f.createdFolder.ID != "" {
		return f.createdFolder, nil
	}
	return maildb.Folder{ID: "folder-new", Name: req.Name, FullPath: req.Name, Type: "user"}, nil
}

func (f *fakeMessageService) RenameFolder(_ context.Context, userID string, folderID string, name string) (maildb.Folder, error) {
	f.lastUserID = userID
	f.lastFolderID = folderID
	f.lastFolderName = name
	return maildb.Folder{ID: folderID, Name: name, FullPath: name, Type: "user"}, nil
}

func (f *fakeMessageService) DeleteFolder(_ context.Context, userID string, folderID string) error {
	f.lastUserID = userID
	f.lastDeletedFolderID = folderID
	return nil
}

func (f *fakeMessageService) ListMessages(_ context.Context, userID string, limit int) ([]maildb.MessageSummary, error) {
	f.lastUserID = userID
	f.lastLimit = limit
	return f.list, nil
}

func (f *fakeMessageService) ListMessagesInFolder(_ context.Context, userID string, folderID string, limit int) ([]maildb.MessageSummary, error) {
	f.lastUserID = userID
	f.lastFolderID = folderID
	f.lastLimit = limit
	return f.list, nil
}

func (f *fakeMessageService) ListMessagesPage(_ context.Context, userID string, folderID string, limit int, _ maildb.MessageListCursor, filter maildb.MessageListFilter) ([]maildb.MessageSummary, error) {
	f.lastUserID = userID
	f.lastFolderID = folderID
	f.lastLimit = limit
	f.lastListFilter = filter
	return f.list, nil
}

func (f *fakeMessageService) ListThreads(_ context.Context, userID string, limit int) ([]maildb.ThreadSummary, error) {
	f.lastUserID = userID
	f.lastLimit = limit
	return f.threads, nil
}

func (f *fakeMessageService) ListThreadsPage(_ context.Context, userID string, limit int, _ maildb.ThreadListCursor, filter maildb.ThreadListFilter) ([]maildb.ThreadSummary, error) {
	f.lastUserID = userID
	f.lastLimit = limit
	f.lastThreadFilter = filter
	return f.threads, nil
}

func (f *fakeMessageService) ListThreadMessages(_ context.Context, userID string, threadID string, limit int) ([]maildb.MessageSummary, error) {
	f.lastUserID = userID
	f.lastThreadID = threadID
	f.lastLimit = limit
	return f.list, nil
}

func (f *fakeMessageService) ListThreadMessagesPage(_ context.Context, userID string, threadID string, limit int, _ maildb.MessageListCursor) ([]maildb.MessageSummary, error) {
	f.lastUserID = userID
	f.lastThreadID = threadID
	f.lastLimit = limit
	return f.list, nil
}

func (f *fakeMessageService) SearchMessages(_ context.Context, query maildb.MessageSearchQuery) ([]maildb.MessageSummary, error) {
	f.lastSearch = query
	return f.list, nil
}

func (f *fakeMessageService) SearchDrafts(_ context.Context, query maildb.DraftSearchQuery) ([]maildb.MessageDetail, error) {
	f.lastDraftSearch = query
	return f.drafts, nil
}

func (f *fakeMessageService) GetMessage(_ context.Context, userID string, messageID string) (maildb.MessageDetail, error) {
	f.lastUserID = userID
	f.lastMessageID = messageID
	return f.detail, nil
}

func (f *fakeMessageService) SetMessageFlag(_ context.Context, userID string, messageID string, flag string, value bool) error {
	f.lastUserID = userID
	f.lastMessageID = messageID
	f.lastFlag = flag
	f.lastFlagValue = value
	return nil
}

func (f *fakeMessageService) BulkSetMessageFlag(_ context.Context, req maildb.BulkMessageFlagRequest) (int64, error) {
	f.lastBulkFlag = req
	return int64(len(req.MessageIDs)), nil
}

func (f *fakeMessageService) BulkSetThreadFlag(_ context.Context, req maildb.BulkThreadFlagRequest) (int64, error) {
	f.lastBulkThreadFlag = req
	return int64(len(req.ThreadIDs)), nil
}

func (f *fakeMessageService) MoveMessage(_ context.Context, userID string, messageID string, folderID string) error {
	f.lastUserID = userID
	f.lastMessageID = messageID
	f.lastMoveFolderID = folderID
	return nil
}

func (f *fakeMessageService) BulkMoveMessages(_ context.Context, req maildb.BulkMessageMoveRequest) (int64, error) {
	f.lastBulkMove = req
	return int64(len(req.MessageIDs)), nil
}

func (f *fakeMessageService) BulkMoveThreads(_ context.Context, req maildb.BulkThreadMoveRequest) (int64, error) {
	f.lastBulkThreadMove = req
	return int64(len(req.ThreadIDs)), nil
}

func (f *fakeMessageService) DeleteMessage(_ context.Context, userID string, messageID string) error {
	f.lastUserID = userID
	f.lastDeletedID = messageID
	return nil
}

func (f *fakeMessageService) BulkDeleteMessages(_ context.Context, req maildb.BulkMessageDeleteRequest) (int64, error) {
	f.lastBulkDelete = req
	return int64(len(req.MessageIDs)), nil
}

func (f *fakeMessageService) BulkDeleteThreads(_ context.Context, req maildb.BulkThreadDeleteRequest) (int64, error) {
	f.lastBulkThreadDelete = req
	return int64(len(req.ThreadIDs)), nil
}

func (f *fakeMessageService) RestoreMessage(_ context.Context, userID string, messageID string) error {
	f.lastUserID = userID
	f.lastRestoredID = messageID
	return nil
}

func (f *fakeMessageService) BulkRestoreMessages(_ context.Context, req maildb.BulkMessageRestoreRequest) (int64, error) {
	f.lastBulkRestore = req
	return int64(len(req.MessageIDs)), nil
}

func (f *fakeMessageService) BulkRestoreThreads(_ context.Context, req maildb.BulkThreadRestoreRequest) (int64, error) {
	f.lastBulkThreadRestore = req
	return int64(len(req.ThreadIDs)), nil
}

func (f *fakeMessageService) ListPushDevices(_ context.Context, userID string, limit int) ([]maildb.PushDevice, error) {
	f.lastUserID = userID
	f.lastLimit = limit
	return f.pushDevices, nil
}

func (f *fakeMessageService) UpsertPushDevice(_ context.Context, req maildb.UpsertPushDeviceRequest) (maildb.PushDevice, error) {
	f.lastPushDevice = req
	return maildb.PushDevice{ID: "device-1", UserID: req.UserID, Platform: req.Platform, Token: req.Token, Status: "active"}, nil
}

func (f *fakeMessageService) DeletePushDevice(_ context.Context, userID string, id string) error {
	f.lastUserID = userID
	f.lastDeletePushDeviceID = id
	return nil
}

func (f *fakeMessageService) SaveDraft(_ context.Context, req mailservice.SaveDraftRequest) (maildb.MessageDetail, error) {
	f.lastDraft = req
	return maildb.MessageDetail{ID: "draft-1", Subject: req.Subject}, nil
}

func (f *fakeMessageService) DeleteDraft(_ context.Context, userID string, draftID string) error {
	f.lastUserID = userID
	f.lastDeletedDraftID = draftID
	return nil
}

func (f *fakeMessageService) SendDraft(_ context.Context, userID string, draftID string) (mailservice.SendTextResult, error) {
	f.lastUserID = userID
	f.lastDeletedDraftID = draftID
	return f.sendResult, nil
}

func (f *fakeMessageService) CreateAttachmentUpload(_ context.Context, req mailservice.CreateAttachmentUploadRequest) (maildb.Attachment, error) {
	f.lastAttachmentUpload = req
	if f.attachmentErr != nil {
		return maildb.Attachment{}, f.attachmentErr
	}
	return maildb.Attachment{ID: "att-1", Filename: req.Filename, MIMEType: req.MIMEType, Size: req.Size}, nil
}

func (f *fakeMessageService) UploadAttachment(_ context.Context, req mailservice.UploadAttachmentRequest) (maildb.Attachment, error) {
	f.lastAttachmentUpload = mailservice.CreateAttachmentUploadRequest{
		UserID:   req.UserID,
		DraftID:  req.DraftID,
		Filename: req.Filename,
		Size:     req.Size,
		MIMEType: req.MIMEType,
	}
	raw, _ := io.ReadAll(req.Body)
	f.lastAttachmentBody = string(raw)
	return maildb.Attachment{ID: "att-1", Filename: req.Filename, MIMEType: req.MIMEType, Size: req.Size}, nil
}

func (f *fakeMessageService) CancelAttachmentUpload(_ context.Context, userID string, attachmentID string) (maildb.Attachment, error) {
	f.lastUserID = userID
	f.lastCancelAttachmentID = attachmentID
	return maildb.Attachment{ID: attachmentID, Status: "deleted"}, nil
}

func (f *fakeMessageService) CreateAttachmentUploadSession(_ context.Context, req mailservice.CreateAttachmentUploadSessionRequest) (maildb.AttachmentUploadSession, error) {
	f.lastUploadSession = req
	return maildb.AttachmentUploadSession{ID: "session-1", UserID: req.UserID, DraftID: req.DraftID, Filename: req.Filename, DeclaredSize: req.DeclaredSize, MIMEType: req.MIMEType, Status: "pending", ExpiresAt: req.ExpiresAt}, nil
}

func (f *fakeMessageService) CancelAttachmentUploadSession(_ context.Context, userID string, sessionID string) (maildb.AttachmentUploadSession, error) {
	f.lastUserID = userID
	f.lastCancelUploadSessionID = sessionID
	return maildb.AttachmentUploadSession{ID: sessionID, UserID: userID, Status: "canceled"}, nil
}

func (f *fakeMessageService) GetAttachmentUploadSession(_ context.Context, userID string, sessionID string) (maildb.AttachmentUploadSession, error) {
	f.lastUserID = userID
	f.lastGetUploadSessionID = sessionID
	return maildb.AttachmentUploadSession{ID: sessionID, UserID: userID, Status: "pending"}, nil
}

func (f *fakeMessageService) StoreAttachmentUploadSessionBody(_ context.Context, req mailservice.StoreAttachmentUploadSessionBodyRequest) (maildb.AttachmentUploadSession, error) {
	f.lastUserID = req.UserID
	f.lastStoreUploadSessionID = req.SessionID
	f.lastUploadSessionChecksum = req.ExpectedChecksumSHA256
	raw, err := io.ReadAll(req.Body)
	if err != nil {
		return maildb.AttachmentUploadSession{}, err
	}
	f.lastUploadSessionBody = string(raw)
	return maildb.AttachmentUploadSession{ID: req.SessionID, UserID: req.UserID, ReceivedSize: int64(len(raw)), Status: "uploading"}, nil
}

func (f *fakeMessageService) FinalizeAttachmentUploadSession(_ context.Context, userID string, sessionID string) (maildb.Attachment, error) {
	f.lastUserID = userID
	f.lastFinalizeUploadSessionID = sessionID
	return maildb.Attachment{ID: "att-1", UploadID: "upload-1", StoragePath: "upload-sessions/user-1/session-1/body", Filename: "large.bin", Size: 7, MIMEType: "application/octet-stream", Status: "uploading"}, nil
}

func (f *fakeMessageService) ListAttachments(_ context.Context, userID string, messageID string) ([]maildb.Attachment, error) {
	f.lastUserID = userID
	f.lastMessageID = messageID
	return f.attachments, nil
}

func (f *fakeMessageService) OpenAttachment(_ context.Context, userID string, messageID string, attachmentID string) (mailservice.AttachmentDownload, error) {
	f.lastUserID = userID
	f.lastMessageID = messageID
	f.lastAttachmentID = attachmentID
	if f.download.Body != nil {
		return f.download, nil
	}
	return mailservice.AttachmentDownload{
		Attachment: maildb.Attachment{ID: attachmentID, Filename: "report.pdf", MIMEType: "application/pdf", Size: 7},
		Body:       io.NopCloser(strings.NewReader("content")),
	}, nil
}

func (f *fakeMessageService) StatAttachment(_ context.Context, userID string, messageID string, attachmentID string) (mailservice.AttachmentMetadata, error) {
	f.lastUserID = userID
	f.lastMessageID = messageID
	f.lastAttachmentID = attachmentID
	if f.attachmentErr != nil {
		return mailservice.AttachmentMetadata{}, f.attachmentErr
	}
	if f.attachmentMetadata.Attachment.ID != "" {
		return f.attachmentMetadata, nil
	}
	return mailservice.AttachmentMetadata{
		Attachment: maildb.Attachment{ID: attachmentID, Filename: "report.pdf", MIMEType: "application/pdf", Size: 7},
		Object:     storage.ObjectInfo{Path: "attachments/att-1", Size: 7},
	}, nil
}

func (f *fakeMessageService) SendText(_ context.Context, req mailservice.SendTextRequest) (mailservice.SendTextResult, error) {
	f.lastSend = req
	return f.sendResult, nil
}

func (f *fakeMessageService) MessageDeliveryStatus(_ context.Context, userID string, messageID string) (maildb.MessageDeliveryStatusView, error) {
	f.lastUserID = userID
	f.lastMessageID = messageID
	return f.deliveryStatus, nil
}
