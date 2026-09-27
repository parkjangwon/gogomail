// Shared quota helpers for the admin console.
//
// Quota is billing/capacity policy: silent corruption (a 1024x shrink on a
// unit toggle, or an invalid string becoming "unlimited") is the worst
// possible outcome. These functions are pure and unit-tested so the parsing
// and unit-conversion rules live in one place instead of being re-derived in
// every form.

export const BYTES_PER_MB = 1048576; // 1024 * 1024
export const BYTES_PER_GB = BYTES_PER_MB * 1024;
export const BYTES_PER_TB = BYTES_PER_GB * 1024;

export const QUOTA_UNITS = {
  MB: BYTES_PER_MB,
  GB: BYTES_PER_GB,
  TB: BYTES_PER_TB,
} as const;

export type QuotaUnit = keyof typeof QUOTA_UNITS;

/**
 * Pick the largest unit that represents `bytes` as a whole number, so a value
 * stored as bytes displays as e.g. "10" GB rather than "10240" MB.
 */
export function bestQuotaUnit(bytes: number): QuotaUnit {
  if (bytes >= QUOTA_UNITS.TB && bytes % QUOTA_UNITS.TB === 0) return 'TB';
  if (bytes >= QUOTA_UNITS.GB && bytes % QUOTA_UNITS.GB === 0) return 'GB';
  return 'MB';
}

/**
 * Format a byte count for display in the given unit. Whole numbers render
 * without a decimal point; fractional values are trimmed to two decimals.
 */
export function formatQuotaValue(bytes: number, unit: QuotaUnit): string {
  const value = bytes / QUOTA_UNITS[unit];
  return Number.isInteger(value) ? String(value) : String(Number(value.toFixed(2)));
}

/**
 * Convert a byte count from being displayed in one unit to being displayed in
 * another WITHOUT changing the underlying byte value. Changing the display
 * unit alone must never mutate stored capacity — it is a view concern only.
 *
 * This is the identity function on bytes; it exists to make the "unit change
 * preserves bytes" contract explicit and testable (see quota.test.ts).
 */
export function convertQuotaUnit(bytes: number): number {
  return bytes;
}

export interface QuotaParseResult {
  /** true when the input is accepted; false means `error` explains why. */
  valid: boolean;
  /**
   * Resolved byte value when valid. `null` means "unlimited" (explicit empty
   * input), which callers typically encode as 0 on the wire.
   */
  bytes: number | null;
  /** Human-readable reason the input was rejected (empty when valid). */
  error: string;
}

export interface ParseQuotaOptions {
  /**
   * When false (default), fractional inputs like "1.5" are rejected so we do
   * not silently truncate. When true, the value is multiplied by the unit and
   * rounded to the nearest byte.
   */
  allowFractional?: boolean;
}

/**
 * Parse a user-typed quota string in the given unit into bytes.
 *
 * Contract (quota is capacity policy — never silently coerce):
 *  - Empty / whitespace  → unlimited (valid, bytes = null).
 *  - "abc" / non-numeric → invalid (blocks save).
 *  - Negative ("-5")     → invalid.
 *  - Fractional ("1.5")  → invalid unless allowFractional is set.
 *  - Otherwise           → bytes = value * QUOTA_UNITS[unit].
 */
export function parseQuotaInput(
  raw: string,
  unit: QuotaUnit,
  options: ParseQuotaOptions = {},
): QuotaParseResult {
  const trimmed = (raw ?? '').trim();
  if (trimmed === '') {
    return { valid: true, bytes: null, error: '' };
  }

  const num = Number(trimmed);
  if (!Number.isFinite(num)) {
    return { valid: false, bytes: null, error: 'invalid_number' };
  }
  if (num < 0) {
    return { valid: false, bytes: null, error: 'negative' };
  }

  const isWhole = Number.isInteger(num);
  if (!isWhole && !options.allowFractional) {
    return { valid: false, bytes: null, error: 'fraction' };
  }

  const bytes = Math.round(num * QUOTA_UNITS[unit]);
  return { valid: true, bytes, error: '' };
}

export interface IntFieldOptions {
  min?: number;
  max?: number;
}

export interface IntFieldResult {
  valid: boolean;
  value: number | null;
  error: string;
}

/**
 * Validate a non-negative integer form field (e.g. password_min_length,
 * session_timeout_minutes). Rejects blanks, non-numbers, negatives, fractions,
 * and out-of-range values instead of falling back to a default and hiding the
 * mistake.
 */
export function validateIntField(
  raw: string,
  options: IntFieldOptions = {},
): IntFieldResult {
  const trimmed = (raw ?? '').trim();
  if (trimmed === '') {
    return { valid: false, value: null, error: 'required' };
  }
  const num = Number(trimmed);
  if (!Number.isFinite(num) || !Number.isInteger(num)) {
    return { valid: false, value: null, error: 'invalid_integer' };
  }
  if (options.min !== undefined && num < options.min) {
    return { valid: false, value: null, error: 'below_min' };
  }
  if (options.max !== undefined && num > options.max) {
    return { valid: false, value: null, error: 'above_max' };
  }
  return { valid: true, value: num, error: '' };
}
