package jmap

import (
	"context"
	"encoding/json"
	"net/http"
)

// Method is implemented by each JMAP method handler.
// accountID is the resolved account for this call; args is the raw JSON
// argument object from the request MethodCall.
type Method interface {
	// Call is intentionally typed with an empty-interface context carrier so
	// that future callers can pass request-scoped values (DB handle, user info)
	// without changing the interface.
	Call(ctx interface{}, accountID string, args json.RawMessage) (json.RawMessage, error)
}

// SessionFunc builds a Session for the authenticated user.
type SessionFunc func(ctx context.Context, userID, accountID string) (*Session, error)

// Handler serves JMAP Session (/.well-known/jmap) and API (/jmap/api)
// endpoints. It dispatches MethodCalls to registered Method implementations.
type Handler struct {
	sessionFn SessionFunc
	methods   map[string]Method
}

// NewHandler creates a Handler with the given session builder function.
// Methods are pre-registered for Email/get and Email/query.
func NewHandler(sessionFn SessionFunc) *Handler {
	h := &Handler{
		sessionFn: sessionFn,
		methods:   make(map[string]Method),
	}
	h.Register("Email/get", emailGetMethod{})
	h.Register("Email/query", emailQueryMethod{})
	return h
}

// Register adds a Method under the given JMAP method name.
func (h *Handler) Register(name string, m Method) {
	h.methods[name] = m
}

// ServeSession handles GET /.well-known/jmap — it returns the JMAP Session
// resource. For this stub, the user identity is taken from a query parameter
// "u"; real implementations should authenticate via JWT/session cookie.
func (h *Handler) ServeSession(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("u")
	if userID == "" {
		userID = "anonymous"
	}
	// Use userID as accountID for the stub; real code will map to a UUID.
	accountID := userID

	sess, err := h.sessionFn(r.Context(), userID, accountID)
	if err != nil {
		http.Error(w, `{"type":"serverFail"}`, http.StatusInternalServerError)
		return
	}
	writeJSONResponse(w, http.StatusOK, sess)
}

// ServeAPI handles POST /jmap/api — it decodes a JMAP Request, dispatches each
// MethodCall to the registered handler, and returns a JMAP Response.
func (h *Handler) ServeAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"type":"notRequest"}`, http.StatusMethodNotAllowed)
		return
	}

	var req Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONResponse(w, http.StatusBadRequest, map[string]string{"type": ErrNotJSON})
		return
	}

	// Default accountID derived from the first PrimaryAccount in the session;
	// for the stub we just use a constant. Real code extracts it from the JWT.
	accountID := "default"

	resp := Response{
		SessionState:    "state-v1",
		MethodResponses: make([]MethodResponse, 0, len(req.MethodCalls)),
	}

	for _, call := range req.MethodCalls {
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
