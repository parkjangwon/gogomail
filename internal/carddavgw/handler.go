package carddavgw

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type contactObjectLookupKey struct {
	addressBookID string
	objectName    string
}

type DiscoveryStore interface {
	LookupPrincipal(ctx context.Context, userID string) (Principal, error)
	ListAddressBookCollections(ctx context.Context, userID string) ([]AddressBook, error)
	LookupAddressBook(ctx context.Context, userID string, addressBookID string) (AddressBook, error)
	ListAddressBookObjects(ctx context.Context, userID string, addressBookID string) ([]ContactObject, error)
	LookupContactObject(ctx context.Context, userID string, addressBookID string, objectName string) (ContactObject, error)
}

type AddressBookObjectLimiter interface {
	ListAddressBookObjectsLimit(ctx context.Context, userID string, addressBookID string, limit int) ([]ContactObject, error)
}

type AddressBookObjectBatchStore interface {
	ListContactObjectsByNameGroups(ctx context.Context, userID string, objectNamesByAddressBook map[string][]string, status string) ([]ContactObject, error)
}

type AddressBookCreator interface {
	CreateAddressBookAtPath(ctx context.Context, req CreateAddressBookAtPathRequest) (AddressBook, error)
}

type AddressBookDeleter interface {
	DeleteAddressBook(ctx context.Context, req DeleteAddressBookRequest) (AddressBook, error)
}

type UserResolver func(*http.Request) (string, error)

const (
	ContactsAccessRoleRead   = "read"
	ContactsAccessRoleWrite  = "write"
	ContactsAccessRoleManage = "manage"
)

const maxConditionalIfHeaderBytes = 8192

type AccessRequest struct {
	ActorUserID  string
	OwnerUserID  string
	Resource     ResourcePath
	RequiredRole string
}

type AccessDecision struct {
	Allowed    bool
	Privileges []XMLName
}

type AccessAuthorizer interface {
	AuthorizeAddressBookAccess(ctx context.Context, req AccessRequest) (AccessDecision, error)
}

// gatewayMetrics is the minimal interface carddavgw uses for observability.
// *protocolmetrics.GatewayMetrics satisfies this interface.
type gatewayMetrics interface {
	RecordCommand(userID string, duration time.Duration)
	RecordError(userID string)
}

type Handler struct {
	Store            DiscoveryStore
	ResolveUser      UserResolver
	AccessAuthorizer AccessAuthorizer
	IncludeSync      bool
	metrics          gatewayMetrics
}

type SyncChangeStore interface {
	ListAddressBookChangesSince(ctx context.Context, req ListAddressBookChangesSinceRequest) ([]AddressBookChange, error)
}

type AddressBookChangeWithObjectStore interface {
	ListAddressBookChangesWithObjectsSince(ctx context.Context, req ListAddressBookChangesSinceRequest, includeVCard bool) ([]AddressBookChangeWithObject, error)
}

type AddressBookUpdater interface {
	UpdateAddressBookProperties(ctx context.Context, req UpdateAddressBookRequest) (AddressBook, error)
}

type ObjectStore interface {
	UpsertContactObject(ctx context.Context, req UpsertContactObjectRequest) (ContactObject, error)
	DeleteContactObject(ctx context.Context, req DeleteContactObjectRequest) (ContactObject, error)
}

type ObjectWalker interface {
	WalkAddressBookObjects(ctx context.Context, userID string, addressBookID string, yield func(ContactObject) (bool, error)) error
}

type AddressBookQueryCandidateWalker interface {
	WalkAddressBookQueryCandidates(ctx context.Context, userID string, addressBookID string, containsText string, yield func(ContactObject) (bool, error)) error
}

type PrincipalSearchStore interface {
	SearchAddressBookObjects(ctx context.Context, userID string, addressBookID string, property string, match string, test string) ([]ContactObject, error)
}

type InvalidSyncTokenError struct {
	Token string
}

func (e InvalidSyncTokenError) Error() string {
	return "CardDAV sync-token is unknown or expired"
}

type TruncatedResultsError struct {
	Operation string
}

func (e TruncatedResultsError) Error() string {
	operation := strings.TrimSpace(e.Operation)
	if operation == "" {
		operation = "CardDAV request"
	}
	return operation + " would truncate results"
}

func NewHandler(store DiscoveryStore, resolveUser UserResolver) *Handler {
	return &Handler{Store: store, ResolveUser: resolveUser, IncludeSync: true}
}

