package httpapi

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/drive"
	"github.com/gogomail/gogomail/internal/ratelimit"
)

func allowDrivePublicShareRequest(w http.ResponseWriter, r *http.Request, opts DriveRouteOptions, token string, action string) bool {
	if opts.PublicShareLimiter == nil {
		return true
	}
	key := drivePublicShareRateLimitKey(r.RemoteAddr, token)
	decision, err := opts.PublicShareLimiter.Allow(r.Context(), key)
	if err != nil {
		return true
	}
	if decision.Allowed {
		return true
	}
	retryAfter := int((decision.RetryAfter + time.Second - time.Nanosecond) / time.Second)
	if retryAfter <= 0 {
		retryAfter = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
	recordDrivePublicShareAccess(r, opts, drivePublicShareAccessEventFromToken(action, "rate_limited", http.StatusTooManyRequests, token, ""))
	writeError(w, http.StatusTooManyRequests, "drive share link rate limit exceeded")
	return false
}

func drivePublicShareRateLimitKey(remoteAddr string, token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return "remote=" + ratelimit.RemoteBucket(remoteAddr) + " token_sha256=" + hex.EncodeToString(sum[:])
}

func drivePublicShareAccessEvent(action string, result string, status int, resolved drive.ResolvedShareLink, token string, rangeHeader string) DrivePublicShareAccessEvent {
	event := drivePublicShareAccessEventFromNode(action, result, status, resolved.Node, token, rangeHeader)
	event.LinkID = resolved.ShareLink.ID
	event.Permission = resolved.ShareLink.Permission
	event.TokenSuffix = resolved.ShareLink.TokenSuffix
	return event
}

func drivePublicShareAccessEventFromNode(action string, result string, status int, node drive.Node, token string, rangeHeader string) DrivePublicShareAccessEvent {
	event := drivePublicShareAccessEventFromToken(action, result, status, token, rangeHeader)
	event.CompanyID = node.CompanyID
	event.DomainID = node.DomainID
	event.UserID = node.UserID
	event.NodeID = node.ID
	return event
}

func drivePublicShareAccessEventFromToken(action string, result string, status int, token string, rangeHeader string) DrivePublicShareAccessEvent {
	return DrivePublicShareAccessEvent{
		Action:      action,
		Result:      result,
		TokenSuffix: drivePublicShareTokenSuffix(token),
		Range:       rangeHeader,
		Status:      status,
	}
}

func drivePublicShareTokenSuffix(token string) string {
	token = strings.TrimSpace(token)
	if len(token) <= 8 {
		return token
	}
	return token[len(token)-8:]
}

func recordDrivePublicShareAccess(r *http.Request, opts DriveRouteOptions, event DrivePublicShareAccessEvent) {
	if opts.PublicShareAudit == nil {
		return
	}
	event.RemoteAddr = ratelimit.RemoteBucket(r.RemoteAddr)
	event.UserAgent = r.UserAgent()
	_ = opts.PublicShareAudit.RecordDrivePublicShareAccess(r.Context(), event)
}

type driveSharedFileMetadataResponse struct {
	NodeID         string    `json:"node_id"`
	Name           string    `json:"name"`
	MIMEType       string    `json:"mime_type,omitempty"`
	Size           int64     `json:"size"`
	ChecksumSHA256 string    `json:"checksum_sha256,omitempty"`
	Permission     string    `json:"permission"`
	ExpiresAt      time.Time `json:"expires_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func driveSharedFileMetadata(resolved drive.ResolvedShareLink) driveSharedFileMetadataResponse {
	return driveSharedFileMetadataResponse{
		NodeID:         resolved.Node.ID,
		Name:           resolved.Node.Name,
		MIMEType:       resolved.Node.MIMEType,
		Size:           resolved.Node.Size,
		ChecksumSHA256: safeSHA256Header(resolved.Node.ChecksumSHA256),
		Permission:     resolved.ShareLink.Permission,
		ExpiresAt:      resolved.ShareLink.ExpiresAt,
		UpdatedAt:      resolved.Node.UpdatedAt,
	}
}

func parseDriveShareTokenPathValue(w http.ResponseWriter, r *http.Request) (string, bool) {
	value := r.PathValue("id")
	if value == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return "", false
	}
	if value != strings.TrimSpace(value) || strings.ContainsAny(value, "\r\n\t ") {
		writeError(w, http.StatusBadRequest, "id must not contain whitespace")
		return "", false
	}
	if len(value) > drive.MaxShareLinkTokenBytes {
		writeError(w, http.StatusBadRequest, "id is too long")
		return "", false
	}
	for _, r := range value {
		if r < 0x21 || r > 0x7e {
			writeError(w, http.StatusBadRequest, "id must contain only printable ASCII")
			return "", false
		}
	}
	return value, true
}

func writeDriveShareLinkError(w http.ResponseWriter, err error) {
	writeError(w, driveShareLinkErrorStatus(err), err.Error())
}

func driveSharePasswordFromRequest(w http.ResponseWriter, r *http.Request) (string, bool) {
	contentType := strings.ToLower(strings.TrimSpace(strings.Split(r.Header.Get("Content-Type"), ";")[0]))
	if contentType == "application/json" || contentType == "" {
		var req struct {
			Password string `json:"password"`
		}
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return "", false
		}
		return req.Password, true
	}
	if contentType == "application/x-www-form-urlencoded" {
		r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)
		if err := r.ParseForm(); err != nil {
			writeError(w, http.StatusBadRequest, "invalid form body")
			return "", false
		}
		return r.PostFormValue("password"), true
	}
	writeError(w, http.StatusUnsupportedMediaType, "unsupported content type")
	return "", false
}

func driveShareLinkErrorStatus(err error) int {
	if errors.Is(err, drive.ErrShareLinkPermissionDenied) {
		return http.StatusForbidden
	}
	if errors.Is(err, drive.ErrShareLinkPasswordRequired) || errors.Is(err, drive.ErrShareLinkPasswordInvalid) {
		return http.StatusUnauthorized
	}
	if strings.Contains(err.Error(), "not found") {
		return http.StatusNotFound
	}
	return http.StatusBadRequest
}

