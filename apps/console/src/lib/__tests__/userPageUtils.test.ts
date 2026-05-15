import { describe, expect, it } from 'vitest';
import {
  buildAutoAddress,
  createEmptyUserDraft,
  formatStorage,
  parseUsersCsv,
  USER_STORAGE_BYTES_PER_GB,
} from '../users/userPageUtils';

describe('userPageUtils', () => {
  it('creates a blank draft with the expected defaults', () => {
    expect(createEmptyUserDraft()).toEqual({
      username: '',
      display_name: '',
      domain_id: '',
      password: '',
      recovery_email: '',
      quota_gb: '0',
    });
  });

  it('builds a lowercase auto address from a trimmed username', () => {
    expect(buildAutoAddress('  Jane.Doe  ', 'example.com')).toBe('jane.doe@example.com');
  });

  it('returns an empty address when username or domain is missing', () => {
    expect(buildAutoAddress('   ', 'example.com')).toBe('');
    expect(buildAutoAddress('jane', undefined)).toBe('');
  });

  it('formats storage with and without a quota limit', () => {
    expect(formatStorage(1.5 * USER_STORAGE_BYTES_PER_GB, 0)).toBe('1.5 GB');
    expect(formatStorage(1.5 * USER_STORAGE_BYTES_PER_GB, 2 * USER_STORAGE_BYTES_PER_GB)).toBe('1.5 / 2.0 GB (75%)');
  });

  it('parses import csv rows and skips blanks', () => {
    expect(parseUsersCsv('\n  jane@example.com, Jane , example.com , secret \n\n')).toEqual([
      {
        email: 'jane@example.com',
        display_name: 'Jane',
        domain_id: 'example.com',
        password: 'secret',
      },
    ]);
  });
});
