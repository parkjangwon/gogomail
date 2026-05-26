package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	netmail "net/mail"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gogomail/gogomail/internal/apikeys"
	"github.com/gogomail/gogomail/internal/auth"
	"github.com/gogomail/gogomail/internal/mail"
	"github.com/gogomail/gogomail/internal/maildb"
	"github.com/gogomail/gogomail/internal/mailservice"
)

func parseQueryLimit(w http.ResponseWriter, r *http.Request) (int, bool) {
	raw, ok := singleQueryValue(w, r, "limit")
	if !ok {
		return 0, false
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return maildb.MessageListDefaultLimit, true
	}
	if len(raw) > maxHTTPControlBytes {
		writeError(w, http.StatusBadRequest, "limit is too long")
		return 0, false
	}
	limit, err := strconv.Atoi(raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "limit must be an integer")
		return 0, false
	}
	if limit <= 0 {
		writeError(w, http.StatusBadRequest, "limit must be positive")
		return 0, false
	}
	if limit > maildb.MessageListMaxLimit {
		writeError(w, http.StatusBadRequest, "limit must be at most 200")
		return 0, false
	}
	return limit, true
}

func parseOptionalBoolQuery(w http.ResponseWriter, r *http.Request, key string) (*bool, bool) {
	raw, ok := singleQueryValue(w, r, key)
	if !ok {
		return nil, false
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, true
	}
	if strings.ContainsAny(raw, "\r\n") {
		writeError(w, http.StatusBadRequest, key+" must not contain CR or LF")
		return nil, false
	}
	if len(raw) > maxHTTPControlBytes {
		writeError(w, http.StatusBadRequest, key+" is too long")
		return nil, false
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, key+" must be a boolean")
		return nil, false
	}
	return &value, true
}

func singleQueryValue(w http.ResponseWriter, r *http.Request, key string) (string, bool) {
	values, ok := r.URL.Query()[key]
	if !ok || len(values) == 0 {
		return "", true
	}
	if len(values) > 1 {
		writeError(w, http.StatusBadRequest, key+" must not be repeated")
		return "", false
	}
	return values[0], true
}

func rejectUnknownQueryKeys(w http.ResponseWriter, r *http.Request, allowed ...string) bool {
	if len(r.URL.Query()) == 0 {
		return true
	}
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, key := range allowed {
		allowedSet[key] = struct{}{}
	}
	for key := range r.URL.Query() {
		if _, ok := allowedSet[key]; !ok {
			writeError(w, http.StatusBadRequest, key+" is not supported")
			return false
		}
	}
	return true
}

func parseBoolQueryDefaultFalse(w http.ResponseWriter, r *http.Request, key string) (bool, bool) {
	value, ok := parseOptionalBoolQuery(w, r, key)
	if !ok {
		return false, false
	}
	if value == nil {
		return false, true
	}
	return *value, true
}

func decodeJSONBody(r *http.Request, dst any) error {
	if err := requireJSONContentType(r); err != nil {
		return err
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, maxJSONBodyBytes+1))
	if err != nil {
		return err
	}
	if len(raw) > maxJSONBodyBytes {
		return errors.New("json body too large")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("body must contain a single JSON value")
		}
		return err
	}
	return nil
}

func requireJSONContentType(r *http.Request) error {
	values := r.Header.Values("Content-Type")
	if len(values) > 1 {
		return errors.New("content-type must not be repeated")
	}
	contentType := ""
	if len(values) == 1 {
		contentType = strings.TrimSpace(values[0])
	}
	if contentType == "" {
		return errors.New("content-type must be application/json")
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return errors.New("content-type must be application/json")
	}
	if !strings.EqualFold(mediaType, "application/json") {
		return errors.New("content-type must be application/json")
	}
	return nil
}

func parseBoundedHTTPPathValue(w http.ResponseWriter, r *http.Request, key string) (string, bool) {
	value := strings.TrimSpace(r.PathValue(key))
	if value == "" {
		writeError(w, http.StatusBadRequest, key+" is required")
		return "", false
	}
	if strings.ContainsAny(value, "\r\n") {
		writeError(w, http.StatusBadRequest, key+" must not contain CR or LF")
		return "", false
	}
	if len(value) > maxHTTPResourceIDBytes {
		writeError(w, http.StatusBadRequest, key+" is too long")
		return "", false
	}
	return value, true
}

func parseBoundedHTTPPathPair(w http.ResponseWriter, r *http.Request, firstKey string, secondKey string) (string, string, bool) {
	first, ok := parseBoundedHTTPPathValue(w, r, firstKey)
	if !ok {
		return "", "", false
	}
	second, ok := parseBoundedHTTPPathValue(w, r, secondKey)
	if !ok {
		return "", "", false
	}
	return first, second, true
}

