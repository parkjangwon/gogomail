package mailservice

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/imapgw"
	"github.com/gogomail/gogomail/internal/mail"
	"github.com/gogomail/gogomail/internal/maildb"
	"github.com/gogomail/gogomail/internal/message"
	"github.com/gogomail/gogomail/internal/outbound"
	"github.com/gogomail/gogomail/internal/searchindex"
	smtpd "github.com/gogomail/gogomail/internal/smtp"
	"github.com/gogomail/gogomail/internal/storage"
)

func TestGetMessageParsesTextBodyFromStorage(t *testing.T) {
	t.Parallel()

	store := storage.NewLocalStore(t.TempDir())
	path := "mailstore/c/d/u/maildir/2026/05/msg.eml"
	raw := strings.Join([]string{
		"Message-ID: <body@example.com>",
		"From: sender@example.net",
		"To: admin@example.com",
		"Subject: body",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"hello body",
	}, "\r\n")
	if err := store.Put(context.Background(), path, strings.NewReader(raw)); err != nil {
		t.Fatalf("Put returned error: %v", err)
	}

	service := New(&fakeRepository{
		detail: maildb.MessageDetail{
			ID:          "msg-1",
			StoragePath: path,
			Flags:       []byte(`{"read":true}`),
		},
		attachments: []maildb.Attachment{
			{ID: "att-1", Filename: "report.pdf"},
		},
	}, store)

	msg, err := service.GetMessage(context.Background(), "user-1", "msg-1")
	if err != nil {
		t.Fatalf("GetMessage returned error: %v", err)
	}
	if msg.TextBody != "hello body" {
		t.Fatalf("TextBody = %q", msg.TextBody)
	}
	if len(msg.Attachments) != 1 || msg.Attachments[0].Filename != "report.pdf" {
		t.Fatalf("Attachments = %+v", msg.Attachments)
	}
}

func TestGetMessageCachesParsedBodyByStoragePath(t *testing.T) {
	t.Parallel()

	store := &recordingStore{body: strings.Join([]string{
		"Message-ID: <body@example.com>",
		"From: sender@example.net",
		"To: admin@example.com",
		"Subject: body",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"cached body",
	}, "\r\n")}
	service := New(&fakeRepository{
		detail: maildb.MessageDetail{
			ID:          "msg-1",
			StoragePath: "mailstore/c/d/u/maildir/2026/05/msg.eml",
			Flags:       []byte(`{"read":true}`),
		},
	}, store)

	first, err := service.GetMessage(context.Background(), "user-1", "msg-1")
	if err != nil {
		t.Fatalf("first GetMessage returned error: %v", err)
	}
	second, err := service.GetMessage(context.Background(), "user-1", "msg-1")
	if err != nil {
		t.Fatalf("second GetMessage returned error: %v", err)
	}
	if first.TextBody != "cached body" || second.TextBody != "cached body" {
		t.Fatalf("TextBody = %q/%q, want cached body", first.TextBody, second.TextBody)
	}
	if store.getCount != 1 {
		t.Fatalf("store.Get count = %d, want 1", store.getCount)
	}
}

func TestGetMessageMarksUnreadMessageRead(t *testing.T) {
	t.Parallel()

	events := &fakeIMAPEventPublisher{}
	repo := &fakeRepository{
		detail: maildb.MessageDetail{
			ID:    "msg-1",
			Flags: []byte(`{"read":false}`),
		},
		imapUIDs: []maildb.IMAPMessageUID{{MessageID: "msg-1", MailboxID: "inbox", UID: 12, ModSeq: 2}},
	}
	service := New(repo, nil).WithIMAPMailboxEvents(events)
	if _, err := service.GetMessage(context.Background(), "user-1", "msg-1"); err != nil {
		t.Fatalf("GetMessage returned error: %v", err)
	}
	if repo.lastFlagMessageID != "msg-1" || repo.lastFlag != "read" {
		t.Fatalf("read marker = %q/%q", repo.lastFlagMessageID, repo.lastFlag)
	}
	if repo.lastIMAPUIDLookupUserID != "user-1" || len(repo.lastIMAPUIDLookupMessageIDs) != 1 || repo.lastIMAPUIDLookupMessageIDs[0] != "msg-1" {
		t.Fatalf("imap uid lookup = %q/%#v", repo.lastIMAPUIDLookupUserID, repo.lastIMAPUIDLookupMessageIDs)
	}
	if len(events.events) != 1 || events.events[0].Type != imapgw.MailboxEventFlags || events.events[0].MailboxID != "inbox" || events.events[0].UID != 12 {
		t.Fatalf("events = %#v, want flags event", events.events)
	}
}

func TestGetMessageRejectsUnsafeMessageID(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{}
	service := New(repo, nil)

	if _, err := service.GetMessage(context.Background(), "user-1", "msg-1\r\nmsg-2"); err == nil {
		t.Fatal("GetMessage accepted newline-bearing message ID")
	}
	if repo.lastGetMessageID != "" {
		t.Fatalf("repository was called with message ID %q", repo.lastGetMessageID)
	}
}

func TestGetMessageRejectsUnsafeStoredBodyPath(t *testing.T) {
	t.Parallel()

	store := &recordingStore{}
	repo := &fakeRepository{
		detail: maildb.MessageDetail{
			ID:          "msg-1",
			StoragePath: "../secret.eml",
			Flags:       []byte(`{"read":true}`),
		},
	}
	service := New(repo, store)

	if _, err := service.GetMessage(context.Background(), "user-1", "msg-1"); err == nil {
		t.Fatal("GetMessage accepted unsafe stored body path")
	}
	if store.getPath != "" {
		t.Fatalf("store.Get was called with %q", store.getPath)
	}
}

func TestGetMessageDoesNotRewriteReadFlag(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{
		detail: maildb.MessageDetail{
			ID:    "msg-1",
			Flags: []byte(`{"read":true}`),
		},
	}
	service := New(repo, nil)
	if _, err := service.GetMessage(context.Background(), "user-1", "msg-1"); err != nil {
		t.Fatalf("GetMessage returned error: %v", err)
	}
	if repo.lastFlag != "" {
		t.Fatalf("unexpected flag write = %q", repo.lastFlag)
	}
}

func TestFetchIMAPMessageRejectsUnsafeStoredBodyPath(t *testing.T) {
	t.Parallel()

	store := &recordingStore{}
	repo := &fakeRepository{
		imapMessage: maildb.IMAPStoredMessage{
			Summary:     imapgw.MessageSummary{ID: "msg-1", MailboxID: "inbox", UID: 12},
			StoragePath: `messages\msg-1.eml`,
		},
	}
	service := New(repo, store)

	if _, err := service.FetchIMAPMessage(context.Background(), imapgw.FetchMessageRequest{UserID: "user-1", MailboxID: "inbox", UID: 12}); err == nil {
		t.Fatal("FetchIMAPMessage accepted unsafe stored body path")
	}
	if store.getPath != "" {
		t.Fatalf("store.Get was called with %q", store.getPath)
	}
}

func TestReadAndFolderMethodsNormalizeIDs(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{
		detail: maildb.MessageDetail{
			ID:    "msg-1",
			Flags: []byte(`{"read":true}`),
		},
	}
	service := New(repo, nil)

	if _, err := service.ListFolders(context.Background(), " user-1 "); err != nil {
		t.Fatalf("ListFolders returned error: %v", err)
	}
	if _, err := service.CreateFolder(context.Background(), maildb.CreateFolderRequest{UserID: " user-1 ", Name: " Archive "}); err != nil {
		t.Fatalf("CreateFolder returned error: %v", err)
	}
	if _, err := service.RenameFolder(context.Background(), " user-1 ", " folder-1 ", " Projects "); err != nil {
		t.Fatalf("RenameFolder returned error: %v", err)
	}
	if err := service.DeleteFolder(context.Background(), " user-1 ", " folder-1 "); err != nil {
		t.Fatalf("DeleteFolder returned error: %v", err)
	}
	if _, err := service.ListMessages(context.Background(), " user-1 ", 10); err != nil {
		t.Fatalf("ListMessages returned error: %v", err)
	}
	if _, err := service.ListMessagesInFolder(context.Background(), " user-1 ", " inbox ", 10); err != nil {
		t.Fatalf("ListMessagesInFolder returned error: %v", err)
	}
	read := false
	starred := true
	hasAttachment := true
	if _, err := service.ListMessagesPage(context.Background(), " user-1 ", " inbox ", 10, maildb.MessageListCursor{ID: "cursor"}, maildb.MessageListFilter{Read: &read, Starred: &starred, HasAttachment: &hasAttachment, Sort: " oldest "}); err != nil {
		t.Fatalf("ListMessagesPage returned error: %v", err)
	}
	if _, err := service.ListThreads(context.Background(), " user-1 ", 10); err != nil {
		t.Fatalf("ListThreads returned error: %v", err)
	}
	if _, err := service.ListThreadMessages(context.Background(), " user-1 ", " thread-1 ", 10); err != nil {
		t.Fatalf("ListThreadMessages returned error: %v", err)
	}
	if _, err := service.GetMessage(context.Background(), " user-1 ", " msg-1 "); err != nil {
		t.Fatalf("GetMessage returned error: %v", err)
	}

	if repo.lastListFoldersUserID != "user-1" {
		t.Fatalf("list folders user = %q", repo.lastListFoldersUserID)
	}
	if repo.lastCreateFolder.UserID != "user-1" || repo.lastCreateFolder.Name != "Archive" {
		t.Fatalf("create folder = %#v", repo.lastCreateFolder)
	}
	if repo.lastRenameFolderUserID != "user-1" || repo.lastRenameFolderID != "folder-1" || repo.lastRenameFolderName != "Projects" {
		t.Fatalf("rename folder = %q/%q/%q", repo.lastRenameFolderUserID, repo.lastRenameFolderID, repo.lastRenameFolderName)
	}
	if repo.lastDeleteFolderUserID != "user-1" || repo.lastDeleteFolderID != "folder-1" {
		t.Fatalf("delete folder = %q/%q", repo.lastDeleteFolderUserID, repo.lastDeleteFolderID)
	}
	if repo.lastListMessagesUserID != "user-1" {
		t.Fatalf("list messages user = %q", repo.lastListMessagesUserID)
	}
	if repo.lastListMessagesInFolderUserID != "user-1" || repo.lastListMessagesFolderID != "inbox" {
		t.Fatalf("list messages in folder = %q/%q", repo.lastListMessagesInFolderUserID, repo.lastListMessagesFolderID)
	}
	if repo.lastPageUserID != "user-1" || repo.lastPageFolderID != "inbox" {
		t.Fatalf("page ids = %q/%q", repo.lastPageUserID, repo.lastPageFolderID)
	}
	if repo.lastPageFilter.Sort != maildb.ListSortOldest {
		t.Fatalf("page sort = %q", repo.lastPageFilter.Sort)
	}
	if repo.lastPageFilter.Read == nil || *repo.lastPageFilter.Read || repo.lastPageFilter.Starred == nil || !*repo.lastPageFilter.Starred {
		t.Fatalf("page filter = %#v", repo.lastPageFilter)
	}
	if repo.lastPageFilter.HasAttachment == nil || !*repo.lastPageFilter.HasAttachment {
		t.Fatalf("page attachment filter = %#v", repo.lastPageFilter)
	}
	if repo.lastListThreadsUserID != "user-1" {
		t.Fatalf("list threads user = %q", repo.lastListThreadsUserID)
	}
	if repo.lastThreadMessagesUserID != "user-1" || repo.lastThreadID != "thread-1" {
		t.Fatalf("thread messages = %q/%q", repo.lastThreadMessagesUserID, repo.lastThreadID)
	}
	if repo.lastGetMessageUserID != "user-1" || repo.lastGetMessageID != "msg-1" {
		t.Fatalf("get message = %q/%q", repo.lastGetMessageUserID, repo.lastGetMessageID)
	}
}

func TestFolderServicesRejectUnsafeInputs(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		call   func(*Service) error
		called func(*fakeRepository) bool
	}{
		{
			name: "list unsafe user",
			call: func(service *Service) error {
				_, err := service.ListFolders(context.Background(), "user-1\nbad")
				return err
			},
			called: func(repo *fakeRepository) bool { return repo.lastListFoldersUserID != "" },
		},
		{
			name: "create unsafe user",
			call: func(service *Service) error {
				_, err := service.CreateFolder(context.Background(), maildb.CreateFolderRequest{
					UserID: "user-1\r\nbad",
					Name:   "Archive",
				})
				return err
			},
			called: func(repo *fakeRepository) bool { return repo.lastCreateFolder.UserID != "" },
		},
		{
			name: "create unsafe name",
			call: func(service *Service) error {
				_, err := service.CreateFolder(context.Background(), maildb.CreateFolderRequest{
					UserID: "user-1",
					Name:   "Archive\nbad",
				})
				return err
			},
			called: func(repo *fakeRepository) bool { return repo.lastCreateFolder.UserID != "" },
		},
		{
			name: "rename unsafe folder id",
			call: func(service *Service) error {
				_, err := service.RenameFolder(context.Background(), "user-1", "folder-1\r\nbad", "Projects")
				return err
			},
			called: func(repo *fakeRepository) bool { return repo.lastRenameFolderID != "" },
		},
		{
			name: "rename oversized name",
			call: func(service *Service) error {
				_, err := service.RenameFolder(context.Background(), "user-1", "folder-1", strings.Repeat("x", maxServiceResourceIDBytes+1))
				return err
			},
			called: func(repo *fakeRepository) bool { return repo.lastRenameFolderID != "" },
		},
		{
			name: "delete oversized folder id",
			call: func(service *Service) error {
				return service.DeleteFolder(context.Background(), "user-1", strings.Repeat("x", maxServiceResourceIDBytes+1))
			},
			called: func(repo *fakeRepository) bool { return repo.lastDeleteFolderID != "" },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			repo := &fakeRepository{}
			service := New(repo, nil)
			if err := tc.call(service); err == nil {
				t.Fatal("folder service accepted unsafe input")
			}
			if tc.called(repo) {
				t.Fatal("repository was called before folder input validation")
			}
		})
	}
}

func TestMailboxListMethodsRejectUnsafeResourceIDs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		run  func(*Service) error
	}{
		{
			name: "list folder",
			run: func(service *Service) error {
				_, err := service.ListMessagesInFolder(context.Background(), "user-1", "inbox\r\nbad", 10)
				return err
			},
		},
		{
			name: "page folder",
			run: func(service *Service) error {
				_, err := service.ListMessagesPage(context.Background(), "user-1", strings.Repeat("x", maxServiceResourceIDBytes+1), 10, maildb.MessageListCursor{}, maildb.MessageListFilter{})
				return err
			},
		},
		{
			name: "thread messages",
			run: func(service *Service) error {
				_, err := service.ListThreadMessages(context.Background(), "user-1", "thread-1\nbad", 10)
				return err
			},
		},
		{
			name: "thread page folder",
			run: func(service *Service) error {
				_, err := service.ListThreadsPage(context.Background(), "user-1", 10, maildb.ThreadListCursor{}, maildb.ThreadListFilter{FolderID: "folder\nbad"})
				return err
			},
		},
		{
			name: "message page sort",
			run: func(service *Service) error {
				_, err := service.ListMessagesPage(context.Background(), "user-1", "inbox", 10, maildb.MessageListCursor{}, maildb.MessageListFilter{Sort: "sideways"})
				return err
			},
		},
		{
			name: "thread page sort",
			run: func(service *Service) error {
				_, err := service.ListThreadsPage(context.Background(), "user-1", 10, maildb.ThreadListCursor{}, maildb.ThreadListFilter{Sort: "sideways"})
				return err
			},
		},
	}
	for _, tc := range tests {
		repo := &fakeRepository{}
		service := New(repo, nil)
		if err := tc.run(service); err == nil {
			t.Fatalf("%s accepted unsafe resource ID", tc.name)
		}
		if repo.lastListMessagesFolderID != "" || repo.lastPageFolderID != "" || repo.lastThreadID != "" {
			t.Fatalf("%s called repository with folder/thread IDs %q/%q/%q", tc.name, repo.lastListMessagesFolderID, repo.lastPageFolderID, repo.lastThreadID)
		}
	}
}

func TestListMethodsNormalizeLimits(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{}
	service := New(repo, nil)

	_, _ = service.ListMessages(context.Background(), "user-1", 0)
	if repo.lastListMessagesLimit != maildb.MessageListDefaultLimit {
		t.Fatalf("list messages limit = %d", repo.lastListMessagesLimit)
	}
	_, _ = service.ListMessagesInFolder(context.Background(), "user-1", "inbox", 500)
	if repo.lastListMessagesInFolderLimit != maildb.MessageListMaxLimit {
		t.Fatalf("list messages in folder limit = %d", repo.lastListMessagesInFolderLimit)
	}
	_, _ = service.ListMessagesPage(context.Background(), "user-1", "inbox", -1, maildb.MessageListCursor{}, maildb.MessageListFilter{})
	if repo.lastPageLimit != maildb.MessageListDefaultLimit {
		t.Fatalf("page limit = %d", repo.lastPageLimit)
	}
	_, _ = service.ListThreads(context.Background(), "user-1", 999)
	if repo.lastListThreadsLimit != maildb.MessageListMaxLimit {
		t.Fatalf("list threads limit = %d", repo.lastListThreadsLimit)
	}
	_, _ = service.ListThreadMessages(context.Background(), "user-1", "thread-1", 0)
	if repo.lastThreadMessagesLimit != maildb.MessageListDefaultLimit {
		t.Fatalf("thread messages limit = %d", repo.lastThreadMessagesLimit)
	}
	_, _ = service.ListPushDevices(context.Background(), "user-1", 500)
	if repo.lastListPushDeviceLimit != maildb.MessageListMaxLimit {
		t.Fatalf("push devices limit = %d", repo.lastListPushDeviceLimit)
	}
}

func TestUpsertPushDeviceNormalizesRequest(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{}
	service := New(repo, nil)

	device, err := service.UpsertPushDevice(context.Background(), maildb.UpsertPushDeviceRequest{
		UserID:   " user-1 ",
		Platform: " FCM ",
		Token:    " token-1 ",
		Label:    " phone ",
	})
	if err != nil {
		t.Fatalf("UpsertPushDevice returned error: %v", err)
	}
	if device.Platform != "fcm" || repo.lastPushDevice.Token != "token-1" || repo.lastPushDevice.Label != "phone" || repo.lastPushDevice.UserID != "user-1" {
		t.Fatalf("device = %+v lastPushDevice = %+v", device, repo.lastPushDevice)
	}
}

func TestPushDeviceReadAndDeleteNormalizeIDs(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{}
	service := New(repo, nil)

	if _, err := service.ListPushDevices(context.Background(), " user-1 ", 25); err != nil {
		t.Fatalf("ListPushDevices returned error: %v", err)
	}
	if err := service.DeletePushDevice(context.Background(), " user-1 ", " device-1 "); err != nil {
		t.Fatalf("DeletePushDevice returned error: %v", err)
	}

	if repo.lastListPushDeviceUserID != "user-1" {
		t.Fatalf("list push devices user = %q", repo.lastListPushDeviceUserID)
	}
	if repo.lastDeletePushDeviceUserID != "user-1" || repo.lastDeletePushDeviceID != "device-1" {
		t.Fatalf("delete push device = %q/%q", repo.lastDeletePushDeviceUserID, repo.lastDeletePushDeviceID)
	}
}

func TestDeletePushDeviceRejectsUnsafeDeviceID(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{}
	service := New(repo, nil)

	tests := []string{
		"device-1\r\nbad",
		strings.Repeat("x", maxServiceResourceIDBytes+1),
	}
	for _, id := range tests {
		if err := service.DeletePushDevice(context.Background(), "user-1", id); err == nil {
			t.Fatalf("DeletePushDevice accepted device ID %q", id)
		}
	}
	if repo.lastDeletePushDeviceID != "" {
		t.Fatalf("repository was called with device ID %q", repo.lastDeletePushDeviceID)
	}
}

func TestMessageDeliveryStatusNormalizesIDs(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{}
	service := New(repo, nil)

	if _, err := service.MessageDeliveryStatus(context.Background(), " user-1 ", " msg-1 "); err != nil {
		t.Fatalf("MessageDeliveryStatus returned error: %v", err)
	}
	if repo.lastDeliveryStatusUserID != "user-1" || repo.lastDeliveryStatusMessageID != "msg-1" {
		t.Fatalf("delivery status ids = %q/%q", repo.lastDeliveryStatusUserID, repo.lastDeliveryStatusMessageID)
	}
}

func TestAttachmentReadMethodsNormalizeIDs(t *testing.T) {
	t.Parallel()

	store := storage.NewLocalStore(t.TempDir())
	if err := store.Put(context.Background(), "attachments/body.txt", strings.NewReader("content")); err != nil {
		t.Fatalf("Put returned error: %v", err)
	}
	repo := &fakeRepository{
		attachments: []maildb.Attachment{{ID: "att-1"}},
		attachment:  maildb.Attachment{ID: "att-1", StoragePath: "attachments/body.txt"},
	}
	service := New(repo, store)

	if _, err := service.ListAttachments(context.Background(), " user-1 ", " msg-1 "); err != nil {
		t.Fatalf("ListAttachments returned error: %v", err)
	}
	if repo.lastAttachmentUserID != "user-1" || repo.lastAttachmentMessageID != "msg-1" {
		t.Fatalf("list attachment ids = %q/%q", repo.lastAttachmentUserID, repo.lastAttachmentMessageID)
	}

	download, err := service.OpenAttachment(context.Background(), " user-1 ", " msg-1 ", " att-1 ")
	if err != nil {
		t.Fatalf("OpenAttachment returned error: %v", err)
	}
	_ = download.Body.Close()
	if repo.lastAttachmentUserID != "user-1" || repo.lastAttachmentMessageID != "msg-1" || repo.lastAttachmentID != "att-1" {
		t.Fatalf("open attachment ids = %q/%q/%q", repo.lastAttachmentUserID, repo.lastAttachmentMessageID, repo.lastAttachmentID)
	}

	metadata, err := service.StatAttachment(context.Background(), " user-1 ", " msg-1 ", " att-1 ")
	if err != nil {
		t.Fatalf("StatAttachment returned error: %v", err)
	}
	if metadata.Object.Size != int64(len("content")) {
		t.Fatalf("attachment object size = %d", metadata.Object.Size)
	}
	if repo.lastAttachmentUserID != "user-1" || repo.lastAttachmentMessageID != "msg-1" || repo.lastAttachmentID != "att-1" {
		t.Fatalf("stat attachment ids = %q/%q/%q", repo.lastAttachmentUserID, repo.lastAttachmentMessageID, repo.lastAttachmentID)
	}
}

func TestOpenAttachmentRejectsUnsafeStoredBodyPath(t *testing.T) {
	t.Parallel()

	store := &recordingStore{}
	repo := &fakeRepository{
		attachment: maildb.Attachment{ID: "att-1", StoragePath: "/var/mail/att-1.bin"},
	}
	service := New(repo, store)

	if _, err := service.OpenAttachment(context.Background(), "user-1", "msg-1", "att-1"); err == nil {
		t.Fatal("OpenAttachment accepted unsafe stored body path")
	}
	if store.getPath != "" {
		t.Fatalf("store.Get was called with %q", store.getPath)
	}
}

func TestAttachmentReadMethodsRejectUnsafeIDs(t *testing.T) {
	t.Parallel()

	store := storage.NewLocalStore(t.TempDir())
	repo := &fakeRepository{}
	service := New(repo, store)

	if _, err := service.ListAttachments(context.Background(), "user-1", strings.Repeat("x", maxServiceResourceIDBytes+1)); err == nil {
		t.Fatal("ListAttachments accepted oversized message ID")
	}
	if _, err := service.OpenAttachment(context.Background(), "user-1", "msg-1", "att-1\nbad"); err == nil {
		t.Fatal("OpenAttachment accepted newline-bearing attachment ID")
	}
	if _, err := service.StatAttachment(context.Background(), "user-1", "msg-1", "att-1\nbad"); err == nil {
		t.Fatal("StatAttachment accepted newline-bearing attachment ID")
	}
	if repo.lastAttachmentID != "" || repo.lastAttachmentMessageID != "" {
		t.Fatalf("repository was called with attachment IDs %q/%q", repo.lastAttachmentMessageID, repo.lastAttachmentID)
	}
}

func TestDeleteDraftNormalizesIDs(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{}
	service := New(repo, nil)

	if err := service.DeleteDraft(context.Background(), " user-1 ", " draft-1 "); err != nil {
		t.Fatalf("DeleteDraft returned error: %v", err)
	}
	if repo.lastSenderUserID != "user-1" || repo.lastSentDraftID != "draft-1" {
		t.Fatalf("delete draft ids = %q/%q", repo.lastSenderUserID, repo.lastSentDraftID)
	}
}

func TestDraftLifecycleRejectsUnsafeDraftID(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{}
	service := New(repo, nil)

	if err := service.DeleteDraft(context.Background(), "user-1", strings.Repeat("x", maxServiceResourceIDBytes+1)); err == nil {
		t.Fatal("DeleteDraft accepted oversized draft ID")
	}
	if _, err := service.SendDraft(context.Background(), "user-1", "draft-1\r\nbad"); err == nil {
		t.Fatal("SendDraft accepted newline-bearing draft ID")
	}
	if repo.lastSentDraftID != "" {
		t.Fatalf("repository was called with draft ID %q", repo.lastSentDraftID)
	}
}

func TestFetchIMAPMessageOpensRawStoredBody(t *testing.T) {
	t.Parallel()

	store := storage.NewLocalStore(t.TempDir())
	path := "mailstore/c/d/u/maildir/2026/05/imap.eml"
	raw := "Subject: raw\r\n\r\nbody"
	if err := store.Put(context.Background(), path, strings.NewReader(raw)); err != nil {
		t.Fatalf("Put returned error: %v", err)
	}

	repo := &fakeRepository{
		imapMessage: maildb.IMAPStoredMessage{
			Summary: imapgw.MessageSummary{
				ID:        "msg-1",
				MailboxID: "inbox",
				UID:       12,
			},
			StoragePath: path,
		},
	}
	service := New(repo, store)

	msg, err := service.FetchIMAPMessage(context.Background(), imapgw.FetchMessageRequest{
		UserID:    " user-1 ",
		MailboxID: "inbox",
		UID:       12,
	})
	if err != nil {
		t.Fatalf("FetchIMAPMessage returned error: %v", err)
	}
	defer msg.Body.Close()

	got, err := io.ReadAll(msg.Body)
	if err != nil {
		t.Fatalf("ReadAll returned error: %v", err)
	}
	if string(got) != raw {
		t.Fatalf("body = %q, want raw stored body", string(got))
	}
	if msg.Summary.UID != 12 || msg.Summary.ID != "msg-1" {
		t.Fatalf("summary = %#v, want repository summary", msg.Summary)
	}
	if repo.lastIMAPMessageUserID != "user-1" || repo.lastIMAPMessageMailboxID != "inbox" {
		t.Fatalf("imap fetch ids = %q/%q", repo.lastIMAPMessageUserID, repo.lastIMAPMessageMailboxID)
	}
}