func QueryUserResolver(r *http.Request) (string, error) {
	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
	if userID == "" {
		return "", fmt.Errorf("user_id is required")
	}
	if strings.ContainsAny(userID, "\r\n") {
		return "", fmt.Errorf("user_id must not contain line breaks")
	}
	return userID, nil
}

// SetMetrics sets optional metrics collector for gateway observability
func (h *Handler) SetMetrics(metrics gatewayMetrics) {
	if h == nil {
		return
	}
	h.metrics = metrics
}

// recordCommand records HTTP operation processing with optional metrics
func (h *Handler) recordCommand(userID string, duration time.Duration) {
	if h == nil || h.metrics == nil {
		return
	}
	h.metrics.RecordCommand(userID, duration)
}

// recordError records HTTP operation error with optional metrics
func (h *Handler) recordError(userID string) {
	if h == nil || h.metrics == nil {
		return
	}
	h.metrics.RecordError(userID)
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h == nil {
		http.Error(w, "carddav handler is not configured", http.StatusInternalServerError)
		return
	}

	// Extract userID for metrics (skip for well-known paths)
	userID := "unknown"
	if !strings.HasPrefix(r.URL.Path, "/.well-known") {
		resolve := h.ResolveUser
		if resolve == nil {
			resolve = QueryUserResolver
		}
		if id, err := resolve(r); err == nil {
			userID = id
		}
	}

	cmdStart := time.Now()
	metricsWriter := newCardDAVMetricsResponseWriter(w)
	w = metricsWriter
	defer func() {
		if metricsWriter.status >= http.StatusBadRequest {
			h.recordError(userID)
		}
	}()

	if r.URL.Path == WellKnownCardDAVPath {
		h.serveWellKnown(w, r)
		h.recordCommand(userID, time.Since(cmdStart))
		return
	}
	switch r.Method {
	case MethodOptions:
		h.serveOptions(w)
	case MethodPropfind:
		h.servePropfind(w, r)
	case MethodProppatch:
		h.serveProppatch(w, r)
	case MethodReport:
		h.serveReport(w, r)
	case MethodGet, MethodHead:
		h.serveGetObject(w, r)
	case MethodPut:
		h.servePutObject(w, r)
	case MethodDelete:
		h.serveDeleteObject(w, r)
	case MethodMkcol:
		h.serveMkcol(w, r)
	default:
		w.Header().Set("Allow", cardDAVDiscoveryAllowHeader())
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}

	h.recordCommand(userID, time.Since(cmdStart))
}

type cardDAVMetricsResponseWriter struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func newCardDAVMetricsResponseWriter(w http.ResponseWriter) *cardDAVMetricsResponseWriter {
	return &cardDAVMetricsResponseWriter{ResponseWriter: w, status: http.StatusOK}
}

func (w *cardDAVMetricsResponseWriter) WriteHeader(status int) {
	if w.wrote {
		return
	}
	w.status = status
	w.wrote = true
	w.ResponseWriter.WriteHeader(status)
}

func (w *cardDAVMetricsResponseWriter) Write(p []byte) (int, error) {
	if !w.wrote {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(p)
}

func (w *cardDAVMetricsResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

type cardDAVUnauthorizedChallenge interface {
	WWWAuthenticate() string
}

func writeCardDAVUnauthorized(w http.ResponseWriter, err error) {
	challenge := cardDAVWWWAuthenticate
	if err != nil {
		for e := err; e != nil; e = errors.Unwrap(e) {
			if unauthorized, ok := e.(cardDAVUnauthorizedChallenge); ok {
				if value := strings.TrimSpace(unauthorized.WWWAuthenticate()); value != "" {
					challenge = value
				}
				break
			}
		}
		w.Header().Set("WWW-Authenticate", challenge)
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	http.Error(w, "unauthorized", http.StatusUnauthorized)
}

func (h *Handler) serveWellKnown(w http.ResponseWriter, r *http.Request) {
	target := RootPath + "/"
	if r.URL.RawQuery != "" {
		target += "?" + r.URL.RawQuery
	}
	w.Header().Set("Location", target)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusMovedPermanently)
}

func (h *Handler) serveOptions(w http.ResponseWriter) {
	w.Header().Set("Allow", cardDAVDiscoveryAllowHeader())
	w.Header().Set("DAV", strings.Join(AdvertisedDAVTokens(h.includeSyncCollection()), ", "))
	w.Header().Set("MS-Author-Via", "DAV")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) includeSyncCollection() bool {
	if !h.IncludeSync {
		return false
	}
	_, ok := h.Store.(SyncChangeStore)
	return ok
}

func cardDAVDiscoveryAllowHeader() string {
	return strings.Join(ImplementedMethods(), ", ")
}
