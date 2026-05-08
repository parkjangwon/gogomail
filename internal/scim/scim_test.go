package scim

import (
	"testing"
)

func TestParseFilter(t *testing.T) {
	tests := []struct {
		input    string
		wantAttr string
		wantOp   string
		wantVal  string
		wantErr  bool
	}{
		{`userName eq "bjensen"`, "userName", "eq", "bjensen", false},
		{`emails.value eq "bjensen@example.com"`, "emails.value", "eq", "bjensen@example.com", false},
		{`displayName sw "Bj"`, "displayName", "sw", "Bj", false},
		{`active eq true`, "active", "eq", "true", false},
		{"", "", "", "", true},
	}

	for _, tt := range tests {
		f, err := ParseFilter(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("ParseFilter(%q) should error", tt.input)
			}
			continue
		}
		if err != nil {
			t.Fatalf("ParseFilter(%q) error: %v", tt.input, err)
		}
		if f.Attribute != tt.wantAttr {
			t.Errorf("ParseFilter(%q) attr = %s, want %s", tt.input, f.Attribute, tt.wantAttr)
		}
		if f.Operator != tt.wantOp {
			t.Errorf("ParseFilter(%q) op = %s, want %s", tt.input, f.Operator, tt.wantOp)
		}
		if f.Value != tt.wantVal {
			t.Errorf("ParseFilter(%q) val = %s, want %s", tt.input, f.Value, tt.wantVal)
		}
	}
}

func TestSCIMUserResource(t *testing.T) {
	u := UserResource{
		ID:         "user-1",
		UserName:   "bjensen",
		ExternalID: "bjensen@example.com",
		Name: Name{
			GivenName:  "Barbara",
			FamilyName: "Jensen",
		},
		Emails: []Email{
			{Value: "bjensen@example.com", Primary: true},
		},
		Active: true,
	}

	if u.UserName != "bjensen" {
		t.Fatalf("UserName = %s, want bjensen", u.UserName)
	}
	if len(u.Emails) != 1 {
		t.Fatalf("Emails len = %d, want 1", len(u.Emails))
	}
	if !u.Emails[0].Primary {
		t.Fatal("Primary email should be true")
	}
}

func TestSCIMListResponse(t *testing.T) {
	resp := ListResponse{
		TotalResults: 2,
		Resources: []UserResource{
			{ID: "user-1", UserName: "bjensen"},
			{ID: "user-2", UserName: "jsmith"},
		},
	}

	if resp.TotalResults != 2 {
		t.Fatalf("TotalResults = %d, want 2", resp.TotalResults)
	}
	if len(resp.Resources) != 2 {
		t.Fatalf("Resources len = %d, want 2", len(resp.Resources))
	}
}

func TestNormalizeSCIMAttribute(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"userName", "username"},
		{"emails.value", "emails.value"},
		{"DisplayName", "displayname"},
	}

	for _, tt := range tests {
		got := normalizeAttribute(tt.input)
		if got != tt.expected {
			t.Errorf("normalizeAttribute(%q) = %s, want %s", tt.input, got, tt.expected)
		}
	}
}