func TestFetchIMAPMessagePreservesMailboxIDSpacing(t *testing.T) {
	t.Parallel()

	store := storage.NewLocalStore(t.TempDir())
	raw := "Subject: spaced\r\n\r\nhello"
	path := "messages/user-1/msg-1.eml"
	if err := store.Put(context.Background(), path, strings.NewReader(raw)); err != nil {
		t.Fatalf("Put returned error: %v", err)
	}
	repo := &fakeRepository{
		imapMessage: maildb.IMAPStoredMessage{
			Summary:     imapgw.MessageSummary{ID: "msg-1", MailboxID: " spaced-inbox ", UID: 12},
			StoragePath: path,
		},
	}
	service := New(repo, store)

	msg, err := service.FetchIMAPMessage(context.Background(), imapgw.FetchMessageRequest{
		UserID:    " user-1 ",
		MailboxID: " INBOX ",
		UID:       12,
	})
	if err != nil {
		t.Fatalf("FetchIMAPMessage returned error: %v", err)
	}
	defer msg.Body.Close()

	if repo.lastIMAPMessageUserID != "user-1" || repo.lastIMAPMessageMailboxID != " INBOX " {
		t.Fatalf("imap fetch ids = %q/%q, want user-1/spaced INBOX", repo.lastIMAPMessageUserID, repo.lastIMAPMessageMailboxID)
	}
	if msg.Summary.MailboxID != " spaced-inbox " {
		t.Fatalf("summary = %#v, want spaced mailbox id", msg.Summary)
	}
}

func TestListIMAPMailboxesDelegatesToRepository(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{
		imapMailboxes: []imapgw.Mailbox{{ID: "inbox", Name: "INBOX"}},
	}
	service := New(repo, nil)

	got, err := service.ListIMAPMailboxes(context.Background(), imapgw.ListMailboxesRequest{UserID: " user-1 "})
	if err != nil {
		t.Fatalf("ListIMAPMailboxes returned error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "inbox" || repo.lastIMAPMailboxUserID != "user-1" {
		t.Fatalf("mailboxes = %#v, user = %q", got, repo.lastIMAPMailboxUserID)
	}
}

func TestListSubscribedIMAPMailboxesDelegatesToRepository(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{
		imapSubscriptions: []imapgw.MailboxSubscription{{Name: "INBOX", Mailbox: imapgw.Mailbox{ID: "inbox", Name: "INBOX"}, Exists: true}},
	}
	service := New(repo, nil)

	got, err := service.ListSubscribedIMAPMailboxes(context.Background(), imapgw.ListMailboxesRequest{UserID: " user-1 "})
	if err != nil {
		t.Fatalf("ListSubscribedIMAPMailboxes returned error: %v", err)
	}
	if len(got) != 1 || got[0].Name != "INBOX" || repo.lastIMAPMailboxUserID != "user-1" {
		t.Fatalf("subscriptions = %#v, user = %q", got, repo.lastIMAPMailboxUserID)
	}
}

func TestIMAPMailboxSubscriptionCommandsDelegateToRepository(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{
		imapSubscription: imapgw.MailboxSubscription{Name: "INBOX", Mailbox: imapgw.Mailbox{ID: "inbox", Name: "INBOX"}, Exists: true},
	}
	service := New(repo, nil)

	got, err := service.SubscribeIMAPMailboxName(context.Background(), " user-1 ", "inbox")
	if err != nil {
		t.Fatalf("SubscribeIMAPMailboxName returned error: %v", err)
	}
	if got.Name != "INBOX" || repo.lastIMAPMailboxUserID != "user-1" || repo.lastIMAPMessageMailboxID != "inbox" {
		t.Fatalf("subscription = %#v, ids = %q/%q", got, repo.lastIMAPMailboxUserID, repo.lastIMAPMessageMailboxID)
	}
	if err := service.UnsubscribeIMAPMailboxName(context.Background(), " user-1 ", "inbox"); err != nil {
		t.Fatalf("UnsubscribeIMAPMailboxName returned error: %v", err)
	}
	if repo.lastUnsubscribeIMAPMailboxID != "inbox" {
		t.Fatalf("unsubscribe mailbox id = %q, want inbox", repo.lastUnsubscribeIMAPMailboxID)
	}
}

func TestIMAPMailboxSubscriptionCommandsPreserveMailboxNameSpacing(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{
		imapSubscription: imapgw.MailboxSubscription{Name: " INBOX ", Exists: false},
	}
	service := New(repo, nil)

	got, err := service.SubscribeIMAPMailboxName(context.Background(), " user-1 ", " INBOX ")
	if err != nil {
		t.Fatalf("SubscribeIMAPMailboxName returned error: %v", err)
	}
	if got.Name != " INBOX " || repo.lastIMAPMailboxUserID != "user-1" || repo.lastIMAPMessageMailboxID != " INBOX " {
		t.Fatalf("subscription = %#v, ids = %q/%q", got, repo.lastIMAPMailboxUserID, repo.lastIMAPMessageMailboxID)
	}
	if err := service.UnsubscribeIMAPMailboxName(context.Background(), " user-1 ", " INBOX "); err != nil {
		t.Fatalf("UnsubscribeIMAPMailboxName returned error: %v", err)
	}
	if repo.lastUnsubscribeIMAPMailboxID != " INBOX " {
		t.Fatalf("unsubscribe mailbox id = %q, want spaced INBOX", repo.lastUnsubscribeIMAPMailboxID)
	}
}

func TestListIMAPMessagesDelegatesToRepository(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{
		imapMessages: []imapgw.MessageSummary{{ID: "msg-1", MailboxID: "inbox", UID: 12}},
	}
	service := New(repo, nil)

	got, err := service.ListIMAPMessages(context.Background(), imapgw.ListMessagesRequest{
		UserID:    " user-1 ",
		MailboxID: "inbox",
		Limit:     500,
		AfterUID:  11,
	})
	if err != nil {
		t.Fatalf("ListIMAPMessages returned error: %v", err)
	}
	if len(got) != 1 || got[0].UID != 12 || repo.lastIMAPMessageAfterUID != 11 {
		t.Fatalf("messages = %#v, after uid = %d", got, repo.lastIMAPMessageAfterUID)
	}
	if repo.lastIMAPMessageUserID != "user-1" || repo.lastIMAPMessageMailboxID != "inbox" || repo.lastIMAPMessageLimit != maildb.MessageListMaxLimit {
		t.Fatalf("imap list = %q/%q/%d", repo.lastIMAPMessageUserID, repo.lastIMAPMessageMailboxID, repo.lastIMAPMessageLimit)
	}
}

func TestListIMAPMessagesPreservesMailboxIDSpacing(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{
		imapMessages: []imapgw.MessageSummary{{ID: "msg-1", MailboxID: " spaced-inbox ", UID: 12}},
	}
	service := New(repo, nil)

	got, err := service.ListIMAPMessages(context.Background(), imapgw.ListMessagesRequest{
		UserID:    " user-1 ",
		MailboxID: " INBOX ",
		Limit:     50,
	})
	if err != nil {
		t.Fatalf("ListIMAPMessages returned error: %v", err)
	}
	if len(got) != 1 || got[0].MailboxID != " spaced-inbox " {
		t.Fatalf("messages = %#v, want spaced mailbox result", got)
	}
	if repo.lastIMAPMessageUserID != "user-1" || repo.lastIMAPMessageMailboxID != " INBOX " {
		t.Fatalf("imap list = %q/%q, want user-1/spaced INBOX", repo.lastIMAPMessageUserID, repo.lastIMAPMessageMailboxID)
	}
}

func TestIMAPReadServicesRejectUnsafeIdentifiers(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		call   func(*Service) error
		called func(*fakeRepository) bool
	}{
		{
			name: "fetch unsafe mailbox",
			call: func(service *Service) error {
				_, err := service.FetchIMAPMessage(context.Background(), imapgw.FetchMessageRequest{
					UserID:    "user-1",
					MailboxID: "inbox\nbad",
					UID:       12,
				})
				return err
			},
			called: func(repo *fakeRepository) bool { return repo.lastIMAPMessageUserID != "" },
		},
		{
			name: "list mailboxes unsafe user",
			call: func(service *Service) error {
				_, err := service.ListIMAPMailboxes(context.Background(), imapgw.ListMailboxesRequest{UserID: "user-1\r\nbad"})
				return err
			},
			called: func(repo *fakeRepository) bool { return repo.lastIMAPMailboxUserID != "" },
		},
		{
			name: "list subscribed unsafe user",
			call: func(service *Service) error {
				_, err := service.ListSubscribedIMAPMailboxes(context.Background(), imapgw.ListMailboxesRequest{
					UserID: imapgw.UserID(strings.Repeat("u", maxServiceResourceIDBytes+1)),
				})
				return err
			},
			called: func(repo *fakeRepository) bool { return repo.lastIMAPMailboxUserID != "" },
		},
		{
			name: "get mailbox unsafe mailbox",
			call: func(service *Service) error {
				_, err := service.GetIMAPMailbox(context.Background(), "user-1", "inbox\r\nbad")
				return err
			},
			called: func(repo *fakeRepository) bool { return repo.lastIMAPMailboxUserID != "" },
		},
		{
			name: "subscribe mailbox unsafe user",
			call: func(service *Service) error {
				_, err := service.SubscribeIMAPMailboxName(context.Background(), "user-1\nbad", "inbox")
				return err
			},
			called: func(repo *fakeRepository) bool { return repo.lastIMAPMailboxUserID != "" },
		},
		{
			name: "unsubscribe mailbox unsafe mailbox",
			call: func(service *Service) error {
				return service.UnsubscribeIMAPMailboxName(context.Background(), "user-1", "inbox\nbad")
			},
			called: func(repo *fakeRepository) bool { return repo.lastIMAPMailboxUserID != "" },
		},
		{
			name: "list messages unsafe mailbox",
			call: func(service *Service) error {
				_, err := service.ListIMAPMessages(context.Background(), imapgw.ListMessagesRequest{
					UserID:    "user-1",
					MailboxID: imapgw.MailboxID(strings.Repeat("m", maxServiceResourceIDBytes+1)),
				})
				return err
			},
			called: func(repo *fakeRepository) bool { return repo.lastIMAPMessageUserID != "" },
		},
		{
			name: "backfill unsafe user",
			call: func(service *Service) error {
				_, err := service.BackfillIMAPMailboxUIDs(context.Background(), "user-1\r\nbad", "inbox", 10)
				return err
			},
			called: func(repo *fakeRepository) bool { return repo.lastBackfillUserID != "" },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			store := storage.NewLocalStore(t.TempDir())
			repo := &fakeRepository{}
			service := New(repo, store)
			if err := tc.call(service); err == nil {
				t.Fatal("IMAP read service accepted unsafe identifier")
			}
			if tc.called(repo) {
				t.Fatal("repository was called before identifier validation")
			}
		})
	}
}

func TestIMAPServicesRejectZeroUIDs(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		call   func(*Service) error
		called func(*fakeRepository) bool
	}{
		{
			name: "fetch zero uid",
			call: func(service *Service) error {
				_, err := service.FetchIMAPMessage(context.Background(), imapgw.FetchMessageRequest{
					UserID:    "user-1",
					MailboxID: "inbox",
					UID:       0,
				})
				return err
			},
			called: func(repo *fakeRepository) bool { return repo.lastIMAPMessageUserID != "" },
		},
		{
			name: "store zero uid",
			call: func(service *Service) error {
				_, err := service.StoreIMAPFlags(context.Background(), imapgw.StoreFlagsRequest{
					UserID:    "user-1",
					MailboxID: "inbox",
					UIDs:      []imapgw.UID{12, 0},
					Flags:     imapgw.MessageFlags{Read: true},
					Mode:      imapgw.StoreFlagsAdd,
				})
				return err
			},
			called: func(repo *fakeRepository) bool { return repo.lastIMAPFlagUserID != "" },
		},
		{
			name: "copy zero uid",
			call: func(service *Service) error {
				_, err := service.CopyIMAPMessages(context.Background(), imapgw.CopyMessagesRequest{
					UserID:          "user-1",
					SourceMailboxID: "inbox",
					DestMailboxID:   "archive",
					UIDs:            []imapgw.UID{0},
				})
				return err
			},
			called: func(repo *fakeRepository) bool { return repo.lastIMAPCopyUserID != "" },
		},
		{
			name: "move zero uid",
			call: func(service *Service) error {
				_, err := service.MoveIMAPMessages(context.Background(), imapgw.MoveMessagesRequest{
					UserID:          "user-1",
					SourceMailboxID: "inbox",
					DestMailboxID:   "archive",
					UIDs:            []imapgw.UID{0},
				})
				return err
			},
			called: func(repo *fakeRepository) bool { return repo.lastIMAPMoveUserID != "" },
		},
		{
			name: "expunge zero uid",
			call: func(service *Service) error {
				_, err := service.ExpungeIMAPMessages(context.Background(), imapgw.ExpungeRequest{
					UserID:    "user-1",
					MailboxID: "inbox",
					UIDs:      []imapgw.UID{0},
				})
				return err
			},
			called: func(repo *fakeRepository) bool { return repo.lastIMAPExpungeUserID != "" },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			repo := &fakeRepository{}
			service := New(repo, storage.NewLocalStore(t.TempDir()))
			if err := tc.call(service); err == nil {
				t.Fatal("IMAP service accepted zero UID")
			}
			if tc.called(repo) {
				t.Fatal("repository was called before UID validation")
			}
		})
	}
}

func TestIMAPMutationServicesRejectEmptyUIDSets(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		call   func(*Service) error
		called func(*fakeRepository) bool
	}{
		{
			name: "store empty uids",
			call: func(service *Service) error {
				_, err := service.StoreIMAPFlags(context.Background(), imapgw.StoreFlagsRequest{
					UserID:    "user-1",
					MailboxID: "inbox",
					Flags:     imapgw.MessageFlags{Read: true},
					Mode:      imapgw.StoreFlagsAdd,
				})
				return err
			},
			called: func(repo *fakeRepository) bool { return repo.lastIMAPFlagUserID != "" },
		},
		{
			name: "copy empty uids",
			call: func(service *Service) error {
				_, err := service.CopyIMAPMessages(context.Background(), imapgw.CopyMessagesRequest{
					UserID:          "user-1",
					SourceMailboxID: "inbox",
					DestMailboxID:   "archive",
				})
				return err
			},
			called: func(repo *fakeRepository) bool { return repo.lastIMAPCopyUserID != "" },
		},
		{
			name: "move empty uids",
			call: func(service *Service) error {
				_, err := service.MoveIMAPMessages(context.Background(), imapgw.MoveMessagesRequest{
					UserID:          "user-1",
					SourceMailboxID: "inbox",
					DestMailboxID:   "archive",
				})
				return err
			},
			called: func(repo *fakeRepository) bool { return repo.lastIMAPMoveUserID != "" },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			repo := &fakeRepository{}
			service := New(repo, nil)
			if err := tc.call(service); err == nil {
				t.Fatal("IMAP mutation service accepted empty UID set")
			}
			if tc.called(repo) {
				t.Fatal("repository was called before empty UID validation")
			}
		})
	}
}

func TestExpungeIMAPMessagesAllowsNilUIDsForCloseSemantics(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{
		imapExpungeSummaries: []imapgw.MessageSummary{{ID: "msg-1", MailboxID: "inbox", UID: 12, SequenceNumber: 1}},
	}
	service := New(repo, nil)

	got, err := service.ExpungeIMAPMessages(context.Background(), imapgw.ExpungeRequest{
		UserID:    "user-1",
		MailboxID: "inbox",
		UIDs:      nil,
	})
	if err != nil {
		t.Fatalf("ExpungeIMAPMessages returned error: %v", err)
	}
	if len(got) != 1 || repo.lastIMAPExpungeUserID != "user-1" || repo.lastIMAPExpungeMailboxID != "inbox" || repo.lastIMAPExpungeUIDs != nil {
		t.Fatalf("expunge result = %#v, request = %q/%q/%v", got, repo.lastIMAPExpungeUserID, repo.lastIMAPExpungeMailboxID, repo.lastIMAPExpungeUIDs)
	}
}

