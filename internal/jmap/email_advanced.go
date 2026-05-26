package jmap

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/textproto"
	"strings"
)

// emailCopyMethod implements Email/copy (RFC 8621 §4.7).
// Full cross-account implementation requires imap_append integration;
// returns notCreated for all items in this implementation.
type emailCopyMethod struct{ deps Deps }

func (m *emailCopyMethod) Call(ctx context.Context, userID string, args json.RawMessage) (json.RawMessage, error) {
	var req struct {
		FromAccountID string                      `json:"fromAccountId"`
		AccountID     string                      `json:"accountId"`
		Create        map[string]EmailCreatePatch `json:"create"`
	}
	if err := json.Unmarshal(args, &req); err != nil {
		return errorResult(ErrInvalidArguments), nil
	}
	if req.AccountID == "" {
		req.AccountID = userID
	}

	resp := SetResponse{
		AccountID:    req.AccountID,
		NotCreated:   make(map[string]SetError),
		NotUpdated:   make(map[string]SetError),
		NotDestroyed: make(map[string]SetError),
	}
	// Stub: Email/copy requires imap_append integration not yet wired.
	for clientID := range req.Create {
		resp.NotCreated[clientID] = SetError{
			Type:        "notFound",
			Description: "Email/copy: source message lookup not implemented",
		}
	}
	return json.Marshal(resp)
}

// EmailImportArgs is the argument object for Email/import (RFC 8621 §4.9).
type EmailImportArgs struct {
	AccountID string                     `json:"accountId"`
	Emails    map[string]EmailImportItem `json:"emails"`
}

// EmailImportItem describes a single email to import.
type EmailImportItem struct {
	BlobID     string          `json:"blobId"`
	MailboxIDs map[string]bool `json:"mailboxIds"`
	Keywords   map[string]bool `json:"keywords,omitempty"`
	ReceivedAt string          `json:"receivedAt,omitempty"`
}

// emailImportMethod implements Email/import (RFC 8621 §4.9).
type emailImportMethod struct{ deps Deps }

func (m *emailImportMethod) Call(ctx context.Context, userID string, args json.RawMessage) (json.RawMessage, error) {
	if m.deps.Repo == nil {
		return errorResult(ErrServerFail), nil
	}
	var req EmailImportArgs
	if err := json.Unmarshal(args, &req); err != nil {
		return errorResult(ErrInvalidArguments), nil
	}
	if req.AccountID == "" {
		req.AccountID = userID
	}

	state, _ := EmailStateFor(ctx, m.deps.Repo.DB(), userID)
	resp := SetResponse{
		AccountID:    req.AccountID,
		OldState:     state,
		NotCreated:   make(map[string]SetError),
		NotUpdated:   make(map[string]SetError),
		NotDestroyed: make(map[string]SetError),
	}

	for clientID, item := range req.Emails {
		if len(item.MailboxIDs) == 0 {
			resp.NotCreated[clientID] = SetError{Type: "invalidProperties", Description: "mailboxIds required"}
			continue
		}
		if item.BlobID == "" {
			resp.NotCreated[clientID] = SetError{Type: "invalidProperties", Description: "blobId required"}
			continue
		}

		// Look up blob in jmap_blobs.
		var storagePath string
		_ = m.deps.Repo.DB().QueryRowContext(ctx,
			`SELECT storage_path FROM jmap_blobs WHERE id=$1::uuid AND account_id=$2::uuid`,
			item.BlobID, userID,
		).Scan(&storagePath)

		if storagePath == "" {
			resp.NotCreated[clientID] = SetError{Type: "notFound", Description: "blobId not found in jmap_blobs"}
			continue
		}
		// Full imap_append integration pending.
		resp.NotCreated[clientID] = SetError{Type: "serverFail", Description: "blob-to-mailbox append pending imap_append integration"}
	}

	resp.NewState, _ = EmailStateFor(ctx, m.deps.Repo.DB(), userID)
	return json.Marshal(resp)
}

