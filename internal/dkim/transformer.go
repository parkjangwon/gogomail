package dkim

import (
	"context"
	"fmt"
	"io"

	"github.com/gogomail/gogomail/internal/delivery"
)

type Signer interface {
	Sign(ctx context.Context, job delivery.Job, message io.ReadCloser) (io.ReadCloser, error)
}

type Transformer struct {
	Signer Signer
}

func (t Transformer) Transform(ctx context.Context, job delivery.Job, message io.ReadCloser) (io.ReadCloser, error) {
	if t.Signer == nil {
		return message, nil
	}
	signed, err := t.Signer.Sign(ctx, job, message)
	if err != nil {
		_ = message.Close()
		return nil, fmt.Errorf("dkim sign message %s: %w", job.MessageID, err)
	}
	if signed == nil {
		_ = message.Close()
		return nil, fmt.Errorf("dkim signer returned nil message for %s", job.MessageID)
	}
	return signed, nil
}
