package mailservice

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/maildb"
	"github.com/gogomail/gogomail/internal/outbound"
)

type SendTextRequest struct {
	UserID          string             `json:"user_id"`
	UserEmail       string             `json:"user_email,omitempty"`
	Intent          ComposeIntent      `json:"intent"`
	SourceMessageID string             `json:"source_message_id"`
	From            string             `json:"from"`
	To              []outbound.Address `json:"to"`
	Cc              []outbound.Address `json:"cc"`
	Bcc             []outbound.Address `json:"bcc"`
	Subject         string             `json:"subject"`
	TextBody        string             `json:"text_body"`
	HTMLBody        string             `json:"html_body,omitempty"`
	AttachmentIDs   []string           `json:"attachment_ids,omitempty"`
	Transactional   bool               `json:"transactional"`
	ScheduledAt     time.Time          `json:"scheduled_at"`
	TrackOpens      bool               `json:"track_opens,omitempty"`
	// DraftID, when non-empty, causes RecordOutgoing and MarkDraftSent to run
	// in a single transaction via RecordOutgoingFromDraft, eliminating the
	// crash window between the two operations.
	DraftID string `json:"draft_id,omitempty"`
}

type SendTextResult struct {
	ID             string        `json:"id"`
	RFCMessageID   string        `json:"message_id"`
	Farm           outbound.Farm `json:"farm"`
	SendStatus     string        `json:"send_status"`
	DeliveryStatus string        `json:"delivery_status"`
	BounceStatus   string        `json:"bounce_status"`
}

type mcpGeneratedNoticeContextKey struct{}
type mcpSendPolicyContextKey struct{}

type MCPSendPolicy struct {
	SenderDomain                string
	ConfirmExternalRecipients   bool
	ExternalRecipientsConfirmed bool
	ConfirmAttachments          bool
	AttachmentsConfirmed        bool
}

func ContextWithMCPGeneratedNotice(ctx context.Context, notice string) context.Context {
	notice = strings.TrimSpace(notice)
	if notice == "" {
		return ctx
	}
	return context.WithValue(ctx, mcpGeneratedNoticeContextKey{}, notice)
}

func MCPGeneratedNoticeFromContext(ctx context.Context) (string, bool) {
	notice, ok := ctx.Value(mcpGeneratedNoticeContextKey{}).(string)
	notice = strings.TrimSpace(notice)
	return notice, ok && notice != ""
}

func ContextWithMCPSendPolicy(ctx context.Context, policy MCPSendPolicy) context.Context {
	return context.WithValue(ctx, mcpSendPolicyContextKey{}, policy)
}

func MCPSendPolicyFromContext(ctx context.Context) (MCPSendPolicy, bool) {
	policy, ok := ctx.Value(mcpSendPolicyContextKey{}).(MCPSendPolicy)
	return policy, ok
}

func ApplyGeneratedNoticeToSendTextRequest(req *SendTextRequest, notice string) {
	if req == nil {
		return
	}
	notice = strings.TrimSpace(notice)
	if notice == "" {
		return
	}
	if strings.TrimSpace(req.TextBody) != "" && !strings.HasPrefix(req.TextBody, notice) {
		req.TextBody = notice + "\n\n" + req.TextBody
	}
	if strings.TrimSpace(req.HTMLBody) != "" && !strings.Contains(req.HTMLBody[:min(len(req.HTMLBody), 2048)], notice) {
		req.HTMLBody = `<p style="color:#8a8a8a;font-size:12px;margin:0 0 12px">` + htmlEscape(notice) + `</p>` + req.HTMLBody
	}
}

