package delivery

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestTransformChainAppliesTransformersInOrder(t *testing.T) {
	t.Parallel()

	chain := TransformChain{
		prefixTransformer{prefix: "A:"},
		prefixTransformer{prefix: "B:"},
	}
	message, err := chain.Transform(context.Background(), Job{QueuedMessage: QueuedMessage{MessageID: "msg-1"}}, io.NopCloser(strings.NewReader("body")))
	if err != nil {
		t.Fatalf("Transform returned error: %v", err)
	}
	defer message.Close()

	got, err := io.ReadAll(message)
	if err != nil {
		t.Fatalf("ReadAll returned error: %v", err)
	}
	if string(got) != "B:A:body" {
		t.Fatalf("transformed message = %q", got)
	}
}

func TestTransformChainClosesInputOnFailure(t *testing.T) {
	t.Parallel()

	input := &trackingReadCloser{Reader: strings.NewReader("body")}
	_, err := TransformChain{failingTransformer{}}.Transform(context.Background(), Job{QueuedMessage: QueuedMessage{MessageID: "msg-1"}}, input)
	if err == nil {
		t.Fatal("Transform accepted failing transformer")
	}
	if !input.closed {
		t.Fatal("input was not closed after transform failure")
	}
}

func TestDirectSMTPTransportOpenMessageAppliesTransformers(t *testing.T) {
	t.Parallel()

	transport := &DirectSMTPTransport{
		Transformers: TransformChain{prefixTransformer{prefix: "DKIM-Signature: test\r\n"}},
	}
	message, err := transport.openMessage(context.Background(), Job{
		QueuedMessage: QueuedMessage{MessageID: "msg-1"},
		OpenMessage: func(context.Context) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("Subject: hello\r\n\r\nbody")), nil
		},
	})
	if err != nil {
		t.Fatalf("openMessage returned error: %v", err)
	}
	defer message.Close()

	got, err := io.ReadAll(message)
	if err != nil {
		t.Fatalf("ReadAll returned error: %v", err)
	}
	if !strings.HasPrefix(string(got), "DKIM-Signature: test\r\nSubject: hello") {
		t.Fatalf("transformed message = %q", got)
	}
}

type prefixTransformer struct {
	prefix string
}

func (t prefixTransformer) Transform(_ context.Context, _ Job, message io.ReadCloser) (io.ReadCloser, error) {
	return readCloser{
		Reader: io.MultiReader(strings.NewReader(t.prefix), message),
		close:  message.Close,
	}, nil
}

type failingTransformer struct{}

func (failingTransformer) Transform(context.Context, Job, io.ReadCloser) (io.ReadCloser, error) {
	return nil, errors.New("transform failed")
}

type trackingReadCloser struct {
	io.Reader
	closed bool
}

func (r *trackingReadCloser) Close() error {
	r.closed = true
	return nil
}

type readCloser struct {
	io.Reader
	close func() error
}

func (r readCloser) Close() error {
	if r.close == nil {
		return nil
	}
	return r.close()
}
