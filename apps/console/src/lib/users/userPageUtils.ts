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

export function parseUsersCsv(text: string): ImportedUserRow[] {
  return text
    .trim()
    .split('\n')
    .filter(Boolean)
    .map((line) => {
      const cols = line.split(',');
      return {
        email: (cols[0] ?? '').trim(),
        display_name: (cols[1] ?? '').trim(),
        domain_id: (cols[2] ?? '').trim(),
        password: (cols[3] ?? '').trim(),
      };
    })
    .filter((user) => user.email);
}
