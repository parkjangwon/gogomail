/**
 * Timezone utilities — shared across mail list, reading pane, calendar, etc.
 */

export function getStoredTimezone(): string {
  try {
    const stored = localStorage.getItem('webmail_timezone');
    if (stored) return stored;
  } catch { /* ignore */ }
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC';
  } catch {
    return 'UTC';
  }
}

/** Returns current UTC offset string like "GMT+9" or "GMT-5:30" for a given IANA timezone. */
function getUtcOffsetLabel(tz: string, date = new Date()): string {
  try {
    const parts = new Intl.DateTimeFormat('en-US', { timeZone: tz, timeZoneName: 'shortOffset' }).formatToParts(date);
    return parts.find((p) => p.type === 'timeZoneName')?.value ?? '';
  } catch {
    return '';
  }
}

export interface TzOption {
  value: string;   // IANA identifier, e.g. "Asia/Seoul"
  label: string;   // Display label, e.g. "Asia/Seoul (GMT+9)"
  offset: number;  // Offset minutes for sorting
}

function offsetMinutes(tz: string, date = new Date()): number {
  try {
    // Compare local time with UTC to compute offset
    const local = new Date(date.toLocaleString('en-US', { timeZone: tz }));
    const utc = new Date(date.toLocaleString('en-US', { timeZone: 'UTC' }));
    return (local.getTime() - utc.getTime()) / 60000;
  } catch {
    return 0;
  }
}

let _tzCache: TzOption[] | null = null;

export function getAllTimezones(): TzOption[] {
  if (_tzCache) return _tzCache;
  const now = new Date();
  let zones: string[];
  try {
    zones = Intl.supportedValuesOf('timeZone');
  } catch {
    // Fallback for browsers that don't support supportedValuesOf
    zones = FALLBACK_ZONES;
  }
  const result = zones.map((tz) => {
    const offsetLabel = getUtcOffsetLabel(tz, now);
    const off = offsetMinutes(tz, now);
    return { value: tz, label: `${tz} (${offsetLabel})`, offset: off };
  });
  // Sort by offset, then by name
  result.sort((a, b) => a.offset !== b.offset ? a.offset - b.offset : a.value.localeCompare(b.value));
  _tzCache = result;
  return result;
}

/** Popular timezones shown at the top of the picker. */
export const POPULAR_TIMEZONES: string[] = [
  'Asia/Seoul',
  'Asia/Tokyo',
  'Asia/Shanghai',
  'Asia/Singapore',
  'Asia/Hong_Kong',
  'Asia/Kolkata',
  'Asia/Dubai',
  'Europe/London',
  'Europe/Paris',
  'Europe/Berlin',
  'Europe/Moscow',
  'Africa/Cairo',
  'America/New_York',
  'America/Chicago',
  'America/Denver',
  'America/Los_Angeles',
  'America/Sao_Paulo',
  'Pacific/Auckland',
  'Pacific/Honolulu',
  'UTC',
];

// Minimal fallback for very old browsers
const FALLBACK_ZONES: string[] = [
  'UTC',
  'America/New_York', 'America/Chicago', 'America/Denver', 'America/Los_Angeles',
  'America/Sao_Paulo', 'Europe/London', 'Europe/Paris', 'Europe/Berlin',
  'Europe/Moscow', 'Africa/Cairo', 'Asia/Dubai', 'Asia/Kolkata',
  'Asia/Dhaka', 'Asia/Bangkok', 'Asia/Singapore', 'Asia/Shanghai',
  'Asia/Seoul', 'Asia/Tokyo', 'Australia/Sydney', 'Pacific/Auckland',
];
