import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

const source = readFileSync(new URL('../src/app/login/page.tsx', import.meta.url), 'utf8');

assert.match(source, /<form[\s\S]*autoComplete="on"/, 'login form should explicitly allow autocomplete');
assert.match(source, /id="email"[\s\S]*name="username"[\s\S]*type="email"[\s\S]*autoComplete="username"/, 'email field should expose username autocomplete metadata');
assert.match(source, /id="password"[\s\S]*name="password"[\s\S]*type="password"[\s\S]*autoComplete="current-password"/, 'password field should expose current-password autocomplete metadata');
assert.match(source, /id="mfa-code"[\s\S]*name="one-time-code"[\s\S]*autoComplete="one-time-code"/, 'MFA field should expose one-time-code autocomplete metadata');

console.log('webmail login autocomplete checks passed');
