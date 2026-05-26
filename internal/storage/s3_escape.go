package storage

import (
	"encoding/hex"
	"strings"
)

func escapeS3Key(key string) string {
	segments := strings.Split(key, "/")
	for i, segment := range segments {
		segments[i] = escapeS3Segment(segment)
	}
	return strings.Join(segments, "/")
}

func escapeS3BasePath(basePath string) string {
	basePath = strings.Trim(basePath, "/")
	if basePath == "" {
		return ""
	}
	return "/" + escapeS3Key(basePath)
}

func escapeS3QueryComponent(value string) string {
	var b strings.Builder
	for i := 0; i < len(value); i++ {
		c := value[i]
		if (c >= 'A' && c <= 'Z') ||
			(c >= 'a' && c <= 'z') ||
			(c >= '0' && c <= '9') ||
			c == '-' || c == '.' || c == '_' || c == '~' {
			b.WriteByte(c)
			continue
		}
		b.WriteByte('%')
		b.WriteString(strings.ToUpper(hex.EncodeToString([]byte{c})))
	}
	return b.String()
}

// escapeS3Segment percent-encodes a single path segment for use in an S3
// canonical URI. AWS SigV4 requires ALL characters except the unreserved set
// (A-Z a-z 0-9 - _ . ~) to be %XX encoded — this is stricter than Go's
// url.PathEscape which leaves @, =, :, !, $, &, (, ), *, +, ,, ; unencoded.
// Using url.PathEscape would cause SignatureDoesNotMatch when object keys
// contain those characters (e.g. "@" in user@example.com paths).
func escapeS3Segment(segment string) string {
	var b strings.Builder
	for i := 0; i < len(segment); i++ {
		c := segment[i]
		if (c >= 'A' && c <= 'Z') ||
			(c >= 'a' && c <= 'z') ||
			(c >= '0' && c <= '9') ||
			c == '-' || c == '.' || c == '_' || c == '~' {
			b.WriteByte(c)
			continue
		}
		b.WriteByte('%')
		b.WriteString(strings.ToUpper(hex.EncodeToString([]byte{c})))
	}
	return b.String()
}
