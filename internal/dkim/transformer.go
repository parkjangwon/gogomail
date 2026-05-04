package dkim

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io"
	"os"
	"strings"

	dkimlib "github.com/emersion/go-msgauth/dkim"
	"github.com/gogomail/gogomail/internal/delivery"
)

type Signer interface {
	Sign(ctx context.Context, job delivery.Job, message io.ReadCloser) (io.ReadCloser, error)
}

type Key struct {
	Domain        string
	Selector      string
	PrivateKeyPEM string
}

type KeyProvider interface {
	DKIMKey(ctx context.Context, job delivery.Job) (Key, error)
}

type RFC6376Signer struct {
	KeyProvider KeyProvider
	TempDir     string
	HeaderKeys  []string
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

func (s RFC6376Signer) Sign(ctx context.Context, job delivery.Job, message io.ReadCloser) (io.ReadCloser, error) {
	if s.KeyProvider == nil {
		return message, nil
	}
	key, err := s.KeyProvider.DKIMKey(ctx, job)
	if err != nil {
		_ = message.Close()
		return nil, err
	}
	cryptoSigner, err := parsePrivateKeyPEM(key.PrivateKeyPEM)
	if err != nil {
		_ = message.Close()
		return nil, err
	}

	domain := strings.TrimSpace(key.Domain)
	if domain == "" {
		domain = domainFromAddress(job.From.Email)
	}
	options := &dkimlib.SignOptions{
		Domain:                 domain,
		Selector:               strings.TrimSpace(key.Selector),
		Identifier:             strings.TrimSpace(job.From.Email),
		Signer:                 cryptoSigner,
		Hash:                   crypto.SHA256,
		HeaderCanonicalization: dkimlib.CanonicalizationRelaxed,
		BodyCanonicalization:   dkimlib.CanonicalizationRelaxed,
		HeaderKeys:             headerKeysOrDefault(s.HeaderKeys),
	}

	dkimSigner, err := dkimlib.NewSigner(options)
	if err != nil {
		_ = message.Close()
		return nil, err
	}

	tmp, err := os.CreateTemp(s.TempDir, "gogomail-dkim-*.eml")
	if err != nil {
		_ = dkimSigner.Close()
		_ = message.Close()
		return nil, err
	}
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
	}

	_, copyErr := io.Copy(io.MultiWriter(tmp, dkimSigner), message)
	closeMessageErr := message.Close()
	if copyErr != nil {
		_ = dkimSigner.Close()
		cleanup()
		return nil, copyErr
	}
	if closeMessageErr != nil {
		_ = dkimSigner.Close()
		cleanup()
		return nil, closeMessageErr
	}
	if err := dkimSigner.Close(); err != nil {
		cleanup()
		return nil, err
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		cleanup()
		return nil, err
	}

	return signedMessage{
		Reader: io.MultiReader(strings.NewReader(dkimSigner.Signature()), tmp),
		file:   tmp,
		path:   tmp.Name(),
	}, nil
}

func parsePrivateKeyPEM(raw string) (crypto.Signer, error) {
	block, _ := pem.Decode([]byte(strings.TrimSpace(raw)))
	if block == nil {
		return nil, fmt.Errorf("dkim private key PEM is invalid")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse dkim private key: %w", err)
	}
	switch key := parsed.(type) {
	case *rsa.PrivateKey:
		return key, nil
	case crypto.Signer:
		return key, nil
	default:
		return nil, fmt.Errorf("unsupported dkim private key type %T", parsed)
	}
}

func PublicKeyDNSFromPrivateKeyPEM(raw string) (string, error) {
	signer, err := parsePrivateKeyPEM(raw)
	if err != nil {
		return "", err
	}
	encoded, err := x509.MarshalPKIXPublicKey(signer.Public())
	if err != nil {
		return "", fmt.Errorf("marshal dkim public key: %w", err)
	}
	return "v=DKIM1; k=rsa; p=" + base64.StdEncoding.EncodeToString(encoded), nil
}

func headerKeysOrDefault(keys []string) []string {
	if len(keys) > 0 {
		return append([]string(nil), keys...)
	}
	return []string{
		"From",
		"To",
		"Cc",
		"Subject",
		"Date",
		"Message-ID",
		"MIME-Version",
		"Content-Type",
		"Content-Transfer-Encoding",
	}
}

func domainFromAddress(address string) string {
	_, domain, ok := strings.Cut(strings.TrimSpace(address), "@")
	if !ok || domain == "" {
		return "localhost"
	}
	return strings.ToLower(domain)
}

type signedMessage struct {
	io.Reader
	file *os.File
	path string
}

func (m signedMessage) Close() error {
	err := m.file.Close()
	removeErr := os.Remove(m.path)
	if err != nil {
		return err
	}
	return removeErr
}
