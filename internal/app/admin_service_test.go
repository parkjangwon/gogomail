package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/audit"
	"github.com/gogomail/gogomail/internal/backpressure"
	"github.com/gogomail/gogomail/internal/directory"
	"github.com/gogomail/gogomail/internal/drive"
	"github.com/gogomail/gogomail/internal/maildb"
)

func TestAdminServiceUpdateBackpressureRecordsAudit(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 5, 1, 2, 3, 0, time.UTC)
	previousUntil := now.Add(time.Hour)
	currentUntil := now.Add(2 * time.Hour)
	store := &fakeBackpressureStore{
		state: backpressure.State{
			Level:     "warning",
			Reason:    "queue lag",
			Until:     &previousUntil,
			UpdatedAt: now,
		},
		updated: backpressure.State{
			Level:     "danger",
			Reason:    "queue lag above threshold",
			Until:     &currentUntil,
			UpdatedAt: now.Add(time.Minute),
		},
	}
	writer := &fakeAuditWriter{}
	service := adminService{backpressure: store, audit: writer}

	state, err := service.UpdateBackpressure(t.Context(), backpressure.StateUpdate{
		Level:  "danger",
		Reason: "queue lag above threshold",
		Until:  &currentUntil,
	})
	if err != nil {
		t.Fatalf("UpdateBackpressure returned error: %v", err)
	}
	if state.Level != "danger" || store.setCalls != 1 || writer.insertCalls != 1 {
		t.Fatalf("state=%+v setCalls=%d insertCalls=%d", state, store.setCalls, writer.insertCalls)
	}
	if writer.log.Category != "admin" || writer.log.Action != "backpressure.update" || writer.log.TargetType != "backpressure" || writer.log.Result != "updated" {
		t.Fatalf("audit log identity = %+v", writer.log)
	}

	var detail struct {
		Scope    string `json:"scope"`
		Previous struct {
			Level  string `json:"level"`
			Reason string `json:"reason"`
		} `json:"previous"`
		Current struct {
			Level  string `json:"level"`
			Reason string `json:"reason"`
		} `json:"current"`
	}
	if err := json.Unmarshal(writer.log.Detail, &detail); err != nil {
		t.Fatalf("unmarshal audit detail: %v", err)
	}
	if detail.Scope != "smtp" || detail.Previous.Level != "warning" || detail.Current.Level != "danger" {
		t.Fatalf("audit detail = %+v", detail)
	}
	if detail.Current.Reason != "queue lag above threshold" {
		t.Fatalf("current reason = %q", detail.Current.Reason)
	}
}

func TestAdminServiceUpdateBackpressureReturnsAuditFailure(t *testing.T) {
	t.Parallel()

	store := &fakeBackpressureStore{
		state:   backpressure.State{Level: "normal"},
		updated: backpressure.State{Level: "critical"},
	}
	writer := &fakeAuditWriter{err: errors.New("audit unavailable")}
	service := adminService{backpressure: store, audit: writer}

	_, err := service.UpdateBackpressure(t.Context(), backpressure.StateUpdate{Level: "critical"})
	if err == nil {
		t.Fatal("UpdateBackpressure accepted unaudited backpressure update")
	}
	if !strings.Contains(err.Error(), "record backpressure audit") {
		t.Fatalf("error = %v, want audit context", err)
	}
	if store.setCalls != 1 || writer.insertCalls != 1 {
		t.Fatalf("setCalls=%d insertCalls=%d", store.setCalls, writer.insertCalls)
	}
}

func TestBackpressureAuditDetailBoundsLegacyReason(t *testing.T) {
	t.Parallel()

	detail, err := backpressureAuditDetail(
		backpressure.State{Level: "warning", Reason: strings.Repeat("p", 600)},
		backpressure.State{Level: "danger", Reason: strings.Repeat("c", 600)},
	)
	if err != nil {
		t.Fatalf("backpressureAuditDetail returned error: %v", err)
	}
	if len(detail) > 1300 {
		t.Fatalf("audit detail length = %d, want bounded detail", len(detail))
	}
	var decoded struct {
		Previous struct {
			Reason string `json:"reason"`
		} `json:"previous"`
		Current struct {
			Reason string `json:"reason"`
		} `json:"current"`
	}
	if err := json.Unmarshal(detail, &decoded); err != nil {
		t.Fatalf("unmarshal audit detail: %v", err)
	}
	if len(decoded.Previous.Reason) != 512 || len(decoded.Current.Reason) != 512 {
		t.Fatalf("reason lengths = %d/%d, want 512/512", len(decoded.Previous.Reason), len(decoded.Current.Reason))
	}
}

