package httpapi

import (
	"context"

	"github.com/gogomail/gogomail/internal/drive"
)

type driveServiceForWebDAV interface {
	ListNodes(ctx context.Context, req drive.ListNodesRequest) ([]drive.Node, error)
	GetNode(ctx context.Context, req drive.GetNodeRequest) (drive.Node, error)
	OpenFile(ctx context.Context, req drive.OpenFileRequest) (drive.FileDownload, error)
	CreateFolder(ctx context.Context, req drive.CreateFolderRequest) (drive.Node, error)
	TrashNode(ctx context.Context, req drive.TrashNodeRequest) (drive.Node, int64, error)
	RenameNode(ctx context.Context, req drive.RenameNodeRequest) (drive.Node, error)
	MoveNode(ctx context.Context, req drive.MoveNodeRequest) (drive.Node, error)
	CopyNode(ctx context.Context, req drive.CopyNodeRequest) (drive.Node, error)
	GetUsageSummary(ctx context.Context, req drive.GetUsageSummaryRequest) (drive.UsageSummary, error)
}

type WebDAVServiceAdapter struct {
	svc driveServiceForWebDAV
}

func NewWebDAVService(svc driveServiceForWebDAV) *WebDAVServiceAdapter {
	return &WebDAVServiceAdapter{svc: svc}
}

func (a *WebDAVServiceAdapter) ListNodes(ctx context.Context, req drive.ListNodesRequest) ([]drive.Node, error) {
	return a.svc.ListNodes(ctx, req)
}

func (a *WebDAVServiceAdapter) GetNode(ctx context.Context, req drive.GetNodeRequest) (drive.Node, error) {
	return a.svc.GetNode(ctx, req)
}

func (a *WebDAVServiceAdapter) OpenFile(ctx context.Context, req drive.OpenFileRequest) (drive.FileDownload, error) {
	return a.svc.OpenFile(ctx, req)
}

func (a *WebDAVServiceAdapter) CreateFolder(ctx context.Context, req drive.CreateFolderRequest) (drive.Node, error) {
	return a.svc.CreateFolder(ctx, req)
}

func (a *WebDAVServiceAdapter) TrashNode(ctx context.Context, req drive.TrashNodeRequest) error {
	_, _, err := a.svc.TrashNode(ctx, req)
	return err
}

func (a *WebDAVServiceAdapter) RenameNode(ctx context.Context, req drive.RenameNodeRequest) (drive.Node, error) {
	return a.svc.RenameNode(ctx, req)
}

func (a *WebDAVServiceAdapter) MoveNode(ctx context.Context, req drive.MoveNodeRequest) (drive.Node, error) {
	return a.svc.MoveNode(ctx, req)
}

func (a *WebDAVServiceAdapter) CopyNode(ctx context.Context, req drive.CopyNodeRequest) (drive.Node, error) {
	return a.svc.CopyNode(ctx, req)
}

func (a *WebDAVServiceAdapter) GetUsageSummary(ctx context.Context, req drive.GetUsageSummaryRequest) (drive.UsageSummary, error) {
	return a.svc.GetUsageSummary(ctx, req)
}

func (a *WebDAVServiceAdapter) LockNode(ctx context.Context, req drive.LockNodeRequest) (drive.LockToken, error) {
	return drive.LockToken{}, nil
}

func (a *WebDAVServiceAdapter) UnlockNode(ctx context.Context, req drive.UnlockNodeRequest) error {
	return nil
}