package jmap

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gogomail/gogomail/internal/auth"
	"github.com/gogomail/gogomail/internal/maildb"
	"github.com/gogomail/gogomail/internal/storage"
)

// Method is implemented by each JMAP method handler.
// accountID is the resolved account for this call; args is the raw JSON
// argument object from the request MethodCall.
type Method interface {
	Call(ctx context.Context, accountID string, args json.RawMessage) (json.RawMessage, error)
}

// SessionFunc builds a Session for the authenticated user.
type SessionFunc func(ctx context.Context, userID, accountID string) (*Session, error)

// Deps holds the external dependencies required by Handler.
type Deps struct {
	Repo  *maildb.Repository
	Store storage.Store
	Auth  *auth.TokenManager
}

// Handler serves JMAP Session (/.well-known/jmap) and API (/jmap/api)
// endpoints. It dispatches MethodCalls to registered Method implementations.
type Handler struct {
	deps      Deps
	sessionFn SessionFunc
	methods   map[string]Method
}

// NewHandler creates a Handler with the given deps and session builder function.
// Methods are pre-registered for Email/get and Email/query.
func NewHandler(deps Deps, sessionFn SessionFunc) *Handler {
	h := &Handler{
		deps:      deps,
		sessionFn: sessionFn,
		methods:   make(map[string]Method),
	}
	h.Register("Email/get", emailGetMethod{})
	h.Register("Email/query", emailQueryMethod{})
	h.Register("Mailbox/get", &mailboxGetMethod{deps: deps})
	h.Register("Mailbox/query", &mailboxQueryMethod{deps: deps})
	h.Register("Mailbox/set", &mailboxSetMethod{deps: deps})
	h.Register("Mailbox/changes", &mailboxChangesMethod{deps: deps})
	h.Register("Thread/get", &threadGetMethod{deps: deps})
	h.Register("Thread/changes", &threadChangesMethod{deps: deps})
	return h
}

// Register adds a Method under the given JMAP method name.
func (h *Handler) Register(name string, m Method) {
	h.methods[name] = m
}

// userIDFromBearer extracts and validates the JWT Bearer token from the
// Authorization header. If Auth is nil (test mode), it falls back to the
// X-Test-UserID header. Returns (userID, true) on success, or ("", false)
// if the token is missing or invalid.
func (h *Handler) userIDFromBearer(r *http.Request) (string, bool) {
	if h.deps.Auth == nil {
		// Test mode: accept X-Test-UserID header as identity.
		uid := r.Header.Get("X-Test-UserID")
		if uid == "" {
			uid = "test-user"
		}
		return uid, true
	}

	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return "", false
	}
	token := strings.TrimPrefix(authHeader, "Bearer ")
	claims, err := h.deps.Auth.VerifyFull(r.Context(), token)
	if err != nil {
		return "", false
	}
	return claims.UserID, true
}

// ServeSession handles GET /.well-known/jmap — it returns the JMAP Session
// resource after authenticating the caller via JWT Bearer token.
func (h *Handler) ServeSession(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.userIDFromBearer(r)
	if !ok {
		writeJSONResponse(w, http.StatusUnauthorized, map[string]string{"type": "unauthorized"})
		return
	}
	accountID := userID

	sess, err := h.sessionFn(r.Context(), userID, accountID)
	if err != nil {
		http.Error(w, `{"type":"serverFail"}`, http.StatusInternalServerError)
		return
	}
	writeJSONResponse(w, http.StatusOK, sess)
}

// knownCapabilities lists the JMAP capability URIs this server supports.
var knownCapabilities = map[string]bool{
	CapabilityCore: true,
	CapabilityMail: true,
}

// ServeAPI handles POST /jmap/api — it decodes a JMAP Request, validates
// the using array, enforces the call limit, dispatches each MethodCall, and
// returns a JMAP Response.
func (h *Handler) ServeAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"type":"notRequest"}`, http.StatusMethodNotAllowed)
		return
	}

	userID, ok := h.userIDFromBearer(r)
	if !ok {
		writeJSONResponse(w, http.StatusUnauthorized, map[string]string{"type": "unauthorized"})
		return
	}
	accountID := userID

	var req Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONResponse(w, http.StatusBadRequest, map[string]string{"type": ErrNotJSON})
		return
	}

	// Validate using array — return unknownCapability for any unrecognised entry.
	for _, cap := range req.Using {
		if !knownCapabilities[cap] {
			writeJSONResponse(w, http.StatusBadRequest, map[string]string{"type": ErrUnknownCapability})
			return
		}
	}

	// Enforce maxCallsInRequest.
	if len(req.MethodCalls) > maxCallsInRequest {
		writeJSONResponse(w, http.StatusBadRequest, map[string]string{"type": ErrRequestTooLarge})
		return
	}

	resp := Response{
		SessionState:    "state-v1",
		MethodResponses: make([]MethodResponse, 0, len(req.MethodCalls)),
	}

	// prevResults stores each call's raw result keyed by callID so that
	// subsequent calls can reference it via RFC 8620 §3.7 back-references.
	prevResults := make(map[string]json.RawMessage)

	for _, call := range req.MethodCalls {
		// Resolve any "#key" back-references in the arguments before dispatch.
		resolvedArgs, err := resolveBackRefs(call.Args, prevResults)
		if err != nil {
			resp.MethodResponses = append(resp.MethodResponses, MethodResponse{
				Name:   "error",
				Result: errorResult("invalidResultReference"),
				CallID: call.CallID,
			})
			continue
		}
		call.Args = resolvedArgs

		method, ok := h.methods[call.Name]
		if !ok {
			resp.MethodResponses = append(resp.MethodResponses, MethodResponse{
				Name:   "error",
				Result: errorResult(ErrUnknownMethod),
				CallID: call.CallID,
			})
			continue
		}

		result, err := method.Call(r.Context(), accountID, call.Args)
		if err != nil {
			resp.MethodResponses = append(resp.MethodResponses, MethodResponse{
				Name:   "error",
				Result: errorResult(ErrServerFail),
				CallID: call.CallID,
			})
			continue
		}

		// Store result so later calls can back-reference it.
		prevResults[call.CallID] = result

		resp.MethodResponses = append(resp.MethodResponses, MethodResponse{
			Name:   call.Name,
			Result: result,
			CallID: call.CallID,
		})
	}

	writeJSONResponse(w, http.StatusOK, resp)
}

// writeJSONResponse marshals v and writes it with Content-Type application/json.
func writeJSONResponse(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// errorResult encodes a JMAP error object {"type": errType}.
func errorResult(errType string) json.RawMessage {
	b, _ := json.Marshal(methodError{Type: errType})
	return b
}
