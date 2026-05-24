package drive

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gogomail/gogomail/internal/storage"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestDriveRepositorySQLAvoidsWideCTEProjection(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"repository.go", "upload_session_repository.go"} {
		raw, err := os.ReadFile(filepath.Join(".", name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		text := string(raw)
		for _, forbidden := range []string{
			"SELECT * FROM updated",
			"SELECT * FROM inserted",
		} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("%s still contains wide CTE projection %q", name, forbidden)
			}
		}
	}
}

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

func TestValidateCreateFileFromObjectRequest(t *testing.T) {
	t.Parallel()

	req, normalizedName, err := ValidateCreateFileFromObjectRequest(CreateFileFromObjectRequest{
		NodeID:         " node-1 ",
		UserID:         " user-1 ",
		ParentID:       " parent-1 ",
		Name:           "  Report.PDF  ",
		StorageBackend: " s3 ",
		StoragePath:    "drive/users/user-1/staging/upload-1",
		MIMEType:       "",
		ChecksumSHA256: strings.Repeat("A", 64),
	})
	if err != nil {
		t.Fatalf("ValidateCreateFileFromObjectRequest returned error: %v", err)
	}
	if req.NodeID != "node-1" || req.UserID != "user-1" || req.ParentID != "parent-1" || req.Name != "Report.PDF" {
		t.Fatalf("request = %+v, want trimmed identity fields", req)
	}
	if req.StorageBackend != "s3" || req.StoragePath != "drive/users/user-1/staging/upload-1" {
		t.Fatalf("storage fields = %q/%q", req.StorageBackend, req.StoragePath)
	}
	if req.MIMEType != "application/octet-stream" {
		t.Fatalf("MIMEType = %q, want default application/octet-stream", req.MIMEType)
	}
	if req.ChecksumSHA256 != strings.Repeat("a", 64) {
		t.Fatalf("checksum = %q, want lowercased sha256", req.ChecksumSHA256)
	}
	if normalizedName != "report.pdf" {
		t.Fatalf("normalized name = %q, want report.pdf", normalizedName)
	}
}

func TestValidateCreateFileFromObjectRequestRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	tests := []CreateFileFromObjectRequest{
		{Name: "Report.pdf", StorageBackend: "s3", StoragePath: "drive/users/user-1/staging/upload-1"},
		{UserID: "user-1", Name: "Report.pdf", StorageBackend: "", StoragePath: "drive/users/user-1/staging/upload-1"},
		{UserID: "user-1", Name: "Report.pdf", StorageBackend: "s3\nbad", StoragePath: "drive/users/user-1/staging/upload-1"},
		{UserID: "user-1", Name: "Report.pdf", StorageBackend: "s3", StoragePath: "../bad"},
		{UserID: "user-1", Name: "Report.pdf", StorageBackend: "s3", StoragePath: "drive/users/user-2/staging/upload-1"},
		{UserID: "user-1", Name: "Report.pdf", StorageBackend: "s3", StoragePath: "drive/users/user-1/staging/upload-1", MIMEType: "text/plain\nbad"},
		{UserID: "user-1", Name: "Report.pdf", StorageBackend: "s3", StoragePath: "drive/users/user-1/staging/upload-1", ChecksumSHA256: "not-sha"},
		{NodeID: "node\n1", UserID: "user-1", Name: "Report.pdf", StorageBackend: "s3", StoragePath: "drive/users/user-1/staging/upload-1"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.UserID+"-"+tc.StorageBackend+"-"+tc.StoragePath, func(t *testing.T) {
			t.Parallel()

			if _, _, err := ValidateCreateFileFromObjectRequest(tc); err == nil {
				t.Fatalf("ValidateCreateFileFromObjectRequest(%+v) error = nil, want rejection", tc)
			}
		})
	}
}

