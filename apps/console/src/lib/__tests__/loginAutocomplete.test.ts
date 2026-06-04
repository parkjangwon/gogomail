import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';

const source = readFileSync('src/app/login/page.tsx', 'utf8');

describe('console login autofill metadata', () => {
  it('marks the admin email input as a username field for password managers', () => {
    expect(source).toMatch(/name="username"/);
    expect(source).toMatch(/autoComplete="username"/);
    expect(source).toMatch(/nativeInputAttributes=\{\{ id: 'admin-email'/);
  });

  it('marks the admin password input as a current password field for password managers', () => {
    expect(source).toMatch(/name="password"/);
    expect(source).toMatch(/autoComplete="current-password"/);
    expect(source).toMatch(/nativeInputAttributes=\{\{ id: 'admin-password'/);
  });

  it('marks the MFA code input as a one-time code field', () => {
    expect(source).toMatch(/name="one-time-code"/);
    expect(source).toMatch(/autoComplete="one-time-code"/);
    expect(source).toMatch(/nativeInputAttributes=\{\{ id: 'admin-mfa-code'/);
  });
});
