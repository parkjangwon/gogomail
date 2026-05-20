package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/admin"
	"github.com/gogomail/gogomail/internal/davsyncretention"
	"github.com/gogomail/gogomail/internal/directory"
	ldapidp "github.com/gogomail/gogomail/internal/idprovider/ldap"
	rdbmsidp "github.com/gogomail/gogomail/internal/idprovider/rdbms"
	"github.com/gogomail/gogomail/internal/maildb"
)

// Query validation helpers - reject unknown query parameters for specific endpoints

func rejectUnknownAPIUsageAggregateQuery(w http.ResponseWriter, r *http.Request) bool {
	return rejectUnknownQueryKeys(w, r,
		"limit",
		"tenant_id",
		"company_id",
		"domain_id",
		"user_id",
		"api_key_id",
		"principal_id",
		"auth_source",
		"method",
		"route",
		"status",
		"from",
		"to",
	)
}

func rejectUnknownAPIUsageLedgerQuery(w http.ResponseWriter, r *http.Request) bool {
	return rejectUnknownQueryKeys(w, r, "limit", "tenant_id", "principal_id", "from", "to")
}

func rejectUnknownAPIUsageLedgerStatsQuery(w http.ResponseWriter, r *http.Request) bool {
	return rejectUnknownQueryKeys(w, r, "tenant_id", "principal_id", "from", "to")
}

func rejectUnknownAPIUsageRetentionReadinessQuery(w http.ResponseWriter, r *http.Request) bool {
	return rejectUnknownQueryKeys(w, r, "cutoff", "tenant_id", "principal_id")
}

func rejectUnknownAPIUsageRetentionRunListQuery(w http.ResponseWriter, r *http.Request) bool {
	return rejectUnknownQueryKeys(w, r, "limit", "tenant_id", "principal_id", "created_from", "created_to")
}

func rejectUnknownDAVSyncRetentionRunListQuery(w http.ResponseWriter, r *http.Request) bool {
	return rejectUnknownQueryKeys(w, r, "limit", "status", "created_from", "created_to")
}

func rejectUnknownDAVSyncRetentionReadinessQuery(w http.ResponseWriter, r *http.Request) bool {
	return rejectUnknownQueryKeys(w, r, "cutoff", "limit")
}

func rejectUnknownAPIUsageExportBatchCreateQuery(w http.ResponseWriter, r *http.Request) bool {
	return rejectUnknownQueryKeys(w, r, "tenant_id", "principal_id", "from", "to")
}

func rejectUnknownAPIUsageExportBatchListQuery(w http.ResponseWriter, r *http.Request) bool {
	return rejectUnknownQueryKeys(w, r, "limit", "tenant_id", "principal_id", "status", "from", "to")
}

// Parse request helpers - parse and validate HTTP request parameters

func parseAdminAttachmentCleanupRequest(w http.ResponseWriter, req adminAttachmentCleanupRunRequest) (time.Time, bool) {
	beforeRaw := strings.TrimSpace(req.Before)
	if beforeRaw == "" {
		writeError(w, http.StatusBadRequest, "before is required")
		return time.Time{}, false
	}
	before, err := time.Parse(time.RFC3339, beforeRaw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "before must be RFC3339 timestamp")
		return time.Time{}, false
	}
	before = before.UTC()
	if before.After(time.Now().UTC()) {
		writeError(w, http.StatusBadRequest, "before must not be in the future")
		return time.Time{}, false
	}
	if req.Limit < 0 {
		writeError(w, http.StatusBadRequest, "limit must not be negative")
		return time.Time{}, false
	}
	return before, true
}

func parseAPIUsageLedgerListRequest(w http.ResponseWriter, r *http.Request, limit int) (maildb.APIUsageLedgerListRequest, bool) {
	tenantID, ok := parseBoundedAdminQuery(w, r, "tenant_id")
	if !ok {
		return maildb.APIUsageLedgerListRequest{}, false
	}
	principalID, ok := parseBoundedAdminQuery(w, r, "principal_id")
	if !ok {
		return maildb.APIUsageLedgerListRequest{}, false
	}
	req := maildb.APIUsageLedgerListRequest{
		Limit:       limit,
		TenantID:    tenantID,
		PrincipalID: principalID,
	}
	from, ok := parseOptionalRFC3339Query(w, r, "from")
	if !ok {
		return maildb.APIUsageLedgerListRequest{}, false
	}
	to, ok := parseOptionalRFC3339Query(w, r, "to")
	if !ok {
		return maildb.APIUsageLedgerListRequest{}, false
	}
	req.From = from
	req.To = to
	if !req.From.IsZero() && !req.To.IsZero() && !req.From.Before(req.To) {
		writeError(w, http.StatusBadRequest, "from must be before to")
		return maildb.APIUsageLedgerListRequest{}, false
	}
	return req, true
}

func parseAPIUsageExportBatchListRequest(w http.ResponseWriter, r *http.Request, limit int) (maildb.APIUsageExportBatchListRequest, bool) {
	var ok bool
	req := maildb.APIUsageExportBatchListRequest{Limit: limit}
	if req.TenantID, ok = parseBoundedAdminQuery(w, r, "tenant_id"); !ok {
		return maildb.APIUsageExportBatchListRequest{}, false
	}
	if req.PrincipalID, ok = parseBoundedAdminQuery(w, r, "principal_id"); !ok {
		return maildb.APIUsageExportBatchListRequest{}, false
	}
	if req.Status, ok = parseBoundedAdminQuery(w, r, "status"); !ok {
		return maildb.APIUsageExportBatchListRequest{}, false
	}
	if req.From, ok = parseOptionalRFC3339Query(w, r, "from"); !ok {
		return maildb.APIUsageExportBatchListRequest{}, false
	}
	if req.To, ok = parseOptionalRFC3339Query(w, r, "to"); !ok {
		return maildb.APIUsageExportBatchListRequest{}, false
	}
	if err := maildb.ValidateAPIUsageExportBatchListRequest(req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return maildb.APIUsageExportBatchListRequest{}, false
	}
	return req, true
}

