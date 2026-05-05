package drive

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/storage"
)

type ObjectCleanupFailureRecorder interface {
	RecordObjectCleanupFailure(context.Context, ObjectCleanupFailure) (ObjectCleanupFailure, error)
}

type ObjectCleanupFailureStore interface {
	ObjectCleanupFailureRecorder
	ListObjectCleanupFailures(context.Context, ListObjectCleanupFailuresRequest) ([]ObjectCleanupFailure, error)
	ResolveObjectCleanupFailure(context.Context, ResolveObjectCleanupFailureRequest) (ObjectCleanupFailure, error)
}

type Service struct {
	repo                   *Repository
	stores                 map[string]storage.Store
	cleanupFailureRecorder ObjectCleanupFailureRecorder
	cleanupFailureStore    ObjectCleanupFailureStore
}

type PermanentDeleteServiceResult struct {
	PermanentDelete PermanentDeleteResult `json:"permanent_delete"`
	Cleanup         ObjectCleanupResult   `json:"cleanup"`
}

type RetryObjectCleanupFailuresResult struct {
	Scanned  int `json:"scanned"`
	Deleted  int `json:"deleted"`
	Resolved int `json:"resolved"`
	Failed   int `json:"failed"`
}

const MaxDriveCopyNodes = 500

const driveCopyChildrenPageLimit = 200

func NewService(repo *Repository, stores map[string]storage.Store) *Service {
	copiedStores := make(map[string]storage.Store, len(stores))
	for backend, store := range stores {
		copiedStores[backend] = store
	}
	service := &Service{repo: repo, stores: copiedStores}
	if repo != nil {
		service.cleanupFailureRecorder = repo
		service.cleanupFailureStore = repo
	}
	return service
}

func (s *Service) WithObjectCleanupFailureRecorder(recorder ObjectCleanupFailureRecorder) *Service {
	if s == nil {
		return nil
	}
	s.cleanupFailureRecorder = recorder
	return s
}

func (s *Service) WithObjectCleanupFailureStore(store ObjectCleanupFailureStore) *Service {
	if s == nil {
		return nil
	}
	s.cleanupFailureStore = store
	s.cleanupFailureRecorder = store
	return s
}

func (s *Service) CreateFolder(ctx context.Context, req CreateFolderRequest) (Node, error) {
	if s == nil || s.repo == nil {
		return Node{}, fmt.Errorf("drive repository is required")
	}
	return s.repo.CreateFolder(ctx, req)
}

func (s *Service) CreateFileFromObject(ctx context.Context, req CreateFileFromObjectRequest) (Node, error) {
	if s == nil || s.repo == nil {
		return Node{}, fmt.Errorf("drive repository is required")
	}
	storageBackend, err := validateStorageBackend(req.StorageBackend)
	if err != nil {
		return Node{}, err
	}
	store := s.stores[storageBackend]
	if store == nil {
		return Node{}, fmt.Errorf("storage store %q is required", storageBackend)
	}
	return s.repo.CreateFileFromObject(ctx, store, req)
}

func (s *Service) CreateUploadSession(ctx context.Context, req CreateUploadSessionRequest) (UploadSession, error) {
	if s == nil || s.repo == nil {
		return UploadSession{}, fmt.Errorf("drive repository is required")
	}
	if strings.TrimSpace(req.UploadID) == "" {
		uploadID, err := NewUploadID()
		if err != nil {
			return UploadSession{}, err
		}
		req.UploadID = uploadID
	}
	return s.repo.CreateUploadSession(ctx, req)
}

func (s *Service) GetUploadSession(ctx context.Context, req GetUploadSessionRequest) (UploadSession, error) {
	if s == nil || s.repo == nil {
		return UploadSession{}, fmt.Errorf("drive repository is required")
	}
	return s.repo.GetUploadSession(ctx, req)
}

func (s *Service) ListUploadSessions(ctx context.Context, req ListUploadSessionsRequest) ([]UploadSession, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("drive repository is required")
	}
	return s.repo.ListUploadSessions(ctx, req)
}

