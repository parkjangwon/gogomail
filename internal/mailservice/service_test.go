package mailservice

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/imapgw"
	"github.com/gogomail/gogomail/internal/maildb"
	"github.com/gogomail/gogomail/internal/outbound"
	"github.com/gogomail/gogomail/internal/searchindex"
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

func TestGetMessageMarksUnreadMessageRead(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{
		detail: maildb.MessageDetail{
			ID:    "msg-1",
			Flags: []byte(`{"read":false}`),
		},
	}
	service := New(repo, nil)
	if _, err := service.GetMessage(context.Background(), "user-1", "msg-1"); err != nil {
		t.Fatalf("GetMessage returned error: %v", err)
	}
	if repo.lastFlagMessageID != "msg-1" || repo.lastFlag != "read" {
		t.Fatalf("read marker = %q/%q", repo.lastFlagMessageID, repo.lastFlag)
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
	if _, err := service.ListMessagesPage(context.Background(), " user-1 ", " inbox ", 10, maildb.MessageListCursor{ID: "cursor"}); err != nil {
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
	_, _ = service.ListMessagesPage(context.Background(), "user-1", "inbox", -1, maildb.MessageListCursor{})
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
		MailboxID: " inbox ",
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

func TestListIMAPMessagesDelegatesToRepository(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{
		imapMessages: []imapgw.MessageSummary{{ID: "msg-1", MailboxID: "inbox", UID: 12}},
	}
	service := New(repo, nil)

	got, err := service.ListIMAPMessages(context.Background(), imapgw.ListMessagesRequest{
		UserID:    " user-1 ",
		MailboxID: " inbox ",
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

func TestBackfillIMAPMailboxUIDsDelegatesToRepository(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{
		backfilledIMAPUIDs: []maildb.IMAPMessageUID{{MessageID: "msg-1", MailboxID: "inbox", UID: 12, ModSeq: 2}},
	}
	service := New(repo, nil)

	got, err := service.BackfillIMAPMailboxUIDs(context.Background(), " user-1 ", " inbox ", 500)
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

func TestStoreIMAPFlagsDelegatesToRepository(t *testing.T) {
	t.Parallel()

	events := &fakeIMAPEventPublisher{}
	repo := &fakeRepository{
		imapFlagSummaries: []imapgw.MessageSummary{{ID: "msg-1", MailboxID: "inbox", UID: 12}},
	}
	service := New(repo, nil).WithIMAPMailboxEvents(events)

	got, err := service.StoreIMAPFlags(context.Background(), imapgw.StoreFlagsRequest{
		UserID:    " user-1 ",
		MailboxID: " inbox ",
		UIDs:      []imapgw.UID{12},
		Flags:     imapgw.MessageFlags{Read: true},
		Mode:      imapgw.StoreFlagsAdd,
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
	if repo.lastIMAPFlagUserID != "user-1" || repo.lastIMAPFlagMailboxID != "inbox" {
		t.Fatalf("store flags ids = %q/%q", repo.lastIMAPFlagUserID, repo.lastIMAPFlagMailboxID)
	}
	if len(events.events) != 1 || events.events[0].Type != imapgw.MailboxEventFlags || events.events[0].UserID != "user-1" || events.events[0].MailboxID != "inbox" || events.events[0].UID != 12 {
		t.Fatalf("events = %#v, want flags event", events.events)
	}
}

func TestSetMessageFlagPublishesIMAPFlagEvent(t *testing.T) {
	t.Parallel()

	events := &fakeIMAPEventPublisher{}
	repo := &fakeRepository{
		imapUIDs: []maildb.IMAPMessageUID{{MessageID: "msg-1", MailboxID: "inbox", UID: 12, ModSeq: 2}},
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

func TestMoveMessagePublishesIMAPExpungeEvent(t *testing.T) {
	t.Parallel()

	events := &fakeIMAPEventPublisher{}
	repo := &fakeRepository{
		imapUIDs: []maildb.IMAPMessageUID{{MessageID: "msg-1", MailboxID: "inbox", UID: 12, ModSeq: 2}},
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
	if len(events.events) != 1 || events.events[0].Type != imapgw.MailboxEventExpunge || events.events[0].MailboxID != "inbox" || events.events[0].UID != 12 {
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
			{MessageID: "msg-1", MailboxID: "inbox", UID: 12, ModSeq: 2},
			{MessageID: "msg-2", MailboxID: "inbox", UID: 13, ModSeq: 3},
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
	if len(events.events) != 2 || events.events[0].Type != imapgw.MailboxEventExpunge || events.events[0].UID != 12 || events.events[1].UID != 13 {
		t.Fatalf("events = %#v, want two expunge events", events.events)
	}
}

func TestDeleteMessagePublishesIMAPExpungeEvent(t *testing.T) {
	t.Parallel()

	events := &fakeIMAPEventPublisher{}
	repo := &fakeRepository{
		imapUIDs: []maildb.IMAPMessageUID{{MessageID: "msg-1", MailboxID: "inbox", UID: 12, ModSeq: 2}},
	}
	service := New(repo, nil).WithIMAPMailboxEvents(events)

	if err := service.DeleteMessage(context.Background(), " user-1 ", " msg-1 "); err != nil {
		t.Fatalf("DeleteMessage returned error: %v", err)
	}
	if repo.lastMutationUserID != "user-1" || repo.lastDeleteMessageID != "msg-1" {
		t.Fatalf("delete mutation = %q/%q", repo.lastMutationUserID, repo.lastDeleteMessageID)
	}
	if len(events.events) != 1 || events.events[0].Type != imapgw.MailboxEventExpunge || events.events[0].MailboxID != "inbox" || events.events[0].UID != 12 {
		t.Fatalf("events = %#v, want expunge event", events.events)
	}
}

func TestBulkDeleteMessagesPublishesIMAPExpungeEvents(t *testing.T) {
	t.Parallel()

	events := &fakeIMAPEventPublisher{}
	repo := &fakeRepository{
		imapUIDs: []maildb.IMAPMessageUID{
			{MessageID: "msg-1", MailboxID: "inbox", UID: 12, ModSeq: 2},
			{MessageID: "msg-2", MailboxID: "inbox", UID: 13, ModSeq: 3},
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
	if len(events.events) != 2 || events.events[0].Type != imapgw.MailboxEventExpunge || events.events[0].UID != 12 || events.events[1].UID != 13 {
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
	imapUIDs                       []maildb.IMAPMessageUID
	imapMailboxes                  []imapgw.Mailbox
	imapMessages                   []imapgw.MessageSummary
	backfilledIMAPUIDs             []maildb.IMAPMessageUID
	attachments                    []maildb.Attachment
	list                           []maildb.MessageSummary
	messagesByID                   []maildb.MessageSummary
	suppressed                     []string
	domainPolicy                   maildb.DomainPolicyView
	sourceThread                   maildb.SourceThreadView
	seenSuppressionRecipients      []string
	lastDomainPolicyID             string
	lastDomainPolicyUserID         string
	lastDraft                      maildb.SaveDraftRequest
	lastAttachmentUpload           maildb.CreateAttachmentUploadRequest
	lastAttachmentCleanup          maildb.ExpireStaleAttachmentUploadsRequest
	lastAttachmentUserID           string
	lastAttachmentMessageID        string
	lastAttachmentID               string
	attachment                     maildb.Attachment
	expiredAttachments             []maildb.Attachment
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
	lastListThreadsUserID          string
	lastListThreadsLimit           int
	lastThreadMessagesUserID       string
	lastThreadID                   string
	lastThreadMessagesLimit        int
	lastGetMessageUserID           string
	lastGetMessageID               string
	lastSearchQuery                maildb.MessageSearchQuery
	lastFlagMessageID              string
	lastFlag                       string
	lastMutationUserID             string
	lastMoveMessageID              string
	lastMoveFolderID               string
	lastDeleteMessageID            string
	lastBulkFlag                   maildb.BulkMessageFlagRequest
	lastBulkMove                   maildb.BulkMessageMoveRequest
	lastBulkDelete                 maildb.BulkMessageDeleteRequest
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
	lastIMAPFlagUserID             string
	lastIMAPFlagMailboxID          string
	lastIMAPUIDLookupUserID        string
	lastIMAPUIDLookupMessageIDs    []string
	lastIMAPMailboxUserID          string
	lastIMAPMessageUserID          string
	lastIMAPMessageMailboxID       string
	lastIMAPMessageLimit           int
	lastIMAPMessageAfterUID        imapgw.UID
	lastBackfillUserID             string
	lastBackfillMailboxID          string
	lastBackfillLimit              int
	recordErr                      error
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

func (f *fakeRepository) ListMessagesPage(_ context.Context, userID string, folderID string, limit int, cursor maildb.MessageListCursor) ([]maildb.MessageSummary, error) {
	f.lastPageUserID = userID
	f.lastPageFolderID = folderID
	f.lastPageLimit = limit
	f.lastPageCursor = cursor
	return []maildb.MessageSummary{{ID: "msg-page"}}, nil
}

func (f *fakeRepository) SearchMessages(_ context.Context, query maildb.MessageSearchQuery) ([]maildb.MessageSummary, error) {
	f.lastSearchQuery = query
	return f.list, nil
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

func (f *fakeRepository) ListThreadMessages(_ context.Context, userID string, threadID string, limit int) ([]maildb.MessageSummary, error) {
	f.lastThreadMessagesUserID = userID
	f.lastThreadID = threadID
	f.lastThreadMessagesLimit = limit
	return nil, nil
}

func (f *fakeRepository) ListFolders(_ context.Context, userID string) ([]maildb.Folder, error) {
	f.lastListFoldersUserID = userID
	return nil, nil
}

func (f *fakeRepository) CreateFolder(_ context.Context, req maildb.CreateFolderRequest) (maildb.Folder, error) {
	f.lastCreateFolder = req
	return maildb.Folder{}, nil
}

func (f *fakeRepository) RenameFolder(_ context.Context, userID string, folderID string, name string) (maildb.Folder, error) {
	f.lastRenameFolderUserID = userID
	f.lastRenameFolderID = folderID
	f.lastRenameFolderName = name
	return maildb.Folder{}, nil
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

func (f *fakeRepository) GetIMAPMailbox(_ context.Context, userID string, mailboxID string) (imapgw.Mailbox, error) {
	f.lastIMAPMailboxUserID = userID
	f.lastIMAPMessageMailboxID = mailboxID
	if len(f.imapMailboxes) == 0 {
		return imapgw.Mailbox{}, nil
	}
	return f.imapMailboxes[0], nil
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

func (f *fakeRepository) StoreIMAPFlags(_ context.Context, userID string, mailboxID string, _ []imapgw.UID, flags imapgw.MessageFlags, mode imapgw.StoreFlagsMode) ([]imapgw.MessageSummary, error) {
	f.lastIMAPFlagUserID = userID
	f.lastIMAPFlagMailboxID = mailboxID
	f.lastIMAPFlags = flags
	f.lastIMAPFlagMode = mode
	return f.imapFlagSummaries, nil
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

func (f *fakeRepository) DeleteMessage(_ context.Context, userID string, messageID string) error {
	f.lastMutationUserID = userID
	f.lastDeleteMessageID = messageID
	return nil
}

func (f *fakeRepository) BulkDeleteMessages(_ context.Context, req maildb.BulkMessageDeleteRequest) (int64, error) {
	f.lastBulkDelete = req
	return int64(len(req.MessageIDs)), nil
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
		AttachmentIDs: []string{"att-1"},
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

func (f *fakeRepository) ExpireStaleAttachmentUploads(_ context.Context, req maildb.ExpireStaleAttachmentUploadsRequest) ([]maildb.Attachment, error) {
	f.lastAttachmentCleanup = req
	return f.expiredAttachments, nil
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

func TestSendDraftSendsAndMarksDraftSent(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{}
	service := New(repo, storage.NewLocalStore(t.TempDir()))
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
		AttachmentIDs:   []string{" att-1 "},
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
		repo.lastDraft.AttachmentIDs[0] != "att-1" ||
		repo.lastDraft.Subject != "draft" {
		t.Fatalf("draft = %+v last = %+v", draft, repo.lastDraft)
	}
}

func TestListMessagesPageDelegatesCursor(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{}
	service := New(repo, nil)
	cursor := maildb.MessageListCursor{ID: "11111111-1111-1111-1111-111111111111"}
	messages, err := service.ListMessagesPage(context.Background(), "user-1", "", 10, cursor)
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

func TestUploadAttachmentSanitizesUserStorageSegment(t *testing.T) {
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
	if err != nil {
		t.Fatalf("UploadAttachment returned error: %v", err)
	}
	if !strings.HasPrefix(repo.lastAttachmentUpload.StoragePath, "uploads/user_1/") {
		t.Fatalf("StoragePath = %q, want sanitized user segment", repo.lastAttachmentUpload.StoragePath)
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
