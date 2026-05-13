package mailservice

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/maildb"
	"github.com/gogomail/gogomail/internal/pop3d"
	smtpd "github.com/gogomail/gogomail/internal/smtp"
	"github.com/gogomail/gogomail/internal/storage"
)

// pop3TestRepository embeds fakeRepository and overrides return values for
// the methods the POP3 adapter actually calls.
type pop3TestRepository struct {
	fakeRepository
	folders         []maildb.Folder
	folderErr       error
	messages        []maildb.MessageSummary
	details         map[string]maildb.MessageDetail
	pageErr         error
	pageCalls       int
	folderUsers     []string
	pageUsers       []string
	pageFolders     []string
	pageCursors     []maildb.MessageListCursor
	bulkDeleteCalls int
	bulkDeleteErr   error
}

func (r *pop3TestRepository) ListFolders(_ context.Context, userID string) ([]maildb.Folder, error) {
	r.folderUsers = append(r.folderUsers, userID)
	if r.folderErr != nil {
		return nil, r.folderErr
	}
	return r.folders, nil
}

func (r *pop3TestRepository) ListMessagesInFolder(_ context.Context, _, _ string, _ int) ([]maildb.MessageSummary, error) {
	return r.messages, nil
}

func (r *pop3TestRepository) ListMessagesPage(_ context.Context, userID, folderID string, limit int, cursor maildb.MessageListCursor, _ maildb.MessageListFilter) ([]maildb.MessageSummary, error) {
	r.pageCalls++
	r.pageUsers = append(r.pageUsers, userID)
	r.pageFolders = append(r.pageFolders, folderID)
	r.pageCursors = append(r.pageCursors, cursor)
	if r.pageErr != nil {
		return nil, r.pageErr
	}
	start := 0
	if cursor.ID != "" {
		start = len(r.messages)
		for i, message := range r.messages {
			if message.ID == cursor.ID {
				start = i + 1
				break
			}
		}
	}
	end := start + limit + 1
	if end > len(r.messages) {
		end = len(r.messages)
	}
	if start >= end {
		return nil, nil
	}
	return append([]maildb.MessageSummary(nil), r.messages[start:end]...), nil
}

func (r *pop3TestRepository) GetMessage(_ context.Context, _, messageID string) (maildb.MessageDetail, error) {
	if d, ok := r.details[messageID]; ok {
		return d, nil
	}
	return maildb.MessageDetail{}, fmt.Errorf("message not found: %s", messageID)
}

func (r *pop3TestRepository) BulkDeleteMessages(_ context.Context, req maildb.BulkMessageDeleteRequest) (int64, error) {
	r.bulkDeleteCalls++
	r.lastBulkDelete = req
	if r.bulkDeleteErr != nil {
		return 0, r.bulkDeleteErr
	}
	return int64(len(req.MessageIDs)), nil
}

// pop3TestStore embeds recordingStore and adds body lookup.
type pop3TestStore struct {
	recordingStore
	bodies map[string]string
}

func (s *pop3TestStore) Get(_ context.Context, path string) (io.ReadCloser, error) {
	if body, ok := s.bodies[path]; ok {
		return io.NopCloser(strings.NewReader(body)), nil
	}
	return nil, fmt.Errorf("object not found: %s", path)
}

// pop3TestAuth validates fixed credentials.
type pop3TestAuth struct {
	validUser          string
	validPass          string
	userID             string
	mustChangePassword bool
	calls              int
	usernames          []string
	passwords          []string
}

func (a *pop3TestAuth) AuthenticatePlain(_ context.Context, _, username, password string) (smtpd.SubmissionUser, error) {
	a.calls++
	a.usernames = append(a.usernames, username)
	a.passwords = append(a.passwords, password)
	if username == a.validUser && password == a.validPass {
		return smtpd.SubmissionUser{UserID: a.userID, Address: username, MustChangePassword: a.mustChangePassword}, nil
	}
	return smtpd.SubmissionUser{}, fmt.Errorf("invalid credentials")
}

func newPOP3TestSetup() (POP3StoreAdapter, *pop3TestRepository, *pop3TestStore) {
	repo := &pop3TestRepository{
		folders: []maildb.Folder{
			{ID: "folder-inbox", Name: "Inbox", SystemType: "inbox"},
		},
		messages: []maildb.MessageSummary{
			{ID: "msg-001", Size: 42},
			{ID: "msg-002", Size: 58},
		},
		details: map[string]maildb.MessageDetail{
			"msg-001": {ID: "msg-001", StoragePath: "path/msg-001"},
			"msg-002": {ID: "msg-002", StoragePath: "path/msg-002"},
		},
	}
	store := &pop3TestStore{
		bodies: map[string]string{
			"path/msg-001": "From: a@example.com\r\n\r\nHello\r\n",
			"path/msg-002": "From: b@example.com\r\n\r\nWorld\r\n",
		},
	}
	auth := &pop3TestAuth{validUser: "alice", validPass: "secret", userID: "user-1"}
	svc := New(repo, store)
	adapter := NewPOP3StoreAdapter(auth, svc)
	return adapter, repo, store
}

func TestPOP3StoreAdapterImplementsInterface(t *testing.T) {
	var _ pop3d.Store = POP3StoreAdapter{}
}

func TestPOP3StoreAdapterAuthenticate(t *testing.T) {
	adapter, _, _ := newPOP3TestSetup()

	mb, err := adapter.Authenticate("alice", "secret")
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if mb == nil {
		t.Fatal("expected non-nil mailbox")
	}
	if mb.MessageCount() != 2 {
		t.Fatalf("expected 2 messages, got %d", mb.MessageCount())
	}
}

func TestPOP3StoreAdapterPassesNormalizedUsernameToAuthenticator(t *testing.T) {
	repo := &pop3TestRepository{
		folders:  []maildb.Folder{{ID: "inbox", SystemType: "inbox"}},
		messages: []maildb.MessageSummary{},
		details:  map[string]maildb.MessageDetail{},
	}
	svc := New(repo, &pop3TestStore{bodies: map[string]string{}})
	auth := &pop3TestAuth{validUser: "alice", validPass: "secret", userID: "user-1"}
	adapter := NewPOP3StoreAdapter(auth, svc)

	if _, err := adapter.Authenticate(" alice ", "secret"); err != nil {
		t.Fatalf("Authenticate returned error: %v", err)
	}
	if len(auth.usernames) != 1 || auth.usernames[0] != "alice" {
		t.Fatalf("auth usernames = %#v, want [alice]", auth.usernames)
	}
}