func (s *Service) CancelUploadSession(ctx context.Context, req CancelUploadSessionRequest) (UploadSession, error) {
	if s == nil || s.repo == nil {
		return UploadSession{}, fmt.Errorf("drive repository is required")
	}
	return s.repo.CancelUploadSession(ctx, req)
}

func (s *Service) ExpireUploadSessions(ctx context.Context, req ExpireUploadSessionsRequest) ([]UploadSession, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("drive repository is required")
	}
	req, err := ValidateExpireUploadSessionsRequest(req)
	if err != nil {
		return nil, err
	}
	expired, err := s.repo.ExpireUploadSessions(ctx, req)
	if err != nil {
		return nil, err
	}
	for _, session := range expired {
		storagePath := strings.TrimSpace(session.StoragePath)
		if storagePath == "" {
			continue
		}
		if _, err := storage.ValidateObjectPath(storagePath); err != nil {
			return expired, fmt.Errorf("expired drive upload session storage path is invalid: %w", err)
		}
		store := s.stores[session.StorageBackend]
		if store == nil {
			return expired, fmt.Errorf("storage store %q is required", session.StorageBackend)
		}
		if err := store.Delete(ctx, storagePath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return expired, fmt.Errorf("delete expired drive upload session body: %w", err)
		}
	}
	return expired, nil
}

func (s *Service) CountStaleUploadSessions(ctx context.Context, req ExpireUploadSessionsRequest) (StaleUploadSessionCount, error) {
	if s == nil || s.repo == nil {
		return StaleUploadSessionCount{}, fmt.Errorf("drive repository is required")
	}
	return s.repo.CountStaleUploadSessions(ctx, req)
}

func (s *Service) ListStaleUploadSessions(ctx context.Context, req ExpireUploadSessionsRequest) ([]UploadSession, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("drive repository is required")
	}
	return s.repo.ListStaleUploadSessions(ctx, req)
}

func (s *Service) StoreUploadSessionBody(ctx context.Context, req StoreUploadSessionBodyRequest) (UploadSession, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if s == nil || s.repo == nil {
		return UploadSession{}, fmt.Errorf("drive repository is required")
	}
	req, err := ValidateStoreUploadSessionBodyRequest(req)
	if err != nil {
		return UploadSession{}, err
	}
	session, err := s.repo.GetUploadSession(ctx, GetUploadSessionRequest{UserID: req.UserID, SessionID: req.SessionID})
	if err != nil {
		return UploadSession{}, err
	}
	if session.Status != UploadSessionStatusPending && session.Status != UploadSessionStatusUploading && session.Status != UploadSessionStatusFailed {
		return UploadSession{}, fmt.Errorf("drive upload session is not writable")
	}
	if !session.ExpiresAt.After(time.Now().UTC()) {
		return UploadSession{}, fmt.Errorf("drive upload session is expired")
	}
	store := s.stores[session.StorageBackend]
	if store == nil {
		return UploadSession{}, fmt.Errorf("storage store %q is required", session.StorageBackend)
	}
	objectID, err := NewUploadID()
	if err != nil {
		return UploadSession{}, err
	}
	storagePath, err := BuildUploadSessionBodyPath(session.UserID, session.ID, objectID)
	if err != nil {
		return UploadSession{}, err
	}

	counter := &countingReader{reader: req.Body}
	limited := &io.LimitedReader{R: counter, N: session.DeclaredSize + 1}
	hash := sha256.New()
	if err := store.Put(ctx, storagePath, io.TeeReader(limited, hash)); err != nil {
		return UploadSession{}, fmt.Errorf("store drive upload session body: %w", err)
	}
	if counter.bytesRead > session.DeclaredSize {
		_ = store.Delete(ctx, storagePath)
		return UploadSession{}, fmt.Errorf("drive upload session body exceeds declared_size")
	}
	checksum := hex.EncodeToString(hash.Sum(nil))
	if req.ExpectedChecksumSHA256 != "" && checksum != req.ExpectedChecksumSHA256 {
		_ = store.Delete(ctx, storagePath)
		return UploadSession{}, fmt.Errorf("drive upload session checksum mismatch")
	}
	updated, err := s.repo.StoreUploadSessionBody(ctx, RecordUploadSessionBodyRequest{
		UserID:         req.UserID,
		SessionID:      req.SessionID,
		ReceivedSize:   counter.bytesRead,
		StoragePath:    storagePath,
		ChecksumSHA256: checksum,
	})
	if err != nil {
		_ = store.Delete(ctx, storagePath)
		return UploadSession{}, err
	}
	if session.StoragePath != "" && session.StoragePath != storagePath {
		_ = store.Delete(ctx, session.StoragePath)
	}
	return updated, nil
}