func TestAdminServiceRunAttachmentCleanupRecordsAudit(t *testing.T) {
	t.Parallel()

	before := time.Date(2026, 5, 5, 1, 2, 3, 0, time.UTC)
	cleanup := &fakeAdminAttachmentCleanup{
		expiredUploads: []maildb.Attachment{{ID: "att-1"}, {ID: "att-2"}},
	}
	writer := &fakeAuditWriter{}
	service := adminService{attachmentCleanup: cleanup, audit: writer}

	expired, err := service.RunAttachmentCleanup(t.Context(), before, 25)
	if err != nil {
		t.Fatalf("RunAttachmentCleanup returned error: %v", err)
	}
	if len(expired) != 2 || writer.insertCalls != 1 {
		t.Fatalf("expired=%d insertCalls=%d", len(expired), writer.insertCalls)
	}
	if writer.log.Action != "attachment_cleanup.uploads_run" || writer.log.TargetType != "attachment_cleanup" || writer.log.Result != "completed" {
		t.Fatalf("audit log = %+v", writer.log)
	}
	var detail struct {
		Scope        string   `json:"scope"`
		ExpiredCount int      `json:"expired_count"`
		ExpiredIDs   []string `json:"expired_ids_sample"`
	}
	if err := json.Unmarshal(writer.log.Detail, &detail); err != nil {
		t.Fatalf("unmarshal audit detail: %v", err)
	}
	if detail.Scope != "uploads" || detail.ExpiredCount != 2 || len(detail.ExpiredIDs) != 2 {
		t.Fatalf("audit detail = %+v", detail)
	}
}

func TestAdminServiceRunAttachmentSessionCleanupRecordsAudit(t *testing.T) {
	t.Parallel()

	before := time.Date(2026, 5, 5, 1, 2, 3, 0, time.UTC)
	cleanup := &fakeAdminAttachmentCleanup{
		expiredSessions: []maildb.AttachmentUploadSession{{ID: "session-1"}},
	}
	writer := &fakeAuditWriter{}
	service := adminService{attachmentCleanup: cleanup, audit: writer}

	expired, err := service.RunAttachmentUploadSessionCleanup(t.Context(), before, 25)
	if err != nil {
		t.Fatalf("RunAttachmentUploadSessionCleanup returned error: %v", err)
	}
	if len(expired) != 1 || writer.insertCalls != 1 {
		t.Fatalf("expired=%d insertCalls=%d", len(expired), writer.insertCalls)
	}
	if writer.log.Action != "attachment_cleanup.sessions_run" || writer.log.TargetType != "attachment_cleanup" || writer.log.Result != "completed" {
		t.Fatalf("audit log = %+v", writer.log)
	}
	var detail struct {
		Scope        string   `json:"scope"`
		ExpiredCount int      `json:"expired_count"`
		ExpiredIDs   []string `json:"expired_ids_sample"`
	}
	if err := json.Unmarshal(writer.log.Detail, &detail); err != nil {
		t.Fatalf("unmarshal audit detail: %v", err)
	}
	if detail.Scope != "upload_sessions" || detail.ExpiredCount != 1 || detail.ExpiredIDs[0] != "session-1" {
		t.Fatalf("audit detail = %+v", detail)
	}
}

