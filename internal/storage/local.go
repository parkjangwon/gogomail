package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type LocalStore struct {
	root   string
	rename func(oldPath, newPath string) error
}

func NewLocalStore(root string) *LocalStore {
	return &LocalStore{root: filepath.Clean(root), rename: os.Rename}
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

	if err := s.ensureObjectParentDir(path); err != nil {
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
	if err := s.rejectSymlinkParentComponents(path); err != nil {
		return nil, err
	}
	if _, err := localObjectInfo(fullPath); err != nil {
		return nil, fmt.Errorf("open storage object: %w", err)
	}

	file, err := os.Open(fullPath)
	if err != nil {
		return nil, fmt.Errorf("open storage object: %w", err)
	}
	return &contextReadCloser{ctx: ctx, closer: file}, nil
}

func (s *LocalStore) GetRange(ctx context.Context, path string, req RangeRequest) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	validated, err := ValidateRangeRequest(req)
	if err != nil {
		return nil, err
	}

	fullPath, err := s.safePath(path)
	if err != nil {
		return nil, err
	}
	if err := s.rejectSymlinkParentComponents(path); err != nil {
		return nil, err
	}
	if _, err := localObjectInfo(fullPath); err != nil {
		return nil, fmt.Errorf("open storage object range: %w", err)
	}

	file, err := os.Open(fullPath)
	if err != nil {
		return nil, fmt.Errorf("open storage object range: %w", err)
	}
	if _, err := file.Seek(validated.Offset, io.SeekStart); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("seek storage object range: %w", err)
	}
	return &limitedReadCloser{ctx: ctx, reader: file, closer: file, remaining: validated.Length}, nil
}

type limitedReadCloser struct {
	ctx       context.Context
	reader    io.Reader
	closer    io.Closer
	remaining int64
}

func (r *limitedReadCloser) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	if r.remaining <= 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > r.remaining {
		p = p[:r.remaining]
	}
	n, err := r.reader.Read(p)
	r.remaining -= int64(n)
	if ctxErr := r.ctx.Err(); ctxErr != nil {
		return n, ctxErr
	}
	if err == io.EOF && r.remaining > 0 {
		return n, io.ErrUnexpectedEOF
	}
	if err == io.EOF && n > 0 && r.remaining == 0 {
		return n, nil
	}
	return n, err
}

func (r *limitedReadCloser) Close() error {
	return r.closer.Close()
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
	if err := s.rejectSymlinkParentComponents(objectPath); err != nil {
		return ObjectInfo{}, err
	}
	info, err := localObjectInfo(fullPath)
	if err != nil {
		return ObjectInfo{}, fmt.Errorf("stat storage object: %w", err)
	}
	return ObjectInfo{
		Path:         objectPath,
		Size:         info.Size(),
		LastModified: info.ModTime(),
	}, nil
}

func (s *LocalStore) Copy(ctx context.Context, sourcePath string, destPath string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	sourceObjectPath, err := ValidateObjectPath(sourcePath)
	if err != nil {
		return fmt.Errorf("unsafe source storage path %q: %w", sourcePath, err)
	}
	destObjectPath, err := ValidateObjectPath(destPath)
	if err != nil {
		return fmt.Errorf("unsafe destination storage path %q: %w", destPath, err)
	}
	if sourceObjectPath == destObjectPath {
		return nil
	}

	source, err := s.Get(ctx, sourceObjectPath)
	if err != nil {
		return fmt.Errorf("open source storage object: %w", err)
	}
	defer source.Close()
	if err := s.Put(ctx, destObjectPath, source); err != nil {
		return fmt.Errorf("copy storage object: %w", err)
	}
	return nil
}

