package searchindex

import (
	"context"
	"fmt"

	"github.com/gogomail/gogomail/internal/maildb"
)

type PostgresDocumentWriter interface {
	UpsertMessageSearchDocument(ctx context.Context, doc maildb.MessageSearchDocument) error
}

type PostgresIndexer struct {
	writer PostgresDocumentWriter
}

func NewPostgresIndexer(writer PostgresDocumentWriter) PostgresIndexer {
	return PostgresIndexer{writer: writer}
}

func (i PostgresIndexer) IndexMessage(ctx context.Context, doc Document) error {
	if i.writer == nil {
		return fmt.Errorf("postgres search document writer is required")
	}
	return i.writer.UpsertMessageSearchDocument(ctx, maildb.MessageSearchDocument{
		MessageID:         doc.MessageID,
		UserID:            doc.UserID,
		BodyText:          doc.BodyText,
		BodyTextTruncated: doc.BodyTruncated,
	})
}
