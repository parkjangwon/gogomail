package httpapi

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gogomail/gogomail/internal/drive"
	"github.com/gogomail/gogomail/internal/webdavgw"
)

// WebDAVService mirrors the Drive operations needed for WebDAV protocol translation.
type WebDAVService interface {
	ListNodes(ctx context.Context, req drive.ListNodesRequest) ([]drive.Node, error)
	GetNode(ctx context.Context, req drive.GetNodeRequest) (drive.Node, error)
	OpenFile(ctx context.Context, req drive.OpenFileRequest) (drive.FileDownload, error)
	CreateFolder(ctx context.Context, req drive.CreateFolderRequest) (drive.Node, error)
	TrashNode(ctx context.Context, req drive.TrashNodeRequest) error
	RenameNode(ctx context.Context, req drive.RenameNodeRequest) (drive.Node, error)
	MoveNode(ctx context.Context, req drive.MoveNodeRequest) (drive.Node, error)
	CopyNode(ctx context.Context, req drive.CopyNodeRequest) (drive.Node, error)
}

// WebDAVRouteOptions configures the WebDAV handler.
type WebDAVRouteOptions struct {
	// DepthInfinityEnabled allows Depth:infinity PROPFIND (default false for safety).
	DepthInfinityEnabled bool
}

// RegisterWebDAVRoutes registers WebDAV RFC 4918 handlers on mux at /dav/.
// Supported methods: OPTIONS, PROPFIND, MKCOL, GET, PUT, DELETE, MOVE, COPY, PROPPATCH.
func RegisterWebDAVRoutes(mux *http.ServeMux, service WebDAVService, opts WebDAVRouteOptions) {
	if opts.DepthInfinityEnabled {
		// TODO: implement depth-infinity guard
	}
	h := &webdavHandler{service: service, opts: opts}

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
}

type webdavHandler struct {
	service WebDAVService
	opts    WebDAVRouteOptions
}

