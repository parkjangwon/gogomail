package storage

import (
	"context"
	"io"
	"time"
)

type ObjectInfo struct {
	Path         string
	Size         int64
	ContentType  string
	ETag         string
	LastModified time.Time
}

type ListOptions struct {
	Prefix string
	Limit  int
	Cursor string
}

type ObjectListPage struct {
	Objects    []ObjectInfo
	NextCursor string
	HasMore    bool
}

type RangeRequest struct {
	Offset int64
	Length int64
}

type Store interface {
	Put(ctx context.Context, path string, body io.Reader) error
	Get(ctx context.Context, path string) (io.ReadCloser, error)
	GetRange(ctx context.Context, path string, req RangeRequest) (io.ReadCloser, error)
	Stat(ctx context.Context, path string) (ObjectInfo, error)
	Copy(ctx context.Context, sourcePath string, destPath string) error
	Move(ctx context.Context, sourcePath string, destPath string) error
	List(ctx context.Context, opts ListOptions) (ObjectListPage, error)
	Delete(ctx context.Context, path string) error
}

type contextReadCloser struct {
	ctx    context.Context
	closer io.ReadCloser
}

func (r *contextReadCloser) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := r.closer.Read(p)
	if ctxErr := r.ctx.Err(); ctxErr != nil {
		return n, ctxErr
	}
	return n, err
}

func (r *contextReadCloser) Close() error {
	return r.closer.Close()
}
