export interface APIUsageDailyRow {
  day: string;
  method: string;
  route: string;
  status: number;
  tenant_id: string;
  company_id: string;
  domain_id: string;
  user_id: string;
  api_key_id: string;
  principal_id: string;
  auth_source: string;
  request_count: number;
  request_bytes: number;
  response_bytes: number;
  latency_ms_total: number;
  latency_ms_max: number;
  latency_ms_average: number;
  first_seen_at: string;
  last_seen_at: string;
}

export interface APIUsageQueryFilters {
  companyId?: string;
  domainId?: string;
  userId?: string;
  principalId?: string;
  authSource?: string;
  method?: string;
  route?: string;
  status?: string | number;
  from?: string;
  to?: string;
  limit?: number;
}

function escapeCsv(value: string): string {
  return `"${(value ?? '').replace(/"/g, '""')}"`;
}

export function buildAPIUsageQuery(filters: APIUsageQueryFilters): string {
  const params = new URLSearchParams();
  if (filters.companyId?.trim()) params.set('company_id', filters.companyId.trim());
  if (filters.domainId?.trim()) params.set('domain_id', filters.domainId.trim());
  if (filters.userId?.trim()) params.set('user_id', filters.userId.trim());
  if (filters.principalId?.trim()) params.set('principal_id', filters.principalId.trim());
  if (filters.authSource?.trim()) params.set('auth_source', filters.authSource.trim());
  if (filters.method?.trim()) params.set('method', filters.method.trim());
  if (filters.route?.trim()) params.set('route', filters.route.trim());
  if (filters.status !== undefined && filters.status !== '') {
    const status = typeof filters.status === 'number' ? filters.status : Number(filters.status);
    if (Number.isFinite(status)) params.set('status', String(status));
  }
  if (filters.from?.trim()) params.set('from', filters.from.trim());
  if (filters.to?.trim()) params.set('to', filters.to.trim());
  if (typeof filters.limit === 'number' && filters.limit > 0) params.set('limit', String(filters.limit));
  return params.toString();
}

export function exportAPIUsageCsv(rows: APIUsageDailyRow[]): string {
  const header = [
    'day',
    'method',
    'route',
    'status',
    'company_id',
    'domain_id',
    'user_id',
    'principal_id',
    'auth_source',
    'request_count',
    'request_bytes',
    'response_bytes',
    'latency_ms_total',
    'latency_ms_max',
    'latency_ms_average',
    'first_seen_at',
    'last_seen_at',
  ].join(',');

  const lines = rows.map((row) => [
    escapeCsv(row.day),
    escapeCsv(row.method),
    escapeCsv(row.route),
    escapeCsv(String(row.status)),
    escapeCsv(row.company_id),
    escapeCsv(row.domain_id),
    escapeCsv(row.user_id),
    escapeCsv(row.principal_id),
    escapeCsv(row.auth_source),
    escapeCsv(String(row.request_count)),
    escapeCsv(String(row.request_bytes)),
    escapeCsv(String(row.response_bytes)),
    escapeCsv(String(row.latency_ms_total)),
    escapeCsv(String(row.latency_ms_max)),
    escapeCsv(String(row.latency_ms_average)),
    escapeCsv(row.first_seen_at),
    escapeCsv(row.last_seen_at),
  ].join(','));

  return [header, ...lines].join('\n');
}

export function summarizeAPIUsage(rows: APIUsageDailyRow[]) {
  const totals = rows.reduce(
    (acc, row) => {
      acc.requests += row.request_count ?? 0;
      acc.errors += row.status >= 400 ? row.request_count ?? 0 : 0;
      acc.latency += row.latency_ms_total ?? 0;
      return acc;
    },
    { requests: 0, errors: 0, latency: 0 },
  );

  return {
    request_count: totals.requests,
    error_rate: totals.requests > 0 ? Math.round((totals.errors / totals.requests) * 100) : 0,
    average_latency_ms: totals.requests > 0 ? Math.round((totals.latency / totals.requests) * 10) / 10 : 0,
    total_request_bytes: rows.reduce((sum, row) => sum + (row.request_bytes ?? 0), 0),
    total_response_bytes: rows.reduce((sum, row) => sum + (row.response_bytes ?? 0), 0),
  };
}