func TestPOP3StoreAdapterPreservesPasswordForAuthenticator(t *testing.T) {
	repo := &pop3TestRepository{
		folders:  []maildb.Folder{{ID: "inbox", SystemType: "inbox"}},
		messages: []maildb.MessageSummary{},
		details:  map[string]maildb.MessageDetail{},
	}
	svc := New(repo, &pop3TestStore{bodies: map[string]string{}})
	auth := &pop3TestAuth{validUser: "alice", validPass: " secret ", userID: "user-1"}
	adapter := NewPOP3StoreAdapter(auth, svc)

	if _, err := adapter.Authenticate("alice", " secret "); err != nil {
		t.Fatalf("Authenticate returned error: %v", err)
	}
	if len(auth.passwords) != 1 || auth.passwords[0] != " secret " {
		t.Fatalf("auth passwords = %#v, want preserved password", auth.passwords)
	}
}

func TestPOP3StoreAdapterMailboxLockKeyUsesUserID(t *testing.T) {
	adapter, _, _ := newPOP3TestSetup()

	mb, err := adapter.Authenticate("alice", "secret")
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	keyed, ok := mb.(interface{ MaildropLockKey() string })
	if !ok {
		t.Fatal("mailbox does not expose MaildropLockKey")
	}
	if got := keyed.MaildropLockKey(); got != "user-1" {
		t.Fatalf("expected lock key user-1, got %s", got)
	}
}

func TestPOP3StoreAdapterAuthenticateLoadsAllInboxPages(t *testing.T) {
	messages := make([]maildb.MessageSummary, 450)
	base := time.Date(2026, 5, 14, 12, 0, 0, 0, time.UTC)
	for i := range messages {
		messages[i] = maildb.MessageSummary{
			ID:         fmt.Sprintf("00000000-0000-0000-0000-%012d", i+1),
			Size:       int64(100 + i),
			ReceivedAt: base.Add(-time.Duration(i) * time.Minute),
		}
	}
	repo := &pop3TestRepository{
		folders:  []maildb.Folder{{ID: "folder-inbox", Name: "Inbox", SystemType: "inbox"}},
		messages: messages,
		details:  map[string]maildb.MessageDetail{},
	}
	svc := New(repo, &pop3TestStore{bodies: map[string]string{}})
	auth := &pop3TestAuth{validUser: "alice", validPass: "secret", userID: "user-1"}
	adapter := NewPOP3StoreAdapter(auth, svc)

	mb, err := adapter.Authenticate("alice", "secret")
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if got := mb.MessageCount(); got != len(messages) {
		t.Fatalf("expected %d messages, got %d", len(messages), got)
	}
	if got := mb.MessageUIDL(0); got != messages[0].ID {
		t.Fatalf("expected first UIDL %s, got %s", messages[0].ID, got)
	}
	if got := mb.MessageUIDL(len(messages) - 1); got != messages[len(messages)-1].ID {
		t.Fatalf("expected last UIDL %s, got %s", messages[len(messages)-1].ID, got)
	}
	if repo.pageCalls != 3 {
		t.Fatalf("expected 3 page calls, got %d", repo.pageCalls)
	}
	if len(repo.pageCursors) != 3 {
		t.Fatalf("page cursors = %#v, want 3 cursors", repo.pageCursors)
	}
	if repo.pageCursors[0] != (maildb.MessageListCursor{}) {
		t.Fatalf("first page cursor = %#v, want zero cursor", repo.pageCursors[0])
	}
	if repo.pageCursors[1].ID != messages[199].ID {
		t.Fatalf("second page cursor ID = %s, want %s", repo.pageCursors[1].ID, messages[199].ID)
	}
	if repo.pageCursors[2].ID != messages[399].ID {
		t.Fatalf("third page cursor ID = %s, want %s", repo.pageCursors[2].ID, messages[399].ID)
	}
}

func TestPOP3StoreAdapterAuthenticateEmptyInbox(t *testing.T) {
	repo := &pop3TestRepository{
		folders:  []maildb.Folder{{ID: "folder-inbox", Name: "Inbox", SystemType: "inbox"}},
		messages: []maildb.MessageSummary{},
		details:  map[string]maildb.MessageDetail{},
	}
	svc := New(repo, &pop3TestStore{bodies: map[string]string{}})
	auth := &pop3TestAuth{validUser: "alice", validPass: "secret", userID: "user-1"}
	adapter := NewPOP3StoreAdapter(auth, svc)

	mb, err := adapter.Authenticate("alice", "secret")
	if err != nil {
		t.Fatalf("Authenticate returned error: %v", err)
	}
	if got := mb.MessageCount(); got != 0 {
		t.Fatalf("message count = %d, want 0", got)
	}
	if repo.pageCalls != 1 {
		t.Fatalf("page calls = %d, want 1", repo.pageCalls)
	}
	if len(repo.pageCursors) != 1 || repo.pageCursors[0] != (maildb.MessageListCursor{}) {
		t.Fatalf("page cursors = %#v, want one zero cursor", repo.pageCursors)
	}
}

func TestPOP3StoreAdapterFindsInboxFolderCaseInsensitively(t *testing.T) {
	repo := &pop3TestRepository{
		folders:  []maildb.Folder{{ID: "folder-inbox", Name: "Inbox", SystemType: "INBOX"}},
		messages: []maildb.MessageSummary{{ID: "msg-001", Size: 42}},
		details:  map[string]maildb.MessageDetail{"msg-001": {ID: "msg-001", StoragePath: "path/msg-001"}},
	}
	store := &pop3TestStore{bodies: map[string]string{"path/msg-001": "From: a@example.com\r\n\r\nHello\r\n"}}
	svc := New(repo, store)
	auth := &pop3TestAuth{validUser: "alice", validPass: "secret", userID: "user-1"}
	adapter := NewPOP3StoreAdapter(auth, svc)

	mb, err := adapter.Authenticate("alice", "secret")
	if err != nil {
		t.Fatalf("Authenticate returned error: %v", err)
	}
	if got := mb.MessageCount(); got != 1 {
		t.Fatalf("message count = %d, want 1", got)
	}
	if len(repo.pageUsers) != 1 || repo.pageUsers[0] != "user-1" {
		t.Fatalf("page users = %#v, want [user-1]", repo.pageUsers)
	}
}

