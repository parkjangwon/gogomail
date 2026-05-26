package httpapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/apikeys"
	"github.com/gogomail/gogomail/internal/auth"
	"github.com/gogomail/gogomail/internal/drive"
	"github.com/gogomail/gogomail/internal/maildb"
	"github.com/gogomail/gogomail/internal/mailservice"
	"github.com/gogomail/gogomail/internal/ratelimit"
)

const (
	maxProfileAvatarBytes  = 256 * 1024
	maxJSONBodyBytes       = 1 << 20
	maxHTTPAuthHeaderBytes = 16 << 10
	maxHTTPControlBytes    = 32
	maxHTTPResourceIDBytes = 200
	maxHTTPUserEmailBytes  = 320
	maxHTTPQueryBytes      = 1024
)

func rejectBodylessRequestPayload(w http.ResponseWriter, r *http.Request) bool {
	if r.ContentLength != 0 || len(r.TransferEncoding) > 0 {
		writeError(w, http.StatusBadRequest, "request body is not supported")
		return false
	}
	if len(r.Header.Values("Content-Type")) > 0 {
		writeError(w, http.StatusBadRequest, "content-type is not supported without a request body")
		return false
	}
	return true
}

type MessageService interface {
	ListFolders(ctx context.Context, userID string) ([]maildb.Folder, error)
	CreateFolder(ctx context.Context, req maildb.CreateFolderRequest) (maildb.Folder, error)
	RenameFolder(ctx context.Context, userID string, folderID string, name string) (maildb.Folder, error)
	DeleteFolder(ctx context.Context, userID string, folderID string) error
	ListMessages(ctx context.Context, userID string, limit int) ([]maildb.MessageSummary, error)
	ListMessagesInFolder(ctx context.Context, userID string, folderID string, limit int) ([]maildb.MessageSummary, error)
	ListMessagesPage(ctx context.Context, userID string, folderID string, limit int, cursor maildb.MessageListCursor, filter maildb.MessageListFilter) ([]maildb.MessageSummary, error)
	ListThreads(ctx context.Context, userID string, limit int) ([]maildb.ThreadSummary, error)
	ListThreadsPage(ctx context.Context, userID string, limit int, cursor maildb.ThreadListCursor, filter maildb.ThreadListFilter) ([]maildb.ThreadSummary, error)
	ListThreadMessages(ctx context.Context, userID string, threadID string, limit int) ([]maildb.MessageSummary, error)
	ListThreadMessagesPage(ctx context.Context, userID string, threadID string, limit int, cursor maildb.MessageListCursor) ([]maildb.MessageSummary, error)
	SearchMessages(ctx context.Context, query maildb.MessageSearchQuery) ([]maildb.MessageSummary, error)
	SearchDrafts(ctx context.Context, query maildb.DraftSearchQuery) ([]maildb.MessageDetail, error)
	GetMessage(ctx context.Context, userID string, messageID string) (maildb.MessageDetail, error)
	SetMessageFlag(ctx context.Context, userID string, messageID string, flag string, value bool) error
	BulkSetMessageFlag(ctx context.Context, req maildb.BulkMessageFlagRequest) (int64, error)
	BulkSetThreadFlag(ctx context.Context, req maildb.BulkThreadFlagRequest) (int64, error)
	MoveMessage(ctx context.Context, userID string, messageID string, folderID string) error
	BulkMoveMessages(ctx context.Context, req maildb.BulkMessageMoveRequest) (int64, error)
	BulkMoveThreads(ctx context.Context, req maildb.BulkThreadMoveRequest) (int64, error)
	DeleteMessage(ctx context.Context, userID string, messageID string) error
	BulkDeleteMessages(ctx context.Context, req maildb.BulkMessageDeleteRequest) (int64, error)
	BulkDeleteThreads(ctx context.Context, req maildb.BulkThreadDeleteRequest) (int64, error)
	RestoreMessage(ctx context.Context, userID string, messageID string) error
	BulkRestoreMessages(ctx context.Context, req maildb.BulkMessageRestoreRequest) (int64, error)
	BulkRestoreThreads(ctx context.Context, req maildb.BulkThreadRestoreRequest) (int64, error)
	ListPushDevices(ctx context.Context, userID string, limit int) ([]maildb.PushDevice, error)
	UpsertPushDevice(ctx context.Context, req maildb.UpsertPushDeviceRequest) (maildb.PushDevice, error)
	DeletePushDevice(ctx context.Context, userID string, id string) error
	UpsertWebPushSubscription(ctx context.Context, req maildb.UpsertWebPushSubscriptionRequest) (maildb.WebPushSubscription, error)
	ListActiveWebPushSubscriptions(ctx context.Context, userID string) ([]maildb.WebPushSubscription, error)
	DeleteWebPushSubscription(ctx context.Context, userID, id string) error
	SaveDraft(ctx context.Context, req mailservice.SaveDraftRequest) (maildb.MessageDetail, error)
	DeleteDraft(ctx context.Context, userID string, draftID string) error
	SendDraft(ctx context.Context, userID string, draftID string) (mailservice.SendTextResult, error)
	CreateAttachmentUpload(ctx context.Context, req mailservice.CreateAttachmentUploadRequest) (maildb.Attachment, error)
	UploadAttachment(ctx context.Context, req mailservice.UploadAttachmentRequest) (maildb.Attachment, error)
	CancelAttachmentUpload(ctx context.Context, userID string, attachmentID string) (maildb.Attachment, error)
	CreateAttachmentUploadSession(ctx context.Context, req mailservice.CreateAttachmentUploadSessionRequest) (maildb.AttachmentUploadSession, error)
	CancelAttachmentUploadSession(ctx context.Context, userID string, sessionID string) (maildb.AttachmentUploadSession, error)
	GetAttachmentUploadSession(ctx context.Context, userID string, sessionID string) (maildb.AttachmentUploadSession, error)
	StoreAttachmentUploadSessionBody(ctx context.Context, req mailservice.StoreAttachmentUploadSessionBodyRequest) (maildb.AttachmentUploadSession, error)
	FinalizeAttachmentUploadSession(ctx context.Context, userID string, sessionID string) (maildb.Attachment, error)
	ListAttachments(ctx context.Context, userID string, messageID string) ([]maildb.Attachment, error)
	OpenAttachment(ctx context.Context, userID string, messageID string, attachmentID string) (mailservice.AttachmentDownload, error)
	StatAttachment(ctx context.Context, userID string, messageID string, attachmentID string) (mailservice.AttachmentMetadata, error)
	SendText(ctx context.Context, req mailservice.SendTextRequest) (mailservice.SendTextResult, error)
	MessageDeliveryStatus(ctx context.Context, userID string, messageID string) (maildb.MessageDeliveryStatusView, error)
	GetWebmailPreferences(ctx context.Context, userID string) (json.RawMessage, error)
	SetWebmailPreferences(ctx context.Context, userID string, prefs json.RawMessage) error
	ListUserMCPAccessKeys(ctx context.Context, userID string) ([]maildb.UserMCPAccessKey, error)
	CreateUserMCPAccessKey(ctx context.Context, req maildb.CreateUserMCPAccessKeyRequest) (maildb.CreatedUserMCPAccessKey, error)
	RevokeUserMCPAccessKey(ctx context.Context, userID, id string) (maildb.UserMCPAccessKey, error)
	GetUserMCPDomainPolicy(ctx context.Context, userID string) (maildb.DomainMCPPolicy, error)
	GetUserProfile(ctx context.Context, userID string) (maildb.UserProfile, error)
	GetUserProfileByEmail(ctx context.Context, email string) (maildb.UserProfile, error)
	ListUserAddresses(ctx context.Context, userID string) ([]maildb.UserAddress, error)
	UpdateUserDisplayName(ctx context.Context, userID, displayName string) error
	UpdateUserAvatarURL(ctx context.Context, userID, avatarURL string) error
	UpdateOwnRecoveryEmail(ctx context.Context, userID, recoveryEmail string) error
	ChangeUserPassword(ctx context.Context, userID, currentPassword, newPassword string) error
}