func parseAPIUsageAggregateListRequest(w http.ResponseWriter, r *http.Request, limit int) (maildb.APIUsageAggregateListRequest, bool) {
	req := maildb.APIUsageAggregateListRequest{Limit: limit}
	var ok bool
	if req.TenantID, ok = parseBoundedAdminQuery(w, r, "tenant_id"); !ok {
		return maildb.APIUsageAggregateListRequest{}, false
	}
	if req.CompanyID, ok = parseBoundedAdminQuery(w, r, "company_id"); !ok {
		return maildb.APIUsageAggregateListRequest{}, false
	}
	if req.DomainID, ok = parseBoundedAdminQuery(w, r, "domain_id"); !ok {
		return maildb.APIUsageAggregateListRequest{}, false
	}
	if req.UserID, ok = parseBoundedAdminQuery(w, r, "user_id"); !ok {
		return maildb.APIUsageAggregateListRequest{}, false
	}
	if req.APIKeyID, ok = parseBoundedAdminQuery(w, r, "api_key_id"); !ok {
		return maildb.APIUsageAggregateListRequest{}, false
	}
	if req.PrincipalID, ok = parseBoundedAdminQuery(w, r, "principal_id"); !ok {
		return maildb.APIUsageAggregateListRequest{}, false
	}
	if req.AuthSource, ok = parseBoundedAdminQuery(w, r, "auth_source"); !ok {
		return maildb.APIUsageAggregateListRequest{}, false
	}
	if req.Method, ok = parseBoundedAdminQuery(w, r, "method"); !ok {
		return maildb.APIUsageAggregateListRequest{}, false
	}
	if req.Route, ok = parseBoundedAdminQuery(w, r, "route"); !ok {
		return maildb.APIUsageAggregateListRequest{}, false
	}
	var statusOK bool
	if req.Status, statusOK = parseOptionalHTTPStatusQuery(w, r, "status"); !statusOK {
		return maildb.APIUsageAggregateListRequest{}, false
	}
	if req.From, ok = parseOptionalRFC3339Query(w, r, "from"); !ok {
		return maildb.APIUsageAggregateListRequest{}, false
	}
	if req.To, ok = parseOptionalRFC3339Query(w, r, "to"); !ok {
		return maildb.APIUsageAggregateListRequest{}, false
	}
	if !req.From.IsZero() && !req.To.IsZero() && !req.From.Before(req.To) {
		writeError(w, http.StatusBadRequest, "from must be before to")
		return maildb.APIUsageAggregateListRequest{}, false
	}
	return req, true
}

func parseOptionalHTTPStatusQuery(w http.ResponseWriter, r *http.Request, key string) (int, bool) {
	raw, ok := singleQueryValue(w, r, key)
	if !ok {
		return 0, false
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, true
	}
	status, err := strconv.Atoi(raw)
	if err != nil || status < 100 || status > 599 {
		writeError(w, http.StatusBadRequest, key+" must be an HTTP status code")
		return 0, false
	}
	return status, true
}

func parseAuditLogListRequest(w http.ResponseWriter, r *http.Request, limit int) (maildb.AuditLogListRequest, bool) {
	category, ok := parseBoundedAdminQuery(w, r, "category")
	if !ok {
		return maildb.AuditLogListRequest{}, false
	}
	action, ok := parseBoundedAdminQuery(w, r, "action")
	if !ok {
		return maildb.AuditLogListRequest{}, false
	}
	actionPrefix, ok := parseBoundedAdminQuery(w, r, "action_prefix")
	if !ok {
		return maildb.AuditLogListRequest{}, false
	}
	result, ok := parseBoundedAdminQuery(w, r, "result")
	if !ok {
		return maildb.AuditLogListRequest{}, false
	}
	targetType, ok := parseBoundedAdminQuery(w, r, "target_type")
	if !ok {
		return maildb.AuditLogListRequest{}, false
	}
	companyID, ok := parseBoundedAdminQuery(w, r, "company_id")
	if !ok {
		return maildb.AuditLogListRequest{}, false
	}
	domainID, ok := parseBoundedAdminQuery(w, r, "domain_id")
	if !ok {
		return maildb.AuditLogListRequest{}, false
	}
	userID, ok := parseBoundedAdminQuery(w, r, "user_id")
	if !ok {
		return maildb.AuditLogListRequest{}, false
	}
	actorID, ok := parseBoundedAdminQuery(w, r, "actor_id")
	if !ok {
		return maildb.AuditLogListRequest{}, false
	}
	targetID, ok := parseBoundedAdminQuery(w, r, "target_id")
	if !ok {
		return maildb.AuditLogListRequest{}, false
	}
	since, ok := parseOptionalRFC3339Query(w, r, "since")
	if !ok {
		return maildb.AuditLogListRequest{}, false
	}
	return maildb.AuditLogListRequest{
		Limit:        limit,
		Category:     category,
		Action:       action,
		ActionPrefix: actionPrefix,
		Result:       result,
		TargetType:   targetType,
		CompanyID:    companyID,
		DomainID:     domainID,
		UserID:       userID,
		ActorID:      actorID,
		TargetID:     targetID,
		Since:        since,
	}, true
}

func parseLoginAuditListRequest(w http.ResponseWriter, r *http.Request, limit int) (admin.LoginAuditFilter, bool) {
	userID, ok := parseBoundedAdminQuery(w, r, "user_id")
	if !ok {
		return admin.LoginAuditFilter{}, false
	}
	successRaw, ok := singleQueryValue(w, r, "success")
	if !ok {
		return admin.LoginAuditFilter{}, false
	}
	successRaw = strings.TrimSpace(successRaw)
	var success *bool
	if successRaw != "" {
		parsed, err := strconv.ParseBool(successRaw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "success must be true or false")
			return admin.LoginAuditFilter{}, false
		}
		success = &parsed
	}
	since, ok := parseOptionalRFC3339Query(w, r, "from_date")
	if !ok {
		return admin.LoginAuditFilter{}, false
	}
	until, ok := parseOptionalRFC3339Query(w, r, "to_date")
	if !ok {
		return admin.LoginAuditFilter{}, false
	}
	var startTime, endTime *time.Time
	if !since.IsZero() {
		value := since
		startTime = &value
	}
	if !until.IsZero() {
		value := until
		endTime = &value
	}
	offset := 0
	if raw, ok := singleQueryValue(w, r, "offset"); ok {
		raw = strings.TrimSpace(raw)
		if raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil || parsed < 0 {
				writeError(w, http.StatusBadRequest, "offset must be a non-negative integer")
				return admin.LoginAuditFilter{}, false
			}
			offset = parsed
		}
	}
	return admin.LoginAuditFilter{
		UserID:    userID,
		Success:   success,
		StartTime: startTime,
		EndTime:   endTime,
		Limit:     limit,
		Offset:    offset,
	}, true
}