func TestAdminServiceListDriveUploadSessionsDelegatesToDrive(t *testing.T) {
	t.Parallel()

	driveStore := &fakeAdminDrive{
		sessions: []drive.UploadSession{{ID: "session-1", UserID: "user-1"}},
	}
	service := adminService{drive: driveStore}
	req := drive.ListUploadSessionsRequest{UserID: " user-1 ", Status: " uploading ", Limit: 5}
	sessions, err := service.ListDriveUploadSessions(t.Context(), req)
	if err != nil {
		t.Fatalf("ListDriveUploadSessions returned error: %v", err)
	}
	if len(sessions) != 1 || sessions[0].ID != "session-1" {
		t.Fatalf("sessions = %+v", sessions)
	}
	if driveStore.lastReq.UserID != "user-1" || driveStore.lastReq.Status != drive.UploadSessionStatusUploading || driveStore.lastReq.Limit != 5 {
		t.Fatalf("lastReq = %+v", driveStore.lastReq)
	}
}

func TestAdminServiceListDirectoryDelegationsDelegatesToDirectory(t *testing.T) {
	t.Parallel()

	directoryStore := &fakeAdminDirectory{
		delegations: []directory.Delegation{{ID: "delegation-1", CompanyID: "company-1"}},
	}
	service := adminService{directory: directoryStore}
	req := directory.ListDelegationsRequest{
		CompanyID:    " company-1 ",
		OwnerKind:    directory.PrincipalKindResource,
		OwnerID:      "room-1",
		DelegateKind: directory.PrincipalKindGroup,
		DelegateID:   "team-1",
		Scope:        directory.DelegationScopeCalendar,
		Role:         directory.DelegationRoleWrite,
		ActiveOnly:   true,
		Limit:        5,
	}
	delegations, err := service.ListDirectoryDelegations(t.Context(), req)
	if err != nil {
		t.Fatalf("ListDirectoryDelegations returned error: %v", err)
	}
	if len(delegations) != 1 || delegations[0].ID != "delegation-1" {
		t.Fatalf("delegations = %+v", delegations)
	}
	if directoryStore.lastReq != req {
		t.Fatalf("lastReq = %+v, want %+v", directoryStore.lastReq, req)
	}
}

func TestAdminServiceSearchDirectoryPrincipalsDelegatesToDirectory(t *testing.T) {
	t.Parallel()

	directoryStore := &fakeAdminDirectory{
		principals: []directory.Principal{{ID: "user-1", Kind: directory.PrincipalKindUser, CompanyID: "company-1"}},
	}
	service := adminService{directory: directoryStore}
	req := directory.SearchPrincipalsRequest{
		CompanyID:  " company-1 ",
		Kinds:      []string{directory.PrincipalKindUser, directory.PrincipalKindResource},
		Query:      " Alice ",
		ActiveOnly: true,
		Limit:      5,
	}
	principals, err := service.SearchDirectoryPrincipals(t.Context(), req)
	if err != nil {
		t.Fatalf("SearchDirectoryPrincipals returned error: %v", err)
	}
	if len(principals) != 1 || principals[0].ID != "user-1" {
		t.Fatalf("principals = %+v", principals)
	}
	if fmt.Sprintf("%+v", directoryStore.lastSearchReq) != fmt.Sprintf("%+v", req) {
		t.Fatalf("lastSearchReq = %+v, want %+v", directoryStore.lastSearchReq, req)
	}
}

func TestAdminServiceListDriveNodesDelegatesToDrive(t *testing.T) {
	t.Parallel()

	driveStore := &fakeAdminDrive{
		nodes: []drive.Node{{ID: "node-1", UserID: "user-1", Name: "Reports", Type: drive.NodeTypeFolder, Status: drive.NodeStatusActive}},
	}
	service := adminService{drive: driveStore}
	nodes, err := service.ListDriveNodes(t.Context(), drive.ListNodesRequest{
		UserID:   " user-1 ",
		ParentID: " parent-1 ",
		Status:   " active ",
		Query:    " Report ",
		Limit:    5,
	})
	if err != nil {
		t.Fatalf("ListDriveNodes returned error: %v", err)
	}
	if len(nodes) != 1 || nodes[0].ID != "node-1" {
		t.Fatalf("nodes = %+v", nodes)
	}
	if driveStore.lastNodeReq.UserID != "user-1" || driveStore.lastNodeReq.ParentID != "parent-1" || driveStore.lastNodeReq.Status != drive.NodeStatusActive || driveStore.lastNodeReq.Query != "report" || driveStore.lastNodeReq.Limit != 5 {
		t.Fatalf("lastNodeReq = %+v", driveStore.lastNodeReq)
	}
}