func (s *Service) FinalizeUploadSession(ctx context.Context, req FinalizeUploadSessionRequest) (Node, error) {
	if s == nil || s.repo == nil {
		return Node{}, fmt.Errorf("drive repository is required")
	}
	req, err := ValidateFinalizeUploadSessionRequest(req)
	if err != nil {
		return Node{}, err
	}
	session, err := s.repo.GetUploadSession(ctx, GetUploadSessionRequest{UserID: req.UserID, SessionID: req.SessionID})
	if err != nil {
		return Node{}, err
	}
	storageBackend, err := validateStorageBackend(session.StorageBackend)
	if err != nil {
		return Node{}, err
	}
	store := s.stores[storageBackend]
	if store == nil {
		return Node{}, fmt.Errorf("storage store %q is required", storageBackend)
	}
	return s.repo.FinalizeUploadSession(ctx, store, req)
}

func (s *Service) ListNodes(ctx context.Context, req ListNodesRequest) ([]Node, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("drive repository is required")
	}
	return s.repo.ListNodes(ctx, req)
}

func (s *Service) GetNode(ctx context.Context, req GetNodeRequest) (Node, error) {
	if s == nil || s.repo == nil {
		return Node{}, fmt.Errorf("drive repository is required")
	}
	return s.repo.GetNode(ctx, req)
}

func (s *Service) OpenFile(ctx context.Context, req OpenFileRequest) (FileDownload, error) {
	node, storagePath, store, err := s.driveFileObject(ctx, req)
	if err != nil {
		return FileDownload{}, err
	}
	body, err := store.Get(ctx, storagePath)
	if err != nil {
		return FileDownload{}, fmt.Errorf("open drive file object: %w", err)
	}
	return FileDownload{Node: node, Body: body}, nil
}

func (s *Service) OpenFileRange(ctx context.Context, req OpenFileRangeRequest) (FileDownload, error) {
	rangeReq, err := storage.ValidateRangeRequest(storage.RangeRequest{Offset: req.Offset, Length: req.Length})
	if err != nil {
		return FileDownload{}, err
	}
	node, storagePath, store, err := s.driveFileObject(ctx, OpenFileRequest{UserID: req.UserID, NodeID: req.NodeID})
	if err != nil {
		return FileDownload{}, err
	}
	body, err := store.GetRange(ctx, storagePath, rangeReq)
	if err != nil {
		return FileDownload{}, fmt.Errorf("open drive file object range: %w", err)
	}
	return FileDownload{Node: node, Body: body}, nil
}

func (s *Service) StatFile(ctx context.Context, req OpenFileRequest) (FileMetadata, error) {
	node, storagePath, store, err := s.driveFileObject(ctx, req)
	if err != nil {
		return FileMetadata{}, err
	}
	info, err := store.Stat(ctx, storagePath)
	if err != nil {
		return FileMetadata{}, fmt.Errorf("stat drive file object: %w", err)
	}
	return FileMetadata{Node: node, Object: info}, nil
}

func (s *Service) driveFileObject(ctx context.Context, req OpenFileRequest) (Node, string, storage.Store, error) {
	if s == nil || s.repo == nil {
		return Node{}, "", nil, fmt.Errorf("drive repository is required")
	}
	validated, err := ValidateGetNodeRequest(GetNodeRequest{UserID: req.UserID, NodeID: req.NodeID, Status: NodeStatusActive})
	if err != nil {
		return Node{}, "", nil, err
	}
	node, err := s.repo.GetNode(ctx, validated)
	if err != nil {
		return Node{}, "", nil, err
	}
	if node.Type != NodeTypeFile {
		return Node{}, "", nil, fmt.Errorf("drive node is not a file")
	}
	storageBackend, err := validateStorageBackend(node.StorageBackend)
	if err != nil {
		return Node{}, "", nil, err
	}
	storagePath, err := storage.ValidateObjectPath(node.StoragePath)
	if err != nil {
		return Node{}, "", nil, fmt.Errorf("unsafe drive file storage path: %w", err)
	}
	store := s.stores[storageBackend]
	if store == nil {
		return Node{}, "", nil, fmt.Errorf("storage store %q is required", storageBackend)
	}
	return node, storagePath, store, nil
}