func TestDriveCreateQueriesUseSargableParentFilters(t *testing.T) {
	t.Parallel()

	for name, query := range map[string]string{
		"folder": buildCreateFolderQuery("parent-1"),
		"file":   buildInsertDriveFileNodeQuery("parent-1"),
	} {
		name := name
		query := query
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			for _, want := range []string{
				"parent AS (",
				"WHERE n.id = $2::uuid",
				"AND n.node_type = 'folder'",
				"WHERE EXISTS (SELECT 1 FROM parent)",
			} {
				if !strings.Contains(query, want) {
					t.Fatalf("%s create query missing %q:\n%s", name, want, query)
				}
			}
			for _, forbidden := range []string{
				"NULLIF($2, '') IS NULL",
				"NULLIF($2, '')::uuid",
				"OR EXISTS (SELECT 1 FROM parent)",
			} {
				if strings.Contains(query, forbidden) {
					t.Fatalf("%s create query contains non-sargable parent guard %q:\n%s", name, forbidden, query)
				}
			}
		})
	}

	for name, query := range map[string]string{
		"folder": buildCreateFolderQuery(""),
		"file":   buildInsertDriveFileNodeQuery(""),
	} {
		name := name
		query := query
		t.Run(name+"_root", func(t *testing.T) {
			t.Parallel()

			if strings.Contains(query, "parent AS") || strings.Contains(query, "EXISTS (SELECT 1 FROM parent)") {
				t.Fatalf("%s root create query unexpectedly includes parent lookup:\n%s", name, query)
			}
			if !strings.Contains(query, "\n  NULLIF($2, '')::uuid,\n") {
				t.Fatalf("%s root create query missing typed NULL parent projection:\n%s", name, query)
			}
		})
	}
}

func TestFindActiveNodeBySiblingNameUsesSargableParentFilters(t *testing.T) {
	t.Parallel()

	rootQuery, rootArgs := buildFindActiveNodeBySiblingNameQuery("user-1", "", "report.pdf", "file")
	if len(rootArgs) != 3 {
		t.Fatalf("root args = %d, want 3", len(rootArgs))
	}
	if !strings.Contains(rootQuery, "AND parent_id IS NULL") {
		t.Fatalf("root sibling query missing parent_id IS NULL:\n%s", rootQuery)
	}
	if strings.Contains(rootQuery, "COALESCE(parent_id,") || strings.Contains(rootQuery, "NULLIF($2") || strings.Contains(rootQuery, "$4") {
		t.Fatalf("root sibling query contains non-sargable/unused parent expression:\n%s", rootQuery)
	}
	if !strings.Contains(rootQuery, "AND normalized_name = $2") || !strings.Contains(rootQuery, "AND node_type = $3") {
		t.Fatalf("root sibling query placeholders drifted:\n%s", rootQuery)
	}

	childQuery, childArgs := buildFindActiveNodeBySiblingNameQuery("user-1", "parent-1", "report.pdf", "file")
	if len(childArgs) != 4 {
		t.Fatalf("child args = %d, want 4", len(childArgs))
	}
	if !strings.Contains(childQuery, "AND parent_id = $2::uuid") {
		t.Fatalf("child sibling query missing direct parent predicate:\n%s", childQuery)
	}
	if strings.Contains(childQuery, "COALESCE(parent_id,") || strings.Contains(childQuery, "NULLIF($2") {
		t.Fatalf("child sibling query contains non-sargable parent expression:\n%s", childQuery)
	}
	if !strings.Contains(childQuery, "AND normalized_name = $3") || !strings.Contains(childQuery, "AND node_type = $4") {
		t.Fatalf("child sibling query placeholders drifted:\n%s", childQuery)
	}
}

func TestMapDriveFileCreateError(t *testing.T) {
	t.Parallel()

	err := mapDriveFileCreateError(&pgconn.PgError{
		Code:           "23505",
		ConstraintName: "idx_drive_nodes_user_active_sibling_name",
	})
	if !errors.Is(err, ErrDriveNodeAlreadyExists) {
		t.Fatalf("mapped error = %v, want ErrDriveNodeAlreadyExists", err)
	}
}

