package httpapi

import (
	"context"
	"errors"
	"testing"

	"github.com/gogomail/gogomail/internal/drive"
)

type fakeDriveServiceForWebDAV struct {
	listNodesFunc       func(ctx context.Context, req drive.ListNodesRequest) ([]drive.Node, error)
	getNodeFunc         func(ctx context.Context, req drive.GetNodeRequest) (drive.Node, error)
	openFileFunc        func(ctx context.Context, req drive.OpenFileRequest) (drive.FileDownload, error)
	createFolderFunc    func(ctx context.Context, req drive.CreateFolderRequest) (drive.Node, error)
	createFileFunc      func(ctx context.Context, req drive.CreateFileRequest) (drive.Node, error)
	trashNodeFunc       func(ctx context.Context, req drive.TrashNodeRequest) (drive.Node, int64, error)
	renameNodeFunc      func(ctx context.Context, req drive.RenameNodeRequest) (drive.Node, error)
	moveNodeFunc        func(ctx context.Context, req drive.MoveNodeRequest) (drive.Node, error)
	copyNodeFunc        func(ctx context.Context, req drive.CopyNodeRequest) (drive.Node, error)
	getUsageSummaryFunc func(ctx context.Context, req drive.GetUsageSummaryRequest) (drive.UsageSummary, error)
}

func (f *fakeDriveServiceForWebDAV) ListNodes(ctx context.Context, req drive.ListNodesRequest) ([]drive.Node, error) {
	if f.listNodesFunc != nil {
		return f.listNodesFunc(ctx, req)
	}
	return nil, errors.New("not implemented")
}

func (f *fakeDriveServiceForWebDAV) GetNode(ctx context.Context, req drive.GetNodeRequest) (drive.Node, error) {
	if f.getNodeFunc != nil {
		return f.getNodeFunc(ctx, req)
	}
	return drive.Node{}, errors.New("not implemented")
}

func (f *fakeDriveServiceForWebDAV) OpenFile(ctx context.Context, req drive.OpenFileRequest) (drive.FileDownload, error) {
	if f.openFileFunc != nil {
		return f.openFileFunc(ctx, req)
	}
	return drive.FileDownload{}, errors.New("not implemented")
}

func (f *fakeDriveServiceForWebDAV) CreateFolder(ctx context.Context, req drive.CreateFolderRequest) (drive.Node, error) {
	if f.createFolderFunc != nil {
		return f.createFolderFunc(ctx, req)
	}
	return drive.Node{}, errors.New("not implemented")
}

func (f *fakeDriveServiceForWebDAV) CreateFile(ctx context.Context, req drive.CreateFileRequest) (drive.Node, error) {
	if f.createFileFunc != nil {
		return f.createFileFunc(ctx, req)
	}
	return drive.Node{}, errors.New("not implemented")
}

func (f *fakeDriveServiceForWebDAV) TrashNode(ctx context.Context, req drive.TrashNodeRequest) (drive.Node, int64, error) {
	if f.trashNodeFunc != nil {
		return f.trashNodeFunc(ctx, req)
	}
	return drive.Node{}, 0, errors.New("not implemented")
}

func (f *fakeDriveServiceForWebDAV) RenameNode(ctx context.Context, req drive.RenameNodeRequest) (drive.Node, error) {
	if f.renameNodeFunc != nil {
		return f.renameNodeFunc(ctx, req)
	}
	return drive.Node{}, errors.New("not implemented")
}

func (f *fakeDriveServiceForWebDAV) MoveNode(ctx context.Context, req drive.MoveNodeRequest) (drive.Node, error) {
	if f.moveNodeFunc != nil {
		return f.moveNodeFunc(ctx, req)
	}
	return drive.Node{}, errors.New("not implemented")
}

func (f *fakeDriveServiceForWebDAV) CopyNode(ctx context.Context, req drive.CopyNodeRequest) (drive.Node, error) {
	if f.copyNodeFunc != nil {
		return f.copyNodeFunc(ctx, req)
	}
	return drive.Node{}, errors.New("not implemented")
}

func (f *fakeDriveServiceForWebDAV) GetUsageSummary(ctx context.Context, req drive.GetUsageSummaryRequest) (drive.UsageSummary, error) {
	if f.getUsageSummaryFunc != nil {
		return f.getUsageSummaryFunc(ctx, req)
	}
	return drive.UsageSummary{}, errors.New("not implemented")
}

func TestWebDAVServiceAdapterImplementsInterface(t *testing.T) {
	t.Parallel()

	fake := &fakeDriveServiceForWebDAV{}
	var _ WebDAVService = NewWebDAVService(fake)
}

func TestWebDAVServiceAdapterDelegatesListNodes(t *testing.T) {
	t.Parallel()

	called := false
	fake := &fakeDriveServiceForWebDAV{
		listNodesFunc: func(ctx context.Context, req drive.ListNodesRequest) ([]drive.Node, error) {
			called = true
			return []drive.Node{{ID: "node-1"}}, nil
		},
	}
	adapter := NewWebDAVService(fake)

	nodes, err := adapter.ListNodes(context.Background(), drive.ListNodesRequest{UserID: "user-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("ListNodes was not delegated")
	}
	if len(nodes) != 1 || nodes[0].ID != "node-1" {
		t.Fatalf("unexpected nodes: %v", nodes)
	}
}

func TestWebDAVServiceAdapterAdaptsTrashNode(t *testing.T) {
	t.Parallel()

	fake := &fakeDriveServiceForWebDAV{
		trashNodeFunc: func(ctx context.Context, req drive.TrashNodeRequest) (drive.Node, int64, error) {
			return drive.Node{ID: req.NodeID}, 1024, nil
		},
	}
	adapter := NewWebDAVService(fake)

	err := adapter.TrashNode(context.Background(), drive.TrashNodeRequest{UserID: "user-1", NodeID: "node-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWebDAVServiceAdapterAdaptsTrashNodeError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("trash failed")
	fake := &fakeDriveServiceForWebDAV{
		trashNodeFunc: func(ctx context.Context, req drive.TrashNodeRequest) (drive.Node, int64, error) {
			return drive.Node{}, 0, wantErr
		},
	}
	adapter := NewWebDAVService(fake)

	err := adapter.TrashNode(context.Background(), drive.TrashNodeRequest{UserID: "user-1", NodeID: "node-1"})
	if err != wantErr {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
}

func TestWebDAVServiceAdapterLockNodeStub(t *testing.T) {
	t.Parallel()

	fake := &fakeDriveServiceForWebDAV{}
	adapter := NewWebDAVService(fake)

	token, err := adapter.LockNode(context.Background(), drive.LockNodeRequest{UserID: "user-1", NodeID: "node-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token.Token != "" {
		t.Fatalf("expected empty stub token, got %q", token.Token)
	}
}

func TestWebDAVServiceAdapterUnlockNodeStub(t *testing.T) {
	t.Parallel()

	fake := &fakeDriveServiceForWebDAV{}
	adapter := NewWebDAVService(fake)

	err := adapter.UnlockNode(context.Background(), drive.UnlockNodeRequest{UserID: "user-1", NodeID: "node-1", LockToken: "token-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}