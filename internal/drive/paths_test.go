package drive

import "testing"

func TestBuildStagedObjectPath(t *testing.T) {
	t.Parallel()

	path, err := BuildStagedObjectPath(" user-1 ", " upload-1 ")
	if err != nil {
		t.Fatalf("BuildStagedObjectPath returned error: %v", err)
	}
	if path != "drive/users/user-1/staging/upload-1" {
		t.Fatalf("path = %q, want stable staged path", path)
	}
}

func TestBuildScopedDriveObjectPaths(t *testing.T) {
	t.Parallel()

	scope := ObjectPathScope{CompanyID: " company-1 ", DomainID: " domain-1 ", UserID: " user-1 "}
	staged, err := BuildScopedStagedObjectPath(scope, " upload-1 ")
	if err != nil {
		t.Fatalf("BuildScopedStagedObjectPath returned error: %v", err)
	}
	if staged != "drive/company-1/domain-1/users/user-1/staging/upload-1" {
		t.Fatalf("staged path = %q", staged)
	}
	node, err := BuildScopedNodeObjectPath(scope, " node-1 ")
	if err != nil {
		t.Fatalf("BuildScopedNodeObjectPath returned error: %v", err)
	}
	if node != "drive/company-1/domain-1/users/user-1/objects/node-1" {
		t.Fatalf("node path = %q", node)
	}
	body, err := BuildScopedUploadSessionBodyPath(scope, " session-1 ", " body-1 ")
	if err != nil {
		t.Fatalf("BuildScopedUploadSessionBodyPath returned error: %v", err)
	}
	if body != "drive/company-1/domain-1/users/user-1/upload-sessions/session-1/bodies/body-1" {
		t.Fatalf("body path = %q", body)
	}
}

func TestBuildNodeObjectPath(t *testing.T) {
	t.Parallel()

	path, err := BuildNodeObjectPath(" user-1 ", " node-1 ")
	if err != nil {
		t.Fatalf("BuildNodeObjectPath returned error: %v", err)
	}
	if path != "drive/users/user-1/objects/node-1" {
		t.Fatalf("path = %q, want stable committed path", path)
	}
}

func TestBuildUploadSessionBodyPath(t *testing.T) {
	t.Parallel()

	path, err := BuildUploadSessionBodyPath(" user-1 ", " session-1 ", " body-1 ")
	if err != nil {
		t.Fatalf("BuildUploadSessionBodyPath returned error: %v", err)
	}
	if path != "drive/users/user-1/upload-sessions/session-1/bodies/body-1" {
		t.Fatalf("path = %q", path)
	}
}

func TestValidateUserObjectPathRequiresUserPrefix(t *testing.T) {
	t.Parallel()

	path, err := validateUserObjectPath(" user-1 ", "drive/users/user-1/staging/upload-1")
	if err != nil {
		t.Fatalf("validateUserObjectPath returned error: %v", err)
	}
	if path != "drive/users/user-1/staging/upload-1" {
		t.Fatalf("path = %q, want canonical user object path", path)
	}
	if _, err := validateUserObjectPath("user-1", "drive/users/user-2/staging/upload-1"); err == nil {
		t.Fatal("validateUserObjectPath accepted another user's object path")
	}
	if path, err := validateUserObjectPath("user-1", "drive/company-1/domain-1/users/user-1/staging/upload-1"); err != nil || path != "drive/company-1/domain-1/users/user-1/staging/upload-1" {
		t.Fatalf("validateUserObjectPath scoped path = %q, %v", path, err)
	}
	if _, err := validateUserObjectPath("user-1", "drive/company-1/domain-1/users/user-2/staging/upload-1"); err == nil {
		t.Fatal("validateUserObjectPath accepted another user's scoped object path")
	}
	if _, err := validateUserObjectPath("user-1", "../bad"); err == nil {
		t.Fatal("validateUserObjectPath accepted unsafe object path")
	}
}

func TestUserObjectPrefix(t *testing.T) {
	t.Parallel()

	prefix, err := UserObjectPrefix(" user-1 ")
	if err != nil {
		t.Fatalf("UserObjectPrefix returned error: %v", err)
	}
	if prefix != "drive/users/user-1/" {
		t.Fatalf("prefix = %q, want user drive prefix", prefix)
	}
}

func TestDriveObjectPathsRejectUnsafeInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		fn   func() (string, error)
	}{
		{name: "staged missing user", fn: func() (string, error) { return BuildStagedObjectPath("", "upload-1") }},
		{name: "staged unsafe upload", fn: func() (string, error) { return BuildStagedObjectPath("user-1", "../bad") }},
		{name: "staged slash user", fn: func() (string, error) { return BuildStagedObjectPath("tenant/user-1", "upload-1") }},
		{name: "node unsafe user", fn: func() (string, error) { return BuildNodeObjectPath("user\n1", "node-1") }},
		{name: "node missing id", fn: func() (string, error) { return BuildNodeObjectPath("user-1", "") }},
		{name: "prefix unsafe user", fn: func() (string, error) { return UserObjectPrefix("user\n1") }},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got, err := tc.fn(); err == nil {
				t.Fatalf("%s returned %q without error", tc.name, got)
			}
		})
	}
}