type webmailCapabilitiesEnvelope struct {
	WebmailCapabilities webmailCapabilities `json:"webmail_capabilities"`
}

func profileAvatarDataURL(contentType string, data []byte) (string, error) {
	contentType = strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	detected := strings.ToLower(http.DetectContentType(data))
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = detected
	}
	switch contentType {
	case "image/png", "image/jpeg", "image/gif", "image/webp":
	default:
		return "", fmt.Errorf("avatar must be a PNG, JPEG, GIF, or WebP image")
	}
	if detected != "application/octet-stream" && !strings.HasPrefix(detected, "image/") {
		return "", fmt.Errorf("avatar content is not an image")
	}
	return "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

func readProfileAvatarUpload(w http.ResponseWriter, r *http.Request) (string, error) {
	r.Body = http.MaxBytesReader(w, r.Body, maxProfileAvatarBytes+4096)
	if err := r.ParseMultipartForm(maxProfileAvatarBytes + 4096); err != nil {
		return "", fmt.Errorf("invalid avatar upload")
	}
	file, header, err := r.FormFile("avatar")
	if err != nil {
		return "", fmt.Errorf("avatar file is required")
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxProfileAvatarBytes+1))
	if err != nil {
		return "", fmt.Errorf("read avatar upload: %w", err)
	}
	if len(data) == 0 {
		return "", fmt.Errorf("avatar file is empty")
	}
	if len(data) > maxProfileAvatarBytes {
		return "", fmt.Errorf("avatar file is too large")
	}
	contentType := header.Header.Get("Content-Type")
	return profileAvatarDataURL(contentType, data)
}