func parseMailFlowLogListRequest(w http.ResponseWriter, r *http.Request, limit int) (maildb.MailFlowLogListRequest, bool) {
	direction, ok := parseBoundedAdminQuery(w, r, "direction")
	if !ok {
		return maildb.MailFlowLogListRequest{}, false
	}
	companyID, ok := parseBoundedAdminQuery(w, r, "company_id")
	if !ok {
		return maildb.MailFlowLogListRequest{}, false
	}
	domainID, ok := parseBoundedAdminQuery(w, r, "domain_id")
	if !ok {
		return maildb.MailFlowLogListRequest{}, false
	}
	userID, ok := parseBoundedAdminQuery(w, r, "user_id")
	if !ok {
		return maildb.MailFlowLogListRequest{}, false
	}
	messageID, ok := parseBoundedAdminQuery(w, r, "message_id")
	if !ok {
		return maildb.MailFlowLogListRequest{}, false
	}
	rfcMessageID, ok := parseBoundedAdminQuery(w, r, "rfc_message_id")
	if !ok {
		return maildb.MailFlowLogListRequest{}, false
	}
	fromAddr, ok := parseBoundedAdminQuery(w, r, "from_addr")
	if !ok {
		return maildb.MailFlowLogListRequest{}, false
	}
	toAddr, ok := parseBoundedAdminQuery(w, r, "to_addr")
	if !ok {
		return maildb.MailFlowLogListRequest{}, false
	}
	subject, ok := parseBoundedAdminQuery(w, r, "subject")
	if !ok {
		return maildb.MailFlowLogListRequest{}, false
	}
	flowStatus, ok := parseBoundedAdminQuery(w, r, "flow_status")
	if !ok {
		return maildb.MailFlowLogListRequest{}, false
	}
	since, ok := parseOptionalRFC3339Query(w, r, "since")
	if !ok {
		return maildb.MailFlowLogListRequest{}, false
	}
	until, ok := parseOptionalRFC3339Query(w, r, "until")
	if !ok {
		return maildb.MailFlowLogListRequest{}, false
	}
	return maildb.MailFlowLogListRequest{
		Limit:        limit,
		Direction:    direction,
		CompanyID:    companyID,
		DomainID:     domainID,
		UserID:       userID,
		MessageID:    messageID,
		RFCMessageID: rfcMessageID,
		FromAddr:     fromAddr,
		ToAddr:       toAddr,
		Subject:      subject,
		FlowStatus:   flowStatus,
		Since:        since,
		Until:        until,
	}, true
}

func parseMailFlowLogStatsRequest(w http.ResponseWriter, r *http.Request) (maildb.MailFlowLogStatsRequest, bool) {
	direction, ok := parseBoundedAdminQuery(w, r, "direction")
	if !ok {
		return maildb.MailFlowLogStatsRequest{}, false
	}
	companyID, ok := parseBoundedAdminQuery(w, r, "company_id")
	if !ok {
		return maildb.MailFlowLogStatsRequest{}, false
	}
	domainID, ok := parseBoundedAdminQuery(w, r, "domain_id")
	if !ok {
		return maildb.MailFlowLogStatsRequest{}, false
	}
	userID, ok := parseBoundedAdminQuery(w, r, "user_id")
	if !ok {
		return maildb.MailFlowLogStatsRequest{}, false
	}
	since, ok := parseOptionalRFC3339Query(w, r, "since")
	if !ok {
		return maildb.MailFlowLogStatsRequest{}, false
	}
	until, ok := parseOptionalRFC3339Query(w, r, "until")
	if !ok {
		return maildb.MailFlowLogStatsRequest{}, false
	}
	return maildb.MailFlowLogStatsRequest{
		Direction: direction,
		CompanyID: companyID,
		DomainID:  domainID,
		UserID:    userID,
		Since:     since,
		Until:     until,
	}, true
}

func parseMailFlowLogDailyStatsRequest(w http.ResponseWriter, r *http.Request) (maildb.MailFlowLogDailyStatsRequest, bool) {
	direction, ok := parseBoundedAdminQuery(w, r, "direction")
	if !ok {
		return maildb.MailFlowLogDailyStatsRequest{}, false
	}
	companyID, ok := parseBoundedAdminQuery(w, r, "company_id")
	if !ok {
		return maildb.MailFlowLogDailyStatsRequest{}, false
	}
	domainID, ok := parseBoundedAdminQuery(w, r, "domain_id")
	if !ok {
		return maildb.MailFlowLogDailyStatsRequest{}, false
	}
	userID, ok := parseBoundedAdminQuery(w, r, "user_id")
	if !ok {
		return maildb.MailFlowLogDailyStatsRequest{}, false
	}
	since, ok := parseOptionalRFC3339Query(w, r, "since")
	if !ok {
		return maildb.MailFlowLogDailyStatsRequest{}, false
	}
	until, ok := parseOptionalRFC3339Query(w, r, "until")
	if !ok {
		return maildb.MailFlowLogDailyStatsRequest{}, false
	}
	return maildb.MailFlowLogDailyStatsRequest{
		Direction: direction,
		CompanyID: companyID,
		DomainID:  domainID,
		UserID:    userID,
		Since:     since,
		Until:     until,
	}, true
}

func parseDirectoryPrincipalSearchRequest(w http.ResponseWriter, r *http.Request, limit int) (directory.SearchPrincipalsRequest, bool) {
	companyID, ok := parseBoundedAdminQuery(w, r, "company_id")
	if !ok {
		return directory.SearchPrincipalsRequest{}, false
	}
	domainID, ok := parseBoundedAdminQuery(w, r, "domain_id")
	if !ok {
		return directory.SearchPrincipalsRequest{}, false
	}
	organizationID, ok := parseBoundedAdminQuery(w, r, "organization_id")
	if !ok {
		return directory.SearchPrincipalsRequest{}, false
	}
	rawKinds, ok := parseBoundedAdminQuery(w, r, "kinds")
	if !ok {
		return directory.SearchPrincipalsRequest{}, false
	}
	query, ok := parseBoundedAdminQuery(w, r, "q")
	if !ok {
		return directory.SearchPrincipalsRequest{}, false
	}
	activeOnlyValue, ok := parseOptionalBoolQuery(w, r, "active_only")
	if !ok {
		return directory.SearchPrincipalsRequest{}, false
	}
	activeOnly := true
	if activeOnlyValue != nil {
		activeOnly = *activeOnlyValue
	}
	return directory.SearchPrincipalsRequest{
		CompanyID:      companyID,
		DomainID:       domainID,
		OrganizationID: organizationID,
		Kinds:          splitDirectoryPrincipalKinds(rawKinds),
		Query:          query,
		ActiveOnly:     activeOnly,
		Limit:          limit,
	}, true
}

