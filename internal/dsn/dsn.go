package dsn

import (
	"bytes"
	"fmt"
	"mime"
	"strconv"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/outbound"
)

type RecipientStatus struct {
	Recipient         string
	OriginalRecipient string
	Action            string
	Status            string
	Diagnostic        string
	RemoteMTA         string
	FinalLogID        string
	LastAttemptAt     time.Time
}

type Report struct {
	ReportingMTA string
	OriginalID   string
	From         outbound.Address
	To           outbound.Address
	Subject      string
	MessageID    string
	Date         time.Time
	Recipients   []RecipientStatus
}

func Compose(report Report) (outbound.ComposedMessage, error) {
	if strings.TrimSpace(report.ReportingMTA) == "" {
		return outbound.ComposedMessage{}, fmt.Errorf("reporting MTA is required")
	}
	if len(report.Recipients) == 0 {
		return outbound.ComposedMessage{}, fmt.Errorf("at least one DSN recipient status is required")
	}
	if report.Date.IsZero() {
		report.Date = time.Now().UTC()
	} else {
		report.Date = report.Date.UTC()
	}
	if strings.TrimSpace(report.Subject) == "" {
		report.Subject = "Delivery Status Notification"
	}
	if strings.TrimSpace(report.MessageID) == "" {
		report.MessageID = outbound.GenerateMessageID(domainFromAddress(report.From.Email))
	}
	boundary := "gogomail-dsn-" + sanitizeBoundaryToken(strings.Trim(report.MessageID, "<>"))

	var buf bytes.Buffer
	writeHeader(&buf, "From", formatAddress(report.From))
	writeHeader(&buf, "To", formatAddress(report.To))
	writeHeader(&buf, "Subject", mime.QEncoding.Encode("utf-8", report.Subject))
	writeHeader(&buf, "Date", report.Date.Format(time.RFC1123Z))
	writeHeader(&buf, "Message-ID", ensureMessageID(report.MessageID))
	writeHeader(&buf, "Auto-Submitted", "auto-replied")
	writeHeader(&buf, "MIME-Version", "1.0")
	writeHeader(&buf, "Content-Type", `multipart/report; report-type=delivery-status; boundary="`+boundary+`"`)
	buf.WriteString("\r\n")
	buf.WriteString("--" + boundary + "\r\n")
	buf.WriteString("Content-Type: text/plain; charset=utf-8\r\n\r\n")
	buf.WriteString("This is an automatically generated Delivery Status Notification.\r\n\r\n")
	buf.WriteString("--" + boundary + "\r\n")
	buf.WriteString("Content-Type: message/delivery-status\r\n\r\n")
	writeDSNField(&buf, "Reporting-MTA", "dns; "+sanitizeDSNValue(report.ReportingMTA))
	if report.OriginalID != "" {
		writeDSNField(&buf, "Original-Envelope-Id", sanitizeDSNValue(report.OriginalID))
	}
	buf.WriteString("\r\n")
	for i, recipient := range report.Recipients {
		if i > 0 {
			buf.WriteString("\r\n")
		}
		if err := writeRecipientStatus(&buf, recipient); err != nil {
			return outbound.ComposedMessage{}, err
		}
	}
	buf.WriteString("--" + boundary + "--\r\n")

	raw := buf.Bytes()
	return outbound.ComposedMessage{Raw: raw, MessageID: ensureMessageID(report.MessageID), Size: int64(len(raw))}, nil
}

func writeRecipientStatus(buf *bytes.Buffer, status RecipientStatus) error {
	if strings.TrimSpace(status.Recipient) == "" {
		return fmt.Errorf("dsn recipient is required")
	}
	action := strings.ToLower(strings.TrimSpace(status.Action))
	if action == "" {
		action = "failed"
	}
	if !validDSNAction(action) {
		return fmt.Errorf("invalid dsn action %q", status.Action)
	}
	dsnStatus := strings.TrimSpace(status.Status)
	if dsnStatus == "" {
		dsnStatus = defaultEnhancedStatusForAction(action)
	}
	if !validEnhancedStatus(dsnStatus) {
		return fmt.Errorf("invalid dsn status %q", status.Status)
	}
	if !dsnStatusMatchesAction(action, dsnStatus) {
		return fmt.Errorf("dsn status %q does not match action %q", dsnStatus, action)
	}
	if status.OriginalRecipient != "" {
		writeDSNField(buf, "Original-Recipient", sanitizeRecipientAddressType(status.OriginalRecipient))
	}
	writeDSNField(buf, "Final-Recipient", sanitizeRecipientAddressType(status.Recipient))
	writeDSNField(buf, "Action", action)
	writeDSNField(buf, "Status", dsnStatus)
	if status.RemoteMTA != "" {
		writeDSNField(buf, "Remote-MTA", "dns; "+sanitizeDSNValue(status.RemoteMTA))
	}
	if status.Diagnostic != "" {
		writeDSNField(buf, "Diagnostic-Code", "smtp; "+sanitizeDSNValue(status.Diagnostic))
	}
	if !status.LastAttemptAt.IsZero() {
		writeDSNField(buf, "Last-Attempt-Date", status.LastAttemptAt.UTC().Format(time.RFC1123Z))
	}
	if status.FinalLogID != "" {
		writeDSNField(buf, "Final-Log-ID", sanitizeDSNValue(status.FinalLogID))
	}
	return nil
}