func TestSubscribeIMAPMailboxUsesEventBroker(t *testing.T) {
	t.Parallel()

	broker := imapgw.NewMailboxEventBroker(1)
	service := New(&fakeRepository{}, nil).WithIMAPMailboxEvents(broker)
	events, cancel, err := service.SubscribeIMAPMailbox(context.Background(), "user-1", "inbox")
	if err != nil {
		t.Fatalf("SubscribeIMAPMailbox returned error: %v", err)
	}
	defer cancel()

	if err := broker.Publish(context.Background(), imapgw.MailboxEvent{Type: imapgw.MailboxEventExists, UserID: "user-1", MailboxID: "inbox", Messages: 1}); err != nil {
		t.Fatalf("Publish returned error: %v", err)
	}
	select {
	case got := <-events:
		if got.Type != imapgw.MailboxEventExists || got.Messages != 1 {
			t.Fatalf("event = %#v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for imap event")
	}
}

func TestSubscribeIMAPMailboxNormalizesEventIdentity(t *testing.T) {
	t.Parallel()

	broker := imapgw.NewMailboxEventBroker(2)
	service := New(&fakeRepository{}, nil).WithIMAPMailboxEvents(broker)
	events, cancel, err := service.SubscribeIMAPMailbox(context.Background(), " user-1 ", " INBOX ")
	if err != nil {
		t.Fatalf("SubscribeIMAPMailbox returned error: %v", err)
	}
	defer cancel()

	if err := broker.Publish(context.Background(), imapgw.MailboxEvent{Type: imapgw.MailboxEventExists, UserID: "user-1", MailboxID: "INBOX", Messages: 1}); err != nil {
		t.Fatalf("Publish trimmed mailbox event returned error: %v", err)
	}
	select {
	case got := <-events:
		if got.Type != imapgw.MailboxEventExists || got.UserID != "user-1" || got.MailboxID != "INBOX" || got.Messages != 1 {
			t.Fatalf("event = %#v, want normalized trimmed mailbox event", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for normalized trimmed mailbox event")
	}

	if err := broker.Publish(context.Background(), imapgw.MailboxEvent{Type: imapgw.MailboxEventExists, UserID: "user-1", MailboxID: " INBOX ", Messages: 2}); err != nil {
		t.Fatalf("Publish exact mailbox event returned error: %v", err)
	}
	select {
	case got := <-events:
		if got.Type != imapgw.MailboxEventExists || got.UserID != "user-1" || got.MailboxID != "INBOX" || got.Messages != 2 {
			t.Fatalf("event = %#v, want normalized spaced mailbox event", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for normalized spaced mailbox event")
	}
}

func TestIMAPStoreAdapterSelectsMailboxState(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{
		imapMailboxes: []imapgw.Mailbox{{ID: "inbox", Name: "INBOX", UIDValidity: 42, UIDNext: 99}},
	}
	adapter := NewIMAPStoreAdapter(New(repo, nil))

	state, err := adapter.SelectMailbox(context.Background(), imapgw.SelectMailboxRequest{
		UserID:    " user-1 ",
		MailboxID: "inbox",
	})
	if err != nil {
		t.Fatalf("SelectMailbox returned error: %v", err)
	}
	if state.ID != "inbox" || state.UIDValidity != 42 || state.UIDNext != 99 {
		t.Fatalf("state = %#v, want mailbox UID state", state)
	}
	if repo.lastIMAPMailboxUserID != "user-1" || repo.lastIMAPMessageMailboxID != "inbox" {
		t.Fatalf("select ids = %q/%q", repo.lastIMAPMailboxUserID, repo.lastIMAPMessageMailboxID)
	}
	wantFlags := []string{imapgw.FlagSeen, imapgw.FlagFlagged, imapgw.FlagAnswered, imapgw.FlagForwarded, imapgw.FlagDraft, imapgw.FlagDeleted}
	if strings.Join(state.PermanentFlags, ",") != strings.Join(wantFlags, ",") {
		t.Fatalf("PermanentFlags = %#v, want %#v", state.PermanentFlags, wantFlags)
	}
}

func TestIMAPStoreAdapterSelectPreservesMailboxIDSpacing(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{
		imapMailboxes: []imapgw.Mailbox{{ID: " spaced-inbox ", Name: " INBOX ", UIDValidity: 42, UIDNext: 99}},
	}
	adapter := NewIMAPStoreAdapter(New(repo, nil))

	state, err := adapter.SelectMailbox(context.Background(), imapgw.SelectMailboxRequest{
		UserID:    " user-1 ",
		MailboxID: " INBOX ",
	})
	if err != nil {
		t.Fatalf("SelectMailbox returned error: %v", err)
	}
	if state.ID != " spaced-inbox " || state.Name != " INBOX " {
		t.Fatalf("state = %#v, want spaced mailbox identity", state)
	}
	if repo.lastIMAPMailboxUserID != "user-1" || repo.lastIMAPMessageMailboxID != " INBOX " {
		t.Fatalf("select ids = %q/%q, want user-1/spaced INBOX", repo.lastIMAPMailboxUserID, repo.lastIMAPMessageMailboxID)
	}
}

func TestIMAPAuthenticatorAdapterUsesSubmissionCredentials(t *testing.T) {
	t.Parallel()

	auth := fakeSubmissionAuthenticator{
		user: smtpd.SubmissionUser{
			UserID:      " user-1 ",
			DomainID:    " domain-1 ",
			Address:     " user@example.com ",
			DisplayName: " User ",
		},
	}
	adapter := NewIMAPAuthenticatorAdapter(&auth)

	session, err := adapter.Authenticate(context.Background(), " user@example.com ", "secret")
	if err != nil {
		t.Fatalf("Authenticate returned error: %v", err)
	}
	if auth.username != "user@example.com" || auth.password != "secret" {
		t.Fatalf("credentials = %q/%q", auth.username, auth.password)
	}
	if session.UserID != "user-1" || session.DomainID != "domain-1" || session.Username != "user@example.com" || session.DisplayName != "User" {
		t.Fatalf("session = %#v", session)
	}
}

func TestIMAPAuthenticatorAdapterRejectsUnsafeCredentials(t *testing.T) {
	t.Parallel()

	adapter := NewIMAPAuthenticatorAdapter(&fakeSubmissionAuthenticator{})
	if _, err := adapter.Authenticate(context.Background(), "user\nbad", "secret"); err == nil {
		t.Fatal("Authenticate accepted unsafe username")
	}
	if _, err := adapter.Authenticate(context.Background(), "user@example.com", "secret\nbad"); err == nil {
		t.Fatal("Authenticate accepted unsafe password")
	}
}

func TestIMAPAuthenticatorAdapterRejectsMustChangePassword(t *testing.T) {
	t.Parallel()

	auth := fakeSubmissionAuthenticator{
		user: smtpd.SubmissionUser{
			UserID:             "user-1",
			DomainID:           "domain-1",
			Address:            "user@example.com",
			MustChangePassword: true,
		},
	}
	adapter := NewIMAPAuthenticatorAdapter(&auth)

	if _, err := adapter.Authenticate(context.Background(), "user@example.com", "secret"); err == nil {
		t.Fatal("Authenticate accepted user that must change password")
	}
}

func TestIMAPBackendAdapterComposesAuthenticatorAndSessionStore(t *testing.T) {
	t.Parallel()

	auth := &fakeSubmissionAuthenticator{user: smtpd.SubmissionUser{UserID: "user-1", DomainID: "domain-1", Address: "user@example.com"}}
	service := New(&fakeRepository{imapMailboxes: []imapgw.Mailbox{{ID: "inbox", Name: "INBOX", UIDValidity: 1, UIDNext: 2}}}, nil)
	backend := NewIMAPBackendAdapter(auth, service)

	session, err := backend.Authenticate(context.Background(), "user@example.com", "secret")
	if err != nil {
		t.Fatalf("Authenticate returned error: %v", err)
	}
	state, err := backend.SelectMailbox(context.Background(), imapgw.SelectMailboxRequest{UserID: session.UserID, MailboxID: "inbox"})
	if err != nil {
		t.Fatalf("SelectMailbox returned error: %v", err)
	}
	if session.UserID != "user-1" || state.ID != "inbox" || state.UIDValidity != 1 {
		t.Fatalf("session/state = %#v/%#v", session, state)
	}
}

func TestBackfillIMAPMailboxUIDsDelegatesToRepository(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{
		backfilledIMAPUIDs: []maildb.IMAPMessageUID{{MessageID: "msg-1", MailboxID: "inbox", UID: 12, ModSeq: 2}},
	}
	service := New(repo, nil)

	got, err := service.BackfillIMAPMailboxUIDs(context.Background(), " user-1 ", "inbox", 500)
	if err != nil {
		t.Fatalf("BackfillIMAPMailboxUIDs returned error: %v", err)
	}
	if len(got) != 1 || got[0].UID != 12 || repo.lastBackfillLimit != maildb.MessageListMaxLimit {
		t.Fatalf("backfill = %#v, limit = %d", got, repo.lastBackfillLimit)
	}
	if repo.lastBackfillUserID != "user-1" || repo.lastBackfillMailboxID != "inbox" {
		t.Fatalf("backfill ids = %q/%q", repo.lastBackfillUserID, repo.lastBackfillMailboxID)
	}
}

func TestBackfillIMAPMailboxUIDsPreservesMailboxIDSpacing(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{
		backfilledIMAPUIDs: []maildb.IMAPMessageUID{{MessageID: "msg-1", MailboxID: " spaced-inbox ", UID: 12, ModSeq: 2}},
	}
	service := New(repo, nil)

	got, err := service.BackfillIMAPMailboxUIDs(context.Background(), " user-1 ", " INBOX ", 50)
	if err != nil {
		t.Fatalf("BackfillIMAPMailboxUIDs returned error: %v", err)
	}
	if len(got) != 1 || got[0].MailboxID != " spaced-inbox " {
		t.Fatalf("backfill = %#v, want spaced mailbox result", got)
	}
	if repo.lastBackfillUserID != "user-1" || repo.lastBackfillMailboxID != " INBOX " {
		t.Fatalf("backfill ids = %q/%q, want user-1/spaced INBOX", repo.lastBackfillUserID, repo.lastBackfillMailboxID)
	}
}

func TestStoreIMAPFlagsDelegatesToRepository(t *testing.T) {
	t.Parallel()

	events := &fakeIMAPEventPublisher{}
	repo := &fakeRepository{
		imapFlagSummaries: []imapgw.MessageSummary{{ID: "msg-1", MailboxID: "inbox", UID: 12}},
	}
	service := New(repo, nil).WithIMAPMailboxEvents(events)

	got, err := service.StoreIMAPFlags(context.Background(), imapgw.StoreFlagsRequest{
		UserID:            " user-1 ",
		MailboxID:         "inbox",
		UIDs:              []imapgw.UID{12},
		Flags:             imapgw.MessageFlags{Read: true},
		Mode:              imapgw.StoreFlagsAdd,
		UnchangedSince:    42,
		UnchangedSinceSet: true,
	})
	if err != nil {
		t.Fatalf("StoreIMAPFlags returned error: %v", err)
	}
	if len(got) != 1 || got[0].UID != 12 {
		t.Fatalf("summaries = %#v, want repository result", got)
	}
	if repo.lastIMAPFlagMode != imapgw.StoreFlagsAdd || !repo.lastIMAPFlags.Read {
		t.Fatalf("stored flags = %#v/%q, want read add", repo.lastIMAPFlags, repo.lastIMAPFlagMode)
	}
	if repo.lastIMAPFlagUnchangedSince != 42 {
		t.Fatalf("unchanged since = %d, want 42", repo.lastIMAPFlagUnchangedSince)
	}
	if !repo.lastIMAPFlagUnchangedSinceSet {
		t.Fatal("unchanged since set = false, want true")
	}
	if repo.lastIMAPFlagUserID != "user-1" || repo.lastIMAPFlagMailboxID != "inbox" {
		t.Fatalf("store flags ids = %q/%q", repo.lastIMAPFlagUserID, repo.lastIMAPFlagMailboxID)
	}
	if len(events.events) != 1 || events.events[0].Type != imapgw.MailboxEventFlags || events.events[0].UserID != "user-1" || events.events[0].MailboxID != "inbox" || events.events[0].UID != 12 {
		t.Fatalf("events = %#v, want flags event", events.events)
	}
}

func TestCreateIMAPMailboxUsesFolderBoundary(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{
		imapMailboxes: []imapgw.Mailbox{{ID: "archive-id", Name: "Archive", UIDValidity: 7, UIDNext: 1}},
	}
	service := New(repo, nil)

	got, err := service.CreateIMAPMailbox(context.Background(), " user-1 ", " /Archive ")
	if err != nil {
		t.Fatalf("CreateIMAPMailbox returned error: %v", err)
	}
	if got.ID != "archive-id" || got.UIDValidity != 7 || got.UIDNext != 1 {
		t.Fatalf("mailbox = %#v, want IMAP mailbox state from repository", got)
	}
	if repo.lastCreateFolder.UserID != "user-1" || repo.lastCreateFolder.Name != "Archive" {
		t.Fatalf("create folder request = %#v, want trimmed user/name", repo.lastCreateFolder)
	}
	if repo.lastIMAPMailboxUserID != "user-1" || repo.lastIMAPMessageMailboxID != "archive-id" {
		t.Fatalf("created mailbox lookup = %q/%q, want user-1/archive-id", repo.lastIMAPMailboxUserID, repo.lastIMAPMessageMailboxID)
	}
}

func TestDeleteIMAPMailboxResolvesWireName(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{
		imapMailboxes: []imapgw.Mailbox{{ID: "archive-id", Name: "Archive", UIDValidity: 7, UIDNext: 1}},
	}
	service := New(repo, nil)

	if err := service.DeleteIMAPMailbox(context.Background(), " user-1 ", " Archive "); err != nil {
		t.Fatalf("DeleteIMAPMailbox returned error: %v", err)
	}
	if repo.lastDeleteFolderUserID != "user-1" || repo.lastDeleteFolderID != "archive-id" {
		t.Fatalf("delete folder ids = %q/%q, want user-1/archive-id", repo.lastDeleteFolderUserID, repo.lastDeleteFolderID)
	}
}

func TestRenameIMAPMailboxResolvesWireName(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{
		imapMailboxes: []imapgw.Mailbox{{ID: "projects-id", Name: "Projects", UIDValidity: 7, UIDNext: 1}},
	}
	service := New(repo, nil)

	if _, err := service.RenameIMAPMailbox(context.Background(), " user-1 ", " Projects ", " /Archive "); err != nil {
		t.Fatalf("RenameIMAPMailbox returned error: %v", err)
	}
	if repo.lastRenameFolderUserID != "user-1" || repo.lastRenameFolderID != "projects-id" || repo.lastRenameFolderName != "Archive" {
		t.Fatalf("rename folder = %q/%q/%q, want user-1/projects-id/Archive", repo.lastRenameFolderUserID, repo.lastRenameFolderID, repo.lastRenameFolderName)
	}
}

func TestCopyIMAPMessagesDelegatesToRepository(t *testing.T) {
	t.Parallel()

	events := &fakeIMAPEventPublisher{}
	repo := &fakeRepository{
		imapCopySummaries: []imapgw.CopyMessageResult{{SourceUID: 12, Destination: imapgw.MessageSummary{ID: "msg-copy-1", MailboxID: "archive", UID: 20}}},
	}
	service := New(repo, nil).WithIMAPMailboxEvents(events)

	got, err := service.CopyIMAPMessages(context.Background(), imapgw.CopyMessagesRequest{
		UserID:          " user-1 ",
		SourceMailboxID: "inbox",
		DestMailboxID:   "archive",
		UIDs:            []imapgw.UID{12, 13},
	})
	if err != nil {
		t.Fatalf("CopyIMAPMessages returned error: %v", err)
	}
	if len(got) != 1 || got[0].SourceUID != 12 || got[0].Destination.UID != 20 {
		t.Fatalf("summaries = %#v, want repository result", got)
	}
	if repo.lastIMAPCopyUserID != "user-1" || repo.lastIMAPCopySourceMailboxID != "inbox" || repo.lastIMAPCopyDestMailboxID != "archive" {
		t.Fatalf("copy ids = %q/%q/%q", repo.lastIMAPCopyUserID, repo.lastIMAPCopySourceMailboxID, repo.lastIMAPCopyDestMailboxID)
	}
	if !reflect.DeepEqual(repo.lastIMAPCopyUIDs, []imapgw.UID{12, 13}) {
		t.Fatalf("copy uids = %v, want [12 13]", repo.lastIMAPCopyUIDs)
	}
	if len(events.events) != 1 || events.events[0].Type != imapgw.MailboxEventExists || events.events[0].UserID != "user-1" || events.events[0].MailboxID != "archive" || events.events[0].UID != 20 {
		t.Fatalf("events = %#v, want exists event", events.events)
	}
}

func TestIMAPMutationServicesPreserveMailboxIDSpacing(t *testing.T) {
	t.Parallel()

	t.Run("store", func(t *testing.T) {
		t.Parallel()

		events := &fakeIMAPEventPublisher{}
		repo := &fakeRepository{
			imapFlagSummaries: []imapgw.MessageSummary{{ID: "msg-1", MailboxID: " spaced-inbox ", UID: 12}},
		}
		service := New(repo, nil).WithIMAPMailboxEvents(events)

		_, err := service.StoreIMAPFlags(context.Background(), imapgw.StoreFlagsRequest{
			UserID:    " user-1 ",
			MailboxID: " INBOX ",
			UIDs:      []imapgw.UID{12},
			Flags:     imapgw.MessageFlags{Read: true},
			Mode:      imapgw.StoreFlagsAdd,
		})
		if err != nil {
			t.Fatalf("StoreIMAPFlags returned error: %v", err)
		}
		if repo.lastIMAPFlagUserID != "user-1" || repo.lastIMAPFlagMailboxID != " INBOX " {
			t.Fatalf("store ids = %q/%q, want user-1/spaced INBOX", repo.lastIMAPFlagUserID, repo.lastIMAPFlagMailboxID)
		}
		if len(events.events) != 1 || events.events[0].MailboxID != " spaced-inbox " {
			t.Fatalf("events = %#v, want spaced summary mailbox id", events.events)
		}
	})

	t.Run("copy", func(t *testing.T) {
		t.Parallel()

		events := &fakeIMAPEventPublisher{}
		repo := &fakeRepository{
			imapCopySummaries: []imapgw.CopyMessageResult{{SourceUID: 12, Destination: imapgw.MessageSummary{ID: "msg-copy-1", MailboxID: " spaced-dest ", UID: 20}}},
		}
		service := New(repo, nil).WithIMAPMailboxEvents(events)

		_, err := service.CopyIMAPMessages(context.Background(), imapgw.CopyMessagesRequest{
			UserID:          " user-1 ",
			SourceMailboxID: " INBOX ",
			DestMailboxID:   " Archive ",
			UIDs:            []imapgw.UID{12},
		})
		if err != nil {
			t.Fatalf("CopyIMAPMessages returned error: %v", err)
		}
		if repo.lastIMAPCopyUserID != "user-1" || repo.lastIMAPCopySourceMailboxID != " INBOX " || repo.lastIMAPCopyDestMailboxID != " Archive " {
			t.Fatalf("copy ids = %q/%q/%q, want exact spaced ids", repo.lastIMAPCopyUserID, repo.lastIMAPCopySourceMailboxID, repo.lastIMAPCopyDestMailboxID)
		}
		if len(events.events) != 1 || events.events[0].MailboxID != " spaced-dest " {
			t.Fatalf("events = %#v, want spaced destination mailbox id", events.events)
		}
	})

	t.Run("move", func(t *testing.T) {
		t.Parallel()

		events := &fakeIMAPEventPublisher{}
		repo := &fakeRepository{
			imapMoveResults: []imapgw.MoveMessageResult{{
				Source:      imapgw.MessageSummary{ID: "msg-1", MailboxID: " spaced-source ", UID: 12},
				Destination: imapgw.MessageSummary{ID: "msg-1", MailboxID: " spaced-dest ", UID: 33},
			}},
		}
		service := New(repo, nil).WithIMAPMailboxEvents(events)

		_, err := service.MoveIMAPMessages(context.Background(), imapgw.MoveMessagesRequest{
			UserID:          " user-1 ",
			SourceMailboxID: " INBOX ",
			DestMailboxID:   " Archive ",
			UIDs:            []imapgw.UID{12},
		})
		if err != nil {
			t.Fatalf("MoveIMAPMessages returned error: %v", err)
		}
		if repo.lastIMAPMoveUserID != "user-1" || repo.lastIMAPMoveSourceMailboxID != " INBOX " || repo.lastIMAPMoveDestMailboxID != " Archive " {
			t.Fatalf("move ids = %q/%q/%q, want exact spaced ids", repo.lastIMAPMoveUserID, repo.lastIMAPMoveSourceMailboxID, repo.lastIMAPMoveDestMailboxID)
		}
		if len(events.events) != 1 || events.events[0].MailboxID != " spaced-source " {
			t.Fatalf("events = %#v, want spaced source mailbox id", events.events)
		}
	})

	t.Run("expunge", func(t *testing.T) {
		t.Parallel()

		events := &fakeIMAPEventPublisher{}
		repo := &fakeRepository{
			imapExpungeSummaries: []imapgw.MessageSummary{{ID: "msg-1", MailboxID: " spaced-inbox ", UID: 12}},
		}
		service := New(repo, nil).WithIMAPMailboxEvents(events)

		_, err := service.ExpungeIMAPMessages(context.Background(), imapgw.ExpungeRequest{
			UserID:    " user-1 ",
			MailboxID: " INBOX ",
			UIDs:      []imapgw.UID{12},
		})
		if err != nil {
			t.Fatalf("ExpungeIMAPMessages returned error: %v", err)
		}
		if repo.lastIMAPExpungeUserID != "user-1" || repo.lastIMAPExpungeMailboxID != " INBOX " {
			t.Fatalf("expunge ids = %q/%q, want user-1/spaced INBOX", repo.lastIMAPExpungeUserID, repo.lastIMAPExpungeMailboxID)
		}
		if len(events.events) != 1 || events.events[0].MailboxID != " spaced-inbox " {
			t.Fatalf("events = %#v, want spaced expunge mailbox id", events.events)
		}
	})
}

func TestIMAPMutationServicesRejectUnsafeIdentifiers(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		call   func(*Service) error
		called func(*fakeRepository) bool
	}{
		{
			name: "store unsafe user",
			call: func(service *Service) error {
				_, err := service.StoreIMAPFlags(context.Background(), imapgw.StoreFlagsRequest{
					UserID:    "user-1\nbad",
					MailboxID: "inbox",
					UIDs:      []imapgw.UID{12},
					Flags:     imapgw.MessageFlags{Read: true},
					Mode:      imapgw.StoreFlagsAdd,
				})
				return err
			},
			called: func(repo *fakeRepository) bool { return repo.lastIMAPFlagUserID != "" },
		},
		{
			name: "copy unsafe destination mailbox",
			call: func(service *Service) error {
				_, err := service.CopyIMAPMessages(context.Background(), imapgw.CopyMessagesRequest{
					UserID:          "user-1",
					SourceMailboxID: "inbox",
					DestMailboxID:   imapgw.MailboxID(strings.Repeat("m", maxServiceResourceIDBytes+1)),
					UIDs:            []imapgw.UID{12},
				})
				return err
			},
			called: func(repo *fakeRepository) bool { return repo.lastIMAPCopyUserID != "" },
		},
		{
			name: "move unsafe source mailbox",
			call: func(service *Service) error {
				_, err := service.MoveIMAPMessages(context.Background(), imapgw.MoveMessagesRequest{
					UserID:          "user-1",
					SourceMailboxID: "inbox\r\nbad",
					DestMailboxID:   "archive",
					UIDs:            []imapgw.UID{12},
				})
				return err
			},
			called: func(repo *fakeRepository) bool { return repo.lastIMAPMoveUserID != "" },
		},
		{
			name: "expunge unsafe mailbox",
			call: func(service *Service) error {
				_, err := service.ExpungeIMAPMessages(context.Background(), imapgw.ExpungeRequest{
					UserID:    "user-1",
					MailboxID: "inbox\nbad",
					UIDs:      []imapgw.UID{12},
				})
				return err
			},
			called: func(repo *fakeRepository) bool { return repo.lastIMAPExpungeUserID != "" },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			repo := &fakeRepository{}
			service := New(repo, nil)
			if err := tc.call(service); err == nil {
				t.Fatal("IMAP mutation service accepted unsafe identifier")
			}
			if tc.called(repo) {
				t.Fatal("repository was called before identifier validation")
			}
		})
	}
}

func TestStoreIMAPFlagsModifiedPublishesOnlySuccessfulSummaries(t *testing.T) {
	t.Parallel()

	events := &fakeIMAPEventPublisher{}
	repo := &fakeRepository{
		imapFlagSummaries: []imapgw.MessageSummary{
			{ID: "msg-1", MailboxID: "inbox", UID: 12},
			{ID: "msg-2", MailboxID: "inbox", UID: 13},
		},
		imapFlagErr: &imapgw.StoreModifiedError{
			UIDs: []imapgw.UID{13},
			Summaries: []imapgw.MessageSummary{
				{ID: "msg-1", MailboxID: "inbox", UID: 12},
				{ID: "msg-2", MailboxID: "inbox", UID: 13},
			},
		},
	}
	service := New(repo, nil).WithIMAPMailboxEvents(events)

	_, err := service.StoreIMAPFlags(context.Background(), imapgw.StoreFlagsRequest{
		UserID:    "user-1",
		MailboxID: "inbox",
		UIDs:      []imapgw.UID{12, 13},
		Flags:     imapgw.MessageFlags{Read: true},
		Mode:      imapgw.StoreFlagsAdd,
	})
	var modified *imapgw.StoreModifiedError
	if !errors.As(err, &modified) {
		t.Fatalf("StoreIMAPFlags error = %v, want StoreModifiedError", err)
	}
	if len(events.events) != 1 || events.events[0].UID != 12 {
		t.Fatalf("events = %#v, want only successful UID 12", events.events)
	}
}

func TestAppendIMAPMessageDelegatesToRepository(t *testing.T) {
	t.Parallel()

	events := &fakeIMAPEventPublisher{}
	repo := &fakeRepository{
		imapAppendTarget: maildb.IMAPAppendTarget{
			UserID:      "user-1",
			MailboxID:   "inbox",
			CompanyID:   "company-1",
			DomainID:    "domain-1",
			Address:     "user@example.com",
			UIDValidity: 12,
		},
		imapAppendResult: imapgw.AppendMessageResult{
			Summary:     imapgw.MessageSummary{ID: "msg-append-1", MailboxID: "inbox", UID: 44, SequenceNumber: 7},
			UIDValidity: 12,
		},
	}
	store := storage.NewLocalStore(t.TempDir())
	service := New(repo, store).WithIMAPMailboxEvents(events)

	got, err := service.AppendIMAPMessage(context.Background(), imapgw.AppendMessageRequest{
		UserID:    " user-1 ",
		MailboxID: "inbox",
		Size:      int64(len("Subject: hi\r\n\r\nhello")),
		Body:      strings.NewReader("Subject: hi\r\n\r\nhello"),
	})
	if err != nil {
		t.Fatalf("AppendIMAPMessage returned error: %v", err)
	}
	if got.Summary.UID != 44 || got.UIDValidity != 12 {
		t.Fatalf("append result = %#v, want repository result", got)
	}
	if repo.lastIMAPAppendUserID != "user-1" || repo.lastIMAPAppendMailboxID != "inbox" {
		t.Fatalf("append target lookup = %q/%q, want canonical ids", repo.lastIMAPAppendUserID, repo.lastIMAPAppendMailboxID)
	}
	if repo.lastIMAPAppendStored.Target.UserID != "user-1" || repo.lastIMAPAppendStored.Target.MailboxID != "inbox" || repo.lastIMAPAppendStored.Size != int64(len("Subject: hi\r\n\r\nhello")) {
		t.Fatalf("append stored request = %#v, want canonical target and size", repo.lastIMAPAppendStored)
	}
	if repo.lastIMAPAppendStored.Parsed.Subject != "hi" {
		t.Fatalf("append parsed subject = %q, want hi", repo.lastIMAPAppendStored.Parsed.Subject)
	}
	if !strings.HasPrefix(repo.lastIMAPAppendStored.StoragePath, "mailstore/company-1/domain-1/user-1/imap-append/") {
		t.Fatalf("append storage path = %q", repo.lastIMAPAppendStored.StoragePath)
	}
	if len(events.events) != 1 || events.events[0].Type != imapgw.MailboxEventExists || events.events[0].UserID != "user-1" || events.events[0].MailboxID != "inbox" || events.events[0].UID != 44 || events.events[0].Messages != 7 {
		t.Fatalf("events = %#v, want exists event", events.events)
	}
}

func TestAppendIMAPMessagePreservesMailboxIDSpacing(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{
		imapAppendTarget: maildb.IMAPAppendTarget{
			UserID:      "user-1",
			MailboxID:   " spaced-inbox ",
			CompanyID:   "company-1",
			DomainID:    "domain-1",
			Address:     "user@example.com",
			UIDValidity: 12,
		},
		imapAppendResult: imapgw.AppendMessageResult{
			Summary:     imapgw.MessageSummary{ID: "msg-append-1", MailboxID: " spaced-inbox ", UID: 44, SequenceNumber: 7},
			UIDValidity: 12,
		},
	}
	store := storage.NewLocalStore(t.TempDir())
	service := New(repo, store)
	appendBody := "Subject: hi\r\n\r\nhello"

	got, err := service.AppendIMAPMessage(context.Background(), imapgw.AppendMessageRequest{
		UserID:    " user-1 ",
		MailboxID: " INBOX ",
		Size:      int64(len(appendBody)),
		Body:      strings.NewReader(appendBody),
	})
	if err != nil {
		t.Fatalf("AppendIMAPMessage returned error: %v", err)
	}
	if got.Summary.MailboxID != " spaced-inbox " {
		t.Fatalf("append result = %#v, want spaced target result", got)
	}
	if repo.lastIMAPAppendUserID != "user-1" || repo.lastIMAPAppendMailboxID != " INBOX " {
		t.Fatalf("append target lookup = %q/%q, want user-1/spaced INBOX", repo.lastIMAPAppendUserID, repo.lastIMAPAppendMailboxID)
	}
	if repo.lastIMAPAppendStored.Target.MailboxID != " spaced-inbox " {
		t.Fatalf("append stored target = %#v, want spaced canonical mailbox id", repo.lastIMAPAppendStored.Target)
	}
}

func TestAppendIMAPMessageRejectsUnsafeIdentifiers(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name      string
		userID    imapgw.UserID
		mailboxID imapgw.MailboxID
	}{
		{name: "user crlf", userID: "user-1\r\nbad", mailboxID: "inbox"},
		{name: "user too long", userID: imapgw.UserID(strings.Repeat("u", maxServiceResourceIDBytes+1)), mailboxID: "inbox"},
		{name: "mailbox crlf", userID: "user-1", mailboxID: "inbox\nbad"},
		{name: "mailbox too long", userID: "user-1", mailboxID: imapgw.MailboxID(strings.Repeat("m", maxServiceResourceIDBytes+1))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			repo := &fakeRepository{}
			service := New(repo, storage.NewLocalStore(t.TempDir()))
			appendBody := "Subject: hi\r\n\r\nhello"

			_, err := service.AppendIMAPMessage(context.Background(), imapgw.AppendMessageRequest{
				UserID:    tc.userID,
				MailboxID: tc.mailboxID,
				Size:      int64(len(appendBody)),
				Body:      strings.NewReader(appendBody),
			})
			if err == nil {
				t.Fatal("AppendIMAPMessage accepted unsafe identifier")
			}
			if repo.lastIMAPAppendUserID != "" || repo.lastIMAPAppendMailboxID != "" {
				t.Fatalf("append target lookup = %q/%q, want no repository work", repo.lastIMAPAppendUserID, repo.lastIMAPAppendMailboxID)
			}
		})
	}
}

func TestAppendIMAPMessageRejectsLiteralSizeMismatch(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{
		imapAppendTarget: maildb.IMAPAppendTarget{
			UserID:    "user-1",
			MailboxID: "inbox",
			CompanyID: "company-1",
			DomainID:  "domain-1",
			Address:   "user@example.com",
		},
	}
	service := New(repo, storage.NewLocalStore(t.TempDir()))

	_, err := service.AppendIMAPMessage(context.Background(), imapgw.AppendMessageRequest{
		UserID:    "user-1",
		MailboxID: "inbox",
		Size:      5,
		Body:      strings.NewReader("Subject: hi\r\n\r\nhello"),
	})
	if err == nil || !strings.Contains(err.Error(), "append literal size mismatch") {
		t.Fatalf("AppendIMAPMessage error = %v, want size mismatch", err)
	}
	if repo.lastIMAPAppendStored.StoragePath != "" {
		t.Fatalf("append stored request = %#v, want no repository append after size mismatch", repo.lastIMAPAppendStored)
	}
}

func TestAppendIMAPMessageMapsMailboxFullToOverQuota(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{
		imapAppendTarget: maildb.IMAPAppendTarget{
			UserID:    "user-1",
			MailboxID: "inbox",
			CompanyID: "company-1",
			DomainID:  "domain-1",
			Address:   "user@example.com",
		},
		imapAppendStoredErr: mail.ErrMailboxFull,
	}
	service := New(repo, storage.NewLocalStore(t.TempDir()))
	appendBody := "Subject: hi\r\n\r\nhello"

	_, err := service.AppendIMAPMessage(context.Background(), imapgw.AppendMessageRequest{
		UserID:    "user-1",
		MailboxID: "inbox",
		Size:      int64(len(appendBody)),
		Body:      strings.NewReader(appendBody),
	})
	if !errors.Is(err, imapgw.ErrOverQuota) {
		t.Fatalf("AppendIMAPMessage error = %v, want ErrOverQuota", err)
	}
	if repo.lastIMAPAppendStored.StoragePath == "" {
		t.Fatal("append stored request did not reach repository")
	}
}

func TestMoveIMAPMessagesDelegatesToRepository(t *testing.T) {
	t.Parallel()

	events := &fakeIMAPEventPublisher{}
	repo := &fakeRepository{
		imapMoveResults: []imapgw.MoveMessageResult{{
			Source:              imapgw.MessageSummary{ID: "msg-1", MailboxID: "inbox", UID: 12, SequenceNumber: 1},
			Destination:         imapgw.MessageSummary{ID: "msg-1", MailboxID: "archive", UID: 33, SequenceNumber: 1},
			SourceHighestModSeq: 44,
		}},
	}
	service := New(repo, nil).WithIMAPMailboxEvents(events)

	got, err := service.MoveIMAPMessages(context.Background(), imapgw.MoveMessagesRequest{
		UserID:          " user-1 ",
		SourceMailboxID: "inbox",
		DestMailboxID:   "archive",
		UIDs:            []imapgw.UID{12},
	})
	if err != nil {
		t.Fatalf("MoveIMAPMessages returned error: %v", err)
	}
	if len(got) != 1 || got[0].Source.UID != 12 || got[0].Destination.UID != 33 || got[0].SourceHighestModSeq != 44 {
		t.Fatalf("move results = %#v, want repository result", got)
	}
	if repo.lastIMAPMoveUserID != "user-1" || repo.lastIMAPMoveSourceMailboxID != "inbox" || repo.lastIMAPMoveDestMailboxID != "archive" || !reflect.DeepEqual(repo.lastIMAPMoveUIDs, []imapgw.UID{12}) {
		t.Fatalf("move request = %q/%q/%q/%v", repo.lastIMAPMoveUserID, repo.lastIMAPMoveSourceMailboxID, repo.lastIMAPMoveDestMailboxID, repo.lastIMAPMoveUIDs)
	}
	if len(events.events) != 1 || events.events[0].Type != imapgw.MailboxEventExpunge || events.events[0].UserID != "user-1" || events.events[0].MailboxID != "inbox" || events.events[0].UID != 12 {
		t.Fatalf("events = %#v, want source expunge event", events.events)
	}
}

func TestExpungeIMAPMessagesDelegatesToRepository(t *testing.T) {
	t.Parallel()

	events := &fakeIMAPEventPublisher{}
	repo := &fakeRepository{
		imapExpungeSummaries: []imapgw.MessageSummary{{ID: "msg-1", MailboxID: "inbox", UID: 12, SequenceNumber: 1}},
	}
	service := New(repo, nil).WithIMAPMailboxEvents(events)

	got, err := service.ExpungeIMAPMessages(context.Background(), imapgw.ExpungeRequest{
		UserID:    " user-1 ",
		MailboxID: "inbox",
		UIDs:      []imapgw.UID{12},
	})
	if err != nil {
		t.Fatalf("ExpungeIMAPMessages returned error: %v", err)
	}
	if len(got) != 1 || got[0].UID != 12 {
		t.Fatalf("summaries = %#v, want repository result", got)
	}
	if repo.lastIMAPExpungeUserID != "user-1" || repo.lastIMAPExpungeMailboxID != "inbox" || !reflect.DeepEqual(repo.lastIMAPExpungeUIDs, []imapgw.UID{12}) {
		t.Fatalf("expunge request = %q/%q/%v", repo.lastIMAPExpungeUserID, repo.lastIMAPExpungeMailboxID, repo.lastIMAPExpungeUIDs)
	}
	if len(events.events) != 1 || events.events[0].Type != imapgw.MailboxEventExpunge || events.events[0].UserID != "user-1" || events.events[0].MailboxID != "inbox" || events.events[0].UID != 12 {
		t.Fatalf("events = %#v, want expunge event", events.events)
	}
}

func TestSetMessageFlagPublishesIMAPFlagEvent(t *testing.T) {
	t.Parallel()

	events := &fakeIMAPEventPublisher{}
	repo := &fakeRepository{
		imapUIDs: []maildb.IMAPMessageUID{{MessageID: "msg-1", MailboxID: "inbox", UID: 12, SequenceNumber: 3, ModSeq: 2}},
	}
	service := New(repo, nil).WithIMAPMailboxEvents(events)

	if err := service.SetMessageFlag(context.Background(), " user-1 ", " msg-1 ", " read ", true); err != nil {
		t.Fatalf("SetMessageFlag returned error: %v", err)
	}
	if repo.lastMutationUserID != "user-1" || repo.lastFlagMessageID != "msg-1" || repo.lastFlag != "read" {
		t.Fatalf("flag mutation = %q/%q/%q", repo.lastMutationUserID, repo.lastFlagMessageID, repo.lastFlag)
	}
	if repo.lastIMAPUIDLookupUserID != "user-1" || len(repo.lastIMAPUIDLookupMessageIDs) != 1 || repo.lastIMAPUIDLookupMessageIDs[0] != "msg-1" {
		t.Fatalf("imap uid lookup = %q/%#v", repo.lastIMAPUIDLookupUserID, repo.lastIMAPUIDLookupMessageIDs)
	}
	if len(events.events) != 1 || events.events[0].Type != imapgw.MailboxEventFlags || events.events[0].MailboxID != "inbox" || events.events[0].UID != 12 {
		t.Fatalf("events = %#v, want flags event", events.events)
	}
}

func TestSetMessageFlagIgnoresIMAPEventPublishError(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{
		imapUIDs: []maildb.IMAPMessageUID{{MessageID: "msg-1", MailboxID: "inbox", UID: 12, ModSeq: 2}},
	}
	service := New(repo, nil).WithIMAPMailboxEvents(&fakeIMAPEventPublisher{err: errors.New("subscriber closed")})

	if err := service.SetMessageFlag(context.Background(), "user-1", "msg-1", "read", true); err != nil {
		t.Fatalf("SetMessageFlag returned error: %v", err)
	}
}

func TestBulkSetMessageFlagPublishesIMAPFlagEvents(t *testing.T) {
	t.Parallel()

	events := &fakeIMAPEventPublisher{}
	repo := &fakeRepository{
		imapUIDs: []maildb.IMAPMessageUID{
			{MessageID: "msg-1", MailboxID: "inbox", UID: 12, ModSeq: 2},
			{MessageID: "msg-2", MailboxID: "inbox", UID: 13, ModSeq: 3},
		},
	}
	service := New(repo, nil).WithIMAPMailboxEvents(events)

	updated, err := service.BulkSetMessageFlag(context.Background(), maildb.BulkMessageFlagRequest{
		UserID:     " user-1 ",
		MessageIDs: []string{" msg-1 ", " msg-2 "},
		Flag:       " read ",
		Value:      true,
	})
	if err != nil {
		t.Fatalf("BulkSetMessageFlag returned error: %v", err)
	}
	if updated != 2 {
		t.Fatalf("updated = %d, want 2", updated)
	}
	if repo.lastBulkFlag.UserID != "user-1" || repo.lastBulkFlag.Flag != "read" || len(repo.lastBulkFlag.MessageIDs) != 2 || repo.lastBulkFlag.MessageIDs[0] != "msg-1" || repo.lastBulkFlag.MessageIDs[1] != "msg-2" {
		t.Fatalf("bulk flag request = %#v", repo.lastBulkFlag)
	}
	if repo.lastIMAPUIDLookupUserID != "user-1" || len(repo.lastIMAPUIDLookupMessageIDs) != 2 || repo.lastIMAPUIDLookupMessageIDs[0] != "msg-1" || repo.lastIMAPUIDLookupMessageIDs[1] != "msg-2" {
		t.Fatalf("imap uid lookup = %q/%#v", repo.lastIMAPUIDLookupUserID, repo.lastIMAPUIDLookupMessageIDs)
	}
	if len(events.events) != 2 || events.events[0].UID != 12 || events.events[1].UID != 13 {
		t.Fatalf("events = %#v, want two flags events", events.events)
	}
}

func TestBulkSetThreadFlagPublishesIMAPFlagEvents(t *testing.T) {
	t.Parallel()

	events := &fakeIMAPEventPublisher{}
	repo := &fakeRepository{
		bulkThreadFlagResult: maildb.BulkThreadFlagResult{
			Updated:    2,
			MessageIDs: []string{"msg-1", "msg-2"},
		},
		imapUIDs: []maildb.IMAPMessageUID{
			{MessageID: "msg-1", MailboxID: "inbox", UID: 12, ModSeq: 2},
			{MessageID: "msg-2", MailboxID: "inbox", UID: 13, ModSeq: 3},
		},
	}
	service := New(repo, nil).WithIMAPMailboxEvents(events)

	updated, err := service.BulkSetThreadFlag(context.Background(), maildb.BulkThreadFlagRequest{
		UserID:    " user-1 ",
		ThreadIDs: []string{" thread-1 ", " thread-2 "},
		Flag:      " read ",
		Value:     true,
	})
	if err != nil {
		t.Fatalf("BulkSetThreadFlag returned error: %v", err)
	}
	if updated != 2 {
		t.Fatalf("updated = %d, want 2", updated)
	}
	if repo.lastBulkThreadFlag.UserID != "user-1" || repo.lastBulkThreadFlag.Flag != "read" || len(repo.lastBulkThreadFlag.ThreadIDs) != 2 || repo.lastBulkThreadFlag.ThreadIDs[0] != "thread-1" || repo.lastBulkThreadFlag.ThreadIDs[1] != "thread-2" {
		t.Fatalf("bulk thread flag request = %#v", repo.lastBulkThreadFlag)
	}
	if repo.lastIMAPUIDLookupUserID != "user-1" || len(repo.lastIMAPUIDLookupMessageIDs) != 2 || repo.lastIMAPUIDLookupMessageIDs[0] != "msg-1" || repo.lastIMAPUIDLookupMessageIDs[1] != "msg-2" {
		t.Fatalf("imap uid lookup = %q/%#v", repo.lastIMAPUIDLookupUserID, repo.lastIMAPUIDLookupMessageIDs)
	}
	if len(events.events) != 2 || events.events[0].UID != 12 || events.events[1].UID != 13 {
		t.Fatalf("events = %#v, want two flags events", events.events)
	}
}

func TestMoveMessagePublishesIMAPExpungeEvent(t *testing.T) {
	t.Parallel()

	events := &fakeIMAPEventPublisher{}
	repo := &fakeRepository{
		imapUIDs: []maildb.IMAPMessageUID{{MessageID: "msg-1", MailboxID: "inbox", UID: 12, SequenceNumber: 3, ModSeq: 2}},
	}
	service := New(repo, nil).WithIMAPMailboxEvents(events)

	if err := service.MoveMessage(context.Background(), " user-1 ", " msg-1 ", " archive "); err != nil {
		t.Fatalf("MoveMessage returned error: %v", err)
	}
	if repo.lastMutationUserID != "user-1" || repo.lastMoveMessageID != "msg-1" || repo.lastMoveFolderID != "archive" {
		t.Fatalf("move mutation = %q/%q/%q", repo.lastMutationUserID, repo.lastMoveMessageID, repo.lastMoveFolderID)
	}
	if repo.lastIMAPUIDLookupUserID != "user-1" || len(repo.lastIMAPUIDLookupMessageIDs) != 1 || repo.lastIMAPUIDLookupMessageIDs[0] != "msg-1" {
		t.Fatalf("imap uid lookup = %q/%#v", repo.lastIMAPUIDLookupUserID, repo.lastIMAPUIDLookupMessageIDs)
	}
	if len(events.events) != 1 || events.events[0].Type != imapgw.MailboxEventExpunge || events.events[0].MailboxID != "inbox" || events.events[0].UID != 12 || events.events[0].SequenceNumber != 3 {
		t.Fatalf("events = %#v, want expunge event", events.events)
	}
}

func TestMoveMessageRejectsUnsafeIDsBeforeIMAPLookup(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{}
	service := New(repo, nil)

	if err := service.MoveMessage(context.Background(), "user-1", "msg-1", "archive\r\nbad"); err == nil {
		t.Fatal("MoveMessage accepted newline-bearing folder ID")
	}
	if repo.lastIMAPUIDLookupUserID != "" || repo.lastMoveFolderID != "" {
		t.Fatalf("repository was called before validation: uid=%q folder=%q", repo.lastIMAPUIDLookupUserID, repo.lastMoveFolderID)
	}
}

func TestBulkMoveMessagesPublishesIMAPExpungeEvents(t *testing.T) {
	t.Parallel()

	events := &fakeIMAPEventPublisher{}
	repo := &fakeRepository{
		imapUIDs: []maildb.IMAPMessageUID{
			{MessageID: "msg-1", MailboxID: "inbox", UID: 12, SequenceNumber: 3, ModSeq: 2},
			{MessageID: "msg-2", MailboxID: "inbox", UID: 13, SequenceNumber: 4, ModSeq: 3},
		},
	}
	service := New(repo, nil).WithIMAPMailboxEvents(events)

	updated, err := service.BulkMoveMessages(context.Background(), maildb.BulkMessageMoveRequest{
		UserID:     " user-1 ",
		MessageIDs: []string{" msg-1 ", " msg-2 "},
		FolderID:   " archive ",
	})
	if err != nil {
		t.Fatalf("BulkMoveMessages returned error: %v", err)
	}
	if updated != 2 {
		t.Fatalf("updated = %d, want 2", updated)
	}
	if repo.lastBulkMove.UserID != "user-1" || repo.lastBulkMove.FolderID != "archive" || len(repo.lastBulkMove.MessageIDs) != 2 || repo.lastBulkMove.MessageIDs[0] != "msg-1" || repo.lastBulkMove.MessageIDs[1] != "msg-2" {
		t.Fatalf("bulk move request = %#v", repo.lastBulkMove)
	}
	if repo.lastIMAPUIDLookupUserID != "user-1" || len(repo.lastIMAPUIDLookupMessageIDs) != 2 || repo.lastIMAPUIDLookupMessageIDs[0] != "msg-1" || repo.lastIMAPUIDLookupMessageIDs[1] != "msg-2" {
		t.Fatalf("imap uid lookup = %q/%#v", repo.lastIMAPUIDLookupUserID, repo.lastIMAPUIDLookupMessageIDs)
	}
	if len(events.events) != 2 || events.events[0].Type != imapgw.MailboxEventExpunge || events.events[0].UID != 12 || events.events[0].SequenceNumber != 3 || events.events[1].UID != 13 || events.events[1].SequenceNumber != 4 {
		t.Fatalf("events = %#v, want two expunge events", events.events)
	}
}

func TestDeleteMessagePublishesIMAPExpungeEvent(t *testing.T) {
	t.Parallel()

	events := &fakeIMAPEventPublisher{}
	repo := &fakeRepository{
		imapUIDs: []maildb.IMAPMessageUID{{MessageID: "msg-1", MailboxID: "inbox", UID: 12, SequenceNumber: 3, ModSeq: 2}},
	}
	service := New(repo, nil).WithIMAPMailboxEvents(events)

	if err := service.DeleteMessage(context.Background(), " user-1 ", " msg-1 "); err != nil {
		t.Fatalf("DeleteMessage returned error: %v", err)
	}
	if repo.lastMutationUserID != "user-1" || repo.lastDeleteMessageID != "msg-1" {
		t.Fatalf("delete mutation = %q/%q", repo.lastMutationUserID, repo.lastDeleteMessageID)
	}
	if len(events.events) != 1 || events.events[0].Type != imapgw.MailboxEventExpunge || events.events[0].MailboxID != "inbox" || events.events[0].UID != 12 || events.events[0].SequenceNumber != 3 {
		t.Fatalf("events = %#v, want expunge event", events.events)
	}
}

func TestRestoreMessageDelegatesToRepository(t *testing.T) {
	t.Parallel()

	events := &fakeIMAPEventPublisher{}
	repo := &fakeRepository{
		ensuredIMAPUIDs: []maildb.IMAPMessageUID{{MessageID: "msg-1", MailboxID: "inbox", UID: 44, SequenceNumber: 3, ModSeq: 9}},
	}
	service := New(repo, nil).WithIMAPMailboxEvents(events)

	if err := service.RestoreMessage(context.Background(), " user-1 ", " msg-1 "); err != nil {
		t.Fatalf("RestoreMessage returned error: %v", err)
	}
	if repo.lastMutationUserID != "user-1" || repo.lastRestoreMessageID != "msg-1" {
		t.Fatalf("restore mutation = %q/%q", repo.lastMutationUserID, repo.lastRestoreMessageID)
	}
	if repo.lastEnsureIMAPUIDUserID != "user-1" || len(repo.lastEnsureIMAPUIDMessageIDs) != 1 || repo.lastEnsureIMAPUIDMessageIDs[0] != "msg-1" {
		t.Fatalf("ensure imap uid request = %q/%#v", repo.lastEnsureIMAPUIDUserID, repo.lastEnsureIMAPUIDMessageIDs)
	}
	if len(events.events) != 1 || events.events[0].Type != imapgw.MailboxEventExists || events.events[0].UserID != "user-1" || events.events[0].MailboxID != "inbox" || events.events[0].UID != 44 || events.events[0].Messages != 3 {
		t.Fatalf("events = %#v, want exists event", events.events)
	}
}

func TestBulkRestoreMessagesNormalizesRequest(t *testing.T) {
	t.Parallel()

	events := &fakeIMAPEventPublisher{}
	repo := &fakeRepository{
		bulkRestoreResult: maildb.BulkMessageRestoreResult{Updated: 2, MessageIDs: []string{"msg-1", "msg-2"}},
		ensuredIMAPUIDs: []maildb.IMAPMessageUID{
			{MessageID: "msg-1", MailboxID: "inbox", UID: 44, SequenceNumber: 3, ModSeq: 9},
			{MessageID: "msg-2", MailboxID: "inbox", UID: 45, SequenceNumber: 4, ModSeq: 10},
		},
	}
	service := New(repo, nil).WithIMAPMailboxEvents(events)

	updated, err := service.BulkRestoreMessages(context.Background(), maildb.BulkMessageRestoreRequest{
		UserID:     " user-1 ",
		MessageIDs: []string{" msg-1 ", " msg-2 "},
	})
	if err != nil {
		t.Fatalf("BulkRestoreMessages returned error: %v", err)
	}
	if updated != 2 {
		t.Fatalf("updated = %d, want 2", updated)
	}
	if repo.lastBulkRestore.UserID != "user-1" || len(repo.lastBulkRestore.MessageIDs) != 2 || repo.lastBulkRestore.MessageIDs[0] != "msg-1" || repo.lastBulkRestore.MessageIDs[1] != "msg-2" {
		t.Fatalf("bulk restore request = %#v", repo.lastBulkRestore)
	}
	if repo.lastEnsureIMAPUIDUserID != "user-1" || len(repo.lastEnsureIMAPUIDMessageIDs) != 2 || repo.lastEnsureIMAPUIDMessageIDs[0] != "msg-1" || repo.lastEnsureIMAPUIDMessageIDs[1] != "msg-2" {
		t.Fatalf("ensure imap uid request = %q/%#v", repo.lastEnsureIMAPUIDUserID, repo.lastEnsureIMAPUIDMessageIDs)
	}
	if len(events.events) != 1 || events.events[0].Type != imapgw.MailboxEventExists || events.events[0].UID != 45 || events.events[0].Messages != 4 {
		t.Fatalf("events = %#v, want coalesced exists event", events.events)
	}
}

func TestBulkRestoreMessagesCoalescesExistsEventsByMailbox(t *testing.T) {
	t.Parallel()

	events := &fakeIMAPEventPublisher{}
	repo := &fakeRepository{
		bulkRestoreResult: maildb.BulkMessageRestoreResult{Updated: 3, MessageIDs: []string{"msg-1", "msg-2", "msg-3"}},
		ensuredIMAPUIDs: []maildb.IMAPMessageUID{
			{MessageID: "msg-1", MailboxID: "inbox", UID: 44, SequenceNumber: 3, ModSeq: 9},
			{MessageID: "msg-2", MailboxID: "archive", UID: 9, SequenceNumber: 2, ModSeq: 4},
			{MessageID: "msg-3", MailboxID: "inbox", UID: 45, SequenceNumber: 4, ModSeq: 10},
		},
	}
	service := New(repo, nil).WithIMAPMailboxEvents(events)

	updated, err := service.BulkRestoreMessages(context.Background(), maildb.BulkMessageRestoreRequest{
		UserID:     "user-1",
		MessageIDs: []string{"msg-1", "msg-2", "msg-3"},
	})
	if err != nil {
		t.Fatalf("BulkRestoreMessages returned error: %v", err)
	}
	if updated != 3 {
		t.Fatalf("updated = %d, want 3", updated)
	}
	if len(events.events) != 2 {
		t.Fatalf("events = %#v, want one exists event per mailbox", events.events)
	}
	if events.events[0].Type != imapgw.MailboxEventExists || events.events[0].MailboxID != "inbox" || events.events[0].UID != 45 || events.events[0].Messages != 4 {
		t.Fatalf("first event = %#v, want coalesced inbox exists event", events.events[0])
	}
	if events.events[1].Type != imapgw.MailboxEventExists || events.events[1].MailboxID != "archive" || events.events[1].UID != 9 || events.events[1].Messages != 2 {
		t.Fatalf("second event = %#v, want archive exists event", events.events[1])
	}
}

func TestBulkRestoreMessagesSkipsEmptyMailboxExistsEvents(t *testing.T) {
	t.Parallel()

	events := &fakeIMAPEventPublisher{}
	repo := &fakeRepository{
		bulkRestoreResult: maildb.BulkMessageRestoreResult{Updated: 2, MessageIDs: []string{"msg-1", "msg-2"}},
		ensuredIMAPUIDs: []maildb.IMAPMessageUID{
			{MessageID: "msg-1", UID: 44, SequenceNumber: 3, ModSeq: 9},
			{MessageID: "msg-2", MailboxID: "inbox", UID: 45, SequenceNumber: 4, ModSeq: 10},
		},
	}
	service := New(repo, nil).WithIMAPMailboxEvents(events)

	updated, err := service.BulkRestoreMessages(context.Background(), maildb.BulkMessageRestoreRequest{
		UserID:     "user-1",
		MessageIDs: []string{"msg-1", "msg-2"},
	})
	if err != nil {
		t.Fatalf("BulkRestoreMessages returned error: %v", err)
	}
	if updated != 2 {
		t.Fatalf("updated = %d, want 2", updated)
	}
	if len(events.events) != 1 || events.events[0].Type != imapgw.MailboxEventExists || events.events[0].MailboxID != "inbox" || events.events[0].UID != 45 || events.events[0].Messages != 4 {
		t.Fatalf("events = %#v, want only valid mailbox exists event", events.events)
	}
}

func TestBulkRestoreThreadsNormalizesRequest(t *testing.T) {
	t.Parallel()

	events := &fakeIMAPEventPublisher{}
	repo := &fakeRepository{
		bulkThreadRestoreResult: maildb.BulkThreadRestoreResult{Updated: 2, MessageIDs: []string{"msg-1", "msg-2"}},
		ensuredIMAPUIDs: []maildb.IMAPMessageUID{
			{MessageID: "msg-1", MailboxID: "inbox", UID: 44, SequenceNumber: 3, ModSeq: 9},
			{MessageID: "msg-2", MailboxID: "inbox", UID: 45, SequenceNumber: 4, ModSeq: 10},
		},
	}
	service := New(repo, nil).WithIMAPMailboxEvents(events)

	updated, err := service.BulkRestoreThreads(context.Background(), maildb.BulkThreadRestoreRequest{
		UserID:    " user-1 ",
		ThreadIDs: []string{" thread-1 ", " thread-2 "},
	})
	if err != nil {
		t.Fatalf("BulkRestoreThreads returned error: %v", err)
	}
	if updated != 2 {
		t.Fatalf("updated = %d, want 2", updated)
	}
	if repo.lastBulkThreadRestore.UserID != "user-1" || len(repo.lastBulkThreadRestore.ThreadIDs) != 2 || repo.lastBulkThreadRestore.ThreadIDs[0] != "thread-1" || repo.lastBulkThreadRestore.ThreadIDs[1] != "thread-2" {
		t.Fatalf("bulk thread restore request = %#v", repo.lastBulkThreadRestore)
	}
	if repo.lastEnsureIMAPUIDUserID != "user-1" || len(repo.lastEnsureIMAPUIDMessageIDs) != 2 || repo.lastEnsureIMAPUIDMessageIDs[0] != "msg-1" || repo.lastEnsureIMAPUIDMessageIDs[1] != "msg-2" {
		t.Fatalf("ensure imap uid request = %q/%#v", repo.lastEnsureIMAPUIDUserID, repo.lastEnsureIMAPUIDMessageIDs)
	}
	if len(events.events) != 1 || events.events[0].Type != imapgw.MailboxEventExists || events.events[0].UID != 45 || events.events[0].Messages != 4 {
		t.Fatalf("events = %#v, want coalesced exists event", events.events)
	}
}

func TestBulkMoveThreadsPublishesIMAPExpungeEvents(t *testing.T) {
	t.Parallel()

	events := &fakeIMAPEventPublisher{}
	repo := &fakeRepository{
		threadMessageIDs: []string{"msg-1", "msg-2"},
		bulkThreadMoveResult: maildb.BulkThreadMoveResult{
			Updated:    2,
			MessageIDs: []string{"msg-1", "msg-2"},
		},
		imapUIDs: []maildb.IMAPMessageUID{
			{MessageID: "msg-1", MailboxID: "inbox", UID: 12, SequenceNumber: 1, ModSeq: 2},
			{MessageID: "msg-2", MailboxID: "inbox", UID: 13, SequenceNumber: 2, ModSeq: 3},
		},
	}
	service := New(repo, nil).WithIMAPMailboxEvents(events)

	updated, err := service.BulkMoveThreads(context.Background(), maildb.BulkThreadMoveRequest{
		UserID:    " user-1 ",
		ThreadIDs: []string{" thread-1 ", " thread-2 "},
		FolderID:  " archive ",
	})
	if err != nil {
		t.Fatalf("BulkMoveThreads returned error: %v", err)
	}
	if updated != 2 {
		t.Fatalf("updated = %d, want 2", updated)
	}
	if repo.lastListThreadMessageUserID != "user-1" || len(repo.lastListThreadMessageThreadIDs) != 2 || repo.lastListThreadMessageThreadIDs[0] != "thread-1" || repo.lastListThreadMessageThreadIDs[1] != "thread-2" {
		t.Fatalf("thread message id lookup = %q/%#v", repo.lastListThreadMessageUserID, repo.lastListThreadMessageThreadIDs)
	}
	if repo.lastBulkThreadMove.UserID != "user-1" || repo.lastBulkThreadMove.FolderID != "archive" || len(repo.lastBulkThreadMove.ThreadIDs) != 2 {
		t.Fatalf("bulk thread move request = %#v", repo.lastBulkThreadMove)
	}
	if repo.lastIMAPUIDLookupUserID != "user-1" || len(repo.lastIMAPUIDLookupMessageIDs) != 2 || repo.lastIMAPUIDLookupMessageIDs[0] != "msg-1" || repo.lastIMAPUIDLookupMessageIDs[1] != "msg-2" {
		t.Fatalf("imap uid lookup = %q/%#v", repo.lastIMAPUIDLookupUserID, repo.lastIMAPUIDLookupMessageIDs)
	}
	if len(events.events) != 2 || events.events[0].Type != imapgw.MailboxEventExpunge || events.events[0].UID != 12 || events.events[1].UID != 13 {
		t.Fatalf("events = %#v, want two expunge events", events.events)
	}
}

func TestBulkDeleteMessagesPublishesIMAPExpungeEvents(t *testing.T) {
	t.Parallel()

	events := &fakeIMAPEventPublisher{}
	repo := &fakeRepository{
		imapUIDs: []maildb.IMAPMessageUID{
			{MessageID: "msg-1", MailboxID: "inbox", UID: 12, SequenceNumber: 3, ModSeq: 2},
			{MessageID: "msg-2", MailboxID: "inbox", UID: 13, SequenceNumber: 4, ModSeq: 3},
		},
	}
	service := New(repo, nil).WithIMAPMailboxEvents(events)

	updated, err := service.BulkDeleteMessages(context.Background(), maildb.BulkMessageDeleteRequest{
		UserID:     " user-1 ",
		MessageIDs: []string{" msg-1 ", " msg-2 "},
	})
	if err != nil {
		t.Fatalf("BulkDeleteMessages returned error: %v", err)
	}
	if updated != 2 {
		t.Fatalf("updated = %d, want 2", updated)
	}
	if repo.lastBulkDelete.UserID != "user-1" || len(repo.lastBulkDelete.MessageIDs) != 2 || repo.lastBulkDelete.MessageIDs[0] != "msg-1" || repo.lastBulkDelete.MessageIDs[1] != "msg-2" {
		t.Fatalf("bulk delete request = %#v", repo.lastBulkDelete)
	}
	if repo.lastIMAPUIDLookupUserID != "user-1" || len(repo.lastIMAPUIDLookupMessageIDs) != 2 || repo.lastIMAPUIDLookupMessageIDs[0] != "msg-1" || repo.lastIMAPUIDLookupMessageIDs[1] != "msg-2" {
		t.Fatalf("imap uid lookup = %q/%#v", repo.lastIMAPUIDLookupUserID, repo.lastIMAPUIDLookupMessageIDs)
	}
	if len(events.events) != 2 || events.events[0].Type != imapgw.MailboxEventExpunge || events.events[0].UID != 12 || events.events[0].SequenceNumber != 3 || events.events[1].UID != 13 || events.events[1].SequenceNumber != 4 {
		t.Fatalf("events = %#v, want two expunge events", events.events)
	}
}

func TestBulkDeleteThreadsPublishesIMAPExpungeEvents(t *testing.T) {
	t.Parallel()

	events := &fakeIMAPEventPublisher{}
	repo := &fakeRepository{
		threadMessageIDs: []string{"msg-1", "msg-2"},
		bulkThreadDeleteResult: maildb.BulkThreadDeleteResult{
			Updated:    2,
			MessageIDs: []string{"msg-1", "msg-2"},
		},
		imapUIDs: []maildb.IMAPMessageUID{
			{MessageID: "msg-1", MailboxID: "inbox", UID: 12, SequenceNumber: 3, ModSeq: 2},
			{MessageID: "msg-2", MailboxID: "inbox", UID: 13, SequenceNumber: 4, ModSeq: 3},
		},
	}
	service := New(repo, nil).WithIMAPMailboxEvents(events)

	updated, err := service.BulkDeleteThreads(context.Background(), maildb.BulkThreadDeleteRequest{
		UserID:    " user-1 ",
		ThreadIDs: []string{" thread-1 ", " thread-2 "},
	})
	if err != nil {
		t.Fatalf("BulkDeleteThreads returned error: %v", err)
	}
	if updated != 2 {
		t.Fatalf("updated = %d, want 2", updated)
	}
	if repo.lastListThreadMessageUserID != "user-1" || len(repo.lastListThreadMessageThreadIDs) != 2 || repo.lastListThreadMessageThreadIDs[0] != "thread-1" || repo.lastListThreadMessageThreadIDs[1] != "thread-2" {
		t.Fatalf("thread message id lookup = %q/%#v", repo.lastListThreadMessageUserID, repo.lastListThreadMessageThreadIDs)
	}
	if repo.lastBulkThreadDelete.UserID != "user-1" || len(repo.lastBulkThreadDelete.ThreadIDs) != 2 || repo.lastBulkThreadDelete.ThreadIDs[0] != "thread-1" || repo.lastBulkThreadDelete.ThreadIDs[1] != "thread-2" {
		t.Fatalf("bulk thread delete request = %#v", repo.lastBulkThreadDelete)
	}
	if repo.lastIMAPUIDLookupUserID != "user-1" || len(repo.lastIMAPUIDLookupMessageIDs) != 2 || repo.lastIMAPUIDLookupMessageIDs[0] != "msg-1" || repo.lastIMAPUIDLookupMessageIDs[1] != "msg-2" {
		t.Fatalf("imap uid lookup = %q/%#v", repo.lastIMAPUIDLookupUserID, repo.lastIMAPUIDLookupMessageIDs)
	}
	if len(events.events) != 2 || events.events[0].Type != imapgw.MailboxEventExpunge || events.events[0].UID != 12 || events.events[0].SequenceNumber != 3 || events.events[1].UID != 13 || events.events[1].SequenceNumber != 4 {
		t.Fatalf("events = %#v, want two expunge events", events.events)
	}
}

func TestSearchMessagesUsesExternalRelevanceSearchAndHydrates(t *testing.T) {
	t.Parallel()

	rank := 1.25
	repo := &fakeRepository{
		messagesByID: []maildb.MessageSummary{{ID: "msg-1", Subject: "hello"}},
	}
	source := &fakeSearchIDSource{
		hits: []searchindex.OpenSearchHit{{
			MessageID: "msg-1",
			Score:     rank,
			Highlights: searchindex.OpenSearchHighlights{
				Subject: []string{"<mark>hello</mark>"},
			},
		}},
	}
	service := New(repo, nil).WithSearchIDSource(source)

	got, err := service.SearchMessages(context.Background(), maildb.MessageSearchQuery{
		UserID:            " user-1 ",
		FolderID:          " folder-1 ",
		Query:             " hello ",
		From:              " sender@example.net ",
		Subject:           " greeting ",
		Limit:             10,
		Sort:              " relevance ",
		IncludeRank:       true,
		IncludeHighlights: true,
	})
	if err != nil {
		t.Fatalf("SearchMessages returned error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "msg-1" {
		t.Fatalf("messages = %#v", got)
	}
	if got[0].SearchRank == nil || *got[0].SearchRank != rank {
		t.Fatalf("SearchRank = %#v, want %v", got[0].SearchRank, rank)
	}
	if got[0].SearchHighlights == nil || len(got[0].SearchHighlights.Subject) != 1 {
		t.Fatalf("SearchHighlights = %#v", got[0].SearchHighlights)
	}
	if len(repo.lastHydrateIDs) != 1 || repo.lastHydrateIDs[0] != "msg-1" {
		t.Fatalf("hydrated ids = %#v", repo.lastHydrateIDs)
	}
	if source.lastQuery.FolderID != "folder-1" {
		t.Fatalf("external folder_id = %q", source.lastQuery.FolderID)
	}
	if source.lastQuery.UserID != "user-1" || source.lastQuery.Query != "hello" || source.lastQuery.From != "sender@example.net" || source.lastQuery.Subject != "greeting" {
		t.Fatalf("external query = %#v", source.lastQuery)
	}
}

func TestSearchMessagesDeduplicatesExternalHitsBeforeHydration(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{
		messagesByID: []maildb.MessageSummary{{ID: "msg-1", Subject: "hello"}},
	}
	service := New(repo, nil).WithSearchIDSource(&fakeSearchIDSource{
		hits: []searchindex.OpenSearchHit{
			{
				MessageID: "msg-1",
				Score:     2,
				Highlights: searchindex.OpenSearchHighlights{
					Subject: []string{"<mark>first</mark>"},
				},
			},
			{
				MessageID: " msg-1 ",
				Score:     1,
				Highlights: searchindex.OpenSearchHighlights{
					Subject: []string{"<mark>second</mark>"},
				},
			},
		},
	})

	got, err := service.SearchMessages(context.Background(), maildb.MessageSearchQuery{
		UserID:            "user-1",
		Query:             "hello",
		Sort:              maildb.MessageSearchSortRelevance,
		IncludeRank:       true,
		IncludeHighlights: true,
	})
	if err != nil {
		t.Fatalf("SearchMessages returned error: %v", err)
	}
	if len(repo.lastHydrateIDs) != 1 || repo.lastHydrateIDs[0] != "msg-1" {
		t.Fatalf("hydrated ids = %#v, want single msg-1", repo.lastHydrateIDs)
	}
	if got[0].SearchRank == nil || *got[0].SearchRank != 2 {
		t.Fatalf("SearchRank = %#v, want first hit score", got[0].SearchRank)
	}
	if got[0].SearchHighlights == nil || got[0].SearchHighlights.Subject[0] != "<mark>first</mark>" {
		t.Fatalf("SearchHighlights = %#v, want first hit highlights", got[0].SearchHighlights)
	}
}

func TestSearchMessagesFallsBackWhenExternalSearchCannotPreserveContract(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{list: []maildb.MessageSummary{{ID: "pg-1"}}}
	service := New(repo, nil).WithSearchIDSource(&fakeSearchIDSource{
		hits: []searchindex.OpenSearchHit{{MessageID: "os-1", Score: 1}},
	})

	got, err := service.SearchMessages(context.Background(), maildb.MessageSearchQuery{
		UserID: "user-1",
		Query:  "hello",
		Sort:   maildb.MessageSearchSortDate,
	})
	if err != nil {
		t.Fatalf("SearchMessages returned error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "pg-1" {
		t.Fatalf("messages = %#v, want postgres fallback", got)
	}
}

func TestSearchMessagesNormalizesPostgresFallbackQuery(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{list: []maildb.MessageSummary{{ID: "pg-1"}}}
	service := New(repo, nil)

	got, err := service.SearchMessages(context.Background(), maildb.MessageSearchQuery{
		UserID:   " user-1 ",
		FolderID: " inbox ",
		Query:    " invoice ",
		From:     " sender@example.net ",
		Subject:  " receipt ",
		Sort:     " date ",
	})
	if err != nil {
		t.Fatalf("SearchMessages returned error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "pg-1" {
		t.Fatalf("messages = %#v", got)
	}
	if repo.lastSearchQuery.UserID != "user-1" || repo.lastSearchQuery.FolderID != "inbox" || repo.lastSearchQuery.Query != "invoice" || repo.lastSearchQuery.From != "sender@example.net" || repo.lastSearchQuery.Subject != "receipt" || repo.lastSearchQuery.Sort != maildb.MessageSearchSortDate {
		t.Fatalf("postgres query = %#v", repo.lastSearchQuery)
	}
}

func TestSearchMessagesRejectsUnsafeQueryFields(t *testing.T) {
	t.Parallel()

	tests := []maildb.MessageSearchQuery{
		{UserID: "user-1", FolderID: "folder-1\r\nbad", Query: "hello"},
		{UserID: "user-1", Query: "hello\nbad"},
		{UserID: "user-1", From: strings.Repeat("x", maxSearchFilterBytes+1)},
		{UserID: "user-1", Subject: strings.Repeat("x", maxSearchFilterBytes+1)},
	}
	for _, query := range tests {
		repo := &fakeRepository{}
		service := New(repo, nil)
		if _, err := service.SearchMessages(context.Background(), query); err == nil {
			t.Fatalf("SearchMessages accepted unsafe query %#v", query)
		}
		if repo.lastSearchQuery.UserID != "" {
			t.Fatalf("repository was called with query %#v", repo.lastSearchQuery)
		}
	}
}

func TestSearchDraftsNormalizesAndDelegates(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{draftSearchResults: []maildb.MessageDetail{{ID: "draft-1", Subject: "invoice draft"}}}
	service := New(repo, nil)

	got, err := service.SearchDrafts(context.Background(), maildb.DraftSearchQuery{
		UserID:  " user-1 ",
		Query:   " invoice ",
		From:    " sender@example.net ",
		Subject: " receipt ",
		Limit:   250,
	})
	if err != nil {
		t.Fatalf("SearchDrafts returned error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "draft-1" {
		t.Fatalf("drafts = %+v", got)
	}
	if repo.lastDraftSearchQuery.UserID != "user-1" || repo.lastDraftSearchQuery.Query != "invoice" || repo.lastDraftSearchQuery.From != "sender@example.net" || repo.lastDraftSearchQuery.Subject != "receipt" {
		t.Fatalf("draft search query = %#v", repo.lastDraftSearchQuery)
	}
	if repo.lastDraftSearchQuery.Limit != maildb.MessageListMaxLimit {
		t.Fatalf("draft search limit = %d", repo.lastDraftSearchQuery.Limit)
	}
}

func TestSearchDraftsRejectsUnsafeQueryFields(t *testing.T) {
	t.Parallel()

	tests := []maildb.DraftSearchQuery{
		{UserID: "user-1", Query: "hello\nbad"},
		{UserID: "user-1", From: "sender\rbad"},
		{UserID: "user-1", Subject: strings.Repeat("s", maxSearchFilterBytes+1)},
	}
	for _, query := range tests {
		query := query
		t.Run(fmt.Sprintf("%#v", query), func(t *testing.T) {
			t.Parallel()

			repo := &fakeRepository{}
			service := New(repo, nil)
			if _, err := service.SearchDrafts(context.Background(), query); err == nil {
				t.Fatalf("SearchDrafts accepted unsafe query %#v", query)
			}
			if repo.lastDraftSearchQuery.UserID != "" {
				t.Fatalf("repository was called with query %#v", repo.lastDraftSearchQuery)
			}
		})
	}
}

func TestCanUseSearchIDSourceContract(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		query maildb.MessageSearchQuery
		want  bool
	}{
		{
			name: "relevance query",
			query: maildb.MessageSearchQuery{
				UserID: "user-1",
				Query:  "hello",
				Sort:   maildb.MessageSearchSortRelevance,
			},
			want: true,
		},
		{
			name: "folder scoped relevance query",
			query: maildb.MessageSearchQuery{
				UserID:   "user-1",
				FolderID: "folder-1",
				Query:    "hello",
				Sort:     maildb.MessageSearchSortRelevance,
			},
			want: true,
		},
		{
			name: "empty query",
			query: maildb.MessageSearchQuery{
				UserID: "user-1",
				Sort:   maildb.MessageSearchSortRelevance,
			},
			want: false,
		},
		{
			name: "default date sort",
			query: maildb.MessageSearchQuery{
				UserID: "user-1",
				Query:  "hello",
			},
			want: false,
		},
		{
			name: "explicit date sort",
			query: maildb.MessageSearchQuery{
				UserID: "user-1",
				Query:  "hello",
				Sort:   maildb.MessageSearchSortDate,
			},
			want: false,
		},
		{
			name: "unknown sort",
			query: maildb.MessageSearchQuery{
				UserID: "user-1",
				Query:  "hello",
				Sort:   "thread",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := canUseSearchIDSource(tt.query); got != tt.want {
				t.Fatalf("canUseSearchIDSource() = %v, want %v", got, tt.want)
			}
		})
	}
}

type fakeRepository struct {
	detail                         maildb.MessageDetail
	imapMessage                    maildb.IMAPStoredMessage
	imapFlagSummaries              []imapgw.MessageSummary
	imapFlagErr                    error
	imapAppendTarget               maildb.IMAPAppendTarget
	imapAppendResult               imapgw.AppendMessageResult
	imapAppendStoredErr            error
	imapCopySummaries              []imapgw.CopyMessageResult
	imapMoveResults                []imapgw.MoveMessageResult
	imapExpungeSummaries           []imapgw.MessageSummary
	imapUIDs                       []maildb.IMAPMessageUID
	imapMailboxes                  []imapgw.Mailbox
	imapSubscriptions              []imapgw.MailboxSubscription
	imapSubscription               imapgw.MailboxSubscription
	imapMessages                   []imapgw.MessageSummary
	backfilledIMAPUIDs             []maildb.IMAPMessageUID
	ensuredIMAPUIDs                []maildb.IMAPMessageUID
	attachments                    []maildb.Attachment
	list                           []maildb.MessageSummary
	draftSearchResults             []maildb.MessageDetail
	bulkThreadFlagResult           maildb.BulkThreadFlagResult
	bulkThreadMoveResult           maildb.BulkThreadMoveResult
	bulkThreadDeleteResult         maildb.BulkThreadDeleteResult
	bulkRestoreResult              maildb.BulkMessageRestoreResult
	bulkThreadRestoreResult        maildb.BulkThreadRestoreResult
	threadMessageIDs               []string
	messagesByID                   []maildb.MessageSummary
	suppressed                     []string
	domainPolicy                   maildb.DomainPolicyView
	sourceThread                   maildb.SourceThreadView
	seenSuppressionRecipients      []string
	lastDomainPolicyID             string
	lastDomainPolicyUserID         string
	lastDraft                      maildb.SaveDraftRequest
	lastAttachmentUpload           maildb.CreateAttachmentUploadRequest
	lastCancelAttachmentUserID     string
	lastCancelAttachmentID         string
	lastAttachmentUploadSession    maildb.CreateAttachmentUploadSessionRequest
	lastCancelUploadSession        maildb.CancelAttachmentUploadSessionRequest
	lastGetUploadSession           maildb.GetAttachmentUploadSessionRequest
	lastStoreUploadSessionBody     maildb.StoreAttachmentUploadSessionBodyRequest
	lastFinalizeUploadSession      maildb.FinalizeAttachmentUploadSessionRequest
	lastExpireUploadSessions       maildb.ExpireAttachmentUploadSessionsRequest
	lastUploadSessionCleanupCount  maildb.ExpireAttachmentUploadSessionsRequest
	lastUploadSessionCleanupList   maildb.ExpireAttachmentUploadSessionsRequest
	lastAttachmentCleanup          maildb.ExpireStaleAttachmentUploadsRequest
	lastAttachmentCleanupCount     maildb.ExpireStaleAttachmentUploadsRequest
	lastAttachmentCleanupList      maildb.ExpireStaleAttachmentUploadsRequest
	lastAttachmentUserID           string
	lastAttachmentMessageID        string
	lastAttachmentID               string
	attachment                     maildb.Attachment
	canceledAttachment             maildb.Attachment
	uploadSession                  maildb.AttachmentUploadSession
	expiredUploadSessions          []maildb.AttachmentUploadSession
	staleUploadSessionCount        maildb.StaleAttachmentUploadSessionCount
	staleUploadSessionCandidates   []maildb.StaleAttachmentUploadSessionCandidate
	expiredAttachments             []maildb.Attachment
	staleAttachmentCount           maildb.StaleAttachmentUploadCount
	staleAttachmentCandidates      []maildb.StaleAttachmentUploadCandidate
	lastListFoldersUserID          string
	lastCreateFolder               maildb.CreateFolderRequest
	lastRenameFolderUserID         string
	lastRenameFolderID             string
	lastRenameFolderName           string
	lastDeleteFolderUserID         string
	lastDeleteFolderID             string
	lastListMessagesUserID         string
	lastListMessagesLimit          int
	lastListMessagesInFolderUserID string
	lastListMessagesFolderID       string
	lastListMessagesInFolderLimit  int
	lastPageUserID                 string
	lastPageFolderID               string
	lastPageLimit                  int
	lastPageFilter                 maildb.MessageListFilter
	lastListThreadsUserID          string
	lastListThreadsLimit           int
	lastListThreadsCursor          maildb.ThreadListCursor
	lastListThreadsFilter          maildb.ThreadListFilter
	lastThreadMessagesUserID       string
	lastThreadID                   string
	lastThreadMessagesLimit        int
	lastThreadMessagesCursor       maildb.MessageListCursor
	lastGetMessageUserID           string
	lastGetMessageID               string
	lastSearchQuery                maildb.MessageSearchQuery
	lastDraftSearchQuery           maildb.DraftSearchQuery
	lastFlagMessageID              string
	lastFlag                       string
	lastBulkFlag                   maildb.BulkMessageFlagRequest
	lastBulkThreadFlag             maildb.BulkThreadFlagRequest
	lastBulkMove                   maildb.BulkMessageMoveRequest
	lastBulkThreadMove             maildb.BulkThreadMoveRequest
	lastBulkDelete                 maildb.BulkMessageDeleteRequest
	lastBulkThreadDelete           maildb.BulkThreadDeleteRequest
	lastBulkRestore                maildb.BulkMessageRestoreRequest
	lastBulkThreadRestore          maildb.BulkThreadRestoreRequest
	lastListThreadMessageUserID    string
	lastListThreadMessageThreadIDs []string
	lastMutationUserID             string
	lastMoveMessageID              string
	lastMoveFolderID               string
	lastDeleteMessageID            string
	lastRestoreMessageID           string
	lastPageCursor                 maildb.MessageListCursor
	lastHydrateIDs                 []string
	lastSentDraftID                string
	lastSentDraftMessageID         string
	lastSenderUserID               string
	lastSenderFrom                 string
	lastOutgoing                   maildb.OutgoingMessage
	lastPushDevice                 maildb.UpsertPushDeviceRequest
	lastListPushDeviceUserID       string
	lastListPushDeviceLimit        int
	lastDeletePushDeviceUserID     string
	lastDeletePushDeviceID         string
	lastDeliveryStatusUserID       string
	lastDeliveryStatusMessageID    string
	lastSourceThreadUserID         string
	lastSourceThreadMessageID      string
	lastIMAPFlags                  imapgw.MessageFlags
	lastIMAPFlagMode               imapgw.StoreFlagsMode
	lastIMAPFlagUnchangedSince     uint64
	lastIMAPFlagUnchangedSinceSet  bool
	lastIMAPFlagUserID             string
	lastIMAPFlagMailboxID          string
	lastIMAPAppendUserID           string
	lastIMAPAppendMailboxID        string
	lastIMAPAppendStored           maildb.AppendStoredIMAPMessageRequest
	lastIMAPCopyUserID             string
	lastIMAPCopySourceMailboxID    string
	lastIMAPCopyDestMailboxID      string
	lastIMAPCopyUIDs               []imapgw.UID
	lastIMAPMoveUserID             string
	lastIMAPMoveSourceMailboxID    string
	lastIMAPMoveDestMailboxID      string
	lastIMAPMoveUIDs               []imapgw.UID
	lastIMAPExpungeUserID          string
	lastIMAPExpungeMailboxID       string
	lastIMAPExpungeUIDs            []imapgw.UID
	lastIMAPUIDLookupUserID        string
	lastIMAPUIDLookupMessageIDs    []string
	lastEnsureIMAPUIDUserID        string
	lastEnsureIMAPUIDMessageIDs    []string
	lastIMAPMailboxUserID          string
	lastIMAPMessageUserID          string
	lastIMAPMessageMailboxID       string
	lastIMAPMessageLimit           int
	lastIMAPMessageAfterUID        imapgw.UID
	lastUnsubscribeIMAPMailboxID   string
	lastBackfillUserID             string
	lastBackfillMailboxID          string
	lastBackfillLimit              int
	recordErr                      error
	storeUploadSessionBodyErr      error
}

type fakeSubmissionAuthenticator struct {
	user     smtpd.SubmissionUser
	username string
	password string
}

func (f *fakeSubmissionAuthenticator) AuthenticatePlain(_ context.Context, _ string, username string, password string) (smtpd.SubmissionUser, error) {
	f.username = username
	f.password = password
	return f.user, nil
}

func (f *fakeRepository) ListMessages(_ context.Context, userID string, limit int) ([]maildb.MessageSummary, error) {
	f.lastListMessagesUserID = userID
	f.lastListMessagesLimit = limit
	return nil, nil
}

func (f *fakeRepository) ListMessagesInFolder(_ context.Context, userID string, folderID string, limit int) ([]maildb.MessageSummary, error) {
	f.lastListMessagesInFolderUserID = userID
	f.lastListMessagesFolderID = folderID
	f.lastListMessagesInFolderLimit = limit
	return nil, nil
}

func (f *fakeRepository) ListMessagesPage(_ context.Context, userID string, folderID string, limit int, cursor maildb.MessageListCursor, filter maildb.MessageListFilter) ([]maildb.MessageSummary, error) {
	f.lastPageUserID = userID
	f.lastPageFolderID = folderID
	f.lastPageLimit = limit
	f.lastPageCursor = cursor
	f.lastPageFilter = filter
	return []maildb.MessageSummary{{ID: "msg-page"}}, nil
}

func (f *fakeRepository) SearchMessages(_ context.Context, query maildb.MessageSearchQuery) ([]maildb.MessageSummary, error) {
	f.lastSearchQuery = query
	return f.list, nil
}

func (f *fakeRepository) SearchDrafts(_ context.Context, query maildb.DraftSearchQuery) ([]maildb.MessageDetail, error) {
	f.lastDraftSearchQuery = query
	return f.draftSearchResults, nil
}

func (f *fakeRepository) ListMessagesByIDs(_ context.Context, _ string, ids []string) ([]maildb.MessageSummary, error) {
	f.lastHydrateIDs = append([]string(nil), ids...)
	return f.messagesByID, nil
}

func (f *fakeRepository) ListThreads(_ context.Context, userID string, limit int) ([]maildb.ThreadSummary, error) {
	f.lastListThreadsUserID = userID
	f.lastListThreadsLimit = limit
	return nil, nil
}

func (f *fakeRepository) ListThreadsPage(_ context.Context, userID string, limit int, cursor maildb.ThreadListCursor, filter maildb.ThreadListFilter) ([]maildb.ThreadSummary, error) {
	f.lastListThreadsUserID = userID
	f.lastListThreadsLimit = limit
	f.lastListThreadsCursor = cursor
	f.lastListThreadsFilter = filter
	return nil, nil
}

func (f *fakeRepository) ListThreadMessages(_ context.Context, userID string, threadID string, limit int) ([]maildb.MessageSummary, error) {
	f.lastThreadMessagesUserID = userID
	f.lastThreadID = threadID
	f.lastThreadMessagesLimit = limit
	return nil, nil
}

func (f *fakeRepository) ListThreadMessagesPage(_ context.Context, userID string, threadID string, limit int, cursor maildb.MessageListCursor) ([]maildb.MessageSummary, error) {
	f.lastThreadMessagesUserID = userID
	f.lastThreadID = threadID
	f.lastThreadMessagesLimit = limit
	f.lastThreadMessagesCursor = cursor
	return nil, nil
}

func (f *fakeRepository) ListFolders(_ context.Context, userID string) ([]maildb.Folder, error) {
	f.lastListFoldersUserID = userID
	return nil, nil
}

func (f *fakeRepository) CreateFolder(_ context.Context, req maildb.CreateFolderRequest) (maildb.Folder, error) {
	f.lastCreateFolder = req
	return maildb.Folder{ID: strings.ToLower(req.Name) + "-id", Name: req.Name, FullPath: req.Name, Type: "user"}, nil
}

func (f *fakeRepository) RenameFolder(_ context.Context, userID string, folderID string, name string) (maildb.Folder, error) {
	f.lastRenameFolderUserID = userID
	f.lastRenameFolderID = folderID
	f.lastRenameFolderName = name
	return maildb.Folder{ID: strings.ToLower(name) + "-id", Name: name, FullPath: name, Type: "user"}, nil
}

func (f *fakeRepository) DeleteFolder(_ context.Context, userID string, folderID string) error {
	f.lastDeleteFolderUserID = userID
	f.lastDeleteFolderID = folderID
	return nil
}

func (f *fakeRepository) GetMessage(_ context.Context, userID string, messageID string) (maildb.MessageDetail, error) {
	f.lastGetMessageUserID = userID
	f.lastGetMessageID = messageID
	return f.detail, nil
}

func (f *fakeRepository) GetIMAPMessage(_ context.Context, userID string, mailboxID string, _ imapgw.UID) (maildb.IMAPStoredMessage, error) {
	f.lastIMAPMessageUserID = userID
	f.lastIMAPMessageMailboxID = mailboxID
	return f.imapMessage, nil
}

func (f *fakeRepository) ListIMAPMailboxes(_ context.Context, userID string) ([]imapgw.Mailbox, error) {
	f.lastIMAPMailboxUserID = userID
	return f.imapMailboxes, nil
}

func (f *fakeRepository) ListSubscribedIMAPMailboxes(_ context.Context, userID string) ([]imapgw.MailboxSubscription, error) {
	f.lastIMAPMailboxUserID = userID
	return f.imapSubscriptions, nil
}

func (f *fakeRepository) GetIMAPMailbox(_ context.Context, userID string, mailboxID string) (imapgw.Mailbox, error) {
	f.lastIMAPMailboxUserID = userID
	f.lastIMAPMessageMailboxID = mailboxID
	if len(f.imapMailboxes) == 0 {
		return imapgw.Mailbox{}, nil
	}
	return f.imapMailboxes[0], nil
}

func (f *fakeRepository) SubscribeIMAPMailbox(_ context.Context, userID string, mailboxID string) (imapgw.MailboxSubscription, error) {
	f.lastIMAPMailboxUserID = userID
	f.lastIMAPMessageMailboxID = mailboxID
	return f.imapSubscription, nil
}

func (f *fakeRepository) UnsubscribeIMAPMailbox(_ context.Context, userID string, mailboxID string) error {
	f.lastIMAPMailboxUserID = userID
	f.lastUnsubscribeIMAPMailboxID = mailboxID
	return nil
}

func (f *fakeRepository) ListIMAPMessages(_ context.Context, userID string, mailboxID string, limit int, afterUID imapgw.UID) ([]imapgw.MessageSummary, error) {
	f.lastIMAPMessageUserID = userID
	f.lastIMAPMessageMailboxID = mailboxID
	f.lastIMAPMessageLimit = limit
	f.lastIMAPMessageAfterUID = afterUID
	return f.imapMessages, nil
}

func (f *fakeRepository) BackfillIMAPMailboxUIDs(_ context.Context, userID string, mailboxID string, limit int) ([]maildb.IMAPMessageUID, error) {
	f.lastBackfillUserID = userID
	f.lastBackfillMailboxID = mailboxID
	f.lastBackfillLimit = limit
	return f.backfilledIMAPUIDs, nil
}

func (f *fakeRepository) StoreIMAPFlags(_ context.Context, userID string, mailboxID string, _ []imapgw.UID, flags imapgw.MessageFlags, mode imapgw.StoreFlagsMode, unchangedSince uint64, unchangedSinceSet bool) ([]imapgw.MessageSummary, error) {
	f.lastIMAPFlagUserID = userID
	f.lastIMAPFlagMailboxID = mailboxID
	f.lastIMAPFlags = flags
	f.lastIMAPFlagMode = mode
	f.lastIMAPFlagUnchangedSince = unchangedSince
	f.lastIMAPFlagUnchangedSinceSet = unchangedSinceSet
	if f.imapFlagErr != nil {
		return f.imapFlagSummaries, f.imapFlagErr
	}
	return f.imapFlagSummaries, nil
}

func (f *fakeRepository) ResolveIMAPAppendTarget(_ context.Context, userID string, mailboxID string) (maildb.IMAPAppendTarget, error) {
	f.lastIMAPAppendUserID = userID
	f.lastIMAPAppendMailboxID = mailboxID
	return f.imapAppendTarget, nil
}

func (f *fakeRepository) AppendStoredIMAPMessage(_ context.Context, req maildb.AppendStoredIMAPMessageRequest) (imapgw.AppendMessageResult, error) {
	f.lastIMAPAppendStored = req
	if f.imapAppendStoredErr != nil {
		return imapgw.AppendMessageResult{}, f.imapAppendStoredErr
	}
	return f.imapAppendResult, nil
}

func (f *fakeRepository) CopyIMAPMessages(_ context.Context, userID string, sourceMailboxID string, destMailboxID string, uids []imapgw.UID) ([]imapgw.CopyMessageResult, error) {
	f.lastIMAPCopyUserID = userID
	f.lastIMAPCopySourceMailboxID = sourceMailboxID
	f.lastIMAPCopyDestMailboxID = destMailboxID
	f.lastIMAPCopyUIDs = append([]imapgw.UID(nil), uids...)
	return f.imapCopySummaries, nil
}

func (f *fakeRepository) MoveIMAPMessages(_ context.Context, userID string, sourceMailboxID string, destMailboxID string, uids []imapgw.UID) ([]imapgw.MoveMessageResult, error) {
	f.lastIMAPMoveUserID = userID
	f.lastIMAPMoveSourceMailboxID = sourceMailboxID
	f.lastIMAPMoveDestMailboxID = destMailboxID
	f.lastIMAPMoveUIDs = append([]imapgw.UID(nil), uids...)
	return f.imapMoveResults, nil
}

func (f *fakeRepository) ExpungeIMAPMessages(_ context.Context, userID string, mailboxID string, uids []imapgw.UID) ([]imapgw.MessageSummary, error) {
	f.lastIMAPExpungeUserID = userID
	f.lastIMAPExpungeMailboxID = mailboxID
	f.lastIMAPExpungeUIDs = append([]imapgw.UID(nil), uids...)
	return f.imapExpungeSummaries, nil
}

func (f *fakeRepository) ExistingIMAPMessageUIDs(_ context.Context, userID string, messageIDs []string) ([]maildb.IMAPMessageUID, error) {
	f.lastIMAPUIDLookupUserID = userID
	f.lastIMAPUIDLookupMessageIDs = append([]string(nil), messageIDs...)
	return f.imapUIDs, nil
}

func (f *fakeRepository) SetMessageFlag(_ context.Context, userID string, messageID string, flag string, _ bool) error {
	f.lastMutationUserID = userID
	f.lastFlagMessageID = messageID
	f.lastFlag = flag
	return nil
}

func (f *fakeRepository) BulkSetMessageFlag(_ context.Context, req maildb.BulkMessageFlagRequest) (int64, error) {
	f.lastBulkFlag = req
	return int64(len(req.MessageIDs)), nil
}

func (f *fakeRepository) BulkSetThreadFlag(_ context.Context, req maildb.BulkThreadFlagRequest) (maildb.BulkThreadFlagResult, error) {
	f.lastBulkThreadFlag = req
	if f.bulkThreadFlagResult.Updated != 0 || len(f.bulkThreadFlagResult.MessageIDs) > 0 {
		return f.bulkThreadFlagResult, nil
	}
	return maildb.BulkThreadFlagResult{Updated: int64(len(req.ThreadIDs)), MessageIDs: []string{"msg-thread"}}, nil
}

func (f *fakeRepository) MoveMessage(_ context.Context, userID string, messageID string, folderID string) error {
	f.lastMutationUserID = userID
	f.lastMoveMessageID = messageID
	f.lastMoveFolderID = folderID
	return nil
}

func (f *fakeRepository) BulkMoveMessages(_ context.Context, req maildb.BulkMessageMoveRequest) (int64, error) {
	f.lastBulkMove = req
	return int64(len(req.MessageIDs)), nil
}

func (f *fakeRepository) ListMessageIDsForThreads(_ context.Context, userID string, threadIDs []string) ([]string, error) {
	f.lastListThreadMessageUserID = userID
	f.lastListThreadMessageThreadIDs = append([]string(nil), threadIDs...)
	return f.threadMessageIDs, nil
}

func (f *fakeRepository) BulkMoveThreads(_ context.Context, req maildb.BulkThreadMoveRequest) (maildb.BulkThreadMoveResult, error) {
	f.lastBulkThreadMove = req
	if f.bulkThreadMoveResult.Updated != 0 || len(f.bulkThreadMoveResult.MessageIDs) > 0 {
		return f.bulkThreadMoveResult, nil
	}
	return maildb.BulkThreadMoveResult{Updated: int64(len(req.ThreadIDs)), MessageIDs: []string{"msg-thread"}}, nil
}

func (f *fakeRepository) DeleteMessage(_ context.Context, userID string, messageID string) error {
	f.lastMutationUserID = userID
	f.lastDeleteMessageID = messageID
	return nil
}

func (f *fakeRepository) BulkDeleteMessages(_ context.Context, req maildb.BulkMessageDeleteRequest) (int64, error) {
	f.lastBulkDelete = req
	return int64(len(req.MessageIDs)), nil
}

func (f *fakeRepository) BulkDeleteThreads(_ context.Context, req maildb.BulkThreadDeleteRequest) (maildb.BulkThreadDeleteResult, error) {
	f.lastBulkThreadDelete = req
	if f.bulkThreadDeleteResult.Updated != 0 || len(f.bulkThreadDeleteResult.MessageIDs) > 0 {
		return f.bulkThreadDeleteResult, nil
	}
	return maildb.BulkThreadDeleteResult{Updated: int64(len(req.ThreadIDs)), MessageIDs: []string{"msg-thread"}}, nil
}

func (f *fakeRepository) RestoreMessage(_ context.Context, userID string, messageID string) error {
	f.lastMutationUserID = userID
	f.lastRestoreMessageID = messageID
	return nil
}

func (f *fakeRepository) BulkRestoreMessages(_ context.Context, req maildb.BulkMessageRestoreRequest) (maildb.BulkMessageRestoreResult, error) {
	f.lastBulkRestore = req
	if f.bulkRestoreResult.Updated != 0 || len(f.bulkRestoreResult.MessageIDs) > 0 {
		return f.bulkRestoreResult, nil
	}
	return maildb.BulkMessageRestoreResult{Updated: int64(len(req.MessageIDs)), MessageIDs: append([]string(nil), req.MessageIDs...)}, nil
}

func (f *fakeRepository) BulkRestoreThreads(_ context.Context, req maildb.BulkThreadRestoreRequest) (maildb.BulkThreadRestoreResult, error) {
	f.lastBulkThreadRestore = req
	if f.bulkThreadRestoreResult.Updated != 0 || len(f.bulkThreadRestoreResult.MessageIDs) > 0 {
		return f.bulkThreadRestoreResult, nil
	}
	return maildb.BulkThreadRestoreResult{Updated: int64(len(req.ThreadIDs)), MessageIDs: []string{"msg-thread"}}, nil
}

func (f *fakeRepository) EnsureIMAPMessageUIDsForMessages(_ context.Context, userID string, messageIDs []string) ([]maildb.IMAPMessageUID, error) {
	f.lastEnsureIMAPUIDUserID = userID
	f.lastEnsureIMAPUIDMessageIDs = append([]string(nil), messageIDs...)
	return f.ensuredIMAPUIDs, nil
}

func (f *fakeRepository) ListPushDevices(_ context.Context, userID string, limit int) ([]maildb.PushDevice, error) {
	f.lastListPushDeviceUserID = userID
	f.lastListPushDeviceLimit = limit
	return nil, nil
}

func (f *fakeRepository) UpsertPushDevice(_ context.Context, req maildb.UpsertPushDeviceRequest) (maildb.PushDevice, error) {
	f.lastPushDevice = req
	return maildb.PushDevice{ID: "device-1", UserID: req.UserID, Platform: req.Platform, Token: req.Token, Status: "active"}, nil
}

func (f *fakeRepository) DeletePushDevice(_ context.Context, userID string, id string) error {
	f.lastDeletePushDeviceUserID = userID
	f.lastDeletePushDeviceID = id
	return nil
}

func (f *fakeRepository) MessageDeliveryStatus(_ context.Context, userID string, messageID string) (maildb.MessageDeliveryStatusView, error) {
	f.lastDeliveryStatusUserID = userID
	f.lastDeliveryStatusMessageID = messageID
	return maildb.MessageDeliveryStatusView{}, nil
}

func (f *fakeRepository) ListAttachments(_ context.Context, userID string, messageID string) ([]maildb.Attachment, error) {
	f.lastAttachmentUserID = userID
	f.lastAttachmentMessageID = messageID
	return f.attachments, nil
}

func (f *fakeRepository) GetAttachment(_ context.Context, userID string, messageID string, attachmentID string) (maildb.Attachment, error) {
	f.lastAttachmentUserID = userID
	f.lastAttachmentMessageID = messageID
	f.lastAttachmentID = attachmentID
	return f.attachment, nil
}

func (f *fakeRepository) AttachmentsByIDs(_ context.Context, userID string, attachmentIDs []string) ([]maildb.Attachment, error) {
	f.lastAttachmentUserID = userID
	f.lastAttachmentID = strings.Join(attachmentIDs, ",")
	if len(f.attachments) == 0 {
		if f.attachment.ID != "" {
			return []maildb.Attachment{f.attachment}, nil
		}
		return nil, nil
	}
	if len(attachmentIDs) == 0 {
		return f.attachments, nil
	}
	lookup := make(map[string]maildb.Attachment, len(f.attachments))
	for _, attachment := range f.attachments {
		lookup[attachment.ID] = attachment
	}
	resolved := make([]maildb.Attachment, 0, len(attachmentIDs))
	for _, id := range attachmentIDs {
		if attachment, ok := lookup[id]; ok {
			resolved = append(resolved, attachment)
		}
	}
	return resolved, nil
}

func (f *fakeRepository) SenderForUser(_ context.Context, userID string, from string) (maildb.Sender, error) {
	f.lastSenderUserID = userID
	f.lastSenderFrom = from
	return maildb.Sender{
		CompanyID:   "company-1",
		DomainID:    "domain-1",
		UserID:      "user-1",
		Address:     "sender@example.com",
		DisplayName: "Sender",
	}, nil
}

func (f *fakeRepository) RecordOutgoing(_ context.Context, msg maildb.OutgoingMessage) (string, error) {
	f.lastOutgoing = msg
	if f.recordErr != nil {
		return "", f.recordErr
	}
	return "msg-1", nil
}

func (f *fakeRepository) SuppressedRecipients(_ context.Context, _ string, recipients []string) ([]string, error) {
	f.seenSuppressionRecipients = append([]string(nil), recipients...)
	return f.suppressed, nil
}

func (f *fakeRepository) DomainPolicy(_ context.Context, domainID string) (maildb.DomainPolicyView, error) {
	f.lastDomainPolicyID = domainID
	if f.domainPolicy.DomainID == "" {
		return maildb.DomainPolicyView{DomainID: domainID, InboundMode: "inherit", OutboundMode: "inherit"}, nil
	}
	return f.domainPolicy, nil
}

func (f *fakeRepository) DomainPolicyForUser(_ context.Context, userID string) (maildb.DomainPolicyView, error) {
	f.lastDomainPolicyUserID = userID
	if f.domainPolicy.DomainID == "" {
		return maildb.DomainPolicyView{DomainID: "domain-1", InboundMode: "inherit", OutboundMode: "inherit"}, nil
	}
	return f.domainPolicy, nil
}

func (f *fakeRepository) SourceThread(_ context.Context, userID string, sourceMessageID string) (maildb.SourceThreadView, error) {
	f.lastSourceThreadUserID = userID
	f.lastSourceThreadMessageID = sourceMessageID
	return f.sourceThread, nil
}

func (f *fakeRepository) SaveDraft(_ context.Context, req maildb.SaveDraftRequest) (maildb.MessageDetail, error) {
	f.lastDraft = req
	return maildb.MessageDetail{ID: "draft-1", Subject: req.Subject}, nil
}

func (f *fakeRepository) DeleteDraft(_ context.Context, userID string, draftID string) error {
	f.lastSentDraftID = draftID
	f.lastSenderUserID = userID
	return nil
}

func (f *fakeRepository) GetDraftForSend(context.Context, string, string) (maildb.DraftForSend, error) {
	return maildb.DraftForSend{
		ID:            "draft-1",
		UserID:        "user-1",
		Intent:        string(ComposeIntentNew),
		To:            []outbound.Address{{Email: "recipient@example.net"}},
		Subject:       "draft subject",
		TextBody:      "draft body",
		HTMLBody:      "<p>draft html</p>",
		AttachmentIDs: []string{"att-1"},
		TrackOpens:    true,
		ScheduledAt:   time.Unix(4102444800, 0).UTC(),
	}, nil
}

func (f *fakeRepository) MarkDraftSent(_ context.Context, _ string, draftID string, sentMessageID string) error {
	f.lastSentDraftID = draftID
	f.lastSentDraftMessageID = sentMessageID
	return nil
}

func (f *fakeRepository) CreateAttachmentUpload(_ context.Context, req maildb.CreateAttachmentUploadRequest) (maildb.Attachment, error) {
	f.lastAttachmentUpload = req
	return maildb.Attachment{ID: "att-1", Filename: req.Filename, MIMEType: req.MIMEType, Size: req.Size}, nil
}

func (f *fakeRepository) CancelAttachmentUpload(_ context.Context, userID string, attachmentID string) (maildb.Attachment, error) {
	f.lastCancelAttachmentUserID = userID
	f.lastCancelAttachmentID = attachmentID
	if f.canceledAttachment.ID != "" {
		return f.canceledAttachment, nil
	}
	return maildb.Attachment{ID: attachmentID, Status: "deleted"}, nil
}

func (f *fakeRepository) CreateAttachmentUploadSession(_ context.Context, req maildb.CreateAttachmentUploadSessionRequest) (maildb.AttachmentUploadSession, error) {
	f.lastAttachmentUploadSession = req
	if f.uploadSession.ID != "" {
		return f.uploadSession, nil
	}
	return maildb.AttachmentUploadSession{ID: "session-1", UserID: req.UserID, Filename: req.Filename, DeclaredSize: req.DeclaredSize, MIMEType: req.MIMEType, Status: "pending"}, nil
}

func (f *fakeRepository) CancelAttachmentUploadSession(_ context.Context, req maildb.CancelAttachmentUploadSessionRequest) (maildb.AttachmentUploadSession, error) {
	f.lastCancelUploadSession = req
	if f.uploadSession.ID != "" {
		return f.uploadSession, nil
	}
	return maildb.AttachmentUploadSession{ID: req.SessionID, UserID: req.UserID, Status: "canceled"}, nil
}

func (f *fakeRepository) GetAttachmentUploadSession(_ context.Context, req maildb.GetAttachmentUploadSessionRequest) (maildb.AttachmentUploadSession, error) {
	f.lastGetUploadSession = req
	if f.uploadSession.ID != "" {
		return f.uploadSession, nil
	}
	return maildb.AttachmentUploadSession{ID: req.SessionID, UserID: req.UserID, DeclaredSize: 7, Status: "pending", ExpiresAt: time.Now().Add(time.Hour)}, nil
}

func (f *fakeRepository) StoreAttachmentUploadSessionBody(_ context.Context, req maildb.StoreAttachmentUploadSessionBodyRequest) (maildb.AttachmentUploadSession, error) {
	f.lastStoreUploadSessionBody = req
	if f.storeUploadSessionBodyErr != nil {
		return maildb.AttachmentUploadSession{}, f.storeUploadSessionBodyErr
	}
	if f.uploadSession.ID != "" {
		f.uploadSession.ReceivedSize = req.ReceivedSize
		f.uploadSession.StoragePath = req.StoragePath
		f.uploadSession.ChecksumSHA256 = req.ChecksumSHA256
		f.uploadSession.Status = "uploading"
		return f.uploadSession, nil
	}
	return maildb.AttachmentUploadSession{ID: req.SessionID, UserID: req.UserID, ReceivedSize: req.ReceivedSize, StoragePath: req.StoragePath, ChecksumSHA256: req.ChecksumSHA256, Status: "uploading"}, nil
}

func (f *fakeRepository) StoreAttachmentUploadSessionChunk(_ context.Context, req maildb.StoreAttachmentUploadSessionChunkRequest) (maildb.AttachmentUploadSession, error) {
	chunkSize := req.ContentRange.LastByte - req.ContentRange.FirstByte + 1
	if f.uploadSession.ID != "" {
		f.uploadSession.ReceivedSize += chunkSize
		f.uploadSession.StoragePath = req.StoragePath
		f.uploadSession.Status = "uploading"
		return f.uploadSession, nil
	}
	return maildb.AttachmentUploadSession{
		ID:           req.SessionID,
		UserID:       req.UserID,
		ReceivedSize: chunkSize,
		StoragePath:  req.StoragePath,
		Status:       "uploading",
	}, nil
}

func (f *fakeRepository) FinalizeAttachmentUploadSession(_ context.Context, req maildb.FinalizeAttachmentUploadSessionRequest) (maildb.Attachment, error) {
	f.lastFinalizeUploadSession = req
	return maildb.Attachment{ID: "att-1", UploadID: "upload-1", StoragePath: "upload-sessions/user-1/session-1/body", Filename: "large.bin", Size: 7, MIMEType: "application/octet-stream", Status: "uploading"}, nil
}

func (f *fakeRepository) ExpireAttachmentUploadSessions(_ context.Context, req maildb.ExpireAttachmentUploadSessionsRequest) ([]maildb.AttachmentUploadSession, error) {
	f.lastExpireUploadSessions = req
	return f.expiredUploadSessions, nil
}

func (f *fakeRepository) CountStaleAttachmentUploadSessions(_ context.Context, req maildb.ExpireAttachmentUploadSessionsRequest) (maildb.StaleAttachmentUploadSessionCount, error) {
	f.lastUploadSessionCleanupCount = req
	return f.staleUploadSessionCount, nil
}

func (f *fakeRepository) ListStaleAttachmentUploadSessions(_ context.Context, req maildb.ExpireAttachmentUploadSessionsRequest) ([]maildb.StaleAttachmentUploadSessionCandidate, error) {
	f.lastUploadSessionCleanupList = req
	return f.staleUploadSessionCandidates, nil
}

type fakeSearchIDSource struct {
	hits      []searchindex.OpenSearchHit
	lastQuery searchindex.OpenSearchSearchQuery
}

func (s *fakeSearchIDSource) SearchMessageIDs(_ context.Context, query searchindex.OpenSearchSearchQuery) ([]searchindex.OpenSearchHit, error) {
	s.lastQuery = query
	return s.hits, nil
}

type fakeIMAPEventPublisher struct {
	events []imapgw.MailboxEvent
	err    error
}

func (p *fakeIMAPEventPublisher) Publish(_ context.Context, event imapgw.MailboxEvent) error {
	if p.err != nil {
		return p.err
	}
	p.events = append(p.events, event)
	return nil
}

type recordingStore struct {
	getPath    string
	deletePath string
	body       string
	getCount   int
}

func (s *recordingStore) Put(context.Context, string, io.Reader) error {
	return nil
}

func (s *recordingStore) Get(_ context.Context, path string) (io.ReadCloser, error) {
	s.getPath = path
	s.getCount++
	return io.NopCloser(strings.NewReader(s.body)), nil
}

func (s *recordingStore) GetRange(_ context.Context, path string, _ storage.RangeRequest) (io.ReadCloser, error) {
	s.getPath = path
	return io.NopCloser(strings.NewReader("")), nil
}

func (s *recordingStore) Stat(_ context.Context, path string) (storage.ObjectInfo, error) {
	return storage.ObjectInfo{Path: path}, nil
}

func (s *recordingStore) Copy(_ context.Context, sourcePath string, destPath string) error {
	s.getPath = sourcePath
	s.deletePath = destPath
	return nil
}

func (s *recordingStore) Move(_ context.Context, sourcePath string, destPath string) error {
	s.getPath = sourcePath
	s.deletePath = destPath
	return nil
}

func (s *recordingStore) List(context.Context, storage.ListOptions) (storage.ObjectListPage, error) {
	return storage.ObjectListPage{}, nil
}

func (s *recordingStore) Delete(_ context.Context, path string) error {
	s.deletePath = path
	return nil
}

func (f *fakeRepository) ExpireStaleAttachmentUploads(_ context.Context, req maildb.ExpireStaleAttachmentUploadsRequest) ([]maildb.Attachment, error) {
	f.lastAttachmentCleanup = req
	return f.expiredAttachments, nil
}

func (f *fakeRepository) CountStaleAttachmentUploads(_ context.Context, req maildb.ExpireStaleAttachmentUploadsRequest) (maildb.StaleAttachmentUploadCount, error) {
	f.lastAttachmentCleanupCount = req
	return f.staleAttachmentCount, nil
}

func (f *fakeRepository) ListStaleAttachmentUploads(_ context.Context, req maildb.ExpireStaleAttachmentUploadsRequest) ([]maildb.StaleAttachmentUploadCandidate, error) {
	f.lastAttachmentCleanupList = req
	return f.staleAttachmentCandidates, nil
}

func TestExpireStaleAttachmentUploadsDeletesStoredObjects(t *testing.T) {
	t.Parallel()

	store := storage.NewLocalStore(t.TempDir())
	if err := store.Put(context.Background(), "uploads/user-1/upload-1/report.pdf", strings.NewReader("content")); err != nil {
		t.Fatalf("Put returned error: %v", err)
	}
	repo := &fakeRepository{
		expiredAttachments: []maildb.Attachment{{ID: "att-1", StoragePath: "uploads/user-1/upload-1/report.pdf"}},
	}
	service := New(repo, store)

	expired, err := service.ExpireStaleAttachmentUploads(context.Background(), time.Now(), 10)
	if err != nil {
		t.Fatalf("ExpireStaleAttachmentUploads returned error: %v", err)
	}
	if len(expired) != 1 {
		t.Fatalf("expired = %+v", expired)
	}
	if _, err := store.Get(context.Background(), "uploads/user-1/upload-1/report.pdf"); err == nil {
		t.Fatal("expired attachment object still exists")
	}
	if repo.lastAttachmentCleanup.Limit != 10 || repo.lastAttachmentCleanup.Before.IsZero() {
		t.Fatalf("cleanup request = %+v", repo.lastAttachmentCleanup)
	}
}

type failingDeleteStore struct {
	err error
}

func (s failingDeleteStore) Put(context.Context, string, io.Reader) error {
	return nil
}

func (s failingDeleteStore) Get(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}

func (s failingDeleteStore) GetRange(context.Context, string, storage.RangeRequest) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}

func (s failingDeleteStore) Stat(_ context.Context, path string) (storage.ObjectInfo, error) {
	return storage.ObjectInfo{Path: path}, nil
}

func (s failingDeleteStore) Copy(context.Context, string, string) error {
	return nil
}

func (s failingDeleteStore) Move(context.Context, string, string) error {
	return nil
}

func (s failingDeleteStore) List(context.Context, storage.ListOptions) (storage.ObjectListPage, error) {
	return storage.ObjectListPage{}, nil
}

func (s failingDeleteStore) Delete(context.Context, string) error {
	return s.err
}

func TestExpireStaleAttachmentUploadsReportsStoredObjectDeleteFailures(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{
		expiredAttachments: []maildb.Attachment{{ID: "att-1", StoragePath: "uploads/user-1/upload-1/report.pdf"}},
	}
	service := New(repo, failingDeleteStore{err: errors.New("permission denied")})

	expired, err := service.ExpireStaleAttachmentUploads(context.Background(), time.Now(), 10)
	if err == nil {
		t.Fatal("ExpireStaleAttachmentUploads returned nil error for delete failure")
	}
	if len(expired) != 1 {
		t.Fatalf("expired = %+v", expired)
	}
	if !strings.Contains(err.Error(), "delete expired attachment objects") || !strings.Contains(err.Error(), "att-1") {
		t.Fatalf("error = %v", err)
	}
}

func TestExpireStaleAttachmentUploadsRejectsUnsafeStoredBodyPath(t *testing.T) {
	t.Parallel()

	store := &recordingStore{}
	repo := &fakeRepository{
		expiredAttachments: []maildb.Attachment{{ID: "att-1", StoragePath: "uploads//user-1/report.pdf"}},
	}
	service := New(repo, store)

	expired, err := service.ExpireStaleAttachmentUploads(context.Background(), time.Now(), 10)
	if err == nil {
		t.Fatal("ExpireStaleAttachmentUploads accepted unsafe stored body path")
	}
	if len(expired) != 1 {
		t.Fatalf("expired = %+v", expired)
	}
	if store.deletePath != "" {
		t.Fatalf("store.Delete was called with %q", store.deletePath)
	}
	if !strings.Contains(err.Error(), "delete expired attachment objects") || !strings.Contains(err.Error(), "att-1") {
		t.Fatalf("error = %v", err)
	}
}

func TestExpireStaleAttachmentUploadsIgnoresMissingStoredObjects(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{
		expiredAttachments: []maildb.Attachment{{ID: "att-1", StoragePath: "uploads/user-1/upload-1/missing.pdf"}},
	}
	service := New(repo, failingDeleteStore{err: os.ErrNotExist})

	expired, err := service.ExpireStaleAttachmentUploads(context.Background(), time.Now(), 10)
	if err != nil {
		t.Fatalf("ExpireStaleAttachmentUploads returned error: %v", err)
	}
	if len(expired) != 1 {
		t.Fatalf("expired = %+v", expired)
	}
}

func TestExpireStaleAttachmentUploadsValidatesRequestBeforeRepository(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{}
	service := New(repo, nil)

	if _, err := service.ExpireStaleAttachmentUploads(context.Background(), time.Time{}, 10); err == nil {
		t.Fatal("ExpireStaleAttachmentUploads accepted zero before")
	}
	if _, err := service.ExpireStaleAttachmentUploads(context.Background(), time.Now(), -1); err == nil {
		t.Fatal("ExpireStaleAttachmentUploads accepted negative limit")
	}
	if !repo.lastAttachmentCleanup.Before.IsZero() {
		t.Fatalf("repository was called with %+v", repo.lastAttachmentCleanup)
	}
}

func TestCountStaleAttachmentUploadsUsesRepositoryPreview(t *testing.T) {
	t.Parallel()

	before := time.Now()
	repo := &fakeRepository{
		staleAttachmentCount: maildb.StaleAttachmentUploadCount{
			TotalCount:   11,
			LimitedCount: 5,
		},
	}
	service := New(repo, nil)

	counts, err := service.CountStaleAttachmentUploads(context.Background(), before, 5)
	if err != nil {
		t.Fatalf("CountStaleAttachmentUploads returned error: %v", err)
	}
	if counts.TotalCount != 11 || counts.LimitedCount != 5 {
		t.Fatalf("counts = %+v", counts)
	}
	if !repo.lastAttachmentCleanupCount.Before.Equal(before) || repo.lastAttachmentCleanupCount.Limit != 5 {
		t.Fatalf("count request = %+v", repo.lastAttachmentCleanupCount)
	}
}

func TestListStaleAttachmentUploadsUsesRepositoryPreview(t *testing.T) {
	t.Parallel()

	before := time.Now()
	repo := &fakeRepository{
		staleAttachmentCandidates: []maildb.StaleAttachmentUploadCandidate{{ID: "att-1"}},
	}
	service := New(repo, nil)

	candidates, err := service.ListStaleAttachmentUploads(context.Background(), before, 12)
	if err != nil {
		t.Fatalf("ListStaleAttachmentUploads returned error: %v", err)
	}
	if len(candidates) != 1 || candidates[0].ID != "att-1" {
		t.Fatalf("candidates = %+v", candidates)
	}
	if !repo.lastAttachmentCleanupList.Before.Equal(before) || repo.lastAttachmentCleanupList.Limit != 12 {
		t.Fatalf("list request = %+v", repo.lastAttachmentCleanupList)
	}
}

func TestSendTextStoresOutgoingMessage(t *testing.T) {
	t.Parallel()

	store := storage.NewLocalStore(t.TempDir())
	repo := &fakeRepository{}
	service := New(repo, store)

	result, err := service.SendText(context.Background(), SendTextRequest{
		UserID:          " user-1 ",
		Intent:          " New ",
		SourceMessageID: " source-ignored ",
		From:            " sender@example.com ",
		To:              []outbound.Address{{Name: " User ", Email: " user@example.net "}},
		AttachmentIDs:   []string{" att-1 "},
		Subject:         "hello",
		TextBody:        "body",
	})
	if err != nil {
		t.Fatalf("SendText returned error: %v", err)
	}
	if result.ID != "msg-1" {
		t.Fatalf("ID = %q, want msg-1", result.ID)
	}
	if result.Farm != outbound.FarmGeneral {
		t.Fatalf("Farm = %q, want general", result.Farm)
	}
	if repo.lastSenderUserID != "user-1" || repo.lastSenderFrom != "sender@example.com" {
		t.Fatalf("sender lookup = %q/%q", repo.lastSenderUserID, repo.lastSenderFrom)
	}
	if repo.lastOutgoing.ComposeIntent != "new" || repo.lastOutgoing.To[0].Email != "user@example.net" || repo.lastOutgoing.To[0].Name != "User" || !repo.lastOutgoing.HasAttachment {
		t.Fatalf("lastOutgoing = %+v", repo.lastOutgoing)
	}
}

func TestSendTextRejectsSuppressedRecipients(t *testing.T) {
	t.Parallel()

	service := New(&fakeRepository{suppressed: []string{"blocked@example.net"}}, storage.NewLocalStore(t.TempDir()))

	_, err := service.SendText(context.Background(), SendTextRequest{
		UserID:   "user-1",
		To:       []outbound.Address{{Email: "blocked@example.net"}},
		Subject:  "hello",
		TextBody: "body",
	})
	if err == nil {
		t.Fatal("SendText accepted suppressed recipient")
	}
}

func TestSendTextReturnsRecordErrorAfterStorageWrite(t *testing.T) {
	t.Parallel()

	service := New(&fakeRepository{recordErr: fmt.Errorf("record failed")}, storage.NewLocalStore(t.TempDir()))
	_, err := service.SendText(context.Background(), SendTextRequest{
		UserID:   "user-1",
		To:       []outbound.Address{{Email: "user@example.net"}},
		Subject:  "hello",
		TextBody: "body",
	})
	if err == nil {
		t.Fatal("SendText succeeded despite record failure")
	}
}

func TestSendTextRejectsMissingRecipients(t *testing.T) {
	t.Parallel()

	service := New(&fakeRepository{}, storage.NewLocalStore(t.TempDir()))
	_, err := service.SendText(context.Background(), SendTextRequest{
		UserID:   "user-1",
		Subject:  "hello",
		TextBody: "body",
	})
	if err == nil {
		t.Fatal("SendText accepted missing recipients")
	}
}

func TestSendTextDeduplicatesSuppressionRecipients(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{}
	service := New(repo, storage.NewLocalStore(t.TempDir()))

	_, err := service.SendText(context.Background(), SendTextRequest{
		UserID:   "user-1",
		To:       []outbound.Address{{Email: "User@Example.net"}},
		Cc:       []outbound.Address{{Email: "user@example.net"}},
		Bcc:      []outbound.Address{{Email: "other@example.net"}},
		Subject:  "hello",
		TextBody: "body",
	})
	if err != nil {
		t.Fatalf("SendText returned error: %v", err)
	}
	want := []string{"user@example.net", "other@example.net"}
	if strings.Join(repo.seenSuppressionRecipients, ",") != strings.Join(want, ",") {
		t.Fatalf("suppression recipients = %v, want %v", repo.seenSuppressionRecipients, want)
	}
}

func TestSendTextRejectsOutboundPolicyRecipientLimit(t *testing.T) {
	t.Parallel()

	service := New(&fakeRepository{
		domainPolicy: maildb.DomainPolicyView{
			DomainID:                "domain-1",
			InboundMode:             "inherit",
			OutboundMode:            "enforce",
			MaxRecipientsPerMessage: 1,
		},
	}, storage.NewLocalStore(t.TempDir()))

	_, err := service.SendText(context.Background(), SendTextRequest{
		UserID:   "user-1",
		To:       []outbound.Address{{Email: "one@example.net"}, {Email: "two@example.net"}},
		Subject:  "hello",
		TextBody: "body",
	})
	if err == nil || !strings.Contains(err.Error(), "max_recipients_per_message") {
		t.Fatalf("SendText error = %v, want max_recipients_per_message", err)
	}
}

func TestSendTextRejectsOutboundPolicyMessageSize(t *testing.T) {
	t.Parallel()

	service := New(&fakeRepository{
		domainPolicy: maildb.DomainPolicyView{
			DomainID:        "domain-1",
			InboundMode:     "inherit",
			OutboundMode:    "enforce",
			MaxMessageBytes: 10,
		},
	}, storage.NewLocalStore(t.TempDir()))

	_, err := service.SendText(context.Background(), SendTextRequest{
		UserID:   "user-1",
		To:       []outbound.Address{{Email: "one@example.net"}},
		Subject:  "hello",
		TextBody: "body",
	})
	if err == nil || !strings.Contains(err.Error(), "max_message_bytes") {
		t.Fatalf("SendText error = %v, want max_message_bytes", err)
	}
}

func TestSendTextDoesNotBlockMonitorPolicy(t *testing.T) {
	t.Parallel()

	service := New(&fakeRepository{
		domainPolicy: maildb.DomainPolicyView{
			DomainID:                "domain-1",
			InboundMode:             "inherit",
			OutboundMode:            "monitor",
			MaxRecipientsPerMessage: 1,
			MaxMessageBytes:         10,
		},
	}, storage.NewLocalStore(t.TempDir()))

	if _, err := service.SendText(context.Background(), SendTextRequest{
		UserID:   "user-1",
		To:       []outbound.Address{{Email: "one@example.net"}, {Email: "two@example.net"}},
		Subject:  "hello",
		TextBody: "body",
	}); err != nil {
		t.Fatalf("SendText returned error: %v", err)
	}
}

func TestSendTextMarksReplySourceAnswered(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{}
	service := New(repo, storage.NewLocalStore(t.TempDir()))
	_, err := service.SendText(context.Background(), SendTextRequest{
		UserID:          "user-1",
		Intent:          ComposeIntentReply,
		SourceMessageID: "msg-original",
		To:              []outbound.Address{{Email: "sender@example.net"}},
		Subject:         "Re: hello",
		TextBody:        "body",
	})
	if err != nil {
		t.Fatalf("SendText returned error: %v", err)
	}
	if repo.lastFlagMessageID != "msg-original" || repo.lastFlag != "answered" {
		t.Fatalf("flag = %q/%q", repo.lastFlagMessageID, repo.lastFlag)
	}
}

func TestSendTextWritesReplyThreadHeaders(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{
		sourceThread: maildb.SourceThreadView{
			MessageID: "<parent@example.com>",
			InReplyTo: "<root@example.com>",
			ThreadID:  "thread-1",
		},
	}
	store := storage.NewLocalStore(t.TempDir())
	service := New(repo, store)
	_, err := service.SendText(context.Background(), SendTextRequest{
		UserID:          " user-1 ",
		Intent:          ComposeIntentReply,
		SourceMessageID: " msg-parent ",
		To:              []outbound.Address{{Email: "sender@example.net"}},
		Subject:         "Re: hello",
		TextBody:        "body",
	})
	if err != nil {
		t.Fatalf("SendText returned error: %v", err)
	}
	if repo.lastSourceThreadUserID != "user-1" || repo.lastSourceThreadMessageID != "msg-parent" {
		t.Fatalf("source thread ids = %q/%q", repo.lastSourceThreadUserID, repo.lastSourceThreadMessageID)
	}
	body, err := store.Get(context.Background(), repo.lastOutgoing.StoragePath)
	if err != nil {
		t.Fatalf("Get stored message returned error: %v", err)
	}
	defer body.Close()
	raw, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("ReadAll returned error: %v", err)
	}
	text := string(raw)
	if !strings.Contains(text, "In-Reply-To: <parent@example.com>\r\n") {
		t.Fatalf("raw missing In-Reply-To: %s", text)
	}
	if !strings.Contains(text, "References: <root@example.com> <parent@example.com>\r\n") {
		t.Fatalf("raw missing References: %s", text)
	}
}

func TestSendTextComposesHTMLAndAttachments(t *testing.T) {
	t.Parallel()

	store := storage.NewLocalStore(t.TempDir())
	attachmentPath := "attachments/report.pdf"
	if err := store.Put(context.Background(), attachmentPath, strings.NewReader("PDFDATA")); err != nil {
		t.Fatalf("Put returned error: %v", err)
	}

	repo := &fakeRepository{
		attachments: []maildb.Attachment{
			{
				ID:          "att-1",
				Filename:    "report.pdf",
				MIMEType:    "application/pdf",
				StoragePath: attachmentPath,
				Status:      "uploading",
			},
		},
	}
	service := New(repo, store)
	_, err := service.SendText(context.Background(), SendTextRequest{
		UserID:        "user-1",
		To:            []outbound.Address{{Email: "user@example.net"}},
		Subject:       "hello",
		TextBody:      "plain body",
		HTMLBody:      "<p>rich body</p>",
		AttachmentIDs: []string{"att-1"},
	})
	if err != nil {
		t.Fatalf("SendText returned error: %v", err)
	}

	body, err := store.Get(context.Background(), repo.lastOutgoing.StoragePath)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	defer body.Close()
	raw, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("ReadAll returned error: %v", err)
	}

	parsed, err := message.ParseEML(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("ParseEML returned error: %v", err)
	}
	if parsed.HTMLBody != "<p>rich body</p>" {
		t.Fatalf("HTMLBody = %q", parsed.HTMLBody)
	}
	if !parsed.HasAttachment || len(parsed.Attachments) != 1 || parsed.Attachments[0].Filename != "report.pdf" {
		t.Fatalf("attachments = %+v", parsed.Attachments)
	}

	structure, err := message.ParseMIMEStructure(bytes.NewReader(raw), message.MIMEStructureOptions{})
	if err != nil {
		t.Fatalf("ParseMIMEStructure returned error: %v", err)
	}
	if structure.Root.MediaType != "MULTIPART" || structure.Root.MediaSubtype != "MIXED" {
		t.Fatalf("root = %+v, want multipart/mixed", structure.Root)
	}
	if len(structure.Root.Parts) != 2 {
		t.Fatalf("root parts = %d, want 2", len(structure.Root.Parts))
	}
	if structure.Root.Parts[0].MediaType != "MULTIPART" || structure.Root.Parts[0].MediaSubtype != "ALTERNATIVE" {
		t.Fatalf("body part = %+v, want multipart/alternative", structure.Root.Parts[0])
	}
	if len(structure.Root.Parts[0].Parts) != 2 || structure.Root.Parts[0].Parts[1].MediaSubtype != "HTML" {
		t.Fatalf("alternative parts = %+v", structure.Root.Parts[0].Parts)
	}
	if structure.Root.Parts[1].Disposition != "ATTACHMENT" || structure.Root.Parts[1].DispositionParams["filename"] != "report.pdf" {
		t.Fatalf("attachment part = %+v", structure.Root.Parts[1])
	}
}

func TestSendDraftSendsAndMarksDraftSent(t *testing.T) {
	t.Parallel()

	store := storage.NewLocalStore(t.TempDir())
	attachmentPath := "attachments/report.pdf"
	if err := store.Put(context.Background(), attachmentPath, strings.NewReader("PDFDATA")); err != nil {
		t.Fatalf("Put returned error: %v", err)
	}
	repo := &fakeRepository{
		attachments: []maildb.Attachment{
			{
				ID:          "att-1",
				Filename:    "report.pdf",
				MIMEType:    "application/pdf",
				StoragePath: attachmentPath,
				Status:      "uploading",
			},
		},
	}
	service := New(repo, store)
	result, err := service.SendDraft(context.Background(), "user-1", "draft-1")
	if err != nil {
		t.Fatalf("SendDraft returned error: %v", err)
	}
	if result.ID != "msg-1" {
		t.Fatalf("result = %+v", result)
	}
	if repo.lastSentDraftID != "draft-1" || repo.lastSentDraftMessageID != "msg-1" {
		t.Fatalf("sent draft marker = %q/%q", repo.lastSentDraftID, repo.lastSentDraftMessageID)
	}
	if !repo.lastOutgoing.HasAttachment {
		t.Fatal("draft attachments were not reflected on outgoing message")
	}
	if repo.lastOutgoing.ScheduledAt.IsZero() {
		t.Fatal("draft scheduled_at was not reflected on outgoing message")
	}

	body, err := store.Get(context.Background(), repo.lastOutgoing.StoragePath)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	defer body.Close()
	raw, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("ReadAll returned error: %v", err)
	}
	parsed, err := message.ParseEML(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("ParseEML returned error: %v", err)
	}
	if parsed.HTMLBody != "<p>draft html</p>" {
		t.Fatalf("HTMLBody = %q", parsed.HTMLBody)
	}
	if !parsed.HasAttachment || len(parsed.Attachments) != 1 || parsed.Attachments[0].Filename != "report.pdf" {
		t.Fatalf("attachments = %+v", parsed.Attachments)
	}
}

