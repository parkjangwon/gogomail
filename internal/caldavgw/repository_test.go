package caldavgw

import (
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestValidateCreateCalendarRequest(t *testing.T) {
	t.Parallel()

	req, normalizedName, syncToken, err := ValidateCreateCalendarRequest(CreateCalendarRequest{
		UserID:      " user-1 ",
		Name:        " Work ",
		Color:       " #aabbcc ",
		Description: " Team calendar ",
	})
	if err != nil {
		t.Fatalf("ValidateCreateCalendarRequest returned error: %v", err)
	}
	if req.UserID != "user-1" || req.Name != "Work" || req.Color != "#AABBCC" || req.Description != "Team calendar" {
		t.Fatalf("request = %+v", req)
	}
	if normalizedName != "work" {
		t.Fatalf("normalized name = %q", normalizedName)
	}
	if !strings.HasPrefix(syncToken, "sync-") {
		t.Fatalf("sync token = %q", syncToken)
	}
}

func TestValidateCreateCalendarAtPathRequest(t *testing.T) {
	t.Parallel()

	req, normalizedName, syncToken, err := ValidateCreateCalendarAtPathRequest(CreateCalendarAtPathRequest{
		UserID:      " user-1 ",
		CalendarID:  "11111111-1111-4111-8111-111111111111",
		Name:        " Project ",
		Color:       " #aabbcc ",
		Description: " Milestones ",
	})
	if err != nil {
		t.Fatalf("ValidateCreateCalendarAtPathRequest returned error: %v", err)
	}
	if req.UserID != "user-1" || req.CalendarID != "11111111-1111-4111-8111-111111111111" || req.Name != "Project" {
		t.Fatalf("request = %+v", req)
	}
	if normalizedName != "project" {
		t.Fatalf("normalized name = %q", normalizedName)
	}
	if !strings.HasPrefix(syncToken, "sync-") {
		t.Fatalf("sync token = %q", syncToken)
	}
}

func TestValidateCreateCalendarRequestRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	tests := []CreateCalendarRequest{
		{Name: "Work"},
		{UserID: "user\n1", Name: "Work"},
		{UserID: "user-1", Name: "bad\nname"},
		{UserID: "user-1", Name: "Work", Color: "blue"},
		{UserID: "user-1", Name: "Work", Description: "bad\nline"},
	}
	for _, req := range tests {
		req := req
		t.Run(req.UserID+"/"+req.Name, func(t *testing.T) {
			t.Parallel()

			if _, _, _, err := ValidateCreateCalendarRequest(req); err == nil {
				t.Fatalf("ValidateCreateCalendarRequest(%+v) error = nil, want rejection", req)
			}
		})
	}
}

func TestValidateCreateCalendarAtPathRequestRejectsNonUUIDPathIDs(t *testing.T) {
	t.Parallel()

	for _, calendarID := range []string{"work", "11111111-1111-4111-8111-11111111111z", "11111111111141118111111111111111"} {
		calendarID := calendarID
		t.Run(calendarID, func(t *testing.T) {
			t.Parallel()

			if _, _, _, err := ValidateCreateCalendarAtPathRequest(CreateCalendarAtPathRequest{
				UserID:     "user-1",
				CalendarID: calendarID,
				Name:       "Work",
			}); err == nil {
				t.Fatal("ValidateCreateCalendarAtPathRequest error = nil, want rejection")
			}
		})
	}
}

func TestMapCalendarObjectUpsertErrorMapsUniqueUIDViolation(t *testing.T) {
	t.Parallel()

	err := mapCalendarObjectUpsertError(&pgconn.PgError{
		Code:           "23505",
		ConstraintName: "idx_caldav_calendar_objects_active_uid",
	})
	if err == nil || !strings.Contains(err.Error(), "UID already exists") {
		t.Fatalf("mapped error = %v, want duplicate UID context", err)
	}
}

func TestMapCalendarObjectUpsertErrorMapsUniqueNameViolation(t *testing.T) {
	t.Parallel()

	err := mapCalendarObjectUpsertError(&pgconn.PgError{
		Code:           "23505",
		ConstraintName: "idx_caldav_calendar_objects_active_name",
	})
	if err == nil || !strings.Contains(err.Error(), "object already exists") {
		t.Fatalf("mapped error = %v, want duplicate object context", err)
	}
}

