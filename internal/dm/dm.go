package dm

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"path"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	RoomTypeDirect = "direct"
	RoomTypeGroup  = "group"

	VisibilityPublic  = "public"
	VisibilityPrivate = "private"

	MessageTypeText      = "text"
	MessageTypeFile      = "file"
	MessageTypeDriveLink = "drive_link"
	MessageTypeSystem    = "system"

	MaxTextBodyBytes   = 32 * 1024
	MaxAttachmentBytes = 20 << 20
	InviteTTL          = 7 * 24 * time.Hour
)

var (
	ErrInvalid   = errors.New("dm invalid request")
	ErrNotFound  = errors.New("dm not found")
	ErrConflict  = errors.New("dm conflict")
	ErrForbidden = errors.New("dm forbidden")
)

// SystemMessages holds the text templates for system-generated DM messages.
// All placeholders use %s for a user's display name.
// Override at startup with Service.WithSystemMessages to support other locales.
type SystemMessages struct {
	MessageDeleted   string // shown for soft-deleted messages (no placeholder)
	MemberInvited    string // %s = invitee display name
	MemberLeft       string // %s = leaving member display name
	OwnerTransferred string // %s = new owner display name
	MemberJoined     string // %s = joining member display name
}

// DefaultSystemMessages returns the built-in Korean system message templates.
func DefaultSystemMessages() SystemMessages {
	return SystemMessages{
		MessageDeleted:   "삭제된 메시지입니다.",
		MemberInvited:    "%s님이 초대되었습니다.",
		MemberLeft:       "%s님이 나갔습니다.",
		OwnerTransferred: "방장이 %s님에게 권한을 위임했습니다.",
		MemberJoined:     "%s님이 참여했습니다.",
	}
}

type Principal struct {
	UserID    string
	CompanyID string
	DomainID  string
}

type User struct {
	ID          string `json:"id"`
	CompanyID   string `json:"company_id,omitempty"`
	DomainID    string `json:"domain_id,omitempty"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email,omitempty"`
	AvatarURL   string `json:"avatar_url,omitempty"`
}

type Room struct {
	ID            string    `json:"id"`
	CompanyID     string    `json:"company_id"`
	DomainID      string    `json:"domain_id"`
	RoomType      string    `json:"room_type"`
	Visibility    string    `json:"visibility,omitempty"`
	Name          string    `json:"name,omitempty"`
	OwnerID       string    `json:"owner_id,omitempty"`
	CreatedBy     string    `json:"created_by"`
	CreatedAt     time.Time `json:"created_at"`
	Members       []User    `json:"members,omitempty"`
	UnreadCount   int       `json:"unread_count,omitempty"`
	LastMessage   *Message  `json:"last_message,omitempty"`
	MemberCount   int       `json:"member_count,omitempty"`
	LastReadID    string    `json:"last_read_message_id,omitempty"`
	CurrentUserID string    `json:"current_user_id,omitempty"`
}

type Message struct {
	ID                    string     `json:"id"`
	RoomID                string     `json:"room_id"`
	SenderID              string     `json:"sender_id,omitempty"`
	MessageType           string     `json:"message_type"`
	Body                  string     `json:"body"`
	AttachmentName        string     `json:"attachment_name,omitempty"`
	AttachmentSize        int64      `json:"attachment_size,omitempty"`
	AttachmentMIMEType    string     `json:"attachment_mime_type,omitempty"`
	AttachmentDownloadURL string     `json:"attachment_download_url,omitempty"`
	DriveFileID           string     `json:"drive_file_id,omitempty"`
	CreatedAt             time.Time  `json:"created_at"`
	EditedAt              *time.Time `json:"edited_at,omitempty"`
	DeletedAt             *time.Time `json:"deleted_at,omitempty"`
	Reactions             []Reaction `json:"reactions,omitempty"`
	ReadCount             int        `json:"read_count,omitempty"`
}

type Reaction struct {
	Emoji string `json:"emoji"`
	Count int    `json:"count"`
	Mine  bool   `json:"mine,omitempty"`
}

type Invite struct {
	Token     string    `json:"token"`
	RoomID    string    `json:"room_id"`
	ExpiresAt time.Time `json:"expires_at"`
}

