package drive

import (
	"strings"
	"testing"
)

func TestNormalizeNodeName(t *testing.T) {
	t.Parallel()

	got, err := NormalizeNodeName("  Q2 Plan.PDF  ")
	if err != nil {
		t.Fatalf("NormalizeNodeName returned error: %v", err)
	}
	if got != "q2 plan.pdf" {
		t.Fatalf("normalized name = %q, want lowercase trimmed name", got)
	}
}

func TestValidateNodeNameRejectsUnsafeNames(t *testing.T) {
	t.Parallel()

	for _, name := range []string{
		"",
		"   ",
		".",
		"..",
		"folder/report.pdf",
		`folder\report.pdf`,
		"report\n.pdf",
		"report\r.pdf",
		"report\x00.pdf",
		strings.Repeat("a", MaxNodeNameBytes+1),
	} {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if _, err := ValidateNodeName(name); err == nil {
				t.Fatalf("ValidateNodeName(%q) error = nil, want rejection", name)
			}
		})
	}
}

func TestValidateNodeTypeAndStatus(t *testing.T) {
	t.Parallel()

	if got, err := ValidateNodeType(" FILE "); err != nil || got != NodeTypeFile {
		t.Fatalf("ValidateNodeType = %q, %v", got, err)
	}
	if got, err := ValidateNodeStatus(" Trashed "); err != nil || got != NodeStatusTrashed {
		t.Fatalf("ValidateNodeStatus = %q, %v", got, err)
	}
	if _, err := ValidateNodeType("shortcut"); err == nil {
		t.Fatal("ValidateNodeType accepted unsupported type")
	}
	if _, err := ValidateNodeStatus("archived"); err == nil {
		t.Fatal("ValidateNodeStatus accepted unsupported status")
	}
}

func TestValidateNodeSort(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "default", want: NodeSortName},
		{name: "name", in: " NAME ", want: NodeSortName},
		{name: "updated", in: "Updated", want: NodeSortUpdated},
		{name: "created", in: "created", want: NodeSortCreated},
		{name: "size", in: "size", want: NodeSortSize},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ValidateNodeSort(tt.in)
			if err != nil || got != tt.want {
				t.Fatalf("ValidateNodeSort(%q) = %q, %v; want %q, nil", tt.in, got, err, tt.want)
			}
		})
	}
	if _, err := ValidateNodeSort("owner"); err == nil {
		t.Fatal("ValidateNodeSort accepted unsupported sort")
	}
}

func TestNewNodeIDReturnsUUIDv4(t *testing.T) {
	t.Parallel()

	id, err := NewNodeID()
	if err != nil {
		t.Fatalf("NewNodeID returned error: %v", err)
	}
	if _, err := validateDriveID("node_id", id, true); err != nil {
		t.Fatalf("NewNodeID = %q is not a valid drive ID: %v", id, err)
	}
	if len(id) != 36 || id[14] != '4' {
		t.Fatalf("NewNodeID = %q, want UUIDv4 format", id)
	}
}