func TestValidateListNodesRequest(t *testing.T) {
	t.Parallel()

	req, err := ValidateListNodesRequest(ListNodesRequest{
		UserID:   " user-1 ",
		ParentID: " parent-1 ",
		Status:   " Trashed ",
		NodeType: " Folder ",
		Query:    " Report_% ",
		Sort:     " Updated ",
		Limit:    500,
	})
	if err != nil {
		t.Fatalf("ValidateListNodesRequest returned error: %v", err)
	}
	if req.UserID != "user-1" || req.ParentID != "parent-1" || req.Status != NodeStatusTrashed || req.NodeType != NodeTypeFolder || req.Query != "report_%" || req.Sort != NodeSortUpdated {
		t.Fatalf("request = %+v, want trimmed status-normalized request", req)
	}
	if got := escapeDriveNodeLikeQuery(req.Query); got != `report\_\%` {
		t.Fatalf("escaped query = %q", got)
	}
	if req.Limit != 200 {
		t.Fatalf("Limit = %d, want max cap 200", req.Limit)
	}

	defaulted, err := ValidateListNodesRequest(ListNodesRequest{UserID: "user-1"})
	if err != nil {
		t.Fatalf("ValidateListNodesRequest default returned error: %v", err)
	}
	if defaulted.Status != NodeStatusActive || defaulted.Sort != NodeSortName || defaulted.Limit != 50 {
		t.Fatalf("defaulted request = %+v, want active/50", defaulted)
	}

	allParents, err := ValidateListNodesRequest(ListNodesRequest{
		UserID:     " user-1 ",
		Status:     " Trashed ",
		NodeType:   " File ",
		Query:      " Report_% ",
		Sort:       " Updated ",
		AllParents: true,
		Limit:      500,
	})
	if err != nil {
		t.Fatalf("ValidateListNodesRequest all-parents returned error: %v", err)
	}
	if allParents.ParentID != "" || !allParents.AllParents || allParents.NodeType != NodeTypeFile || allParents.Query != "report_%" || allParents.Sort != NodeSortUpdated || allParents.Limit != 200 {
		t.Fatalf("all-parents request = %+v, want normalized whole-drive search", allParents)
	}
}

func TestListNodesQueryUsesSargableOptionalFilters(t *testing.T) {
	t.Parallel()

	req, err := ValidateListNodesRequest(ListNodesRequest{
		UserID:   " user-1 ",
		ParentID: " parent-1 ",
		Status:   " Trashed ",
		NodeType: " Folder ",
		Query:    " Report_% ",
		Sort:     " Updated ",
		Limit:    100,
	})
	if err != nil {
		t.Fatalf("ValidateListNodesRequest returned error: %v", err)
	}
	query, args := buildListNodesQuery(req)
	for _, want := range []string{
		"FROM drive_nodes",
		"WHERE user_id = $1::uuid",
		"AND status = $2",
		"AND normalized_name LIKE '%' || $3 || '%' ESCAPE",
		"AND node_type = $4",
		"AND parent_id = $5::uuid",
		"ORDER BY CASE WHEN node_type = 'folder' THEN 0 ELSE 1 END, updated_at DESC, normalized_name ASC, id ASC",
		"LIMIT $6",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("list nodes query missing %q:\n%s", want, query)
		}
	}
	for _, forbidden := range []string{
		"$5 = '' OR",
		"$7 = '' OR",
		"$6::boolean",
		"NULLIF($",
	} {
		if strings.Contains(query, forbidden) {
			t.Fatalf("list nodes query contains non-sargable optional filter %q:\n%s", forbidden, query)
		}
	}
	if len(args) != 6 {
		t.Fatalf("args len = %d, want 6", len(args))
	}
	if args[0] != "user-1" || args[1] != NodeStatusTrashed || args[2] != `report\_\%` || args[3] != NodeTypeFolder || args[4] != "parent-1" || args[5] != 100 {
		t.Fatalf("args = %#v", args)
	}

	rootReq, err := ValidateListNodesRequest(ListNodesRequest{UserID: "user-1"})
	if err != nil {
		t.Fatalf("ValidateListNodesRequest root returned error: %v", err)
	}
	query, args = buildListNodesQuery(rootReq)
	if !strings.Contains(query, "AND parent_id IS NULL") {
		t.Fatalf("root list query missing parent_id IS NULL:\n%s", query)
	}
	for _, unexpected := range []string{
		"normalized_name LIKE",
		"node_type = $",
		"AND parent_id = $",
		"NULLIF($",
	} {
		if strings.Contains(query, unexpected) {
			t.Fatalf("root list query unexpectedly includes %q:\n%s", unexpected, query)
		}
	}
	if len(args) != 3 {
		t.Fatalf("root args len = %d, want 3", len(args))
	}
	if args[0] != "user-1" || args[1] != NodeStatusActive || args[2] != 50 {
		t.Fatalf("root args = %#v", args)
	}

	allParentsReq, err := ValidateListNodesRequest(ListNodesRequest{
		UserID:     "user-1",
		AllParents: true,
		Limit:      25,
	})
	if err != nil {
		t.Fatalf("ValidateListNodesRequest all-parents returned error: %v", err)
	}
	query, args = buildListNodesQuery(allParentsReq)
	if strings.Contains(query, "AND parent_id") {
		t.Fatalf("all-parents query unexpectedly includes parent predicate:\n%s", query)
	}
	if len(args) != 3 {
		t.Fatalf("all-parents args len = %d, want 3", len(args))
	}
	if args[0] != "user-1" || args[1] != NodeStatusActive || args[2] != 25 {
		t.Fatalf("all-parents args = %#v", args)
	}
}

func TestValidateListNodesRequestRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	tests := []ListNodesRequest{
		{Status: NodeStatusActive},
		{UserID: "user\n1", Status: NodeStatusActive},
		{UserID: "user-1", ParentID: "parent\n1", Status: NodeStatusActive},
		{UserID: "user-1", ParentID: "parent-1", Status: NodeStatusActive, AllParents: true},
		{UserID: "user-1", Status: "archived"},
		{UserID: "user-1", Status: NodeStatusActive, NodeType: "shortcut"},
		{UserID: "user-1", Status: NodeStatusActive, Sort: "owner"},
		{UserID: "user-1", Status: NodeStatusActive, Query: strings.Repeat("q", MaxNodeNameBytes+1)},
		{UserID: "user-1", Status: NodeStatusActive, Query: "report\nbad"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.UserID+"-"+tc.Status, func(t *testing.T) {
			t.Parallel()

			if _, err := ValidateListNodesRequest(tc); err == nil {
				t.Fatalf("ValidateListNodesRequest(%+v) error = nil, want rejection", tc)
			}
		})
	}
}

func TestDriveNodeListOrderBy(t *testing.T) {
	t.Parallel()

	tests := map[string][]string{
		NodeSortName:    {"normalized_name ASC", "id ASC"},
		NodeSortUpdated: {"updated_at DESC", "normalized_name ASC"},
		NodeSortCreated: {"created_at DESC", "normalized_name ASC"},
		NodeSortSize:    {"size DESC", "normalized_name ASC"},
	}
	for sortMode, wants := range tests {
		sortMode := sortMode
		wants := wants
		t.Run(sortMode, func(t *testing.T) {
			t.Parallel()

			got := driveNodeListOrderBy(sortMode)
			if !strings.Contains(got, "CASE WHEN node_type = 'folder'") {
				t.Fatalf("order by = %q, want folder-first ordering", got)
			}
			for _, want := range wants {
				if !strings.Contains(got, want) {
					t.Fatalf("order by = %q, want %q", got, want)
				}
			}
		})
	}
}