func TestAdminServiceGetDriveNodeDelegatesToDrive(t *testing.T) {
	t.Parallel()

	driveStore := &fakeAdminDrive{
		node: drive.Node{ID: "node-1", UserID: "user-1", Name: "Report.pdf", Type: drive.NodeTypeFile, Status: drive.NodeStatusActive},
	}
	service := adminService{drive: driveStore}
	node, err := service.GetDriveNode(t.Context(), drive.GetNodeRequest{
		UserID: " user-1 ",
		NodeID: " node-1 ",
		Status: " active ",
	})
	if err != nil {
		t.Fatalf("GetDriveNode returned error: %v", err)
	}
	if node.ID != "node-1" {
		t.Fatalf("node = %+v", node)
	}
	if driveStore.lastGetNodeReq.UserID != "user-1" || driveStore.lastGetNodeReq.NodeID != "node-1" || driveStore.lastGetNodeReq.Status != drive.NodeStatusActive {
		t.Fatalf("lastGetNodeReq = %+v", driveStore.lastGetNodeReq)
	}
}

func TestAdminServiceGetDriveUsageSummaryDelegatesToDrive(t *testing.T) {
	t.Parallel()

	driveStore := &fakeAdminDrive{
		usageSummary: drive.UsageSummary{UserID: "user-1", QuotaUsed: 1024, ActiveNodes: 3, ActiveBytes: 1024},
	}
	service := adminService{drive: driveStore}
	summary, err := service.GetDriveUsageSummary(t.Context(), drive.GetUsageSummaryRequest{UserID: " user-1 "})
	if err != nil {
		t.Fatalf("GetDriveUsageSummary returned error: %v", err)
	}
	if summary.UserID != "user-1" || summary.ActiveBytes != 1024 {
		t.Fatalf("summary = %+v", summary)
	}
	if driveStore.lastUsageReq.UserID != "user-1" {
		t.Fatalf("lastUsageReq = %+v", driveStore.lastUsageReq)
	}
}

func TestAdminServiceDriveUploadCleanupPreviewDelegatesToDrive(t *testing.T) {
	t.Parallel()

	before := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	driveStore := &fakeAdminDrive{
		count:    drive.StaleUploadSessionCount{TotalCount: 3, LimitedCount: 2},
		sessions: []drive.UploadSession{{ID: "session-1"}, {ID: "session-2"}},
	}
	service := adminService{drive: driveStore}
	count, err := service.CountStaleDriveUploadSessions(t.Context(), before, 2)
	if err != nil {
		t.Fatalf("CountStaleDriveUploadSessions returned error: %v", err)
	}
	if count.TotalCount != 3 || count.LimitedCount != 2 {
		t.Fatalf("count = %+v", count)
	}
	sessions, err := service.ListStaleDriveUploadSessions(t.Context(), before, 2)
	if err != nil {
		t.Fatalf("ListStaleDriveUploadSessions returned error: %v", err)
	}
	if len(sessions) != 2 || driveStore.lastCleanupReq.Limit != 2 || !driveStore.lastCleanupReq.Before.Equal(before) {
		t.Fatalf("sessions = %+v lastReq = %+v", sessions, driveStore.lastCleanupReq)
	}
}

func TestAdminServiceRunDriveUploadCleanupRecordsAudit(t *testing.T) {
	t.Parallel()

	before := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	driveStore := &fakeAdminDrive{
		sessions: []drive.UploadSession{{ID: "session-1"}},
	}
	writer := &fakeAuditWriter{}
	service := adminService{drive: driveStore, audit: writer}
	expired, err := service.RunDriveUploadSessionCleanup(t.Context(), before, 10)
	if err != nil {
		t.Fatalf("RunDriveUploadSessionCleanup returned error: %v", err)
	}
	if len(expired) != 1 || writer.insertCalls != 1 {
		t.Fatalf("expired=%d insertCalls=%d", len(expired), writer.insertCalls)
	}
	if writer.log.Action != "drive_upload_cleanup.sessions_run" || writer.log.TargetType != "drive_upload_cleanup" || writer.log.Result != "completed" {
		t.Fatalf("audit log = %+v", writer.log)
	}
	if driveStore.lastCleanupReq.Limit != 10 || !driveStore.lastCleanupReq.Before.Equal(before) {
		t.Fatalf("lastCleanupReq = %+v", driveStore.lastCleanupReq)
	}
}

