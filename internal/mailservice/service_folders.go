package mailservice

import (
	"context"
	"fmt"
	"strings"

	"github.com/gogomail/gogomail/internal/maildb"
)

func (s *Service) ListFolders(ctx context.Context, userID string) ([]maildb.Folder, error) {
	userID = strings.TrimSpace(userID)
	if err := validateServiceResourceID("user_id", userID); err != nil {
		return nil, err
	}
	return s.repository.ListFolders(ctx, userID)
}

func (s *Service) CreateFolder(ctx context.Context, req maildb.CreateFolderRequest) (maildb.Folder, error) {
	req.UserID = strings.TrimSpace(req.UserID)
	req.Name = strings.TrimSpace(req.Name)
	if err := validateServiceResourceID("user_id", req.UserID); err != nil {
		return maildb.Folder{}, err
	}
	if err := validateServiceResourceID("folder_name", req.Name); err != nil {
		return maildb.Folder{}, err
	}
	return s.repository.CreateFolder(ctx, req)
}

func (s *Service) RenameFolder(ctx context.Context, userID string, folderID string, name string) (maildb.Folder, error) {
	userID = strings.TrimSpace(userID)
	folderID = strings.TrimSpace(folderID)
	name = strings.TrimSpace(name)
	if err := validateServiceResourceID("user_id", userID); err != nil {
		return maildb.Folder{}, err
	}
	if err := validateServiceResourceID("folder_id", folderID); err != nil {
		return maildb.Folder{}, err
	}
	if err := validateServiceResourceID("folder_name", name); err != nil {
		return maildb.Folder{}, err
	}
	return s.repository.RenameFolder(ctx, userID, folderID, name)
}

func (s *Service) DeleteFolder(ctx context.Context, userID string, folderID string) error {
	userID = strings.TrimSpace(userID)
	folderID = strings.TrimSpace(folderID)
	if err := validateServiceResourceID("user_id", userID); err != nil {
		return fmt.Errorf("delete folder: %w", err)
	}
	if err := validateServiceResourceID("folder_id", folderID); err != nil {
		return fmt.Errorf("delete folder: %w", err)
	}
	return s.repository.DeleteFolder(ctx, userID, folderID)
}