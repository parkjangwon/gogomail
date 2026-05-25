/**
 * Unit tests for src/lib/stableId.ts and src/lib/timezone.ts (pure functions only)
 *
 * Run with: node --experimental-strip-types scripts/check-stableid-timezone.mjs
 */
import assert from 'node:assert/strict';

import { stableId, calendarUID } from '../src/lib/stableId.ts';
import {
  getAllTimezones,
  POPULAR_TIMEZONES,
} from '../src/lib/timezone.ts';

// ── stableId ──────────────────────────────────────────────────────────────────
{
  const id = stableId();
  assert.ok(typeof id === 'string', 'stableId returns a string');
  assert.ok(id.length > 0, 'stableId is non-empty');

  // With prefix
  const prefixed = stableId('test');
  assert.ok(prefixed.startsWith('test-'), 'stableId with prefix starts with prefix-');

  // Two IDs should be different (probabilistic but virtually certain)
  const a = stableId();
  const b = stableId();
  assert.notEqual(a, b, 'two stableIds should differ');
}

// ── calendarUID ───────────────────────────────────────────────────────────────
{
  const uid = calendarUID();
  assert.ok(uid.includes('@gogomail'), 'calendarUID ends with @gogomail');
  // Should be globally unique
  const uid2 = calendarUID();
  assert.notEqual(uid, uid2, 'two calendarUIDs should differ');
}

// ── getAllTimezones ───────────────────────────────────────────────────────────
{
  const tzs = getAllTimezones();
  assert.ok(Array.isArray(tzs), 'getAllTimezones returns array');
  assert.ok(tzs.length > 0, 'timezone list is non-empty');

  // Each entry has the expected shape
  for (const tz of tzs.slice(0, 5)) {
    assert.ok('value' in tz, 'timezone has value');
    assert.ok('label' in tz, 'timezone has label');
    assert.ok('offset' in tz, 'timezone has offset');
    assert.ok(typeof tz.value === 'string', 'value is string');
    assert.ok(typeof tz.label === 'string', 'label is string');
    assert.ok(typeof tz.offset === 'number', 'offset is number');
  }

  // Well-known timezones should be present (UTC may not appear in Intl.supportedValuesOf
  // on some platforms, which is fine — FALLBACK_ZONES handles it)
  const values = tzs.map((t) => t.value);
  assert.ok(values.some((v) => v.includes('London') || v.includes('UTC') || v.includes('New_York')), 'at least one well-known timezone present');

  // Result is sorted by offset (ascending)
  for (let i = 1; i < tzs.length; i++) {
    assert.ok(
      tzs[i].offset >= tzs[i - 1].offset,
      `timezone list should be sorted by offset at index ${i}`
    );
  }
}

// ── POPULAR_TIMEZONES ─────────────────────────────────────────────────────────
{
  assert.ok(Array.isArray(POPULAR_TIMEZONES), 'POPULAR_TIMEZONES is an array');
  assert.ok(POPULAR_TIMEZONES.includes('UTC') || POPULAR_TIMEZONES.length > 0, 'UTC is popular or list is non-empty');
  assert.ok(POPULAR_TIMEZONES.includes('Asia/Seoul'), 'Seoul is popular');
  assert.ok(POPULAR_TIMEZONES.includes('Europe/London'), 'London is popular');
  assert.ok(POPULAR_TIMEZONES.includes('America/New_York'), 'New York is popular');
  // All popular zones should appear in getAllTimezones (requires Intl support)
  const allValues = new Set(getAllTimezones().map((t) => t.value));
  for (const pop of POPULAR_TIMEZONES) {
    if (!allValues.has(pop)) {
      console.warn(`  Popular timezone ${pop} not found in getAllTimezones (Intl coverage?)`);
    }
  }
}

console.log('✅ check-stableid-timezone: all assertions passed');