type UserAuthenticator interface {
	AuthenticateUser(ctx context.Context, email, password string) (maildb.AuthenticatedUser, error)
}

type UserRefreshTokenStore interface {
	CreateUserRefreshToken(ctx context.Context, userID string) (string, error)
	RotateUserRefreshToken(ctx context.Context, token string) (maildb.RotatedUserRefreshToken, error)
}

// MFAStore is the subset of maildb.Repository methods used by the MFA login flow.
type MFAStore interface {
	GetUserMFAStatus(ctx context.Context, userID string) (maildb.UserMFAStatus, error)
	GetMFASecret(ctx context.Context, userID string) (secret string, recoveryCodes []string, err error)
	GetPendingMFASecret(ctx context.Context, userID string) (secret string, err error)
	VerifyAndRecordTOTP(ctx context.Context, userID, secret, code string, now time.Time) error
	VerifyAndConsumeRecoveryCode(ctx context.Context, userID, code string) error
	SetupMFASecret(ctx context.Context, userID, secret string, recoveryCodes []string) error
	ActivateMFA(ctx context.Context, userID string) error
	DisableMFA(ctx context.Context, userID string) error
}

// ConfigResolver resolves runtime config values for MFA policy lookup.
type ConfigResolver interface {
	Resolve(ctx context.Context, userID, domainID, companyID, key string) (json.RawMessage, error)
}

type MailRouteOptions struct {
	MutationLimiter   MailMutationLimiter
	SessionRevoker    auth.SessionRevoker
	Authenticator     UserAuthenticator
	MFAStore          MFAStore
	ConfigResolver    ConfigResolver
	RefreshTokenStore UserRefreshTokenStore
	// LoginLimiter guards POST /api/v1/auth/token against brute-force attacks.
	// When nil a default in-process limiter of 10 attempts per IP per minute is used.
	LoginLimiter *AdminIPRateLimiter
	// APIKeyLimiter guards API-key-authenticated mutation endpoints against abuse.
	// When nil a default in-process limiter of 120 requests per API key per minute is used.
	APIKeyLimiter *AdminIPRateLimiter
	// WebPushVAPIDPublicKey is the VAPID public key exposed to clients via
	// GET /api/v1/config/web-push. When empty the endpoint returns null.
	WebPushVAPIDPublicKey string
}

type MailMutationLimiter interface {
	Allow(ctx context.Context, key string) (ratelimit.Decision, error)
}

type mailboxOverviewEnvelope struct {
	MailboxOverview mailboxOverview `json:"mailbox_overview"`
}

type mailboxOverview struct {
	TotalMessages   int64             `json:"total_messages"`
	UnreadMessages  int64             `json:"unread_messages"`
	StarredMessages int64             `json:"starred_messages"`
	TotalSizeBytes  int64             `json:"total_size_bytes"`
	SystemFolders   map[string]string `json:"system_folders"`
}

type webmailCapabilities struct {
	ContractVersion       string                        `json:"contract_version"`
	Modules               map[string]string             `json:"modules"`
	MaxListLimit          int                           `json:"max_list_limit"`
	SupportedMessageFlags []string                      `json:"supported_message_flags"`
	BulkActions           webmailBulkActionCapabilities `json:"bulk_actions"`
	MailboxActions        webmailMailboxCapabilities    `json:"mailbox_actions"`
	Compose               webmailComposeCapabilities    `json:"compose"`
	Search                webmailSearchCapabilities     `json:"search"`
	Attachments           webmailAttachmentCapabilities `json:"attachments"`
	Drive                 webmailDriveCapabilities      `json:"drive"`
	PushNotifications     webmailPushCapabilities       `json:"push_notifications"`
}