func TestPOP3StoreAdapterUsesFirstInboxFolderMatch(t *testing.T) {
	repo := &pop3TestRepository{
		folders: []maildb.Folder{
			{ID: "folder-archive", Name: "Archive", SystemType: "archive"},
			{ID: "folder-inbox-primary", Name: "Inbox", SystemType: "inbox"},
			{ID: "folder-inbox-secondary", Name: "Inbox Copy", SystemType: "INBOX"},
		},
		messages: []maildb.MessageSummary{{ID: "msg-001", Size: 42}},
		details:  map[string]maildb.MessageDetail{"msg-001": {ID: "msg-001", StoragePath: "path/msg-001"}},
	}
	store := &pop3TestStore{bodies: map[string]string{"path/msg-001": "From: a@example.com\r\n\r\nHello\r\n"}}
	svc := New(repo, store)
	auth := &pop3TestAuth{validUser: "alice", validPass: "secret", userID: "user-1"}
	adapter := NewPOP3StoreAdapter(auth, svc)

	mb, err := adapter.Authenticate("alice", "secret")
	if err != nil {
		t.Fatalf("Authenticate returned error: %v", err)
	}
	if got := mb.MessageCount(); got != 1 {
		t.Fatalf("message count = %d, want 1", got)
	}
	if len(repo.pageFolders) != 1 || repo.pageFolders[0] != "folder-inbox-primary" {
		t.Fatalf("page folders = %#v, want [folder-inbox-primary]", repo.pageFolders)
	}
}

func TestPOP3StoreAdapterMissingInboxSkipsMessagePageLookup(t *testing.T) {
	repo := &pop3TestRepository{
		folders:  []maildb.Folder{{ID: "folder-archive", Name: "Archive", SystemType: "archive"}},
		messages: []maildb.MessageSummary{{ID: "msg-001", Size: 42}},
		details:  map[string]maildb.MessageDetail{"msg-001": {ID: "msg-001", StoragePath: "path/msg-001"}},
	}
	svc := New(repo, &pop3TestStore{bodies: map[string]string{}})
	auth := &pop3TestAuth{validUser: "alice", validPass: "secret", userID: "user-1"}
	adapter := NewPOP3StoreAdapter(auth, svc)

	if _, err := adapter.Authenticate("alice", "secret"); err == nil {
		t.Fatal("expected missing inbox error")
	}
	if len(repo.folderUsers) != 1 || repo.folderUsers[0] != "user-1" {
		t.Fatalf("folder users = %#v, want [user-1]", repo.folderUsers)
	}
	if len(repo.pageFolders) != 0 {
		t.Fatalf("page folders = %#v, want no message page lookup", repo.pageFolders)
	}
}

func TestPOP3StoreAdapterFolderListingErrorSkipsMessagePageLookup(t *testing.T) {
	repo := &pop3TestRepository{
		folderErr: fmt.Errorf("folder list failed"),
		messages:  []maildb.MessageSummary{{ID: "msg-001", Size: 42}},
		details:   map[string]maildb.MessageDetail{"msg-001": {ID: "msg-001", StoragePath: "path/msg-001"}},
	}
	svc := New(repo, &pop3TestStore{bodies: map[string]string{}})
	auth := &pop3TestAuth{validUser: "alice", validPass: "secret", userID: "user-1"}
	adapter := NewPOP3StoreAdapter(auth, svc)

	if _, err := adapter.Authenticate("alice", "secret"); err == nil || !strings.Contains(err.Error(), "list folders") {
		t.Fatalf("expected list folders error, got %v", err)
	}
	if len(repo.folderUsers) != 1 || repo.folderUsers[0] != "user-1" {
		t.Fatalf("folder users = %#v, want [user-1]", repo.folderUsers)
	}
	if len(repo.pageFolders) != 0 {
		t.Fatalf("page folders = %#v, want no message page lookup", repo.pageFolders)
	}
}

func TestPOP3StoreAdapterMessagePageErrorPropagates(t *testing.T) {
	repo := &pop3TestRepository{
		folders:  []maildb.Folder{{ID: "folder-inbox", Name: "Inbox", SystemType: "inbox"}},
		pageErr:  fmt.Errorf("page list failed"),
		messages: []maildb.MessageSummary{{ID: "msg-001", Size: 42}},
		details:  map[string]maildb.MessageDetail{"msg-001": {ID: "msg-001", StoragePath: "path/msg-001"}},
	}
	svc := New(repo, &pop3TestStore{bodies: map[string]string{}})
	auth := &pop3TestAuth{validUser: "alice", validPass: "secret", userID: "user-1"}
	adapter := NewPOP3StoreAdapter(auth, svc)

	if _, err := adapter.Authenticate("alice", "secret"); err == nil || !strings.Contains(err.Error(), "list inbox messages") {
		t.Fatalf("expected list inbox messages error, got %v", err)
	}
	if repo.pageCalls != 1 {
		t.Fatalf("page calls = %d, want 1", repo.pageCalls)
	}
	if len(repo.pageFolders) != 1 || repo.pageFolders[0] != "folder-inbox" {
		t.Fatalf("page folders = %#v, want [folder-inbox]", repo.pageFolders)
	}
}

