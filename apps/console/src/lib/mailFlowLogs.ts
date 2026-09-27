import { CSV_BOM, sanitizeCsvCell } from './export';

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
  return `"${sanitizeCsvCell(value ?? '').replace(/"/g, '""')}"`;
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
  // Free-text search maps to the backend `q` OR-search parameter, which matches
  // from_addr / rcpt_to / subject / rfc_message_id with OR semantics.
  if (filters.search?.trim()) params.set('q', filters.search.trim());
  if (filters.since?.trim()) params.set('since', filters.since.trim());
  if (filters.until?.trim()) params.set('until', filters.until.trim());
  if (typeof filters.limit === 'number' && filters.limit > 0) params.set('limit', String(filters.limit));

  return params.toString();
}

export function exportMailFlowLogsCsv(rows: MailFlowLogRow[]): string {
  const header = ['id', 'from', 'to', 'subject', 'status', 'created_at'].join(',');
  const lines = rows.map((row) =>
    [
      sanitizeCsvCell(row.id ?? ''),
      escapeCsv(row.from),
      escapeCsv(row.to),
      escapeCsv(row.subject),
      sanitizeCsvCell(row.status ?? ''),
      sanitizeCsvCell(row.created_at || row.timestamp || ''),
    ].join(',')
  );
  return [header, ...lines].join('\n');
}

/**
 * Trigger a client-side CSV download. The anchor is appended to the DOM before
 * clicking (required by Firefox, which ignores clicks on detached anchors) and
 * removed afterwards.
 */
export function downloadCsv(csv: string, filename: string): void {
  const blob = new Blob([CSV_BOM + csv], { type: 'text/csv;charset=utf-8;' });
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement('a');
  anchor.href = url;
  anchor.download = filename;
  anchor.style.display = 'none';
  document.body.appendChild(anchor);
  anchor.click();
  document.body.removeChild(anchor);
  URL.revokeObjectURL(url);
}

/**
 * Validate an optional RFC3339 / `datetime-local` timestamp. Returns an error
 * message key-friendly boolean; empty input is considered valid (no filter).
 * Accepts both full RFC3339 (`2026-05-01T00:00:00Z`) and the value produced by
 * an HTML `datetime-local` input (`2026-05-01T00:00`).
 */
export function isValidDateTimeInput(value: string): boolean {
  const trimmed = value.trim();
  if (!trimmed) return true;
  const parsed = Date.parse(trimmed);
  return !Number.isNaN(parsed);
}

/**
 * Normalize a datetime input to RFC3339 for the backend. Empty input returns an
 * empty string. `datetime-local` values (no timezone) are treated as UTC.
 */
export function toRFC3339(value: string): string {
  const trimmed = value.trim();
  if (!trimmed) return '';
  // Already has timezone / Z suffix — pass through.
  if (/[zZ]$|[+-]\d{2}:?\d{2}$/.test(trimmed)) return trimmed;
  // Bare `datetime-local`-style value with no timezone: treat as UTC.
  // Append seconds if only minute precision was provided so the backend
  // RFC3339 parser accepts it.
  if (/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}$/.test(trimmed)) return `${trimmed}:00Z`;
  if (/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}$/.test(trimmed)) return `${trimmed}Z`;
  return trimmed;
}