func EnforceMCPSendPolicy(ctx context.Context, req SendTextRequest) error {
	policy, ok := MCPSendPolicyFromContext(ctx)
	if !ok {
		return nil
	}
	if policy.ConfirmAttachments && len(req.AttachmentIDs) > 0 && !policy.AttachmentsConfirmed {
		return fmt.Errorf("mcp attachment confirmation is required")
	}
	if policy.ConfirmExternalRecipients && sendTextRequestHasExternalRecipient(req, policy.SenderDomain) && !policy.ExternalRecipientsConfirmed {
		return fmt.Errorf("mcp external-recipient confirmation is required")
	}
	return nil
}

func sendTextRequestHasExternalRecipient(req SendTextRequest, fallbackSenderDomain string) bool {
	domain := emailDomain(req.From)
	if domain == "" {
		domain = emailDomain(fallbackSenderDomain)
	}
	if domain == "" {
		return false
	}
	for _, recipient := range append(append(req.To, req.Cc...), req.Bcc...) {
		if candidate := emailDomain(recipient.Email); candidate != "" && candidate != domain {
			return true
		}
	}
	return false
}

func emailDomain(addressOrDomain string) string {
	addressOrDomain = strings.TrimSpace(addressOrDomain)
	if addressOrDomain == "" {
		return ""
	}
	if _, domain, ok := strings.Cut(addressOrDomain, "@"); ok {
		return strings.ToLower(strings.TrimSpace(domain))
	}
	if strings.Contains(addressOrDomain, ".") && !strings.ContainsAny(addressOrDomain, " \t\r\n") {
		return strings.ToLower(addressOrDomain)
	}
	return ""
}

func htmlEscape(value string) string {
	value = strings.ReplaceAll(value, "&", "&amp;")
	value = strings.ReplaceAll(value, "<", "&lt;")
	value = strings.ReplaceAll(value, ">", "&gt;")
	value = strings.ReplaceAll(value, `"`, "&quot;")
	value = strings.ReplaceAll(value, `'`, "&#39;")
	return value
}

func NormalizeSendTextResult(result SendTextResult) SendTextResult {
	if strings.TrimSpace(result.SendStatus) == "" {
		result.SendStatus = "queued"
	}
	if strings.TrimSpace(result.DeliveryStatus) == "" {
		result.DeliveryStatus = "pending"
	}
	if strings.TrimSpace(result.BounceStatus) == "" {
		result.BounceStatus = "none"
	}
	return result
}

func (s *Service) SendDraft(ctx context.Context, userID string, draftID string) (SendTextResult, error) {
	userID = strings.TrimSpace(userID)
	draftID = strings.TrimSpace(draftID)
	if userID == "" {
		return SendTextResult{}, fmt.Errorf("user_id is required")
	}
	if err := validateServiceResourceID("draft_id", draftID); err != nil {
		return SendTextResult{}, err
	}
	repo, ok := s.repository.(DraftSendRepository)
	if !ok {
		return SendTextResult{}, fmt.Errorf("draft send repository is required")
	}
	draft, err := repo.GetDraftForSend(ctx, userID, draftID)
	if err != nil {
		return SendTextResult{}, err
	}
	// Pass DraftID so SendText can use AtomicDraftSendRepository when available,
	// combining RecordOutgoing and MarkDraftSent into one transaction.
	req := SendTextRequest{
		UserID:          userID,
		Intent:          ComposeIntent(draft.Intent),
		SourceMessageID: draft.SourceMessageID,
		From:            draft.From,
		To:              draft.To,
		Cc:              draft.Cc,
		Bcc:             draft.Bcc,
		Subject:         draft.Subject,
		TextBody:        draft.TextBody,
		HTMLBody:        draft.HTMLBody,
		AttachmentIDs:   draft.AttachmentIDs,
		TrackOpens:      draft.TrackOpens,
		ScheduledAt:     draft.ScheduledAt,
		DraftID:         draftID,
	}
	if notice, ok := MCPGeneratedNoticeFromContext(ctx); ok {
		ApplyGeneratedNoticeToSendTextRequest(&req, notice)
	}
	if err := EnforceMCPSendPolicy(ctx, req); err != nil {
		return SendTextResult{}, err
	}
	result, err := s.SendText(ctx, req)
	if err != nil {
		return SendTextResult{}, err
	}
	// If the repository does not implement AtomicDraftSendRepository, the
	// atomic path was not taken and we must mark the draft sent separately.
	// This is the legacy two-step path with a small crash window.
	if _, ok := s.repository.(AtomicDraftSendRepository); !ok {
		if err := repo.MarkDraftSent(ctx, userID, draftID, result.ID); err != nil {
			return SendTextResult{}, err
		}
	}
	return result, nil
}

