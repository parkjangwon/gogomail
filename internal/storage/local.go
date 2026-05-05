package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type LocalStore struct {
	root string
}

func NewLocalStore(root string) *LocalStore {
	return &LocalStore{root: filepath.Clean(root)}
}

func (s *LocalStore) Put(ctx context.Context, path string, body io.Reader) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if body == nil {
		return fmt.Errorf("storage body is required")
	}

	fullPath, err := s.safePath(path)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return fmt.Errorf("create storage directory: %w", err)
	}

	file, err := os.CreateTemp(filepath.Dir(fullPath), "."+filepath.Base(fullPath)+".*.tmp")
	if err != nil {
		return fmt.Errorf("open temporary storage object: %w", err)
	}
	tmpPath := file.Name()
	committed := false
	defer func() {
		if !committed {
			_ = os.Remove(tmpPath)
		}
	}()

	_, copyErr := io.Copy(file, contextReader{ctx: ctx, reader: body})
	closeErr := file.Close()
	if copyErr != nil {
		return fmt.Errorf("write storage object: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close storage object: %w", closeErr)
	}
	if err := os.Chmod(tmpPath, 0o644); err != nil {
		return fmt.Errorf("set storage object permissions: %w", err)
	}

	if err := os.Rename(tmpPath, fullPath); err != nil {
		return fmt.Errorf("commit storage object: %w", err)
	}
	committed = true
	return nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

func (s *LocalStore) Get(ctx context.Context, path string) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	fullPath, err := s.safePath(path)
	if err != nil {
		return nil, err
	}

	file, err := os.Open(fullPath)
	if err != nil {
		return nil, fmt.Errorf("open storage object: %w", err)
	}
	return file, nil
}

func (s *LocalStore) Stat(ctx context.Context, path string) (ObjectInfo, error) {
	if err := ctx.Err(); err != nil {
		return ObjectInfo{}, err
	}

	objectPath, err := ValidateObjectPath(path)
	if err != nil {
		return ObjectInfo{}, fmt.Errorf("unsafe storage path %q: %w", path, err)
	}
	fullPath, err := s.safePath(objectPath)
	if err != nil {
		return ObjectInfo{}, err
	}
	info, err := os.Stat(fullPath)
	if err != nil {
		return ObjectInfo{}, fmt.Errorf("stat storage object: %w", err)
	}
	if info.IsDir() {
		return ObjectInfo{}, fmt.Errorf("storage object is a directory")
	}
	return ObjectInfo{
		Path:         objectPath,
		Size:         info.Size(),
		LastModified: info.ModTime(),
	}, nil
}

func (s *LocalStore) Delete(ctx context.Context, path string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	fullPath, err := s.safePath(path)
	if err != nil {
		return err
	}

	if err := os.Remove(fullPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("delete storage object: %w", err)
	}
	return nil
}

func (s *LocalStore) Check(ctx context.Context) error {
	objectPath := "health/readiness-" + strconv.FormatInt(time.Now().UnixNano(), 10) + ".txt"
	const body = "gogomail storage readiness\n"
	if err := s.Put(ctx, objectPath, strings.NewReader(body)); err != nil {
		return fmt.Errorf("write readiness probe: %w", err)
	}
	readCloser, err := s.Get(ctx, objectPath)
	if err != nil {
		_ = s.Delete(ctx, objectPath)
		return fmt.Errorf("read readiness probe: %w", err)
	}
	got, readErr := readStorageCheckBody(readCloser, len(body))
	closeErr := readCloser.Close()
	if readErr != nil {
		_ = s.Delete(ctx, objectPath)
		return fmt.Errorf("read readiness probe body: %w", readErr)
	}
	if closeErr != nil {
		_ = s.Delete(ctx, objectPath)
		return fmt.Errorf("close readiness probe body: %w", closeErr)
	}
	if string(got) != body {
		_ = s.Delete(ctx, objectPath)
		return fmt.Errorf("readiness probe body mismatch")
	}
	if err := s.Delete(ctx, objectPath); err != nil {
		return fmt.Errorf("delete readiness probe: %w", err)
	}
	return nil
}

func (s *LocalStore) safePath(path string) (string, error) {
	objectPath, err := ValidateObjectPath(path)
	if err != nil {
		return "", fmt.Errorf("unsafe storage path %q: %w", path, err)
	}

	fullPath := filepath.Join(s.root, filepath.FromSlash(objectPath))
	rel, err := filepath.Rel(s.root, fullPath)
	if err != nil {
		return "", fmt.Errorf("resolve storage path: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe storage path %q", path)
	}
	return fullPath, nil
}
