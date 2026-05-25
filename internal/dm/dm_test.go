package dm

import (
	"bytes"
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestCryptoWrapsRoomKeyAndEncryptsBody(t *testing.T) {
	master := bytes.Repeat([]byte{0x42}, MasterKeyBytes)
	crypto, err := NewCrypto(master)
	if err != nil {
		t.Fatalf("NewCrypto: %v", err)
	}
	roomKey, err := crypto.GenerateRoomKey()
	if err != nil {
		t.Fatalf("GenerateRoomKey: %v", err)
	}
	wrapped, err := crypto.WrapRoomKey(roomKey)
	if err != nil {
		t.Fatalf("WrapRoomKey: %v", err)
	}
	if bytes.Contains(wrapped, roomKey) {
		t.Fatalf("wrapped key contains plaintext room key")
	}
	unwrapped, err := crypto.UnwrapRoomKey(wrapped)
	if err != nil {
		t.Fatalf("UnwrapRoomKey: %v", err)
	}
	if !bytes.Equal(unwrapped, roomKey) {
		t.Fatalf("unwrapped key mismatch")
	}
	ciphertext, err := crypto.EncryptBody(roomKey, []byte("secret dm body"))
	if err != nil {
		t.Fatalf("EncryptBody: %v", err)
	}
	if bytes.Contains(ciphertext, []byte("secret")) {
		t.Fatalf("ciphertext contains plaintext body")
	}
	plaintext, err := crypto.DecryptBody(roomKey, ciphertext)
	if err != nil {
		t.Fatalf("DecryptBody: %v", err)
	}
	if string(plaintext) != "secret dm body" {
		t.Fatalf("plaintext = %q", plaintext)
	}
}

func TestExtractMessageURLsNormalizesDeduplicatesAndCaps(t *testing.T) {
	body := "see https://example.com/a, and https://example.com/a plus http://example.net/x)."
	got := ExtractMessageURLs(body)
	want := []string{"https://example.com/a", "http://example.net/x"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("urls = %#v, want %#v", got, want)
	}
}

func TestServiceSendMessageEncryptsStoredBodyAndExtractsURLs(t *testing.T) {
	crypto, wrappedKey := testCryptoAndWrappedRoomKey(t)
	store := &fakeStore{wrappedRoomKey: wrappedKey}
	service := NewService(store, crypto)
	msg, err := service.SendMessage(context.Background(), testPrincipal(), "room-1", SendMessageRequest{
		Body: "hello https://example.com/r",
	})
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if msg.Body != "hello https://example.com/r" {
		t.Fatalf("returned body = %q", msg.Body)
	}
	if bytes.Contains(store.inserted.BodyCiphertext, []byte("hello")) {
		t.Fatalf("stored body was not encrypted")
	}
	if got, want := store.insertedURLs, []string{"https://example.com/r"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("urls = %#v, want %#v", got, want)
	}
}

func TestServiceListMessagesDecryptsParticipantMessages(t *testing.T) {
	crypto, wrappedKey := testCryptoAndWrappedRoomKey(t)
	roomKey, err := crypto.UnwrapRoomKey(wrappedKey)
	if err != nil {
		t.Fatalf("unwrap: %v", err)
	}
	ciphertext, err := crypto.EncryptBody(roomKey, []byte("plain body"))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	now := time.Now().UTC()
	store := &fakeStore{
		wrappedRoomKey: wrappedKey,
		listMessages: []MessageRecord{{
			Message:        Message{ID: "msg-1", RoomID: "room-1", SenderID: "user-2", MessageType: MessageTypeText, CreatedAt: now},
			BodyCiphertext: ciphertext,
		}},
	}
	service := NewService(store, crypto)
	messages, err := service.ListMessages(context.Background(), testPrincipal(), "room-1", MessageCursor{Limit: 50})
	if err != nil {
		t.Fatalf("ListMessages: %v", err)
	}
	if len(messages) != 1 || messages[0].Body != "plain body" {
		t.Fatalf("messages = %#v", messages)
	}
}

func TestServiceSendAttachmentStoresObjectAndEncryptsPath(t *testing.T) {
	crypto, wrappedKey := testCryptoAndWrappedRoomKey(t)
	attachments := &fakeAttachmentStore{}
	store := &fakeStore{wrappedRoomKey: wrappedKey}
	service := NewService(store, crypto).WithAttachmentStore(attachments)
	msg, err := service.SendAttachment(context.Background(), testPrincipal(), "room-1", AttachmentUpload{
		Filename:    "../report.pdf",
		ContentType: "application/pdf",
		Size:        7,
		Body:        []byte("content"),
	})
	if err != nil {
		t.Fatalf("SendAttachment: %v", err)
	}
	if msg.MessageType != MessageTypeFile || msg.AttachmentName != "_report.pdf" {
		t.Fatalf("message = %#v", msg)
	}
	if attachments.path == "" || !strings.Contains(attachments.path, "/room-1/") {
		t.Fatalf("stored attachment path = %q", attachments.path)
	}
	if bytes.Contains(store.inserted.AttachmentStoragePathCiphertext, []byte(attachments.path)) {
		t.Fatalf("attachment storage path was not encrypted")
	}
}

func TestServiceOpenAttachmentUsesSignedExpiringToken(t *testing.T) {
	crypto, wrappedKey := testCryptoAndWrappedRoomKey(t)
	attachments := &fakeAttachmentStore{}
	store := &fakeStore{wrappedRoomKey: wrappedKey}
	service := NewService(store, crypto).WithAttachmentStore(attachments)
	service.now = func() time.Time { return time.Unix(100, 0).UTC() }
	msg, err := service.SendAttachment(context.Background(), testPrincipal(), "room-1", AttachmentUpload{
		Filename:    "report.txt",
		ContentType: "text/plain",
		Size:        7,
		Body:        []byte("content"),
	})
	if err != nil {
		t.Fatalf("SendAttachment: %v", err)
	}
	token, err := service.SignAttachmentDownload(msg.ID, service.now().Add(time.Hour))
	if err != nil {
		t.Fatalf("SignAttachmentDownload: %v", err)
	}
	download, err := service.OpenAttachment(context.Background(), token)
	if err != nil {
		t.Fatalf("OpenAttachment: %v", err)
	}
	defer download.Body.Close()
	body, err := io.ReadAll(download.Body)
	if err != nil {
		t.Fatalf("read download: %v", err)
	}
	if string(body) != "content" || download.Filename != "report.txt" || download.ContentType != "text/plain" {
		t.Fatalf("download = (%q, %q, %q)", body, download.Filename, download.ContentType)
	}
	service.now = func() time.Time { return time.Unix(100, 0).UTC().Add(2 * time.Hour) }
	if _, err := service.OpenAttachment(context.Background(), token); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expired OpenAttachment err = %v, want ErrForbidden", err)
	}
}

