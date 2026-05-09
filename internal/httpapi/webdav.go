package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gogomail/gogomail/internal/drive"
	"github.com/gogomail/gogomail/internal/webdavgw"
)

// WebDAVService mirrors the Drive operations needed for WebDAV protocol translation.
type WebDAVService interface {
	ListNodes(ctx context.Context, req drive.ListNodesRequest) ([]drive.Node, error)
	GetNode(ctx context.Context, req drive.GetNodeRequest) (drive.Node, error)
	OpenFile(ctx context.Context, req drive.OpenFileRequest) (drive.FileDownload, error)
	CreateFolder(ctx context.Context, req drive.CreateFolderRequest) (drive.Node, error)
	CreateFile(ctx context.Context, req drive.CreateFileRequest) (drive.Node, error)
	TrashNode(ctx context.Context, req drive.TrashNodeRequest) error
	RenameNode(ctx context.Context, req drive.RenameNodeRequest) (drive.Node, error)
	MoveNode(ctx context.Context, req drive.MoveNodeRequest) (drive.Node, error)
	CopyNode(ctx context.Context, req drive.CopyNodeRequest) (drive.Node, error)
	GetUsageSummary(ctx context.Context, req drive.GetUsageSummaryRequest) (drive.UsageSummary, error)
	LockNode(ctx context.Context, req drive.LockNodeRequest) (drive.LockToken, error)
	UnlockNode(ctx context.Context, req drive.UnlockNodeRequest) error
}

// WebDAVRouteOptions configures the WebDAV handler.
type WebDAVRouteOptions struct {
	DepthInfinityEnabled bool
	Metrics              WebDAVMetrics
}

// RegisterWebDAVRoutes registers WebDAV RFC 4918 handlers on mux at /dav/.
// Supported methods: OPTIONS, PROPFIND, MKCOL, GET, PUT, DELETE, MOVE, COPY, PROPPATCH.
func RegisterWebDAVRoutes(mux *http.ServeMux, service WebDAVService, opts WebDAVRouteOptions) {
	_ = opts.DepthInfinityEnabled
	h := &webdavHandler{
		service: service,
		opts:    opts,
		locks:   make(map[string]webdavLock),
		metrics: webdavMetricsOrDefault(opts.Metrics),
	}

	// CORS preflight for WebDAV (some clients send Origin)
	mux.HandleFunc("OPTIONS /dav/", h.handleOptions)

	// PROPFIND — list directory contents with DAV: properties
	mux.HandleFunc("PROPFIND /dav/", h.handlePropfind)

	// MKCOL — create a collection (folder)
	mux.HandleFunc("MKCOL /dav/", h.handleMkcol)

	// GET — download a file
	mux.HandleFunc("GET /dav/", h.handleGet)

	// PUT — upload/replace a file (not implemented in v1, returns 501)
	mux.HandleFunc("PUT /dav/", h.handlePut)

	mux.HandleFunc("DELETE /dav/", h.handleDelete)

	// MOVE — rename or move a node
	mux.HandleFunc("MOVE /dav/", h.handleMove)

	// COPY — copy a node
	mux.HandleFunc("COPY /dav/", h.handleCopy)

	// PROPPATCH — update dead properties (e.g. displayname)
	mux.HandleFunc("PROPPATCH /dav/", h.handleProppatch)

	// LOCK — acquire an exclusive/shared lock on a resource
	mux.HandleFunc("LOCK /dav/", h.handleLock)

	// UNLOCK — release a lock held on a resource
	mux.HandleFunc("UNLOCK /dav/", h.handleUnlock)
}

type webdavHandler struct {
	service WebDAVService
	opts    WebDAVRouteOptions
	locks   map[string]webdavLock
	mu      sync.Mutex
	metrics WebDAVMetrics
}

type webdavLock struct {
	Token  string
	UserID string
	Expiry time.Time
}

func (h *webdavHandler) observe(ctx context.Context, method WebDAVMetricMethod, userID, path string, result WebDAVMetricResult, errMsg string) {
	h.metrics.ObserveWebDAV(ctx, WebDAVMetricEvent{
		Method: method,
		Result: result,
		UserID: userID,
		Path:   path,
		Error:  errMsg,
	})
}

func (h *webdavHandler) handleOptions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("DAV", "1, 3")
	w.Header().Set("Allow", "OPTIONS, PROPFIND, MKCOL, GET, PUT, DELETE, MOVE, COPY, PROPPATCH, LOCK, UNLOCK")
	w.Header().Set("Content-Length", "0")
	w.WriteHeader(http.StatusNoContent)
	h.observe(r.Context(), WebDAVMethodOptions, "", "", WebDAVResultOK, "")
}

