package dm

import (
	"fmt"
	"strings"
	"time"
)

// RoomExport holds all data required to render a room export file.
type RoomExport struct {
	Room     Room
	Messages []Message
	ExportAt time.Time
}

// FormatExportTXT renders a RoomExport as human-readable plain text.
// Deleted messages are shown as [삭제됨]; system messages are labeled [시스템].
// loc is the timezone for all displayed timestamps; if nil, UTC is used.
func FormatExportTXT(e RoomExport, loc *time.Location) string {
	if loc == nil {
		loc = time.UTC
	}
	tzLabel := loc.String()

	var sb strings.Builder

	roomName := e.Room.Name
	if roomName == "" {
		names := make([]string, 0, len(e.Room.Members))
		for _, m := range e.Room.Members {
			names = append(names, m.DisplayName)
		}
		roomName = strings.Join(names, ", ")
	}
	if roomName == "" {
		roomName = e.Room.ID
	}

	participantLines := make([]string, 0, len(e.Room.Members))
	for _, m := range e.Room.Members {
		if m.Email != "" {
			participantLines = append(participantLines, fmt.Sprintf("  - %s (%s)", m.DisplayName, m.Email))
		} else {
			participantLines = append(participantLines, fmt.Sprintf("  - %s", m.DisplayName))
		}
	}

	formatTS := func(t time.Time) string {
		return t.In(loc).Format("2006-01-02 15:04:05") + " " + tzLabel
	}

	fmt.Fprintf(&sb, "============================\n")
	fmt.Fprintf(&sb, "Room: %s\n", roomName)
	fmt.Fprintf(&sb, "Type: %s\n", e.Room.RoomType)
	fmt.Fprintf(&sb, "Participants:\n%s\n", strings.Join(participantLines, "\n"))
	fmt.Fprintf(&sb, "Exported: %s\n", formatTS(e.ExportAt))
	fmt.Fprintf(&sb, "Messages: %d\n", len(e.Messages))
	fmt.Fprintf(&sb, "============================\n\n")

	senderByID := make(map[string]User, len(e.Room.Members))
	for _, m := range e.Room.Members {
		senderByID[m.ID] = m
	}

	for _, msg := range e.Messages {
		ts := formatTS(msg.CreatedAt)

		if msg.DeletedAt != nil {
			fmt.Fprintf(&sb, "[%s] %s\n\n", ts, msg.Body)
			continue
		}

		if msg.MessageType == MessageTypeSystem {
			fmt.Fprintf(&sb, "[%s] [시스템]: %s\n\n", ts, msg.Body)
			continue
		}

		sender := senderByID[msg.SenderID]
		senderLabel := sender.DisplayName
		if sender.Email != "" {
			senderLabel = fmt.Sprintf("%s (%s)", sender.DisplayName, sender.Email)
		} else if senderLabel == "" {
			senderLabel = msg.SenderID
		}

		switch msg.MessageType {
		case MessageTypeFile:
			fmt.Fprintf(&sb, "[%s] %s:\n  [파일: %s]\n\n", ts, senderLabel, msg.AttachmentName)
		case MessageTypeDriveLink:
			fmt.Fprintf(&sb, "[%s] %s:\n  [드라이브: %s]\n\n", ts, senderLabel, msg.DriveFileID)
		default: // MessageTypeText and anything else
			fmt.Fprintf(&sb, "[%s] %s:\n  %s\n\n", ts, senderLabel, msg.Body)
		}
	}

	return sb.String()
}