func TestServiceAddMembersEncryptsSystemMessages(t *testing.T) {
	crypto, wrappedKey := testCryptoAndWrappedRoomKey(t)
	store := &fakeStore{
		wrappedRoomKey: wrappedKey,
		users: []User{{
			ID:          "user-2",
			CompanyID:   "company-1",
			DomainID:    "domain-1",
			DisplayName: "김철수",
		}},
	}
	service := NewService(store, crypto)
	messages, err := service.AddMembers(context.Background(), testPrincipal(), "room-1", []string{"user-2"})
	if err != nil {
		t.Fatalf("AddMembers: %v", err)
	}
	if len(messages) != 1 || messages[0].MessageType != MessageTypeSystem || messages[0].Body != "김철수님이 초대되었습니다." {
		t.Fatalf("messages = %#v", messages)
	}
	if len(store.addedSystem) != 1 {
		t.Fatalf("added system messages = %#v", store.addedSystem)
	}
	if bytes.Contains(store.addedSystem[0].BodyCiphertext, []byte("김철수")) {
		t.Fatalf("system message body was not encrypted")
	}
}

func TestServiceTransferOwnerEncryptsSystemMessage(t *testing.T) {
	crypto, wrappedKey := testCryptoAndWrappedRoomKey(t)
	store := &fakeStore{
		wrappedRoomKey: wrappedKey,
		users:          []User{{ID: "user-2", CompanyID: "company-1", DomainID: "domain-1", DisplayName: "이영희"}},
	}
	service := NewService(store, crypto)
	message, err := service.TransferOwner(context.Background(), testPrincipal(), "room-1", "user-2")
	if err != nil {
		t.Fatalf("TransferOwner: %v", err)
	}
	if message.Body != "방장이 이영희님에게 권한을 위임했습니다." {
		t.Fatalf("message body = %q", message.Body)
	}
	if bytes.Contains(store.transferSystem.BodyCiphertext, []byte("이영희")) {
		t.Fatalf("transfer system message body was not encrypted")
	}
}