func parseContentRange(hdr string) (*mailservice.ContentRange, error) {
	hdr = strings.TrimSpace(hdr)
	if !strings.HasPrefix(hdr, "bytes ") {
		return nil, fmt.Errorf("content-range must start with 'bytes'")
	}
	rest := strings.TrimPrefix(hdr, "bytes ")
	parts := strings.Split(rest, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("content-range must be 'bytes first-last/total'")
	}
	rangePart := strings.TrimSpace(parts[0])
	totalPart := strings.TrimSpace(parts[1])
	rangeBounds := strings.Split(rangePart, "-")
	if len(rangeBounds) != 2 {
		return nil, fmt.Errorf("content-range range must be 'first-last'")
	}
	first, err := strconv.ParseInt(rangeBounds[0], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("content-range first-byte is invalid")
	}
	last, err := strconv.ParseInt(rangeBounds[1], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("content-range last-byte is invalid")
	}
	total, err := strconv.ParseInt(totalPart, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("content-range total-size is invalid")
	}
	return &mailservice.ContentRange{
		FirstByte: first,
		LastByte:  last,
		TotalSize: total,
	}, nil
}

func parseBoundedHTTPQuery(w http.ResponseWriter, r *http.Request, key string, required bool, maxBytes int) (string, bool) {
	value, ok := singleQueryValue(w, r, key)
	if !ok {
		return "", false
	}
	value = strings.TrimSpace(value)
	return parseBoundedHTTPValue(w, key, value, required, maxBytes)
}

func parseBoundedHTTPFormValue(w http.ResponseWriter, r *http.Request, key string, required bool, maxBytes int) (string, bool) {
	value, ok := singleHTTPMultipartFormValue(w, r, key)
	if !ok {
		return "", false
	}
	value = strings.TrimSpace(value)
	return parseBoundedHTTPValue(w, key, value, required, maxBytes)
}

func singleHTTPMultipartFormValue(w http.ResponseWriter, r *http.Request, key string) (string, bool) {
	if r.MultipartForm == nil || r.MultipartForm.Value == nil {
		return "", true
	}
	values := r.MultipartForm.Value[key]
	if len(values) == 0 {
		return "", true
	}
	if len(values) > 1 {
		writeError(w, http.StatusBadRequest, key+" must not be repeated")
		return "", false
	}
	return values[0], true
}

func singleHTTPMultipartFile(w http.ResponseWriter, r *http.Request, key string) (multipart.File, *multipart.FileHeader, bool) {
	if r.MultipartForm == nil || r.MultipartForm.File == nil {
		return nil, nil, true
	}
	files := r.MultipartForm.File[key]
	if len(files) == 0 {
		return nil, nil, true
	}
	if len(files) > 1 {
		writeError(w, http.StatusBadRequest, key+" must not be repeated")
		return nil, nil, false
	}
	file, err := files[0].Open()
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid multipart attachment upload")
		return nil, nil, false
	}
	return file, files[0], true
}

func parseBoundedHTTPValue(w http.ResponseWriter, key string, value string, required bool, maxBytes int) (string, bool) {
	if value == "" {
		if required {
			writeError(w, http.StatusBadRequest, key+" is required")
			return "", false
		}
		return "", true
	}
	if strings.ContainsAny(value, "\r\n") {
		writeError(w, http.StatusBadRequest, key+" must not contain CR or LF")
		return "", false
	}
	if len(value) > maxBytes {
		writeError(w, http.StatusBadRequest, key+" is too long")
		return "", false
	}
	return value, true
}

func userIDFromRequest(w http.ResponseWriter, r *http.Request, tokenManager *auth.TokenManager, services ...MessageService) (string, bool) {
	var service MessageService
	if len(services) > 0 {
		service = services[0]
	}
	if info, ok := apikeys.KeyInfoFromContext(r.Context()); ok && info != nil {
		if strings.TrimSpace(info.UserID) != "" {
			return userScopedAPIKeyUserIDFromRequest(w, r, info, "", "")
		}
		if service != nil {
			return apiKeyUserIDFromRequest(w, r, service, info, "", "")
		}
	}
	if tokenManager == nil {
		return parseBoundedHTTPQuery(w, r, "user_id", true, maxHTTPResourceIDBytes)
	}
	claims, ok := claimsFromRequest(w, r, tokenManager)
	if !ok {
		return "", false
	}
	return claims.UserID, true
}

func sessionUserIDFromRequest(w http.ResponseWriter, r *http.Request, tokenManager *auth.TokenManager) (string, bool) {
	if tokenManager == nil {
		writeError(w, http.StatusServiceUnavailable, "session authentication is required")
		return "", false
	}
	claims, ok := claimsFromRequest(w, r, tokenManager)
	if !ok {
		return "", false
	}
	return claims.UserID, true
}

