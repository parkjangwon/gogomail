package delivery

import (
	"context"
	"fmt"
	"io"
)

type MessageTransformer interface {
	Transform(ctx context.Context, job Job, message io.ReadCloser) (io.ReadCloser, error)
}

type TransformChain []MessageTransformer

func (c TransformChain) Transform(ctx context.Context, job Job, message io.ReadCloser) (io.ReadCloser, error) {
	current := message
	for _, transformer := range c {
		if transformer == nil {
			continue
		}
		next, err := transformer.Transform(ctx, job, current)
		if err != nil {
			_ = current.Close()
			return nil, fmt.Errorf("transform message %s: %w", job.MessageID, err)
		}
		current = next
	}
	return current, nil
}