func TestServiceJoinInviteUsesInviteRoomKeyAndEncryptsSystemMessage(t *testing.T) {
	crypto, wrappedKey := testCryptoAndWrappedRoomKey(t)
	store := &fakeStore{
		wrappedRoomKey: wrappedKey,
		users:          []User{{ID: "user-1", CompanyID: "company-1", DomainID: "domain-1", DisplayName: "박민준"}},
	}
	service := NewService(store, crypto)
	message, err := service.JoinInvite(context.Background(), testPrincipal(), "invite-1")
	if err != nil {
		t.Fatalf("JoinInvite: %v", err)
	}
	if message.Body != "박민준님이 참여했습니다." {
		t.Fatalf("message body = %q", message.Body)
	}
	if bytes.Contains(store.joinSystem.BodyCiphertext, []byte("박민준")) {
		t.Fatalf("join system message body was not encrypted")
	}
}

func TestServiceCreateDirectRoomRejectsSelf(t *testing.T) {
	crypto, _ := testCryptoAndWrappedRoomKey(t)
	service := NewService(&fakeStore{}, crypto)
	_, err := service.CreateRoom(context.Background(), testPrincipal(), CreateRoomRequest{
		RoomType: RoomTypeDirect,
		UserIDs:  []string{"user-1"},
	})
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("CreateRoom err = %v, want ErrInvalid", err)
	}
}

func TestValidateUUIDsRejectsDirectoryAliases(t *testing.T) {
	err := validateUUIDs([]string{"kang.hyunjae@parkjw.org"})
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("validateUUIDs err = %v, want ErrInvalid", err)
	}
}

func TestFindDirectRoomQueryUsesContiguousParameters(t *testing.T) {
	if strings.Contains(findDirectRoomQuery, "$5") {
		t.Fatalf("findDirectRoomQuery has an unused parameter gap: %s", findDirectRoomQuery)
	}
	for _, parameter := range []string{"$1", "$2", "$3", "$4"} {
		if !strings.Contains(findDirectRoomQuery, parameter) {
			t.Fatalf("findDirectRoomQuery missing %s", parameter)
		}
	}
}

func TestServiceToggleReactionRequiresSingleEmoji(t *testing.T) {
	crypto, _ := testCryptoAndWrappedRoomKey(t)
	service := NewService(&fakeStore{}, crypto)
	err := service.ToggleReaction(context.Background(), testPrincipal(), "msg-1", "ok")
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("ToggleReaction err = %v, want ErrInvalid", err)
	}
	if err := service.ToggleReaction(context.Background(), testPrincipal(), "msg-1", "👍"); err != nil {
		t.Fatalf("ToggleReaction single emoji: %v", err)
	}
}

func TestHardDeleteRoomDestroysKeyBeforeRows(t *testing.T) {
	statements := hardDeleteRoomStatements()
	if len(statements) != 5 {
		t.Fatalf("hard delete statements = %d, want 5", len(statements))
	}
	want := []string{
		"DELETE FROM dm_room_keys",
		"DELETE FROM dm_messages",
		"DELETE FROM dm_participants",
		"DELETE FROM dm_invites",
		"DELETE FROM dm_rooms",
	}
	for i, prefix := range want {
		if !strings.HasPrefix(statements[i], prefix) {
			t.Fatalf("statement[%d] = %q, want prefix %q", i, statements[i], prefix)
		}
	}
}

func testCryptoAndWrappedRoomKey(t *testing.T) (*Crypto, []byte) {
	t.Helper()
	crypto, err := NewCrypto(bytes.Repeat([]byte{0x11}, MasterKeyBytes))
	if err != nil {
		t.Fatalf("NewCrypto: %v", err)
	}
	roomKey := bytes.Repeat([]byte{0x22}, RoomKeyBytes)
	wrapped, err := crypto.WrapRoomKey(roomKey)
	if err != nil {
		t.Fatalf("WrapRoomKey: %v", err)
	}
	return crypto, wrapped
}

func testPrincipal() Principal {
	return Principal{UserID: "user-1", CompanyID: "company-1", DomainID: "domain-1"}
}

type fakeStore struct {
	wrappedRoomKey []byte
	inserted       MessageRecord
	insertedURLs   []string
	listMessages   []MessageRecord
	users          []User
	addedSystem    []MessageRecord
	transferSystem MessageRecord
	joinSystem     MessageRecord
}

type fakeAttachmentStore struct {
	path string
	body []byte
}

func (f *fakeAttachmentStore) Put(_ context.Context, path string, body io.Reader) error {
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	f.path = path
	f.body = data
	return nil
}

func (f *fakeAttachmentStore) Get(_ context.Context, path string) (io.ReadCloser, error) {
	if path != f.path {
		return nil, ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(f.body)), nil
}

func (f *fakeStore) CreateDirectRoom(context.Context, Principal, string, []byte) (Room, error) {
	return Room{ID: "room-1", RoomType: RoomTypeDirect}, nil
}

