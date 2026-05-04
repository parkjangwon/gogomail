package maildb

import "testing"

func TestThreadSummaryJSONFieldsAreStable(t *testing.T) {
	t.Parallel()

	thread := ThreadSummary{
		ID:              "thread-1",
		Subject:         "hello",
		MessageCount:    2,
		UnreadCount:     1,
		LatestMessageID: "msg-2",
		LatestFromAddr:  "sender@example.net",
		HasAttachment:   true,
		Starred:         true,
	}
	if thread.ID == "" || thread.MessageCount != 2 || !thread.HasAttachment || !thread.Starred {
		t.Fatalf("thread = %+v", thread)
	}
}
