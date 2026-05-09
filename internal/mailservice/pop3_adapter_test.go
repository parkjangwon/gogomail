package mailservice

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/gogomail/gogomail/internal/maildb"
	"github.com/gogomail/gogomail/internal/pop3d"
	smtpd "github.com/gogomail/gogomail/internal/smtp"
	"github.com/gogomail/gogomail/internal/storage"
)

// pop3TestRepository embeds fakeRepository and overrides return values for
// the methods the POP3 adapter actually calls.
type pop3TestRepository struct {
	fakeRepository
	folders  []maildb.Folder
	messages []maildb.MessageSummary
	details  map[string]maildb.MessageDetail
}

func (r *pop3TestRepository) ListFolders(_ context.Context, _ string) ([]maildb.Folder, error) {
	return r.folders, nil
}

func (r *pop3TestRepository) ListMessagesInFolder(_ context.Context, _, _ string, _ int) ([]maildb.MessageSummary, error) {
	return r.messages, nil
}

func (r *pop3TestRepository) GetMessage(_ context.Context, _, messageID string) (maildb.MessageDetail, error) {
	if d, ok := r.details[messageID]; ok {
		return d, nil
	}
	return maildb.MessageDetail{}, fmt.Errorf("message not found: %s", messageID)
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
	validUser string
	validPass string
	userID    string
}

func (a *pop3TestAuth) AuthenticatePlain(_ context.Context, _, username, password string) (smtpd.SubmissionUser, error) {
	if username == a.validUser && password == a.validPass {
		return smtpd.SubmissionUser{UserID: a.userID, Address: username}, nil
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

func TestPOP3StoreAdapterAuthenticateFail(t *testing.T) {
	adapter, _, _ := newPOP3TestSetup()

	_, err := adapter.Authenticate("alice", "wrong")
	if err == nil {
		t.Fatal("expected authentication error for wrong password")
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
}

// Ensure storage.Store interface has the methods we use in the fake.
var _ storage.Store = (*pop3TestStore)(nil)
