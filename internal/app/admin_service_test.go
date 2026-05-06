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

func TestAdminServiceCreateDirectoryDelegationUsesAuditedDirectoryBoundary(t *testing.T) {
	t.Parallel()

	directoryStore := &fakeAdminDirectory{
		delegation: directory.Delegation{ID: "delegation-1", CompanyID: "company-1"},
	}
	service := adminService{directory: directoryStore}
	req := directory.CreateDelegationRequest{
		CompanyID:    " company-1 ",
		OwnerKind:    directory.PrincipalKindResource,
		OwnerID:      "room-1",
		DelegateKind: directory.PrincipalKindGroup,
		DelegateID:   "team-1",
		Scope:        directory.DelegationScopeCalendar,
		Role:         directory.DelegationRoleWrite,
	}
	delegation, err := service.CreateDirectoryDelegation(t.Context(), req)
	if err != nil {
		t.Fatalf("CreateDirectoryDelegation returned error: %v", err)
	}
	if delegation.ID != "delegation-1" {
		t.Fatalf("delegation = %+v", delegation)
	}
	if directoryStore.lastDelegationCreateReq != req {
		t.Fatalf("lastDelegationCreateReq = %+v, want %+v", directoryStore.lastDelegationCreateReq, req)
	}
	if directoryStore.createDelegationWithAuditCalls != 1 {
		t.Fatalf("createDelegationWithAuditCalls = %d, want 1", directoryStore.createDelegationWithAuditCalls)
	}
}

func TestAdminServiceCreateDirectoryGroupMembershipUsesAuditedDirectoryBoundary(t *testing.T) {
	t.Parallel()

	directoryStore := &fakeAdminDirectory{
		membership: directory.GroupMembership{ID: "membership-1", Status: "active"},
	}
	service := adminService{directory: directoryStore}
	req := directory.CreateGroupMembershipRequest{
		GroupID:    " group-1 ",
		MemberKind: directory.PrincipalKindUser,
		MemberID:   "user-1",
		Role:       directory.GroupMembershipRoleManager,
	}
	membership, err := service.CreateDirectoryGroupMembership(t.Context(), req)
	if err != nil {
		t.Fatalf("CreateDirectoryGroupMembership returned error: %v", err)
	}
	if membership.ID != "membership-1" {
		t.Fatalf("membership = %+v", membership)
	}
	if directoryStore.lastMembershipCreateReq != req {
		t.Fatalf("lastMembershipCreateReq = %+v, want %+v", directoryStore.lastMembershipCreateReq, req)
	}
	if directoryStore.createMembershipWithAuditCalls != 1 {
		t.Fatalf("createMembershipWithAuditCalls = %d, want 1", directoryStore.createMembershipWithAuditCalls)
	}
}

func TestAdminServiceListDirectoryGroupMembershipsDelegatesToDirectory(t *testing.T) {
	t.Parallel()

	directoryStore := &fakeAdminDirectory{
		memberships: []directory.GroupMembership{{ID: "membership-1", Status: "active"}},
	}
	service := adminService{directory: directoryStore}
	req := directory.ListGroupMembershipsRequest{
		CompanyID:  " company-1 ",
		GroupID:    "group-1",
		MemberKind: directory.PrincipalKindUser,
		MemberID:   "user-1",
		Role:       directory.GroupMembershipRoleManager,
		ActiveOnly: true,
		Limit:      10,
	}
	memberships, err := service.ListDirectoryGroupMemberships(t.Context(), req)
	if err != nil {
		t.Fatalf("ListDirectoryGroupMemberships returned error: %v", err)
	}
	if len(memberships) != 1 || memberships[0].ID != "membership-1" {
		t.Fatalf("memberships = %+v", memberships)
	}
	if directoryStore.lastMembershipListReq != req {
		t.Fatalf("lastMembershipListReq = %+v, want %+v", directoryStore.lastMembershipListReq, req)
	}
}