type webmailBulkActionCapabilities struct {
	MaxMessageIDs int  `json:"max_message_ids"`
	Flags         bool `json:"flags"`
	ThreadFlags   bool `json:"thread_flags"`
	Move          bool `json:"move"`
	ThreadMove    bool `json:"thread_move"`
	Delete        bool `json:"delete"`
	ThreadDelete  bool `json:"thread_delete"`
	Restore       bool `json:"restore"`
	ThreadRestore bool `json:"thread_restore"`
}

type webmailMailboxCapabilities struct {
	Folders bool `json:"folders"`
	Threads bool `json:"threads"`
	Drafts  bool `json:"drafts"`
}

type webmailComposeCapabilities struct {
	Intents              []string `json:"intents"`
	MaxRecipients        int      `json:"max_recipients"`
	MaxSubjectBytes      int      `json:"max_subject_bytes"`
	MaxTextBodyBytes     int      `json:"max_text_body_bytes"`
	MaxAttachmentIDs     int      `json:"max_attachment_ids"`
	ReplyForwardSources  bool     `json:"reply_forward_sources"`
	DomainPolicyEnforced bool     `json:"domain_policy_enforced"`
}

type webmailSearchCapabilities struct {
	Messages       bool     `json:"messages"`
	Drafts         bool     `json:"drafts"`
	Filters        []string `json:"filters"`
	Highlights     bool     `json:"highlights"`
	OpaqueCursors  bool     `json:"opaque_cursors"`
	MaxQueryBytes  int      `json:"max_query_bytes"`
	MaxFilterBytes int      `json:"max_filter_bytes"`
}

type webmailAttachmentCapabilities struct {
	MaxAttachmentBytes      int64 `json:"max_attachment_bytes"`
	MaxFilenameBytes        int   `json:"max_filename_bytes"`
	MetadataReservation     bool  `json:"metadata_reservation"`
	DirectMultipartUpload   bool  `json:"direct_multipart_upload"`
	UploadSessions          bool  `json:"upload_sessions"`
	UploadSessionBody       bool  `json:"upload_session_body"`
	UploadSessionChecksum   bool  `json:"upload_session_checksum"`
	FinalizeUploadSessions  bool  `json:"finalize_upload_sessions"`
	CancelPendingUploads    bool  `json:"cancel_pending_uploads"`
	CancelUploadSessions    bool  `json:"cancel_upload_sessions"`
	ResumableChunkedUploads bool  `json:"resumable_chunked_uploads"`
	QuotaReservedOnMetadata bool  `json:"quota_reserved_on_metadata"`
	RequiresDeclaredSize    bool  `json:"requires_declared_size"`
	MaxSessionTTLSeconds    int64 `json:"max_session_ttl_seconds"`
}

type webmailDriveCapabilities struct {
	Nodes                      bool     `json:"nodes"`
	NodeNameSearch             bool     `json:"node_name_search"`
	NodeAllParentsSearch       bool     `json:"node_all_parents_search"`
	NodeTypeFilter             bool     `json:"node_type_filter"`
	SupportedNodeTypes         []string `json:"supported_node_types"`
	NodeSortControls           bool     `json:"node_sort_controls"`
	SupportedNodeSorts         []string `json:"supported_node_sorts"`
	NodeDetail                 bool     `json:"node_detail"`
	NodeDownload               bool     `json:"node_download"`
	NodeRangeDownload          bool     `json:"node_range_download"`
	CopyNodes                  bool     `json:"copy_nodes"`
	MaxCopyNodes               int      `json:"max_copy_nodes"`
	ShareLinks                 bool     `json:"share_links"`
	ShareLinkPermissions       []string `json:"share_link_permissions"`
	MaxShareLinkTTLSeconds     int64    `json:"max_share_link_ttl_seconds"`
	DefaultShareLinkTTLSeconds int64    `json:"default_share_link_ttl_seconds"`
	UsageSummary               bool     `json:"usage_summary"`
	CreateFolders              bool     `json:"create_folders"`
	RenameNodes                bool     `json:"rename_nodes"`
	MoveNodes                  bool     `json:"move_nodes"`
	TrashRestore               bool     `json:"trash_restore"`
	PermanentDelete            bool     `json:"permanent_delete"`
	UploadSessions             bool     `json:"upload_sessions"`
	ListUploadSessions         bool     `json:"list_upload_sessions"`
	UploadSessionBody          bool     `json:"upload_session_body"`
	UploadSessionChecksum      bool     `json:"upload_session_checksum"`
	FinalizeUploadSessions     bool     `json:"finalize_upload_sessions"`
	CancelUploadSessions       bool     `json:"cancel_upload_sessions"`
	ResumableChunkedUploads    bool     `json:"resumable_chunked_uploads"`
	MaxUploadSessionBytes      int64    `json:"max_upload_session_bytes"`
	MaxSessionTTLSeconds       int64    `json:"max_session_ttl_seconds"`
	DefaultSessionTTLSeconds   int64    `json:"default_session_ttl_seconds"`
}