// propfindRequest matches WebDAV PROPFIND request body.
type propfindRequest struct {
	XMLName xml.Name `xml:"propfind"`
	Prop    struct {
		Fields []xml.Name `xml:"prop>"`
	} `xml:"prop"`
}

func (h *webdavHandler) handlePropfind(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	depth := r.Header.Get("Depth")
	if depth == "" {
		depth = "1"
	}
	if depth == "infinity" && !h.opts.DepthInfinityEnabled {
		http.Error(w, "Depth infinity not allowed", http.StatusForbidden)
		h.observe(ctx, WebDAVMethodPropfind, "", "", WebDAVResultRejected, "depth_infinity_forbidden")
		return
	}

	reqBody, _ := io.ReadAll(r.Body)
	_ = reqBody

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		userID = r.Header.Get("X-WebDAV-User-ID")
	}
	if userID == "" {
		http.Error(w, "user_id required", http.StatusUnauthorized)
		h.observe(ctx, WebDAVMethodPropfind, "", "", WebDAVResultRejected, "unauthorized")
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/dav/")
	path = strings.TrimSuffix(path, "/")

	var parentID string
	if path != "" {
		parentID = path
	}

	nodes, err := h.service.ListNodes(ctx, drive.ListNodesRequest{
		UserID:   userID,
		ParentID: parentID,
		Status:   drive.NodeStatusActive,
		Limit:    1000,
	})
	if err != nil {
		http.Error(w, "failed to list nodes: "+err.Error(), http.StatusInternalServerError)
		h.observe(ctx, WebDAVMethodPropfind, userID, path, WebDAVResultError, err.Error())
		return
	}

	// Convert to WebDAV resources
	resources := make([]webdavgw.Resource, 0, len(nodes)+1)

	// Root collection with RFC 4331 quota properties
	rootRes := webdavgw.Resource{
		Href:         "/dav/",
		Name:         "Drive",
		IsCollection: true,
	}
	if summary, err := h.service.GetUsageSummary(ctx, drive.GetUsageSummaryRequest{UserID: userID}); err == nil {
		used := summary.QuotaUsed
		rootRes.QuotaUsedBytes = &used
		if summary.QuotaLimit > 0 {
			avail := summary.QuotaLimit - summary.QuotaUsed
			if avail < 0 {
				avail = 0
			}
			rootRes.QuotaAvailableBytes = &avail
		}
	}
	resources = append(resources, rootRes)

	for _, n := range nodes {
		resources = append(resources, webdavgw.Resource{
			Href:         "/dav/" + n.ID + "/",
			Name:         n.Name,
			Size:         n.Size,
			IsCollection: n.Type == drive.NodeTypeFolder,
			Modified:     n.UpdatedAt,
			ContentType:  n.MIMEType,
		})
	}

	// Marshal as WebDAV multistatus XML
	body, err := webdavgw.MarshalPropfindResponse(resources)
	if err != nil {
		http.Error(w, "marshal failed: "+err.Error(), http.StatusInternalServerError)
		h.observe(ctx, WebDAVMethodPropfind, userID, path, WebDAVResultError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(http.StatusMultiStatus)
	w.Write(body)
	h.observe(ctx, WebDAVMethodPropfind, userID, path, WebDAVResultOK, "")
}

func (h *webdavHandler) handleMkcol(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		userID = r.Header.Get("X-WebDAV-User-ID")
	}
	if userID == "" {
		http.Error(w, "user_id required", http.StatusUnauthorized)
		h.observe(ctx, WebDAVMethodMkcol, "", "", WebDAVResultRejected, "unauthorized")
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/dav/")
	path = strings.TrimSuffix(path, "/")

	var parentID, name string
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		parentID = path[:idx]
		name = path[idx+1:]
	} else {
		name = path
	}

	if name == "" {
		http.Error(w, "collection name required", http.StatusBadRequest)
		h.observe(ctx, WebDAVMethodMkcol, userID, path, WebDAVResultRejected, "missing_name")
		return
	}

	node, err := h.service.CreateFolder(ctx, drive.CreateFolderRequest{
		UserID:   userID,
		ParentID: parentID,
		Name:     name,
	})
	if err != nil {
		http.Error(w, "create folder failed: "+err.Error(), http.StatusInternalServerError)
		h.observe(ctx, WebDAVMethodMkcol, userID, path, WebDAVResultError, err.Error())
		return
	}

	w.Header().Set("Location", "/dav/"+node.ID+"/")
	w.WriteHeader(http.StatusCreated)
	h.observe(ctx, WebDAVMethodMkcol, userID, path, WebDAVResultOK, "")
}

func (h *webdavHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		userID = r.Header.Get("X-WebDAV-User-ID")
	}
	if userID == "" {
		http.Error(w, "user_id required", http.StatusUnauthorized)
		h.observe(ctx, WebDAVMethodGet, "", "", WebDAVResultRejected, "unauthorized")
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/dav/")
	path = strings.TrimSuffix(path, "/")
	nodeID := strings.Split(path, "/")[0]

	dl, err := h.service.OpenFile(ctx, drive.OpenFileRequest{
		UserID: userID,
		NodeID: nodeID,
	})
	if err != nil {
		http.Error(w, "open file failed: "+err.Error(), http.StatusInternalServerError)
		h.observe(ctx, WebDAVMethodGet, userID, path, WebDAVResultError, err.Error())
		return
	}
	defer dl.Body.Close()

	w.Header().Set("Content-Type", dl.Node.MIMEType)
	w.Header().Set("Content-Length", strconv.FormatInt(dl.Node.Size, 10))
	w.Header().Set("ETag", fmt.Sprintf(`"%s"`, dl.Node.ID))
	w.WriteHeader(http.StatusOK)
	io.Copy(w, dl.Body)
	h.observe(ctx, WebDAVMethodGet, userID, path, WebDAVResultOK, "")
}

func (h *webdavHandler) handlePut(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		userID = r.Header.Get("X-WebDAV-User-ID")
	}
	if userID == "" {
		http.Error(w, "user_id required", http.StatusUnauthorized)
		h.observe(ctx, WebDAVMethodPut, "", "", WebDAVResultRejected, "unauthorized")
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/dav/")
	path = strings.TrimSuffix(path, "/")

	var parentID, name string
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		parentID = path[:idx]
		name = path[idx+1:]
	} else {
		name = path
	}

	if name == "" {
		http.Error(w, "file name required", http.StatusBadRequest)
		h.observe(ctx, WebDAVMethodPut, userID, path, WebDAVResultRejected, "missing_name")
		return
	}

	mimeType := r.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	var contentLength int64
	if cl := r.Header.Get("Content-Length"); cl != "" {
		contentLength, _ = strconv.ParseInt(cl, 10, 64)
	}

	existingNodes, err := h.service.ListNodes(ctx, drive.ListNodesRequest{
		UserID:   userID,
		ParentID: parentID,
		Status:   drive.NodeStatusActive,
	})
	if err == nil {
		for _, n := range existingNodes {
			if n.Name == name && n.Type == drive.NodeTypeFile {
				_ = h.service.TrashNode(ctx, drive.TrashNodeRequest{UserID: userID, NodeID: n.ID})
				break
			}
		}
	}

	node, err := h.service.CreateFile(ctx, drive.CreateFileRequest{
		UserID:   userID,
		ParentID: parentID,
		Name:     name,
		Body:     r.Body,
		Size:     contentLength,
		MIMEType: mimeType,
	})
	if err != nil {
		if strings.Contains(err.Error(), "quota exceeded") {
			http.Error(w, "insufficient storage", http.StatusInsufficientStorage)
			h.observe(ctx, WebDAVMethodPut, userID, path, WebDAVResultRejected, "quota_exceeded")
			return
		}
		http.Error(w, "create file failed: "+err.Error(), http.StatusInternalServerError)
		h.observe(ctx, WebDAVMethodPut, userID, path, WebDAVResultError, err.Error())
		return
	}

	w.Header().Set("Location", "/dav/"+node.ID+"/"+node.Name)
	w.WriteHeader(http.StatusCreated)
	h.observe(ctx, WebDAVMethodPut, userID, path, WebDAVResultOK, "")
}

