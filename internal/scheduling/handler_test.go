package scheduling

import (
	"bytes"
	"context"
	"io"
	"net/url"
	"strings"
	"testing"

	ical "github.com/emersion/go-ical"
	"github.com/gogomail/gogomail/internal/eventstream"
)

func TestExtractParticipantsFromITIP(t *testing.T) {
	t.Parallel()

	icsBody := `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Test//EN
BEGIN:VEVENT
UID:event-1@example.com
DTSTART:20260501T090000Z
DTEND:20260501T100000Z
ORGANIZER;CN="Organizer":mailto:organizer@example.com
ATTENDEE;CN="Attendee 1":mailto:attendee1@example.com
ATTENDEE;CN="Attendee 2":mailto:attendee2@example.com
SUMMARY:Test Event
END:VEVENT
END:VCALENDAR
`

	attendees, organizer, err := extractParticipants([]byte(icsBody))
	if err != nil {
		t.Fatalf("extractParticipants returned error: %v", err)
	}

	if organizer != "organizer@example.com" {
		t.Errorf("organizer = %q, want %q", organizer, "organizer@example.com")
	}

	if len(attendees) != 2 {
		t.Errorf("len(attendees) = %d, want 2", len(attendees))
	}

	if attendees[0] != "attendee1@example.com" {
		t.Errorf("attendees[0] = %q, want %q", attendees[0], "attendee1@example.com")
	}

	if attendees[1] != "attendee2@example.com" {
		t.Errorf("attendees[1] = %q, want %q", attendees[1], "attendee2@example.com")
	}
}

func TestDirectPropAccess(t *testing.T) {
	t.Parallel()

	icsBody := `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Test//EN
BEGIN:VEVENT
UID:event-1@example.com
ORGANIZER;CN="Organizer":mailto:organizer@example.com
ATTENDEE:mailto:attendee1@example.com
END:VEVENT
END:VCALENDAR
`

	cal, err := ical.NewDecoder(bytes.NewReader([]byte(icsBody))).Decode()
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}

	child := cal.Component.Children[0]

	props, ok := child.Props["ORGANIZER"]
	t.Logf("ORGANIZER props: ok=%v, len=%d", ok, len(props))
	if ok && len(props) > 0 {
		prop := props[0]
		t.Logf("ORGANIZER Value = %q", prop.Value)
		uri, err := prop.URI()
		t.Logf("ORGANIZER URI: %v, err=%v", uri, err)
		if uri != nil {
			t.Logf("ORGANIZER URI.String() = %q", uri.String())
		}
	}

	props, ok = child.Props[ical.PropAttendee]
	t.Logf("PropAttendee props: ok=%v, len=%d", ok, len(props))
	if ok && len(props) > 0 {
		prop := props[0]
		t.Logf("ATTENDEE Value = %q", prop.Value)
		uri, err := prop.URI()
		t.Logf("ATTENDEE URI: %v, err=%v", uri, err)
		if uri != nil {
			t.Logf("ATTENDEE URI.String() = %q", uri.String())
		}
	}
}

func TestExtractParticipantsNoOrganizer(t *testing.T) {
	t.Parallel()

	icsBody := `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Test//EN
BEGIN:VEVENT
UID:event-1@example.com
DTSTART:20260501T090000Z
DTEND:20260501T100000Z
ATTENDEE:mailto:attendee1@example.com
SUMMARY:Test Event
END:VEVENT
END:VCALENDAR
`

	attendees, organizer, err := extractParticipants([]byte(icsBody))
	if err != nil {
		t.Fatalf("extractParticipants returned error: %v", err)
	}

	if organizer != "" {
		t.Errorf("organizer = %q, want empty", organizer)
	}

	if len(attendees) != 1 {
		t.Errorf("len(attendees) = %d, want 1", len(attendees))
	}
}

func TestDetectMethod(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		icsBody     string
		wantMethod  string
	}{
		{"REQUEST method", "BEGIN:VCALENDAR\r\nMETHOD:REQUEST\r\nEND:VCALENDAR", "REQUEST"},
		{"CANCEL method", "BEGIN:VCALENDAR\r\nMETHOD:CANCEL\r\nEND:VCALENDAR", "CANCEL"},
		{"PUBLISH method", "BEGIN:VCALENDAR\r\nMETHOD:PUBLISH\r\nEND:VCALENDAR", "PUBLISH"},
		{"REPLY method", "BEGIN:VCALENDAR\r\nMETHOD:REPLY\r\nEND:VCALENDAR", "REPLY"},
		{"no method defaults to REQUEST", "BEGIN:VCALENDAR\r\nEND:VCALENDAR", "REQUEST"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectMethod([]byte(tt.icsBody))
			if got != tt.wantMethod {
				t.Errorf("detectMethod = %q, want %q", got, tt.wantMethod)
			}
		})
	}
}

func TestITIPSubject(t *testing.T) {
	t.Parallel()

	tests := []struct {
		method string
		uid    string
		want   string
	}{
		{"REQUEST", "uid-1", "Meeting Request"},
		{"CANCEL", "uid-1", "Cancelled: Calendar Event"},
		{"REPLY", "uid-1", "Accepted: Calendar Event"},
		{"COUNTER", "uid-1", "Counter Proposal: Calendar Event"},
		{"DECLINECOUNTER", "uid-1", "Declined: Counter Proposal"},
		{"ADD", "uid-1", "Additional Instance: Calendar Event"},
		{"REFRESH", "uid-1", "Meeting Request Update"},
		{"PUBLISH", "uid-1", "Meeting Request"},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			got := iTIPSubject(tt.method, tt.uid)
			if got != tt.want {
				t.Errorf("iTIPSubject(%q, %q) = %q, want %q", tt.method, tt.uid, got, tt.want)
			}
		})
	}
}