type webmailPushCapabilities struct {
	Devices   bool     `json:"devices"`
	Platforms []string `json:"platforms"`
}

func buildMailboxOverview(folders []maildb.Folder) mailboxOverview {
	overview := mailboxOverview{
		SystemFolders: make(map[string]string),
	}
	for _, folder := range folders {
		overview.TotalMessages += folder.Total
		overview.UnreadMessages += folder.Unread
		overview.StarredMessages += folder.Starred
		overview.TotalSizeBytes += folder.TotalSize
		if folder.SystemType != "" {
			overview.SystemFolders[folder.SystemType] = folder.ID
		}
	}
	return overview
}

func currentWebmailCapabilities() webmailCapabilities {
	return webmailCapabilities{
		ContractVersion: BackendContractVersion,
		Modules: map[string]string{
			"mail":  "available",
			"drive": "available",
		},
		MaxListLimit:          maildb.MessageListMaxLimit,
		SupportedMessageFlags: []string{"read", "starred", "answered", "forwarded"},
		BulkActions: webmailBulkActionCapabilities{
			MaxMessageIDs: maildb.BulkMessageMaxIDs,
			Flags:         true,
			ThreadFlags:   true,
			Move:          true,
			ThreadMove:    true,
			Delete:        true,
			ThreadDelete:  true,
			Restore:       true,
			ThreadRestore: true,
		},
		MailboxActions: webmailMailboxCapabilities{
			Folders: true,
			Threads: true,
			Drafts:  true,
		},
		Compose: webmailComposeCapabilities{
			Intents:              []string{string(mailservice.ComposeIntentNew), string(mailservice.ComposeIntentReply), string(mailservice.ComposeIntentForward)},
			MaxRecipients:        mailservice.MaxComposeRecipients,
			MaxSubjectBytes:      mailservice.MaxComposeSubjectBytes,
			MaxTextBodyBytes:     mailservice.MaxComposeTextBodyBytes,
			MaxAttachmentIDs:     mailservice.MaxComposeAttachments,
			ReplyForwardSources:  true,
			DomainPolicyEnforced: true,
		},
		Search: webmailSearchCapabilities{
			Messages:       true,
			Drafts:         true,
			Filters:        []string{"q", "folder_id", "from", "to", "cc", "bcc", "subject", "has_attachment"},
			Highlights:     true,
			OpaqueCursors:  true,
			MaxQueryBytes:  maxHTTPQueryBytes,
			MaxFilterBytes: maxHTTPQueryBytes,
		},
		Attachments: webmailAttachmentCapabilities{
			MaxAttachmentBytes:      mailservice.MaxAttachmentUploadBytes,
			MaxFilenameBytes:        mailservice.MaxAttachmentFilenameBytes,
			MetadataReservation:     true,
			DirectMultipartUpload:   true,
			UploadSessions:          true,
			UploadSessionBody:       true,
			UploadSessionChecksum:   true,
			FinalizeUploadSessions:  true,
			CancelPendingUploads:    true,
			CancelUploadSessions:    true,
			ResumableChunkedUploads: false,
			QuotaReservedOnMetadata: true,
			RequiresDeclaredSize:    true,
			MaxSessionTTLSeconds:    int64(mailservice.MaxAttachmentUploadSessionTTL.Seconds()),
		},
		Drive: webmailDriveCapabilities{
			Nodes:                      true,
			NodeNameSearch:             true,
			NodeAllParentsSearch:       true,
			NodeTypeFilter:             true,
			SupportedNodeTypes:         []string{drive.NodeTypeFolder, drive.NodeTypeFile},
			NodeSortControls:           true,
			SupportedNodeSorts:         []string{drive.NodeSortName, drive.NodeSortUpdated, drive.NodeSortCreated, drive.NodeSortSize},
			NodeDetail:                 true,
			NodeDownload:               true,
			NodeRangeDownload:          true,
			CopyNodes:                  true,
			MaxCopyNodes:               drive.MaxDriveCopyNodes,
			ShareLinks:                 true,
			ShareLinkPermissions:       []string{drive.ShareLinkPermissionView, drive.ShareLinkPermissionDownload},
			MaxShareLinkTTLSeconds:     int64(drive.MaxShareLinkTTL.Seconds()),
			DefaultShareLinkTTLSeconds: int64(drive.DefaultShareLinkTTL.Seconds()),
			UsageSummary:               true,
			CreateFolders:              true,
			RenameNodes:                true,
			MoveNodes:                  true,
			TrashRestore:               true,
			PermanentDelete:            true,
			UploadSessions:             true,
			ListUploadSessions:         true,
			UploadSessionBody:          true,
			UploadSessionChecksum:      true,
			FinalizeUploadSessions:     true,
			CancelUploadSessions:       true,
			ResumableChunkedUploads:    true,
			MaxUploadSessionBytes:      drive.MaxUploadSessionBytes,
			MaxSessionTTLSeconds:       int64(drive.MaxUploadSessionTTL.Seconds()),
			DefaultSessionTTLSeconds:   int64(drive.DefaultUploadSessionTTL.Seconds()),
		},
		PushNotifications: webmailPushCapabilities{
			Devices:   true,
			Platforms: []string{"apns", "fcm", "webpush"},
		},
	}
}