func (s *Service) SendText(ctx context.Context, req SendTextRequest) (SendTextResult, error) {
	if s.repository == nil {
		return SendTextResult{}, fmt.Errorf("mail repository is required")
	}
	if s.store == nil {
		return SendTextResult{}, fmt.Errorf("mail storage is required")
	}
	req = normalizeSendTextRequest(req)
	expandedReq, err := s.expandRecipientGroups(ctx, req)
	if err != nil {
		return SendTextResult{}, err
	}
	req = expandedReq
	if err := ValidateSendTextRequest(req); err != nil {
		return SendTextResult{}, err
	}

	sender, err := s.repository.SenderForUser(ctx, req.UserID, req.From)
	if err != nil {
		return SendTextResult{}, err
	}

	recipients := recipientEmails(req)
	suppressed, err := s.repository.SuppressedRecipients(ctx, sender.DomainID, recipients)
	if err != nil {
		return SendTextResult{}, err
	}
	if len(suppressed) > 0 {
		return SendTextResult{}, fmt.Errorf("suppressed recipients: %s", strings.Join(suppressed, ", "))
	}
	policy, err := s.domainPolicy(ctx, sender.DomainID)
	if err != nil {
		return SendTextResult{}, err
	}
	if err := enforceOutboundRecipientPolicy(req, policy); err != nil {
		return SendTextResult{}, err
	}
	sourceThread, err := s.sourceThread(ctx, req)
	if err != nil {
		return SendTextResult{}, err
	}

	from := outbound.Address{Name: sender.DisplayName, Email: sender.Address}

	// Build tracking pixels if requested.
	type pixelEntry struct {
		pixelID string
		email   string
	}
	var pixels []pixelEntry
	finalHTMLBody := req.HTMLBody
	if req.TrackOpens && s.trackingRepo != nil && s.publicBaseURL != "" {
		allRecipients := make([]outbound.Address, 0, len(req.To)+len(req.Cc)+len(req.Bcc))
		allRecipients = append(allRecipients, req.To...)
		allRecipients = append(allRecipients, req.Cc...)
		allRecipients = append(allRecipients, req.Bcc...)
		var pixelImgs strings.Builder
		for _, addr := range allRecipients {
			var raw [16]byte
			if _, err := rand.Read(raw[:]); err != nil {
				return SendTextResult{}, fmt.Errorf("generate pixel id: %w", err)
			}
			pid := hex.EncodeToString(raw[:])
			pixels = append(pixels, pixelEntry{pixelID: pid, email: addr.Email})
			pixelImgs.WriteString(`<img src="`)
			pixelImgs.WriteString(s.publicBaseURL)
			pixelImgs.WriteString("/t/")
			pixelImgs.WriteString(pid)
			pixelImgs.WriteString(`" width="1" height="1" alt="" style="display:none">`)
		}
		if strings.TrimSpace(finalHTMLBody) != "" {
			if idx := strings.LastIndex(strings.ToLower(finalHTMLBody), "</body>"); idx >= 0 {
				finalHTMLBody = finalHTMLBody[:idx] + pixelImgs.String() + finalHTMLBody[idx:]
			} else {
				finalHTMLBody += pixelImgs.String()
			}
		} else {
			finalHTMLBody = `<html><body><pre style="font-family:inherit;white-space:pre-wrap">` +
				req.TextBody + `</pre>` + pixelImgs.String() + `</body></html>`
		}
	}

	attachments, err := s.resolveAttachmentParts(ctx, req.UserID, req.AttachmentIDs)
	if err != nil {
		return SendTextResult{}, err
	}

	composedMsg := outbound.TextMessage{
		From:        from,
		To:          req.To,
		Cc:          req.Cc,
		Bcc:         req.Bcc,
		Subject:     req.Subject,
		TextBody:    req.TextBody,
		HTMLBody:    finalHTMLBody,
		InReplyTo:   sourceThread.MessageID,
		References:  sourceThread.References(),
		Attachments: attachments,
	}

	now := time.Now().UTC()
	objectID := randomObjectID()
	path := strings.Join([]string{
		"mailstore",
		sender.CompanyID,
		sender.DomainID,
		sender.UserID,
		"maildir",
		now.Format("2006"),
		now.Format("01"),
		objectID + ".eml",
	}, "/")

	composed, err := s.storeComposedMessage(ctx, path, composedMsg)
	if err != nil {
		return SendTextResult{}, err
	}
	if err := enforceOutboundSizePolicy(composed.Size, policy); err != nil {
		s.deleteMessageObjectBestEffort(ctx, path, "send_text_size_policy_failure", "user_id", sender.UserID, "domain_id", sender.DomainID, "company_id", sender.CompanyID)
		return SendTextResult{}, err
	}

	farm := outbound.Classify(outbound.ClassificationInput{
		Transactional:  req.Transactional,
		RecipientCount: len(req.To) + len(req.Cc) + len(req.Bcc),
		ScheduledAt:    req.ScheduledAt,
	})
	outgoingMsg := maildb.OutgoingMessage{
		CompanyID:       sender.CompanyID,
		DomainID:        sender.DomainID,
		UserID:          sender.UserID,
		ComposeIntent:   string(req.Intent),
		SourceMessageID: req.SourceMessageID,
		RFCMessageID:    composed.MessageID,
		Subject:         req.Subject,
		From:            from,
		To:              req.To,
		Cc:              req.Cc,
		Bcc:             req.Bcc,
		SentAt:          now,
		ScheduledAt:     req.ScheduledAt,
		Size:            composed.Size,
		HasAttachment:   len(req.AttachmentIDs) > 0,
		StoragePath:     path,
		Farm:            farm,
	}
	var id string
	if req.DraftID != "" {
		// Use the atomic variant when available: combines message insertion,
		// outbox insertion, and draft status update in a single transaction to
		// eliminate the crash window between RecordOutgoing and MarkDraftSent.
		if atomicRepo, ok := s.repository.(AtomicDraftSendRepository); ok {
			id, err = atomicRepo.RecordOutgoingFromDraft(ctx, outgoingMsg, req.DraftID)
		} else {
			id, err = s.repository.RecordOutgoing(ctx, outgoingMsg)
		}
	} else {
		id, err = s.repository.RecordOutgoing(ctx, outgoingMsg)
	}
	if err != nil {
		s.deleteMessageObjectBestEffort(ctx, path, "send_text_record_failure", "user_id", sender.UserID, "domain_id", sender.DomainID, "company_id", sender.CompanyID)
		return SendTextResult{}, err
	}
	if err := s.markSourceMessageAfterSend(ctx, req); err != nil {
		return SendTextResult{}, err
	}

	// Store tracking pixels (best-effort: do not fail the send on error).
	if len(pixels) > 0 && s.trackingRepo != nil {
		dbPixels := make([]maildb.TrackingPixel, 0, len(pixels))
		for _, p := range pixels {
			dbPixels = append(dbPixels, maildb.TrackingPixel{
				PixelID:        p.pixelID,
				MessageID:      id,
				SenderUserID:   sender.UserID,
				RecipientEmail: p.email,
			})
		}
		if err := s.trackingRepo.CreateTrackingPixels(ctx, dbPixels); err != nil {
			slog.Warn("failed to create tracking pixels", "message_id", id, "error", err)
		}
	}

	return NormalizeSendTextResult(SendTextResult{
		ID:             id,
		RFCMessageID:   composed.MessageID,
		Farm:           farm,
		SendStatus:     "queued",
		DeliveryStatus: "pending",
		BounceStatus:   "none",
	}), nil
}

