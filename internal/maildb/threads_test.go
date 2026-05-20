package maildb

import (
	"strings"
	"testing"
)

func TestThreadSummaryJSONFieldsAreStable(t *testing.T) {
	t.Parallel()

	thread := ThreadSummary{
		ID:              "thread-1",
		Subject:         "hello",
		Preview:         "body preview",
		MessageCount:    2,
		UnreadCount:     1,
		LatestMessageID: "msg-2",
		LatestFromAddr:  "sender@example.net",
		HasAttachment:   true,
		Starred:         true,
	}
	if thread.ID == "" || thread.Preview == "" || thread.MessageCount != 2 || !thread.HasAttachment || !thread.Starred {
		t.Fatalf("thread = %+v", thread)
	}
}

func TestThreadListSQLUsesLatestMessagePreview(t *testing.T) {
	t.Parallel()

	for name, query := range map[string]string{
		"newest": threadListPageNewestSQL,
		"oldest": threadListPageOldestSQL,
	} {
		name := name
		query := query
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			for _, want := range []string{
				"LEFT JOIN message_search_documents msd",
				"left(btrim(regexp_replace(left(coalesce(msd.body_text, ''), 2000), '[[:space:]]+', ' ', 'g')), 280) AS preview",
				"(array_agg(preview ORDER BY message_at DESC, id DESC))[1] AS preview",
				"SELECT\n  thread_key,\n  subject,\n  preview,\n  message_count,\n  unread_count,\n  latest_message_id,\n  latest_from_addr,\n  latest_at,\n  has_attachment,\n  starred\nFROM thread_summaries",
			} {
				if !strings.Contains(query, want) {
					t.Fatalf("thread list query does not include %q:\n%s", want, query)
				}
			}
			if strings.Contains(query, "SELECT *\nFROM thread_summaries") {
				t.Fatalf("thread list query still projects all thread summary columns:\n%s", query)
			}
		})
	}
}

func TestThreadMessagesSQLUsesIndexedThreadMatch(t *testing.T) {
	t.Parallel()

	for _, want := range []string{
		"messages.thread_id = $2::uuid",
		"messages.id = $2::uuid",
		"ORDER BY message_at ASC, id ASC",
	} {
		if !strings.Contains(threadMessagesPageSQL, want) {
			t.Fatalf("threadMessagesPageSQL does not include %q:\n%s", want, threadMessagesPageSQL)
		}
	}
	if strings.Contains(threadMessagesPageSQL, "COALESCE(messages.thread_id, messages.id)::text = $2") {
		t.Fatalf("threadMessagesPageSQL still uses COALESCE thread match:\n%s", threadMessagesPageSQL)
	}
}