func TestAdminServiceDeleteDirectoryDelegationUsesAuditedDirectoryBoundary(t *testing.T) {
	t.Parallel()

	directoryStore := &fakeAdminDirectory{
		delegation: directory.Delegation{ID: "delegation-1", Status: "deleted"},
	}
	service := adminService{directory: directoryStore}
	delegation, err := service.DeleteDirectoryDelegation(t.Context(), " delegation-1 ")
	if err != nil {
		t.Fatalf("DeleteDirectoryDelegation returned error: %v", err)
	}
	if delegation.ID != "delegation-1" || delegation.Status != "deleted" {
		t.Fatalf("delegation = %+v", delegation)
	}
	if directoryStore.lastDelegationDeleteID != " delegation-1 " {
		t.Fatalf("lastDelegationDeleteID = %q", directoryStore.lastDelegationDeleteID)
	}
	if directoryStore.deleteDelegationWithAuditCalls != 1 {
		t.Fatalf("deleteDelegationWithAuditCalls = %d, want 1", directoryStore.deleteDelegationWithAuditCalls)
	}
}

func TestAdminServiceDeleteDirectoryGroupMembershipUsesAuditedDirectoryBoundary(t *testing.T) {
	t.Parallel()

	directoryStore := &fakeAdminDirectory{
		membership: directory.GroupMembership{ID: "membership-1", Status: "deleted"},
	}
	service := adminService{directory: directoryStore}
	membership, err := service.DeleteDirectoryGroupMembership(t.Context(), " membership-1 ")
	if err != nil {
		t.Fatalf("DeleteDirectoryGroupMembership returned error: %v", err)
	}
	if membership.ID != "membership-1" || membership.Status != "deleted" {
		t.Fatalf("membership = %+v", membership)
	}
	if directoryStore.lastMembershipDeleteID != " membership-1 " {
		t.Fatalf("lastMembershipDeleteID = %q", directoryStore.lastMembershipDeleteID)
	}
	if directoryStore.deleteMembershipWithAuditCalls != 1 {
		t.Fatalf("deleteMembershipWithAuditCalls = %d, want 1", directoryStore.deleteMembershipWithAuditCalls)
	}
}

func TestAdminServiceUpdateDirectoryDelegationRoleUsesAuditedDirectoryBoundary(t *testing.T) {
	t.Parallel()

	directoryStore := &fakeAdminDirectory{
		delegation: directory.Delegation{ID: "delegation-1", Role: directory.DelegationRoleManage, Status: "active"},
	}
	service := adminService{directory: directoryStore}
	req := directory.UpdateDelegationRoleRequest{
		ID:   " delegation-1 ",
		Role: directory.DelegationRoleManage,
	}
	delegation, err := service.UpdateDirectoryDelegationRole(t.Context(), req)
	if err != nil {
		t.Fatalf("UpdateDirectoryDelegationRole returned error: %v", err)
	}
	if delegation.ID != "delegation-1" || delegation.Role != directory.DelegationRoleManage {
		t.Fatalf("delegation = %+v", delegation)
	}
	if directoryStore.lastDelegationRoleUpdateReq != req {
		t.Fatalf("lastDelegationRoleUpdateReq = %+v, want %+v", directoryStore.lastDelegationRoleUpdateReq, req)
	}
	if directoryStore.updateDelegationRoleWithAuditCalls != 1 {
		t.Fatalf("updateDelegationRoleWithAuditCalls = %d, want 1", directoryStore.updateDelegationRoleWithAuditCalls)
	}
}