type MediaItem struct {
	MessageID          string    `json:"message_id"`
	MessageType        string    `json:"message_type"`
	SenderID           string    `json:"sender_id,omitempty"`
	URL                string    `json:"url,omitempty"`
	AttachmentName     string    `json:"attachment_name,omitempty"`
	AttachmentSize     int64     `json:"attachment_size,omitempty"`
	AttachmentMIMEType string    `json:"attachment_mime_type,omitempty"`
	DownloadURL        string    `json:"download_url,omitempty"`
	DriveFileID        string    `json:"drive_file_id,omitempty"`
	DriveName          string    `json:"drive_name,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
}

type SearchResult struct {
	Message Message `json:"message"`
	Before  string  `json:"before,omitempty"`
	After   string  `json:"after,omitempty"`
}

type CreateRoomRequest struct {
	RoomType   string   `json:"room_type"`
	UserIDs    []string `json:"user_ids"`
	Name       string   `json:"name"`
	Visibility string   `json:"visibility"`
}

type SendMessageRequest struct {
	Body        string `json:"body"`
	DriveFileID string `json:"drive_file_id"`
}

type AttachmentUpload struct {
	Filename    string
	Size        int64
	ContentType string
	Body        []byte
}

type AttachmentDownload struct {
	Filename    string
	Size        int64
	ContentType string
	Body        io.ReadCloser
}

type Store interface {
	CreateDirectRoom(ctx context.Context, principal Principal, otherUserID string, keyCiphertext []byte) (Room, error)
	CreateGroupRoom(ctx context.Context, principal Principal, req CreateRoomRequest, keyCiphertext []byte) (Room, error)
	ListRooms(ctx context.Context, principal Principal) ([]Room, error)
	ListPublicRooms(ctx context.Context, principal Principal) ([]Room, error)
	Users(ctx context.Context, principal Principal, userIDs []string) ([]User, error)
	AddMembers(ctx context.Context, principal Principal, roomID string, userIDs []string, systemMessages []MessageRecord) ([]MessageRecord, error)
	RemoveMember(ctx context.Context, principal Principal, roomID string, targetUserID string, systemMessage MessageRecord) (RoomRemoval, error)
	TransferOwner(ctx context.Context, principal Principal, roomID string, targetUserID string, systemMessage MessageRecord) (MessageRecord, error)
	CreateInvite(ctx context.Context, principal Principal, roomID string, expiresAt time.Time) (Invite, error)
	RoomKeyForInvite(ctx context.Context, principal Principal, token string) (string, []byte, error)
	JoinInvite(ctx context.Context, principal Principal, token string, systemMessage MessageRecord) (MessageRecord, error)
	RoomKeyForParticipant(ctx context.Context, principal Principal, roomID string) ([]byte, error)
	RoomKeyForMessageOwner(ctx context.Context, principal Principal, messageID string) (string, []byte, error)
	AttachmentByMessageID(ctx context.Context, messageID string) (MessageRecord, []byte, error)
	InsertMessage(ctx context.Context, principal Principal, msg MessageRecord, urls []string) (MessageRecord, error)
	ListMessages(ctx context.Context, principal Principal, roomID string, cursor MessageCursor) ([]MessageRecord, error)
	UpdateTextMessage(ctx context.Context, principal Principal, messageID string, bodyCiphertext []byte, urls []string) (MessageRecord, error)
	SoftDeleteMessage(ctx context.Context, principal Principal, messageID string) (MessageRecord, error)
	ToggleReaction(ctx context.Context, principal Principal, messageID string, emoji string) error
	MarkRead(ctx context.Context, principal Principal, roomID string, lastMessageID string) error
	ListSearchCandidates(ctx context.Context, principal Principal, roomID string, beforeMessageID string, limit int) ([]MessageRecord, error)
	ListMedia(ctx context.Context, principal Principal, roomID string, query MediaQuery) ([]MediaItem, error)
	GetRoom(ctx context.Context, principal Principal, roomID string) (Room, error)
	ListAllMessagesForExport(ctx context.Context, principal Principal, roomID string) ([]MessageRecord, error)
}

type AttachmentStore interface {
	Put(ctx context.Context, path string, body io.Reader) error
	Get(ctx context.Context, path string) (io.ReadCloser, error)
}

type MessageRecord struct {
	Message
	BodyCiphertext                  []byte
	AttachmentStoragePathCiphertext []byte
}

type RoomRemoval struct {
	SystemMessage       Message `json:"system_message,omitempty"`
	DeletedRoom         bool    `json:"deleted_room"`
	systemMessageRecord MessageRecord
}

type MessageCursor struct {
	BeforeID string
	AfterID  string
	Limit    int
}

type MediaQuery struct {
	Type     string
	BeforeID string
	Limit    int
}

type Service struct {
	store       Store
	crypto      *Crypto
	attachments AttachmentStore
	now         func() time.Time
	messages    SystemMessages
}

func NewService(store Store, crypto *Crypto) *Service {
	return &Service{store: store, crypto: crypto, now: time.Now, messages: DefaultSystemMessages()}
}

// WithSystemMessages replaces the default (Korean) system message templates.
// Call before the service handles any requests.
func (s *Service) WithSystemMessages(msgs SystemMessages) *Service {
	s.messages = msgs
	return s
}

func (s *Service) WithAttachmentStore(store AttachmentStore) *Service {
	s.attachments = store
	return s
}

func (s *Service) CreateRoom(ctx context.Context, principal Principal, req CreateRoomRequest) (Room, error) {
	principal = normalizePrincipal(principal)
	if err := validatePrincipal(principal); err != nil {
		return Room{}, err
	}
	req.RoomType = strings.ToLower(strings.TrimSpace(req.RoomType))
	req.Visibility = strings.ToLower(strings.TrimSpace(req.Visibility))
	req.Name = strings.TrimSpace(req.Name)
	req.UserIDs = cleanIDs(req.UserIDs)
	roomKey, wrapped, err := s.newWrappedRoomKey()
	if err != nil {
		return Room{}, err
	}
	defer zeroBytes(roomKey)
	switch req.RoomType {
	case RoomTypeDirect:
		if len(req.UserIDs) != 1 {
			return Room{}, fmt.Errorf("%w: direct rooms require exactly one other user", ErrInvalid)
		}
		if req.UserIDs[0] == principal.UserID {
			return Room{}, fmt.Errorf("%w: direct rooms require a different user", ErrInvalid)
		}
		return s.store.CreateDirectRoom(ctx, principal, req.UserIDs[0], wrapped)
	case RoomTypeGroup:
		if req.Visibility == "" {
			req.Visibility = VisibilityPrivate
		}
		if req.Visibility != VisibilityPrivate && req.Visibility != VisibilityPublic {
			return Room{}, fmt.Errorf("%w: visibility must be public or private", ErrInvalid)
		}
		if req.Name == "" {
			return Room{}, fmt.Errorf("%w: group name is required", ErrInvalid)
		}
		return s.store.CreateGroupRoom(ctx, principal, req, wrapped)
	default:
		return Room{}, fmt.Errorf("%w: room_type must be direct or group", ErrInvalid)
	}
}

func (s *Service) ListRooms(ctx context.Context, principal Principal) ([]Room, error) {
	if err := validatePrincipal(normalizePrincipal(principal)); err != nil {
		return nil, err
	}
	return s.store.ListRooms(ctx, normalizePrincipal(principal))
}

func (s *Service) ListPublicRooms(ctx context.Context, principal Principal) ([]Room, error) {
	if err := validatePrincipal(normalizePrincipal(principal)); err != nil {
		return nil, err
	}
	return s.store.ListPublicRooms(ctx, normalizePrincipal(principal))
}

func (s *Service) SendMessage(ctx context.Context, principal Principal, roomID string, req SendMessageRequest) (Message, error) {
	principal = normalizePrincipal(principal)
	if err := validatePrincipal(principal); err != nil {
		return Message{}, err
	}
	roomID = strings.TrimSpace(roomID)
	if roomID == "" {
		return Message{}, fmt.Errorf("%w: room_id is required", ErrInvalid)
	}
	body := strings.TrimSpace(req.Body)
	driveFileID := strings.TrimSpace(req.DriveFileID)
	if body == "" && driveFileID == "" {
		return Message{}, fmt.Errorf("%w: body or drive_file_id is required", ErrInvalid)
	}
	if len([]byte(body)) > MaxTextBodyBytes {
		return Message{}, fmt.Errorf("%w: message body is too large", ErrInvalid)
	}
	roomKey, err := s.roomKey(ctx, principal, roomID)
	if err != nil {
		return Message{}, err
	}
	defer zeroBytes(roomKey)
	ciphertext, err := s.crypto.EncryptBody(roomKey, []byte(body))
	if err != nil {
		return Message{}, err
	}
	msgType := MessageTypeText
	if driveFileID != "" {
		msgType = MessageTypeDriveLink
	}
	record := MessageRecord{
		Message: Message{
			RoomID:      roomID,
			SenderID:    principal.UserID,
			MessageType: msgType,
			Body:        body,
			DriveFileID: driveFileID,
		},
		BodyCiphertext: ciphertext,
	}
	stored, err := s.store.InsertMessage(ctx, principal, record, ExtractMessageURLs(body))
	if err != nil {
		return Message{}, err
	}
	stored.Body = body
	return stored.Message, nil
}

func (s *Service) SendAttachment(ctx context.Context, principal Principal, roomID string, upload AttachmentUpload) (Message, error) {
	principal = normalizePrincipal(principal)
	if err := validatePrincipal(principal); err != nil {
		return Message{}, err
	}
	if s.attachments == nil {
		return Message{}, fmt.Errorf("%w: attachment storage is not configured", ErrInvalid)
	}
	roomID = strings.TrimSpace(roomID)
	if roomID == "" {
		return Message{}, fmt.Errorf("%w: room_id is required", ErrInvalid)
	}
	upload.Filename = sanitizeAttachmentName(upload.Filename)
	if upload.Filename == "" {
		return Message{}, fmt.Errorf("%w: attachment filename is required", ErrInvalid)
	}
	if upload.Size <= 0 || upload.Size > MaxAttachmentBytes || int64(len(upload.Body)) != upload.Size {
		return Message{}, fmt.Errorf("%w: attachment size is invalid", ErrInvalid)
	}
	contentType, err := safeMIMEType(upload.ContentType)
	if err != nil {
		return Message{}, err
	}
	roomKey, err := s.roomKey(ctx, principal, roomID)
	if err != nil {
		return Message{}, err
	}
	defer zeroBytes(roomKey)
	storagePath := fmt.Sprintf("dm/%s/%s/%d-%s", path.Clean(principal.DomainID), path.Clean(roomID), s.now().UTC().UnixNano(), upload.Filename)
	if err := s.attachments.Put(ctx, storagePath, bytes.NewReader(upload.Body)); err != nil {
		return Message{}, fmt.Errorf("store dm attachment: %w", err)
	}
	bodyCiphertext, err := s.crypto.EncryptBody(roomKey, []byte(upload.Filename))
	if err != nil {
		return Message{}, err
	}
	pathCiphertext, err := s.crypto.EncryptBody(roomKey, []byte(storagePath))
	if err != nil {
		return Message{}, err
	}
	record := MessageRecord{
		Message: Message{
			RoomID:             roomID,
			SenderID:           principal.UserID,
			MessageType:        MessageTypeFile,
			AttachmentName:     upload.Filename,
			AttachmentSize:     upload.Size,
			AttachmentMIMEType: contentType,
		},
		BodyCiphertext:                  bodyCiphertext,
		AttachmentStoragePathCiphertext: pathCiphertext,
	}
	stored, err := s.store.InsertMessage(ctx, principal, record, nil)
	if err != nil {
		return Message{}, err
	}
	stored.Body = upload.Filename
	return stored.Message, nil
}

func (s *Service) ListMessages(ctx context.Context, principal Principal, roomID string, cursor MessageCursor) ([]Message, error) {
	principal = normalizePrincipal(principal)
	if err := validatePrincipal(principal); err != nil {
		return nil, err
	}
	if cursor.Limit <= 0 || cursor.Limit > 100 {
		cursor.Limit = 50
	}
	roomKey, err := s.roomKey(ctx, principal, roomID)
	if err != nil {
		return nil, err
	}
	defer zeroBytes(roomKey)
	records, err := s.store.ListMessages(ctx, principal, strings.TrimSpace(roomID), cursor)
	if err != nil {
		return nil, err
	}
	return s.decryptRecords(roomKey, records)
}

func (s *Service) EditMessage(ctx context.Context, principal Principal, messageID string, body string) (Message, error) {
	principal = normalizePrincipal(principal)
	if err := validatePrincipal(principal); err != nil {
		return Message{}, err
	}
	body = strings.TrimSpace(body)
	if body == "" {
		return Message{}, fmt.Errorf("%w: body is required", ErrInvalid)
	}
	if len([]byte(body)) > MaxTextBodyBytes {
		return Message{}, fmt.Errorf("%w: message body is too large", ErrInvalid)
	}
	roomID, key, err := s.roomKeyForMessageOwner(ctx, principal, strings.TrimSpace(messageID))
	if err != nil {
		return Message{}, err
	}
	defer zeroBytes(key)
	ciphertext, err := s.crypto.EncryptBody(key, []byte(body))
	if err != nil {
		return Message{}, err
	}
	updated, err := s.store.UpdateTextMessage(ctx, principal, strings.TrimSpace(messageID), ciphertext, ExtractMessageURLs(body))
	if err != nil {
		return Message{}, err
	}
	updated.RoomID = roomID
	updated.Body = body
	return updated.Message, nil
}

func (s *Service) DeleteMessage(ctx context.Context, principal Principal, messageID string) (Message, error) {
	principal = normalizePrincipal(principal)
	if err := validatePrincipal(principal); err != nil {
		return Message{}, err
	}
	deleted, err := s.store.SoftDeleteMessage(ctx, principal, strings.TrimSpace(messageID))
	if err != nil {
		return Message{}, err
	}
	deleted.Body = s.messages.MessageDeleted
	return deleted.Message, nil
}

func (s *Service) ToggleReaction(ctx context.Context, principal Principal, messageID string, emoji string) error {
	principal = normalizePrincipal(principal)
	if err := validatePrincipal(principal); err != nil {
		return err
	}
	emoji = strings.TrimSpace(emoji)
	if !validSingleEmoji(emoji) {
		return fmt.Errorf("%w: emoji must be a single emoji", ErrInvalid)
	}
	return s.store.ToggleReaction(ctx, principal, strings.TrimSpace(messageID), emoji)
}

func (s *Service) MarkRead(ctx context.Context, principal Principal, roomID string, lastMessageID string) error {
	principal = normalizePrincipal(principal)
	if err := validatePrincipal(principal); err != nil {
		return err
	}
	return s.store.MarkRead(ctx, principal, strings.TrimSpace(roomID), strings.TrimSpace(lastMessageID))
}

func (s *Service) Search(ctx context.Context, principal Principal, roomID string, q string, before string, limit int) ([]SearchResult, error) {
	// NOTE: DM 메시지는 AES-GCM 암호화로 저장된다. DB 레벨 FTS가 불가능하므로
	// 메시지를 복호화한 뒤 애플리케이션 레이어에서 strings.Contains로 검색한다.
	// 최대 1000개 메시지를 스캔하므로 메시지 수가 많은 방에서는 오래된 메시지가
	// 검색 범위에서 벗어날 수 있다.
	principal = normalizePrincipal(principal)
	if err := validatePrincipal(principal); err != nil {
		return nil, err
	}
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return nil, fmt.Errorf("%w: q is required", ErrInvalid)
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	roomKey, err := s.roomKey(ctx, principal, roomID)
	if err != nil {
		return nil, err
	}
	defer zeroBytes(roomKey)
	records, err := s.store.ListSearchCandidates(ctx, principal, strings.TrimSpace(roomID), strings.TrimSpace(before), 1000)
	if err != nil {
		return nil, err
	}
	messages, err := s.decryptRecords(roomKey, records)
	if err != nil {
		return nil, err
	}
	results := make([]SearchResult, 0, limit)
	for _, msg := range messages {
		if msg.MessageType == MessageTypeSystem || msg.DeletedAt != nil {
			continue
		}
		if strings.Contains(strings.ToLower(msg.Body), q) {
			results = append(results, SearchResult{Message: msg})
			if len(results) >= limit {
				break
			}
		}
	}
	return results, nil
}

func (s *Service) ListMedia(ctx context.Context, principal Principal, roomID string, query MediaQuery) ([]MediaItem, error) {
	principal = normalizePrincipal(principal)
	if err := validatePrincipal(principal); err != nil {
		return nil, err
	}
	// Normalize API-level type names to the store's internal tokens.
	// API uses: "file", "drive_link", "link"
	// Store uses: "file" (→ listFileMedia), "drive" (→ listDriveMedia), "links" (→ listLinkMedia)
	switch strings.ToLower(strings.TrimSpace(query.Type)) {
	case "drive_link", "drive":
		query.Type = "drive"
	case "link", "links":
		query.Type = "links"
	default:
		query.Type = "file"
	}
	if query.Limit <= 0 || query.Limit > 100 {
		query.Limit = 30
	}
	return s.store.ListMedia(ctx, principal, strings.TrimSpace(roomID), query)
}

func (s *Service) SignAttachmentDownload(messageID string, expiresAt time.Time) (string, error) {
	if s == nil || s.crypto == nil {
		return "", fmt.Errorf("dm crypto is not configured")
	}
	return s.crypto.SignAttachmentToken(messageID, expiresAt)
}

func (s *Service) VerifyAttachmentDownload(token string) (string, error) {
	if s == nil || s.crypto == nil {
		return "", fmt.Errorf("dm crypto is not configured")
	}
	return s.crypto.VerifyAttachmentToken(token, s.now())
}

func (s *Service) OpenAttachment(ctx context.Context, token string) (AttachmentDownload, error) {
	if s == nil || s.crypto == nil {
		return AttachmentDownload{}, fmt.Errorf("dm crypto is not configured")
	}
	if s.attachments == nil {
		return AttachmentDownload{}, fmt.Errorf("%w: attachment storage is not configured", ErrInvalid)
	}
	messageID, err := s.VerifyAttachmentDownload(token)
	if err != nil {
		return AttachmentDownload{}, err
	}
	record, wrapped, err := s.store.AttachmentByMessageID(ctx, messageID)
	if err != nil {
		return AttachmentDownload{}, err
	}
	roomKey, err := s.crypto.UnwrapRoomKey(wrapped)
	if err != nil {
		return AttachmentDownload{}, fmt.Errorf("dm room key unavailable: %w", err)
	}
	defer zeroBytes(roomKey)
	plainPath, err := s.crypto.DecryptBody(roomKey, record.AttachmentStoragePathCiphertext)
	if err != nil {
		return AttachmentDownload{}, err
	}
	body, err := s.attachments.Get(ctx, string(plainPath))
	if err != nil {
		return AttachmentDownload{}, err
	}
	return AttachmentDownload{
		Filename:    record.AttachmentName,
		Size:        record.AttachmentSize,
		ContentType: record.AttachmentMIMEType,
		Body:        body,
	}, nil
}

func (s *Service) AddMembers(ctx context.Context, principal Principal, roomID string, userIDs []string) ([]Message, error) {
	principal = normalizePrincipal(principal)
	if err := validatePrincipal(principal); err != nil {
		return nil, err
	}
	roomID = strings.TrimSpace(roomID)
	userIDs = cleanIDs(userIDs)
	if roomID == "" || len(userIDs) == 0 {
		return nil, fmt.Errorf("%w: room_id and user_ids are required", ErrInvalid)
	}
	key, err := s.roomKey(ctx, principal, roomID)
	if err != nil {
		return nil, err
	}
	defer zeroBytes(key)
	users, err := s.store.Users(ctx, principal, userIDs)
	if err != nil {
		return nil, err
	}
	if len(users) != len(userIDs) {
		return nil, fmt.Errorf("%w: users must belong to the same domain", ErrInvalid)
	}
	systemMessages, err := s.memberSystemMessages(key, roomID, users, s.messages.MemberInvited)
	if err != nil {
		return nil, err
	}
	records, err := s.store.AddMembers(ctx, principal, roomID, userIDs, systemMessages)
	if err != nil {
		return nil, err
	}
	return s.decryptRecords(key, records)
}

func (s *Service) RemoveMember(ctx context.Context, principal Principal, roomID string, userID string) (RoomRemoval, error) {
	principal = normalizePrincipal(principal)
	if err := validatePrincipal(principal); err != nil {
		return RoomRemoval{}, err
	}
	roomID = strings.TrimSpace(roomID)
	userID = strings.TrimSpace(userID)
	if roomID == "" || userID == "" {
		return RoomRemoval{}, fmt.Errorf("%w: room_id and user_id are required", ErrInvalid)
	}
	key, err := s.roomKey(ctx, principal, roomID)
	if err != nil {
		return RoomRemoval{}, err
	}
	defer zeroBytes(key)
	users, err := s.store.Users(ctx, principal, []string{userID})
	if err != nil {
		return RoomRemoval{}, err
	}
	if len(users) != 1 {
		return RoomRemoval{}, fmt.Errorf("%w: users must belong to the same domain", ErrInvalid)
	}
	systemMessage, err := s.systemMessage(key, roomID, fmt.Sprintf(s.messages.MemberLeft, displayName(users[0])))
	if err != nil {
		return RoomRemoval{}, err
	}
	removal, err := s.store.RemoveMember(ctx, principal, roomID, userID, systemMessage)
	if err != nil || removal.DeletedRoom {
		return removal, err
	}
	messages, err := s.decryptRecords(key, []MessageRecord{removal.systemMessageRecord})
	if err != nil {
		return RoomRemoval{}, err
	}
	removal.SystemMessage = messages[0]
	return removal, nil
}

func (s *Service) TransferOwner(ctx context.Context, principal Principal, roomID string, userID string) (Message, error) {
	principal = normalizePrincipal(principal)
	if err := validatePrincipal(principal); err != nil {
		return Message{}, err
	}
	roomID = strings.TrimSpace(roomID)
	userID = strings.TrimSpace(userID)
	if roomID == "" || userID == "" {
		return Message{}, fmt.Errorf("%w: room_id and user_id are required", ErrInvalid)
	}
	key, err := s.roomKey(ctx, principal, roomID)
	if err != nil {
		return Message{}, err
	}
	defer zeroBytes(key)
	users, err := s.store.Users(ctx, principal, []string{userID})
	if err != nil {
		return Message{}, err
	}
	if len(users) != 1 {
		return Message{}, fmt.Errorf("%w: users must belong to the same domain", ErrInvalid)
	}
	systemMessage, err := s.systemMessage(key, roomID, fmt.Sprintf(s.messages.OwnerTransferred, displayName(users[0])))
	if err != nil {
		return Message{}, err
	}
	record, err := s.store.TransferOwner(ctx, principal, roomID, userID, systemMessage)
	if err != nil {
		return Message{}, err
	}
	msgs, err := s.decryptRecords(key, []MessageRecord{record})
	if err != nil {
		return Message{}, err
	}
	return msgs[0], nil
}

func (s *Service) CreateInvite(ctx context.Context, principal Principal, roomID string) (Invite, error) {
	return s.store.CreateInvite(ctx, normalizePrincipal(principal), strings.TrimSpace(roomID), s.now().UTC().Add(InviteTTL))
}

func (s *Service) JoinInvite(ctx context.Context, principal Principal, token string) (Message, error) {
	principal = normalizePrincipal(principal)
	if err := validatePrincipal(principal); err != nil {
		return Message{}, err
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return Message{}, fmt.Errorf("%w: invite token is required", ErrInvalid)
	}
	roomID, wrapped, err := s.store.RoomKeyForInvite(ctx, principal, token)
	if err != nil {
		return Message{}, err
	}
	key, err := s.crypto.UnwrapRoomKey(wrapped)
	if err != nil {
		return Message{}, err
	}
	defer zeroBytes(key)
	users, err := s.store.Users(ctx, principal, []string{principal.UserID})
	if err != nil {
		return Message{}, err
	}
	if len(users) != 1 {
		return Message{}, fmt.Errorf("%w: users must belong to the same domain", ErrInvalid)
	}
	systemMessage, err := s.systemMessage(key, roomID, fmt.Sprintf(s.messages.MemberJoined, displayName(users[0])))
	if err != nil {
		return Message{}, err
	}
	record, err := s.store.JoinInvite(ctx, principal, token, systemMessage)
	if err != nil {
		return Message{}, err
	}
	msgs, err := s.decryptRecords(key, []MessageRecord{record})
	if err != nil {
		return Message{}, err
	}
	return msgs[0], nil
}

func (s *Service) newWrappedRoomKey() ([]byte, []byte, error) {
	if s == nil || s.crypto == nil {
		return nil, nil, fmt.Errorf("dm crypto is not configured")
	}
	roomKey, err := s.crypto.GenerateRoomKey()
	if err != nil {
		return nil, nil, err
	}
	wrapped, err := s.crypto.WrapRoomKey(roomKey)
	if err != nil {
		zeroBytes(roomKey)
		return nil, nil, err
	}
	return roomKey, wrapped, nil
}

func (s *Service) roomKey(ctx context.Context, principal Principal, roomID string) ([]byte, error) {
	if strings.TrimSpace(roomID) == "" {
		return nil, fmt.Errorf("%w: room_id is required", ErrInvalid)
	}
	wrapped, err := s.store.RoomKeyForParticipant(ctx, principal, strings.TrimSpace(roomID))
	if err != nil {
		return nil, err
	}
	key, err := s.crypto.UnwrapRoomKey(wrapped)
	if err != nil {
		return nil, fmt.Errorf("dm room key unavailable: %w", err)
	}
	return key, nil
}

func (s *Service) roomKeyForMessageOwner(ctx context.Context, principal Principal, messageID string) (string, []byte, error) {
	if messageID == "" {
		return "", nil, fmt.Errorf("%w: message_id is required", ErrInvalid)
	}
	roomID, wrapped, err := s.store.RoomKeyForMessageOwner(ctx, principal, messageID)
	if err != nil {
		return "", nil, err
	}
	key, err := s.crypto.UnwrapRoomKey(wrapped)
	if err != nil {
		return "", nil, fmt.Errorf("dm room key unavailable: %w", err)
	}
	return roomID, key, nil
}

func (s *Service) decryptRecords(roomKey []byte, records []MessageRecord) ([]Message, error) {
	out := make([]Message, 0, len(records))
	for _, record := range records {
		msg := record.Message
		if record.DeletedAt != nil {
			msg.Body = s.messages.MessageDeleted
			out = append(out, msg)
			continue
		}
		plain, err := s.crypto.DecryptBody(roomKey, record.BodyCiphertext)
		if err != nil {
			return nil, err
		}
		msg.Body = string(plain)
		out = append(out, msg)
	}
	return out, nil
}

func (s *Service) memberSystemMessages(roomKey []byte, roomID string, users []User, format string) ([]MessageRecord, error) {
	records := make([]MessageRecord, 0, len(users))
	for _, user := range users {
		record, err := s.systemMessage(roomKey, roomID, fmt.Sprintf(format, displayName(user)))
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, nil
}

func (s *Service) systemMessage(roomKey []byte, roomID string, body string) (MessageRecord, error) {
	ciphertext, err := s.crypto.EncryptBody(roomKey, []byte(body))
	if err != nil {
		return MessageRecord{}, err
	}
	return MessageRecord{
		Message: Message{
			RoomID:      roomID,
			MessageType: MessageTypeSystem,
			Body:        body,
		},
		BodyCiphertext: ciphertext,
	}, nil
}

func normalizePrincipal(p Principal) Principal {
	p.UserID = strings.TrimSpace(p.UserID)
	p.CompanyID = strings.TrimSpace(p.CompanyID)
	p.DomainID = strings.TrimSpace(p.DomainID)
	return p
}

func validatePrincipal(p Principal) error {
	if p.UserID == "" {
		return fmt.Errorf("%w: user_id is required", ErrInvalid)
	}
	if p.DomainID == "" {
		return fmt.Errorf("%w: domain_id is required", ErrInvalid)
	}
	if p.CompanyID == "" {
		return fmt.Errorf("%w: company_id is required", ErrInvalid)
	}
	return nil
}

func cleanIDs(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func zeroBytes(data []byte) {
	for i := range data {
		data[i] = 0
	}
}

func displayName(user User) string {
	name := strings.TrimSpace(user.DisplayName)
	if name != "" {
		return name
	}
	return strings.TrimSpace(user.ID)
}

func validSingleEmoji(value string) bool {
	if value == "" || !utf8.ValidString(value) {
		return false
	}
	count := 0
	for _, r := range value {
		if r == '\ufe0f' {
			continue
		}
		count++
		if count > 1 {
			return false
		}
		if r < 0x1F000 {
			return false
		}
	}
	return count == 1
}

func safeMIMEType(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "application/octet-stream", nil
	}
	mediaType, _, err := mime.ParseMediaType(value)
	if err != nil || !strings.Contains(mediaType, "/") || strings.ContainsAny(value, "\r\n") {
		return "", fmt.Errorf("%w: invalid attachment MIME type", ErrInvalid)
	}
	return value, nil
}

func sanitizeAttachmentName(value string) string {
	value = strings.TrimSpace(value)
	value = strings.NewReplacer("\\", "_", "/", "_", "\r", "_", "\n", "_", "\t", " ").Replace(value)
	value = strings.Trim(value, ". ")
	return truncateStringRunes(value, 180)
}

func truncateStringRunes(value string, max int) string {
	if max <= 0 {
		return ""
	}
	count := 0
	for i := range value {
		if count == max {
			return value[:i]
		}
		count++
	}
	return value
}