func bindRequestUserID(w http.ResponseWriter, r *http.Request, tokenManager *auth.TokenManager, service MessageService, dst *string, bodyEmails ...string) bool {
	if info, ok := apikeys.KeyInfoFromContext(r.Context()); ok && info != nil {
		bodyEmail := ""
		if len(bodyEmails) > 0 {
			bodyEmail = bodyEmails[0]
		}
		if strings.TrimSpace(info.UserID) != "" {
			userID, ok := userScopedAPIKeyUserIDFromRequest(w, r, info, strings.TrimSpace(*dst), strings.TrimSpace(bodyEmail))
			if !ok {
				return false
			}
			*dst = userID
			return true
		}
		userID, ok := apiKeyUserIDFromRequest(w, r, service, info, strings.TrimSpace(*dst), strings.TrimSpace(bodyEmail))
		if !ok {
			return false
		}
		*dst = userID
		return true
	}
	if tokenManager != nil {
		userID, ok := userIDFromRequest(w, r, tokenManager, service)
		if !ok {
			return false
		}
		*dst = userID
		return true
	}
	if _, ok := r.URL.Query()["user_id"]; ok {
		userID, ok := parseBoundedHTTPQuery(w, r, "user_id", true, maxHTTPResourceIDBytes)
		if !ok {
			return false
		}
		*dst = userID
		return true
	}
	userID, ok := parseBoundedHTTPValue(w, "user_id", strings.TrimSpace(*dst), true, maxHTTPResourceIDBytes)
	if !ok {
		return false
	}
	*dst = userID
	return true
}