func TestSaveDraftDelegatesToDraftRepository(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{}
	service := New(repo, nil)
	draft, err := service.SaveDraft(context.Background(), SaveDraftRequest{
		UserID:          " user-1 ",
		DraftID:         " draft-1 ",
		Intent:          " Reply ",
		SourceMessageID: " source-1 ",
		From:            " sender@example.com ",
		To:              []outbound.Address{{Name: " User ", Email: " user@example.net "}},
		HTMLBody:        "<p>draft html</p>",
		AttachmentIDs:   []string{" att-1 "},
		TrackOpens:      true,
		ScheduledAt:     time.Unix(4102444800, 0).UTC(),
		Subject:         "draft",
	})
	if err != nil {
		t.Fatalf("SaveDraft returned error: %v", err)
	}
	if draft.ID != "draft-1" ||
		repo.lastDraft.UserID != "user-1" ||
		repo.lastDraft.DraftID != "draft-1" ||
		repo.lastDraft.Intent != "reply" ||
		repo.lastDraft.SourceMessageID != "source-1" ||
		repo.lastDraft.From != "sender@example.com" ||
		repo.lastDraft.To[0].Name != "User" ||
		repo.lastDraft.To[0].Email != "user@example.net" ||
		repo.lastDraft.HTMLBody != "<p>draft html</p>" ||
		repo.lastDraft.AttachmentIDs[0] != "att-1" ||
		!repo.lastDraft.TrackOpens ||
		repo.lastDraft.ScheduledAt.IsZero() ||
		repo.lastDraft.Subject != "draft" {
		t.Fatalf("draft = %+v last = %+v", draft, repo.lastDraft)
	}
}