func TestAdminServiceUpdateDirectoryGroupMembershipRoleUsesAuditedDirectoryBoundary(t *testing.T) {
	t.Parallel()

	directoryStore := &fakeAdminDirectory{
		membership: directory.GroupMembership{ID: "membership-1", Role: directory.GroupMembershipRoleOwner, Status: "active"},
	}
	service := adminService{directory: directoryStore}
	req := directory.UpdateGroupMembershipRoleRequest{
		ID:   " membership-1 ",
		Role: directory.GroupMembershipRoleOwner,
	}
	membership, err := service.UpdateDirectoryGroupMembershipRole(t.Context(), req)
	if err != nil {
		t.Fatalf("UpdateDirectoryGroupMembershipRole returned error: %v", err)
	}
	if membership.ID != "membership-1" || membership.Role != directory.GroupMembershipRoleOwner {
		t.Fatalf("membership = %+v", membership)
	}
	if directoryStore.lastMembershipRoleUpdateReq != req {
		t.Fatalf("lastMembershipRoleUpdateReq = %+v, want %+v", directoryStore.lastMembershipRoleUpdateReq, req)
	}
	if directoryStore.updateMembershipRoleWithAuditCalls != 1 {
		t.Fatalf("updateMembershipRoleWithAuditCalls = %d, want 1", directoryStore.updateMembershipRoleWithAuditCalls)
	}
}