func splitDirectoryPrincipalKinds(value string) []string {
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n' || r == '\r'
	})
	kinds := make([]string, 0, len(fields))
	for _, field := range fields {
		if field = strings.TrimSpace(field); field != "" {
			kinds = append(kinds, field)
		}
	}
	return kinds
}

func parseDirectoryAliasResolveRequest(w http.ResponseWriter, r *http.Request) (directory.ResolveAliasRequest, bool) {
	address, ok := parseBoundedAdminQuery(w, r, "address")
	if !ok {
		return directory.ResolveAliasRequest{}, false
	}
	activeOnlyValue, ok := parseOptionalBoolQuery(w, r, "active_only")
	if !ok {
		return directory.ResolveAliasRequest{}, false
	}
	activeOnly := true
	if activeOnlyValue != nil {
		activeOnly = *activeOnlyValue
	}
	return directory.ResolveAliasRequest{
		Address:    address,
		ActiveOnly: activeOnly,
	}, true
}

func parseDirectoryAliasListRequest(w http.ResponseWriter, r *http.Request, limit int) (directory.ListAliasesRequest, bool) {
	companyID, ok := parseBoundedAdminQuery(w, r, "company_id")
	if !ok {
		return directory.ListAliasesRequest{}, false
	}
	domainID, ok := parseBoundedAdminQuery(w, r, "domain_id")
	if !ok {
		return directory.ListAliasesRequest{}, false
	}
	targetKind, ok := parseBoundedAdminQuery(w, r, "target_kind")
	if !ok {
		return directory.ListAliasesRequest{}, false
	}
	targetID, ok := parseBoundedAdminQuery(w, r, "target_id")
	if !ok {
		return directory.ListAliasesRequest{}, false
	}
	query, ok := parseBoundedAdminQuery(w, r, "q")
	if !ok {
		return directory.ListAliasesRequest{}, false
	}
	activeOnlyValue, ok := parseOptionalBoolQuery(w, r, "active_only")
	if !ok {
		return directory.ListAliasesRequest{}, false
	}
	activeOnly := true
	if activeOnlyValue != nil {
		activeOnly = *activeOnlyValue
	}
	return directory.ListAliasesRequest{
		CompanyID:  companyID,
		DomainID:   domainID,
		TargetKind: targetKind,
		TargetID:   targetID,
		Query:      query,
		ActiveOnly: activeOnly,
		Limit:      limit,
	}, true
}

func parseDirectoryDelegationListRequest(w http.ResponseWriter, r *http.Request, limit int) (directory.ListDelegationsRequest, bool) {
	companyID, ok := parseBoundedAdminQuery(w, r, "company_id")
	if !ok {
		return directory.ListDelegationsRequest{}, false
	}
	ownerKind, ok := parseBoundedAdminQuery(w, r, "owner_kind")
	if !ok {
		return directory.ListDelegationsRequest{}, false
	}
	ownerID, ok := parseBoundedAdminQuery(w, r, "owner_id")
	if !ok {
		return directory.ListDelegationsRequest{}, false
	}
	delegateKind, ok := parseBoundedAdminQuery(w, r, "delegate_kind")
	if !ok {
		return directory.ListDelegationsRequest{}, false
	}
	delegateID, ok := parseBoundedAdminQuery(w, r, "delegate_id")
	if !ok {
		return directory.ListDelegationsRequest{}, false
	}
	scope, ok := parseBoundedAdminQuery(w, r, "scope")
	if !ok {
		return directory.ListDelegationsRequest{}, false
	}
	role, ok := parseBoundedAdminQuery(w, r, "role")
	if !ok {
		return directory.ListDelegationsRequest{}, false
	}
	activeOnlyValue, ok := parseOptionalBoolQuery(w, r, "active_only")
	if !ok {
		return directory.ListDelegationsRequest{}, false
	}
	activeOnly := true
	if activeOnlyValue != nil {
		activeOnly = *activeOnlyValue
	}
	return directory.ListDelegationsRequest{
		CompanyID:    companyID,
		OwnerKind:    ownerKind,
		OwnerID:      ownerID,
		DelegateKind: delegateKind,
		DelegateID:   delegateID,
		Scope:        scope,
		Role:         role,
		ActiveOnly:   activeOnly,
		Limit:        limit,
	}, true
}

func parseDirectoryGroupMembershipListRequest(w http.ResponseWriter, r *http.Request, limit int) (directory.ListGroupMembershipsRequest, bool) {
	companyID, ok := parseBoundedAdminQuery(w, r, "company_id")
	if !ok {
		return directory.ListGroupMembershipsRequest{}, false
	}
	groupID, ok := parseBoundedAdminQuery(w, r, "group_id")
	if !ok {
		return directory.ListGroupMembershipsRequest{}, false
	}
	memberKind, ok := parseBoundedAdminQuery(w, r, "member_kind")
	if !ok {
		return directory.ListGroupMembershipsRequest{}, false
	}
	memberID, ok := parseBoundedAdminQuery(w, r, "member_id")
	if !ok {
		return directory.ListGroupMembershipsRequest{}, false
	}
	role, ok := parseBoundedAdminQuery(w, r, "role")
	if !ok {
		return directory.ListGroupMembershipsRequest{}, false
	}
	activeOnlyValue, ok := parseOptionalBoolQuery(w, r, "active_only")
	if !ok {
		return directory.ListGroupMembershipsRequest{}, false
	}
	activeOnly := true
	if activeOnlyValue != nil {
		activeOnly = *activeOnlyValue
	}
	return directory.ListGroupMembershipsRequest{
		CompanyID:  companyID,
		GroupID:    groupID,
		MemberKind: memberKind,
		MemberID:   memberID,
		Role:       role,
		ActiveOnly: activeOnly,
		Limit:      limit,
	}, true
}

func parseAPIUsageLedgerRetentionRequest(w http.ResponseWriter, r *http.Request) (maildb.APIUsageLedgerRetentionRequest, bool) {
	tenantID, ok := parseBoundedAdminQuery(w, r, "tenant_id")
	if !ok {
		return maildb.APIUsageLedgerRetentionRequest{}, false
	}
	principalID, ok := parseBoundedAdminQuery(w, r, "principal_id")
	if !ok {
		return maildb.APIUsageLedgerRetentionRequest{}, false
	}
	cutoff, ok := parseOptionalRFC3339Query(w, r, "cutoff")
	if !ok {
		return maildb.APIUsageLedgerRetentionRequest{}, false
	}
	if cutoff.IsZero() {
		writeError(w, http.StatusBadRequest, "cutoff is required")
		return maildb.APIUsageLedgerRetentionRequest{}, false
	}
	if cutoff.After(time.Now().UTC()) {
		writeError(w, http.StatusBadRequest, "cutoff must not be in the future")
		return maildb.APIUsageLedgerRetentionRequest{}, false
	}
	return maildb.APIUsageLedgerRetentionRequest{
		Cutoff:      cutoff,
		TenantID:    tenantID,
		PrincipalID: principalID,
	}, true
}