func TestListMessagesPageDelegatesCursor(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{}
	service := New(repo, nil)
	cursor := maildb.MessageListCursor{ID: "11111111-1111-1111-1111-111111111111"}
	messages, err := service.ListMessagesPage(context.Background(), "user-1", "", 10, cursor, maildb.MessageListFilter{})
	if err != nil {
		t.Fatalf("ListMessagesPage returned error: %v", err)
	}
	if len(messages) != 1 || messages[0].ID != "msg-page" {
		t.Fatalf("messages = %+v", messages)
	}
	if repo.lastPageCursor.ID != cursor.ID {
		t.Fatalf("cursor = %+v, want %+v", repo.lastPageCursor, cursor)
	}
}

func TestThreadPageMethodsDelegateCursors(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{}
	service := New(repo, nil)
	threadCursor := maildb.ThreadListCursor{ID: "11111111-1111-1111-1111-111111111111"}
	read := false
	starred := true
	hasAttachment := true
	if _, err := service.ListThreadsPage(context.Background(), "user-1", 10, threadCursor, maildb.ThreadListFilter{FolderID: " folder-1 ", Read: &read, Starred: &starred, HasAttachment: &hasAttachment, Sort: " oldest "}); err != nil {
		t.Fatalf("ListThreadsPage returned error: %v", err)
	}
	if repo.lastListThreadsCursor.ID != threadCursor.ID {
		t.Fatalf("thread cursor = %+v, want %+v", repo.lastListThreadsCursor, threadCursor)
	}
	if repo.lastListThreadsFilter.Read == nil || *repo.lastListThreadsFilter.Read || repo.lastListThreadsFilter.Starred == nil || !*repo.lastListThreadsFilter.Starred {
		t.Fatalf("thread filter = %#v", repo.lastListThreadsFilter)
	}
	if repo.lastListThreadsFilter.FolderID != "folder-1" {
		t.Fatalf("thread folder filter = %#v", repo.lastListThreadsFilter)
	}
	if repo.lastListThreadsFilter.HasAttachment == nil || !*repo.lastListThreadsFilter.HasAttachment {
		t.Fatalf("thread attachment filter = %#v", repo.lastListThreadsFilter)
	}
	if repo.lastListThreadsFilter.Sort != maildb.ListSortOldest {
		t.Fatalf("thread sort = %#v", repo.lastListThreadsFilter)
	}
	messageCursor := maildb.MessageListCursor{ID: "22222222-2222-2222-2222-222222222222"}
	if _, err := service.ListThreadMessagesPage(context.Background(), "user-1", "thread-1", 10, messageCursor); err != nil {
		t.Fatalf("ListThreadMessagesPage returned error: %v", err)
	}
	if repo.lastThreadMessagesCursor.ID != messageCursor.ID {
		t.Fatalf("thread message cursor = %+v, want %+v", repo.lastThreadMessagesCursor, messageCursor)
	}
}