func TestPOP3StoreAdapterMessagePageCursorDecodeErrorPropagates(t *testing.T) {
	messages := make([]maildb.MessageSummary, maildb.MessageListMaxLimit+1)
	base := time.Date(2026, 5, 14, 12, 0, 0, 0, time.UTC)
	for i := range messages {
		messages[i] = maildb.MessageSummary{
			ID:         fmt.Sprintf("msg-%03d", i+1),
			Size:       int64(100 + i),
			ReceivedAt: base.Add(-time.Duration(i) * time.Minute),
		}
	}
	repo := &pop3TestRepository{
		folders:  []maildb.Folder{{ID: "folder-inbox", Name: "Inbox", SystemType: "inbox"}},
		messages: messages,
		details:  map[string]maildb.MessageDetail{},
	}
	svc := New(repo, &pop3TestStore{bodies: map[string]string{}})
	auth := &pop3TestAuth{validUser: "alice", validPass: "secret", userID: "user-1"}
	adapter := NewPOP3StoreAdapter(auth, svc)

	if _, err := adapter.Authenticate("alice", "secret"); err == nil || !strings.Contains(err.Error(), "decode inbox cursor") {
		t.Fatalf("expected decode inbox cursor error, got %v", err)
	}
	if repo.pageCalls != 1 {
		t.Fatalf("page calls = %d, want 1", repo.pageCalls)
	}
	if len(repo.pageFolders) != 1 || repo.pageFolders[0] != "folder-inbox" {
		t.Fatalf("page folders = %#v, want [folder-inbox]", repo.pageFolders)
	}
}

func TestPOP3StoreAdapterMissingMessagePageCursorFails(t *testing.T) {
	messages := make([]maildb.MessageSummary, maildb.MessageListMaxLimit+1)
	for i := range messages {
		messages[i] = maildb.MessageSummary{
			ID:   fmt.Sprintf("00000000-0000-0000-0000-%012d", i+1),
			Size: int64(100 + i),
		}
	}
	repo := &pop3TestRepository{
		folders:  []maildb.Folder{{ID: "folder-inbox", Name: "Inbox", SystemType: "inbox"}},
		messages: messages,
		details:  map[string]maildb.MessageDetail{},
	}
	svc := New(repo, &pop3TestStore{bodies: map[string]string{}})
	auth := &pop3TestAuth{validUser: "alice", validPass: "secret", userID: "user-1"}
	adapter := NewPOP3StoreAdapter(auth, svc)

	if _, err := adapter.Authenticate("alice", "secret"); err == nil || !strings.Contains(err.Error(), "missing inbox cursor") {
		t.Fatalf("expected missing inbox cursor error, got %v", err)
	}
	if repo.pageCalls != 1 {
		t.Fatalf("page calls = %d, want 1", repo.pageCalls)
	}
	if len(repo.pageFolders) != 1 || repo.pageFolders[0] != "folder-inbox" {
		t.Fatalf("page folders = %#v, want [folder-inbox]", repo.pageFolders)
	}
}

func TestPOP3StoreAdapterAuthenticateFail(t *testing.T) {
	adapter, repo, _ := newPOP3TestSetup()
	auth := adapter.authenticator.(*pop3TestAuth)

	_, err := adapter.Authenticate("alice", "wrong")
	if err == nil {
		t.Fatal("expected authentication error for wrong password")
	}
	if auth.calls != 1 {
		t.Fatalf("auth calls = %d, want 1", auth.calls)
	}
	if len(repo.folderUsers) != 0 {
		t.Fatalf("folder users = %#v, want no service lookup", repo.folderUsers)
	}
	if len(repo.pageUsers) != 0 {
		t.Fatalf("page users = %#v, want no service lookup", repo.pageUsers)
	}
}

func TestNormalizePOP3Username(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "plain", input: "alice", want: "alice"},
		{name: "trim spaces", input: " alice ", want: "alice"},
		{name: "empty", input: " \t ", wantErr: true},
		{name: "carriage return", input: "ali\rce", wantErr: true},
		{name: "line feed", input: "ali\nce", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizePOP3Username(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("normalize returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("normalize = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidatePOP3Password(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "plain", input: "secret"},
		{name: "empty"},
		{name: "spaces", input: " secret "},
		{name: "carriage return", input: "sec\rret", wantErr: true},
		{name: "line feed", input: "sec\nret", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePOP3Password(tt.input)
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("validate returned error: %v", err)
			}
		})
	}
}

func TestPOP3StoreAdapterRejectsInvalidCredentialsBeforeAuthenticator(t *testing.T) {
	tests := []struct {
		name     string
		username string
		password string
	}{
		{name: "empty username", username: " \t ", password: "secret"},
		{name: "username crlf", username: "ali\r\nce", password: "secret"},
		{name: "password crlf", username: "alice", password: "sec\r\nret"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &pop3TestRepository{
				folders:  []maildb.Folder{{ID: "inbox", SystemType: "inbox"}},
				messages: []maildb.MessageSummary{},
				details:  map[string]maildb.MessageDetail{},
			}
			svc := New(repo, &pop3TestStore{bodies: map[string]string{}})
			auth := &pop3TestAuth{validUser: "alice", validPass: "secret", userID: "user-1"}
			adapter := NewPOP3StoreAdapter(auth, svc)

			if _, err := adapter.Authenticate(tt.username, tt.password); err == nil {
				t.Fatal("expected invalid credentials to be rejected")
			}
			if auth.calls != 0 {
				t.Fatalf("auth calls = %d, want 0", auth.calls)
			}
			if len(repo.folderUsers) != 0 {
				t.Fatalf("folder users = %#v, want no service lookup", repo.folderUsers)
			}
		})
	}
}

func TestPOP3StoreAdapterAuthenticateNilAuth(t *testing.T) {
	repo := &pop3TestRepository{
		folders:  []maildb.Folder{{ID: "inbox", SystemType: "inbox"}},
		messages: []maildb.MessageSummary{},
		details:  map[string]maildb.MessageDetail{},
	}
	svc := New(repo, &pop3TestStore{bodies: map[string]string{}})
	adapter := NewPOP3StoreAdapter(nil, svc)

	_, err := adapter.Authenticate("alice", "secret")
	if err == nil {
		t.Fatal("expected error for nil authenticator")
	}
}