func (s *Service) storeComposedMessage(ctx context.Context, path string, msg outbound.TextMessage) (outbound.ComposedMessage, error) {
	if s.store == nil {
		return outbound.ComposedMessage{}, fmt.Errorf("mail storage is required")
	}
	tmp, err := os.CreateTemp("", "gogomail-outgoing-*.eml")
	if err != nil {
		return outbound.ComposedMessage{}, fmt.Errorf("create outgoing message spool: %w", err)
	}
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
	}()
	composed, err := outbound.ComposeTextToWriter(tmp, msg)
	if err != nil {
		return outbound.ComposedMessage{}, err
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		return outbound.ComposedMessage{}, fmt.Errorf("rewind outgoing message spool: %w", err)
	}
	if err := s.store.Put(ctx, path, tmp); err != nil {
		return outbound.ComposedMessage{}, fmt.Errorf("store outgoing message: %w", err)
	}
	return composed, nil
}

func normalizeSendTextRequest(req SendTextRequest) SendTextRequest {
	intent, err := NormalizeComposeIntent(string(req.Intent))
	if err == nil {
		req.Intent = intent
	}
	req.UserID = strings.TrimSpace(req.UserID)
	req.SourceMessageID = strings.TrimSpace(req.SourceMessageID)
	req.From = strings.TrimSpace(req.From)
	req.To = normalizeComposeAddresses(req.To)
	req.Cc = normalizeComposeAddresses(req.Cc)
	req.Bcc = normalizeComposeAddresses(req.Bcc)
	req.AttachmentIDs = normalizeStringList(req.AttachmentIDs)
	return req
}

