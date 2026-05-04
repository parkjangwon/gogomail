package searchindex

import (
	"context"
	"testing"

	"github.com/gogomail/gogomail/internal/maildb"
)

func TestPostgresIndexerMapsBodyDocument(t *testing.T) {
	t.Parallel()

	writer := &fakePostgresWriter{}
	indexer := NewPostgresIndexer(writer)
	err := indexer.IndexMessage(context.Background(), Document{
		MessageID:     "msg-1",
		UserID:        "user-1",
		BodyText:      "search me",
		BodyTruncated: true,
	})
	if err != nil {
		t.Fatalf("IndexMessage returned error: %v", err)
	}

	if writer.doc.MessageID != "msg-1" || writer.doc.UserID != "user-1" {
		t.Fatalf("mapped doc identity = %+v", writer.doc)
	}
	if writer.doc.BodyText != "search me" || !writer.doc.BodyTextTruncated {
		t.Fatalf("mapped doc body = %+v", writer.doc)
	}
}

type fakePostgresWriter struct {
	doc maildb.MessageSearchDocument
}

func (w *fakePostgresWriter) UpsertMessageSearchDocument(_ context.Context, doc maildb.MessageSearchDocument) error {
	w.doc = doc
	return nil
}
