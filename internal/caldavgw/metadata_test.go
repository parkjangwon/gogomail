package caldavgw

import (
	"strings"
	"testing"
)

func TestCalendarMetadataValidation(t *testing.T) {
	t.Parallel()

	name, err := ValidateCalendarName(" Work ")
	if err != nil {
		t.Fatalf("ValidateCalendarName returned error: %v", err)
	}
	if name != "Work" {
		t.Fatalf("name = %q, want Work", name)
	}
	normalized, err := NormalizeCalendarName(" Work ")
	if err != nil {
		t.Fatalf("NormalizeCalendarName returned error: %v", err)
	}
	if normalized != "work" {
		t.Fatalf("normalized = %q, want work", normalized)
	}
	color, err := ValidateCalendarColor(" #a0B1c2 ")
	if err != nil {
		t.Fatalf("ValidateCalendarColor returned error: %v", err)
	}
	if color != "#A0B1C2" {
		t.Fatalf("color = %q, want uppercase hex", color)
	}
	description, err := ValidateCalendarDescription(" Team calendar ")
	if err != nil {
		t.Fatalf("ValidateCalendarDescription returned error: %v", err)
	}
	if description != "Team calendar" {
		t.Fatalf("description = %q", description)
	}
}

func TestCalendarMetadataRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"", "bad\nname", strings.Repeat("n", MaxCalendarNameBytes+1)} {
		if _, err := ValidateCalendarName(name); err == nil {
			t.Fatalf("ValidateCalendarName(%q) error = nil, want rejection", name)
		}
	}
	for _, color := range []string{"red", "#xyz123", "#12345"} {
		if _, err := ValidateCalendarColor(color); err == nil {
			t.Fatalf("ValidateCalendarColor(%q) error = nil, want rejection", color)
		}
	}
	if _, err := ValidateCalendarDescription("line\nbreak"); err == nil {
		t.Fatal("ValidateCalendarDescription accepted line break")
	}
}

func TestCalendarObjectMetadata(t *testing.T) {
	t.Parallel()

	uid, err := ValidateCalendarObjectUID(" event-1@example.com ")
	if err != nil {
		t.Fatalf("ValidateCalendarObjectUID returned error: %v", err)
	}
	if uid != "event-1@example.com" {
		t.Fatalf("uid = %q", uid)
	}
	component, err := ValidateCalendarComponent("vtodo")
	if err != nil {
		t.Fatalf("ValidateCalendarComponent returned error: %v", err)
	}
	if component != ComponentVTODO {
		t.Fatalf("component = %q, want VTODO", component)
	}
	etag, err := StrongETag([]byte("BEGIN:VCALENDAR\r\nEND:VCALENDAR\r\n"))
	if err != nil {
		t.Fatalf("StrongETag returned error: %v", err)
	}
	if !strings.HasPrefix(etag, `"`) || !strings.HasSuffix(etag, `"`) || len(etag) != 66 {
		t.Fatalf("etag = %q, want quoted sha256", etag)
	}
	token, err := SyncTokenForETag(etag)
	if err != nil {
		t.Fatalf("SyncTokenForETag returned error: %v", err)
	}
	if !strings.HasPrefix(token, "sync-") {
		t.Fatalf("sync token = %q", token)
	}
}

func TestCalendarObjectMetadataRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	for _, uid := range []string{"", "uid\nbad", strings.Repeat("u", MaxCalendarObjectUIDBytes+1)} {
		if _, err := ValidateCalendarObjectUID(uid); err == nil {
			t.Fatalf("ValidateCalendarObjectUID(%q) error = nil, want rejection", uid)
		}
	}
	if _, err := ValidateCalendarComponent("VALARM"); err == nil {
		t.Fatal("ValidateCalendarComponent accepted unsupported top-level component")
	}
	if _, err := StrongETag(nil); err == nil {
		t.Fatal("StrongETag accepted empty body")
	}
	if _, err := StrongETag([]byte(strings.Repeat("x", MaxCalendarObjectBytes+1))); err == nil {
		t.Fatal("StrongETag accepted oversized body")
	}
	if _, err := ValidateStrongETag(`"ABC"`); err == nil {
		t.Fatal("ValidateStrongETag accepted malformed etag")
	}
}