func parseAPIUsageLedgerRetentionRunRequest(w http.ResponseWriter, req adminAPIUsageLedgerRetentionRunRequest) (maildb.APIUsageLedgerRetentionRunRequest, bool) {
	tenantID := strings.TrimSpace(req.TenantID)
	if strings.ContainsAny(tenantID, "\r\n") {
		writeError(w, http.StatusBadRequest, "tenant_id must not contain CR or LF")
		return maildb.APIUsageLedgerRetentionRunRequest{}, false
	}
	if len(tenantID) > maxAdminQueryFilterBytes {
		writeError(w, http.StatusBadRequest, "tenant_id is too long")
		return maildb.APIUsageLedgerRetentionRunRequest{}, false
	}
	principalID := strings.TrimSpace(req.PrincipalID)
	if strings.ContainsAny(principalID, "\r\n") {
		writeError(w, http.StatusBadRequest, "principal_id must not contain CR or LF")
		return maildb.APIUsageLedgerRetentionRunRequest{}, false
	}
	if len(principalID) > maxAdminQueryFilterBytes {
		writeError(w, http.StatusBadRequest, "principal_id is too long")
		return maildb.APIUsageLedgerRetentionRunRequest{}, false
	}
	cutoffRaw := strings.TrimSpace(req.Cutoff)
	if cutoffRaw == "" {
		writeError(w, http.StatusBadRequest, "cutoff is required")
		return maildb.APIUsageLedgerRetentionRunRequest{}, false
	}
	cutoff, err := time.Parse(time.RFC3339, cutoffRaw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "cutoff must be RFC3339 timestamp")
		return maildb.APIUsageLedgerRetentionRunRequest{}, false
	}
	cutoff = cutoff.UTC()
	if cutoff.After(time.Now().UTC()) {
		writeError(w, http.StatusBadRequest, "cutoff must not be in the future")
		return maildb.APIUsageLedgerRetentionRunRequest{}, false
	}
	if req.Limit < 0 {
		writeError(w, http.StatusBadRequest, "limit must not be negative")
		return maildb.APIUsageLedgerRetentionRunRequest{}, false
	}
	if !req.DryRun && !req.ConfirmReady {
		writeError(w, http.StatusBadRequest, "confirm_ready is required for destructive retention runs")
		return maildb.APIUsageLedgerRetentionRunRequest{}, false
	}
	return maildb.APIUsageLedgerRetentionRunRequest{
		Cutoff:       cutoff,
		TenantID:     tenantID,
		PrincipalID:  principalID,
		Limit:        req.Limit,
		DryRun:       req.DryRun,
		ConfirmReady: req.ConfirmReady,
	}, true
}

func parseAPIUsageLedgerRetentionRunListRequest(w http.ResponseWriter, r *http.Request, limit int) (maildb.APIUsageLedgerRetentionRunListRequest, bool) {
	tenantID, ok := parseBoundedAdminQuery(w, r, "tenant_id")
	if !ok {
		return maildb.APIUsageLedgerRetentionRunListRequest{}, false
	}
	principalID, ok := parseBoundedAdminQuery(w, r, "principal_id")
	if !ok {
		return maildb.APIUsageLedgerRetentionRunListRequest{}, false
	}
	createdFrom, ok := parseOptionalRFC3339Query(w, r, "created_from")
	if !ok {
		return maildb.APIUsageLedgerRetentionRunListRequest{}, false
	}
	createdTo, ok := parseOptionalRFC3339Query(w, r, "created_to")
	if !ok {
		return maildb.APIUsageLedgerRetentionRunListRequest{}, false
	}
	if !createdFrom.IsZero() && !createdTo.IsZero() && !createdFrom.Before(createdTo) {
		writeError(w, http.StatusBadRequest, "created_from must be before created_to")
		return maildb.APIUsageLedgerRetentionRunListRequest{}, false
	}
	return maildb.APIUsageLedgerRetentionRunListRequest{
		Limit:       limit,
		TenantID:    tenantID,
		PrincipalID: principalID,
		CreatedFrom: createdFrom,
		CreatedTo:   createdTo,
	}, true
}

func parseDAVSyncRetentionRunListRequest(w http.ResponseWriter, r *http.Request, limit int) (davsyncretention.RunListRequest, bool) {
	statusRaw, ok := parseBoundedAdminQuery(w, r, "status")
	if !ok {
		return davsyncretention.RunListRequest{}, false
	}
	status := davsyncretention.RunStatus(statusRaw)
	if status != "" && status != davsyncretention.RunStatusCompleted && status != davsyncretention.RunStatusFailed {
		writeError(w, http.StatusBadRequest, "status is unsupported")
		return davsyncretention.RunListRequest{}, false
	}
	createdFrom, ok := parseOptionalRFC3339Query(w, r, "created_from")
	if !ok {
		return davsyncretention.RunListRequest{}, false
	}
	createdTo, ok := parseOptionalRFC3339Query(w, r, "created_to")
	if !ok {
		return davsyncretention.RunListRequest{}, false
	}
	if !createdFrom.IsZero() && !createdTo.IsZero() && !createdFrom.Before(createdTo) {
		writeError(w, http.StatusBadRequest, "created_from must be before created_to")
		return davsyncretention.RunListRequest{}, false
	}
	return davsyncretention.RunListRequest{
		Limit:       limit,
		Status:      status,
		CreatedFrom: createdFrom,
		CreatedTo:   createdTo,
	}, true
}

func parseDAVSyncRetentionRunRequest(w http.ResponseWriter, req adminDAVSyncRetentionRunRequest) (davsyncretention.RunRequest, bool) {
	cutoffRaw := strings.TrimSpace(req.Cutoff)
	if cutoffRaw == "" {
		writeError(w, http.StatusBadRequest, "cutoff is required")
		return davsyncretention.RunRequest{}, false
	}
	cutoff, err := time.Parse(time.RFC3339, cutoffRaw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "cutoff must be RFC3339 timestamp")
		return davsyncretention.RunRequest{}, false
	}
	normalized, err := davsyncretention.NormalizeRunRequest(davsyncretention.RunRequest{
		Cutoff:       cutoff,
		Limit:        req.Limit,
		DryRun:       req.DryRun,
		ConfirmReady: req.ConfirmReady,
	}, time.Now)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return davsyncretention.RunRequest{}, false
	}
	return normalized, true
}

