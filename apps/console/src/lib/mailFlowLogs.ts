export interface MailFlowLogRow {
  id: string;
  from: string;
  to: string;
  subject: string;
  status: string;
  created_at: string;
  timestamp: string;
  message_size: number;
}

export interface MailFlowLogQueryFilters {
  companyId?: string;
  domainId?: string;
  userId?: string;
  status?: string;
  direction?: string;
  fromAddr?: string;
  toAddr?: string;
  subject?: string;
  rfcMessageId?: string;
  search?: string;
  since?: string;
  until?: string;
  limit?: number;
}

function escapeCsv(value: string): string {
  return `"${(value ?? '').replace(/"/g, '""')}"`;
}

function appendSearchTerms(params: URLSearchParams, search: string) {
  const term = search.trim();
  if (!term) return;
  params.set('from_addr', term);
  params.set('to_addr', term);
  params.set('subject', term);
  params.set('rfc_message_id', term);
}

export function buildMailFlowLogsQuery(filters: MailFlowLogQueryFilters): string {
  const params = new URLSearchParams();

  if (filters.companyId?.trim()) params.set('company_id', filters.companyId.trim());
  if (filters.domainId?.trim()) params.set('domain_id', filters.domainId.trim());
  if (filters.userId?.trim()) params.set('user_id', filters.userId.trim());
  if (filters.status?.trim()) params.set('flow_status', filters.status.trim());
  if (filters.direction?.trim()) params.set('direction', filters.direction.trim());
  if (filters.fromAddr?.trim()) params.set('from_addr', filters.fromAddr.trim());
  if (filters.toAddr?.trim()) params.set('to_addr', filters.toAddr.trim());
  if (filters.subject?.trim()) params.set('subject', filters.subject.trim());
  if (filters.rfcMessageId?.trim()) params.set('rfc_message_id', filters.rfcMessageId.trim());
  appendSearchTerms(params, filters.search ?? '');
  if (filters.since?.trim()) params.set('since', filters.since.trim());
  if (filters.until?.trim()) params.set('until', filters.until.trim());
  if (typeof filters.limit === 'number' && filters.limit > 0) params.set('limit', String(filters.limit));

  return params.toString();
}

export function exportMailFlowLogsCsv(rows: MailFlowLogRow[]): string {
  const header = ['id', 'from', 'to', 'subject', 'status', 'created_at'].join(',');
  const lines = rows.map((row) =>
    [
      row.id,
      escapeCsv(row.from),
      escapeCsv(row.to),
      escapeCsv(row.subject),
      row.status,
      row.created_at || row.timestamp || '',
    ].join(',')
  );
  return [header, ...lines].join('\n');
}
