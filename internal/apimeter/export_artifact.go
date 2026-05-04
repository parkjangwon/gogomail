package apimeter

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

const ExportArtifactContentTypeNDJSON = "application/x-ndjson"

type ExportArtifactStore interface {
	Put(ctx context.Context, path string, body io.Reader) error
}

type ExportArtifactWriteRequest struct {
	ObjectKey string
	Metadata  json.RawMessage
	Encode    func(io.Writer) error
}

type ExportArtifactWriteResult struct {
	ObjectKey   string
	ContentType string
	ByteCount   int64
	SHA256Hex   string
	Metadata    json.RawMessage
}

func WriteExportArtifact(ctx context.Context, store ExportArtifactStore, req ExportArtifactWriteRequest) (ExportArtifactWriteResult, error) {
	if store == nil {
		return ExportArtifactWriteResult{}, fmt.Errorf("export artifact store is required")
	}
	if req.Encode == nil {
		return ExportArtifactWriteResult{}, fmt.Errorf("export artifact encoder is required")
	}
	objectKey, err := normalizeExportArtifactObjectKey(req.ObjectKey)
	if err != nil {
		return ExportArtifactWriteResult{}, err
	}
	metadata, err := normalizeExportArtifactMetadata(req.Metadata)
	if err != nil {
		return ExportArtifactWriteResult{}, err
	}

	reader, writer := io.Pipe()
	hash := sha256.New()
	counter := &countingWriter{}
	encodeErr := make(chan error, 1)
	go func() {
		target := io.MultiWriter(writer, hash, counter)
		err := req.Encode(target)
		if err != nil {
			_ = writer.CloseWithError(err)
			encodeErr <- err
			return
		}
		encodeErr <- writer.Close()
	}()

	putErr := store.Put(ctx, objectKey, reader)
	closeErr := reader.Close()
	err = <-encodeErr
	if putErr != nil {
		return ExportArtifactWriteResult{}, fmt.Errorf("write export artifact object: %w", putErr)
	}
	if closeErr != nil {
		return ExportArtifactWriteResult{}, fmt.Errorf("close export artifact stream: %w", closeErr)
	}
	if err != nil {
		return ExportArtifactWriteResult{}, fmt.Errorf("encode export artifact object: %w", err)
	}

	return ExportArtifactWriteResult{
		ObjectKey:   objectKey,
		ContentType: ExportArtifactContentTypeNDJSON,
		ByteCount:   counter.n,
		SHA256Hex:   hex.EncodeToString(hash.Sum(nil)),
		Metadata:    metadata,
	}, nil
}

func EncodeNDJSON[T any](w io.Writer, records []T) error {
	encoder := json.NewEncoder(w)
	for _, record := range records {
		if err := encoder.Encode(record); err != nil {
			return fmt.Errorf("encode ndjson record: %w", err)
		}
	}
	return nil
}

func normalizeExportArtifactObjectKey(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("object_key is required")
	}
	if strings.ContainsAny(value, "\r\n") {
		return "", fmt.Errorf("object_key cannot contain line breaks")
	}
	cleaned := filepath.ToSlash(filepath.Clean(value))
	if cleaned == "." || strings.HasPrefix(cleaned, "../") || cleaned == ".." || strings.HasPrefix(cleaned, "/") {
		return "", fmt.Errorf("object_key must be a relative storage path")
	}
	return cleaned, nil
}

func normalizeExportArtifactMetadata(value json.RawMessage) (json.RawMessage, error) {
	if len(value) == 0 {
		return json.RawMessage(`{}`), nil
	}
	var metadata map[string]any
	if err := json.Unmarshal(value, &metadata); err != nil {
		return nil, fmt.Errorf("metadata must be a JSON object: %w", err)
	}
	return value, nil
}

type countingWriter struct {
	n int64
}

func (w *countingWriter) Write(p []byte) (int, error) {
	w.n += int64(len(p))
	return len(p), nil
}
