package httpapi

import (
	"net/http"
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
