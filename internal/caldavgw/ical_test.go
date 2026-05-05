package caldavgw

import (
	"strings"
	"testing"
	"time"
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

func TestCalendarObjectMatchesTimeRange(t *testing.T) {
	t.Parallel()

	body := []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//gogomail//CalDAV Test//EN\r\nBEGIN:VEVENT\r\nUID:event-1@example.com\r\nDTSTAMP:20260506T000000Z\r\nDTSTART:20260506T010000Z\r\nDTEND:20260506T020000Z\r\nSUMMARY:Planning\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n")
	matches, err := CalendarObjectMatchesTimeRange(body, &TimeRange{
		Start: mustCalDAVTime(t, "20260506T013000Z"),
		End:   mustCalDAVTime(t, "20260506T030000Z"),
	})
	if err != nil {
		t.Fatalf("CalendarObjectMatchesTimeRange returned error: %v", err)
	}
	if !matches {
		t.Fatal("matches = false, want true")
	}
	matches, err = CalendarObjectMatchesTimeRange(body, &TimeRange{
		Start: mustCalDAVTime(t, "20260507T000000Z"),
		End:   mustCalDAVTime(t, "20260508T000000Z"),
	})
	if err != nil {
		t.Fatalf("CalendarObjectMatchesTimeRange returned error: %v", err)
	}
	if matches {
		t.Fatal("matches = true, want false")
	}
}

func TestCalendarObjectBusyPeriodsFiltersOpaqueConfirmedEvents(t *testing.T) {
	t.Parallel()

	body := []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//gogomail//CalDAV Test//EN\r\nBEGIN:VEVENT\r\nUID:busy@example.com\r\nDTSTAMP:20260506T000000Z\r\nDTSTART:20260506T010000Z\r\nDTEND:20260506T020000Z\r\nSUMMARY:Busy\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n")
	periods, err := CalendarObjectBusyPeriods(body, TimeRange{
		Start: mustCalDAVTime(t, "20260506T013000Z"),
		End:   mustCalDAVTime(t, "20260506T030000Z"),
	})
	if err != nil {
		t.Fatalf("CalendarObjectBusyPeriods returned error: %v", err)
	}
	if len(periods) != 1 {
		t.Fatalf("periods = %+v, want one clipped busy period", periods)
	}
	if got := periods[0].Start.Format("20060102T150405Z"); got != "20260506T013000Z" {
		t.Fatalf("period start = %s", got)
	}
	if got := periods[0].End.Format("20060102T150405Z"); got != "20260506T020000Z" {
		t.Fatalf("period end = %s", got)
	}
}

func TestCalendarObjectBusyPeriodsSkipsTransparentAndCancelledEvents(t *testing.T) {
	t.Parallel()

	for name, body := range map[string][]byte{
		"transparent": []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VEVENT\r\nUID:transparent@example.com\r\nDTSTAMP:20260506T000000Z\r\nDTSTART:20260506T010000Z\r\nDTEND:20260506T020000Z\r\nTRANSP:TRANSPARENT\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"),
		"cancelled":   []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VEVENT\r\nUID:cancelled@example.com\r\nDTSTAMP:20260506T000000Z\r\nDTSTART:20260506T010000Z\r\nDTEND:20260506T020000Z\r\nSTATUS:CANCELLED\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"),
	} {
		name, body := name, body
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			periods, err := CalendarObjectBusyPeriods(body, TimeRange{
				Start: mustCalDAVTime(t, "20260506T000000Z"),
				End:   mustCalDAVTime(t, "20260507T000000Z"),
			})
			if err != nil {
				t.Fatalf("CalendarObjectBusyPeriods returned error: %v", err)
			}
			if len(periods) != 0 {
				t.Fatalf("periods = %+v, want none", periods)
			}
		})
	}
}

func TestCalendarObjectBusyPeriodsMarksTentativeEvents(t *testing.T) {
	t.Parallel()

	body := []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VEVENT\r\nUID:tentative@example.com\r\nDTSTAMP:20260506T000000Z\r\nDTSTART:20260506T010000Z\r\nDTEND:20260506T020000Z\r\nSTATUS:TENTATIVE\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n")
	periods, err := CalendarObjectBusyPeriods(body, TimeRange{
		Start: mustCalDAVTime(t, "20260506T000000Z"),
		End:   mustCalDAVTime(t, "20260507T000000Z"),
	})
	if err != nil {
		t.Fatalf("CalendarObjectBusyPeriods returned error: %v", err)
	}
	if len(periods) != 1 || periods[0].Type != "BUSY-TENTATIVE" {
		t.Fatalf("periods = %+v, want tentative busy", periods)
	}
}

func TestCoalesceBusyPeriodsMergesOverlappingSameTypes(t *testing.T) {
	t.Parallel()

	periods := CoalesceBusyPeriods([]BusyPeriod{
		{Start: mustCalDAVTime(t, "20260506T010000Z"), End: mustCalDAVTime(t, "20260506T020000Z"), Type: "BUSY"},
		{Start: mustCalDAVTime(t, "20260506T013000Z"), End: mustCalDAVTime(t, "20260506T030000Z"), Type: "BUSY"},
		{Start: mustCalDAVTime(t, "20260506T040000Z"), End: mustCalDAVTime(t, "20260506T050000Z"), Type: "BUSY-TENTATIVE"},
	})
	if len(periods) != 2 {
		t.Fatalf("periods = %+v, want 2 coalesced periods", periods)
	}
	if got := periods[0].End.Format("20060102T150405Z"); got != "20260506T030000Z" {
		t.Fatalf("coalesced end = %s", got)
	}
	if periods[1].Type != "BUSY-TENTATIVE" {
		t.Fatalf("second period = %+v", periods[1])
	}
}

func TestBuildFreeBusyCalendar(t *testing.T) {
	t.Parallel()

	body, err := BuildFreeBusyCalendar("user-1", "work", TimeRange{
		Start: mustCalDAVTime(t, "20260506T000000Z"),
		End:   mustCalDAVTime(t, "20260507T000000Z"),
	}, []BusyPeriod{{
		Start: mustCalDAVTime(t, "20260506T010000Z"),
		End:   mustCalDAVTime(t, "20260506T020000Z"),
	}})
	if err != nil {
		t.Fatalf("BuildFreeBusyCalendar returned error: %v", err)
	}
	text := string(body)
	for _, want := range []string{
		"BEGIN:VCALENDAR",
		"METHOD:REPLY",
		"BEGIN:VFREEBUSY",
		"DTSTART:20260506T000000Z",
		"DTEND:20260507T000000Z",
		"FREEBUSY;FBTYPE=BUSY:20260506T010000Z/20260506T020000Z",
		"END:VFREEBUSY",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("free-busy calendar missing %q:\n%s", want, text)
		}
	}
}

func mustCalDAVTime(t *testing.T, value string) time.Time {
	t.Helper()

	parsed, err := parseICalendarUTC(value)
	if err != nil {
		t.Fatalf("parseICalendarUTC(%q): %v", value, err)
	}
	return parsed
}
