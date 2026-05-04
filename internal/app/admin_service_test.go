package app

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/audit"
	"github.com/gogomail/gogomail/internal/backpressure"
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
