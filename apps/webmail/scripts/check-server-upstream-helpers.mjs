import assert from 'node:assert/strict';

import { fetchUpstreamOrNull, readJSONOrDefault } from '../src/lib/server/upstream.ts';

const previousFetch = globalThis.fetch;

try {
  globalThis.fetch = async () => new Response('{"ok":true}', {
    status: 200,
    headers: { 'content-type': 'application/json' },
  });
  const response = await fetchUpstreamOrNull('https://backend.example.test/health');
  assert.equal(response?.status, 200);

  globalThis.fetch = async () => {
    throw new Error('network unavailable');
  };
  assert.equal(await fetchUpstreamOrNull('https://backend.example.test/down'), null);

  assert.deepEqual(
    await readJSONOrDefault(new Response('{"error":"upstream"}'), { error: 'fallback' }),
    { error: 'upstream' },
  );
  assert.deepEqual(
    await readJSONOrDefault(new Response('{bad json'), { error: 'fallback' }),
    { error: 'fallback' },
  );
} finally {
  globalThis.fetch = previousFetch;
}

console.log('server upstream helper checks passed');
