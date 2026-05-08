package scheduling

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/mail"
	"net/textproto"
	"strings"
	"time"

	ical "github.com/emersion/go-ical"
	"github.com/gogomail/gogomail/internal/delivery"
	"github.com/gogomail/gogomail/internal/eventstream"
	"github.com/gogomail/gogomail/internal/outbound"
	"github.com/redis/go-redis/v9"
)

const EventNameSchedulingOutbox = "scheduling.outbox"

const DeliveryStream = "mail.outbound.general"

type Handler struct {
	logger *slog.Logger
	queue  Queue
	store  ObjectStore
}

type ObjectStore interface {
	Put(ctx context.Context, path string, r io.Reader) error
}

type Queue interface {
	Enqueue(ctx context.Context, topic string, partitionKey string, payload []byte) error
}

type DeliveryQueue struct {
	client *redis.Client
}

func NewDeliveryQueue(client *redis.Client) *DeliveryQueue {
	return &DeliveryQueue{client: client}
}

func (q *DeliveryQueue) Enqueue(ctx context.Context, topic string, partitionKey string, payload []byte) error {
	if q.client == nil {
		return fmt.Errorf("redis client is required")
	}

	values := map[string]any{
		"partition_key": partitionKey,
		"payload":       string(payload),
	}

	return q.client.XAdd(ctx, &redis.XAddArgs{
		Stream: DeliveryStream,
		Values: values,
	}).Err()
}

func NewHandler(logger *slog.Logger, queue Queue, store ObjectStore) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{
		logger: logger,
		queue:  queue,
		store:  store,
	}
}

type schedulingPayload struct {
	SchemaVersion string `json:"schema_version"`
	DavKind      string `json:"dav_kind"`
	UserID       string `json:"user_id"`
	UID          string `json:"uid"`
	Method       string `json:"method"`
	ICSPayload   string `json:"payload"`
}

func (h *Handler) HandleEvent(ctx context.Context, msg eventstream.Message) error {
	var payload schedulingPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("decode scheduling outbox event: %w", err)
	}

	h.logger.Info("handling scheduling outbox event",
		"user_id", payload.UserID,
		"uid", payload.UID,
		"method", payload.Method,
		"schema_version", payload.SchemaVersion,
		"dav_kind", payload.DavKind,
	)

	attendees, organizer, err := extractParticipants([]byte(payload.ICSPayload))
	if err != nil {
		h.logger.Warn("failed to extract participants from iCalendar",
			"uid", payload.UID,
			"error", err,
		)
		return nil
	}

	if organizer == "" {
		h.logger.Warn("iCalendar has no organizer, cannot send iTIP",
			"uid", payload.UID,
		)
		return nil
	}

	itipMessage, err := buildITIPMessage([]byte(payload.ICSPayload), payload.Method, organizer)
	if err != nil {
		h.logger.Warn("failed to build iTIP message",
			"uid", payload.UID,
			"error", err,
		)
		return nil
	}

	for _, attendee := range attendees {
		if attendee == organizer {
			continue
		}
		if err := h.sendToAttendee(ctx, payload.UID, payload.UserID, organizer, attendee, itipMessage); err != nil {
			h.logger.Error("failed to enqueue iTIP for attendee",
				"uid", payload.UID,
				"attendee", attendee,
				"error", err,
			)
		}
	}

	return nil
}

