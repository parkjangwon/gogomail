/**
 * Unit tests for src/lib/compose/composeUtils.ts
 *
 * Run with: node --experimental-strip-types scripts/check-compose-utils.mjs
 */
import assert from 'node:assert/strict';

import {
  escapeHtml,
  parseAddr,
  parseAddrs,
  isValidEmailAddress,
  isRecipientGroupToken,
  invalidRecipientAddresses,
  backendComposeIntent,
  emailOf,
} from '../src/lib/compose/composeUtils.ts';

// ── escapeHtml ───────────────────────────────────────────────────────────────
assert.equal(escapeHtml(''), '', 'empty string');
assert.equal(escapeHtml('hello'), 'hello', 'plain text unchanged');
assert.equal(escapeHtml('a & b'), 'a &amp; b', 'ampersand');
assert.equal(escapeHtml('<b>bold</b>'), '&lt;b&gt;bold&lt;/b&gt;', 'angle brackets');
assert.equal(escapeHtml('<script>alert(1)</script>'), '&lt;script&gt;alert(1)&lt;/script&gt;', 'script tag XSS');
assert.equal(escapeHtml('a < b & c > d'), 'a &lt; b &amp; c &gt; d', 'mixed');

// ── parseAddr ────────────────────────────────────────────────────────────────
assert.deepEqual(parseAddr('alice@example.com'), { address: 'alice@example.com' }, 'bare email');
assert.deepEqual(parseAddr('Alice Smith <alice@example.com>'), { name: 'Alice Smith', address: 'alice@example.com' }, 'name + angle-addr');
// Note: parseAddr does NOT strip leading/trailing whitespace from the full string.
// The caller (parseAddrs) is responsible for trimming before calling parseAddr.
assert.deepEqual(parseAddr('Bob <bob@example.com>'), { name: 'Bob', address: 'bob@example.com' }, 'name + angle-addr no extra spaces');
// parseAddr requires at least one character before '<' to match the name+angle pattern.
// A bare '<addr>' with no name prefix falls through to the plain address case.
assert.deepEqual(parseAddr('<no-name@example.com>'), { address: '<no-name@example.com>' }, 'bare angle-addr without name treated as raw address');

// ── parseAddrs ───────────────────────────────────────────────────────────────
assert.deepEqual(parseAddrs('a@x.com, b@x.com'), [{ address: 'a@x.com' }, { address: 'b@x.com' }], 'CSV');
assert.deepEqual(parseAddrs('Alice <a@x.com>, Bob <b@x.com>'), [{ name: 'Alice', address: 'a@x.com' }, { name: 'Bob', address: 'b@x.com' }], 'CSV with names');
// parseAddrs splits on commas outside angle-brackets but does not handle
// quoted commas in display names — this is a known limitation documented here.
// Input '"Doe, Jane" <jane@x.com>' produces two fragments due to the quoted comma.
assert.equal(parseAddrs('"Doe, Jane" <jane@x.com>').length, 2, 'quoted comma in display name splits (known limitation)');
assert.deepEqual(parseAddrs(''), [], 'empty string → empty array');
assert.deepEqual(parseAddrs('  '), [], 'whitespace only → empty array');

// ── isValidEmailAddress ──────────────────────────────────────────────────────
assert.ok(isValidEmailAddress('user@example.com'), 'valid simple address');
assert.ok(isValidEmailAddress('user+tag@sub.example.co.uk'), 'valid address with subdomains and plus');
assert.ok(!isValidEmailAddress(''), 'empty string invalid');
assert.ok(!isValidEmailAddress('no-at-sign'), 'no @ sign invalid');
assert.ok(!isValidEmailAddress('@nodomain.com'), 'no local part invalid');
assert.ok(!isValidEmailAddress('user@'), 'no domain invalid');
assert.ok(!isValidEmailAddress('user@has..dots'), 'consecutive dots in domain invalid');
assert.ok(!isValidEmailAddress('user@.startsdot.com'), 'domain starts with dot invalid');
assert.ok(!isValidEmailAddress('user@endsdot.'), 'domain ends with dot invalid');
assert.ok(!isValidEmailAddress('has space@example.com'), 'space in address invalid');
assert.ok(!isValidEmailAddress('has<angle@example.com'), 'angle bracket invalid');

// ── isRecipientGroupToken ─────────────────────────────────────────────────────
assert.ok(isRecipientGroupToken('org:550e8400-e29b-41d4-a716-446655440000'), 'valid org token');
assert.ok(isRecipientGroupToken('org:550e8400-e29b-41d4-a716-446655440000:children'), 'valid org:children token');
assert.ok(isRecipientGroupToken('addressbook:550e8400-e29b-41d4-a716-446655440000'), 'valid addressbook token');
assert.ok(!isRecipientGroupToken('user@example.com'), 'email is not a group token');
assert.ok(!isRecipientGroupToken('org:not-a-uuid'), 'org without valid UUID is not a token');

// ── invalidRecipientAddresses ────────────────────────────────────────────────
assert.deepEqual(invalidRecipientAddresses('a@x.com, b@x.com'), [], 'all valid → empty');
assert.deepEqual(invalidRecipientAddresses('not-an-email'), ['not-an-email'], 'invalid plain token');
assert.deepEqual(invalidRecipientAddresses('a@x.com, bad, b@x.com'), ['bad'], 'mixed valid/invalid');
assert.deepEqual(invalidRecipientAddresses(''), [], 'empty string → no invalids');

// ── backendComposeIntent ──────────────────────────────────────────────────────
assert.equal(backendComposeIntent('new'), 'new', 'new stays new');
assert.equal(backendComposeIntent('reply'), 'reply', 'reply stays reply');
assert.equal(backendComposeIntent('reply_all'), 'reply', 'reply_all maps to reply');
assert.equal(backendComposeIntent('forward'), 'forward', 'forward stays forward');

// ── emailOf ──────────────────────────────────────────────────────────────────
assert.equal(emailOf({ email: 'a@x.com', address: 'b@x.com' }), 'a@x.com', 'email field preferred when non-empty');
// emailOf uses `||` so empty email falls through to address
assert.equal(emailOf({ email: '', address: 'b@x.com' }), 'b@x.com', 'empty email falls through to address (|| semantics)');
assert.equal(emailOf({ address: 'b@x.com' }), 'b@x.com', 'falls back to address when email absent');
assert.equal(emailOf({}), '', 'both absent → empty string');

console.log('✅ check-compose-utils: all assertions passed');