func (s *LocalStore) Move(ctx context.Context, sourcePath string, destPath string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	sourceObjectPath, err := ValidateObjectPath(sourcePath)
	if err != nil {
		return fmt.Errorf("unsafe source storage path %q: %w", sourcePath, err)
	}
	destObjectPath, err := ValidateObjectPath(destPath)
	if err != nil {
		return fmt.Errorf("unsafe destination storage path %q: %w", destPath, err)
	}
	if sourceObjectPath == destObjectPath {
		return nil
	}

	sourceFullPath, err := s.safePath(sourceObjectPath)
	if err != nil {
		return err
	}
	if err := s.rejectSymlinkParentComponents(sourceObjectPath); err != nil {
		return err
	}
	if _, err := localObjectInfo(sourceFullPath); err != nil {
		return fmt.Errorf("move source storage object: %w", err)
	}
	destFullPath, err := s.safePath(destObjectPath)
	if err != nil {
		return err
	}
	if err := s.ensureObjectParentDir(destObjectPath); err != nil {
		return fmt.Errorf("create destination storage directory: %w", err)
	}
	rename := s.rename
	if rename == nil {
		rename = os.Rename
	}
	if err := rename(sourceFullPath, destFullPath); err != nil {
		if errors.Is(err, syscall.EXDEV) {
			if err := s.Copy(ctx, sourceObjectPath, destObjectPath); err != nil {
				return fmt.Errorf("move storage object: rename fallback copy failed: %w", err)
			}
			if err := s.Delete(ctx, sourceObjectPath); err != nil {
				return fmt.Errorf("move storage object: rename fallback delete failed: %w", err)
			}
			return nil
		}
		return fmt.Errorf("move storage object: %w", err)
	}
	return nil
}

func (s *LocalStore) List(ctx context.Context, opts ListOptions) (ObjectListPage, error) {
	if err := ctx.Err(); err != nil {
		return ObjectListPage{}, err
	}
	prefix, err := ValidateObjectPrefix(opts.Prefix)
	if err != nil {
		return ObjectListPage{}, fmt.Errorf("unsafe storage prefix %q: %w", opts.Prefix, err)
	}
	cursor, err := ValidateListCursor(opts.Cursor)
	if err != nil {
		return ObjectListPage{}, err
	}
	limit := NormalizeListLimit(opts.Limit)

	root := s.root
	if prefix != "" {
		root = filepath.Join(s.root, filepath.FromSlash(prefix))
	}
	if prefix != "" {
		if err := s.rejectSymlinkPathComponents(prefix); err != nil {
			if os.IsNotExist(err) {
				return ObjectListPage{Objects: []ObjectInfo{}}, nil
			}
			return ObjectListPage{}, fmt.Errorf("stat storage prefix: %w", err)
		}
	}
	if info, err := os.Lstat(root); err != nil {
		if os.IsNotExist(err) {
			return ObjectListPage{Objects: []ObjectInfo{}}, nil
		}
		return ObjectListPage{}, fmt.Errorf("stat storage prefix: %w", err)
	} else if info.Mode()&os.ModeSymlink != 0 {
		return ObjectListPage{}, fmt.Errorf("stat storage prefix: storage path component is a symbolic link")
	} else if !info.IsDir() {
		return ObjectListPage{}, fmt.Errorf("stat storage prefix: storage prefix is not a directory")
	}

	page := ObjectListPage{Objects: make([]ObjectInfo, 0, limit)}
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		base := filepath.Base(path)
		if strings.HasPrefix(base, ".") && strings.HasSuffix(base, ".tmp") {
			return nil
		}
		rel, err := filepath.Rel(s.root, path)
		if err != nil {
			return fmt.Errorf("resolve storage object: %w", err)
		}
		objectPath, err := ValidateObjectPath(filepath.ToSlash(rel))
		if err != nil {
			return fmt.Errorf("unsafe listed storage path %q: %w", rel, err)
		}
		if cursor != "" && objectPath <= cursor {
			return nil
		}
		if len(page.Objects) >= limit {
			page.HasMore = true
			return filepath.SkipAll
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("stat listed storage object: %w", err)
		}
		page.Objects = append(page.Objects, ObjectInfo{
			Path:         objectPath,
			Size:         info.Size(),
			LastModified: info.ModTime(),
		})
		page.NextCursor = objectPath
		return nil
	})
	if err != nil {
		return ObjectListPage{}, fmt.Errorf("list storage objects: %w", err)
	}
	if !page.HasMore {
		page.NextCursor = ""
	}
	return page, nil
}