func (h *Handler) sendToAttendee(ctx context.Context, uid, userID, organizer, attendee string, itipMessage []byte) error {
	if h.queue == nil || h.store == nil {
		h.logger.Info("scheduling would send iTIP to attendee (queue/store not configured)",
			"uid", uid,
			"attendee", attendee,
			"organizer", organizer,
		)
		return nil
	}

	organizerAddr, err := mail.ParseAddress(organizer)
	if err != nil {
		return fmt.Errorf("parse organizer address: %w", err)
	}
	attendeeAddr, err := mail.ParseAddress(attendee)
	if err != nil {
		return fmt.Errorf("parse attendee address: %w", err)
	}

	method := detectMethod(itipMessage)
	subject := iTIPSubject(method, uid)

	msg := buildMultipartMessage(organizerAddr, attendeeAddr, subject, itipMessage)

	storagePath := fmt.Sprintf("scheduling/%s/%s-%d.ics", userID, uid, time.Now().UnixNano())

	if err := h.store.Put(ctx, storagePath, bytes.NewReader(msg)); err != nil {
		return fmt.Errorf("store iTIP message: %w", err)
	}

	queued := delivery.QueuedMessage{
		Event:        "mail.queued",
		MessageID:    uid,
		RFCMessageID: generateMessageID(),
		CompanyID:    "",
		DomainID:     "",
		Farm:         outbound.FarmGeneral,
		From:         outbound.Address{Name: organizerAddr.Name, Email: organizerAddr.Address},
		To:           []outbound.Address{{Email: attendeeAddr.Address}},
		Subject:      subject,
		StoragePath:  storagePath,
		Size:         int64(len(msg)),
	}

	payloadBytes, err := json.Marshal(queued)
	if err != nil {
		return fmt.Errorf("marshal queue payload: %w", err)
	}

	if err := h.queue.Enqueue(ctx, "mail.outbound.general", uid, payloadBytes); err != nil {
		return fmt.Errorf("enqueue iTIP: %w", err)
	}

	h.logger.Info("enqueued iTIP message for attendee",
		"uid", uid,
		"attendee", attendee,
		"organizer", organizer,
		"method", method,
	)

	return nil
}

func buildMultipartMessage(from *mail.Address, to *mail.Address, subject string, itipBody []byte) []byte {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	header := make(textproto.MIMEHeader)
	header.Set("From", from.String())
	header.Set("To", to.String())
	header.Set("Subject", subject)
	header.Set("Date", time.Now().UTC().Format(time.RFC1123Z))
	header.Set("Message-ID", generateMessageID())
	header.Set("MIME-Version", "1.0")
	header.Set("Content-Type", fmt.Sprintf("multipart/alternative; boundary=%q", w.Boundary()))

	for k, v := range header {
		buf.WriteString(k + ": " + v[0] + "\r\n")
	}
	buf.WriteString("\r\n")

	buf.WriteString("--" + w.Boundary() + "\r\n")
	buf.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	buf.WriteString("Content-Transfer-Encoding: 7bit\r\n")
	buf.WriteString("\r\n")
	buf.WriteString("This is an iCalendar scheduling message. Please use a compatible calendar client to process this message.\r\n")
	buf.WriteString("\r\n")

	buf.WriteString("--" + w.Boundary() + "\r\n")
	buf.WriteString("Content-Type: message/rfc822; charset=utf-8\r\n")
	buf.WriteString("Content-Transfer-Encoding: 7bit\r\n")
	buf.WriteString("\r\n")
	buf.Write(itipBody)
	buf.WriteString("\r\n")

	buf.WriteString("--" + w.Boundary() + "--\r\n")

	return buf.Bytes()
}

func buildITIPMessage(icsBody []byte, method, organizer string) ([]byte, error) {
	if method == "" {
		method = detectMethod(icsBody)
	}

	var buf bytes.Buffer
	buf.WriteString("BEGIN:VCALENDAR\r\n")
	buf.WriteString("VERSION:2.0\r\n")
	buf.WriteString("PRODID:-//gogomail//CalDAV iMIP//EN\r\n")
	buf.WriteString("METHOD:" + method + "\r\n")

	cal, err := ical.NewDecoder(bytes.NewReader(icsBody)).Decode()
	if err == nil && cal != nil && cal.Component != nil {
		for _, child := range cal.Component.Children {
			component := strings.ToUpper(strings.TrimSpace(child.Name))
			if component == "VEVENT" || component == "VTODO" || component == "VJOURNAL" || component == "VFREEBUSY" {
				encodeComponent(&buf, child)
			}
		}
	}

	buf.WriteString("END:VCALENDAR\r\n")
	return buf.Bytes(), nil
}