func TestAdminServiceReassignDirectoryGroupMembershipUsesAuditedDirectoryBoundary(t *testing.T) {
	t.Parallel()

	directoryStore := &fakeAdminDirectory{
		membership: directory.GroupMembership{ID: "membership-1", GroupID: "group-2", MemberID: "user-2", Status: "active"},
	}
	service := adminService{directory: directoryStore}
	req := directory.ReassignGroupMembershipRequest{
		ID:         " membership-1 ",
		GroupID:    "group-2",
		MemberKind: directory.PrincipalKindUser,
		MemberID:   "user-2",
	}
	membership, err := service.ReassignDirectoryGroupMembership(t.Context(), req)
	if err != nil {
		t.Fatalf("ReassignDirectoryGroupMembership returned error: %v", err)
	}
	if membership.ID != "membership-1" || membership.GroupID != "group-2" || membership.MemberID != "user-2" {
		t.Fatalf("membership = %+v", membership)
	}
	if directoryStore.lastMembershipReassignReq != req {
		t.Fatalf("lastMembershipReassignReq = %+v, want %+v", directoryStore.lastMembershipReassignReq, req)
	}
	if directoryStore.reassignMembershipWithAuditCalls != 1 {
		t.Fatalf("reassignMembershipWithAuditCalls = %d, want 1", directoryStore.reassignMembershipWithAuditCalls)
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

func TestAdminServiceResolveDirectoryAliasDelegatesToDirectory(t *testing.T) {
	t.Parallel()

	directoryStore := &fakeAdminDirectory{
		alias: directory.Alias{ID: "alias-1", Address: "ops@example.com", TargetKind: directory.PrincipalKindGroup, TargetID: "group-1"},
	}
	service := adminService{directory: directoryStore}
	req := directory.ResolveAliasRequest{Address: " Ops@Example.COM ", ActiveOnly: true}
	alias, err := service.ResolveDirectoryAlias(t.Context(), req)
	if err != nil {
		t.Fatalf("ResolveDirectoryAlias returned error: %v", err)
	}
	if alias.ID != "alias-1" {
		t.Fatalf("alias = %+v", alias)
	}
	if directoryStore.lastAliasReq != req {
		t.Fatalf("lastAliasReq = %+v, want %+v", directoryStore.lastAliasReq, req)
	}
}

func TestAdminServiceListDirectoryAliasesDelegatesToDirectory(t *testing.T) {
	t.Parallel()

	directoryStore := &fakeAdminDirectory{
		aliases: []directory.Alias{{ID: "alias-1", Address: "ops@example.com"}},
	}
	service := adminService{directory: directoryStore}
	req := directory.ListAliasesRequest{
		CompanyID:  " company-1 ",
		DomainID:   " domain-1 ",
		TargetKind: directory.PrincipalKindGroup,
		TargetID:   "group-1",
		Query:      " ops ",
		ActiveOnly: true,
		Limit:      5,
	}
	aliases, err := service.ListDirectoryAliases(t.Context(), req)
	if err != nil {
		t.Fatalf("ListDirectoryAliases returned error: %v", err)
	}
	if len(aliases) != 1 || aliases[0].ID != "alias-1" {
		t.Fatalf("aliases = %+v", aliases)
	}
	if directoryStore.lastAliasListReq != req {
		t.Fatalf("lastAliasListReq = %+v, want %+v", directoryStore.lastAliasListReq, req)
	}
}

func TestAdminServiceCreateDirectoryAliasUsesAuditedDirectoryBoundary(t *testing.T) {
	t.Parallel()

	directoryStore := &fakeAdminDirectory{
		alias: directory.Alias{ID: "alias-1", Address: "ops@example.com"},
	}
	service := adminService{directory: directoryStore}
	req := directory.CreateAliasRequest{
		CompanyID:  " company-1 ",
		DomainID:   " domain-1 ",
		Address:    " Ops@Example.COM ",
		TargetKind: directory.PrincipalKindGroup,
		TargetID:   "group-1",
	}
	alias, err := service.CreateDirectoryAlias(t.Context(), req)
	if err != nil {
		t.Fatalf("CreateDirectoryAlias returned error: %v", err)
	}
	if alias.ID != "alias-1" {
		t.Fatalf("alias = %+v", alias)
	}
	if directoryStore.lastAliasCreateReq != req {
		t.Fatalf("lastAliasCreateReq = %+v, want %+v", directoryStore.lastAliasCreateReq, req)
	}
	if directoryStore.createAliasWithAuditCalls != 1 {
		t.Fatalf("createAliasWithAuditCalls = %d, want 1", directoryStore.createAliasWithAuditCalls)
	}
}

func TestAdminServiceDeleteDirectoryAliasUsesAuditedDirectoryBoundary(t *testing.T) {
	t.Parallel()

	directoryStore := &fakeAdminDirectory{
		alias: directory.Alias{ID: "alias-1", Status: "deleted"},
	}
	service := adminService{directory: directoryStore}
	alias, err := service.DeleteDirectoryAlias(t.Context(), " alias-1 ")
	if err != nil {
		t.Fatalf("DeleteDirectoryAlias returned error: %v", err)
	}
	if alias.ID != "alias-1" || alias.Status != "deleted" {
		t.Fatalf("alias = %+v", alias)
	}
	if directoryStore.lastAliasDeleteID != " alias-1 " {
		t.Fatalf("lastAliasDeleteID = %q", directoryStore.lastAliasDeleteID)
	}
	if directoryStore.deleteAliasWithAuditCalls != 1 {
		t.Fatalf("deleteAliasWithAuditCalls = %d, want 1", directoryStore.deleteAliasWithAuditCalls)
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
	delegations                        []directory.Delegation
	delegation                         directory.Delegation
	memberships                        []directory.GroupMembership
	membership                         directory.GroupMembership
	alias                              directory.Alias
	aliases                            []directory.Alias
	principals                         []directory.Principal
	lastReq                            directory.ListDelegationsRequest
	lastDelegationCreateReq            directory.CreateDelegationRequest
	lastDelegationDeleteID             string
	lastDelegationRoleUpdateReq        directory.UpdateDelegationRoleRequest
	lastMembershipCreateReq            directory.CreateGroupMembershipRequest
	lastMembershipListReq              directory.ListGroupMembershipsRequest
	lastMembershipDeleteID             string
	lastMembershipRoleUpdateReq        directory.UpdateGroupMembershipRoleRequest
	lastMembershipReassignReq          directory.ReassignGroupMembershipRequest
	lastAliasReq                       directory.ResolveAliasRequest
	lastAliasCreateReq                 directory.CreateAliasRequest
	lastAliasDeleteID                  string
	lastAliasListReq                   directory.ListAliasesRequest
	lastSearchReq                      directory.SearchPrincipalsRequest
	createAliasWithAuditCalls          int
	createDelegationWithAuditCalls     int
	createMembershipWithAuditCalls     int
	deleteDelegationWithAuditCalls     int
	deleteMembershipWithAuditCalls     int
	deleteAliasWithAuditCalls          int
	updateDelegationRoleWithAuditCalls int
	updateMembershipRoleWithAuditCalls int
	reassignMembershipWithAuditCalls   int
}

func (f *fakeAdminDirectory) ListDelegations(_ context.Context, req directory.ListDelegationsRequest) ([]directory.Delegation, error) {
	f.lastReq = req
	return f.delegations, nil
}

func (f *fakeAdminDirectory) CreateDelegationWithAudit(_ context.Context, req directory.CreateDelegationRequest) (directory.Delegation, error) {
	f.createDelegationWithAuditCalls++
	f.lastDelegationCreateReq = req
	return f.delegation, nil
}

func (f *fakeAdminDirectory) CreateGroupMembershipWithAudit(_ context.Context, req directory.CreateGroupMembershipRequest) (directory.GroupMembership, error) {
	f.createMembershipWithAuditCalls++
	f.lastMembershipCreateReq = req
	return f.membership, nil
}

func (f *fakeAdminDirectory) ListGroupMemberships(_ context.Context, req directory.ListGroupMembershipsRequest) ([]directory.GroupMembership, error) {
	f.lastMembershipListReq = req
	return f.memberships, nil
}

func (f *fakeAdminDirectory) DeleteDelegationWithAudit(_ context.Context, id string) (directory.Delegation, error) {
	f.deleteDelegationWithAuditCalls++
	f.lastDelegationDeleteID = id
	return f.delegation, nil
}

func (f *fakeAdminDirectory) DeleteGroupMembershipWithAudit(_ context.Context, id string) (directory.GroupMembership, error) {
	f.deleteMembershipWithAuditCalls++
	f.lastMembershipDeleteID = id
	return f.membership, nil
}

func (f *fakeAdminDirectory) UpdateDelegationRoleWithAudit(_ context.Context, req directory.UpdateDelegationRoleRequest) (directory.Delegation, error) {
	f.updateDelegationRoleWithAuditCalls++
	f.lastDelegationRoleUpdateReq = req
	return f.delegation, nil
}

func (f *fakeAdminDirectory) UpdateGroupMembershipRoleWithAudit(_ context.Context, req directory.UpdateGroupMembershipRoleRequest) (directory.GroupMembership, error) {
	f.updateMembershipRoleWithAuditCalls++
	f.lastMembershipRoleUpdateReq = req
	return f.membership, nil
}

func (f *fakeAdminDirectory) ReassignGroupMembershipWithAudit(_ context.Context, req directory.ReassignGroupMembershipRequest) (directory.GroupMembership, error) {
	f.reassignMembershipWithAuditCalls++
	f.lastMembershipReassignReq = req
	return f.membership, nil
}

func (f *fakeAdminDirectory) SearchPrincipals(_ context.Context, req directory.SearchPrincipalsRequest) ([]directory.Principal, error) {
	f.lastSearchReq = req
	return f.principals, nil
}

func (f *fakeAdminDirectory) ResolveAlias(_ context.Context, req directory.ResolveAliasRequest) (directory.Alias, error) {
	f.lastAliasReq = req
	return f.alias, nil
}

func (f *fakeAdminDirectory) ListAliases(_ context.Context, req directory.ListAliasesRequest) ([]directory.Alias, error) {
	f.lastAliasListReq = req
	return f.aliases, nil
}

func (f *fakeAdminDirectory) CreateAliasWithAudit(_ context.Context, req directory.CreateAliasRequest) (directory.Alias, error) {
	f.createAliasWithAuditCalls++
	f.lastAliasCreateReq = req
	return f.alias, nil
}

func (f *fakeAdminDirectory) DeleteAliasWithAudit(_ context.Context, id string) (directory.Alias, error) {
	f.deleteAliasWithAuditCalls++
	f.lastAliasDeleteID = id
	return f.alias, nil
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