func TestAdminServiceListDriveObjectCleanupFailuresDelegatesToDrive(t *testing.T) {
	t.Parallel()

	driveStore := &fakeAdminDrive{
		failures: []drive.ObjectCleanupFailure{{ID: "failure-1", UserID: "user-1", Status: drive.ObjectCleanupFailureStatusPending}},
	}
	service := adminService{drive: driveStore}
	failures, err := service.ListDriveObjectCleanupFailures(t.Context(), drive.ListObjectCleanupFailuresRequest{
		UserID: " user-1 ",
		Status: " pending ",
		Limit:  5,
	})
	if err != nil {
		t.Fatalf("ListDriveObjectCleanupFailures returned error: %v", err)
	}
	if len(failures) != 1 || failures[0].ID != "failure-1" {
		t.Fatalf("failures = %+v", failures)
	}
	if driveStore.lastFailureReq.UserID != "user-1" || driveStore.lastFailureReq.Status != drive.ObjectCleanupFailureStatusPending || driveStore.lastFailureReq.Limit != 5 {
		t.Fatalf("lastFailureReq = %+v", driveStore.lastFailureReq)
	}
}

func TestAdminServiceResolveDriveObjectCleanupFailureRecordsAudit(t *testing.T) {
	t.Parallel()

	driveStore := &fakeAdminDrive{
		resolvedFailure: drive.ObjectCleanupFailure{
			ID:             "failure-1",
			UserID:         "user-1",
			NodeID:         "node-1",
			StorageBackend: "s3",
			StoragePath:    "drive/users/user-1/files/node-1/body",
			Status:         drive.ObjectCleanupFailureStatusResolved,
			Attempts:       2,
		},
	}
	writer := &fakeAuditWriter{}
	service := adminService{drive: driveStore, audit: writer}
	resolved, err := service.ResolveDriveObjectCleanupFailure(t.Context(), " failure-1 ")
	if err != nil {
		t.Fatalf("ResolveDriveObjectCleanupFailure returned error: %v", err)
	}
	if resolved.ID != "failure-1" || driveStore.lastResolveReq.ID != "failure-1" {
		t.Fatalf("resolved=%+v lastReq=%+v", resolved, driveStore.lastResolveReq)
	}
	if writer.insertCalls != 1 || writer.log.Action != "drive_cleanup_failure.resolve" || writer.log.TargetID != "failure-1" {
		t.Fatalf("audit log = %+v insertCalls=%d", writer.log, writer.insertCalls)
	}
}

func TestAdminServiceRetryDriveObjectCleanupFailuresRecordsAudit(t *testing.T) {
	t.Parallel()

	driveStore := &fakeAdminDrive{
		retryResult: drive.RetryObjectCleanupFailuresResult{Scanned: 3, Deleted: 2, Resolved: 2, Failed: 1},
		retryErr:    fmt.Errorf("remaining cleanup failure"),
	}
	writer := &fakeAuditWriter{}
	service := adminService{drive: driveStore, audit: writer}
	result, err := service.RetryDriveObjectCleanupFailures(t.Context(), drive.ListObjectCleanupFailuresRequest{
		UserID: " user-1 ",
		Limit:  5,
	})
	if err != nil {
		t.Fatalf("RetryDriveObjectCleanupFailures returned error: %v", err)
	}
	if result.Failed != 1 || driveStore.lastRetryReq.UserID != "user-1" || driveStore.lastRetryReq.Status != drive.ObjectCleanupFailureStatusPending || driveStore.lastRetryReq.Limit != 5 {
		t.Fatalf("result=%+v lastReq=%+v", result, driveStore.lastRetryReq)
	}
	if writer.insertCalls != 1 || writer.log.Action != "drive_cleanup_failure.retry_run" || writer.log.Result != "partial" {
		t.Fatalf("audit log = %+v insertCalls=%d", writer.log, writer.insertCalls)
	}
}