func (s *LocalStore) Delete(ctx context.Context, path string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	fullPath, err := s.safePath(path)
	if err != nil {
		return err
	}
	if err := s.rejectSymlinkParentComponents(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	if _, err := localObjectInfo(fullPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("delete storage object: %w", err)
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
	info, err := s.Stat(ctx, objectPath)
	if err != nil {
		_ = s.Delete(ctx, objectPath)
		return fmt.Errorf("stat readiness probe: %w", err)
	}
	if info.Path != objectPath || info.Size != int64(len(body)) {
		_ = s.Delete(ctx, objectPath)
		return fmt.Errorf("readiness probe metadata mismatch")
	}
	rangeCloser, err := s.GetRange(ctx, objectPath, RangeRequest{Offset: 0, Length: int64(len("gogomail"))})
	if err != nil {
		_ = s.Delete(ctx, objectPath)
		return fmt.Errorf("range readiness probe: %w", err)
	}
	rangeGot, rangeReadErr := readStorageCheckBody(rangeCloser, len("gogomail"))
	rangeCloseErr := rangeCloser.Close()
	if rangeReadErr != nil {
		_ = s.Delete(ctx, objectPath)
		return fmt.Errorf("read range readiness probe body: %w", rangeReadErr)
	}
	if rangeCloseErr != nil {
		_ = s.Delete(ctx, objectPath)
		return fmt.Errorf("close range readiness probe body: %w", rangeCloseErr)
	}
	if string(rangeGot) != "gogomail" {
		_ = s.Delete(ctx, objectPath)
		return fmt.Errorf("readiness probe range body mismatch")
	}
	if err := s.Delete(ctx, objectPath); err != nil {
		return fmt.Errorf("delete readiness probe: %w", err)
	}
	return nil
}

func localObjectInfo(path string) (os.FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("storage object is a symbolic link")
	}
	if info.IsDir() {
		return nil, fmt.Errorf("storage object is a directory")
	}
	return info, nil
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

func (s *LocalStore) ensureObjectParentDir(objectPath string) error {
	objectPath, err := ValidateObjectPath(objectPath)
	if err != nil {
		return fmt.Errorf("unsafe storage path %q: %w", objectPath, err)
	}
	segments := strings.Split(objectPath, "/")
	current := s.root
	for _, segment := range segments[:len(segments)-1] {
		current = filepath.Join(current, segment)
		info, err := os.Lstat(current)
		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("storage path component is a symbolic link")
			}
			if !info.IsDir() {
				return fmt.Errorf("storage path component is not a directory")
			}
			continue
		}
		if !os.IsNotExist(err) {
			return err
		}
		if err := os.Mkdir(current, 0o755); err != nil {
			if !os.IsExist(err) {
				return err
			}
		}
		info, err = os.Lstat(current)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("storage path component is a symbolic link")
		}
		if !info.IsDir() {
			return fmt.Errorf("storage path component is not a directory")
		}
	}
	return nil
}

func (s *LocalStore) rejectSymlinkPathComponents(objectPath string) error {
	objectPath, err := ValidateObjectPath(objectPath)
	if err != nil {
		return fmt.Errorf("unsafe storage path %q: %w", objectPath, err)
	}
	return s.rejectSymlinkComponents(strings.Split(objectPath, "/"))
}

func (s *LocalStore) rejectSymlinkParentComponents(objectPath string) error {
	objectPath, err := ValidateObjectPath(objectPath)
	if err != nil {
		return fmt.Errorf("unsafe storage path %q: %w", objectPath, err)
	}
	segments := strings.Split(objectPath, "/")
	if len(segments) <= 1 {
		return nil
	}
	return s.rejectSymlinkComponents(segments[:len(segments)-1])
}

func (s *LocalStore) rejectSymlinkComponents(segments []string) error {
	current := s.root
	for _, segment := range segments {
		current = filepath.Join(current, segment)
		info, err := os.Lstat(current)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("storage path component is a symbolic link")
		}
	}
	return nil
}
