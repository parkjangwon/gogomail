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

type Store interface {
	Put(ctx context.Context, path string, body io.Reader) error
	Get(ctx context.Context, path string) (io.ReadCloser, error)
	Stat(ctx context.Context, path string) (ObjectInfo, error)
	Delete(ctx context.Context, path string) error
}
