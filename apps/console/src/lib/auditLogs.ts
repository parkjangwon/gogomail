export interface AuditLogRow {
  id: string;
  actor_id: string;
  category: string;
  action: string;
  target_type: string;
  target_id: string;
  result: string;
  ip_address: string;
  created_at: string;
}

export interface AuditLogQueryFilters {
  companyId?: string;
  domainId?: string;
  userId?: string;
  category?: string;
  action?: string;
  targetType?: string;
  fromDate?: string;
  toDate?: string;
  limit?: number;
  offset?: number;
}

function escapeCsv(value: string): string {
  return `"${(value ?? '').replace(/"/g, '""')}"`;
}

export function buildAuditLogsQuery(filters: AuditLogQueryFilters): string {
  const params = new URLSearchParams();

  if (filters.companyId?.trim()) params.set('company_id', filters.companyId.trim());
  if (filters.domainId?.trim()) params.set('domain_id', filters.domainId.trim());
  if (filters.userId?.trim()) params.set('user_id', filters.userId.trim());
  if (filters.category?.trim()) params.set('category', filters.category.trim());
  if (filters.action?.trim()) params.set('action', filters.action.trim());
  if (filters.targetType?.trim()) params.set('target_type', filters.targetType.trim());
  if (filters.fromDate?.trim()) params.set('from_date', filters.fromDate.trim());
  if (filters.toDate?.trim()) params.set('to_date', filters.toDate.trim());
  if (typeof filters.limit === 'number' && filters.limit > 0) params.set('limit', String(filters.limit));
  if (typeof filters.offset === 'number' && filters.offset >= 0) params.set('offset', String(filters.offset));

  return params.toString();
}

export function exportAuditLogsCsv(rows: AuditLogRow[]): string {
  const header = [
    'id',
    'actor_id',
    'category',
    'action',
    'target_type',
    'target_id',
    'result',
    'ip_address',
    'created_at',
  ].join(',');

  const lines = rows.map((row) =>
    [
      escapeCsv(row.id),
      escapeCsv(row.actor_id),
      escapeCsv(row.category),
      escapeCsv(row.action),
      escapeCsv(row.target_type),
      escapeCsv(row.target_id),
      escapeCsv(row.result),
      escapeCsv(row.ip_address),
      escapeCsv(row.created_at),
    ].join(',')
  );

  return [header, ...lines].join('\n');
}