func TestBuildITIPMessage(t *testing.T) {
	t.Parallel()

	icsBody := `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Test//EN
BEGIN:VEVENT
UID:event-1@example.com
DTSTART:20260501T090000Z
DTEND:20260501T100000Z
SUMMARY:Test Event
END:VEVENT
END:VCALENDAR
`

	msg, err := buildITIPMessage([]byte(icsBody), "REQUEST", "mailto:organizer@example.com")
	if err != nil {
		t.Fatalf("buildITIPMessage returned error: %v", err)
	}

	if len(msg) == 0 {
		t.Error("buildITIPMessage returned empty message")
	}

	if !strings.Contains(string(msg), "METHOD:REQUEST") {
		t.Error("buildITIPMessage missing METHOD:REQUEST")
	}

	if !strings.Contains(string(msg), "PRODID:-//gogomail//CalDAV iMIP//EN") {
		t.Error("buildITIPMessage missing PRODID")
	}

	if !strings.Contains(string(msg), "BEGIN:VEVENT") {
		t.Error("buildITIPMessage missing VEVENT")
	}
}

func TestHandlerHandleEventNoQueueStore(t *testing.T) {
	t.Parallel()

	handler := NewHandler(nil, nil, nil, nil)

	eventPayload := `{"event":"scheduling.outbox","schema_version":"2026-05-08.scheduling.v1","dav_kind":"caldav-scheduling","user_id":"user-1","uid":"event-1","method":"REQUEST","payload":"BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Test//EN\r\nBEGIN:VEVENT\r\nUID:event-1@example.com\r\nDTSTART:20260501T090000Z\r\nDTEND:20260501T100000Z\r\nORGANIZER:mailto:organizer@example.com\r\nATTENDEE:mailto:attendee@example.com\r\nSUMMARY:Test Event\r\nEND:VEVENT\r\nEND:VCALENDAR"}`

	event := eventstream.Message{
		ID:           "event-1",
		Stream:       "scheduling.outbox",
		OutboxID:     "outbox-1",
		PartitionKey: "user-1",
		Payload:      []byte(eventPayload),
	}

	err := handler.HandleEvent(context.Background(), event)
	if err != nil {
		t.Fatalf("HandleEvent returned error: %v", err)
	}
}

func TestExtractCalAddress(t *testing.T) {
	t.Parallel()

	icsBody := `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Test//EN
BEGIN:VEVENT
UID:event-1@example.com
ORGANIZER;CN="Organizer":mailto:organizer@example.com
ATTENDEE:mailto:attendee1@example.com
END:VEVENT
END:VCALENDAR
`

	cal, err := ical.NewDecoder(bytes.NewReader([]byte(icsBody))).Decode()
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}

	child := cal.Component.Children[0]

	props := child.Props[ical.PropOrganizer]
	prop := props[0]
	t.Logf("ORGANIZER Value = %q", prop.Value)

	uri, err := prop.URI()
	t.Logf("ORGANIZER URI: %v, err=%v", uri, err)
	if uri != nil {
		t.Logf("ORGANIZER URI.String() = %q", uri.String())
		parsed, err := url.Parse(uri.String())
		t.Logf("parsed: %v, err=%v", parsed, err)
		if parsed != nil {
			t.Logf("parsed.Path = %q", parsed.Path)
		}
	}
}

type fakeQueue struct{}

func (f *fakeQueue) Enqueue(ctx context.Context, topic string, partitionKey string, payload []byte) error {
	return nil
}

type fakeStore struct{}

func (f *fakeStore) Put(ctx context.Context, path string, r io.Reader) error {
	return nil
}

func TestDefaultAttendeeResolverWithNilReposReturnsExternal(t *testing.T) {
	t.Parallel()

	resolver := NewDefaultAttendeeResolver(nil, nil)
	resolutions, err := resolver.ResolveAttendees(context.Background(), "user-1", []string{"external@example.com"})
	if err != nil {
		t.Fatalf("ResolveAttendees returned error: %v", err)
	}
	if len(resolutions) != 1 {
		t.Fatalf("len(resolutions) = %d, want 1", len(resolutions))
	}
	if resolutions[0].Kind != AttendeeKindExternal {
		t.Errorf("resolutions[0].Kind = %v, want %v", resolutions[0].Kind, AttendeeKindExternal)
	}
	if resolutions[0].Address != "external@example.com" {
		t.Errorf("resolutions[0].Address = %q, want %q", resolutions[0].Address, "external@example.com")
	}
}

func TestDefaultAttendeeResolverWithNilReposMultipleAddresses(t *testing.T) {
	t.Parallel()

	resolver := NewDefaultAttendeeResolver(nil, nil)
	addresses := []string{"user1@example.com", "user2@example.com", "external@example.com"}
	resolutions, err := resolver.ResolveAttendees(context.Background(), "user-1", addresses)
	if err != nil {
		t.Fatalf("ResolveAttendees returned error: %v", err)
	}
	if len(resolutions) != 3 {
		t.Fatalf("len(resolutions) = %d, want 3", len(resolutions))
	}
	for i, res := range resolutions {
		if res.Kind != AttendeeKindExternal {
			t.Errorf("resolutions[%d].Kind = %v, want %v", i, res.Kind, AttendeeKindExternal)
		}
		if res.Address != addresses[i] {
			t.Errorf("resolutions[%d].Address = %q, want %q", i, res.Address, addresses[i])
		}
	}
}