func writeHeader(buf *bytes.Buffer, key, value string) {
	buf.WriteString(key + ": " + strings.ReplaceAll(strings.ReplaceAll(value, "\r", ""), "\n", "") + "\r\n")
}

func writeDSNField(buf *bytes.Buffer, key, value string) {
	writeHeader(buf, key, value)
}

func formatAddress(addr outbound.Address) string {
	name := strings.TrimSpace(addr.Name)
	email := sanitizeAddressEmail(addr.Email)
	if name == "" {
		return "<" + email + ">"
	}
	return mime.QEncoding.Encode("utf-8", name) + " <" + email + ">"
}

func sanitizeAddressEmail(email string) string {
	email = sanitizeDSNValue(email)
	email = strings.ReplaceAll(email, " ", "")
	email = strings.Trim(email, "<>")
	if email == "" {
		return "postmaster@localhost"
	}
	return email
}

func ensureMessageID(messageID string) string {
	messageID = sanitizeMessageIDValue(messageID)
	if !strings.HasPrefix(messageID, "<") {
		messageID = "<" + messageID
	}
	if !strings.HasSuffix(messageID, ">") {
		messageID += ">"
	}
	return messageID
}

func sanitizeMessageIDValue(messageID string) string {
	messageID = sanitizeDSNValue(messageID)
	messageID = strings.ReplaceAll(messageID, " ", "")
	messageID = strings.TrimSpace(messageID)
	if messageID == "" {
		return "dsn@localhost"
	}
	return messageID
}

func domainFromAddress(address string) string {
	_, domain, ok := strings.Cut(strings.TrimSpace(address), "@")
	if !ok || domain == "" {
		return "localhost"
	}
	return domain
}

func sanitizeDSNValue(value string) string {
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	return strings.Join(strings.Fields(value), " ")
}

func sanitizeRecipientAddressType(value string) string {
	value = sanitizeDSNValue(value)
	if strings.Contains(value, ";") {
		return value
	}
	return "rfc822; " + strings.ToLower(strings.TrimSpace(value))
}

func sanitizeBoundaryToken(value string) string {
	value = sanitizeDSNValue(value)
	if value == "" {
		return "boundary"
	}
	var b strings.Builder
	b.Grow(len(value))
	lastDash := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' || r == '@' {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-.")
	if out == "" {
		return "boundary"
	}
	if len(out) > 52 {
		out = strings.TrimRight(out[:52], "-.")
		if out == "" {
			return "boundary"
		}
	}
	return out
}

func validDSNAction(action string) bool {
	switch action {
	case "failed", "delayed", "delivered", "relayed", "expanded":
		return true
	default:
		return false
	}
}

func validEnhancedStatus(status string) bool {
	parts := strings.Split(status, ".")
	if len(parts) != 3 {
		return false
	}
	if len(parts[0]) != 1 {
		return false
	}
	class, err := strconv.Atoi(parts[0])
	if err != nil || (class != 2 && class != 4 && class != 5) {
		return false
	}
	for _, part := range parts[1:] {
		if part == "" || len(part) > 3 {
			return false
		}
		for _, r := range part {
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	return true
}

func defaultEnhancedStatusForAction(action string) string {
	switch action {
	case "delivered", "relayed", "expanded":
		return "2.0.0"
	case "delayed":
		return "4.0.0"
	default:
		return "5.0.0"
	}
}

func dsnStatusMatchesAction(action string, status string) bool {
	if status == "" {
		return false
	}
	switch action {
	case "delivered", "relayed", "expanded":
		return status[0] == '2'
	case "delayed":
		return status[0] == '4'
	default:
		return status[0] == '5'
	}
}