func encodeComponent(buf *bytes.Buffer, comp *ical.Component) {
	buf.WriteString("BEGIN:" + comp.Name + "\r\n")

	for name, props := range comp.Props {
		for _, prop := range props {
			if name == "ATTACH" || name == "X-ALT-DESC" {
				continue
			}
			line := encodeProperty(name, prop)
			if line != "" {
				buf.WriteString(line + "\r\n")
			}
		}
	}

	for _, child := range comp.Children {
		childName := strings.ToUpper(strings.TrimSpace(child.Name))
		if childName == "VEVENT" || childName == "VTODO" || childName == "VJOURNAL" || childName == "VFREEBUSY" || childName == "VALARM" {
			encodeComponent(buf, child)
		}
	}

	buf.WriteString("END:" + comp.Name + "\r\n")
}

func encodeProperty(name string, prop ical.Prop) string {
	value, err := prop.Text()
	if err != nil {
		return ""
	}

	line := name
	for pname, pvalues := range prop.Params {
		if len(pvalues) > 0 {
			line += ";" + pname + "=" + pvalues[0]
		}
	}
	line += ":" + value

	if len(line) > 75 {
		parts := splitLine(line, 75)
		return strings.Join(parts, "\r\n ")
	}
	return line
}

func splitLine(s string, maxLen int) []string {
	var parts []string
	for len(s) > maxLen {
		parts = append(parts, s[:maxLen])
		s = " " + s[maxLen:]
	}
	parts = append(parts, s)
	return parts
}

func detectMethod(icsBody []byte) string {
	lines := strings.Split(string(icsBody), "\r\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToUpper(line), "METHOD:") {
			return strings.TrimPrefix(line, "METHOD:")
		}
	}
	return "REQUEST"
}

func iTIPSubject(method, uid string) string {
	switch strings.ToUpper(method) {
	case "CANCEL":
		return "Cancelled: Calendar Event"
	case "REPLY":
		return "Accepted: Calendar Event"
	case "COUNTER":
		return "Counter Proposal: Calendar Event"
	case "DECLINECOUNTER":
		return "Declined: Counter Proposal"
	case "ADD":
		return "Additional Instance: Calendar Event"
	case "REFRESH":
		return "Meeting Request Update"
	default:
		return "Meeting Request"
	}
}

func extractParticipants(icsBody []byte) (attendees []string, organizer string, err error) {
	cal, err := ical.NewDecoder(bytes.NewReader(icsBody)).Decode()
	if err != nil {
		return nil, "", fmt.Errorf("decode iCalendar: %w", err)
	}
	if cal == nil || cal.Component == nil {
		return nil, "", fmt.Errorf("iCalendar body must contain VCALENDAR root")
	}

	if cal.Component == nil {
		return nil, "", fmt.Errorf("iCalendar body must contain VCALENDAR root")
	}

	for _, child := range cal.Component.Children {
		component := strings.ToUpper(strings.TrimSpace(child.Name))
		if component != "VEVENT" && component != "VTODO" && component != "VJOURNAL" {
			continue
		}

		if props, ok := child.Props[ical.PropOrganizer]; ok && len(props) > 0 {
			organizer = extractCalAddress(props[0])
		}

		if props, ok := child.Props[ical.PropAttendee]; ok {
			for _, prop := range props {
				if email := extractCalAddress(prop); email != "" && !contains(attendees, email) {
					attendees = append(attendees, email)
				}
			}
		}
	}

	return attendees, organizer, nil
}

func stripMailto(addr string) string {
	addr = strings.TrimSpace(addr)
	if strings.HasPrefix(strings.ToUpper(addr), "MAILTO:") {
		addr = strings.TrimPrefix(addr, "mailto:")
		addr = strings.TrimPrefix(addr, "MAILTO:")
	}
	return strings.TrimSpace(addr)
}

func extractCalAddress(prop ical.Prop) string {
	val := strings.TrimSpace(prop.Value)
	if val == "" {
		return ""
	}
	return stripMailto(val)
}

func contains(list []string, s string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}

func generateMessageID() string {
	return fmt.Sprintf("<%d.%d@caldav-scheduling>", time.Now().UnixNano(), time.Now().UnixNano()%10000)
}