func userScopedAPIKeyUserIDFromRequest(w http.ResponseWriter, r *http.Request, info *apikeys.KeyInfo, bodyUserID string, bodyUserEmail string) (string, bool) {
	userID := strings.TrimSpace(info.UserID)
	if userID == "" {
		writeError(w, http.StatusForbidden, "api key is not bound to a user")
		return "", false
	}
	requiredScope := requiredUserAPIKeyScope(r)
	if !apiKeyHasScope(info, requiredScope) {
		writeError(w, http.StatusForbidden, "api key scope "+requiredScope+" is required")
		return "", false
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions && !allowAPIKeyRequest(w, r, userMCPAPIKeyMutationLimiter) {
		return "", false
	}
	if !userAPIKeyConfirmationOK(w, r, info) {
		return "", false
	}
	queryUserID, ok := parseBoundedHTTPQuery(w, r, "user_id", false, maxHTTPResourceIDBytes)
	if !ok {
		return "", false
	}
	headerUserID, ok := singleHTTPHeaderValue(w, r, "X-Gogomail-User-ID", maxHTTPResourceIDBytes)
	if !ok {
		return "", false
	}
	bodyUserID, ok = parseBoundedHTTPValue(w, "user_id", bodyUserID, false, maxHTTPResourceIDBytes)
	if !ok {
		return "", false
	}
	for _, candidate := range []string{queryUserID, headerUserID, bodyUserID} {
		if strings.TrimSpace(candidate) != "" && strings.TrimSpace(candidate) != userID {
			writeError(w, http.StatusForbidden, "api key is not allowed for the requested user")
			return "", false
		}
	}
	if strings.TrimSpace(bodyUserEmail) != "" {
		if _, ok := normalizeAPIUserEmail(w, bodyUserEmail, true); !ok {
			return "", false
		}
	}
	r.Header.Set("X-Gogomail-Resolved-User-ID", userID)
	return userID, true
}

func userAPIKeyConfirmationOK(w http.ResponseWriter, r *http.Request, info *apikeys.KeyInfo) bool {
	if info == nil || strings.EqualFold(strings.TrimSpace(info.PermissionMode), maildb.MCPPermissionModeBypass) {
		return true
	}
	expected, ok := requiredUserAPIKeyConfirmation(r)
	if !ok {
		return true
	}
	got, ok := singleHTTPHeaderValue(w, r, "X-Gogomail-MCP-Confirm", maxHTTPResourceIDBytes)
	if !ok {
		return false
	}
	if strings.TrimSpace(got) != expected {
		writeError(w, http.StatusForbidden, "mcp confirmation header must equal "+expected)
		return false
	}
	return true
}

func apiKeyUserIDFromRequest(w http.ResponseWriter, r *http.Request, service MessageService, info *apikeys.KeyInfo, bodyUserID string, bodyUserEmail string) (string, bool) {
	requiredScope := requiredMailAPIKeyScope(r)
	if !apiKeyHasMailScope(info, requiredScope) {
		writeError(w, http.StatusForbidden, "api key scope "+requiredScope+" is required")
		return "", false
	}
	if service == nil {
		writeError(w, http.StatusServiceUnavailable, "mail service is not configured")
		return "", false
	}
	headerUserID, ok := singleHTTPHeaderValue(w, r, "X-Gogomail-User-ID", maxHTTPResourceIDBytes)
	if !ok {
		return "", false
	}
	headerUserEmail, ok := singleHTTPHeaderValue(w, r, "X-Gogomail-User-Email", maxHTTPUserEmailBytes)
	if !ok {
		return "", false
	}
	queryUserID, ok := parseBoundedHTTPQuery(w, r, "user_id", false, maxHTTPResourceIDBytes)
	if !ok {
		return "", false
	}
	queryUserEmail, ok := parseBoundedHTTPQuery(w, r, "user_email", false, maxHTTPUserEmailBytes)
	if !ok {
		return "", false
	}
	bodyUserID, ok = parseBoundedHTTPValue(w, "user_id", bodyUserID, false, maxHTTPResourceIDBytes)
	if !ok {
		return "", false
	}
	bodyUserEmail, ok = parseBoundedHTTPValue(w, "user_email", bodyUserEmail, false, maxHTTPUserEmailBytes)
	if !ok {
		return "", false
	}
	userID := firstNonEmpty(headerUserID, queryUserID, bodyUserID)
	userEmail := firstNonEmpty(headerUserEmail, queryUserEmail, bodyUserEmail)
	if userID == "" && userEmail == "" {
		writeError(w, http.StatusBadRequest, "user_email, X-Gogomail-User-Email, user_id, or X-Gogomail-User-ID is required for API key requests")
		return "", false
	}
	for _, candidate := range []string{headerUserID, queryUserID, bodyUserID} {
		if candidate != "" && candidate != userID {
			writeError(w, http.StatusBadRequest, "request user identifiers must match")
			return "", false
		}
	}
	normalizedEmail, ok := normalizeAPIUserEmail(w, userEmail, userEmail != "")
	if !ok {
		return "", false
	}
	for _, candidate := range []string{headerUserEmail, queryUserEmail, bodyUserEmail} {
		if candidate == "" {
			continue
		}
		normalizedCandidate, ok := normalizeAPIUserEmail(w, candidate, true)
		if !ok {
			return "", false
		}
		if normalizedCandidate != normalizedEmail {
			writeError(w, http.StatusBadRequest, "request user identifiers must match")
			return "", false
		}
	}

	var profile maildb.UserProfile
	if userID != "" {
		var err error
		profile, err = service.GetUserProfile(r.Context(), userID)
		if err != nil {
			writeError(w, http.StatusForbidden, "api key is not allowed for the requested user")
			return "", false
		}
	}
	if normalizedEmail != "" {
		emailProfile, err := service.GetUserProfileByEmail(r.Context(), normalizedEmail)
		if err != nil {
			writeError(w, http.StatusForbidden, "api key is not allowed for the requested user")
			return "", false
		}
		if profile.UserID != "" && strings.TrimSpace(profile.UserID) != strings.TrimSpace(emailProfile.UserID) {
			writeError(w, http.StatusBadRequest, "request user identifiers must match")
			return "", false
		}
		profile = emailProfile
	}
	if strings.TrimSpace(profile.UserID) == "" {
		writeError(w, http.StatusForbidden, "api key is not allowed for the requested user")
		return "", false
	}
	if strings.TrimSpace(profile.DomainID) == "" || strings.TrimSpace(profile.DomainID) != strings.TrimSpace(info.DomainID) {
		writeError(w, http.StatusForbidden, "api key is not allowed for the requested user")
		return "", false
	}
	r.Header.Set("X-Gogomail-Resolved-User-ID", profile.UserID)
	return profile.UserID, true
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func normalizeAPIUserEmail(w http.ResponseWriter, email string, required bool) (string, bool) {
	email = strings.TrimSpace(email)
	if email == "" {
		if required {
			writeError(w, http.StatusBadRequest, "user_email is required")
			return "", false
		}
		return "", true
	}
	if len(email) > maxHTTPUserEmailBytes {
		writeError(w, http.StatusBadRequest, "user_email is too long")
		return "", false
	}
	if strings.ContainsAny(email, " \t\r\n") {
		writeError(w, http.StatusBadRequest, "user_email must be a single email address")
		return "", false
	}
	addr, err := netmail.ParseAddress(email)
	if err != nil || addr.Address != email {
		writeError(w, http.StatusBadRequest, "user_email must be a valid email address")
		return "", false
	}
	local, domain, ok := strings.Cut(addr.Address, "@")
	if !ok || local == "" || domain == "" || !strings.Contains(domain, ".") {
		writeError(w, http.StatusBadRequest, "user_email must be a valid email address")
		return "", false
	}
	return strings.ToLower(addr.Address), true
}

func requiredMailAPIKeyScope(r *http.Request) string {
	if r == nil {
		return "mail:read"
	}
	if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
		return "mail:read"
	}
	pattern := strings.TrimSpace(r.Pattern)
	path := ""
	if r.URL != nil {
		path = r.URL.Path
	}
	route := pattern + " " + path
	if strings.Contains(route, "/api/v1/messages/send") || strings.Contains(route, "/api/v1/drafts/{id}/send") || strings.Contains(route, "/api/v1/drafts/") && strings.Contains(route, "/send") {
		return "mail:send"
	}
	return "mail:manage"
}

func apiKeyHasMailScope(info *apikeys.KeyInfo, required string) bool {
	return apiKeyHasScope(info, required)
}

func apiKeyHasScope(info *apikeys.KeyInfo, required string) bool {
	if info == nil {
		return false
	}
	required = normalizeMailScope(required)
	requiredFamily, requiredAction, _ := strings.Cut(required, ":")
	for _, scope := range info.Scopes {
		scope = normalizeMailScope(scope)
		scopeFamily, scopeAction, _ := strings.Cut(scope, ":")
		if scope == "*" || scope == required || scope == requiredFamily || scope == requiredFamily+":*" {
			return true
		}
		if scopeFamily != requiredFamily {
			continue
		}
		if scopeAction == "manage" {
			return true
		}
		if scopeAction == "write" && (requiredAction == "read" || requiredAction == "write") {
			return true
		}
		switch required {
		case "mail:read":
			if scope == "mail" || scope == "mail:*" || scope == "mail:read" || scope == "mail:manage" || scope == "mail:write" {
				return true
			}
		case "mail:send":
			if scope == "mail" || scope == "mail:*" || scope == "mail:send" || scope == "mail:manage" {
				return true
			}
		case "mail:manage":
			if scope == "mail" || scope == "mail:*" || scope == "mail:manage" {
				return true
			}
		}
	}
	return false
}

func requiredUserAPIKeyScope(r *http.Request) string {
	if r == nil {
		return "mail:read"
	}
	path := ""
	if r.URL != nil {
		path = r.URL.Path
	}
	if strings.Contains(path, "/api/v1/drive/") {
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			return "drive:read"
		}
		if r.Method == http.MethodDelete || strings.Contains(path, "/trash") || strings.Contains(path, "/share-links") {
			return "drive:manage"
		}
		return "drive:write"
	}
	if strings.Contains(path, "/api/v1/calendars") || strings.Contains(path, "/api/v1/calendar-") {
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			return "calendar:read"
		}
		if r.Method == http.MethodDelete {
			return "calendar:manage"
		}
		return "calendar:write"
	}
	if strings.Contains(path, "/api/mail/addressbooks") || strings.Contains(path, "/api/mail/contacts") || strings.Contains(path, "/api/mail/directory") {
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			return "contacts:read"
		}
		if r.Method == http.MethodDelete {
			return "contacts:manage"
		}
		return "contacts:write"
	}
	return requiredMailAPIKeyScope(r)
}