func (s *Service) GetUsageSummary(ctx context.Context, req GetUsageSummaryRequest) (UsageSummary, error) {
	if s == nil || s.repo == nil {
		return UsageSummary{}, fmt.Errorf("drive repository is required")
	}
	return s.repo.GetUsageSummary(ctx, req)
}

func (s *Service) TrashNode(ctx context.Context, req TrashNodeRequest) (Node, int64, error) {
	if s == nil || s.repo == nil {
		return Node{}, 0, fmt.Errorf("drive repository is required")
	}
	return s.repo.TrashNode(ctx, req)
}

func (s *Service) RestoreNode(ctx context.Context, req RestoreNodeRequest) (Node, int64, error) {
	if s == nil || s.repo == nil {
		return Node{}, 0, fmt.Errorf("drive repository is required")
	}
	return s.repo.RestoreNode(ctx, req)
}

func (s *Service) RenameNode(ctx context.Context, req RenameNodeRequest) (Node, error) {
	if s == nil || s.repo == nil {
		return Node{}, fmt.Errorf("drive repository is required")
	}
	return s.repo.RenameNode(ctx, req)
}

func (s *Service) MoveNode(ctx context.Context, req MoveNodeRequest) (Node, error) {
	if s == nil || s.repo == nil {
		return Node{}, fmt.Errorf("drive repository is required")
	}
	return s.repo.MoveNode(ctx, req)
}

func (s *Service) CopyNode(ctx context.Context, req CopyNodeRequest) (Node, error) {
	if s == nil || s.repo == nil {
		return Node{}, fmt.Errorf("drive repository is required")
	}
	req, _, err := ValidateCopyNodeRequest(req)
	if err != nil {
		return Node{}, err
	}
	source, err := s.repo.GetNode(ctx, GetNodeRequest{UserID: req.UserID, NodeID: req.NodeID})
	if err != nil {
		return Node{}, err
	}
	switch source.Type {
	case NodeTypeFile:
		return s.copyDriveFileNode(ctx, source, req.ParentID, req.Name)
	case NodeTypeFolder:
		return s.copyDriveFolderNode(ctx, source, req.ParentID, req.Name)
	default:
		return Node{}, fmt.Errorf("unsupported drive node type %q", source.Type)
	}
}

func (s *Service) copyDriveFolderNode(ctx context.Context, source Node, parentID string, name string) (Node, error) {
	root, err := s.repo.CreateFolder(ctx, CreateFolderRequest{UserID: source.UserID, ParentID: parentID, Name: name})
	if err != nil {
		return Node{}, err
	}
	remaining := MaxDriveCopyNodes - 1
	if err := s.copyDriveFolderChildren(ctx, source.UserID, source.ID, root.ID, &remaining); err != nil {
		if cleanupErr := s.cleanupCopiedDriveTree(ctx, source.UserID, root.ID); cleanupErr != nil {
			return Node{}, fmt.Errorf("copy drive folder tree: %v; cleanup copied tree: %w", err, cleanupErr)
		}
		return Node{}, err
	}
	return root, nil
}

