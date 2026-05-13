package mailservice

import (
	"context"
	"fmt"
	"strings"

	"github.com/gogomail/gogomail/internal/maildb"
	"github.com/gogomail/gogomail/internal/pop3d"
	smtpd "github.com/gogomail/gogomail/internal/smtp"
)

// POP3StoreAdapter bridges the POP3 server with the mail service.
type POP3StoreAdapter struct {
	authenticator smtpd.SubmissionAuthenticator
	service       *Service
}

var _ pop3d.Store = POP3StoreAdapter{}

// NewPOP3StoreAdapter creates a POP3StoreAdapter.
func NewPOP3StoreAdapter(authenticator smtpd.SubmissionAuthenticator, service *Service) POP3StoreAdapter {
	return POP3StoreAdapter{authenticator: authenticator, service: service}
}

// Authenticate verifies credentials and returns the user's INBOX as a Mailbox.
func (a POP3StoreAdapter) Authenticate(user, pass string) (pop3d.Mailbox, error) {
	if a.authenticator == nil {
		return nil, fmt.Errorf("pop3 authenticator is required")
	}
	user, err := normalizePOP3Username(user)
	if err != nil {
		return nil, err
	}
	if err := validatePOP3Password(pass); err != nil {
		return nil, err
	}

	ctx := context.Background()
	authUser, err := a.authenticator.AuthenticatePlain(ctx, "", user, pass)
	if err != nil {
		return nil, fmt.Errorf("authentication failed")
	}
	if authUser.MustChangePassword {
		return nil, fmt.Errorf("password change required")
	}
	userID, err := normalizePOP3AuthenticatedUserID(authUser.UserID)
	if err != nil {
		return nil, err
	}

	folders, err := a.service.ListFolders(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list folders: %w", err)
	}
	inboxID := ""
	for _, f := range folders {
		if strings.EqualFold(f.SystemType, "inbox") {
			inboxID = f.ID
			break
		}
	}
	if inboxID == "" {
		return nil, fmt.Errorf("inbox not found")
	}

	summaries, err := a.listInboxMessages(ctx, userID, inboxID)
	if err != nil {
		return nil, fmt.Errorf("list inbox messages: %w", err)
	}

	msgs := make([]pop3InboxMsg, len(summaries))
	for i, s := range summaries {
		msgs[i] = pop3InboxMsg{id: s.ID, size: int(s.Size)}
	}

	return &pop3Mailbox{
		ctx:     ctx,
		service: a.service,
		userID:  userID,
		msgs:    msgs,
		deleted: make([]bool, len(msgs)),
		content: make([]string, len(msgs)),
		loaded:  make([]bool, len(msgs)),
		pending: make([]string, 0),
	}, nil
}

func (a POP3StoreAdapter) listInboxMessages(ctx context.Context, userID, inboxID string) ([]maildb.MessageSummary, error) {
	const pageSize = maildb.MessageListMaxLimit
	var all []maildb.MessageSummary
	var cursor maildb.MessageListCursor

	for {
		summaries, err := a.service.ListMessagesPage(ctx, userID, inboxID, pageSize, cursor, maildb.MessageListFilter{})
		if err != nil {
			return nil, err
		}
		page, err := maildb.NewMessageListPage(summaries, pageSize)
		if err != nil {
			return nil, err
		}
		all = append(all, page.Messages...)
		if !page.HasMore {
			return all, nil
		}
		cursor, err = maildb.DecodeMessageListCursor(page.NextCursor)
		if err != nil {
			return nil, fmt.Errorf("decode inbox cursor: %w", err)
		}
	}
}

func normalizePOP3Username(user string) (string, error) {
	user = strings.TrimSpace(user)
	if user == "" || strings.ContainsAny(user, "\r\n") {
		return "", fmt.Errorf("invalid username")
	}
	return user, nil
}

func validatePOP3Password(pass string) error {
	if strings.ContainsAny(pass, "\r\n") {
		return fmt.Errorf("invalid password")
	}
	return nil
}

func normalizePOP3AuthenticatedUserID(userID string) (string, error) {
	if strings.ContainsAny(userID, "\r\n") {
		return "", fmt.Errorf("invalid authenticated user id")
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return "", fmt.Errorf("authenticated user id is required")
	}
	return userID, nil
}

type pop3InboxMsg struct {
	id   string
	size int
}

type pop3Mailbox struct {
	ctx     context.Context
	service *Service
	userID  string
	msgs    []pop3InboxMsg
	deleted []bool
	content []string
	loaded  []bool
	pending []string
}

func (m *pop3Mailbox) MessageCount() int { return len(m.msgs) }

func (m *pop3Mailbox) MaildropLockKey() string { return m.userID }

func (m *pop3Mailbox) MessageSize(i int) int {
	if i < 0 || i >= len(m.msgs) || m.deleted[i] {
		return 0
	}
	return m.msgs[i].size
}

func (m *pop3Mailbox) MessageUIDL(i int) string {
	if i < 0 || i >= len(m.msgs) {
		return ""
	}
	return m.msgs[i].id
}

func (m *pop3Mailbox) MessageContent(i int) string {
	content, err := m.MessageContentWithError(i)
	if err != nil {
		return ""
	}
	return content
}

func (m *pop3Mailbox) MessageContentWithError(i int) (string, error) {
	if i < 0 || i >= len(m.msgs) || m.deleted[i] {
		return "", fmt.Errorf("invalid message index")
	}
	if !m.loaded[i] {
		body, err := m.service.FetchRawMessageBody(m.ctx, m.userID, m.msgs[i].id)
		if err != nil {
			return "", fmt.Errorf("fetch raw message body: %w", err)
		}
		m.content[i] = body
		m.loaded[i] = true
	}
	return m.content[i], nil
}

func (m *pop3Mailbox) MarkDeleted(i int) error {
	if i < 0 || i >= len(m.msgs) {
		return fmt.Errorf("invalid message index")
	}
	if !m.deleted[i] {
		m.deleted[i] = true
		m.pending = append(m.pending, m.msgs[i].id)
	}
	return nil
}

func (m *pop3Mailbox) ResetDeleted() {
	for i := range m.deleted {
		m.deleted[i] = false
	}
	m.pending = m.pending[:0]
}

func (m *pop3Mailbox) Deleted(i int) bool {
	if i < 0 || i >= len(m.msgs) {
		return false
	}
	return m.deleted[i]
}

// CommitDeletes is called by the POP3 server on QUIT to soft-delete pending messages.
func (m *pop3Mailbox) CommitDeletes() error {
	if len(m.pending) == 0 {
		return nil
	}
	ids := uniquePOP3PendingIDs(m.pending)
	if len(ids) == 0 {
		m.pending = m.pending[:0]
		return nil
	}
	_, err := m.service.BulkDeleteMessages(m.ctx, maildb.BulkMessageDeleteRequest{
		UserID:     m.userID,
		MessageIDs: ids,
	})
	if err != nil {
		return err
	}
	m.pending = m.pending[:0]
	return nil
}

func uniquePOP3PendingIDs(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	unique := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	return unique
}