func (s *Service) resolveAttachmentParts(ctx context.Context, userID string, attachmentIDs []string) ([]outbound.Attachment, error) {
	if len(attachmentIDs) == 0 {
		return nil, nil
	}
	if s.store == nil {
		return nil, fmt.Errorf("mail storage is required")
	}
	repo, ok := s.repository.(interface {
		AttachmentsByIDs(context.Context, string, []string) ([]maildb.Attachment, error)
	})
	if !ok {
		return nil, fmt.Errorf("attachment lookup repository is required")
	}
	attachments, err := repo.AttachmentsByIDs(ctx, strings.TrimSpace(userID), attachmentIDs)
	if err != nil {
		return nil, err
	}
	parts := make([]outbound.Attachment, 0, len(attachments))
	for _, attachment := range attachments {
		parts = append(parts, outbound.Attachment{
			Filename: attachment.Filename,
			MIMEType: attachment.MIMEType,
			Open: func(storagePath string) func() (io.ReadCloser, error) {
				return func() (io.ReadCloser, error) {
					body, err := s.store.Get(ctx, storagePath)
					if err != nil {
						return nil, fmt.Errorf("open attachment %q: %w", storagePath, err)
					}
					return body, nil
				}
			}(attachment.StoragePath),
		})
	}
	return parts, nil
}

func normalizeComposeAddresses(addresses []outbound.Address) []outbound.Address {
	for i := range addresses {
		addresses[i].Name = strings.TrimSpace(addresses[i].Name)
		addresses[i].Email = strings.TrimSpace(addresses[i].Email)
	}
	return addresses
}

