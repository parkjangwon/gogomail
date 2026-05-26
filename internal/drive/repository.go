package drive

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/storage"
	"github.com/jackc/pgx/v5/pgconn"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ObjectPathScopeForUser(ctx context.Context, userID string) (ObjectPathScope, error) {
	if r == nil || r.db == nil {
		return ObjectPathScope{}, fmt.Errorf("database handle is required")
	}
	userID = strings.TrimSpace(userID)
	if _, err := validateDriveObjectPathID("user_id", userID); err != nil {
		return ObjectPathScope{}, err
	}
	const query = `
SELECT d.company_id::text, d.id::text, u.id::text
FROM users u
JOIN domains d ON d.id = u.domain_id
WHERE u.id = $1::uuid
  AND u.status = 'active'
  AND d.status = 'active'`
	var scope ObjectPathScope
	if err := r.db.QueryRowContext(ctx, query, userID).Scan(&scope.CompanyID, &scope.DomainID, &scope.UserID); err != nil {
		if err == sql.ErrNoRows {
			return ObjectPathScope{}, fmt.Errorf("active user not found")
		}
		return ObjectPathScope{}, fmt.Errorf("lookup drive object path scope: %w", err)
	}
	return validateObjectPathScope(scope)
}

type Node struct {
	ID             string    `json:"id"`
	CompanyID      string    `json:"company_id,omitempty"`
	DomainID       string    `json:"domain_id,omitempty"`
	UserID         string    `json:"user_id"`
	ParentID       string    `json:"parent_id,omitempty"`
	Type           string    `json:"node_type"`
	Name           string    `json:"name"`
	NormalizedName string    `json:"normalized_name"`
	MIMEType       string    `json:"mime_type,omitempty"`
	Size           int64     `json:"size"`
	StorageBackend string    `json:"storage_backend,omitempty"`
	StoragePath    string    `json:"storage_path,omitempty"`
	ChecksumSHA256 string    `json:"checksum_sha256,omitempty"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type UsageSummary struct {
	UserID                  string `json:"user_id"`
	QuotaUsed               int64  `json:"quota_used"`
	QuotaLimit              int64  `json:"quota_limit"`
	TotalNodes              int64  `json:"total_nodes"`
	ActiveNodes             int64  `json:"active_nodes"`
	TrashedNodes            int64  `json:"trashed_nodes"`
	DeletedNodes            int64  `json:"deleted_nodes"`
	FolderCount             int64  `json:"folder_count"`
	FileCount               int64  `json:"file_count"`
	ActiveBytes             int64  `json:"active_bytes"`
	TrashedBytes            int64  `json:"trashed_bytes"`
	DeletedBytes            int64  `json:"deleted_bytes"`
	PendingUploadSessions   int64  `json:"pending_upload_sessions"`
	UploadingUploadSessions int64  `json:"uploading_upload_sessions"`
	FailedUploadSessions    int64  `json:"failed_upload_sessions"`
	PendingUploadBytes      int64  `json:"pending_upload_bytes"`
}

type CreateFolderRequest struct {
	UserID   string
	ParentID string
	Name     string
}

type CreateFileRequest struct {
	UserID   string
	ParentID string
	Name     string
	Body     io.Reader
	Size     int64
	MIMEType string
}

type CreateFileFromObjectRequest struct {
	NodeID         string
	UserID         string
	ParentID       string
	Name           string
	StorageBackend string
	StoragePath    string
	MIMEType       string
	ChecksumSHA256 string
}

type ListNodesRequest struct {
	UserID     string
	ParentID   string
	Status     string
	NodeType   string
	Query      string
	Sort       string
	AllParents bool
	Limit      int
}

type GetUsageSummaryRequest struct {
	UserID string
}

type GetNodeRequest struct {
	UserID string
	NodeID string
	Status string
}

type OpenFileRequest struct {
	UserID string
	NodeID string
}

type OpenFileRangeRequest struct {
	UserID string
	NodeID string
	Offset int64
	Length int64
}

type FileDownload struct {
	Node      Node
	ShareLink ShareLink
	Body      io.ReadCloser
}

type FileMetadata struct {
	Node      Node
	ShareLink ShareLink
	Object    storage.ObjectInfo
}

type TrashNodeRequest struct {
	UserID string
	NodeID string
}

type RestoreNodeRequest struct {
	UserID string
	NodeID string
}

type RenameNodeRequest struct {
	UserID string
	NodeID string
	Name   string
}

type MoveNodeRequest struct {
	UserID   string
	NodeID   string
	ParentID string
}

type CopyNodeRequest struct {
	UserID   string
	NodeID   string
	ParentID string
	Name     string
}

type PermanentDeleteNodeRequest struct {
	UserID string
	NodeID string
}

type DeletedObject struct {
	StorageBackend string `json:"storage_backend"`
	StoragePath    string `json:"storage_path"`
}

type PermanentDeleteResult struct {
	Root          Node            `json:"root"`
	DeletedNodes  int64           `json:"deleted_nodes"`
	ReleasedBytes int64           `json:"released_bytes"`
	Objects       []DeletedObject `json:"objects,omitempty"`
}

func ValidateCreateFolderRequest(req CreateFolderRequest) (CreateFolderRequest, string, error) {
	userID, err := validateDriveID("user_id", req.UserID, true)
	if err != nil {
		return CreateFolderRequest{}, "", err
	}
	parentID, err := validateDriveID("parent_id", req.ParentID, false)
	if err != nil {
		return CreateFolderRequest{}, "", err
	}
	name, err := ValidateNodeName(req.Name)
	if err != nil {
		return CreateFolderRequest{}, "", err
	}
	normalizedName, err := NormalizeNodeName(name)
	if err != nil {
		return CreateFolderRequest{}, "", err
	}
	return CreateFolderRequest{
		UserID:   userID,
		ParentID: parentID,
		Name:     name,
	}, normalizedName, nil
}

func ValidateCreateFileFromObjectRequest(req CreateFileFromObjectRequest) (CreateFileFromObjectRequest, string, error) {
	nodeID, err := validateDriveID("node_id", req.NodeID, false)
	if err != nil {
		return CreateFileFromObjectRequest{}, "", err
	}
	userID, err := validateDriveID("user_id", req.UserID, true)
	if err != nil {
		return CreateFileFromObjectRequest{}, "", err
	}
	parentID, err := validateDriveID("parent_id", req.ParentID, false)
	if err != nil {
		return CreateFileFromObjectRequest{}, "", err
	}
	name, err := ValidateNodeName(req.Name)
	if err != nil {
		return CreateFileFromObjectRequest{}, "", err
	}
	normalizedName, err := NormalizeNodeName(name)
	if err != nil {
		return CreateFileFromObjectRequest{}, "", err
	}
	storageBackend, err := validateStorageBackend(req.StorageBackend)
	if err != nil {
		return CreateFileFromObjectRequest{}, "", err
	}
	storagePath, err := validateUserObjectPath(userID, req.StoragePath)
	if err != nil {
		return CreateFileFromObjectRequest{}, "", fmt.Errorf("unsafe storage path %q: %w", req.StoragePath, err)
	}
	mimeType, err := validateDriveMIMEType(req.MIMEType)
	if err != nil {
		return CreateFileFromObjectRequest{}, "", err
	}
	checksum, err := validateDriveChecksum(req.ChecksumSHA256)
	if err != nil {
		return CreateFileFromObjectRequest{}, "", err
	}
	return CreateFileFromObjectRequest{
		NodeID:         nodeID,
		UserID:         userID,
		ParentID:       parentID,
		Name:           name,
		StorageBackend: storageBackend,
		StoragePath:    storagePath,
		MIMEType:       mimeType,
		ChecksumSHA256: checksum,
	}, normalizedName, nil
}

func ValidateListNodesRequest(req ListNodesRequest) (ListNodesRequest, error) {
	userID, err := validateDriveID("user_id", req.UserID, true)
	if err != nil {
		return ListNodesRequest{}, err
	}
	parentID, err := validateDriveID("parent_id", req.ParentID, false)
	if err != nil {
		return ListNodesRequest{}, err
	}
	status := strings.TrimSpace(req.Status)
	if status == "" {
		status = NodeStatusActive
	}
	status, err = ValidateNodeStatus(status)
	if err != nil {
		return ListNodesRequest{}, err
	}
	nodeType := strings.TrimSpace(req.NodeType)
	if nodeType != "" {
		nodeType, err = ValidateNodeType(nodeType)
		if err != nil {
			return ListNodesRequest{}, err
		}
	}
	query, err := validateDriveNodeSearchQuery(req.Query)
	if err != nil {
		return ListNodesRequest{}, err
	}
	sortMode, err := ValidateNodeSort(req.Sort)
	if err != nil {
		return ListNodesRequest{}, err
	}
	if req.AllParents && parentID != "" {
		return ListNodesRequest{}, fmt.Errorf("parent_id cannot be combined with all_parents")
	}
	limit := normalizeDriveListLimit(req.Limit)
	return ListNodesRequest{
		UserID:     userID,
		ParentID:   parentID,
		Status:     status,
		NodeType:   nodeType,
		Query:      query,
		Sort:       sortMode,
		AllParents: req.AllParents,
		Limit:      limit,
	}, nil
}

func ValidateGetUsageSummaryRequest(req GetUsageSummaryRequest) (GetUsageSummaryRequest, error) {
	userID, err := validateDriveID("user_id", req.UserID, true)
	if err != nil {
		return GetUsageSummaryRequest{}, err
	}
	return GetUsageSummaryRequest{UserID: userID}, nil
}

func ValidateGetNodeRequest(req GetNodeRequest) (GetNodeRequest, error) {
	userID, err := validateDriveID("user_id", req.UserID, true)
	if err != nil {
		return GetNodeRequest{}, err
	}
	nodeID, err := validateDriveID("node_id", req.NodeID, true)
	if err != nil {
		return GetNodeRequest{}, err
	}
	status := strings.TrimSpace(req.Status)
	if status == "" {
		status = NodeStatusActive
	}
	status, err = ValidateNodeStatus(status)
	if err != nil {
		return GetNodeRequest{}, err
	}
	return GetNodeRequest{UserID: userID, NodeID: nodeID, Status: status}, nil
}

func ValidateTrashNodeRequest(req TrashNodeRequest) (TrashNodeRequest, error) {
	userID, err := validateDriveID("user_id", req.UserID, true)
	if err != nil {
		return TrashNodeRequest{}, err
	}
	nodeID, err := validateDriveID("node_id", req.NodeID, true)
	if err != nil {
		return TrashNodeRequest{}, err
	}
	return TrashNodeRequest{UserID: userID, NodeID: nodeID}, nil
}

func ValidateRestoreNodeRequest(req RestoreNodeRequest) (RestoreNodeRequest, error) {
	userID, err := validateDriveID("user_id", req.UserID, true)
	if err != nil {
		return RestoreNodeRequest{}, err
	}
	nodeID, err := validateDriveID("node_id", req.NodeID, true)
	if err != nil {
		return RestoreNodeRequest{}, err
	}
	return RestoreNodeRequest{UserID: userID, NodeID: nodeID}, nil
}

func ValidateRenameNodeRequest(req RenameNodeRequest) (RenameNodeRequest, string, error) {
	userID, err := validateDriveID("user_id", req.UserID, true)
	if err != nil {
		return RenameNodeRequest{}, "", err
	}
	nodeID, err := validateDriveID("node_id", req.NodeID, true)
	if err != nil {
		return RenameNodeRequest{}, "", err
	}
	name, err := ValidateNodeName(req.Name)
	if err != nil {
		return RenameNodeRequest{}, "", err
	}
	normalizedName, err := NormalizeNodeName(name)
	if err != nil {
		return RenameNodeRequest{}, "", err
	}
	return RenameNodeRequest{UserID: userID, NodeID: nodeID, Name: name}, normalizedName, nil
}

func ValidateMoveNodeRequest(req MoveNodeRequest) (MoveNodeRequest, error) {
	userID, err := validateDriveID("user_id", req.UserID, true)
	if err != nil {
		return MoveNodeRequest{}, err
	}
	nodeID, err := validateDriveID("node_id", req.NodeID, true)
	if err != nil {
		return MoveNodeRequest{}, err
	}
	parentID, err := validateDriveID("parent_id", req.ParentID, false)
	if err != nil {
		return MoveNodeRequest{}, err
	}
	if parentID == nodeID {
		return MoveNodeRequest{}, fmt.Errorf("parent_id must not equal node_id")
	}
	return MoveNodeRequest{UserID: userID, NodeID: nodeID, ParentID: parentID}, nil
}

func ValidateCopyNodeRequest(req CopyNodeRequest) (CopyNodeRequest, string, error) {
	userID, err := validateDriveID("user_id", req.UserID, true)
	if err != nil {
		return CopyNodeRequest{}, "", err
	}
	nodeID, err := validateDriveID("node_id", req.NodeID, true)
	if err != nil {
		return CopyNodeRequest{}, "", err
	}
	parentID, err := validateDriveID("parent_id", req.ParentID, false)
	if err != nil {
		return CopyNodeRequest{}, "", err
	}
	name, err := ValidateNodeName(req.Name)
	if err != nil {
		return CopyNodeRequest{}, "", err
	}
	normalizedName, err := NormalizeNodeName(name)
	if err != nil {
		return CopyNodeRequest{}, "", err
	}
	return CopyNodeRequest{UserID: userID, NodeID: nodeID, ParentID: parentID, Name: name}, normalizedName, nil
}

func ValidatePermanentDeleteNodeRequest(req PermanentDeleteNodeRequest) (PermanentDeleteNodeRequest, error) {
	userID, err := validateDriveID("user_id", req.UserID, true)
	if err != nil {
		return PermanentDeleteNodeRequest{}, err
	}
	nodeID, err := validateDriveID("node_id", req.NodeID, true)
	if err != nil {
		return PermanentDeleteNodeRequest{}, err
	}
	return PermanentDeleteNodeRequest{UserID: userID, NodeID: nodeID}, nil
}

func (r *Repository) CreateFolder(ctx context.Context, req CreateFolderRequest) (Node, error) {
	if r == nil || r.db == nil {
		return Node{}, fmt.Errorf("database handle is required")
	}
	req, normalizedName, err := ValidateCreateFolderRequest(req)
	if err != nil {
		return Node{}, err
	}

	query := buildCreateFolderQuery(req.ParentID)

	var node Node
	err = r.db.QueryRowContext(ctx, query, req.UserID, req.ParentID, req.Name, normalizedName).Scan(
		&node.ID,
		&node.CompanyID,
		&node.DomainID,
		&node.UserID,
		&node.ParentID,
		&node.Type,
		&node.Name,
		&node.NormalizedName,
		&node.MIMEType,
		&node.Size,
		&node.StorageBackend,
		&node.StoragePath,
		&node.ChecksumSHA256,
		&node.Status,
		&node.CreatedAt,
		&node.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return Node{}, fmt.Errorf("active user or parent folder not found")
		}
		if isDriveNodeSiblingNameConflict(err) {
			existing, lookupErr := r.findActiveNodeBySiblingName(ctx, req.UserID, req.ParentID, normalizedName, NodeTypeFolder)
			if lookupErr == nil {
				return existing, nil
			}
			return Node{}, lookupErr
		}
		return Node{}, fmt.Errorf("create drive folder: %w", err)
	}
	return node, nil
}

func buildCreateFolderQuery(parentID string) string {
	parentCTE, parentIDExpr, parentWhere := driveParentFolderInsertFragments(parentID)
	return fmt.Sprintf(`
WITH owner AS (
  SELECT u.id AS user_id, d.id AS domain_id, d.company_id
  FROM users u
  JOIN domains d ON d.id = u.domain_id
  WHERE u.id = $1::uuid
    AND u.status = 'active'
    AND d.status = 'active'
)
%s
INSERT INTO drive_nodes (
  id,
  company_id,
  domain_id,
  user_id,
  parent_id,
  node_type,
  name,
  normalized_name,
  status
)
SELECT
  gen_random_uuid(),
  owner.company_id,
  owner.domain_id,
  owner.user_id,
  %s,
  'folder',
  $3,
  $4,
  'active'
FROM owner
%s
RETURNING
  id::text,
  company_id::text,
  domain_id::text,
  user_id::text,
  COALESCE(parent_id::text, ''),
  node_type,
  name,
  normalized_name,
  mime_type,
  size,
  storage_backend,
  storage_path,
  checksum_sha256,
  status,
  created_at,
  updated_at`, parentCTE, parentIDExpr, parentWhere)
}

func driveParentFolderInsertFragments(parentID string) (string, string, string) {
	if parentID == "" {
		return "", "NULLIF($2, '')::uuid", ""
	}
	return `,
parent AS (
  SELECT n.id
  FROM drive_nodes n
  JOIN owner ON owner.user_id = n.user_id
  WHERE n.id = $2::uuid
    AND n.node_type = 'folder'
    AND n.status = 'active'
)`, "(SELECT id FROM parent)", "WHERE EXISTS (SELECT 1 FROM parent)"
}

const listNodesBaseSQL = `
SELECT
  id::text,
  company_id::text,
  domain_id::text,
  user_id::text,
  COALESCE(parent_id::text, ''),
  node_type,
  name,
  normalized_name,
  mime_type,
  size,
  storage_backend,
  storage_path,
  checksum_sha256,
  status,
  created_at,
  updated_at
FROM drive_nodes`

func buildListNodesQuery(req ListNodesRequest) (string, []any) {
	args := []any{req.UserID, req.Status}
	conditions := []string{"user_id = $1::uuid", "status = $2"}
	if req.Query != "" {
		args = append(args, escapeDriveNodeLikeQuery(req.Query))
		conditions = append(conditions, fmt.Sprintf("normalized_name LIKE '%%' || $%d || '%%' ESCAPE '\\'", len(args)))
	}
	if req.NodeType != "" {
		args = append(args, req.NodeType)
		conditions = append(conditions, fmt.Sprintf("node_type = $%d", len(args)))
	}
	if !req.AllParents {
		if req.ParentID == "" {
			conditions = append(conditions, "parent_id IS NULL")
		} else {
			args = append(args, req.ParentID)
			conditions = append(conditions, fmt.Sprintf("parent_id = $%d::uuid", len(args)))
		}
	}
	args = append(args, req.Limit)
	query := listNodesBaseSQL + "\nWHERE " + strings.Join(conditions, "\n  AND ") + "\nORDER BY " + driveNodeListOrderBy(req.Sort) + fmt.Sprintf("\nLIMIT $%d", len(args))
	return query, args
}

func (r *Repository) ListNodes(ctx context.Context, req ListNodesRequest) ([]Node, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	req, err := ValidateListNodesRequest(req)
	if err != nil {
		return nil, err
	}
	query, args := buildListNodesQuery(req)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list drive nodes: %w", err)
	}
	defer rows.Close()

	nodes := make([]Node, 0, req.Limit)
	for rows.Next() {
		var node Node
		if err := rows.Scan(
			&node.ID,
			&node.CompanyID,
			&node.DomainID,
			&node.UserID,
			&node.ParentID,
			&node.Type,
			&node.Name,
			&node.NormalizedName,
			&node.MIMEType,
			&node.Size,
			&node.StorageBackend,
			&node.StoragePath,
			&node.ChecksumSHA256,
			&node.Status,
			&node.CreatedAt,
			&node.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan drive node: %w", err)
		}
		nodes = append(nodes, node)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate drive nodes: %w", err)
	}
	return nodes, nil
}

func driveNodeListOrderBy(sortMode string) string {
	typeFirst := "CASE WHEN node_type = 'folder' THEN 0 ELSE 1 END"
	switch sortMode {
	case NodeSortUpdated:
		return typeFirst + ", updated_at DESC, normalized_name ASC, id ASC"
	case NodeSortCreated:
		return typeFirst + ", created_at DESC, normalized_name ASC, id ASC"
	case NodeSortSize:
		return typeFirst + ", size DESC, normalized_name ASC, id ASC"
	default:
		return typeFirst + ", normalized_name ASC, id ASC"
	}
}

func (r *Repository) GetUsageSummary(ctx context.Context, req GetUsageSummaryRequest) (UsageSummary, error) {
	if r == nil || r.db == nil {
		return UsageSummary{}, fmt.Errorf("database handle is required")
	}
	req, err := ValidateGetUsageSummaryRequest(req)
	if err != nil {
		return UsageSummary{}, err
	}
	const query = `
WITH owner AS (
  SELECT
    u.id,
    u.quota_used,
    COALESCE(u.quota_limit, 0) AS quota_limit
  FROM users u
  WHERE u.id = $1::uuid
),
node_stats AS (
  SELECT
    COUNT(*) AS total_nodes,
    COUNT(*) FILTER (WHERE status = 'active') AS active_nodes,
    COUNT(*) FILTER (WHERE status = 'trashed') AS trashed_nodes,
    COUNT(*) FILTER (WHERE status = 'deleted') AS deleted_nodes,
    COUNT(*) FILTER (WHERE node_type = 'folder' AND status <> 'deleted') AS folder_count,
    COUNT(*) FILTER (WHERE node_type = 'file' AND status <> 'deleted') AS file_count,
    COALESCE(SUM(size) FILTER (WHERE status = 'active' AND node_type = 'file'), 0) AS active_bytes,
    COALESCE(SUM(size) FILTER (WHERE status = 'trashed' AND node_type = 'file'), 0) AS trashed_bytes,
    COALESCE(SUM(size) FILTER (WHERE status = 'deleted' AND node_type = 'file'), 0) AS deleted_bytes
  FROM drive_nodes
  WHERE user_id = $1::uuid
),
upload_stats AS (
  SELECT
    COUNT(*) FILTER (WHERE status = 'pending') AS pending_upload_sessions,
    COUNT(*) FILTER (WHERE status = 'uploading') AS uploading_upload_sessions,
    COUNT(*) FILTER (WHERE status = 'failed') AS failed_upload_sessions,
    COALESCE(SUM(declared_size) FILTER (WHERE status IN ('pending', 'uploading', 'failed')), 0) AS pending_upload_bytes
  FROM drive_upload_sessions
  WHERE user_id = $1::uuid
)
SELECT
  owner.id::text,
  owner.quota_used,
  owner.quota_limit,
  COALESCE(node_stats.total_nodes, 0),
  COALESCE(node_stats.active_nodes, 0),
  COALESCE(node_stats.trashed_nodes, 0),
  COALESCE(node_stats.deleted_nodes, 0),
  COALESCE(node_stats.folder_count, 0),
  COALESCE(node_stats.file_count, 0),
  COALESCE(node_stats.active_bytes, 0),
  COALESCE(node_stats.trashed_bytes, 0),
  COALESCE(node_stats.deleted_bytes, 0),
  COALESCE(upload_stats.pending_upload_sessions, 0),
  COALESCE(upload_stats.uploading_upload_sessions, 0),
  COALESCE(upload_stats.failed_upload_sessions, 0),
  COALESCE(upload_stats.pending_upload_bytes, 0)
FROM owner
CROSS JOIN node_stats
CROSS JOIN upload_stats`
	var summary UsageSummary
	err = r.db.QueryRowContext(ctx, query, req.UserID).Scan(
		&summary.UserID,
		&summary.QuotaUsed,
		&summary.QuotaLimit,
		&summary.TotalNodes,
		&summary.ActiveNodes,
		&summary.TrashedNodes,
		&summary.DeletedNodes,
		&summary.FolderCount,
		&summary.FileCount,
		&summary.ActiveBytes,
		&summary.TrashedBytes,
		&summary.DeletedBytes,
		&summary.PendingUploadSessions,
		&summary.UploadingUploadSessions,
		&summary.FailedUploadSessions,
		&summary.PendingUploadBytes,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return UsageSummary{}, fmt.Errorf("drive usage user not found")
		}
		return UsageSummary{}, fmt.Errorf("get drive usage summary: %w", err)
	}
	return summary, nil
}

func (r *Repository) GetNode(ctx context.Context, req GetNodeRequest) (Node, error) {
	if r == nil || r.db == nil {
		return Node{}, fmt.Errorf("database handle is required")
	}
	req, err := ValidateGetNodeRequest(req)
	if err != nil {
		return Node{}, err
	}
	const query = `
SELECT
  n.id::text,
  n.company_id::text,
  n.domain_id::text,
  n.user_id::text,
  COALESCE(n.parent_id::text, ''),
  n.node_type,
  n.name,
  n.normalized_name,
  n.mime_type,
  n.size,
  n.storage_backend,
  n.storage_path,
  n.checksum_sha256,
  n.status,
  n.created_at,
  n.updated_at
FROM drive_nodes n
JOIN users u ON u.id = n.user_id
JOIN domains d ON d.id = u.domain_id
WHERE n.id = $2::uuid
  AND n.user_id = $1::uuid
  AND n.status = $3
  AND u.status = 'active'
  AND d.status = 'active'`
	var node Node
	err = r.db.QueryRowContext(ctx, query, req.UserID, req.NodeID, req.Status).Scan(
		&node.ID,
		&node.CompanyID,
		&node.DomainID,
		&node.UserID,
		&node.ParentID,
		&node.Type,
		&node.Name,
		&node.NormalizedName,
		&node.MIMEType,
		&node.Size,
		&node.StorageBackend,
		&node.StoragePath,
		&node.ChecksumSHA256,
		&node.Status,
		&node.CreatedAt,
		&node.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return Node{}, fmt.Errorf("drive node not found")
		}
		return Node{}, fmt.Errorf("get drive node: %w", err)
	}
	return node, nil
}

func (r *Repository) TrashNode(ctx context.Context, req TrashNodeRequest) (Node, int64, error) {
	if r == nil || r.db == nil {
		return Node{}, 0, fmt.Errorf("database handle is required")
	}
	req, err := ValidateTrashNodeRequest(req)
	if err != nil {
		return Node{}, 0, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Node{}, 0, fmt.Errorf("begin trash drive node transaction: %w", err)
	}
	defer tx.Rollback()

	root, err := lockActiveDriveNode(ctx, tx, req.UserID, req.NodeID)
	if err != nil {
		return Node{}, 0, err
	}
	updated, err := trashDriveNodeTree(ctx, tx, req.UserID, req.NodeID)
	if err != nil {
		return Node{}, 0, err
	}
	if err := tx.Commit(); err != nil {
		return Node{}, 0, fmt.Errorf("commit trash drive node transaction: %w", err)
	}
	root.Status = NodeStatusTrashed
	now := time.Now().UTC()
	root.UpdatedAt = now
	return root, updated, nil
}

func (r *Repository) RestoreNode(ctx context.Context, req RestoreNodeRequest) (Node, int64, error) {
	if r == nil || r.db == nil {
		return Node{}, 0, fmt.Errorf("database handle is required")
	}
	req, err := ValidateRestoreNodeRequest(req)
	if err != nil {
		return Node{}, 0, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Node{}, 0, fmt.Errorf("begin restore drive node transaction: %w", err)
	}
	defer tx.Rollback()

	root, err := lockTrashedDriveNode(ctx, tx, req.UserID, req.NodeID)
	if err != nil {
		return Node{}, 0, err
	}
	updated, err := restoreDriveNodeTree(ctx, tx, req.UserID, req.NodeID)
	if err != nil {
		return Node{}, 0, err
	}
	if err := tx.Commit(); err != nil {
		return Node{}, 0, fmt.Errorf("commit restore drive node transaction: %w", err)
	}
	root.Status = NodeStatusActive
	root.UpdatedAt = time.Now().UTC()
	return root, updated, nil
}

func (r *Repository) RenameNode(ctx context.Context, req RenameNodeRequest) (Node, error) {
	if r == nil || r.db == nil {
		return Node{}, fmt.Errorf("database handle is required")
	}
	req, normalizedName, err := ValidateRenameNodeRequest(req)
	if err != nil {
		return Node{}, err
	}
	const query = `
WITH owner AS (
  SELECT u.id AS user_id
  FROM users u
  JOIN domains d ON d.id = u.domain_id
  WHERE u.id = $1::uuid
    AND u.status = 'active'
    AND d.status = 'active'
),
updated AS (
  UPDATE drive_nodes n
  SET
    name = $3,
    normalized_name = $4,
    updated_at = now()
  FROM owner
  WHERE n.id = $2::uuid
    AND n.user_id = owner.user_id
    AND n.status = 'active'
  RETURNING
    n.id::text,
    n.company_id::text,
    n.domain_id::text,
    n.user_id::text,
    COALESCE(n.parent_id::text, ''),
    n.node_type,
    n.name,
    n.normalized_name,
    n.mime_type,
    n.size,
    n.storage_backend,
    n.storage_path,
    n.checksum_sha256,
    n.status,
    n.created_at,
    n.updated_at
)
SELECT
  id,
  company_id,
  domain_id,
  user_id,
  parent_id,
  node_type,
  name,
  normalized_name,
  mime_type,
  size,
  storage_backend,
  storage_path,
  checksum_sha256,
  status,
  created_at,
  updated_at
FROM updated`
	var node Node
	err = r.db.QueryRowContext(ctx, query, req.UserID, req.NodeID, req.Name, normalizedName).Scan(
		&node.ID,
		&node.CompanyID,
		&node.DomainID,
		&node.UserID,
		&node.ParentID,
		&node.Type,
		&node.Name,
		&node.NormalizedName,
		&node.MIMEType,
		&node.Size,
		&node.StorageBackend,
		&node.StoragePath,
		&node.ChecksumSHA256,
		&node.Status,
		&node.CreatedAt,
		&node.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return Node{}, fmt.Errorf("active drive node not found")
		}
		return Node{}, fmt.Errorf("rename drive node: %w", err)
	}
	return node, nil
}

func lockTrashedDriveNodeForDelete(ctx context.Context, tx *sql.Tx, userID string, nodeID string) (Node, error) {
	const query = `
SELECT
  id::text,
  company_id::text,
  domain_id::text,
  user_id::text,
  COALESCE(parent_id::text, ''),
  node_type,
  name,
  normalized_name,
  mime_type,
  size,
  storage_backend,
  storage_path,
  checksum_sha256,
  status,
  created_at,
  updated_at
FROM drive_nodes
WHERE id = $1::uuid
  AND user_id = $2::uuid
  AND status = 'trashed'
FOR UPDATE`
	var node Node
	err := tx.QueryRowContext(ctx, query, nodeID, userID).Scan(
		&node.ID,
		&node.CompanyID,
		&node.DomainID,
		&node.UserID,
		&node.ParentID,
		&node.Type,
		&node.Name,
		&node.NormalizedName,
		&node.MIMEType,
		&node.Size,
		&node.StorageBackend,
		&node.StoragePath,
		&node.ChecksumSHA256,
		&node.Status,
		&node.CreatedAt,
		&node.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Node{}, fmt.Errorf("trashed drive node not found")
		}
		return Node{}, fmt.Errorf("lock trashed drive node for delete: %w", err)
	}
	return node, nil
}

func lockTrashedDriveNode(ctx context.Context, tx *sql.Tx, userID string, nodeID string) (Node, error) {
	const query = `
SELECT
  id::text,
  company_id::text,
  domain_id::text,
  user_id::text,
  COALESCE(parent_id::text, ''),
  node_type,
  name,
  normalized_name,
  mime_type,
  size,
  storage_backend,
  storage_path,
  checksum_sha256,
  status,
  created_at,
  updated_at
FROM drive_nodes
WHERE id = $1::uuid
  AND user_id = $2::uuid
  AND status = 'trashed'
  AND (
    parent_id IS NULL
    OR EXISTS (
      SELECT 1
      FROM drive_nodes parent
      WHERE parent.id = drive_nodes.parent_id
        AND parent.user_id = $2::uuid
        AND parent.status = 'active'
    )
  )
FOR UPDATE`
	var node Node
	err := tx.QueryRowContext(ctx, query, nodeID, userID).Scan(
		&node.ID,
		&node.CompanyID,
		&node.DomainID,
		&node.UserID,
		&node.ParentID,
		&node.Type,
		&node.Name,
		&node.NormalizedName,
		&node.MIMEType,
		&node.Size,
		&node.StorageBackend,
		&node.StoragePath,
		&node.ChecksumSHA256,
		&node.Status,
		&node.CreatedAt,
		&node.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Node{}, fmt.Errorf("restorable trashed drive node not found")
		}
		return Node{}, fmt.Errorf("lock trashed drive node: %w", err)
	}
	return node, nil
}

func lockActiveDriveNode(ctx context.Context, tx *sql.Tx, userID string, nodeID string) (Node, error) {
	const query = `
SELECT
  id::text,
  company_id::text,
  domain_id::text,
  user_id::text,
  COALESCE(parent_id::text, ''),
  node_type,
  name,
  normalized_name,
  mime_type,
  size,
  storage_backend,
  storage_path,
  checksum_sha256,
  status,
  created_at,
  updated_at
FROM drive_nodes
WHERE id = $1::uuid
  AND user_id = $2::uuid
  AND status = 'active'
FOR UPDATE`
	var node Node
	err := tx.QueryRowContext(ctx, query, nodeID, userID).Scan(
		&node.ID,
		&node.CompanyID,
		&node.DomainID,
		&node.UserID,
		&node.ParentID,
		&node.Type,
		&node.Name,
		&node.NormalizedName,
		&node.MIMEType,
		&node.Size,
		&node.StorageBackend,
		&node.StoragePath,
		&node.ChecksumSHA256,
		&node.Status,
		&node.CreatedAt,
		&node.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Node{}, fmt.Errorf("active drive node not found")
		}
		return Node{}, fmt.Errorf("lock drive node: %w", err)
	}
	return node, nil
}

func trashDriveNodeTree(ctx context.Context, tx *sql.Tx, userID string, nodeID string) (int64, error) {
	const query = `
WITH RECURSIVE tree AS (
  SELECT id
  FROM drive_nodes
  WHERE id = $2::uuid
    AND user_id = $1::uuid
    AND status = 'active'
  UNION ALL
  SELECT child.id
  FROM drive_nodes child
  JOIN tree ON child.parent_id = tree.id
  WHERE child.user_id = $1::uuid
    AND child.status = 'active'
)
UPDATE drive_nodes
SET status = 'trashed',
    trashed_at = COALESCE(trashed_at, now()),
    updated_at = now()
WHERE id IN (SELECT id FROM tree)`
	result, err := tx.ExecContext(ctx, query, userID, nodeID)
	if err != nil {
		return 0, fmt.Errorf("trash drive node tree: %w", err)
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("count trashed drive nodes: %w", err)
	}
	if updated == 0 {
		return 0, fmt.Errorf("active drive node not found")
	}
	return updated, nil
}

func markDriveNodeTreeDeleted(ctx context.Context, tx *sql.Tx, userID string, nodeID string) (int64, int64, []DeletedObject, error) {
	const query = `
WITH RECURSIVE tree AS (
  SELECT id
  FROM drive_nodes
  WHERE id = $2::uuid
    AND user_id = $1::uuid
    AND status = 'trashed'
  UNION ALL
  SELECT child.id
  FROM drive_nodes child
  JOIN tree ON child.parent_id = tree.id
  WHERE child.user_id = $1::uuid
    AND child.status = 'trashed'
)
UPDATE drive_nodes
SET status = 'deleted',
    deleted_at = COALESCE(deleted_at, now()),
    updated_at = now()
WHERE id IN (SELECT id FROM tree)
RETURNING node_type, size, storage_backend, storage_path`
	rows, err := tx.QueryContext(ctx, query, userID, nodeID)
	if err != nil {
		return 0, 0, nil, fmt.Errorf("mark drive node tree deleted: %w", err)
	}
	defer rows.Close()

	var deletedNodes int64
	var releasedBytes int64
	var objects []DeletedObject
	for rows.Next() {
		var nodeType string
		var size int64
		var storageBackend string
		var storagePath string
		if err := rows.Scan(&nodeType, &size, &storageBackend, &storagePath); err != nil {
			return 0, 0, nil, fmt.Errorf("scan deleted drive node: %w", err)
		}
		deletedNodes++
		if nodeType != NodeTypeFile {
			continue
		}
		releasedBytes += size
		if storageBackend != "" && storagePath != "" {
			objects = append(objects, DeletedObject{StorageBackend: storageBackend, StoragePath: storagePath})
		}
	}
	if err := rows.Err(); err != nil {
		return 0, 0, nil, fmt.Errorf("iterate deleted drive nodes: %w", err)
	}
	if deletedNodes == 0 {
		return 0, 0, nil, fmt.Errorf("trashed drive node not found")
	}
	return deletedNodes, releasedBytes, objects, nil
}

func restoreDriveNodeTree(ctx context.Context, tx *sql.Tx, userID string, nodeID string) (int64, error) {
	const query = `
WITH RECURSIVE tree AS (
  SELECT id
  FROM drive_nodes
  WHERE id = $2::uuid
    AND user_id = $1::uuid
    AND status = 'trashed'
    AND (
      parent_id IS NULL
      OR EXISTS (
        SELECT 1
        FROM drive_nodes parent
        WHERE parent.id = drive_nodes.parent_id
          AND parent.user_id = $1::uuid
          AND parent.status = 'active'
      )
    )
  UNION ALL
  SELECT child.id
  FROM drive_nodes child
  JOIN tree ON child.parent_id = tree.id
  WHERE child.user_id = $1::uuid
    AND child.status = 'trashed'
)
UPDATE drive_nodes
SET status = 'active',
    trashed_at = NULL,
    updated_at = now()
WHERE id IN (SELECT id FROM tree)`
	result, err := tx.ExecContext(ctx, query, userID, nodeID)
	if err != nil {
		return 0, fmt.Errorf("restore drive node tree: %w", err)
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("count restored drive nodes: %w", err)
	}
	if updated == 0 {
		return 0, fmt.Errorf("restorable trashed drive node not found")
	}
	return updated, nil
}

func (r *Repository) findActiveNodeBySiblingName(ctx context.Context, userID, parentID, normalizedName, nodeType string) (Node, error) {
	if r == nil || r.db == nil {
		return Node{}, fmt.Errorf("database handle is required")
	}
	query, args := buildFindActiveNodeBySiblingNameQuery(userID, parentID, normalizedName, nodeType)
	var node Node
	err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&node.ID,
		&node.CompanyID,
		&node.DomainID,
		&node.UserID,
		&node.ParentID,
		&node.Type,
		&node.Name,
		&node.NormalizedName,
		&node.MIMEType,
		&node.Size,
		&node.StorageBackend,
		&node.StoragePath,
		&node.ChecksumSHA256,
		&node.Status,
		&node.CreatedAt,
		&node.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Node{}, fmt.Errorf("%w: drive node already exists in this folder", ErrDriveNodeAlreadyExists)
		}
		return Node{}, fmt.Errorf("lookup existing drive node: %w", err)
	}
	return node, nil
}

func buildFindActiveNodeBySiblingNameQuery(userID, parentID, normalizedName, nodeType string) (string, []any) {
	args := []any{userID}
	parentPredicate := "parent_id IS NULL"
	nameArg := 2
	typeArg := 3
	if strings.TrimSpace(parentID) != "" {
		args = append(args, parentID)
		parentPredicate = "parent_id = $2::uuid"
		nameArg = 3
		typeArg = 4
	}
	args = append(args, normalizedName, nodeType)
	query := fmt.Sprintf(`
SELECT
  id::text,
  company_id::text,
  domain_id::text,
  user_id::text,
  COALESCE(parent_id::text, ''),
  node_type,
  name,
  normalized_name,
  mime_type,
  size,
  storage_backend,
  storage_path,
  checksum_sha256,
  status,
  created_at,
  updated_at
FROM drive_nodes
WHERE user_id = $1::uuid
  AND status = 'active'
  AND %s
  AND normalized_name = $%d
  AND node_type = $%d`, parentPredicate, nameArg, typeArg)
	return query, args
}

func isDriveNodeSiblingNameConflict(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "idx_drive_nodes_user_active_sibling_name"
}

func mapDriveFileCreateError(err error) error {
	if isDriveNodeSiblingNameConflict(err) {
		return fmt.Errorf("%w: drive file already exists in this folder", ErrDriveNodeAlreadyExists)
	}
	return fmt.Errorf("create drive file: %w", err)
}

func validateDriveID(field string, value string, required bool) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		if required {
			return "", fmt.Errorf("%s is required", field)
		}
		return "", nil
	}
	if len(value) > 128 {
		return "", fmt.Errorf("%s is too long", field)
	}
	if strings.ContainsAny(value, "\r\n") {
		return "", fmt.Errorf("%s must not contain line breaks", field)
	}
	return value, nil
}

func validateStorageBackend(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("storage_backend is required")
	}
	if len(value) > 64 {
		return "", fmt.Errorf("storage_backend is too long")
	}
	if strings.ContainsAny(value, "\r\n") {
		return "", fmt.Errorf("storage_backend must not contain line breaks")
	}
	return value, nil
}

func validateDriveMIMEType(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "application/octet-stream", nil
	}
	if len(value) > 255 {
		return "", fmt.Errorf("mime_type is too long")
	}
	if strings.ContainsAny(value, "\r\n") {
		return "", fmt.Errorf("mime_type must not contain line breaks")
	}
	return value, nil
}

func validateDriveChecksum(value string) (string, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return "", nil
	}
	if len(value) != 64 {
		return "", fmt.Errorf("checksum_sha256 must be 64 lowercase hex characters")
	}
	for _, r := range value {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return "", fmt.Errorf("checksum_sha256 must be 64 lowercase hex characters")
		}
	}
	return value, nil
}

func normalizeDriveListLimit(limit int) int {
	if limit <= 0 {
		return 50
	}
	if limit > 200 {
		return 200
	}
	return limit
}
