package webdavgw

import (
	"testing"
	"time"
)

func TestPropfindResponse(t *testing.T) {
	resources := []Resource{
		{
			Href:     "/dav/files/",
			Name:     "files",
			IsCollection: true,
			Modified: time.Now(),
		},
		{
			Href:     "/dav/files/doc.txt",
			Name:     "doc.txt",
			Size:     1024,
			IsCollection: false,
			Modified: time.Now(),
		},
	}

	xml, err := MarshalPropfindResponse(resources)
	if err != nil {
		t.Fatalf("MarshalPropfindResponse error: %v", err)
	}
	if len(xml) == 0 {
		t.Fatal("MarshalPropfindResponse returned empty xml")
	}
	if !contains(string(xml), "multistatus") {
		t.Fatal("xml missing multistatus element")
	}
	if !contains(string(xml), "doc.txt") {
		t.Fatal("xml missing doc.txt resource")
	}
}

func TestParsePropfindRequest(t *testing.T) {
	xml := `<?xml version="1.0"?>
<propfind xmlns="DAV:">
  <prop>
    <displayname/>
    <getcontentlength/>
  </prop>
</propfind>`

	props, err := ParsePropfindRequest([]byte(xml))
	if err != nil {
		t.Fatalf("ParsePropfindRequest error: %v", err)
	}
	if len(props) != 2 {
		t.Fatalf("props len = %d, want 2", len(props))
	}
	if props[0] != "displayname" {
		t.Fatalf("props[0] = %s, want displayname", props[0])
	}
}

func TestNormalizeHref(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/dav/files", "/dav/files"},
		{"/dav/files/", "/dav/files/"},
		{"dav/files", "/dav/files"},
	}

	for _, tt := range tests {
		got := NormalizeHref(tt.input)
		if got != tt.expected {
			t.Errorf("NormalizeHref(%q) = %s, want %s", tt.input, got, tt.expected)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