func TestCreateAttachmentUploadDelegatesToRepository(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{}
	service := New(repo, nil)
	attachment, err := service.CreateAttachmentUpload(context.Background(), CreateAttachmentUploadRequest{
		UserID:      " user-1 ",
		DraftID:     " draft-1 ",
		Filename:    " report.pdf ",
		Size:        42,
		MIMEType:    " application/pdf ",
		StoragePath: " uploads/user-1/report.pdf ",
	})
	if err != nil {
		t.Fatalf("CreateAttachmentUpload returned error: %v", err)
	}
	if attachment.ID != "att-1" ||
		repo.lastAttachmentUpload.UserID != "user-1" ||
		repo.lastAttachmentUpload.DraftID != "draft-1" ||
		repo.lastAttachmentUpload.Filename != "report.pdf" ||
		repo.lastAttachmentUpload.MIMEType != "application/pdf" ||
		repo.lastAttachmentUpload.StoragePath != "uploads/user-1/report.pdf" {
		t.Fatalf("attachment = %+v last = %+v", attachment, repo.lastAttachmentUpload)
	}
}

func TestCreateAttachmentUploadRejectsDomainAttachmentLimit(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{domainPolicy: maildb.DomainPolicyView{
		DomainID:           "domain-1",
		InboundMode:        "inherit",
		OutboundMode:       "enforce",
		MaxAttachmentBytes: 10,
	}}
	service := New(repo, nil)
	_, err := service.CreateAttachmentUpload(context.Background(), CreateAttachmentUploadRequest{
		UserID:   "user-1",
		Filename: "report.pdf",
		Size:     11,
		MIMEType: "application/pdf",
	})
	if err == nil {
		t.Fatal("CreateAttachmentUpload accepted attachment over domain limit")
	}
	if repo.lastAttachmentUpload.Filename != "" {
		t.Fatalf("metadata should not be recorded: %+v", repo.lastAttachmentUpload)
	}
	if repo.lastDomainPolicyUserID != "user-1" {
		t.Fatalf("domain policy user = %q", repo.lastDomainPolicyUserID)
	}
}

