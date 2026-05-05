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
