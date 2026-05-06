package storage

import (
	"context"
	"fmt"
	"strings"
)

type DeletePrefixOptions struct {
	Prefix string
	Limit  int
	Cursor string
}

type DeletePrefixResult struct {
	Deleted    int
	NextCursor string
	HasMore    bool
}

type DeletePrefixUnsafeObjectError struct {
	ObjectPath string
	Err        error
}

type DeletePrefixOutOfScopeObjectError struct {
	Prefix     string
	ObjectPath string
}

func (e DeletePrefixUnsafeObjectError) Error() string {
	return fmt.Sprintf("storage prefix listing returned unsafe object path %q: %v", e.ObjectPath, e.Err)
}

func (e DeletePrefixUnsafeObjectError) Unwrap() error {
	return e.Err
}

func (e DeletePrefixOutOfScopeObjectError) Error() string {
	return fmt.Sprintf("storage prefix listing returned object path %q outside requested prefix %q", e.ObjectPath, e.Prefix)
}

func DeletePrefix(ctx context.Context, store Store, opts DeletePrefixOptions) (DeletePrefixResult, error) {
	if err := ctx.Err(); err != nil {
		return DeletePrefixResult{}, err
	}
	if store == nil {
		return DeletePrefixResult{}, fmt.Errorf("storage store is required")
	}
	prefix, err := ValidateObjectPrefix(opts.Prefix)
	if err != nil {
		return DeletePrefixResult{}, fmt.Errorf("unsafe storage prefix %q: %w", opts.Prefix, err)
	}
	if prefix == "" {
		return DeletePrefixResult{}, fmt.Errorf("storage prefix is required")
	}
	cursor, err := ValidateListCursor(opts.Cursor)
	if err != nil {
		return DeletePrefixResult{}, err
	}

	page, err := store.List(ctx, ListOptions{
		Prefix: prefix,
		Limit:  opts.Limit,
		Cursor: cursor,
	})
	if err != nil {
		return DeletePrefixResult{}, err
	}
	result := DeletePrefixResult{
		NextCursor: page.NextCursor,
		HasMore:    page.HasMore,
	}
	if page.HasMore && page.NextCursor == "" {
		return result, fmt.Errorf("storage prefix listing is truncated without a continuation cursor")
	}
	for _, object := range page.Objects {
		objectPath, err := ValidateObjectPath(object.Path)
		if err != nil {
			return result, DeletePrefixUnsafeObjectError{ObjectPath: object.Path, Err: err}
		}
		if !objectPathMatchesPrefix(objectPath, prefix) {
			return result, DeletePrefixOutOfScopeObjectError{Prefix: prefix, ObjectPath: objectPath}
		}
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if err := store.Delete(ctx, objectPath); err != nil {
			return result, fmt.Errorf("delete storage prefix object %q: %w", objectPath, err)
		}
		result.Deleted++
	}
	return result, nil
}

func objectPathMatchesPrefix(objectPath string, prefix string) bool {
	if prefix == "" {
		return false
	}
	return objectPath == prefix || strings.HasPrefix(objectPath, prefix+"/")
}