func TestValidateGetUsageSummaryRequest(t *testing.T) {
	t.Parallel()

	req, err := ValidateGetUsageSummaryRequest(GetUsageSummaryRequest{UserID: " user-1 "})
	if err != nil {
		t.Fatalf("ValidateGetUsageSummaryRequest returned error: %v", err)
	}
	if req.UserID != "user-1" {
		t.Fatalf("request = %+v, want trimmed user id", req)
	}
	for _, userID := range []string{"", "user\n1"} {
		userID := userID
		t.Run(userID, func(t *testing.T) {
			t.Parallel()

			if _, err := ValidateGetUsageSummaryRequest(GetUsageSummaryRequest{UserID: userID}); err == nil {
				t.Fatalf("ValidateGetUsageSummaryRequest(%q) error = nil, want rejection", userID)
			}
		})
	}
}

func TestValidateGetNodeRequest(t *testing.T) {
	t.Parallel()

	req, err := ValidateGetNodeRequest(GetNodeRequest{UserID: " user-1 ", NodeID: " node-1 ", Status: " Trashed "})
	if err != nil {
		t.Fatalf("ValidateGetNodeRequest returned error: %v", err)
	}
	if req.UserID != "user-1" || req.NodeID != "node-1" || req.Status != NodeStatusTrashed {
		t.Fatalf("request = %+v, want trimmed status-normalized request", req)
	}
	defaulted, err := ValidateGetNodeRequest(GetNodeRequest{UserID: "user-1", NodeID: "node-1"})
	if err != nil {
		t.Fatalf("ValidateGetNodeRequest default returned error: %v", err)
	}
	if defaulted.Status != NodeStatusActive {
		t.Fatalf("defaulted request = %+v, want active status", defaulted)
	}
}

func TestValidateGetNodeRequestRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	tests := []GetNodeRequest{
		{NodeID: "node-1"},
		{UserID: "user-1"},
		{UserID: "user\n1", NodeID: "node-1"},
		{UserID: "user-1", NodeID: "node\n1"},
		{UserID: "user-1", NodeID: "node-1", Status: "archived"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.UserID+"-"+tc.NodeID+"-"+tc.Status, func(t *testing.T) {
			t.Parallel()

			if _, err := ValidateGetNodeRequest(tc); err == nil {
				t.Fatalf("ValidateGetNodeRequest(%+v) error = nil, want rejection", tc)
			}
		})
	}
}

func TestValidateTrashNodeRequest(t *testing.T) {
	t.Parallel()

	req, err := ValidateTrashNodeRequest(TrashNodeRequest{UserID: " user-1 ", NodeID: " node-1 "})
	if err != nil {
		t.Fatalf("ValidateTrashNodeRequest returned error: %v", err)
	}
	if req.UserID != "user-1" || req.NodeID != "node-1" {
		t.Fatalf("request = %+v, want trimmed IDs", req)
	}
}

func TestValidateTrashNodeRequestRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	tests := []TrashNodeRequest{
		{NodeID: "node-1"},
		{UserID: "user-1"},
		{UserID: "user\n1", NodeID: "node-1"},
		{UserID: "user-1", NodeID: "node\n1"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.UserID+"-"+tc.NodeID, func(t *testing.T) {
			t.Parallel()

			if _, err := ValidateTrashNodeRequest(tc); err == nil {
				t.Fatalf("ValidateTrashNodeRequest(%+v) error = nil, want rejection", tc)
			}
		})
	}
}

func TestValidateRestoreNodeRequest(t *testing.T) {
	t.Parallel()

	req, err := ValidateRestoreNodeRequest(RestoreNodeRequest{UserID: " user-1 ", NodeID: " node-1 "})
	if err != nil {
		t.Fatalf("ValidateRestoreNodeRequest returned error: %v", err)
	}
	if req.UserID != "user-1" || req.NodeID != "node-1" {
		t.Fatalf("request = %+v, want trimmed IDs", req)
	}
}

func TestValidateRestoreNodeRequestRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	tests := []RestoreNodeRequest{
		{NodeID: "node-1"},
		{UserID: "user-1"},
		{UserID: "user\n1", NodeID: "node-1"},
		{UserID: "user-1", NodeID: "node\n1"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.UserID+"-"+tc.NodeID, func(t *testing.T) {
			t.Parallel()

			if _, err := ValidateRestoreNodeRequest(tc); err == nil {
				t.Fatalf("ValidateRestoreNodeRequest(%+v) error = nil, want rejection", tc)
			}
		})
	}
}

