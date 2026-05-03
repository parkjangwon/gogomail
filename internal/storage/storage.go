package storage

import (
	"context"
	"io"
)

type Store interface {
	Put(ctx context.Context, path string, body io.Reader) error
	Get(ctx context.Context, path string) (io.ReadCloser, error)
	Delete(ctx context.Context, path string) error
}