func (f *fakeStore) CreateGroupRoom(context.Context, Principal, CreateRoomRequest, []byte) (Room, error) {
	return Room{ID: "room-1", RoomType: RoomTypeGroup}, nil
}

func (f *fakeStore) ListRooms(context.Context, Principal) ([]Room, error) { return nil, nil }

func (f *fakeStore) ListPublicRooms(context.Context, Principal) ([]Room, error) { return nil, nil }

func (f *fakeStore) Users(_ context.Context, _ Principal, userIDs []string) ([]User, error) {
	if len(f.users) > 0 {
		return f.users, nil
	}
	users := make([]User, 0, len(userIDs))
	for _, id := range userIDs {
		users = append(users, User{ID: id, CompanyID: "company-1", DomainID: "domain-1", DisplayName: id})
	}
	return users, nil
}

func (f *fakeStore) AddMembers(_ context.Context, _ Principal, _ string, _ []string, systemMessages []MessageRecord) ([]MessageRecord, error) {
	f.addedSystem = append([]MessageRecord(nil), systemMessages...)
	for i := range systemMessages {
		systemMessages[i].ID = "sys"
		systemMessages[i].CreatedAt = time.Now().UTC()
	}
	return systemMessages, nil
}

func (f *fakeStore) RemoveMember(_ context.Context, _ Principal, _ string, _ string, systemMessage MessageRecord) (RoomRemoval, error) {
	systemMessage.ID = "sys-remove"
	systemMessage.CreatedAt = time.Now().UTC()
	return RoomRemoval{SystemMessage: systemMessage.Message, systemMessageRecord: systemMessage}, nil
}

func (f *fakeStore) TransferOwner(_ context.Context, _ Principal, _ string, _ string, systemMessage MessageRecord) (MessageRecord, error) {
	f.transferSystem = systemMessage
	systemMessage.ID = "sys-transfer"
	systemMessage.CreatedAt = time.Now().UTC()
	return systemMessage, nil
}

func (f *fakeStore) CreateInvite(context.Context, Principal, string, time.Time) (Invite, error) {
	return Invite{}, nil
}

func (f *fakeStore) RoomKeyForInvite(context.Context, Principal, string) (string, []byte, error) {
	return "room-1", append([]byte(nil), f.wrappedRoomKey...), nil
}

func (f *fakeStore) JoinInvite(_ context.Context, _ Principal, _ string, systemMessage MessageRecord) (MessageRecord, error) {
	f.joinSystem = systemMessage
	systemMessage.ID = "sys-join"
	systemMessage.CreatedAt = time.Now().UTC()
	return systemMessage, nil
}

func (f *fakeStore) RoomKeyForParticipant(context.Context, Principal, string) ([]byte, error) {
	return append([]byte(nil), f.wrappedRoomKey...), nil
}

func (f *fakeStore) RoomKeyForMessageOwner(context.Context, Principal, string) (string, []byte, error) {
	return "room-1", append([]byte(nil), f.wrappedRoomKey...), nil
}

func (f *fakeStore) AttachmentByMessageID(context.Context, string) (MessageRecord, []byte, error) {
	return f.inserted, append([]byte(nil), f.wrappedRoomKey...), nil
}

func (f *fakeStore) InsertMessage(_ context.Context, _ Principal, msg MessageRecord, urls []string) (MessageRecord, error) {
	f.inserted = msg
	f.insertedURLs = append([]string(nil), urls...)
	msg.ID = "msg-1"
	msg.CreatedAt = time.Now().UTC()
	return msg, nil
}

func (f *fakeStore) ListMessages(context.Context, Principal, string, MessageCursor) ([]MessageRecord, error) {
	return f.listMessages, nil
}

func (f *fakeStore) UpdateTextMessage(context.Context, Principal, string, []byte, []string) (MessageRecord, error) {
	return MessageRecord{}, nil
}

func (f *fakeStore) SoftDeleteMessage(context.Context, Principal, string) (MessageRecord, error) {
	return MessageRecord{}, nil
}

func (f *fakeStore) ToggleReaction(context.Context, Principal, string, string) error { return nil }

func (f *fakeStore) MarkRead(context.Context, Principal, string, string) error { return nil }

func (f *fakeStore) ListSearchCandidates(context.Context, Principal, string, string, int) ([]MessageRecord, error) {
	return nil, nil
}

func (f *fakeStore) ListMedia(context.Context, Principal, string, MediaQuery) ([]MediaItem, error) {
	return nil, nil
}

var _ Store = (*fakeStore)(nil)
