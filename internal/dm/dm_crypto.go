package dm

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

const (
	RoomKeyBytes   = 32
	MasterKeyBytes = 32
)

type Crypto struct {
	masterKey []byte
	rand      io.Reader
}

func NewCrypto(masterKey []byte) (*Crypto, error) {
	if len(masterKey) != MasterKeyBytes {
		return nil, fmt.Errorf("dm master key must be %d bytes", MasterKeyBytes)
	}
	key := append([]byte(nil), masterKey...)
	return &Crypto{masterKey: key, rand: rand.Reader}, nil
}

func ParseMasterKey(value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, fmt.Errorf("GOGOMAIL_DM_MASTER_KEY is required")
	}
	if decoded, err := base64.StdEncoding.DecodeString(value); err == nil && len(decoded) == MasterKeyBytes {
		return decoded, nil
	}
	if decoded, err := base64.RawStdEncoding.DecodeString(value); err == nil && len(decoded) == MasterKeyBytes {
		return decoded, nil
	}
	if decoded, err := base64.RawURLEncoding.DecodeString(value); err == nil && len(decoded) == MasterKeyBytes {
		return decoded, nil
	}
	if decoded, err := hex.DecodeString(value); err == nil && len(decoded) == MasterKeyBytes {
		return decoded, nil
	}
	if len([]byte(value)) == MasterKeyBytes {
		return []byte(value), nil
	}
	return nil, fmt.Errorf("GOGOMAIL_DM_MASTER_KEY must decode to %d bytes", MasterKeyBytes)
}

func (c *Crypto) GenerateRoomKey() ([]byte, error) {
	key := make([]byte, RoomKeyBytes)
	if _, err := io.ReadFull(c.random(), key); err != nil {
		return nil, fmt.Errorf("generate dm room key: %w", err)
	}
	return key, nil
}

func (c *Crypto) WrapRoomKey(roomKey []byte) ([]byte, error) {
	if len(roomKey) != RoomKeyBytes {
		return nil, fmt.Errorf("dm room key must be %d bytes", RoomKeyBytes)
	}
	return c.seal(c.masterKey, roomKey)
}

func (c *Crypto) UnwrapRoomKey(ciphertext []byte) ([]byte, error) {
	return openGCM(c.masterKey, ciphertext)
}

func (c *Crypto) EncryptBody(roomKey []byte, plaintext []byte) ([]byte, error) {
	if len(roomKey) != RoomKeyBytes {
		return nil, fmt.Errorf("dm room key must be %d bytes", RoomKeyBytes)
	}
	return c.seal(roomKey, plaintext)
}

func (c *Crypto) DecryptBody(roomKey []byte, ciphertext []byte) ([]byte, error) {
	if len(roomKey) != RoomKeyBytes {
		return nil, fmt.Errorf("dm room key must be %d bytes", RoomKeyBytes)
	}
	return openGCM(roomKey, ciphertext)
}

func (c *Crypto) SignAttachmentToken(messageID string, expiresAt time.Time) (string, error) {
	messageID = strings.TrimSpace(messageID)
	if messageID == "" {
		return "", fmt.Errorf("%w: message_id is required", ErrInvalid)
	}
	exp := expiresAt.UTC().Unix()
	payload := fmt.Sprintf("%s.%d", messageID, exp)
	mac := hmac.New(sha256.New, c.masterKey)
	_, _ = mac.Write([]byte("dm-attachment-download:" + payload))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + signature, nil
}

func (c *Crypto) VerifyAttachmentToken(token string, now time.Time) (string, error) {
	token = strings.TrimSpace(token)
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return "", fmt.Errorf("%w: invalid attachment token", ErrInvalid)
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", fmt.Errorf("%w: invalid attachment token", ErrInvalid)
	}
	payload := string(payloadBytes)
	dot := strings.LastIndexByte(payload, '.')
	if dot <= 0 || dot == len(payload)-1 {
		return "", fmt.Errorf("%w: invalid attachment token", ErrInvalid)
	}
	messageID := strings.TrimSpace(payload[:dot])
	exp, err := strconv.ParseInt(payload[dot+1:], 10, 64)
	if err != nil || exp <= 0 {
		return "", fmt.Errorf("%w: invalid attachment token", ErrInvalid)
	}
	if now.UTC().Unix() > exp {
		return "", fmt.Errorf("%w: attachment token expired", ErrForbidden)
	}
	mac := hmac.New(sha256.New, c.masterKey)
	_, _ = mac.Write([]byte("dm-attachment-download:" + payload))
	want := mac.Sum(nil)
	got, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !hmac.Equal(got, want) {
		return "", fmt.Errorf("%w: invalid attachment token", ErrForbidden)
	}
	return messageID, nil
}

func (c *Crypto) seal(key []byte, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(c.random(), nonce); err != nil {
		return nil, fmt.Errorf("generate dm nonce: %w", err)
	}
	out := make([]byte, 0, len(nonce)+len(plaintext)+gcm.Overhead())
	out = append(out, nonce...)
	out = gcm.Seal(out, nonce, plaintext, nil)
	return out, nil
}

func openGCM(key []byte, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(ciphertext) < gcm.NonceSize()+gcm.Overhead() {
		return nil, fmt.Errorf("dm ciphertext is too short")
	}
	nonce := ciphertext[:gcm.NonceSize()]
	payload := ciphertext[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, payload, nil)
	if err != nil {
		return nil, fmt.Errorf("dm decrypt: %w", err)
	}
	return plaintext, nil
}

func (c *Crypto) random() io.Reader {
	if c != nil && c.rand != nil {
		return c.rand
	}
	return rand.Reader
}
