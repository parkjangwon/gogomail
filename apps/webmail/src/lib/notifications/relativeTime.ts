/**
 * Localized "x minutes ago"-style formatter using Intl.RelativeTimeFormat.
 * Falls back to translated short forms when t() is provided and the browser
 * lacks RelativeTimeFormat (very old environments).
 */

type TimeAgoT = (key: 'now' | 'minute' | 'hour' | 'day', vars?: { count: number }) => string;

const SECOND = 1000;
const MINUTE = 60 * SECOND;
const HOUR = 60 * MINUTE;
const DAY = 24 * HOUR;

export function formatRelativeTime(timestamp: number, locale: string, t?: TimeAgoT): string {
  const diff = Date.now() - timestamp;
  const absDiff = Math.abs(diff);

  if (absDiff < MINUTE) {
    return t ? t('now') : 'just now';
  }

  // Prefer Intl.RelativeTimeFormat for proper localization
  if (typeof Intl !== 'undefined' && typeof Intl.RelativeTimeFormat === 'function') {
    try {
      const rtf = new Intl.RelativeTimeFormat(locale, { numeric: 'auto', style: 'short' });
      if (absDiff < HOUR) return rtf.format(-Math.floor(diff / MINUTE), 'minute');
      if (absDiff < DAY) return rtf.format(-Math.floor(diff / HOUR), 'hour');
      return rtf.format(-Math.floor(diff / DAY), 'day');
    } catch {
      // fall through
    }
  }

  if (!t) {
    if (absDiff < HOUR) return `${Math.floor(diff / MINUTE)}m ago`;
    if (absDiff < DAY) return `${Math.floor(diff / HOUR)}h ago`;
    return `${Math.floor(diff / DAY)}d ago`;
  }
  if (absDiff < HOUR) return t('minute', { count: Math.floor(diff / MINUTE) });
  if (absDiff < DAY) return t('hour', { count: Math.floor(diff / HOUR) });
  return t('day', { count: Math.floor(diff / DAY) });
}