func TestDomainPolicyNormalizesDomainID(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{}
	service := New(repo, nil)

	policy, err := service.domainPolicy(context.Background(), " domain-1 ")
	if err != nil {
		t.Fatalf("domainPolicy returned error: %v", err)
	}
	if policy.DomainID != "domain-1" || repo.lastDomainPolicyID != "domain-1" {
		t.Fatalf("domain policy = %+v last id = %q", policy, repo.lastDomainPolicyID)
	}
}

func TestCancelAttachmentUploadDeletesStoredObject(t *testing.T) {
	t.Parallel()

	store := storage.NewLocalStore(t.TempDir())
	if err := store.Put(context.Background(), "uploads/user-1/upload-1/report.pdf", strings.NewReader("content")); err != nil {
		t.Fatalf("Put returned error: %v", err)
	}
	repo := &fakeRepository{
		canceledAttachment: maildb.Attachment{
			ID:          "att-1",
			StoragePath: "uploads/user-1/upload-1/report.pdf",
			Status:      "deleted",
		},
	}
	service := New(repo, store)

	attachment, err := service.CancelAttachmentUpload(context.Background(), " user-1 ", " att-1 ")
	if err != nil {
		t.Fatalf("CancelAttachmentUpload returned error: %v", err)
	}
	if attachment.ID != "att-1" || repo.lastCancelAttachmentUserID != "user-1" || repo.lastCancelAttachmentID != "att-1" {
		t.Fatalf("attachment = %+v repo user/id = %q/%q", attachment, repo.lastCancelAttachmentUserID, repo.lastCancelAttachmentID)
	}
	body, err := store.Get(context.Background(), "uploads/user-1/upload-1/report.pdf")
	if err == nil {
		_ = body.Close()
		t.Fatal("canceled attachment object still exists")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Get returned %v, want os.ErrNotExist", err)
	}
}

func TestCancelAttachmentUploadRejectsUnsafeStoredBodyPath(t *testing.T) {
	t.Parallel()

	store := &recordingStore{}
	repo := &fakeRepository{
		canceledAttachment: maildb.Attachment{
			ID:          "att-1",
			StoragePath: "uploads/../report.pdf",
			Status:      "deleted",
		},
	}
	service := New(repo, store)

	if _, err := service.CancelAttachmentUpload(context.Background(), "user-1", "att-1"); err == nil {
		t.Fatal("CancelAttachmentUpload accepted unsafe stored body path")
	}
	if store.deletePath != "" {
		t.Fatalf("store.Delete was called with %q", store.deletePath)
	}
}

func TestCancelAttachmentUploadValidatesResourceIDs(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{}
	service := New(repo, nil)

	tests := []struct {
		name         string
		userID       string
		attachmentID string
	}{
		{name: "unsafe user", userID: "user\n1", attachmentID: "att-1"},
		{name: "unsafe attachment", userID: "user-1", attachmentID: "att\n1"},
		{name: "oversized user", userID: strings.Repeat("u", maxServiceResourceIDBytes+1), attachmentID: "att-1"},
		{name: "oversized attachment", userID: "user-1", attachmentID: strings.Repeat("a", maxServiceResourceIDBytes+1)},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if _, err := service.CancelAttachmentUpload(context.Background(), tt.userID, tt.attachmentID); err == nil {
				t.Fatal("CancelAttachmentUpload accepted unsafe resource IDs")
			}
			if repo.lastCancelAttachmentID != "" {
				t.Fatalf("repository was called with attachment %q", repo.lastCancelAttachmentID)
			}
		})
	}
}

func TestCreateAttachmentUploadSessionDelegatesToRepository(t *testing.T) {
	t.Parallel()

	expiresAt := time.Now().Add(time.Hour)
	repo := &fakeRepository{}
	service := New(repo, nil)
	session, err := service.CreateAttachmentUploadSession(context.Background(), CreateAttachmentUploadSessionRequest{
		UserID:       " user-1 ",
		DraftID:      " draft-1 ",
		Filename:     " large.bin ",
		DeclaredSize: 42,
		MIMEType:     " application/octet-stream ",
		ExpiresAt:    expiresAt,
	})
	if err != nil {
		t.Fatalf("CreateAttachmentUploadSession returned error: %v", err)
	}
	if session.ID != "session-1" ||
		repo.lastAttachmentUploadSession.UserID != "user-1" ||
		repo.lastAttachmentUploadSession.DraftID != "draft-1" ||
		repo.lastAttachmentUploadSession.Filename != "large.bin" ||
		repo.lastAttachmentUploadSession.DeclaredSize != 42 ||
		repo.lastAttachmentUploadSession.MIMEType != "application/octet-stream" ||
		!repo.lastAttachmentUploadSession.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("session = %+v last = %+v", session, repo.lastAttachmentUploadSession)
	}
}