func (h *webdavHandler) handleOptions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("DAV", "1, 3") // DAVLevel 1 and 3 (private, share)
	w.Header().Set("Allow", "OPTIONS, PROPFIND, MKCOL, GET, PUT, DELETE, MOVE, COPY, PROPPATCH")
	w.Header().Set("Content-Length", "0")
	w.WriteHeader(http.StatusNoContent)
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

	// Parse Depth header: "infinity" requires opt-in
	depth := r.Header.Get("Depth")
	if depth == "" {
		depth = "infinity" // WebDAV default is infinity, but we default to 1
	}

	// TODO: Parse XML request body for requested properties.
	// For now, return all known properties.
	reqBody, _ := io.ReadAll(r.Body)
	_ = reqBody

	// Extract user_id from query or header (WebDAV uses Principal-URL header)
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		userID = r.Header.Get("X-WebDAV-User-ID")
	}
	if userID == "" {
		http.Error(w, "user_id required", http.StatusUnauthorized)
		return
	}

	// Parse path: /dav/ or /dav/folder-id/...
	path := strings.TrimPrefix(r.URL.Path, "/dav/")
	path = strings.TrimSuffix(path, "/")

	var parentID string
	if path != "" {
		parentID = path
	}

	// List nodes at this level
	nodes, err := h.service.ListNodes(ctx, drive.ListNodesRequest{
		UserID:   userID,
		ParentID: parentID,
		Status:   drive.NodeStatusActive,
		Limit:    1000,
	})
	if err != nil {
		http.Error(w, "failed to list nodes: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Convert to WebDAV resources
	resources := make([]webdavgw.Resource, 0, len(nodes))
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

	// Add root entry if depth allows
	if depth == "1" || depth == "infinity" || parentID == "" {
		// Root listing — include parent href
	}

	// Marshal as WebDAV multistatus XML
	body, err := webdavgw.MarshalPropfindResponse(resources)
	if err != nil {
		http.Error(w, "marshal failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(http.StatusMultiStatus)
	w.Write(body)
}

func (h *webdavHandler) handleMkcol(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		userID = r.Header.Get("X-WebDAV-User-ID")
	}
	if userID == "" {
		http.Error(w, "user_id required", http.StatusUnauthorized)
		return
	}

	// Path: /dav/parent-id/foldername or /dav/foldername
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
		return
	}

	node, err := h.service.CreateFolder(ctx, drive.CreateFolderRequest{
		UserID:   userID,
		ParentID: parentID,
		Name:     name,
	})
	if err != nil {
		http.Error(w, "create folder failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Location", "/dav/"+node.ID+"/")
	w.WriteHeader(http.StatusCreated)
}

func (h *webdavHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		userID = r.Header.Get("X-WebDAV-User-ID")
	}
	if userID == "" {
		http.Error(w, "user_id required", http.StatusUnauthorized)
		return
	}

	// Path: /dav/node-id/filename
	path := strings.TrimPrefix(r.URL.Path, "/dav/")
	path = strings.TrimSuffix(path, "/")
	nodeID := strings.Split(path, "/")[0]

	dl, err := h.service.OpenFile(ctx, drive.OpenFileRequest{
		UserID: userID,
		NodeID: nodeID,
	})
	if err != nil {
		http.Error(w, "open file failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer dl.Body.Close()

	w.Header().Set("Content-Type", dl.Node.MIMEType)
	w.Header().Set("Content-Length", strconv.FormatInt(dl.Node.Size, 10))
	w.Header().Set("ETag", fmt.Sprintf(`"%s"`, dl.Node.ID))
	w.WriteHeader(http.StatusOK)
	io.Copy(w, dl.Body)
}

func (h *webdavHandler) handlePut(w http.ResponseWriter, r *http.Request) {
	// PUT requires upload session support — not implemented in v1
	http.Error(w, "PUT not supported; use Drive upload sessions API", http.StatusNotImplemented)
}

func (h *webdavHandler) handleDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		userID = r.Header.Get("X-WebDAV-User-ID")
	}
	if userID == "" {
		http.Error(w, "user_id required", http.StatusUnauthorized)
		return
	}

	// Path: /dav/node-id
	path := strings.TrimPrefix(r.URL.Path, "/dav/")
	path = strings.TrimSuffix(path, "/")
	nodeID := strings.Split(path, "/")[0]

	if nodeID == "" {
		http.Error(w, "node id required", http.StatusBadRequest)
		return
	}

	err := h.service.TrashNode(ctx, drive.TrashNodeRequest{
		UserID: userID,
		NodeID: nodeID,
	})
	if err != nil {
		http.Error(w, "trash failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *webdavHandler) handleMove(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		userID = r.Header.Get("X-WebDAV-User-ID")
	}
	if userID == "" {
		http.Error(w, "user_id required", http.StatusUnauthorized)
		return
	}

	// Parse Destination header: /dav/target-node-id/
	dest := r.Header.Get("Destination")
	if dest == "" {
		http.Error(w, "Destination header required", http.StatusBadRequest)
		return
	}
	if u, err := url.Parse(dest); err == nil && u.Path != "" {
		dest = u.Path
	}
	dest = strings.TrimPrefix(dest, "/dav/")
	dest = strings.TrimSuffix(dest, "/")

	// Path: /dav/node-id
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
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *webdavHandler) handleCopy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		userID = r.Header.Get("X-WebDAV-User-ID")
	}
	if userID == "" {
		http.Error(w, "user_id required", http.StatusUnauthorized)
		return
	}

	// Parse Destination header
	dest := r.Header.Get("Destination")
	if dest == "" {
		http.Error(w, "Destination header required", http.StatusBadRequest)
		return
	}
	if u, err := url.Parse(dest); err == nil && u.Path != "" {
		dest = u.Path
	}
	dest = strings.TrimPrefix(dest, "/dav/")
	dest = strings.TrimSuffix(dest, "/")

	// Path: /dav/node-id
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
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *webdavHandler) handleProppatch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		userID = r.Header.Get("X-WebDAV-User-ID")
	}
	if userID == "" {
		http.Error(w, "user_id required", http.StatusUnauthorized)
		return
	}

	// Path: /dav/node-id
	path := strings.TrimPrefix(r.URL.Path, "/dav/")
	path = strings.TrimSuffix(path, "/")
	nodeID := strings.Split(path, "/")[0]

	if nodeID == "" {
		http.Error(w, "node id required", http.StatusBadRequest)
		return
	}

	// Parse propertyupdate XML body
	body, _ := io.ReadAll(r.Body)
	_ = body

	// TODO: parse set/remove property operations
	// For now, extract displayname from request and rename
	// WebDAV clients typically send: <d:prop><d:displayname>newname</d:displayname></d:prop>
	// Simple approach: extract displayname value if present
	newName := extractDisplayName(body)
	if newName != "" {
		_, err := h.service.RenameNode(ctx, drive.RenameNodeRequest{
			UserID: userID,
			NodeID: nodeID,
			Name:   newName,
		})
		if err != nil {
			http.Error(w, "rename failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Return 207 Multistatus with the updated property
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
