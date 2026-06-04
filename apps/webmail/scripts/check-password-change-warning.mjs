import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

const settingsAccount = readFileSync(new URL('../src/components/settings-view/useSettingsAccount.ts', import.meta.url), 'utf8');
const mailSession = readFileSync(new URL('../src/app/mail/useMailSession.ts', import.meta.url), 'utf8');
const maildbMe = readFileSync(new URL('../../../internal/maildb/me.go', import.meta.url), 'utf8');

assert.match(
  settingsAccount,
  /await changePassword\(pwCurrent, pwNew\);[\s\S]*localStorage\.removeItem\('webmail_must_change_password'\)/,
  'successful password changes should clear the local must-change-password flag'
);
assert.match(
  settingsAccount,
  /window\.dispatchEvent\(new Event\('webmail:must-change-password-cleared'\)\)/,
  'successful password changes should notify the active mail session to hide the warning immediately'
);
assert.match(
  mailSession,
  /setMustChangePassword\(false\)[\s\S]*webmail:must-change-password-cleared/,
  'mail session should hide the warning when password-change cleanup is dispatched'
);
assert.match(
  maildbMe,
  /UPDATE users SET[\s\S]*must_change_password = false[\s\S]*WHERE id = \$1::uuid/,
  'backend password change should persistently clear users.must_change_password'
);

console.log('password change warning checks passed');