func TestCreateAttachmentUploadSessionRejectsDomainAttachmentLimit(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{domainPolicy: maildb.DomainPolicyView{
		DomainID:           "domain-1",
		InboundMode:        "inherit",
		OutboundMode:       "enforce",
		MaxAttachmentBytes: 10,
	}}
	service := New(repo, nil)
	_, err := service.CreateAttachmentUploadSession(context.Background(), CreateAttachmentUploadSessionRequest{
		UserID:       "user-1",
		Filename:     "large.bin",
		DeclaredSize: 11,
		MIMEType:     "application/octet-stream",
		ExpiresAt:    time.Now().Add(time.Hour),
	})
	if err == nil {
		t.Fatal("CreateAttachmentUploadSession accepted attachment over domain limit")
	}
	if repo.lastAttachmentUploadSession.Filename != "" {
		t.Fatalf("session should not be recorded: %+v", repo.lastAttachmentUploadSession)
	}
}

func TestCreateAttachmentUploadSessionRejectsExpiredSession(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{}
	service := New(repo, nil)
	_, err := service.CreateAttachmentUploadSession(context.Background(), CreateAttachmentUploadSessionRequest{
		UserID:       "user-1",
		Filename:     "large.bin",
		DeclaredSize: 42,
		MIMEType:     "application/octet-stream",
		ExpiresAt:    time.Now().Add(-time.Minute),
	})
	if err == nil {
		t.Fatal("CreateAttachmentUploadSession accepted expired session")
	}
	if repo.lastAttachmentUploadSession.Filename != "" {
		t.Fatalf("session should not be recorded: %+v", repo.lastAttachmentUploadSession)
	}
}

func TestCreateAttachmentUploadSessionRejectsOverlongSessionTTL(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{}
	service := New(repo, nil)
	_, err := service.CreateAttachmentUploadSession(context.Background(), CreateAttachmentUploadSessionRequest{
		UserID:       "user-1",
		Filename:     "large.bin",
		DeclaredSize: 42,
		MIMEType:     "application/octet-stream",
		ExpiresAt:    time.Now().Add(MaxAttachmentUploadSessionTTL + time.Hour),
	})
	if err == nil {
		t.Fatal("CreateAttachmentUploadSession accepted overlong session TTL")
	}
	if repo.lastAttachmentUploadSession.Filename != "" {
		t.Fatalf("session should not be recorded: %+v", repo.lastAttachmentUploadSession)
	}
}

func TestCancelAttachmentUploadSessionDelegatesToRepository(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{}
	service := New(repo, nil)
	session, err := service.CancelAttachmentUploadSession(context.Background(), " user-1 ", " session-1 ")
	if err != nil {
		t.Fatalf("CancelAttachmentUploadSession returned error: %v", err)
	}
	if session.ID != "session-1" || repo.lastCancelUploadSession.UserID != "user-1" || repo.lastCancelUploadSession.SessionID != "session-1" {
		t.Fatalf("session = %+v last = %+v", session, repo.lastCancelUploadSession)
	}
}

func TestCancelAttachmentUploadSessionDeletesStoredBody(t *testing.T) {
	t.Parallel()

	store := storage.NewLocalStore(t.TempDir())
	path := "upload-sessions/user-1/session-1/body"
	if err := store.Put(context.Background(), path, strings.NewReader("content")); err != nil {
		t.Fatalf("Put returned error: %v", err)
	}
	repo := &fakeRepository{
		uploadSession: maildb.AttachmentUploadSession{
			ID:          "session-1",
			UserID:      "user-1",
			StoragePath: path,
			Status:      "canceled",
		},
	}
	service := New(repo, store)
	session, err := service.CancelAttachmentUploadSession(context.Background(), " user-1 ", " session-1 ")
	if err != nil {
		t.Fatalf("CancelAttachmentUploadSession returned error: %v", err)
	}
	if session.ID != "session-1" || repo.lastCancelUploadSession.UserID != "user-1" || repo.lastCancelUploadSession.SessionID != "session-1" {
		t.Fatalf("session = %+v last = %+v", session, repo.lastCancelUploadSession)
	}
	if _, err := store.Get(context.Background(), path); err == nil {
		t.Fatal("stored upload session body still exists after cancellation")
	}
}

func TestGetAttachmentUploadSessionDelegatesToRepository(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{}
	service := New(repo, nil)
	session, err := service.GetAttachmentUploadSession(context.Background(), " user-1 ", " session-1 ")
	if err != nil {
		t.Fatalf("GetAttachmentUploadSession returned error: %v", err)
	}
	if session.ID != "session-1" || repo.lastGetUploadSession.UserID != "user-1" || repo.lastGetUploadSession.SessionID != "session-1" {
		t.Fatalf("session = %+v last = %+v", session, repo.lastGetUploadSession)
	}
}

func TestStoreAttachmentUploadSessionBodyWritesStorageAndRecordsDigest(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{
		uploadSession: maildb.AttachmentUploadSession{
			ID:           "session-1",
			UserID:       "user-1",
			DeclaredSize: 7,
			Status:       "pending",
			ExpiresAt:    time.Now().Add(time.Hour),
		},
	}
	store := storage.NewLocalStore(t.TempDir())
	service := New(repo, store)
	wantChecksum := sha256.Sum256([]byte("content"))
	session, err := service.StoreAttachmentUploadSessionBody(context.Background(), StoreAttachmentUploadSessionBodyRequest{
		UserID:                 " user-1 ",
		SessionID:              " session-1 ",
		ExpectedChecksumSHA256: " " + hex.EncodeToString(wantChecksum[:]) + " ",
		Body:                   strings.NewReader("content"),
	})
	if err != nil {
		t.Fatalf("StoreAttachmentUploadSessionBody returned error: %v", err)
	}
	if session.Status != "uploading" ||
		repo.lastStoreUploadSessionBody.UserID != "user-1" ||
		repo.lastStoreUploadSessionBody.SessionID != "session-1" ||
		repo.lastStoreUploadSessionBody.ReceivedSize != 7 ||
		repo.lastStoreUploadSessionBody.ChecksumSHA256 != hex.EncodeToString(wantChecksum[:]) {
		t.Fatalf("session = %+v store request = %+v", session, repo.lastStoreUploadSessionBody)
	}
	body, err := store.Get(context.Background(), repo.lastStoreUploadSessionBody.StoragePath)
	if err != nil {
		t.Fatalf("Get stored body returned error: %v", err)
	}
	defer body.Close()
	raw, _ := io.ReadAll(body)
	if string(raw) != "content" {
		t.Fatalf("stored body = %q", raw)
	}
}

func TestStoreAttachmentUploadSessionBodyPreservesPreviousBodyOnRepositoryFailure(t *testing.T) {
	t.Parallel()

	oldChecksum := sha256.Sum256([]byte("oldbody"))
	oldPath := "upload-sessions/user-1/session-1/bodies/old"
	repo := &fakeRepository{
		uploadSession: maildb.AttachmentUploadSession{
			ID:             "session-1",
			UserID:         "user-1",
			DeclaredSize:   7,
			ReceivedSize:   7,
			StoragePath:    oldPath,
			ChecksumSHA256: hex.EncodeToString(oldChecksum[:]),
			Status:         "uploading",
			ExpiresAt:      time.Now().Add(time.Hour),
		},
		storeUploadSessionBodyErr: errors.New("database unavailable"),
	}
	store := storage.NewLocalStore(t.TempDir())
	if err := store.Put(context.Background(), oldPath, strings.NewReader("oldbody")); err != nil {
		t.Fatalf("Put old body returned error: %v", err)
	}
	service := New(repo, store)
	_, err := service.StoreAttachmentUploadSessionBody(context.Background(), StoreAttachmentUploadSessionBodyRequest{
		UserID:    "user-1",
		SessionID: "session-1",
		Body:      strings.NewReader("content"),
	})
	if err == nil {
		t.Fatal("StoreAttachmentUploadSessionBody returned nil error for repository failure")
	}
	if repo.lastStoreUploadSessionBody.StoragePath == "" || repo.lastStoreUploadSessionBody.StoragePath == oldPath {
		t.Fatalf("new body should be staged at a distinct path: %+v", repo.lastStoreUploadSessionBody)
	}
	if _, err := store.Get(context.Background(), repo.lastStoreUploadSessionBody.StoragePath); err == nil {
		t.Fatalf("failed staged body still exists at %s", repo.lastStoreUploadSessionBody.StoragePath)
	}
	body, err := store.Get(context.Background(), oldPath)
	if err != nil {
		t.Fatalf("previous body was removed after repository failure: %v", err)
	}
	defer body.Close()
	raw, _ := io.ReadAll(body)
	if string(raw) != "oldbody" {
		t.Fatalf("previous body = %q", raw)
	}
}

func TestStoreAttachmentUploadSessionBodyDeletesPreviousBodyAfterReplacement(t *testing.T) {
	t.Parallel()

	oldChecksum := sha256.Sum256([]byte("oldbody"))
	oldPath := "upload-sessions/user-1/session-1/bodies/old"
	repo := &fakeRepository{
		uploadSession: maildb.AttachmentUploadSession{
			ID:             "session-1",
			UserID:         "user-1",
			DeclaredSize:   7,
			ReceivedSize:   7,
			StoragePath:    oldPath,
			ChecksumSHA256: hex.EncodeToString(oldChecksum[:]),
			Status:         "uploading",
			ExpiresAt:      time.Now().Add(time.Hour),
		},
	}
	store := storage.NewLocalStore(t.TempDir())
	if err := store.Put(context.Background(), oldPath, strings.NewReader("oldbody")); err != nil {
		t.Fatalf("Put old body returned error: %v", err)
	}
	service := New(repo, store)
	session, err := service.StoreAttachmentUploadSessionBody(context.Background(), StoreAttachmentUploadSessionBodyRequest{
		UserID:    "user-1",
		SessionID: "session-1",
		Body:      strings.NewReader("content"),
	})
	if err != nil {
		t.Fatalf("StoreAttachmentUploadSessionBody returned error: %v", err)
	}
	if session.StoragePath == oldPath || session.StoragePath == "" {
		t.Fatalf("replacement storage path = %q", session.StoragePath)
	}
	if _, err := store.Get(context.Background(), oldPath); err == nil {
		t.Fatal("previous body still exists after successful replacement")
	}
	body, err := store.Get(context.Background(), session.StoragePath)
	if err != nil {
		t.Fatalf("Get replacement body returned error: %v", err)
	}
	defer body.Close()
	raw, _ := io.ReadAll(body)
	if string(raw) != "content" {
		t.Fatalf("replacement body = %q", raw)
	}
}

func TestStoreAttachmentUploadSessionBodyRejectsChecksumMismatch(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{
		uploadSession: maildb.AttachmentUploadSession{
			ID:           "session-1",
			UserID:       "user-1",
			DeclaredSize: 7,
			Status:       "pending",
			ExpiresAt:    time.Now().Add(time.Hour),
		},
	}
	store := storage.NewLocalStore(t.TempDir())
	service := New(repo, store)
	_, err := service.StoreAttachmentUploadSessionBody(context.Background(), StoreAttachmentUploadSessionBodyRequest{
		UserID:                 "user-1",
		SessionID:              "session-1",
		ExpectedChecksumSHA256: strings.Repeat("0", 64),
		Body:                   strings.NewReader("content"),
	})
	if err == nil {
		t.Fatal("StoreAttachmentUploadSessionBody accepted checksum mismatch")
	}
	if repo.lastStoreUploadSessionBody.StoragePath != "" {
		t.Fatalf("mismatched body should not be recorded: %+v", repo.lastStoreUploadSessionBody)
	}
}

func TestStoreAttachmentUploadSessionBodyRejectsSizeMismatch(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{
		uploadSession: maildb.AttachmentUploadSession{
			ID:           "session-1",
			UserID:       "user-1",
			DeclaredSize: 8,
			Status:       "pending",
			ExpiresAt:    time.Now().Add(time.Hour),
		},
	}
	store := storage.NewLocalStore(t.TempDir())
	service := New(repo, store)
	_, err := service.StoreAttachmentUploadSessionBody(context.Background(), StoreAttachmentUploadSessionBodyRequest{
		UserID:    "user-1",
		SessionID: "session-1",
		Body:      strings.NewReader("content"),
	})
	if err == nil {
		t.Fatal("StoreAttachmentUploadSessionBody accepted size mismatch")
	}
	if repo.lastStoreUploadSessionBody.StoragePath != "" {
		t.Fatalf("body should not be recorded: %+v", repo.lastStoreUploadSessionBody)
	}
}

func TestStoreAttachmentUploadSessionBodyRejectsTerminalSession(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{
		uploadSession: maildb.AttachmentUploadSession{
			ID:           "session-1",
			UserID:       "user-1",
			DeclaredSize: 7,
			Status:       "finalized",
			ExpiresAt:    time.Now().Add(time.Hour),
		},
	}
	store := storage.NewLocalStore(t.TempDir())
	service := New(repo, store)
	_, err := service.StoreAttachmentUploadSessionBody(context.Background(), StoreAttachmentUploadSessionBodyRequest{
		UserID:    "user-1",
		SessionID: "session-1",
		Body:      strings.NewReader("content"),
	})
	if err == nil {
		t.Fatal("StoreAttachmentUploadSessionBody accepted terminal session")
	}
	if repo.lastStoreUploadSessionBody.StoragePath != "" {
		t.Fatalf("terminal body should not be recorded: %+v", repo.lastStoreUploadSessionBody)
	}
}

func TestFinalizeAttachmentUploadSessionDelegatesToRepository(t *testing.T) {
	t.Parallel()

	checksum := sha256.Sum256([]byte("content"))
	repo := &fakeRepository{
		uploadSession: maildb.AttachmentUploadSession{
			ID:             "session-1",
			UserID:         "user-1",
			DeclaredSize:   7,
			ReceivedSize:   7,
			StoragePath:    "upload-sessions/user-1/session-1/body",
			ChecksumSHA256: hex.EncodeToString(checksum[:]),
			Status:         "uploading",
			ExpiresAt:      time.Now().Add(time.Hour),
		},
	}
	store := storage.NewLocalStore(t.TempDir())
	if err := store.Put(context.Background(), "upload-sessions/user-1/session-1/body", strings.NewReader("content")); err != nil {
		t.Fatalf("Put returned error: %v", err)
	}
	service := New(repo, store)
	attachment, err := service.FinalizeAttachmentUploadSession(context.Background(), " user-1 ", " session-1 ")
	if err != nil {
		t.Fatalf("FinalizeAttachmentUploadSession returned error: %v", err)
	}
	if attachment.ID != "att-1" || repo.lastFinalizeUploadSession.UserID != "user-1" || repo.lastFinalizeUploadSession.SessionID != "session-1" {
		t.Fatalf("attachment = %+v finalize request = %+v", attachment, repo.lastFinalizeUploadSession)
	}
}

func TestFinalizeAttachmentUploadSessionRejectsMissingStoredBody(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{
		uploadSession: maildb.AttachmentUploadSession{
			ID:             "session-1",
			UserID:         "user-1",
			DeclaredSize:   7,
			ReceivedSize:   7,
			StoragePath:    "upload-sessions/user-1/session-1/body",
			ChecksumSHA256: strings.Repeat("a", 64),
			Status:         "uploading",
			ExpiresAt:      time.Now().Add(time.Hour),
		},
	}
	service := New(repo, storage.NewLocalStore(t.TempDir()))
	_, err := service.FinalizeAttachmentUploadSession(context.Background(), "user-1", "session-1")
	if err == nil {
		t.Fatal("FinalizeAttachmentUploadSession accepted missing stored body")
	}
	if repo.lastFinalizeUploadSession.SessionID != "" {
		t.Fatalf("finalize should not be recorded: %+v", repo.lastFinalizeUploadSession)
	}
}

func TestFinalizeAttachmentUploadSessionRejectsUnsafeStoredBodyPath(t *testing.T) {
	t.Parallel()

	checksum := sha256.Sum256([]byte("content"))
	for _, storagePath := range []string{
		"upload-sessions/user-1/../body",
		"upload-sessions//user-1/body",
		`upload-sessions\user-1\body`,
		"uploads/user-1/body",
	} {
		storagePath := storagePath
		t.Run(storagePath, func(t *testing.T) {
			t.Parallel()

			repo := &fakeRepository{
				uploadSession: maildb.AttachmentUploadSession{
					ID:             "session-1",
					UserID:         "user-1",
					DeclaredSize:   7,
					ReceivedSize:   7,
					StoragePath:    storagePath,
					ChecksumSHA256: hex.EncodeToString(checksum[:]),
					Status:         "uploading",
					ExpiresAt:      time.Now().Add(time.Hour),
				},
			}
			service := New(repo, storage.NewLocalStore(t.TempDir()))
			if _, err := service.FinalizeAttachmentUploadSession(context.Background(), "user-1", "session-1"); err == nil {
				t.Fatalf("FinalizeAttachmentUploadSession accepted unsafe path %q", storagePath)
			}
			if repo.lastFinalizeUploadSession.SessionID != "" {
				t.Fatalf("unsafe path reached finalize: %+v", repo.lastFinalizeUploadSession)
			}
		})
	}
}

func TestFinalizeAttachmentUploadSessionRejectsStoredChecksumMismatch(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{
		uploadSession: maildb.AttachmentUploadSession{
			ID:             "session-1",
			UserID:         "user-1",
			DeclaredSize:   7,
			ReceivedSize:   7,
			StoragePath:    "upload-sessions/user-1/session-1/body",
			ChecksumSHA256: strings.Repeat("a", 64),
			Status:         "uploading",
			ExpiresAt:      time.Now().Add(time.Hour),
		},
	}
	store := storage.NewLocalStore(t.TempDir())
	if err := store.Put(context.Background(), "upload-sessions/user-1/session-1/body", strings.NewReader("content")); err != nil {
		t.Fatalf("Put returned error: %v", err)
	}
	service := New(repo, store)
	_, err := service.FinalizeAttachmentUploadSession(context.Background(), "user-1", "session-1")
	if err == nil {
		t.Fatal("FinalizeAttachmentUploadSession accepted checksum mismatch")
	}
	if repo.lastFinalizeUploadSession.SessionID != "" {
		t.Fatalf("finalize should not be recorded: %+v", repo.lastFinalizeUploadSession)
	}
}

func TestExpireAttachmentUploadSessionsDelegatesToRepository(t *testing.T) {
	t.Parallel()

	before := time.Now()
	repo := &fakeRepository{
		expiredUploadSessions: []maildb.AttachmentUploadSession{{ID: "session-1", Status: "expired"}},
	}
	service := New(repo, nil)
	sessions, err := service.ExpireAttachmentUploadSessions(context.Background(), before, 7)
	if err != nil {
		t.Fatalf("ExpireAttachmentUploadSessions returned error: %v", err)
	}
	if len(sessions) != 1 || sessions[0].ID != "session-1" {
		t.Fatalf("sessions = %+v", sessions)
	}
	if !repo.lastExpireUploadSessions.Before.Equal(before) || repo.lastExpireUploadSessions.Limit != 7 {
		t.Fatalf("expire request = %+v", repo.lastExpireUploadSessions)
	}
}

func TestCountStaleAttachmentUploadSessionsDelegatesToRepository(t *testing.T) {
	t.Parallel()

	before := time.Now()
	repo := &fakeRepository{
		staleUploadSessionCount: maildb.StaleAttachmentUploadSessionCount{
			TotalCount:   9,
			LimitedCount: 4,
		},
	}
	service := New(repo, nil)
	counts, err := service.CountStaleAttachmentUploadSessions(context.Background(), before, 4)
	if err != nil {
		t.Fatalf("CountStaleAttachmentUploadSessions returned error: %v", err)
	}
	if counts.TotalCount != 9 || counts.LimitedCount != 4 {
		t.Fatalf("counts = %+v", counts)
	}
	if !repo.lastUploadSessionCleanupCount.Before.Equal(before) || repo.lastUploadSessionCleanupCount.Limit != 4 {
		t.Fatalf("count request = %+v", repo.lastUploadSessionCleanupCount)
	}
}

func TestListStaleAttachmentUploadSessionsDelegatesToRepository(t *testing.T) {
	t.Parallel()

	before := time.Now()
	repo := &fakeRepository{
		staleUploadSessionCandidates: []maildb.StaleAttachmentUploadSessionCandidate{{
			ID:     "session-1",
			UserID: "user-1",
			Status: "uploading",
		}},
	}
	service := New(repo, nil)
	candidates, err := service.ListStaleAttachmentUploadSessions(context.Background(), before, 6)
	if err != nil {
		t.Fatalf("ListStaleAttachmentUploadSessions returned error: %v", err)
	}
	if len(candidates) != 1 || candidates[0].ID != "session-1" {
		t.Fatalf("candidates = %+v", candidates)
	}
	if !repo.lastUploadSessionCleanupList.Before.Equal(before) || repo.lastUploadSessionCleanupList.Limit != 6 {
		t.Fatalf("list request = %+v", repo.lastUploadSessionCleanupList)
	}
}

func TestExpireAttachmentUploadSessionsDeletesStoredBodies(t *testing.T) {
	t.Parallel()

	store := storage.NewLocalStore(t.TempDir())
	path := "upload-sessions/user-1/session-1/body"
	if err := store.Put(context.Background(), path, strings.NewReader("content")); err != nil {
		t.Fatalf("Put returned error: %v", err)
	}
	before := time.Now()
	repo := &fakeRepository{
		expiredUploadSessions: []maildb.AttachmentUploadSession{{ID: "session-1", UserID: "user-1", StoragePath: path, Status: "expired"}},
	}
	service := New(repo, store)
	sessions, err := service.ExpireAttachmentUploadSessions(context.Background(), before, 7)
	if err != nil {
		t.Fatalf("ExpireAttachmentUploadSessions returned error: %v", err)
	}
	if len(sessions) != 1 || sessions[0].ID != "session-1" {
		t.Fatalf("sessions = %+v", sessions)
	}
	if _, err := store.Get(context.Background(), path); err == nil {
		t.Fatal("stored upload session body still exists after expiry")
	}
}

func TestUploadAttachmentWritesStorageAndRecordsMetadata(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{}
	store := storage.NewLocalStore(t.TempDir())
	service := New(repo, store)

	attachment, err := service.UploadAttachment(context.Background(), UploadAttachmentRequest{
		UserID:   " user-1 ",
		DraftID:  " draft-1 ",
		Filename: " report.pdf ",
		Size:     7,
		MIMEType: " application/pdf ",
		Body:     strings.NewReader("content"),
	})
	if err != nil {
		t.Fatalf("UploadAttachment returned error: %v", err)
	}
	if attachment.ID != "att-1" {
		t.Fatalf("attachment = %+v", attachment)
	}
	if repo.lastAttachmentUpload.StoragePath == "" {
		t.Fatal("StoragePath was not recorded")
	}
	if repo.lastAttachmentUpload.UserID != "user-1" ||
		repo.lastAttachmentUpload.DraftID != "draft-1" ||
		repo.lastAttachmentUpload.Filename != "report.pdf" ||
		repo.lastAttachmentUpload.MIMEType != "application/pdf" {
		t.Fatalf("lastAttachmentUpload = %+v", repo.lastAttachmentUpload)
	}
	body, err := store.Get(context.Background(), repo.lastAttachmentUpload.StoragePath)
	if err != nil {
		t.Fatalf("stored attachment missing: %v", err)
	}
	defer body.Close()
	raw, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("ReadAll returned error: %v", err)
	}
	if string(raw) != "content" {
		t.Fatalf("stored body = %q", raw)
	}
}

func TestUploadAttachmentRejectsDomainAttachmentLimitBeforeStorageWrite(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{domainPolicy: maildb.DomainPolicyView{
		DomainID:           "domain-1",
		InboundMode:        "inherit",
		OutboundMode:       "enforce",
		MaxAttachmentBytes: 4,
	}}
	store := storage.NewLocalStore(t.TempDir())
	service := New(repo, store)

	_, err := service.UploadAttachment(context.Background(), UploadAttachmentRequest{
		UserID:   "user-1",
		Filename: "report.pdf",
		Size:     7,
		MIMEType: "application/pdf",
		Body:     strings.NewReader("content"),
	})
	if err == nil {
		t.Fatal("UploadAttachment accepted attachment over domain limit")
	}
	if repo.lastAttachmentUpload.Filename != "" {
		t.Fatalf("metadata should not be recorded: %+v", repo.lastAttachmentUpload)
	}
}

func TestUploadAttachmentRejectsUnsafeUserStorageSegment(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{}
	service := New(repo, storage.NewLocalStore(t.TempDir()))

	_, err := service.UploadAttachment(context.Background(), UploadAttachmentRequest{
		UserID:   "../user\n1",
		Filename: "report.pdf",
		Size:     7,
		MIMEType: "application/pdf",
		Body:     strings.NewReader("content"),
	})
	if err == nil {
		t.Fatal("UploadAttachment accepted unsafe user_id")
	}
}

func TestUploadAttachmentRejectsBodyLargerThanLimit(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{}
	store := storage.NewLocalStore(t.TempDir())
	service := New(repo, store)

	_, err := service.UploadAttachment(context.Background(), UploadAttachmentRequest{
		UserID:   "user-1",
		Filename: "large.bin",
		Size:     1,
		MIMEType: "application/octet-stream",
		Body:     bytes.NewReader(bytes.Repeat([]byte("x"), int(MaxAttachmentUploadBytes)+1)),
	})
	if err == nil {
		t.Fatal("UploadAttachment accepted oversized body")
	}
	if repo.lastAttachmentUpload.Filename != "" {
		t.Fatalf("metadata should not be recorded: %+v", repo.lastAttachmentUpload)
	}
}

func TestUploadAttachmentRejectsDeclaredSizeMismatch(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{}
	store := storage.NewLocalStore(t.TempDir())
	service := New(repo, store)

	_, err := service.UploadAttachment(context.Background(), UploadAttachmentRequest{
		UserID:   "user-1",
		Filename: "report.pdf",
		Size:     99,
		MIMEType: "application/pdf",
		Body:     strings.NewReader("content"),
	})
	if err == nil {
		t.Fatal("UploadAttachment accepted mismatched declared size")
	}
	if repo.lastAttachmentUpload.Filename != "" {
		t.Fatalf("metadata should not be recorded: %+v", repo.lastAttachmentUpload)
	}
}