func requiredUserAPIKeyConfirmation(r *http.Request) (string, bool) {
	if r == nil || r.URL == nil {
		return "", false
	}
	path := r.URL.Path
	switch {
	case r.Method == http.MethodPost && path == "/api/v1/messages/send":
		return "send message", true
	case r.Method == http.MethodPost && strings.HasPrefix(path, "/api/v1/drafts/") && strings.HasSuffix(path, "/send"):
		return "send draft " + r.PathValue("id"), true
	case r.Method == http.MethodDelete && strings.HasPrefix(path, "/api/v1/drafts/"):
		return "delete draft " + r.PathValue("id"), true
	case r.Method == http.MethodDelete && strings.HasPrefix(path, "/api/v1/messages/"):
		return "delete message " + r.PathValue("id"), true
	case r.Method == http.MethodDelete && strings.HasPrefix(path, "/api/mail/addressbooks/") && strings.Contains(path, "/contacts/"):
		return "delete contact " + r.PathValue("name"), true
	case r.Method == http.MethodDelete && strings.HasPrefix(path, "/api/mail/addressbooks/"):
		return "delete addressbook " + r.PathValue("id"), true
	case r.Method == http.MethodPost && strings.HasPrefix(path, "/api/v1/drive/nodes/") && strings.HasSuffix(path, "/trash"):
		return "trash drive " + r.PathValue("id"), true
	case r.Method == http.MethodDelete && strings.HasPrefix(path, "/api/v1/drive/nodes/"):
		return "delete drive " + r.PathValue("id"), true
	case r.Method == http.MethodPost && strings.HasPrefix(path, "/api/v1/drive/nodes/") && strings.HasSuffix(path, "/share-links"):
		return "share drive " + r.PathValue("id"), true
	case r.Method == http.MethodDelete && strings.HasPrefix(path, "/api/v1/drive/share-links/"):
		return "DELETE " + path, true
	case r.Method == http.MethodDelete && strings.HasPrefix(path, "/api/v1/calendars/") && strings.Contains(path, "/objects/"):
		return "delete calendar " + r.PathValue("name"), true
	case r.Method == http.MethodDelete && strings.HasPrefix(path, "/api/v1/calendars/"):
		return "delete calendar " + r.PathValue("id"), true
	case r.Method == http.MethodDelete && strings.HasPrefix(path, "/api/v1/attachments/"):
		return "DELETE " + path, true
	case r.Method == http.MethodPost && path == "/api/v1/messages/bulk/delete":
		return "POST " + path, true
	case r.Method == http.MethodPost && path == "/api/v1/threads/bulk/delete":
		return "POST " + path, true
	case r.Method == http.MethodDelete && strings.HasPrefix(path, "/api/v1/calendar-subscriptions/"):
		return "DELETE " + path, true
	default:
		return "", false
	}
}

var defaultMCPSettingsJSON = json.RawMessage(`{"enabled":false,"permission_mode":"basic","generated_mail_notice_enabled":true,"generated_mail_notice_text":"MCP를 통해 작성된 메일입니다.","require_confirmation_for_sensitive_actions":true,"bypass_mode_allowed":false,"mail":{"send_enabled":true,"confirm_external_recipients":true,"confirm_attachments":true,"daily_send_limit":100},"scopes":{"mail":["read","write","send","delete"],"contacts":["read","write","delete"],"drive":["read","write","delete","share"],"calendar":["read","write","delete","invite"]}}`)