func parseDAVSyncRetentionReadinessRequest(w http.ResponseWriter, r *http.Request) (davsyncretention.ReadinessRequest, bool) {
	cutoffRaw, ok := singleQueryValue(w, r, "cutoff")
	if !ok {
		return davsyncretention.ReadinessRequest{}, false
	}
	cutoffRaw = strings.TrimSpace(cutoffRaw)
	if cutoffRaw == "" {
		writeError(w, http.StatusBadRequest, "cutoff is required")
		return davsyncretention.ReadinessRequest{}, false
	}
	cutoff, err := time.Parse(time.RFC3339, cutoffRaw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "cutoff must be RFC3339 timestamp")
		return davsyncretention.ReadinessRequest{}, false
	}
	limit, ok := parseDAVSyncRetentionLimit(w, r)
	if !ok {
		return davsyncretention.ReadinessRequest{}, false
	}
	req, err := davsyncretention.NormalizeReadinessRequest(davsyncretention.ReadinessRequest{
		Cutoff: cutoff,
		Limit:  limit,
	}, time.Now)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return davsyncretention.ReadinessRequest{}, false
	}
	return req, true
}

func parseDAVSyncRetentionLimit(w http.ResponseWriter, r *http.Request) (int, bool) {
	raw, ok := singleQueryValue(w, r, "limit")
	if !ok {
		return 0, false
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, true
	}
	if len(raw) > maxHTTPControlBytes {
		writeError(w, http.StatusBadRequest, "limit is too long")
		return 0, false
	}
	limit, err := strconv.Atoi(raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "limit must be an integer")
		return 0, false
	}
	if limit <= 0 {
		writeError(w, http.StatusBadRequest, "limit must be positive")
		return 0, false
	}
	if limit > davsyncretention.MaxReadinessLimit {
		writeError(w, http.StatusBadRequest, "limit must be at most "+strconv.Itoa(davsyncretention.MaxReadinessLimit))
		return 0, false
	}
	return limit, true
}

func apiUsageLedgerRequestFromBatch(batch maildb.APIUsageExportBatchView, limit int) maildb.APIUsageLedgerListRequest {
	req := maildb.APIUsageLedgerListRequest{
		Limit:       limit,
		TenantID:    batch.TenantID,
		PrincipalID: batch.PrincipalID,
	}
	if batch.WindowStart != nil {
		req.From = batch.WindowStart.UTC()
	}
	if batch.WindowEnd != nil {
		req.To = batch.WindowEnd.UTC()
	}
	return req
}

func parseOptionalRFC3339Query(w http.ResponseWriter, r *http.Request, key string) (time.Time, bool) {
	raw, ok := singleQueryValue(w, r, key)
	if !ok {
		return time.Time{}, false
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, true
	}
	value, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, key+" must be RFC3339 timestamp")
		return time.Time{}, false
	}
	return value.UTC(), true
}

const maxAdminQueryFilterBytes = 1024

func parseBoundedAdminQuery(w http.ResponseWriter, r *http.Request, key string) (string, bool) {
	value, ok := singleQueryValue(w, r, key)
	if !ok {
		return "", false
	}
	value = strings.TrimSpace(value)
	if strings.ContainsAny(value, "\r\n") {
		writeError(w, http.StatusBadRequest, key+" must not contain CR or LF")
		return "", false
	}
	if len(value) > maxAdminQueryFilterBytes {
		writeError(w, http.StatusBadRequest, key+" is too long")
		return "", false
	}
	return value, true
}

func parseBoundedAdminPathValue(w http.ResponseWriter, r *http.Request, key string) (string, bool) {
	value := strings.TrimSpace(r.PathValue(key))
	if value == "" {
		writeError(w, http.StatusBadRequest, key+" is required")
		return "", false
	}
	if strings.ContainsAny(value, "\r\n") {
		writeError(w, http.StatusBadRequest, key+" must not contain CR or LF")
		return "", false
	}
	if len(value) > maxAdminQueryFilterBytes {
		writeError(w, http.StatusBadRequest, key+" is too long")
		return "", false
	}
	return value, true
}

func parseBoundedAdminPathPair(w http.ResponseWriter, r *http.Request, firstKey string, secondKey string) (string, string, bool) {
	first, ok := parseBoundedAdminPathValue(w, r, firstKey)
	if !ok {
		return "", "", false
	}
	second, ok := parseBoundedAdminPathValue(w, r, secondKey)
	if !ok {
		return "", "", false
	}
	return first, second, true
}

func parseBoundedAdminPathTriple(w http.ResponseWriter, r *http.Request, firstKey string, secondKey string, thirdKey string) (string, string, string, bool) {
	first, second, ok := parseBoundedAdminPathPair(w, r, firstKey, secondKey)
	if !ok {
		return "", "", "", false
	}
	third, ok := parseBoundedAdminPathValue(w, r, thirdKey)
	if !ok {
		return "", "", "", false
	}
	return first, second, third, true
}

// Route registration helpers - split RegisterAdminRoutes into focused functions

func registerWebhookRoutes(mux *http.ServeMux, adminAuth func(http.HandlerFunc) http.HandlerFunc, service AdminService) {
	mux.HandleFunc("GET /admin/v1/companies/{id}/webhooks", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleGetCompanyWebhooks(w, r, service)
	}))
	mux.HandleFunc("POST /admin/v1/companies/{id}/webhooks", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handlePostCompanyWebhook(w, r, service)
	}))
	mux.HandleFunc("DELETE /admin/v1/companies/{id}/webhooks/{webhookId}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleDeleteCompanyWebhook(w, r, service)
	}))
	mux.HandleFunc("POST /admin/v1/companies/{id}/webhooks/{webhookId}/test", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleTestCompanyWebhook(w, r, service)
	}))
}

func registerNotificationTemplateRoutes(mux *http.ServeMux, adminAuth func(http.HandlerFunc) http.HandlerFunc, service AdminService) {
	mux.HandleFunc("GET /admin/v1/companies/{id}/notification-templates", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleGetNotifTemplates(w, r, service)
	}))
	mux.HandleFunc("PUT /admin/v1/companies/{id}/notification-templates/{templateId}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handlePutNotifTemplate(w, r, service)
	}))
}