func (h *webdavHandler) handleDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		userID = r.Header.Get("X-WebDAV-User-ID")
	}
	if userID == "" {
		http.Error(w, "user_id required", http.StatusUnauthorized)
		h.observe(ctx, WebDAVMethodDelete, "", "", WebDAVResultRejected, "unauthorized")
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/dav/")
	path = strings.TrimSuffix(path, "/")
	nodeID := strings.Split(path, "/")[0]

	if nodeID == "" {
		http.Error(w, "node id required", http.StatusBadRequest)
		h.observe(ctx, WebDAVMethodDelete, userID, path, WebDAVResultRejected, "missing_node_id")
		return
	}

	err := h.service.TrashNode(ctx, drive.TrashNodeRequest{
		UserID: userID,
		NodeID: nodeID,
	})
	if err != nil {
		http.Error(w, "trash failed: "+err.Error(), http.StatusInternalServerError)
		h.observe(ctx, WebDAVMethodDelete, userID, path, WebDAVResultError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
	h.observe(ctx, WebDAVMethodDelete, userID, path, WebDAVResultOK, "")
}

func (h *webdavHandler) handleMove(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		userID = r.Header.Get("X-WebDAV-User-ID")
	}
	if userID == "" {
		http.Error(w, "user_id required", http.StatusUnauthorized)
		h.observe(ctx, WebDAVMethodMove, "", "", WebDAVResultRejected, "unauthorized")
		return
	}

	dest := r.Header.Get("Destination")
	if dest == "" {
		http.Error(w, "Destination header required", http.StatusBadRequest)
		h.observe(ctx, WebDAVMethodMove, userID, "", WebDAVResultRejected, "missing_destination")
		return
	}
	if u, err := url.Parse(dest); err == nil && u.Path != "" {
		dest = u.Path
	}
	dest = strings.TrimPrefix(dest, "/dav/")
	dest = strings.TrimSuffix(dest, "/")

	path := strings.TrimPrefix(r.URL.Path, "/dav/")
	path = strings.TrimSuffix(path, "/")
	nodeID := strings.Split(path, "/")[0]

	var parentID string
	if idx := strings.LastIndex(dest, "/"); idx >= 0 {
		parentID = dest[:idx]
	} else {
		parentID = dest
	}

	_, err := h.service.MoveNode(ctx, drive.MoveNodeRequest{
		UserID:   userID,
		NodeID:   nodeID,
		ParentID: parentID,
	})
	if err != nil {
		http.Error(w, "move failed: "+err.Error(), http.StatusInternalServerError)
		h.observe(ctx, WebDAVMethodMove, userID, path, WebDAVResultError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
	h.observe(ctx, WebDAVMethodMove, userID, path, WebDAVResultOK, "")
}

func (h *webdavHandler) handleCopy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		userID = r.Header.Get("X-WebDAV-User-ID")
	}
	if userID == "" {
		http.Error(w, "user_id required", http.StatusUnauthorized)
		h.observe(ctx, WebDAVMethodCopy, "", "", WebDAVResultRejected, "unauthorized")
		return
	}

	dest := r.Header.Get("Destination")
	if dest == "" {
		http.Error(w, "Destination header required", http.StatusBadRequest)
		h.observe(ctx, WebDAVMethodCopy, userID, "", WebDAVResultRejected, "missing_destination")
		return
	}
	if u, err := url.Parse(dest); err == nil && u.Path != "" {
		dest = u.Path
	}
	dest = strings.TrimPrefix(dest, "/dav/")
	dest = strings.TrimSuffix(dest, "/")

	path := strings.TrimPrefix(r.URL.Path, "/dav/")
	path = strings.TrimSuffix(path, "/")
	nodeID := strings.Split(path, "/")[0]

	var parentID, name string
	if idx := strings.LastIndex(dest, "/"); idx >= 0 {
		parentID = dest[:idx]
		name = dest[idx+1:]
	} else {
		parentID = ""
		name = dest
	}

	_, err := h.service.CopyNode(ctx, drive.CopyNodeRequest{
		UserID:   userID,
		NodeID:   nodeID,
		ParentID: parentID,
		Name:     name,
	})
	if err != nil {
		http.Error(w, "copy failed: "+err.Error(), http.StatusInternalServerError)
		h.observe(ctx, WebDAVMethodCopy, userID, path, WebDAVResultError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
	h.observe(ctx, WebDAVMethodCopy, userID, path, WebDAVResultOK, "")
}

func (h *webdavHandler) handleProppatch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		userID = r.Header.Get("X-WebDAV-User-ID")
	}
	if userID == "" {
		http.Error(w, "user_id required", http.StatusUnauthorized)
		h.observe(ctx, WebDAVMethodProppatch, "", "", WebDAVResultRejected, "unauthorized")
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/dav/")
	path = strings.TrimSuffix(path, "/")
	nodeID := strings.Split(path, "/")[0]

	if nodeID == "" {
		http.Error(w, "node id required", http.StatusBadRequest)
		h.observe(ctx, WebDAVMethodProppatch, userID, path, WebDAVResultRejected, "missing_node_id")
		return
	}

	body, _ := io.ReadAll(r.Body)

	newName := extractDisplayName(body)
	if newName != "" {
		_, err := h.service.RenameNode(ctx, drive.RenameNodeRequest{
			UserID: userID,
			NodeID: nodeID,
			Name:   newName,
		})
		if err != nil {
			http.Error(w, "rename failed: "+err.Error(), http.StatusInternalServerError)
			h.observe(ctx, WebDAVMethodProppatch, userID, path, WebDAVResultError, err.Error())
			return
		}
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.WriteHeader(http.StatusMultiStatus)
	w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<d:multistatus xmlns:d="DAV:">
  <d:response>
    <d:href>/dav/` + nodeID + `/</d:href>
    <d:propstat>
      <d:prop><d:displayname/></d:prop>
      <d:status>HTTP/1.1 200 OK</d:status>
    </d:propstat>
  </d:response>
</d:multistatus>`))
	h.observe(ctx, WebDAVMethodProppatch, userID, path, WebDAVResultOK, "")
}

func extractDisplayName(body []byte) string {
	const prefix = "<d:displayname>"
	const suffix = "</d:displayname>"
	start := strings.Index(string(body), prefix)
	if start < 0 {
		return ""
	}
	start += len(prefix)
	end := strings.Index(string(body)[start:], suffix)
	if end < 0 {
		return ""
	}
	return string(body)[start : start+end]
}

func (h *webdavHandler) handleLock(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		userID = r.Header.Get("X-WebDAV-User-ID")
	}
	if userID == "" {
		http.Error(w, "user_id required", http.StatusUnauthorized)
		h.observe(ctx, WebDAVMethodLock, "", "", WebDAVResultRejected, "unauthorized")
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/dav/")
	path = "/" + path

	token := generateLockToken()
	lock := webdavLock{
		Token:  token,
		UserID: userID,
		Expiry: time.Now().Add(5 * time.Minute),
	}

	h.mu.Lock()
	h.locks[path] = lock
	h.mu.Unlock()

	w.Header().Set("Lock-Token", fmt.Sprintf("<%s%s>", drive.LockTokenScheme, token))
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<d:multistatus xmlns:d="DAV:">
  <d:response>
    <d:href>` + path + `</d:href>
    <d:propstat>
      <d:prop>
        <d:lockdiscovery>
          <d:activelock>
            <d:locktype><d:write/></d:locktype>
            <d:lockscope><d:exclusive/></d:lockscope>
            <d:depth>infinity</d:depth>
            <d:owner><d:href>` + userID + `</d:href></d:owner>
            <d:timeout>Second-300</d:timeout>
            <d:locktoken>
              <d:href>` + drive.LockTokenScheme + token + `</d:href>
            </d:locktoken>
          </d:activelock>
        </d:lockdiscovery>
      </d:prop>
      <d:status>HTTP/1.1 200 OK</d:status>
    </d:propstat>
  </d:response>
</d:multistatus>`))
	h.observe(ctx, WebDAVMethodLock, userID, path, WebDAVResultOK, "")
}

func (h *webdavHandler) handleUnlock(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		userID = r.Header.Get("X-WebDAV-User-ID")
	}
	if userID == "" {
		http.Error(w, "user_id required", http.StatusUnauthorized)
		h.observe(ctx, WebDAVMethodUnlock, "", "", WebDAVResultRejected, "unauthorized")
		return
	}

	lockTokenHdr := r.Header.Get("Lock-Token")
	lockTokenHdr = strings.TrimPrefix(lockTokenHdr, "<")
	lockTokenHdr = strings.TrimSuffix(lockTokenHdr, ">")
	token := strings.TrimPrefix(lockTokenHdr, drive.LockTokenScheme)

	path := strings.TrimPrefix(r.URL.Path, "/dav/")
	path = "/" + path

	h.mu.Lock()
	defer h.mu.Unlock()

	existing, ok := h.locks[path]
	if !ok || existing.UserID != userID || existing.Token != token {
		http.Error(w, "lock not found or not owned", http.StatusConflict)
		h.observe(ctx, WebDAVMethodUnlock, userID, path, WebDAVResultRejected, "lock_not_found")
		return
	}
	if existing.Expiry.Before(time.Now()) {
		delete(h.locks, path)
		http.Error(w, "lock expired", http.StatusConflict)
		h.observe(ctx, WebDAVMethodUnlock, userID, path, WebDAVResultRejected, "lock_expired")
		return
	}

	delete(h.locks, path)
	w.WriteHeader(http.StatusNoContent)
	h.observe(ctx, WebDAVMethodUnlock, userID, path, WebDAVResultOK, "")
}

func generateLockToken() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
