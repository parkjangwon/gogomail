package scim

import (
	"encoding/json"
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

// TestBooleanParsing verifies ParseBool accepts all required forms.
func TestBooleanParsing(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    bool
		wantErr bool
	}{
		// JSON booleans
		{"json true", "true", true, false},
		{"json false", "false", false, false},
		// String variants (case-insensitive)
		{`string "true"`, `"true"`, true, false},
		{`string "True"`, `"True"`, true, false},
		{`string "TRUE"`, `"TRUE"`, true, false},
		{`string "false"`, `"false"`, false, false},
		{`string "False"`, `"False"`, false, false},
		{`string "FALSE"`, `"FALSE"`, false, false},
		// null → false
		{"null", "null", false, false},
		// invalid
		{`string "yes"`, `"yes"`, false, true},
		{"number 1", "1", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseBool(json.RawMessage(tt.raw))
			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseBool(%s) expected error, got nil", tt.raw)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseBool(%s) unexpected error: %v", tt.raw, err)
			}
			if got != tt.want {
				t.Errorf("ParseBool(%s) = %v, want %v", tt.raw, got, tt.want)
			}
		})
	}
}

// TestAttributeMatches verifies case-insensitive attribute name comparison.
func TestAttributeMatches(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"email", "email", true},
		{"Email", "email", true},
		{"EMAIL", "email", true},
		{"emails.value", "Emails.Value", true},
		{"userName", "USERNAME", true},
		{"userName", "displayName", false},
	}

	for _, tt := range tests {
		got := AttributeMatches(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("AttributeMatches(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

// TestValidateUserResource verifies required-field and email-format validation.
func TestValidateUserResource(t *testing.T) {
	t.Run("valid user", func(t *testing.T) {
		u := &UserResource{UserName: "bjensen", Emails: []Email{{Value: "bjensen@example.com"}}}
		if err := ValidateUserResource(u); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("null/empty userName rejected", func(t *testing.T) {
		u := &UserResource{UserName: ""}
		if err := ValidateUserResource(u); err == nil {
			t.Error("expected error for empty userName, got nil")
		}
	})

	t.Run("whitespace-only userName rejected", func(t *testing.T) {
		u := &UserResource{UserName: "   "}
		if err := ValidateUserResource(u); err == nil {
			t.Error("expected error for whitespace userName, got nil")
		}
	})

	t.Run("invalid email rejected", func(t *testing.T) {
		u := &UserResource{UserName: "bjensen", Emails: []Email{{Value: "not-an-email"}}}
		if err := ValidateUserResource(u); err == nil {
			t.Error("expected error for invalid email, got nil")
		}
	})

	t.Run("valid email passes", func(t *testing.T) {
		u := &UserResource{UserName: "bjensen", Emails: []Email{{Value: "user@domain.org"}}}
		if err := ValidateUserResource(u); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("no emails passes", func(t *testing.T) {
		u := &UserResource{UserName: "bjensen"}
		if err := ValidateUserResource(u); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}
