package scheduling

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	ical "github.com/emersion/go-ical"
	"github.com/gogomail/gogomail/internal/eventstream"
)

const EventNameSchedulingOutbox = "scheduling.outbox"

type Handler struct {
	logger *slog.Logger
}

func NewHandler(logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{logger: logger}
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

	for _, attendee := range attendees {
		h.logger.Info("scheduling would send to attendee",
			"uid", payload.UID,
			"method", payload.Method,
			"attendee", attendee,
			"organizer", organizer,
		)
	}

	return nil
}

func extractParticipants(icsBody []byte) (attendees []string, organizer string, err error) {
	cal, err := ical.NewDecoder(bytes.NewReader(icsBody)).Decode()
	if err != nil {
		return nil, "", fmt.Errorf("decode iCalendar: %w", err)
	}
	if cal == nil || cal.Component == nil {
		return nil, "", fmt.Errorf("iCalendar body must contain VCALENDAR root")
	}

	for _, child := range cal.Children {
		component := strings.ToUpper(strings.TrimSpace(child.Name))
		if component != "VEVENT" && component != "VTODO" && component != "VJOURNAL" {
			continue
		}

		if props, ok := child.Props["ORGANIZER"]; ok && len(props) > 0 {
			if org, err := props[0].Text(); err == nil {
				organizer = stripMailto(org)
			}
		}

		if props, ok := child.Props["ATTENDEE"]; ok {
			for _, prop := range props {
				if email, err := prop.Text(); err == nil {
					email = stripMailto(email)
					if email != "" && !contains(attendees, email) {
						attendees = append(attendees, email)
					}
				}
			}
		}

		if props, ok := child.Props["ATTACH"]; ok {
			for _, prop := range props {
				if uri, err := prop.Text(); err == nil && strings.HasPrefix(uri, "mailto:") {
					email := stripMailto(uri)
					if email != "" && !contains(attendees, email) && !contains([]string{organizer}, email) {
						attendees = append(attendees, email)
					}
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

func contains(list []string, s string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}