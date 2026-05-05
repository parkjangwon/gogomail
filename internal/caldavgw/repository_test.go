package caldavgw

import (
	"strings"
	"testing"
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

func TestValidateUpsertObjectRequest(t *testing.T) {
	t.Parallel()

	body := []byte("BEGIN:VCALENDAR\r\nBEGIN:VEVENT\r\nUID:event-1@example.com\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n")
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

	validBody := []byte("BEGIN:VCALENDAR\r\nBEGIN:VEVENT\r\nUID:event-1@example.com\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n")
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