func TestValidateRenameNodeRequest(t *testing.T) {
	t.Parallel()

	req, normalizedName, err := ValidateRenameNodeRequest(RenameNodeRequest{UserID: " user-1 ", NodeID: " node-1 ", Name: "  Report.PDF  "})
	if err != nil {
		t.Fatalf("ValidateRenameNodeRequest returned error: %v", err)
	}
	if req.UserID != "user-1" || req.NodeID != "node-1" || req.Name != "Report.PDF" {
		t.Fatalf("request = %+v, want trimmed fields", req)
	}
	if normalizedName != "report.pdf" {
		t.Fatalf("normalized name = %q, want report.pdf", normalizedName)
	}
}

func TestValidateRenameNodeRequestRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	tests := []RenameNodeRequest{
		{NodeID: "node-1", Name: "Report.pdf"},
		{UserID: "user-1", Name: "Report.pdf"},
		{UserID: "user\n1", NodeID: "node-1", Name: "Report.pdf"},
		{UserID: "user-1", NodeID: "node\n1", Name: "Report.pdf"},
		{UserID: "user-1", NodeID: "node-1", Name: ""},
		{UserID: "user-1", NodeID: "node-1", Name: "Reports/2026"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.UserID+"-"+tc.NodeID+"-"+tc.Name, func(t *testing.T) {
			t.Parallel()

			if _, _, err := ValidateRenameNodeRequest(tc); err == nil {
				t.Fatalf("ValidateRenameNodeRequest(%+v) error = nil, want rejection", tc)
			}
		})
	}
}

func TestValidateMoveNodeRequest(t *testing.T) {
	t.Parallel()

	req, err := ValidateMoveNodeRequest(MoveNodeRequest{UserID: " user-1 ", NodeID: " node-1 ", ParentID: " parent-1 "})
	if err != nil {
		t.Fatalf("ValidateMoveNodeRequest returned error: %v", err)
	}
	if req.UserID != "user-1" || req.NodeID != "node-1" || req.ParentID != "parent-1" {
		t.Fatalf("request = %+v, want trimmed fields", req)
	}
	rootReq, err := ValidateMoveNodeRequest(MoveNodeRequest{UserID: "user-1", NodeID: "node-1"})
	if err != nil {
		t.Fatalf("ValidateMoveNodeRequest root move returned error: %v", err)
	}
	if rootReq.ParentID != "" {
		t.Fatalf("root request = %+v, want empty parent", rootReq)
	}
}

func TestValidateMoveNodeRequestRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	tests := []MoveNodeRequest{
		{NodeID: "node-1", ParentID: "parent-1"},
		{UserID: "user-1", ParentID: "parent-1"},
		{UserID: "user\n1", NodeID: "node-1", ParentID: "parent-1"},
		{UserID: "user-1", NodeID: "node\n1", ParentID: "parent-1"},
		{UserID: "user-1", NodeID: "node-1", ParentID: "parent\n1"},
		{UserID: "user-1", NodeID: "same-1", ParentID: "same-1"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.UserID+"-"+tc.NodeID+"-"+tc.ParentID, func(t *testing.T) {
			t.Parallel()

			if _, err := ValidateMoveNodeRequest(tc); err == nil {
				t.Fatalf("ValidateMoveNodeRequest(%+v) error = nil, want rejection", tc)
			}
		})
	}
}

func TestValidateCopyNodeRequest(t *testing.T) {
	t.Parallel()

	req, normalizedName, err := ValidateCopyNodeRequest(CopyNodeRequest{UserID: " user-1 ", NodeID: " node-1 ", ParentID: " parent-1 ", Name: " Report Copy.pdf "})
	if err != nil {
		t.Fatalf("ValidateCopyNodeRequest returned error: %v", err)
	}
	if req.UserID != "user-1" || req.NodeID != "node-1" || req.ParentID != "parent-1" || req.Name != "Report Copy.pdf" {
		t.Fatalf("request = %+v, want trimmed fields", req)
	}
	if normalizedName != "report copy.pdf" {
		t.Fatalf("normalized name = %q, want report copy.pdf", normalizedName)
	}
}

func TestValidateCopyNodeRequestRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	tests := []CopyNodeRequest{
		{NodeID: "node-1", ParentID: "parent-1", Name: "Report.pdf"},
		{UserID: "user-1", ParentID: "parent-1", Name: "Report.pdf"},
		{UserID: "user\n1", NodeID: "node-1", ParentID: "parent-1", Name: "Report.pdf"},
		{UserID: "user-1", NodeID: "node\n1", ParentID: "parent-1", Name: "Report.pdf"},
		{UserID: "user-1", NodeID: "node-1", ParentID: "parent\n1", Name: "Report.pdf"},
		{UserID: "user-1", NodeID: "node-1", ParentID: "parent-1", Name: ""},
		{UserID: "user-1", NodeID: "node-1", ParentID: "parent-1", Name: "Reports/2026"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.UserID+"-"+tc.NodeID+"-"+tc.Name, func(t *testing.T) {
			t.Parallel()

			if _, _, err := ValidateCopyNodeRequest(tc); err == nil {
				t.Fatalf("ValidateCopyNodeRequest(%+v) error = nil, want rejection", tc)
			}
		})
	}
}

func TestValidatePermanentDeleteNodeRequest(t *testing.T) {
	t.Parallel()

	req, err := ValidatePermanentDeleteNodeRequest(PermanentDeleteNodeRequest{UserID: " user-1 ", NodeID: " node-1 "})
	if err != nil {
		t.Fatalf("ValidatePermanentDeleteNodeRequest returned error: %v", err)
	}
	if req.UserID != "user-1" || req.NodeID != "node-1" {
		t.Fatalf("request = %+v, want trimmed IDs", req)
	}
}

func TestValidatePermanentDeleteNodeRequestRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	tests := []PermanentDeleteNodeRequest{
		{NodeID: "node-1"},
		{UserID: "user-1"},
		{UserID: "user\n1", NodeID: "node-1"},
		{UserID: "user-1", NodeID: "node\n1"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.UserID+"-"+tc.NodeID, func(t *testing.T) {
			t.Parallel()

			if _, err := ValidatePermanentDeleteNodeRequest(tc); err == nil {
				t.Fatalf("ValidatePermanentDeleteNodeRequest(%+v) error = nil, want rejection", tc)
			}
		})
	}
}

func TestCreateFileFromObjectRequiresStore(t *testing.T) {
	t.Parallel()

	repo := NewRepository(nil)
	_, err := repo.CreateFileFromObject(context.Background(), nil, CreateFileFromObjectRequest{})
	if err == nil || !strings.Contains(err.Error(), "database handle is required") {
		t.Fatalf("CreateFileFromObject err = %v, want database handle rejection first", err)
	}
}

type fakeStore struct {
	info storage.ObjectInfo
	err  error
}

func (s fakeStore) Put(context.Context, string, io.Reader) error {
	return nil
}

func (s fakeStore) Get(context.Context, string) (io.ReadCloser, error) {
	return nil, nil
}

func (s fakeStore) GetRange(context.Context, string, storage.RangeRequest) (io.ReadCloser, error) {
	return nil, nil
}

func (s fakeStore) Stat(context.Context, string) (storage.ObjectInfo, error) {
	if s.err != nil {
		return storage.ObjectInfo{}, s.err
	}
	return s.info, nil
}

func (s fakeStore) Copy(context.Context, string, string) error {
	return nil
}

func (s fakeStore) Move(context.Context, string, string) error {
	return nil
}

func (s fakeStore) List(context.Context, storage.ListOptions) (storage.ObjectListPage, error) {
	return storage.ObjectListPage{}, nil
}

func (s fakeStore) Delete(context.Context, string) error {
	return nil
}