// EmailParseArgs is the argument object for Email/parse (RFC 8621 §4.10).
type EmailParseArgs struct {
	AccountID           string   `json:"accountId"`
	BlobIDs             []string `json:"blobIds"`
	Properties          []string `json:"properties,omitempty"`
	FetchTextBodyValues bool     `json:"fetchTextBodyValues,omitempty"`
}

// emailParseMethod implements Email/parse (RFC 8621 §4.10).
type emailParseMethod struct{ deps Deps }

func (m *emailParseMethod) Call(ctx context.Context, userID string, args json.RawMessage) (json.RawMessage, error) {
	if m.deps.Repo == nil {
		return errorResult(ErrServerFail), nil
	}
	var req EmailParseArgs
	if err := json.Unmarshal(args, &req); err != nil {
		return errorResult(ErrInvalidArguments), nil
	}
	if req.AccountID == "" {
		req.AccountID = userID
	}

	parsed := make(map[string]EmailObject)
	notParsable := []string{}
	notFound := []string{}

	for _, blobID := range req.BlobIDs {
		var storagePath string
		_ = m.deps.Repo.DB().QueryRowContext(ctx,
			`SELECT storage_path FROM jmap_blobs WHERE id=$1::uuid AND account_id=$2::uuid`,
			blobID, userID,
		).Scan(&storagePath)

		if storagePath == "" {
			notFound = append(notFound, blobID)
			continue
		}

		if m.deps.Store == nil {
			notParsable = append(notParsable, blobID)
			continue
		}

		reader, err := m.deps.Store.Get(ctx, storagePath)
		if err != nil {
			notParsable = append(notParsable, blobID)
			continue
		}
		raw, _ := io.ReadAll(reader)
		reader.Close()

		email, err := parseMIMEToJMAP(raw, req.Properties)
		if err != nil {
			notParsable = append(notParsable, blobID)
			continue
		}
		parsed[blobID] = email
	}

	return json.Marshal(map[string]any{
		"accountId":   req.AccountID,
		"parsed":      parsed,
		"notParsable": notParsable,
		"notFound":    notFound,
	})
}

// parseMIMEToJMAP parses a raw MIME email into a JMAP EmailObject.
func parseMIMEToJMAP(raw []byte, props []string) (EmailObject, error) {
	reader := textproto.NewReader(bufio.NewReader(bytes.NewReader(raw)))
	headers, err := reader.ReadMIMEHeader()
	if err != nil && !strings.Contains(err.Error(), "EOF") {
		return EmailObject{}, err
	}

	email := EmailObject{}
	email.Subject = headers.Get("Subject")

	if from := headers.Get("From"); from != "" {
		email.From = []EmailAddress{{Email: from}}
	}
	if to := headers.Get("To"); to != "" {
		email.To = []EmailAddress{{Email: to}}
	}

	ct := headers.Get("Content-Type")
	if ct == "" {
		ct = "text/plain"
	}
	mediaType, params, _ := mime.ParseMediaType(ct)

	if strings.HasPrefix(mediaType, "multipart/") {
		boundary := params["boundary"]
		if boundary != "" {
			mr := multipart.NewReader(bytes.NewReader(raw), boundary)
			for {
				part, err := mr.NextPart()
				if err != nil {
					break
				}
				partCT := part.Header.Get("Content-Type")
				if strings.HasPrefix(partCT, "text/plain") {
					body, _ := io.ReadAll(part)
					email.BodyValues = map[string]EmailBodyValue{"1": {Value: string(body)}}
					email.TextBody = []EmailBodyPart{{PartID: "1", Type: "text/plain"}}
					break
				}
			}
		}
	} else if strings.HasPrefix(mediaType, "text/") {
		// Find body after blank-line separator.
		idx := bytes.Index(raw, []byte("\r\n\r\n"))
		if idx < 0 {
			idx = bytes.Index(raw, []byte("\n\n"))
		}
		body := ""
		if idx >= 0 {
			body = strings.TrimSpace(string(raw[idx+2:]))
		}
		email.BodyValues = map[string]EmailBodyValue{"1": {Value: body}}
		email.TextBody = []EmailBodyPart{{PartID: "1", Type: "text/plain"}}
	}

	return email, nil
}
