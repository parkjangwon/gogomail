import assert from 'node:assert/strict';
import { toDateTimeLocalValue } from '../src/lib/dateTimeLocal.ts';

assert.equal(toDateTimeLocalValue(new Date(2026, 0, 2, 3, 4)), '2026-01-02T03:04');
assert.equal(toDateTimeLocalValue(new Date(2026, 10, 12, 13, 45)), '2026-11-12T13:45');

console.log('dateTimeLocal formatter checks passed');
