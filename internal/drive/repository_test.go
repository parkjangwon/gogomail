package drive

import (
	"strings"
	"testing"
)

func TestValidateCreateFolderRequest(t *testing.T) {
	t.Parallel()

	req, normalizedName, err := ValidateCreateFolderRequest(CreateFolderRequest{
		UserID:   " user-1 ",
		ParentID: " parent-1 ",
		Name:     "  Reports  ",
	})
	if err != nil {
		t.Fatalf("ValidateCreateFolderRequest returned error: %v", err)
	}
	if req.UserID != "user-1" || req.ParentID != "parent-1" || req.Name != "Reports" {
		t.Fatalf("request = %+v, want trimmed fields", req)
	}
	if normalizedName != "reports" {
		t.Fatalf("normalized name = %q, want reports", normalizedName)
	}
}

func TestValidateCreateFolderRequestRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	tests := []CreateFolderRequest{
		{Name: "Reports"},
		{UserID: "user-1", ParentID: "parent\n1", Name: "Reports"},
		{UserID: strings.Repeat("u", 129), Name: "Reports"},
		{UserID: "user-1", Name: ""},
		{UserID: "user-1", Name: "Reports/2026"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.UserID+"-"+tc.Name, func(t *testing.T) {
			t.Parallel()

			if _, _, err := ValidateCreateFolderRequest(tc); err == nil {
				t.Fatalf("ValidateCreateFolderRequest(%+v) error = nil, want rejection", tc)
			}
		})
	}
}
