package dkim

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/gogomail/gogomail/internal/delivery"
)

func TestTransformerDelegatesToSigner(t *testing.T) {
	t.Parallel()

	transformer := Transformer{Signer: fakeSigner{prefix: "DKIM-Signature: test\r\n"}}
	message, err := transformer.Transform(context.Background(), delivery.Job{
		QueuedMessage: delivery.QueuedMessage{MessageID: "msg-1"},
	}, io.NopCloser(strings.NewReader("Subject: hello\r\n\r\nbody")))
	if err != nil {
		t.Fatalf("Transform returned error: %v", err)
	}
	defer message.Close()

	got, err := io.ReadAll(message)
	if err != nil {
		t.Fatalf("ReadAll returned error: %v", err)
	}
	if !strings.HasPrefix(string(got), "DKIM-Signature: test\r\nSubject: hello") {
		t.Fatalf("signed message = %q", got)
	}
}

func TestTransformerClosesInputOnSignerError(t *testing.T) {
	t.Parallel()

	input := &trackingReadCloser{Reader: strings.NewReader("Subject: hello\r\n\r\nbody")}
	_, err := Transformer{Signer: failingSigner{}}.Transform(context.Background(), delivery.Job{
		QueuedMessage: delivery.QueuedMessage{MessageID: "msg-1"},
	}, input)
	if err == nil {
		t.Fatal("Transform accepted signer failure")
	}
	if !input.closed {
		t.Fatal("input was not closed on signer failure")
	}
}

type fakeSigner struct {
	prefix string
}

func (s fakeSigner) Sign(_ context.Context, _ delivery.Job, message io.ReadCloser) (io.ReadCloser, error) {
	return readCloser{
		Reader: io.MultiReader(strings.NewReader(s.prefix), message),
		close:  message.Close,
	}, nil
}

type failingSigner struct{}

func (failingSigner) Sign(context.Context, delivery.Job, io.ReadCloser) (io.ReadCloser, error) {
	return nil, errors.New("sign failed")
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