func allowMailMutationRequest(w http.ResponseWriter, r *http.Request, opts MailRouteOptions, userID string, action string) bool {
	if opts.MutationLimiter == nil {
		return true
	}
	key := mailMutationRateLimitKey(userID, action)
	decision, err := opts.MutationLimiter.Allow(r.Context(), key)
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
	writeError(w, http.StatusTooManyRequests, "mail mutation rate limit exceeded")
	return false
}

func mailMutationRateLimitKey(userID string, action string) string {
	return "user=" + userID + " action=" + strings.ToLower(strings.TrimSpace(action))
}

// allowAPIKeyRequest enforces per-API-key rate limiting for mutation endpoints.
// Returns true to continue, false when the limit is exceeded (response already written).
func allowAPIKeyRequest(w http.ResponseWriter, r *http.Request, limiter *AdminIPRateLimiter) bool {
	if limiter == nil {
		return true
	}
	info, ok := apikeys.KeyInfoFromContext(r.Context())
	if !ok || info == nil || strings.TrimSpace(info.ID) == "" {
		return true
	}
	if !limiter.allow(info.ID) {
		writeError(w, http.StatusTooManyRequests, "api key rate limit exceeded")
		return false
	}
	return true
}

var userMCPAPIKeyMutationLimiter = NewAdminIPRateLimiter(120, time.Minute)

func RegisterMailRoutes(mux *http.ServeMux, service MessageService, tokenManager *auth.TokenManager) {
	RegisterMailRoutesWithOptions(mux, service, tokenManager, MailRouteOptions{})
}


func RegisterMailRoutesWithOptions(mux *http.ServeMux, service MessageService, tokenManager *auth.TokenManager, opts MailRouteOptions) {
	if opts.LoginLimiter == nil {
		opts.LoginLimiter = NewAdminIPRateLimiter(10, time.Minute)
	}
	if opts.APIKeyLimiter == nil {
		opts.APIKeyLimiter = NewAdminIPRateLimiter(120, time.Minute)
	}
	registerAuthRoutes(mux, service, tokenManager, opts)
	registerFolderRoutes(mux, service, tokenManager, opts)
	registerMessageRoutes(mux, service, tokenManager, opts)
	registerThreadRoutes(mux, service, tokenManager, opts)
	registerDraftRoutes(mux, service, tokenManager, opts)
	registerAttachmentRoutes(mux, service, tokenManager, opts)
	registerPushRoutes(mux, service, tokenManager, opts)
	registerProfileRoutes(mux, service, tokenManager, opts)
}