func TestPOP3StoreAdapterRejectsMustChangePassword(t *testing.T) {
	repo := &pop3TestRepository{
		folders:  []maildb.Folder{{ID: "inbox", SystemType: "inbox"}},
		messages: []maildb.MessageSummary{},
		details:  map[string]maildb.MessageDetail{},
	}
	svc := New(repo, &pop3TestStore{bodies: map[string]string{}})
	auth := &pop3TestAuth{validUser: "alice", validPass: "secret", userID: "user-1", mustChangePassword: true}
	adapter := NewPOP3StoreAdapter(auth, svc)

	if _, err := adapter.Authenticate("alice", "secret"); err == nil {
		t.Fatal("expected error for user that must change password")
	}
	if auth.calls != 1 {
		t.Fatalf("auth calls = %d, want 1", auth.calls)
	}
	if len(repo.folderUsers) != 0 {
		t.Fatalf("folder users = %#v, want no service lookup", repo.folderUsers)
	}
	if len(repo.pageUsers) != 0 {
		t.Fatalf("page users = %#v, want no service lookup", repo.pageUsers)
	}
}

func TestPOP3StoreAdapterRechecksAuthPolicyEachLogin(t *testing.T) {
	repo := &pop3TestRepository{
		folders:  []maildb.Folder{{ID: "inbox", SystemType: "inbox"}},
		messages: []maildb.MessageSummary{},
		details:  map[string]maildb.MessageDetail{},
	}
	svc := New(repo, &pop3TestStore{bodies: map[string]string{}})
	auth := &pop3TestAuth{validUser: "alice", validPass: "secret", userID: "user-1"}
	adapter := NewPOP3StoreAdapter(auth, svc)

	if _, err := adapter.Authenticate("alice", "secret"); err != nil {
		t.Fatalf("initial Authenticate returned error: %v", err)
	}
	auth.mustChangePassword = true
	if _, err := adapter.Authenticate("alice", "secret"); err == nil {
		t.Fatal("expected second Authenticate to use fresh must-change-password policy")
	}
	if auth.calls != 2 {
		t.Fatalf("auth calls = %d, want 2", auth.calls)
	}
}

func TestPOP3StoreAdapterUsesFreshAuthenticatedUserID(t *testing.T) {
	repo := &pop3TestRepository{
		folders:  []maildb.Folder{{ID: "inbox", SystemType: "inbox"}},
		messages: []maildb.MessageSummary{},
		details:  map[string]maildb.MessageDetail{},
	}
	svc := New(repo, &pop3TestStore{bodies: map[string]string{}})
	auth := &pop3TestAuth{validUser: "alice", validPass: "secret", userID: "user-1"}
	adapter := NewPOP3StoreAdapter(auth, svc)

	first, err := adapter.Authenticate("alice", "secret")
	if err != nil {
		t.Fatalf("initial Authenticate returned error: %v", err)
	}
	auth.userID = "user-2"
	second, err := adapter.Authenticate("alice", "secret")
	if err != nil {
		t.Fatalf("second Authenticate returned error: %v", err)
	}
	firstLock, ok := first.(interface{ MaildropLockKey() string })
	if !ok {
		t.Fatal("first mailbox does not expose MaildropLockKey")
	}
	secondLock, ok := second.(interface{ MaildropLockKey() string })
	if !ok {
		t.Fatal("second mailbox does not expose MaildropLockKey")
	}
	if firstLock.MaildropLockKey() != "user-1" || secondLock.MaildropLockKey() != "user-2" {
		t.Fatalf("maildrop lock keys = %q/%q, want user-1/user-2", firstLock.MaildropLockKey(), secondLock.MaildropLockKey())
	}
	if len(repo.folderUsers) != 2 || repo.folderUsers[0] != "user-1" || repo.folderUsers[1] != "user-2" {
		t.Fatalf("folder users = %#v, want user-1 then user-2", repo.folderUsers)
	}
	if len(repo.pageUsers) != 2 || repo.pageUsers[0] != "user-1" || repo.pageUsers[1] != "user-2" {
		t.Fatalf("page users = %#v, want user-1 then user-2", repo.pageUsers)
	}
}

func TestPOP3StoreAdapterTrimsAuthenticatedUserID(t *testing.T) {
	repo := &pop3TestRepository{
		folders:  []maildb.Folder{{ID: "inbox", SystemType: "inbox"}},
		messages: []maildb.MessageSummary{},
		details:  map[string]maildb.MessageDetail{},
	}
	svc := New(repo, &pop3TestStore{bodies: map[string]string{}})
	auth := &pop3TestAuth{validUser: "alice", validPass: "secret", userID: " user-1 "}
	adapter := NewPOP3StoreAdapter(auth, svc)

	mb, err := adapter.Authenticate("alice", "secret")
	if err != nil {
		t.Fatalf("Authenticate returned error: %v", err)
	}
	keyed, ok := mb.(interface{ MaildropLockKey() string })
	if !ok {
		t.Fatal("mailbox does not expose MaildropLockKey")
	}
	if got := keyed.MaildropLockKey(); got != "user-1" {
		t.Fatalf("maildrop lock key = %q, want user-1", got)
	}
	if len(repo.folderUsers) != 1 || repo.folderUsers[0] != "user-1" {
		t.Fatalf("folder users = %#v, want [user-1]", repo.folderUsers)
	}
	if len(repo.pageUsers) != 1 || repo.pageUsers[0] != "user-1" {
		t.Fatalf("page users = %#v, want [user-1]", repo.pageUsers)
	}
}

func TestPOP3StoreAdapterRejectsEmptyAuthenticatedUserID(t *testing.T) {
	repo := &pop3TestRepository{
		folders:  []maildb.Folder{{ID: "inbox", SystemType: "inbox"}},
		messages: []maildb.MessageSummary{},
		details:  map[string]maildb.MessageDetail{},
	}
	svc := New(repo, &pop3TestStore{bodies: map[string]string{}})
	auth := &pop3TestAuth{validUser: "alice", validPass: "secret", userID: " \t "}
	adapter := NewPOP3StoreAdapter(auth, svc)

	if _, err := adapter.Authenticate("alice", "secret"); err == nil {
		t.Fatal("expected empty authenticated user ID to be rejected")
	}
	if auth.calls != 1 {
		t.Fatalf("auth calls = %d, want 1", auth.calls)
	}
	if len(repo.folderUsers) != 0 {
		t.Fatalf("folder users = %#v, want no service lookup", repo.folderUsers)
	}
	if len(repo.pageUsers) != 0 {
		t.Fatalf("page users = %#v, want no service lookup", repo.pageUsers)
	}
}

