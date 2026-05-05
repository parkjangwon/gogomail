package caldavgw

import (
	"strings"
	"testing"
)

func TestParseICalendarObjectExtractsUIDAndComponent(t *testing.T) {
	t.Parallel()

	body := []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//gogomail//CalDAV Test//EN\r\nBEGIN:VEVENT\r\nUID:event-1@example.com\r\nDTSTAMP:20260506T000000Z\r\nDTSTART:20260506T010000Z\r\nDTEND:20260506T020000Z\r\nSUMMARY:Planning\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n")
	object, err := ParseICalendarObject(body)
	if err != nil {
		t.Fatalf("ParseICalendarObject returned error: %v", err)
	}
	if object.UID != "event-1@example.com" || object.Component != ComponentVEVENT {
		t.Fatalf("object = %+v", object)
	}
}

func TestParseICalendarObjectAllowsTimezonePlusOneCalendarComponent(t *testing.T) {
	t.Parallel()

	body := []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//gogomail//CalDAV Test//EN\r\nBEGIN:VTIMEZONE\r\nTZID:Asia/Seoul\r\nEND:VTIMEZONE\r\nBEGIN:VTODO\r\nUID:todo-1@example.com\r\nDTSTAMP:20260506T000000Z\r\nSUMMARY:Follow up\r\nEND:VTODO\r\nEND:VCALENDAR\r\n")
	object, err := ParseICalendarObject(body)
	if err != nil {
		t.Fatalf("ParseICalendarObject returned error: %v", err)
	}
	if object.UID != "todo-1@example.com" || object.Component != ComponentVTODO {
		t.Fatalf("object = %+v", object)
	}
}

func TestParseICalendarObjectRejectsInvalidObjects(t *testing.T) {
	t.Parallel()

	tests := map[string][]byte{
		"empty":             nil,
		"not calendar":      []byte("BEGIN:VEVENT\r\nUID:event-1@example.com\r\nEND:VEVENT\r\n"),
		"missing uid":       []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VEVENT\r\nSUMMARY:No UID\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"),
		"duplicate uid":     []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VEVENT\r\nUID:a@example.com\r\nUID:b@example.com\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"),
		"multiple objects":  []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VEVENT\r\nUID:a@example.com\r\nEND:VEVENT\r\nBEGIN:VTODO\r\nUID:b@example.com\r\nEND:VTODO\r\nEND:VCALENDAR\r\n"),
		"unsupported only":  []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VALARM\r\nACTION:DISPLAY\r\nEND:VALARM\r\nEND:VCALENDAR\r\n"),
		"oversized uid":     []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VEVENT\r\nUID:" + strings.Repeat("u", MaxICalendarUIDBytes+1) + "\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"),
		"too many children": []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VEVENT\r\nUID:event-1@example.com\r\n" + strings.Repeat("BEGIN:VALARM\r\nACTION:DISPLAY\r\nEND:VALARM\r\n", MaxICalendarComponents+1) + "END:VEVENT\r\nEND:VCALENDAR\r\n"),
	}
	for name, body := range tests {
		name, body := name, body
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if _, err := ParseICalendarObject(body); err == nil {
				t.Fatal("ParseICalendarObject error = nil, want rejection")
			}
		})
	}
}

func TestValidateUpsertObjectRequestCanDeriveMetadataFromICalendarBody(t *testing.T) {
	t.Parallel()

	body := []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//gogomail//CalDAV Test//EN\r\nBEGIN:VJOURNAL\r\nUID:journal-1@example.com\r\nDTSTAMP:20260506T000000Z\r\nSUMMARY:Note\r\nEND:VJOURNAL\r\nEND:VCALENDAR\r\n")
	req, _, _, err := ValidateUpsertObjectRequest(UpsertObjectRequest{
		UserID:     "user-1",
		CalendarID: "calendar-1",
		ObjectName: "journal.ics",
		ICS:        body,
	})
	if err != nil {
		t.Fatalf("ValidateUpsertObjectRequest returned error: %v", err)
	}
	if req.UID != "journal-1@example.com" || req.Component != ComponentVJOURNAL {
		t.Fatalf("request = %+v", req)
	}
}

func TestValidateUpsertObjectRequestRejectsMetadataMismatch(t *testing.T) {
	t.Parallel()

	body := []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//gogomail//CalDAV Test//EN\r\nBEGIN:VEVENT\r\nUID:event-1@example.com\r\nDTSTAMP:20260506T000000Z\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n")
	if _, _, _, err := ValidateUpsertObjectRequest(UpsertObjectRequest{
		UserID:     "user-1",
		CalendarID: "calendar-1",
		ObjectName: "event.ics",
		UID:        "other@example.com",
		ICS:        body,
	}); err == nil {
		t.Fatal("ValidateUpsertObjectRequest accepted mismatched UID")
	}
	if _, _, _, err := ValidateUpsertObjectRequest(UpsertObjectRequest{
		UserID:     "user-1",
		CalendarID: "calendar-1",
		ObjectName: "event.ics",
		Component:  ComponentVTODO,
		ICS:        body,
	}); err == nil {
		t.Fatal("ValidateUpsertObjectRequest accepted mismatched component")
	}
}
