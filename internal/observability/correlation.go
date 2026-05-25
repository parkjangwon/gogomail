package observability

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/gogomail/gogomail/internal/httpapi"
)

func requestIDForEvent(ctx context.Context, prefix string, parts ...string) string {
	if requestID := httpapi.RequestIDFromContext(ctx); requestID != "" {
		return requestID
	}
	hash := sha256.New()
	hash.Write([]byte(prefix))
	for _, part := range parts {
		hash.Write([]byte{0})
		hash.Write([]byte(strings.TrimSpace(part)))
	}
	return prefix + "-" + hex.EncodeToString(hash.Sum(nil))[:24]
}