func (s *Service) copyDriveFolderChildren(ctx context.Context, userID string, sourceParentID string, destParentID string, remaining *int) error {
	if remaining == nil || *remaining <= 0 {
		return fmt.Errorf("drive folder copy exceeds %d nodes", MaxDriveCopyNodes)
	}
	children, err := s.repo.ListNodes(ctx, ListNodesRequest{
		UserID:   userID,
		ParentID: sourceParentID,
		Status:   NodeStatusActive,
		Limit:    driveCopyChildrenPageLimit,
	})
	if err != nil {
		return err
	}
	if len(children) >= driveCopyChildrenPageLimit {
		return fmt.Errorf("drive folder copy child page exceeds supported limit")
	}
	for _, child := range children {
		if *remaining <= 0 {
			return fmt.Errorf("drive folder copy exceeds %d nodes", MaxDriveCopyNodes)
		}
		*remaining = *remaining - 1
		switch child.Type {
		case NodeTypeFile:
			if _, err := s.copyDriveFileNode(ctx, child, destParentID, child.Name); err != nil {
				return err
			}
		case NodeTypeFolder:
			copiedFolder, err := s.repo.CreateFolder(ctx, CreateFolderRequest{UserID: userID, ParentID: destParentID, Name: child.Name})
			if err != nil {
				return err
			}
			if err := s.copyDriveFolderChildren(ctx, userID, child.ID, copiedFolder.ID, remaining); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported drive node type %q", child.Type)
		}
	}
	return nil
}

func (s *Service) copyDriveFileNode(ctx context.Context, source Node, parentID string, name string) (Node, error) {
	if source.Type != NodeTypeFile {
		return Node{}, fmt.Errorf("drive node is not a file")
	}
	if source.Status != NodeStatusActive {
		return Node{}, fmt.Errorf("drive node is not active")
	}
	sourcePath, err := storage.ValidateObjectPath(source.StoragePath)
	if err != nil {
		return Node{}, fmt.Errorf("unsafe drive file storage path: %w", err)
	}
	store := s.stores[source.StorageBackend]
	if store == nil {
		return Node{}, fmt.Errorf("storage store %q is required", source.StorageBackend)
	}
	if _, err := store.Stat(ctx, sourcePath); err != nil {
		return Node{}, fmt.Errorf("stat source drive file object: %w", err)
	}
	newNodeID, err := NewNodeID()
	if err != nil {
		return Node{}, err
	}
	destPath, err := BuildNodeObjectPath(source.UserID, newNodeID)
	if err != nil {
		return Node{}, err
	}
	if err := store.Copy(ctx, sourcePath, destPath); err != nil {
		return Node{}, fmt.Errorf("copy drive file object: %w", err)
	}
	node, err := s.repo.CreateFileFromObject(ctx, store, CreateFileFromObjectRequest{
		NodeID:         newNodeID,
		UserID:         source.UserID,
		ParentID:       parentID,
		Name:           name,
		StorageBackend: source.StorageBackend,
		StoragePath:    destPath,
		MIMEType:       source.MIMEType,
		ChecksumSHA256: source.ChecksumSHA256,
	})
	if err != nil {
		if cleanupErr := store.Delete(ctx, destPath); cleanupErr != nil {
			if recordErr := s.recordCopiedObjectCleanupFailure(ctx, source.UserID, source.StorageBackend, destPath, cleanupErr); recordErr != nil {
				return Node{}, fmt.Errorf("record drive copy cleanup failure after metadata error %v and cleanup error %v: %w", err, cleanupErr, recordErr)
			}
			return Node{}, fmt.Errorf("create copied drive file metadata: %v; cleanup copied object: %w", err, cleanupErr)
		}
		return Node{}, err
	}
	return node, nil
}

func (s *Service) cleanupCopiedDriveTree(ctx context.Context, userID string, rootID string) error {
	if _, _, err := s.repo.TrashNode(ctx, TrashNodeRequest{UserID: userID, NodeID: rootID}); err != nil {
		return err
	}
	_, err := s.PermanentDeleteNode(ctx, PermanentDeleteNodeRequest{UserID: userID, NodeID: rootID})
	return err
}

func (s *Service) PermanentDeleteNode(ctx context.Context, req PermanentDeleteNodeRequest) (PermanentDeleteServiceResult, error) {
	if s == nil || s.repo == nil {
		return PermanentDeleteServiceResult{}, fmt.Errorf("drive repository is required")
	}
	deleted, err := s.repo.PermanentDeleteNode(ctx, req)
	if err != nil {
		return PermanentDeleteServiceResult{}, err
	}
	cleanup, err := CleanupDeletedObjects(ctx, s.stores, deleted.Objects)
	result := PermanentDeleteServiceResult{
		PermanentDelete: deleted,
		Cleanup:         cleanup,
	}
	if err != nil {
		recordErr := s.recordObjectCleanupFailure(ctx, deleted, err)
		if recordErr != nil {
			return result, fmt.Errorf("record drive object cleanup failure after cleanup error %v: %w", err, recordErr)
		}
		return result, fmt.Errorf("cleanup permanently deleted drive objects: %w", err)
	}
	return result, nil
}