func TestValidateUpdateCalendarRequest(t *testing.T) {
	t.Parallel()

	name := " Product "
	color := " #aabbcc "
	description := " Launch dates "
	req, normalizedName, syncToken, err := ValidateUpdateCalendarRequest(UpdateCalendarRequest{
		UserID:      " user-1 ",
		CalendarID:  " calendar-1 ",
		Name:        &name,
		Color:       &color,
		Description: &description,
	})
	if err != nil {
		t.Fatalf("ValidateUpdateCalendarRequest returned error: %v", err)
	}
	if req.UserID != "user-1" || req.CalendarID != "calendar-1" {
		t.Fatalf("request ids = %+v", req)
	}
	if req.Name == nil || *req.Name != "Product" || normalizedName != "product" {
		t.Fatalf("name = %v normalized = %q", req.Name, normalizedName)
	}
	if req.Color == nil || *req.Color != "#AABBCC" {
		t.Fatalf("color = %v", req.Color)
	}
	if req.Description == nil || *req.Description != "Launch dates" {
		t.Fatalf("description = %v", req.Description)
	}
	if !strings.HasPrefix(syncToken, "sync-") {
		t.Fatalf("sync token = %q", syncToken)
	}
}

func TestValidateUpdateCalendarRequestRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	validName := "Work"
	badName := "bad\nname"
	badColor := "blue"
	badDescription := "bad\nline"
	tests := []UpdateCalendarRequest{
		{CalendarID: "calendar-1", Name: &validName},
		{UserID: "user-1", CalendarID: "calendar-1"},
		{UserID: "user-1", CalendarID: "calendar-1", Name: &badName},
		{UserID: "user-1", CalendarID: "calendar-1", Color: &badColor},
		{UserID: "user-1", CalendarID: "calendar-1", Description: &badDescription},
	}
	for _, req := range tests {
		req := req
		t.Run(req.UserID+"/"+req.CalendarID, func(t *testing.T) {
			t.Parallel()

			if _, _, _, err := ValidateUpdateCalendarRequest(req); err == nil {
				t.Fatalf("ValidateUpdateCalendarRequest(%+v) error = nil, want rejection", req)
			}
		})
	}
}

func TestValidateUpsertObjectRequest(t *testing.T) {
	t.Parallel()

	body := []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//gogomail//CalDAV Test//EN\r\nBEGIN:VEVENT\r\nUID:event-1@example.com\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n")
	etag, err := StrongETag(body)
	if err != nil {
		t.Fatalf("StrongETag returned error: %v", err)
	}
	req, gotETag, syncToken, err := ValidateUpsertObjectRequest(UpsertObjectRequest{
		UserID:       " user-1 ",
		CalendarID:   " calendar-1 ",
		ObjectName:   " event-1.ics ",
		UID:          " event-1@example.com ",
		Component:    "vevent",
		ICS:          body,
		ObservedETag: etag,
	})
	if err != nil {
		t.Fatalf("ValidateUpsertObjectRequest returned error: %v", err)
	}
	if req.UserID != "user-1" || req.CalendarID != "calendar-1" || req.ObjectName != "event-1.ics" {
		t.Fatalf("request ids = %+v", req)
	}
	if req.UID != "event-1@example.com" || req.Component != ComponentVEVENT {
		t.Fatalf("request metadata = %+v", req)
	}
	if gotETag != etag || req.ObservedETag != etag {
		t.Fatalf("etag = %q observed = %q want %q", gotETag, req.ObservedETag, etag)
	}
	if !strings.HasPrefix(syncToken, "sync-") {
		t.Fatalf("sync token = %q", syncToken)
	}
}

func TestValidateUpsertObjectRequestRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	validBody := []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//gogomail//CalDAV Test//EN\r\nBEGIN:VEVENT\r\nUID:event-1@example.com\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n")
	tests := []UpsertObjectRequest{
		{CalendarID: "calendar-1", ObjectName: "event.ics", UID: "uid", ICS: validBody},
		{UserID: "user-1", ObjectName: "event.ics", UID: "uid", ICS: validBody},
		{UserID: "user-1", CalendarID: "calendar-1", ObjectName: "event.txt", UID: "uid", ICS: validBody},
		{UserID: "user-1", CalendarID: "calendar-1", ObjectName: "bad/name.ics", UID: "uid", ICS: validBody},
		{UserID: "user-1", CalendarID: "calendar-1", ObjectName: "event.ics", UID: "bad\nuid", ICS: validBody},
		{UserID: "user-1", CalendarID: "calendar-1", ObjectName: "event.ics", UID: "uid", Component: "VALARM", ICS: validBody},
		{UserID: "user-1", CalendarID: "calendar-1", ObjectName: "event.ics", UID: "uid"},
		{UserID: "user-1", CalendarID: "calendar-1", ObjectName: "event.ics", UID: "uid", ICS: validBody, ObservedETag: `"ABC"`},
	}
	for _, req := range tests {
		req := req
		t.Run(req.ObjectName+"/"+req.UID, func(t *testing.T) {
			t.Parallel()

			if _, _, _, err := ValidateUpsertObjectRequest(req); err == nil {
				t.Fatalf("ValidateUpsertObjectRequest(%+v) error = nil, want rejection", req)
			}
		})
	}
}