func registerSecurityPostureRoutes(mux *http.ServeMux, adminAuth func(http.HandlerFunc) http.HandlerFunc, service AdminService) {
	mux.HandleFunc("GET /admin/v1/companies/{id}/security/posture", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		handleGetSecurityPosture(w, r, service, id)
	}))
}

func registerGlobalSignatureRoutes(mux *http.ServeMux, adminAuth func(http.HandlerFunc) http.HandlerFunc, service AdminService) {
	mux.HandleFunc("GET /admin/v1/companies/{id}/signature", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		handleGetSignature(w, r, service)
	}))
	mux.HandleFunc("PUT /admin/v1/companies/{id}/signature", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handlePutSignature(w, r, service)
	}))
}

func registerLegalHoldsRoutes(mux *http.ServeMux, adminAuth func(http.HandlerFunc) http.HandlerFunc, service AdminService) {
	mux.HandleFunc("GET /admin/v1/companies/{id}/legal-holds", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		handleGetLegalHolds(w, r, service)
	}))
	mux.HandleFunc("POST /admin/v1/companies/{id}/legal-holds", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleCreateLegalHold(w, r, service)
	}))
	mux.HandleFunc("DELETE /admin/v1/companies/{id}/legal-holds/{holdId}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		handleDeleteLegalHold(w, r, service)
	}))
}

func registerSCIMStatusRoutes(mux *http.ServeMux, adminAuth func(http.HandlerFunc) http.HandlerFunc, service AdminService) {
	mux.HandleFunc("GET /admin/v1/companies/{id}/scim/status", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		handleGetSCIMStatus(w, r, service)
	}))
}

func registerSeatUsageRoutes(mux *http.ServeMux, adminAuth func(http.HandlerFunc) http.HandlerFunc, service AdminService) {
	mux.HandleFunc("GET /admin/v1/companies/{id}/seat-usage", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		handleGetSeatUsage(w, r, service)
	}))
}

func registerAuditLogExportRoutes(mux *http.ServeMux, adminAuth func(http.HandlerFunc) http.HandlerFunc, service AdminService) {
	mux.HandleFunc("GET /admin/v1/companies/{id}/audit-logs/export", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleExportCompanyAuditLogs(w, r, service)
	}))
}

func registerTenantHealthRoutes(mux *http.ServeMux, adminAuth func(http.HandlerFunc) http.HandlerFunc, service AdminService) {
	mux.HandleFunc("GET /admin/v1/companies/{id}/health", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleGetCompanyHealth(w, r, service)
	}))
}

func registerChangeHistoryAndApprovalsRoutes(mux *http.ServeMux, adminAuth func(http.HandlerFunc) http.HandlerFunc, service AdminService) {
	mux.HandleFunc("GET /admin/v1/companies/{id}/change-history", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleGetCompanyChangeHistory(w, r, service)
	}))
	mux.HandleFunc("GET /admin/v1/companies/{id}/pending-approvals", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleGetPendingApprovals(w, r, service)
	}))
	mux.HandleFunc("POST /admin/v1/companies/{id}/pending-approvals", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleCreatePendingApproval(w, r, service)
	}))
	mux.HandleFunc("POST /admin/v1/companies/{id}/pending-approvals/{approvalId}/approve", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleApproveApproval(w, r, service)
	}))
	mux.HandleFunc("POST /admin/v1/companies/{id}/pending-approvals/{approvalId}/reject", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleRejectApproval(w, r, service)
	}))
}

// ─── LDAP Sync ────────────────────────────────────────────────────────────────

func registerLDAPSyncRoutes(mux *http.ServeMux, adminAuth func(http.HandlerFunc) http.HandlerFunc, service AdminService) {
	mux.HandleFunc("POST /admin/v1/domains/{id}/ldap/sync", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleLDAPSync(w, r, service)
	}))
	mux.HandleFunc("GET /admin/v1/domains/{id}/ldap/sync-history", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleLDAPSyncHistory(w, r, service)
	}))
	mux.HandleFunc("GET /admin/v1/domains/{id}/ldap/conflicts", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleLDAPSyncConflicts(w, r, service)
	}))
	mux.HandleFunc("POST /admin/v1/domains/{id}/ldap/conflicts/{conflictId}/resolve", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleResolveLDAPConflict(w, r, service)
	}))
}