func extractMCPSettings(prefs json.RawMessage) json.RawMessage {
	var root map[string]json.RawMessage
	if len(prefs) > 0 && json.Unmarshal(prefs, &root) == nil {
		if raw := root["mcp"]; len(raw) > 0 && json.Valid(raw) {
			return raw
		}
	}
	return defaultMCPSettingsJSON
}

func effectiveMCPSettings(ctx context.Context, service MessageService, userID string, raw json.RawMessage, keyInfo *apikeys.KeyInfo) (json.RawMessage, error) {
	if !json.Valid(raw) {
		raw = defaultMCPSettingsJSON
	}
	var settings map[string]any
	if err := json.Unmarshal(raw, &settings); err != nil || settings == nil {
		if err := json.Unmarshal(defaultMCPSettingsJSON, &settings); err != nil {
			return nil, err
		}
	}
	policy, err := service.GetUserMCPDomainPolicy(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !policy.AllowBypassMode {
		settings["bypass_mode_allowed"] = false
		settings["permission_mode"] = maildb.MCPPermissionModeBasic
	} else if _, ok := settings["bypass_mode_allowed"].(bool); !ok {
		settings["bypass_mode_allowed"] = true
	}
	if keyInfo != nil && strings.TrimSpace(keyInfo.UserID) != "" && !strings.EqualFold(strings.TrimSpace(keyInfo.PermissionMode), maildb.MCPPermissionModeBypass) {
		settings["bypass_mode_allowed"] = false
		settings["permission_mode"] = maildb.MCPPermissionModeBasic
	}
	if policy.ForceGeneratedMailNotice {
		settings["generated_mail_notice_enabled"] = true
		settings["generated_mail_notice_forced"] = true
	} else {
		settings["generated_mail_notice_forced"] = false
	}
	out, err := json.Marshal(settings)
	if err != nil {
		return nil, err
	}
	return out, nil
}

type userMCPRuntimeSettings struct {
	GeneratedMailNoticeEnabled *bool  `json:"generated_mail_notice_enabled"`
	GeneratedMailNoticeText    string `json:"generated_mail_notice_text"`
	Mail                       struct {
		SendEnabled               *bool `json:"send_enabled"`
		ConfirmExternalRecipients *bool `json:"confirm_external_recipients"`
		ConfirmAttachments        *bool `json:"confirm_attachments"`
		DailySendLimit            *int  `json:"daily_send_limit"`
	} `json:"mail"`
}

type userMCPSendCounter struct {
	Day   string
	Count int
}

var (
	userMCPSendCountersMu sync.Mutex
	userMCPSendCounters   = map[string]userMCPSendCounter{}
)

func loadUserMCPRuntimeSettings(ctx context.Context, service MessageService, r *http.Request, userID string) (userMCPRuntimeSettings, bool, error) {
	var settings userMCPRuntimeSettings
	info, ok := apikeys.KeyInfoFromContext(r.Context())
	if !ok || info == nil || strings.TrimSpace(info.UserID) == "" {
		return settings, false, nil
	}
	prefs, err := service.GetWebmailPreferences(ctx, userID)
	if err != nil {
		return settings, true, err
	}
	raw, err := effectiveMCPSettings(ctx, service, userID, extractMCPSettings(prefs), info)
	if err != nil {
		return settings, true, err
	}
	if err := json.Unmarshal(raw, &settings); err != nil {
		return settings, true, err
	}
	return settings, true, nil
}

func userMCPGeneratedNotice(ctx context.Context, service MessageService, r *http.Request, userID string) (string, bool, error) {
	settings, ok, err := loadUserMCPRuntimeSettings(ctx, service, r, userID)
	if err != nil || !ok {
		return "", false, err
	}
	if settings.GeneratedMailNoticeEnabled != nil && !*settings.GeneratedMailNoticeEnabled {
		return "", false, nil
	}
	notice := strings.TrimSpace(settings.GeneratedMailNoticeText)
	if notice == "" {
		notice = "MCP를 통해 작성된 메일입니다."
	}
	return notice, true, nil
}

func userMCPSendPolicyContext(w http.ResponseWriter, r *http.Request, service MessageService, ctx context.Context, userID string, req *mailservice.SendTextRequest) (context.Context, bool) {
	settings, ok, err := loadUserMCPRuntimeSettings(r.Context(), service, r, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to apply mcp settings")
		return ctx, false
	}
	if !ok {
		return ctx, true
	}
	if settings.Mail.SendEnabled != nil && !*settings.Mail.SendEnabled {
		writeError(w, http.StatusForbidden, "mcp mail sending is disabled")
		return ctx, false
	}
	policy := mailservice.MCPSendPolicy{
		ConfirmExternalRecipients:   true,
		ConfirmAttachments:          true,
		ExternalRecipientsConfirmed: strings.TrimSpace(r.Header.Get("X-Gogomail-MCP-External-Confirm")) == "external recipients",
		AttachmentsConfirmed:        strings.TrimSpace(r.Header.Get("X-Gogomail-MCP-Attachment-Confirm")) == "send attachments",
	}
	if settings.Mail.ConfirmExternalRecipients != nil {
		policy.ConfirmExternalRecipients = *settings.Mail.ConfirmExternalRecipients
	}
	if settings.Mail.ConfirmAttachments != nil {
		policy.ConfirmAttachments = *settings.Mail.ConfirmAttachments
	}
	info, _ := apikeys.KeyInfoFromContext(r.Context())
	isBypass := info != nil && strings.EqualFold(strings.TrimSpace(info.PermissionMode), maildb.MCPPermissionModeBypass)
	if isBypass {
		policy.ConfirmExternalRecipients = false
		policy.ConfirmAttachments = false
	}
	if profile, err := service.GetUserProfile(r.Context(), userID); err == nil {
		policy.SenderDomain = emailDomain(profile.Email)
	}
	ctx = mailservice.ContextWithMCPSendPolicy(ctx, policy)
	if info != nil && !isBypass {
		if req != nil && strings.TrimSpace(req.From) == "" {
			if policy.SenderDomain != "" {
				reqCopy := *req
				reqCopy.From = policy.SenderDomain
				req = &reqCopy
			}
		}
		if req != nil && policy.ConfirmAttachments && len(req.AttachmentIDs) > 0 && !policy.AttachmentsConfirmed {
			writeError(w, http.StatusForbidden, "mcp attachment confirmation header must equal send attachments")
			return ctx, false
		}
		if req != nil && policy.ConfirmExternalRecipients && mailservice.EnforceMCPSendPolicy(ctx, *req) != nil && !policy.ExternalRecipientsConfirmed {
			writeError(w, http.StatusForbidden, "mcp external-recipient confirmation header must equal external recipients")
			return ctx, false
		}
	}
	dailyLimit := 100
	if settings.Mail.DailySendLimit != nil {
		dailyLimit = *settings.Mail.DailySendLimit
	}
	if dailyLimit >= 0 && userMCPSendCount(r, userID) >= dailyLimit {
		writeError(w, http.StatusForbidden, "mcp daily send limit exceeded")
		return ctx, false
	}
	return ctx, true
}

func emailDomain(address string) string {
	address = strings.TrimSpace(address)
	_, domain, ok := strings.Cut(address, "@")
	if !ok {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(domain))
}

func userMCPSendCounterKey(r *http.Request, userID string) string {
	keyID := "unknown-key"
	if info, ok := apikeys.KeyInfoFromContext(r.Context()); ok && strings.TrimSpace(info.ID) != "" {
		keyID = strings.TrimSpace(info.ID)
	}
	return keyID + ":" + strings.TrimSpace(userID)
}

func userMCPSendCount(r *http.Request, userID string) int {
	key := userMCPSendCounterKey(r, userID)
	day := time.Now().UTC().Format("2006-01-02")
	userMCPSendCountersMu.Lock()
	defer userMCPSendCountersMu.Unlock()
	counter := userMCPSendCounters[key]
	if counter.Day != day {
		return 0
	}
	return counter.Count
}

func recordUserMCPSend(r *http.Request, userID string) {
	if info, ok := apikeys.KeyInfoFromContext(r.Context()); !ok || info == nil || strings.TrimSpace(info.UserID) == "" {
		return
	}
	key := userMCPSendCounterKey(r, userID)
	day := time.Now().UTC().Format("2006-01-02")
	userMCPSendCountersMu.Lock()
	defer userMCPSendCountersMu.Unlock()
	counter := userMCPSendCounters[key]
	if counter.Day != day {
		counter = userMCPSendCounter{Day: day}
	}
	counter.Count++
	userMCPSendCounters[key] = counter
}

func mergeMCPSettings(ctx context.Context, service MessageService, userID string, mcp json.RawMessage) (json.RawMessage, error) {
	if !json.Valid(mcp) {
		return nil, fmt.Errorf("mcp settings must be valid JSON")
	}
	var obj map[string]any
	if err := json.Unmarshal(mcp, &obj); err != nil || obj == nil {
		return nil, fmt.Errorf("mcp settings must be a JSON object")
	}
	prefs, err := service.GetWebmailPreferences(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to load preferences")
	}
	var root map[string]json.RawMessage
	if len(prefs) == 0 || json.Unmarshal(prefs, &root) != nil || root == nil {
		root = map[string]json.RawMessage{}
	}
	root["mcp"] = mcp
	merged, err := json.Marshal(root)
	if err != nil {
		return nil, fmt.Errorf("failed to save mcp settings")
	}
	if err := service.SetWebmailPreferences(ctx, userID, merged); err != nil {
		return nil, fmt.Errorf("failed to save mcp settings")
	}
	return merged, nil
}

func normalizeMailScope(scope string) string {
	scope = strings.ToLower(strings.TrimSpace(scope))
	return strings.ReplaceAll(scope, ".", ":")
}

func claimsFromRequest(w http.ResponseWriter, r *http.Request, tokenManager *auth.TokenManager) (auth.Claims, bool) {
	token, ok := bearerToken(w, r)
	if !ok {
		return auth.Claims{}, false
	}
	if token == "" {
		writeError(w, http.StatusUnauthorized, "bearer token is required")
		return auth.Claims{}, false
	}
	claims, err := tokenManager.VerifyFull(r.Context(), token)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid bearer token")
		return auth.Claims{}, false
	}
	if claims.TokenType == "mfa_pending" {
		writeError(w, http.StatusUnauthorized, "mfa verification required")
		return auth.Claims{}, false
	}
	return claims, true
}

