import assert from 'node:assert/strict';
import { isPrivateAddress, assertImageContentType } from '../src/lib/security/outboundUrl.ts';
import { assertSameOriginForMutation, encodeBackendPath, headersForBackend } from '../src/lib/security/proxy.ts';

for (const addr of ['127.0.0.1', '10.1.2.3', '172.16.0.1', '192.168.1.5', '169.254.169.254', '::1', 'fe80::1']) {
  assert.equal(isPrivateAddress(addr), true, `${addr} should be private`);
}
assert.equal(isPrivateAddress('93.184.216.34'), false);

assert.doesNotThrow(() => assertImageContentType('image/png'));
assert.doesNotThrow(() => assertImageContentType('image/avif; charset=binary'));
assert.throws(() => assertImageContentType('image/svg+xml'));
assert.throws(() => assertImageContentType('text/html'));

assert.equal(encodeBackendPath(['messages', 'id 1']), 'messages/id%201');
assert.throws(() => encodeBackendPath(['messages', '../secret']));
assert.throws(() => assertSameOriginForMutation('POST', 'https://mail.example.test/api/mail/messages', new Headers({ origin: 'https://evil.example.test' })));
assert.throws(() => assertSameOriginForMutation('POST', 'https://mail.example.test/api/mail/messages', new Headers()));
assert.doesNotThrow(() => assertSameOriginForMutation('POST', 'https://mail.example.test/api/mail/messages', new Headers({ origin: 'https://mail.example.test' })));

const proxiedHeaders = headersForBackend(new Headers({
  authorization: 'Bearer attacker',
  cookie: 'webmail_token=attacker',
  'content-type': 'application/json',
}), 'trusted-token');
assert.equal(proxiedHeaders.get('authorization'), 'Bearer trusted-token');
assert.equal(proxiedHeaders.get('cookie'), null);
assert.equal(proxiedHeaders.get('content-type'), 'application/json');

console.log('webmail security helper checks passed');