func TestAttachmentCleanupAuditDetailSamplesIDs(t *testing.T) {
	t.Parallel()

	ids := make([]string, 0, maxAttachmentCleanupAuditSample+2)
	for i := 0; i < maxAttachmentCleanupAuditSample+2; i++ {
		ids = append(ids, "att-"+strconv.Itoa(i))
	}
	detail, err := attachmentCleanupAuditDetail("uploads", time.Date(2026, 5, 5, 1, 2, 3, 0, time.FixedZone("KST", 9*60*60)), 0, ids)
	if err != nil {
		t.Fatalf("attachmentCleanupAuditDetail returned error: %v", err)
	}
	var got struct {
		Before       string   `json:"before"`
		Limit        int      `json:"limit"`
		ExpiredCount int      `json:"expired_count"`
		ExpiredIDs   []string `json:"expired_ids_sample"`
	}
	if err := json.Unmarshal(detail, &got); err != nil {
		t.Fatalf("unmarshal audit detail: %v", err)
	}
	if got.Before != "2026-05-04T16:02:03Z" || got.Limit != maildb.AttachmentCleanupDefaultLimit || got.ExpiredCount != maxAttachmentCleanupAuditSample+2 || len(got.ExpiredIDs) != maxAttachmentCleanupAuditSample {
		t.Fatalf("audit detail = %+v", got)
	}
}

type fakeBackpressureStore struct {
	state    backpressure.State
	updated  backpressure.State
	stateErr error
	setErr   error
	setCalls int
}

func (f *fakeBackpressureStore) State(context.Context) (backpressure.State, error) {
	if f.stateErr != nil {
		return backpressure.State{}, f.stateErr
	}
	return f.state, nil
}

func (f *fakeBackpressureStore) SetState(_ context.Context, update backpressure.StateUpdate) (backpressure.State, error) {
	f.setCalls++
	if f.setErr != nil {
		return backpressure.State{}, f.setErr
	}
	if f.updated.Level == "" {
		return backpressure.State{Level: update.Level, Reason: update.Reason, Until: update.Until}, nil
	}
	return f.updated, nil
}

type fakeAuditWriter struct {
	log         audit.Log
	err         error
	insertCalls int
}

func (f *fakeAuditWriter) Insert(_ context.Context, log audit.Log) error {
	f.insertCalls++
	f.log = log
	return f.err
}

type fakeAdminAttachmentCleanup struct {
	expiredUploads  []maildb.Attachment
	expiredSessions []maildb.AttachmentUploadSession
	err             error
	sessionErr      error
}

type fakeAdminDrive struct {
	node            drive.Node
	nodes           []drive.Node
	usageSummary    drive.UsageSummary
	sessions        []drive.UploadSession
	count           drive.StaleUploadSessionCount
	failures        []drive.ObjectCleanupFailure
	resolvedFailure drive.ObjectCleanupFailure
	retryResult     drive.RetryObjectCleanupFailuresResult
	retryErr        error
	lastGetNodeReq  drive.GetNodeRequest
	lastNodeReq     drive.ListNodesRequest
	lastUsageReq    drive.GetUsageSummaryRequest
	lastReq         drive.ListUploadSessionsRequest
	lastCleanupReq  drive.ExpireUploadSessionsRequest
	lastFailureReq  drive.ListObjectCleanupFailuresRequest
	lastResolveReq  drive.ResolveObjectCleanupFailureRequest
	lastRetryReq    drive.ListObjectCleanupFailuresRequest
}

type fakeAdminDirectory struct {
	delegations   []directory.Delegation
	principals    []directory.Principal
	lastReq       directory.ListDelegationsRequest
	lastSearchReq directory.SearchPrincipalsRequest
}

