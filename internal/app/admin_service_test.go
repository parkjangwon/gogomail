package app

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/audit"
	"github.com/gogomail/gogomail/internal/backpressure"
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
	sessions []drive.UploadSession
	lastReq  drive.ListUploadSessionsRequest
}

func (f *fakeAdminDrive) ListUploadSessions(_ context.Context, req drive.ListUploadSessionsRequest) ([]drive.UploadSession, error) {
	f.lastReq = req
	return f.sessions, nil
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
