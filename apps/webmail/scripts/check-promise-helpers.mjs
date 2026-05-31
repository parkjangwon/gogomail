import assert from 'node:assert/strict';

import { ignoreNonCritical } from '../src/lib/promise.ts';

const previousWarn = console.warn;
const warnings = [];
console.warn = (...args) => {
  warnings.push(args);
};

try {
  ignoreNonCritical(Promise.resolve('ok'), 'promiseHelpers.resolved');
  await Promise.resolve();
  assert.equal(warnings.length, 0);

  ignoreNonCritical(Promise.reject(new Error('boom')), 'promiseHelpers.rejectedError');
  await Promise.resolve();
  assert.equal(warnings.length, 1);
  assert.equal(warnings[0][0], 'non-critical async task failed');
  assert.deepEqual(warnings[0][1], {
    context: 'promiseHelpers.rejectedError',
    error: 'boom',
  });

  ignoreNonCritical(Promise.reject('string failure'), 'promiseHelpers.rejectedString');
  await Promise.resolve();
  assert.equal(warnings.length, 2);
  assert.deepEqual(warnings[1][1], {
    context: 'promiseHelpers.rejectedString',
    error: 'string failure',
  });
} finally {
  console.warn = previousWarn;
}

console.log('promise helper checks passed');
