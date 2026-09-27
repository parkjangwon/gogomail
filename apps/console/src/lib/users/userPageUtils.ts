export interface CreateUserDraft {
  username: string;
  display_name: string;
  domain_id: string;
  password: string;
  recovery_email: string;
  quota_gb: string;
}

export interface ImportedUserRow {
  email: string;
  display_name: string;
  domain_id: string;
  password: string;
}

export const USER_STORAGE_BYTES_PER_GB = 1_073_741_824;

export function createEmptyUserDraft(): CreateUserDraft {
  return {
    username: '',
    display_name: '',
    domain_id: '',
    password: '',
    recovery_email: '',
    quota_gb: '0',
  };
}

export function buildAutoAddress(username: string, domainName?: string) {
  const trimmedUsername = username.trim().toLowerCase();
  if (!trimmedUsername || !domainName) return '';
  return `${trimmedUsername}@${domainName}`;
}

export function formatStorage(usedBytes: number, limitBytes: number) {
  const usedGb = (usedBytes / USER_STORAGE_BYTES_PER_GB).toFixed(1);
  if (!limitBytes) return `${usedGb} GB`;
  const limitGb = (limitBytes / USER_STORAGE_BYTES_PER_GB).toFixed(1);
  const pct = Math.round((usedBytes / limitBytes) * 100);
  return `${usedGb} / ${limitGb} GB (${pct}%)`;
}

/** Column order expected by the bulk-import endpoint. */
const USER_CSV_COLUMNS = ['email', 'display_name', 'domain_id', 'password'] as const;

/**
 * Thrown when a user-import CSV cannot be parsed safely. Carries the offending
 * line numbers so the UI can surface exactly which rows were rejected instead
 * of silently mis-mapping columns (which could create wrong accounts).
 */
export class UserCsvParseError extends Error {
  constructor(message: string, public rows: number[] = []) {
    super(message);
    this.name = 'UserCsvParseError';
  }
}

/**
 * Parse a single CSV line/document into rows of fields following RFC-4180:
 * fields may be double-quoted, quoted fields may contain commas, CR/LF, and
 * escaped quotes (`""`). Returns an array of records, each an array of fields.
 * A trailing newline does not produce an empty record.
 */
export function parseCsv(text: string): string[][] {
  const records: string[][] = [];
  let field = '';
  let record: string[] = [];
  let inQuotes = false;
  let fieldStarted = false;
  const n = text.length;

  const endField = () => {
    record.push(field);
    field = '';
    fieldStarted = false;
  };
  const endRecord = () => {
    endField();
    records.push(record);
    record = [];
  };

  for (let i = 0; i < n; i++) {
    const ch = text[i];
    if (inQuotes) {
      if (ch === '"') {
        if (text[i + 1] === '"') {
          field += '"';
          i++;
        } else {
          inQuotes = false;
        }
      } else {
        field += ch;
      }
      continue;
    }

    if (ch === '"' && !fieldStarted) {
      inQuotes = true;
      fieldStarted = true;
      continue;
    }
    if (ch === ',') {
      endField();
      continue;
    }
    if (ch === '\r') {
      // Handle CRLF and lone CR as a record terminator.
      if (text[i + 1] === '\n') i++;
      endRecord();
      continue;
    }
    if (ch === '\n') {
      endRecord();
      continue;
    }
    field += ch;
    fieldStarted = true;
  }

  // Flush the final field/record unless the input ended on a record boundary.
  if (fieldStarted || field.length > 0 || record.length > 0) {
    endRecord();
  }

  return records;
}

/**
 * Parse a user-import CSV into typed rows. Uses an RFC-4180 parser so quoted
 * fields (e.g. a password or display name containing a comma) no longer shift
 * columns. Blank lines are skipped. An optional header row (`email,...`) is
 * detected and skipped. Rows whose column count does not match the expected
 * four columns are rejected loudly via {@link UserCsvParseError} rather than
 * silently mis-mapped — a mis-mapped row could create an account with the
 * wrong address or password.
 */
export function parseUsersCsv(text: string): ImportedUserRow[] {
  const records = parseCsv(text).filter(
    (record) => !(record.length === 1 && record[0].trim() === ''),
  );

  if (records.length === 0) return [];

  // Detect and drop a header row if the first record looks like column names.
  const first = records[0].map((c) => c.trim().toLowerCase());
  const looksLikeHeader =
    first[0] === 'email' && first.some((c) => USER_CSV_COLUMNS.includes(c as (typeof USER_CSV_COLUMNS)[number]));
  const dataRecords = looksLikeHeader ? records.slice(1) : records;

  const badRows: number[] = [];
  const rows: ImportedUserRow[] = [];

  dataRecords.forEach((record, idx) => {
    // Line number as seen by the user (1-based, accounting for header skip).
    const lineNo = (looksLikeHeader ? idx + 2 : idx + 1);
    if (record.length !== USER_CSV_COLUMNS.length) {
      badRows.push(lineNo);
      return;
    }
    const email = (record[0] ?? '').trim();
    if (!email) {
      // A row with an empty email is unusable; reject it loudly too.
      badRows.push(lineNo);
      return;
    }
    rows.push({
      email,
      display_name: (record[1] ?? '').trim(),
      domain_id: (record[2] ?? '').trim(),
      password: (record[3] ?? '').trim(),
    });
  });

  if (badRows.length > 0) {
    throw new UserCsvParseError(
      `Invalid CSV: expected ${USER_CSV_COLUMNS.length} columns (${USER_CSV_COLUMNS.join(', ')}) on line(s) ${badRows.join(', ')}.`,
      badRows,
    );
  }

  return rows;
}
