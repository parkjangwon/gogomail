export interface LoginAuditRow {
  id: string;
  user_id: string;
  company_id: string;
  ip_address: string;
  user_agent: string;
  success: boolean;
  failure_reason: string;
  timestamp: string;
}

export interface LoginAuditQueryFilters {
  companyId?: string;
  userId?: string;
  success?: boolean;
  fromDate?: string;
  toDate?: string;
  limit?: number;
  offset?: number;
}

function escapeCsv(value: string): string {
  return `"${(value ?? '').replace(/"/g, '""')}"`;
}

export function buildLoginAuditsQuery(filters: LoginAuditQueryFilters): string {
  const params = new URLSearchParams();
  if (filters.companyId?.trim()) params.set('company_id', filters.companyId.trim());
  if (filters.userId?.trim()) params.set('user_id', filters.userId.trim());
  if (typeof filters.success === 'boolean') params.set('success', String(filters.success));
  if (filters.fromDate?.trim()) params.set('from_date', filters.fromDate.trim());
  if (filters.toDate?.trim()) params.set('to_date', filters.toDate.trim());
  if (typeof filters.limit === 'number' && filters.limit > 0) params.set('limit', String(filters.limit));
  if (typeof filters.offset === 'number' && filters.offset >= 0) params.set('offset', String(filters.offset));
  return params.toString();
}

export function exportLoginAuditsCsv(rows: LoginAuditRow[]): string {
  const header = ['id', 'user_id', 'company_id', 'ip_address', 'user_agent', 'success', 'failure_reason', 'timestamp'].join(',');
  const lines = rows.map((row) =>
    [
      escapeCsv(row.id),
      escapeCsv(row.user_id),
      escapeCsv(row.company_id),
      escapeCsv(row.ip_address),
      escapeCsv(row.user_agent),
      escapeCsv(String(row.success)),
      escapeCsv(row.failure_reason),
      escapeCsv(row.timestamp),
    ].join(',')
  );
  return [header, ...lines].join('\n');
}