func handleLDAPSync(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	if !rejectUnknownQueryKeys(w, r, "sync_type") {
		return
	}
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	syncType, ok := parseBoundedAdminQuery(w, r, "sync_type")
	if !ok {
		return
	}
	if syncType == "" {
		writeError(w, http.StatusBadRequest, "sync_type is required")
		return
	}
	result, err := service.TriggerLDAPSync(r.Context(), id, syncType)
	if err != nil {
		if errors.Is(err, ldapidp.ErrSyncNotConfigured) {
			writeError(w, http.StatusNotImplemented, err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func handleLDAPSyncHistory(w http.ResponseWriter, r *http.Request, service AdminService) {
	if !rejectBodylessRequestPayload(w, r) {
		return
	}
	if !rejectUnknownQueryKeys(w, r, "limit", "offset") {
		return
	}
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	limit, ok := parseQueryLimit(w, r)
	if !ok {
		return
	}
	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		var err error
		offset, err = strconv.Atoi(o)
		if err != nil || offset < 0 {
			writeError(w, http.StatusBadRequest, "invalid offset")
			return
		}
	}
	runs, err := service.GetLDAPSyncRuns(r.Context(), maildb.LDAPSyncRunListRequest{
		DomainID: id,
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sync_runs": runs})
}

func handleLDAPSyncConflicts(w http.ResponseWriter, r *http.Request, service AdminService) {
	if !rejectBodylessRequestPayload(w, r) {
		return
	}
	if !rejectUnknownQueryKeys(w, r, "unresolved_only", "sync_run_id", "limit", "offset") {
		return
	}
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	limit, ok := parseQueryLimit(w, r)
	if !ok {
		return
	}
	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		var err error
		offset, err = strconv.Atoi(o)
		if err != nil || offset < 0 {
			writeError(w, http.StatusBadRequest, "invalid offset")
			return
		}
	}
	unresolvedOnly := r.URL.Query().Get("unresolved_only") == "true"
	syncRunID := r.URL.Query().Get("sync_run_id")
	conflicts, err := service.GetLDAPSyncConflicts(r.Context(), maildb.LDAPSyncConflictListRequest{
		DomainID:       id,
		SyncRunID:      syncRunID,
		UnresolvedOnly: unresolvedOnly,
		Limit:          limit,
		Offset:         offset,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"conflicts": conflicts})
}

func handleResolveLDAPConflict(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	if !rejectUnknownQueryKeys(w, r) {
		return
	}
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	conflictID, ok := parseBoundedAdminPathValue(w, r, "conflictId")
	if !ok {
		return
	}
	var req struct {
		Resolution string `json:"resolution"` // 'prefer_local', 'prefer_ldap', 'manual'
	}
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Resolution == "" {
		writeError(w, http.StatusBadRequest, "resolution is required")
		return
	}
	if err := service.ResolveLDAPSyncConflict(r.Context(), conflictID, req.Resolution); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":      "resolved",
		"conflict_id": conflictID,
		"domain_id":   id,
		"resolution":  req.Resolution,
	})
}

type loginAuditListResponse struct {
	LoginAudits []loginAuditResponse `json:"login_audits"`
	Limit       int                  `json:"limit"`
	Offset      int                  `json:"offset"`
}

type loginAuditLister interface {
	ListLoginAttempts(ctx context.Context, filter admin.LoginAuditFilter) ([]admin.LoginAuditLog, error)
}

type loginAuditResponse struct {
	ID            string `json:"id"`
	UserID        string `json:"user_id"`
	CompanyID     string `json:"company_id"`
	IPAddress     string `json:"ip_address"`
	UserAgent     string `json:"user_agent"`
	Success       bool   `json:"success"`
	FailureReason string `json:"failure_reason,omitempty"`
	Timestamp     string `json:"timestamp"`
}

func handleCompanyLoginAudits(service loginAuditLister) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r, "company_id", "user_id", "success", "from_date", "to_date", "limit", "offset") {
			return
		}
		companyID, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		filter, ok := parseLoginAuditListRequest(w, r, limit)
		if !ok {
			return
		}
		filter.CompanyID = companyID
		logs, err := service.ListLoginAttempts(r.Context(), filter)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		items := make([]loginAuditResponse, len(logs))
		for i, log := range logs {
			items[i] = loginAuditResponse{
				ID:            log.ID,
				UserID:        log.UserID,
				CompanyID:     log.CompanyID,
				IPAddress:     log.IPAddress,
				UserAgent:     log.UserAgent,
				Success:       log.Success,
				FailureReason: log.FailureReason,
				Timestamp:     log.Timestamp.UTC().Format(time.RFC3339),
			}
		}
		writeJSON(w, http.StatusOK, loginAuditListResponse{
			LoginAudits: items,
			Limit:       filter.Limit,
			Offset:      filter.Offset,
		})
	}
}

func registerRDBMSSyncRoutes(mux *http.ServeMux, adminAuth func(http.HandlerFunc) http.HandlerFunc, service AdminService) {
	mux.HandleFunc("POST /admin/v1/domains/{id}/rdbms/sync", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleRDBMSSync(w, r, service)
	}))
	mux.HandleFunc("GET /admin/v1/domains/{id}/rdbms/sync-history", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleRDBMSSyncHistory(w, r, service)
	}))
	mux.HandleFunc("GET /admin/v1/domains/{id}/rdbms/conflicts", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleRDBMSSyncConflicts(w, r, service)
	}))
	mux.HandleFunc("POST /admin/v1/domains/{id}/rdbms/conflicts/{conflictId}/resolve", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleResolveRDBMSConflict(w, r, service)
	}))
}

func handleRDBMSSync(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	if !rejectUnknownQueryKeys(w, r, "sync_type") {
		return
	}
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	syncType, ok := parseBoundedAdminQuery(w, r, "sync_type")
	if !ok {
		return
	}
	if syncType == "" {
		writeError(w, http.StatusBadRequest, "sync_type is required")
		return
	}
	result, err := service.TriggerRDBMSSync(r.Context(), id, syncType)
	if err != nil {
		if errors.Is(err, rdbmsidp.ErrSyncNotConfigured) || errors.Is(err, rdbmsidp.ErrMembershipSyncUnsupported) {
			writeError(w, http.StatusNotImplemented, err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func handleRDBMSSyncHistory(w http.ResponseWriter, r *http.Request, service AdminService) {
	if !rejectBodylessRequestPayload(w, r) {
		return
	}
	if !rejectUnknownQueryKeys(w, r, "limit", "offset") {
		return
	}
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	limit, ok := parseQueryLimit(w, r)
	if !ok {
		return
	}
	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		var err error
		offset, err = strconv.Atoi(o)
		if err != nil || offset < 0 {
			writeError(w, http.StatusBadRequest, "invalid offset")
			return
		}
	}
	runs, err := service.GetRDBMSSyncRuns(r.Context(), maildb.RDBMSSyncRunListRequest{
		DomainID: id,
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sync_runs": runs})
}

func handleRDBMSSyncConflicts(w http.ResponseWriter, r *http.Request, service AdminService) {
	if !rejectBodylessRequestPayload(w, r) {
		return
	}
	if !rejectUnknownQueryKeys(w, r, "unresolved_only", "limit", "offset") {
		return
	}
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	limit, ok := parseQueryLimit(w, r)
	if !ok {
		return
	}
	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		var err error
		offset, err = strconv.Atoi(o)
		if err != nil || offset < 0 {
			writeError(w, http.StatusBadRequest, "invalid offset")
			return
		}
	}
	unresolvedOnly := r.URL.Query().Get("unresolved_only") == "true"
	conflicts, err := service.GetRDBMSSyncConflicts(r.Context(), maildb.RDBMSSyncConflictListRequest{
		DomainID:       id,
		UnresolvedOnly: unresolvedOnly,
		Limit:          limit,
		Offset:         offset,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"conflicts": conflicts})
}

func handleResolveRDBMSConflict(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	if !rejectUnknownQueryKeys(w, r) {
		return
	}
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	conflictID, ok := parseBoundedAdminPathValue(w, r, "conflictId")
	if !ok {
		return
	}
	var req struct {
		Resolution string `json:"resolution"` // 'prefer_local', 'prefer_rdbms', 'manual'
	}
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Resolution == "" {
		writeError(w, http.StatusBadRequest, "resolution is required")
		return
	}
	if err := service.ResolveRDBMSSyncConflict(r.Context(), conflictID, req.Resolution); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":      "resolved",
		"conflict_id": conflictID,
		"domain_id":   id,
		"resolution":  req.Resolution,
	})
}