func (s *Service) expandRecipientGroups(ctx context.Context, req SendTextRequest) (SendTextRequest, error) {
	repo, ok := s.repository.(RecipientGroupRepository)
	if !hasRecipientGroupTokens(req) {
		return req, nil
	}
	if !ok {
		return SendTextRequest{}, fmt.Errorf("recipient group repository is required")
	}
	seen := make(map[string]struct{})
	cache := make(map[string][]outbound.Address)
	var err error
	req.To, err = s.expandRecipientGroupField(ctx, repo, req.UserID, req.To, seen, cache)
	if err != nil {
		return SendTextRequest{}, err
	}
	req.Cc, err = s.expandRecipientGroupField(ctx, repo, req.UserID, req.Cc, seen, cache)
	if err != nil {
		return SendTextRequest{}, err
	}
	req.Bcc, err = s.expandRecipientGroupField(ctx, repo, req.UserID, req.Bcc, seen, cache)
	if err != nil {
		return SendTextRequest{}, err
	}
	return req, nil
}

func hasRecipientGroupTokens(req SendTextRequest) bool {
	for _, field := range [][]outbound.Address{req.To, req.Cc, req.Bcc} {
		for _, addr := range field {
			if recipientGroupTokenKind(addr.Email) != "" {
				return true
			}
		}
	}
	return false
}

func (s *Service) expandRecipientGroupField(ctx context.Context, repo RecipientGroupRepository, userID string, addresses []outbound.Address, seen map[string]struct{}, cache map[string][]outbound.Address) ([]outbound.Address, error) {
	expanded := make([]outbound.Address, 0, len(addresses))
	appendAddress := func(addr outbound.Address) {
		key := strings.ToLower(strings.TrimSpace(addr.Email))
		if key == "" {
			return
		}
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		expanded = append(expanded, addr)
	}
	for _, addr := range addresses {
		switch recipientGroupTokenKind(addr.Email) {
		case "org":
			orgID, includeChildren := parseOrgRecipientToken(addr.Email)
			cacheKey := "org:" + orgID + ":" + strconv.FormatBool(includeChildren)
			members, ok := cache[cacheKey]
			if !ok {
				var err error
				members, err = repo.ExpandOrgRecipients(ctx, userID, orgID, includeChildren)
				if err != nil {
					return nil, err
				}
				cache[cacheKey] = members
			}
			for _, member := range members {
				appendAddress(member)
			}
		case "addressbook":
			bookID := strings.TrimPrefix(strings.TrimSpace(addr.Email), "addressbook:")
			cacheKey := "addressbook:" + bookID
			contacts, ok := cache[cacheKey]
			if !ok {
				var err error
				contacts, err = repo.ExpandAddressBookRecipients(ctx, userID, bookID)
				if err != nil {
					return nil, err
				}
				cache[cacheKey] = contacts
			}
			for _, contact := range contacts {
				appendAddress(contact)
			}
		default:
			appendAddress(addr)
		}
	}
	return expanded, nil
}

func recipientGroupTokenKind(email string) string {
	email = strings.TrimSpace(email)
	if strings.HasPrefix(email, "org:") {
		return "org"
	}
	if strings.HasPrefix(email, "addressbook:") {
		return "addressbook"
	}
	return ""
}

func parseOrgRecipientToken(token string) (string, bool) {
	token = strings.TrimSpace(strings.TrimPrefix(token, "org:"))
	return strings.TrimSuffix(token, ":children"), strings.HasSuffix(token, ":children")
}

func (s *Service) sourceThread(ctx context.Context, req SendTextRequest) (maildb.SourceThreadView, error) {
	req = normalizeSendTextRequest(req)
	intent, err := NormalizeComposeIntent(string(req.Intent))
	if err != nil {
		return maildb.SourceThreadView{}, err
	}
	if intent != ComposeIntentReply || req.SourceMessageID == "" {
		return maildb.SourceThreadView{}, nil
	}
	repo, ok := s.repository.(SourceThreadRepository)
	if !ok {
		return maildb.SourceThreadView{}, fmt.Errorf("source thread repository is required")
	}
	return repo.SourceThread(ctx, req.UserID, req.SourceMessageID)
}