func bearerToken(w http.ResponseWriter, r *http.Request) (string, bool) {
	authHeader, ok := singleHTTPHeaderValue(w, r, "Authorization", maxHTTPAuthHeaderBytes)
	if !ok {
		return "", false
	}
	if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		return strings.TrimSpace(authHeader[len("bearer "):]), true
	}
	return "", true
}

func singleHTTPHeaderValue(w http.ResponseWriter, r *http.Request, key string, maxBytes int) (string, bool) {
	values := r.Header.Values(key)
	if len(values) == 0 {
		return "", true
	}
	if len(values) > 1 {
		writeError(w, http.StatusBadRequest, key+" must not be repeated")
		return "", false
	}
	value := strings.TrimSpace(values[0])
	if len(value) > maxBytes {
		writeError(w, http.StatusBadRequest, key+" is too long")
		return "", false
	}
	return value, true
}

func contentDispositionAttachment(filename string) string {
	utf8Name := strings.NewReplacer("\\", "_", `"`, "_", "\r", "_", "\n", "_").Replace(strings.TrimSpace(filename))
	if utf8Name == "" {
		utf8Name = "attachment"
	}
	utf8Name = truncateRunes(utf8Name, 180)
	asciiName := asciiAttachmentFilename(utf8Name)
	return `attachment; filename="` + asciiName + `"; filename*=UTF-8''` + url.PathEscape(utf8Name)
}