func TestValidateObjectReadAndDeleteRequests(t *testing.T) {
	t.Parallel()

	list, err := ValidateListObjectsRequest(ListObjectsRequest{
		UserID:     " user-1 ",
		CalendarID: " calendar-1 ",
		Limit:      5000,
	})
	if err != nil {
		t.Fatalf("ValidateListObjectsRequest returned error: %v", err)
	}
	if list.Status != CalendarStatusActive || list.Limit != 1000 {
		t.Fatalf("list request = %+v", list)
	}
	syncList, err := ValidateListObjectsForSyncRequest(ListObjectsRequest{
		UserID:     " user-1 ",
		CalendarID: " calendar-1 ",
		Limit:      MaxWebDAVReportLimit + 1,
	})
	if err != nil {
		t.Fatalf("ValidateListObjectsForSyncRequest returned error: %v", err)
	}
	if syncList.Limit != MaxWebDAVReportLimit+1 {
		t.Fatalf("sync list request = %+v", syncList)
	}
	get, err := ValidateGetObjectRequest(GetObjectRequest{
		UserID:     " user-1 ",
		CalendarID: " calendar-1 ",
		ObjectName: " event.ics ",
		Status:     CalendarStatusDeleted,
	})
	if err != nil {
		t.Fatalf("ValidateGetObjectRequest returned error: %v", err)
	}
	if get.ObjectName != "event.ics" || get.Status != CalendarStatusDeleted {
		t.Fatalf("get request = %+v", get)
	}
	deleted, syncToken, err := ValidateDeleteObjectRequest(DeleteObjectRequest{
		UserID:     " user-1 ",
		CalendarID: " calendar-1 ",
		ObjectName: " event.ics ",
	})
	if err != nil {
		t.Fatalf("ValidateDeleteObjectRequest returned error: %v", err)
	}
	if deleted.ObjectName != "event.ics" || !strings.HasPrefix(syncToken, "sync-") {
		t.Fatalf("delete request = %+v token = %q", deleted, syncToken)
	}
	deleteCalendar, err := ValidateDeleteCalendarRequest(DeleteCalendarRequest{
		UserID:     " user-1 ",
		CalendarID: " calendar-1 ",
	})
	if err != nil {
		t.Fatalf("ValidateDeleteCalendarRequest returned error: %v", err)
	}
	if deleteCalendar.UserID != "user-1" || deleteCalendar.CalendarID != "calendar-1" {
		t.Fatalf("delete calendar request = %+v", deleteCalendar)
	}
	if _, err := ValidateDeleteCalendarRequest(DeleteCalendarRequest{UserID: "user-1"}); err == nil {
		t.Fatal("ValidateDeleteCalendarRequest accepted missing calendar id")
	}
	changes, err := ValidateListChangesSinceRequest(ListChangesSinceRequest{
		UserID:     " user-1 ",
		CalendarID: " calendar-1 ",
		SyncToken:  " sync-123 ",
		Limit:      5000,
	})
	if err != nil {
		t.Fatalf("ValidateListChangesSinceRequest returned error: %v", err)
	}
	if changes.SyncToken != "sync-123" || changes.Limit != MaxWebDAVReportLimit+1 {
		t.Fatalf("changes request = %+v", changes)
	}
	if _, err := ValidateListChangesSinceRequest(ListChangesSinceRequest{UserID: "user-1", CalendarID: "calendar-1"}); err == nil {
		t.Fatal("ValidateListChangesSinceRequest accepted missing sync token")
	}
}

func TestValidateCalendarObjectName(t *testing.T) {
	t.Parallel()

	name, err := ValidateCalendarObjectName(" Event.ICS ")
	if err != nil {
		t.Fatalf("ValidateCalendarObjectName returned error: %v", err)
	}
	if name != "Event.ICS" {
		t.Fatalf("name = %q", name)
	}
	for _, value := range []string{"", "event.txt", "bad/name.ics", "bad\\name.ics", "bad\nname.ics", strings.Repeat("x", MaxCalendarObjectNameBytes+1) + ".ics"} {
		value := value
		t.Run(value, func(t *testing.T) {
			t.Parallel()

			if _, err := ValidateCalendarObjectName(value); err == nil {
				t.Fatalf("ValidateCalendarObjectName(%q) error = nil, want rejection", value)
			}
		})
	}
}