func (s *Service) domainPolicy(ctx context.Context, domainID string) (maildb.DomainPolicyView, error) {
	domainID = strings.TrimSpace(domainID)
	repo, ok := s.repository.(DomainPolicyRepository)
	if !ok {
		return maildb.DomainPolicyView{DomainID: domainID, InboundMode: "inherit", OutboundMode: "inherit"}, nil
	}
	return repo.DomainPolicy(ctx, domainID)
}

func (s *Service) enforceAttachmentPolicy(ctx context.Context, userID string, size int64) error {
	repo, ok := s.repository.(UserDomainPolicyRepository)
	if !ok {
		return nil
	}
	userID = strings.TrimSpace(userID)
	policy, err := repo.DomainPolicyForUser(ctx, userID)
	if err != nil {
		return err
	}
	return enforceAttachmentSizePolicy(size, policy)
}

func enforceOutboundRecipientPolicy(req SendTextRequest, policy maildb.DomainPolicyView) error {
	if policy.OutboundMode != "enforce" || policy.MaxRecipientsPerMessage <= 0 {
		return nil
	}
	recipientCount := len(recipientEmails(req))
	if recipientCount > policy.MaxRecipientsPerMessage {
		return fmt.Errorf("domain outbound policy max_recipients_per_message exceeded: %d > %d", recipientCount, policy.MaxRecipientsPerMessage)
	}
	return nil
}

func enforceOutboundSizePolicy(size int64, policy maildb.DomainPolicyView) error {
	if policy.OutboundMode != "enforce" || policy.MaxMessageBytes <= 0 {
		return nil
	}
	if size > policy.MaxMessageBytes {
		return fmt.Errorf("domain outbound policy max_message_bytes exceeded: %d > %d", size, policy.MaxMessageBytes)
	}
	return nil
}

func enforceAttachmentSizePolicy(size int64, policy maildb.DomainPolicyView) error {
	if policy.OutboundMode != "enforce" || policy.MaxAttachmentBytes <= 0 {
		return nil
	}
	if size > policy.MaxAttachmentBytes {
		return fmt.Errorf("domain outbound policy max_attachment_bytes exceeded: %d > %d", size, policy.MaxAttachmentBytes)
	}
	return nil
}

func (s *Service) markSourceMessageAfterSend(ctx context.Context, req SendTextRequest) error {
	intent, err := NormalizeComposeIntent(string(req.Intent))
	if err != nil {
		return err
	}
	switch intent {
	case ComposeIntentReply:
		return s.repository.SetMessageFlag(ctx, req.UserID, req.SourceMessageID, "answered", true)
	case ComposeIntentForward:
		return s.repository.SetMessageFlag(ctx, req.UserID, req.SourceMessageID, "forwarded", true)
	default:
		return nil
	}
}

func randomObjectID() string {
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("%d-%s", time.Now().UnixMilli(), hex.EncodeToString(random[:]))
}

func recipientEmails(req SendTextRequest) []string {
	recipients := make([]string, 0, len(req.To)+len(req.Cc)+len(req.Bcc))
	seen := make(map[string]struct{}, len(req.To)+len(req.Cc)+len(req.Bcc))
	appendRecipient := func(email string) {
		email = strings.ToLower(strings.TrimSpace(email))
		if email == "" {
			return
		}
		if _, ok := seen[email]; ok {
			return
		}
		seen[email] = struct{}{}
		recipients = append(recipients, email)
	}
	for _, addr := range req.To {
		appendRecipient(addr.Email)
	}
	for _, addr := range req.Cc {
		appendRecipient(addr.Email)
	}
	for _, addr := range req.Bcc {
		appendRecipient(addr.Email)
	}
	return recipients
}

// stripHTMLTags returns a plain-text approximation of an HTML string by
// removing all tags and collapsing whitespace.  Used as a TextBody fallback
// when only an inline html_body is available (e.g. seed/test data with no
// object-store body).