func (f *fakeAdminDirectory) ListDelegations(_ context.Context, req directory.ListDelegationsRequest) ([]directory.Delegation, error) {
	f.lastReq = req
	return f.delegations, nil
}

func (f *fakeAdminDirectory) SearchPrincipals(_ context.Context, req directory.SearchPrincipalsRequest) ([]directory.Principal, error) {
	f.lastSearchReq = req
	return f.principals, nil
}

func (f *fakeAdminDrive) GetNode(_ context.Context, req drive.GetNodeRequest) (drive.Node, error) {
	f.lastGetNodeReq = req
	return f.node, nil
}

func (f *fakeAdminDrive) GetUsageSummary(_ context.Context, req drive.GetUsageSummaryRequest) (drive.UsageSummary, error) {
	f.lastUsageReq = req
	return f.usageSummary, nil
}

func (f *fakeAdminDrive) ListNodes(_ context.Context, req drive.ListNodesRequest) ([]drive.Node, error) {
	f.lastNodeReq = req
	return f.nodes, nil
}

func (f *fakeAdminDrive) ListUploadSessions(_ context.Context, req drive.ListUploadSessionsRequest) ([]drive.UploadSession, error) {
	f.lastReq = req
	return f.sessions, nil
}

func (f *fakeAdminDrive) CountStaleUploadSessions(_ context.Context, req drive.ExpireUploadSessionsRequest) (drive.StaleUploadSessionCount, error) {
	f.lastCleanupReq = req
	return f.count, nil
}

func (f *fakeAdminDrive) ListStaleUploadSessions(_ context.Context, req drive.ExpireUploadSessionsRequest) ([]drive.UploadSession, error) {
	f.lastCleanupReq = req
	return f.sessions, nil
}

func (f *fakeAdminDrive) ExpireUploadSessions(_ context.Context, req drive.ExpireUploadSessionsRequest) ([]drive.UploadSession, error) {
	f.lastCleanupReq = req
	return f.sessions, nil
}

func (f *fakeAdminDrive) ListObjectCleanupFailures(_ context.Context, req drive.ListObjectCleanupFailuresRequest) ([]drive.ObjectCleanupFailure, error) {
	f.lastFailureReq = req
	return f.failures, nil
}

func (f *fakeAdminDrive) ResolveObjectCleanupFailure(_ context.Context, req drive.ResolveObjectCleanupFailureRequest) (drive.ObjectCleanupFailure, error) {
	f.lastResolveReq = req
	return f.resolvedFailure, nil
}

func (f *fakeAdminDrive) RetryObjectCleanupFailures(_ context.Context, req drive.ListObjectCleanupFailuresRequest) (drive.RetryObjectCleanupFailuresResult, error) {
	f.lastRetryReq = req
	return f.retryResult, f.retryErr
}

func (f *fakeAdminAttachmentCleanup) ExpireStaleAttachmentUploads(context.Context, time.Time, int) ([]maildb.Attachment, error) {
	return f.expiredUploads, f.err
}

func (f *fakeAdminAttachmentCleanup) CountStaleAttachmentUploads(context.Context, time.Time, int) (maildb.StaleAttachmentUploadCount, error) {
	return maildb.StaleAttachmentUploadCount{}, nil
}

func (f *fakeAdminAttachmentCleanup) ListStaleAttachmentUploads(context.Context, time.Time, int) ([]maildb.StaleAttachmentUploadCandidate, error) {
	return nil, nil
}

func (f *fakeAdminAttachmentCleanup) ExpireAttachmentUploadSessions(context.Context, time.Time, int) ([]maildb.AttachmentUploadSession, error) {
	return f.expiredSessions, f.sessionErr
}

func (f *fakeAdminAttachmentCleanup) CountStaleAttachmentUploadSessions(context.Context, time.Time, int) (maildb.StaleAttachmentUploadSessionCount, error) {
	return maildb.StaleAttachmentUploadSessionCount{}, nil
}

func (f *fakeAdminAttachmentCleanup) ListStaleAttachmentUploadSessions(context.Context, time.Time, int) ([]maildb.StaleAttachmentUploadSessionCandidate, error) {
	return nil, nil
}