func TestPOP3StoreAdapterRejectsControlCharacterAuthenticatedUserID(t *testing.T) {
	repo := &pop3TestRepository{
		folders:  []maildb.Folder{{ID: "inbox", SystemType: "inbox"}},
		messages: []maildb.MessageSummary{},
		details:  map[string]maildb.MessageDetail{},
	}
	svc := New(repo, &pop3TestStore{bodies: map[string]string{}})
	auth := &pop3TestAuth{validUser: "alice", validPass: "secret", userID: "user-1\r\nuser-2"}
	adapter := NewPOP3StoreAdapter(auth, svc)

	if _, err := adapter.Authenticate("alice", "secret"); err == nil {
		t.Fatal("expected authenticated user ID with CRLF to be rejected")
	}
	if auth.calls != 1 {
		t.Fatalf("auth calls = %d, want 1", auth.calls)
	}
	if len(repo.folderUsers) != 0 {
		t.Fatalf("folder users = %#v, want no service lookup", repo.folderUsers)
	}
	if len(repo.pageUsers) != 0 {
		t.Fatalf("page users = %#v, want no service lookup", repo.pageUsers)
	}
}

func TestNormalizePOP3AuthenticatedUserID(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "plain", input: "user-1", want: "user-1"},
		{name: "trim spaces", input: " user-1 ", want: "user-1"},
		{name: "empty", input: " \t ", wantErr: true},
		{name: "carriage return", input: "user-1\ruser-2", wantErr: true},
		{name: "line feed", input: "user-1\nuser-2", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizePOP3AuthenticatedUserID(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("normalize returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("normalize = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizePOP3MessageSize(t *testing.T) {
	tests := []struct {
		name string
		size int64
		want int
	}{
		{name: "negative", size: -1, want: 0},
		{name: "zero", size: 0, want: 0},
		{name: "positive", size: 42, want: 42},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizePOP3MessageSize(tt.size); got != tt.want {
				t.Fatalf("normalize size = %d, want %d", got, tt.want)
			}
		})
	}

	if got := normalizePOP3MessageSizeForInt(101, 100); got != 100 {
		t.Fatalf("normalize oversized size = %d, want 100", got)
	}
}

func TestPOP3MailboxMessageSize(t *testing.T) {
	adapter, _, _ := newPOP3TestSetup()
	mb, _ := adapter.Authenticate("alice", "secret")

	if got := mb.MessageSize(0); got != 42 {
		t.Fatalf("expected size 42, got %d", got)
	}
	if got := mb.MessageSize(1); got != 58 {
		t.Fatalf("expected size 58, got %d", got)
	}
}

func TestPOP3StoreAdapterNormalizesMessageSizes(t *testing.T) {
	repo := &pop3TestRepository{
		folders: []maildb.Folder{{ID: "folder-inbox", Name: "Inbox", SystemType: "inbox"}},
		messages: []maildb.MessageSummary{
			{ID: "msg-negative", Size: -5},
			{ID: "msg-zero", Size: 0},
			{ID: "msg-positive", Size: 42},
		},
		details: map[string]maildb.MessageDetail{},
	}
	svc := New(repo, &pop3TestStore{bodies: map[string]string{}})
	auth := &pop3TestAuth{validUser: "alice", validPass: "secret", userID: "user-1"}
	adapter := NewPOP3StoreAdapter(auth, svc)

	mb, err := adapter.Authenticate("alice", "secret")
	if err != nil {
		t.Fatalf("Authenticate returned error: %v", err)
	}
	if got := mb.MessageSize(0); got != 0 {
		t.Fatalf("negative size normalized to %d, want 0", got)
	}
	if got := mb.MessageSize(1); got != 0 {
		t.Fatalf("zero size normalized to %d, want 0", got)
	}
	if got := mb.MessageSize(2); got != 42 {
		t.Fatalf("positive size normalized to %d, want 42", got)
	}
}

func TestPOP3MailboxMessageSizeInvalidIndex(t *testing.T) {
	adapter, _, _ := newPOP3TestSetup()
	mb, _ := adapter.Authenticate("alice", "secret")

	if got := mb.MessageSize(-1); got != 0 {
		t.Fatalf("negative index size = %d, want 0", got)
	}
	if got := mb.MessageSize(mb.MessageCount()); got != 0 {
		t.Fatalf("out-of-range index size = %d, want 0", got)
	}
}

func TestPOP3MailboxMessageUIDL(t *testing.T) {
	adapter, _, _ := newPOP3TestSetup()
	mb, _ := adapter.Authenticate("alice", "secret")

	if got := mb.MessageUIDL(0); got != "msg-001" {
		t.Fatalf("expected msg-001, got %s", got)
	}
	if got := mb.MessageUIDL(1); got != "msg-002" {
		t.Fatalf("expected msg-002, got %s", got)
	}
}

func TestPOP3MailboxMessageUIDLInvalidIndex(t *testing.T) {
	adapter, _, _ := newPOP3TestSetup()
	mb, _ := adapter.Authenticate("alice", "secret")

	if got := mb.MessageUIDL(-1); got != "" {
		t.Fatalf("negative index UIDL = %q, want empty", got)
	}
	if got := mb.MessageUIDL(mb.MessageCount()); got != "" {
		t.Fatalf("out-of-range index UIDL = %q, want empty", got)
	}
}

func TestPOP3MailboxMessageContent(t *testing.T) {
	adapter, _, _ := newPOP3TestSetup()
	mb, _ := adapter.Authenticate("alice", "secret")

	content := mb.MessageContent(0)
	if content == "" {
		t.Fatal("expected non-empty content")
	}
	if !strings.Contains(content, "From: a@example.com") {
		t.Fatalf("unexpected content: %q", content)
	}
}

func TestPOP3MailboxMessageContentInvalidIndex(t *testing.T) {
	adapter, _, _ := newPOP3TestSetup()
	mb, _ := adapter.Authenticate("alice", "secret")

	if got := mb.MessageContent(-1); got != "" {
		t.Fatalf("negative index content = %q, want empty", got)
	}
	if got := mb.MessageContent(mb.MessageCount()); got != "" {
		t.Fatalf("out-of-range index content = %q, want empty", got)
	}
	contentWithError, ok := mb.(interface {
		MessageContentWithError(int) (string, error)
	})
	if !ok {
		t.Fatal("mailbox does not expose MessageContentWithError")
	}
	if _, err := contentWithError.MessageContentWithError(-1); err == nil {
		t.Fatal("expected error for negative content index")
	}
	if _, err := contentWithError.MessageContentWithError(mb.MessageCount()); err == nil {
		t.Fatal("expected error for out-of-range content index")
	}
}

func TestPOP3MailboxMessageContentLazyLoad(t *testing.T) {
	adapter, _, _ := newPOP3TestSetup()
	mb, _ := adapter.Authenticate("alice", "secret")

	// call twice — should not error on second call
	c1 := mb.MessageContent(0)
	c2 := mb.MessageContent(0)
	if c1 != c2 {
		t.Fatal("expected same content on repeated call")
	}
}

func TestPOP3MailboxMessageContentDeleted(t *testing.T) {
	adapter, _, _ := newPOP3TestSetup()
	mb, _ := adapter.Authenticate("alice", "secret")

	if err := mb.MarkDeleted(0); err != nil {
		t.Fatalf("mark deleted: %v", err)
	}
	if got := mb.MessageContent(0); got != "" {
		t.Fatalf("deleted content = %q, want empty", got)
	}
	contentWithError, ok := mb.(interface {
		MessageContentWithError(int) (string, error)
	})
	if !ok {
		t.Fatal("mailbox does not expose MessageContentWithError")
	}
	if _, err := contentWithError.MessageContentWithError(0); err == nil {
		t.Fatal("expected error for deleted content")
	}
}

func TestPOP3MailboxResetRestoresContentAccess(t *testing.T) {
	adapter, _, _ := newPOP3TestSetup()
	mb, _ := adapter.Authenticate("alice", "secret")

	if err := mb.MarkDeleted(0); err != nil {
		t.Fatalf("mark deleted: %v", err)
	}
	mb.ResetDeleted()
	contentWithError, ok := mb.(interface {
		MessageContentWithError(int) (string, error)
	})
	if !ok {
		t.Fatal("mailbox does not expose MessageContentWithError")
	}
	content, err := contentWithError.MessageContentWithError(0)
	if err != nil {
		t.Fatalf("content after reset: %v", err)
	}
	if !strings.Contains(content, "From: a@example.com") {
		t.Fatalf("unexpected content after reset: %q", content)
	}
}

func TestPOP3MailboxMarkDeletedAndReset(t *testing.T) {
	adapter, _, _ := newPOP3TestSetup()
	mb, _ := adapter.Authenticate("alice", "secret")

	if err := mb.MarkDeleted(0); err != nil {
		t.Fatalf("mark deleted: %v", err)
	}
	if !mb.Deleted(0) {
		t.Fatal("expected message 0 to be deleted")
	}
	if mb.Deleted(1) {
		t.Fatal("expected message 1 to not be deleted")
	}

	mb.ResetDeleted()
	if mb.Deleted(0) {
		t.Fatal("expected message 0 to be un-deleted after reset")
	}
}

func TestPOP3MailboxMarkDeletedDuplicate(t *testing.T) {
	adapter, repo, _ := newPOP3TestSetup()
	mb, _ := adapter.Authenticate("alice", "secret")

	mailbox, ok := mb.(*pop3Mailbox)
	if !ok {
		t.Fatal("expected pop3 mailbox adapter")
	}
	if err := mailbox.MarkDeleted(0); err != nil {
		t.Fatalf("mark deleted: %v", err)
	}
	if err := mailbox.MarkDeleted(0); err != nil {
		t.Fatalf("mark deleted again: %v", err)
	}
	if len(mailbox.pending) != 1 || mailbox.pending[0] != "msg-001" {
		t.Fatalf("pending deletes = %#v, want one msg-001", mailbox.pending)
	}
	if err := mailbox.CommitDeletes(); err != nil {
		t.Fatalf("commit deletes: %v", err)
	}
	if got := repo.lastBulkDelete.MessageIDs; len(got) != 1 || got[0] != "msg-001" {
		t.Fatalf("deleted IDs = %#v, want [msg-001]", got)
	}
}

func TestPOP3MailboxSizeZeroWhenDeleted(t *testing.T) {
	adapter, _, _ := newPOP3TestSetup()
	mb, _ := adapter.Authenticate("alice", "secret")

	_ = mb.MarkDeleted(0)
	if got := mb.MessageSize(0); got != 0 {
		t.Fatalf("expected size 0 for deleted message, got %d", got)
	}
}

func TestPOP3MailboxCommitDeletes(t *testing.T) {
	adapter, repo, _ := newPOP3TestSetup()
	mb, _ := adapter.Authenticate("alice", "secret")

	_ = mb.MarkDeleted(0)
	_ = mb.MarkDeleted(1)

	committer, ok := mb.(interface{ CommitDeletes() error })
	if !ok {
		t.Fatal("mailbox does not implement CommitDeletes")
	}
	if err := committer.CommitDeletes(); err != nil {
		t.Fatalf("commit deletes: %v", err)
	}
	if len(repo.lastBulkDelete.MessageIDs) != 2 {
		t.Fatalf("expected 2 deleted messages, got %d", len(repo.lastBulkDelete.MessageIDs))
	}
}

func TestPOP3MailboxCommitDeletesDeduplicatesPendingIDs(t *testing.T) {
	adapter, repo, _ := newPOP3TestSetup()
	mb, _ := adapter.Authenticate("alice", "secret")

	mailbox, ok := mb.(*pop3Mailbox)
	if !ok {
		t.Fatal("expected pop3 mailbox adapter")
	}
	mailbox.pending = []string{" msg-001 ", "msg-001", "", "msg-002", "msg-002"}

	if err := mailbox.CommitDeletes(); err != nil {
		t.Fatalf("commit deletes: %v", err)
	}
	got := repo.lastBulkDelete.MessageIDs
	if len(got) != 2 || got[0] != "msg-001" || got[1] != "msg-002" {
		t.Fatalf("expected unique pending deletes [msg-001 msg-002], got %#v", got)
	}
	if len(mailbox.pending) != 0 {
		t.Fatalf("expected pending deletes to clear after success, got %#v", mailbox.pending)
	}
}

func TestPOP3MailboxCommitDeletesClearsPending(t *testing.T) {
	adapter, repo, _ := newPOP3TestSetup()
	mb, _ := adapter.Authenticate("alice", "secret")

	mailbox, ok := mb.(*pop3Mailbox)
	if !ok {
		t.Fatal("expected pop3 mailbox adapter")
	}
	if err := mailbox.MarkDeleted(0); err != nil {
		t.Fatalf("mark deleted: %v", err)
	}
	if err := mailbox.CommitDeletes(); err != nil {
		t.Fatalf("commit deletes: %v", err)
	}
	if len(mailbox.pending) != 0 {
		t.Fatalf("pending deletes = %#v, want empty", mailbox.pending)
	}
	if repo.bulkDeleteCalls != 1 {
		t.Fatalf("bulk delete calls = %d, want 1", repo.bulkDeleteCalls)
	}
	if got := repo.lastBulkDelete.MessageIDs; len(got) != 1 || got[0] != "msg-001" {
		t.Fatalf("deleted IDs = %#v, want [msg-001]", got)
	}
	if err := mailbox.CommitDeletes(); err != nil {
		t.Fatalf("second commit deletes: %v", err)
	}
	if repo.bulkDeleteCalls != 1 {
		t.Fatalf("bulk delete calls after second commit = %d, want 1", repo.bulkDeleteCalls)
	}
}

func TestPOP3MailboxCommitDeleteFailurePreservesPending(t *testing.T) {
	adapter, repo, _ := newPOP3TestSetup()
	repo.bulkDeleteErr = fmt.Errorf("bulk delete failed")
	mb, _ := adapter.Authenticate("alice", "secret")

	mailbox, ok := mb.(*pop3Mailbox)
	if !ok {
		t.Fatal("expected pop3 mailbox adapter")
	}
	if err := mailbox.MarkDeleted(0); err != nil {
		t.Fatalf("mark deleted: %v", err)
	}
	if err := mailbox.CommitDeletes(); err == nil {
		t.Fatal("expected commit delete error")
	}
	if repo.bulkDeleteCalls != 1 {
		t.Fatalf("bulk delete calls = %d, want 1", repo.bulkDeleteCalls)
	}
	if len(mailbox.pending) != 1 || mailbox.pending[0] != "msg-001" {
		t.Fatalf("pending deletes = %#v, want [msg-001]", mailbox.pending)
	}
	if !mailbox.Deleted(0) {
		t.Fatal("expected message to remain marked deleted after failed commit")
	}
}

func TestPOP3MailboxResetAfterCommitFailureClearsPending(t *testing.T) {
	adapter, repo, _ := newPOP3TestSetup()
	repo.bulkDeleteErr = fmt.Errorf("bulk delete failed")
	mb, _ := adapter.Authenticate("alice", "secret")

	mailbox, ok := mb.(*pop3Mailbox)
	if !ok {
		t.Fatal("expected pop3 mailbox adapter")
	}
	if err := mailbox.MarkDeleted(0); err != nil {
		t.Fatalf("mark deleted: %v", err)
	}
	if err := mailbox.CommitDeletes(); err == nil {
		t.Fatal("expected commit delete error")
	}
	mailbox.ResetDeleted()
	if len(mailbox.pending) != 0 {
		t.Fatalf("pending deletes after reset = %#v, want empty", mailbox.pending)
	}
	if mailbox.Deleted(0) {
		t.Fatal("expected message deletion flag to clear after reset")
	}
	repo.bulkDeleteErr = nil
	if err := mailbox.CommitDeletes(); err != nil {
		t.Fatalf("commit after reset: %v", err)
	}
	if repo.bulkDeleteCalls != 1 {
		t.Fatalf("bulk delete calls after reset commit = %d, want 1", repo.bulkDeleteCalls)
	}
}

func TestPOP3MailboxResetClearsPending(t *testing.T) {
	adapter, repo, _ := newPOP3TestSetup()
	mb, _ := adapter.Authenticate("alice", "secret")

	_ = mb.MarkDeleted(0)
	mb.ResetDeleted()

	committer := mb.(interface{ CommitDeletes() error })
	_ = committer.CommitDeletes()

	if len(repo.lastBulkDelete.MessageIDs) != 0 {
		t.Fatalf("expected 0 deleted messages after reset, got %d", len(repo.lastBulkDelete.MessageIDs))
	}
}

func TestPOP3MailboxMarkDeletedInvalidIndex(t *testing.T) {
	adapter, _, _ := newPOP3TestSetup()
	mb, _ := adapter.Authenticate("alice", "secret")

	if err := mb.MarkDeleted(99); err == nil {
		t.Fatal("expected error for invalid index")
	}
}

func TestPOP3AdapterNoStorage(t *testing.T) {
	repo := &pop3TestRepository{
		folders:  []maildb.Folder{{ID: "inbox", SystemType: "inbox"}},
		messages: []maildb.MessageSummary{{ID: "msg-x", Size: 10}},
		details:  map[string]maildb.MessageDetail{"msg-x": {ID: "msg-x", StoragePath: "path/x"}},
	}
	// nil store → FetchRawMessageBody returns error
	svc := New(repo, nil)
	auth := &pop3TestAuth{validUser: "alice", validPass: "secret", userID: "u1"}
	adapter := NewPOP3StoreAdapter(auth, svc)

	mb, err := adapter.Authenticate("alice", "secret")
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	content := mb.MessageContent(0)
	if content != "" {
		t.Fatalf("expected empty content for nil store, got %q", content)
	}
	contentWithError, ok := mb.(interface {
		MessageContentWithError(int) (string, error)
	})
	if !ok {
		t.Fatal("mailbox does not expose MessageContentWithError")
	}
	if _, err := contentWithError.MessageContentWithError(0); err == nil {
		t.Fatal("expected raw body fetch error")
	}
}

// Ensure storage.Store interface has the methods we use in the fake.
var _ storage.Store = (*pop3TestStore)(nil)