func writeAttachmentDownloadHeaders(w http.ResponseWriter, attachment maildb.Attachment) {
	w.Header().Set("Content-Type", attachmentContentType(attachment.MIMEType))
	w.Header().Set("Content-Disposition", contentDispositionAttachment(attachment.Filename))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if attachment.Size >= 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(attachment.Size, 10))
	}
}

func attachmentWithStatSize(attachment maildb.Attachment, size int64) maildb.Attachment {
	attachment.Size = size
	return attachment
}

func truncateRunes(value string, max int) string {
	if max <= 0 {
		return ""
	}
	for i := range value {
		if max == 0 {
			return value[:i]
		}
		max--
	}
	return value
}

func asciiAttachmentFilename(filename string) string {
	var builder strings.Builder
	for _, r := range filename {
		if r >= 0x20 && r <= 0x7e {
			builder.WriteRune(r)
			continue
		}
		builder.WriteByte('_')
	}
	if builder.Len() == 0 {
		return "attachment"
	}
	return builder.String()
}

func attachmentContentType(mimeType string) string {
	return safeContentType(mimeType, "application/octet-stream")
}

func safeContentType(mimeType string, fallback string) string {
	mimeType = strings.TrimSpace(mimeType)
	if mimeType == "" || strings.ContainsAny(mimeType, "\r\n") {
		return fallback
	}
	mediaType, _, err := mime.ParseMediaType(mimeType)
	if err != nil {
		return fallback
	}
	typeName, subType, ok := strings.Cut(mediaType, "/")
	if !ok || strings.TrimSpace(typeName) == "" || strings.TrimSpace(subType) == "" {
		return fallback
	}
	return mimeType
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeNDJSON[T any](w http.ResponseWriter, status int, rows []T) {
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	encoder := json.NewEncoder(w)
	for _, row := range rows {
		_ = encoder.Encode(row)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	code := "internal_error"
	switch status {
	case http.StatusBadRequest:
		code = "bad_request"
	case http.StatusUnauthorized:
		code = "unauthorized"
	case http.StatusForbidden:
		code = "forbidden"
	case http.StatusNotFound:
		code = "not_found"
	case http.StatusConflict:
		code = "conflict"
	case http.StatusRequestEntityTooLarge:
		code = "payload_too_large"
	case http.StatusInsufficientStorage:
		code = "insufficient_storage"
	case http.StatusRequestedRangeNotSatisfiable:
		code = "range_not_satisfiable"
	case http.StatusNotImplemented:
		code = "not_implemented"
	}
	writeJSON(w, status, map[string]any{
		"error": map[string]any{
			"code":        code,
			"message":     message,
			"status":      status,
			"status_text": http.StatusText(status),
		},
		"error_message": message,
	})
}

func writeInternalServerError(w http.ResponseWriter) {
	writeError(w, http.StatusInternalServerError, "internal server error")
}

func writeMailServiceError(w http.ResponseWriter, err error) {
	if errors.Is(err, mail.ErrMailboxFull) {
		writeError(w, http.StatusInsufficientStorage, err.Error())
		return
	}
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		writeError(w, http.StatusRequestEntityTooLarge, "attachment upload request is too large")
		return
	}
	errMsg := err.Error()
	if strings.Contains(errMsg, "chunk range overlap") || strings.Contains(errMsg, "chunk gap") {
		writeError(w, http.StatusRequestedRangeNotSatisfiable, errMsg)
		return
	}
	writeError(w, http.StatusBadRequest, errMsg)
}