func (s *Service) RetryObjectCleanupFailures(ctx context.Context, req ListObjectCleanupFailuresRequest) (RetryObjectCleanupFailuresResult, error) {
	if s == nil || s.cleanupFailureStore == nil {
		return RetryObjectCleanupFailuresResult{}, fmt.Errorf("drive cleanup failure store is required")
	}
	req.Status = ObjectCleanupFailureStatusPending
	failures, err := s.cleanupFailureStore.ListObjectCleanupFailures(ctx, req)
	if err != nil {
		return RetryObjectCleanupFailuresResult{}, err
	}
	result := RetryObjectCleanupFailuresResult{Scanned: len(failures)}
	for _, failure := range failures {
		cleanup, err := CleanupDeletedObjects(ctx, s.stores, []DeletedObject{{
			StorageBackend: failure.StorageBackend,
			StoragePath:    failure.StoragePath,
		}})
		result.Deleted += cleanup.Deleted
		if err != nil {
			result.Failed++
			if recordErr := s.recordObjectCleanupFailure(ctx, PermanentDeleteResult{Root: Node{ID: failure.NodeID, UserID: failure.UserID}}, err); recordErr != nil {
				return result, fmt.Errorf("record drive object cleanup retry failure after cleanup error %v: %w", err, recordErr)
			}
			continue
		}
		if _, err := s.cleanupFailureStore.ResolveObjectCleanupFailure(ctx, ResolveObjectCleanupFailureRequest{ID: failure.ID}); err != nil {
			return result, err
		}
		result.Resolved++
	}
	if result.Failed > 0 {
		return result, fmt.Errorf("retry drive object cleanup: %d failures remain", result.Failed)
	}
	return result, nil
}

func (s *Service) ListObjectCleanupFailures(ctx context.Context, req ListObjectCleanupFailuresRequest) ([]ObjectCleanupFailure, error) {
	if s == nil || s.cleanupFailureStore == nil {
		return nil, fmt.Errorf("drive cleanup failure store is required")
	}
	return s.cleanupFailureStore.ListObjectCleanupFailures(ctx, req)
}

func (s *Service) ResolveObjectCleanupFailure(ctx context.Context, req ResolveObjectCleanupFailureRequest) (ObjectCleanupFailure, error) {
	if s == nil || s.cleanupFailureStore == nil {
		return ObjectCleanupFailure{}, fmt.Errorf("drive cleanup failure store is required")
	}
	return s.cleanupFailureStore.ResolveObjectCleanupFailure(ctx, req)
}

func (s *Service) recordObjectCleanupFailure(ctx context.Context, deleted PermanentDeleteResult, cleanupErr error) error {
	if s == nil || s.cleanupFailureRecorder == nil {
		return nil
	}
	var objectErr ObjectCleanupError
	if !errors.As(cleanupErr, &objectErr) {
		return nil
	}
	_, err := s.cleanupFailureRecorder.RecordObjectCleanupFailure(ctx, ObjectCleanupFailure{
		UserID:         deleted.Root.UserID,
		NodeID:         deleted.Root.ID,
		StorageBackend: objectErr.StorageBackend,
		StoragePath:    objectErr.StoragePath,
		LastError:      objectErr.Err.Error(),
	})
	return err
}

func (s *Service) recordCopiedObjectCleanupFailure(ctx context.Context, userID string, storageBackend string, storagePath string, cleanupErr error) error {
	if s == nil || s.cleanupFailureRecorder == nil {
		return nil
	}
	_, err := s.cleanupFailureRecorder.RecordObjectCleanupFailure(ctx, ObjectCleanupFailure{
		UserID:         userID,
		StorageBackend: storageBackend,
		StoragePath:    storagePath,
		LastError:      cleanupErr.Error(),
	})
	return err
}
