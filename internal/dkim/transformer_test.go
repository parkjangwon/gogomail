package dkim

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"io"
	"strings"
	"testing"

	dkimlib "github.com/emersion/go-msgauth/dkim"
	"github.com/gogomail/gogomail/internal/delivery"
	"github.com/gogomail/gogomail/internal/outbound"
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

func TestRFC6376SignerSignsAndVerifiesMessage(t *testing.T) {
	t.Parallel()

	privateKey, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("GenerateKey returned error: %v", err)
	}
	privatePEM := string(pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	}))

	signer := RFC6376Signer{
		TempDir: t.TempDir(),
		KeyProvider: staticKeyProvider{key: Key{
			Domain:        "example.com",
			Selector:      "s1",
			PrivateKeyPEM: privatePEM,
		}},
	}
	raw := strings.Join([]string{
		"From: Sender <sender@example.com>",
		"To: User <user@example.net>",
		"Subject: hello",
		"Date: Sun, 03 May 2026 09:00:00 +0000",
		"Message-ID: <signed@example.com>",
		"",
		"body",
	}, "\r\n")

	signed, err := signer.Sign(context.Background(), delivery.Job{
		QueuedMessage: delivery.QueuedMessage{
			MessageID: "msg-1",
			From:      outbound.Address{Email: "sender@example.com"},
		},
	}, io.NopCloser(strings.NewReader(raw)))
	if err != nil {
		t.Fatalf("Sign returned error: %v", err)
	}
	defer signed.Close()

	signedRaw, err := io.ReadAll(signed)
	if err != nil {
		t.Fatalf("ReadAll returned error: %v", err)
	}
	if !strings.HasPrefix(string(signedRaw), "DKIM-Signature:") {
		t.Fatalf("signed message missing DKIM-Signature: %q", signedRaw)
	}

	publicKeyRaw, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("MarshalPKIXPublicKey returned error: %v", err)
	}
	verifications, err := dkimlib.VerifyWithOptions(strings.NewReader(string(signedRaw)), &dkimlib.VerifyOptions{
		LookupTXT: func(domain string) ([]string, error) {
			if domain != "s1._domainkey.example.com" {
				t.Fatalf("LookupTXT domain = %q", domain)
			}
			return []string{"v=DKIM1; k=rsa; p=" + base64.StdEncoding.EncodeToString(publicKeyRaw)}, nil
		},
	})
	if err != nil {
		t.Fatalf("VerifyWithOptions returned error: %v", err)
	}
	if len(verifications) != 1 || verifications[0].Err != nil {
		t.Fatalf("verifications = %+v", verifications)
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

type staticKeyProvider struct {
	key Key
}

func (p staticKeyProvider) DKIMKey(context.Context, delivery.Job) (Key, error) {
	return p.key, nil
}

func (r readCloser) Close() error {
	if r.close == nil {
		return nil
	}
	return r.close()
}
