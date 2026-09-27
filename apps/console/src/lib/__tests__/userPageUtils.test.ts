import { describe, expect, it } from 'vitest';
import {
  buildAutoAddress,
  createEmptyUserDraft,
  formatStorage,
  parseUsersCsv,
  UserCsvParseError,
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

  it('keeps commas inside quoted fields instead of shifting columns', () => {
    expect(
      parseUsersCsv('jane@example.com,"Doe, Jane",example.com,"p,ass,word"'),
    ).toEqual([
      {
        email: 'jane@example.com',
        display_name: 'Doe, Jane',
        domain_id: 'example.com',
        password: 'p,ass,word',
      },
    ]);
  });

  it('unescapes doubled quotes in quoted fields', () => {
    expect(parseUsersCsv('jane@example.com,"a""b",example.com,secret')).toEqual([
      {
        email: 'jane@example.com',
        display_name: 'a"b',
        domain_id: 'example.com',
        password: 'secret',
      },
    ]);
  });

  it('detects and skips a header row', () => {
    expect(
      parseUsersCsv('email,display_name,domain_id,password\njane@example.com,Jane,example.com,secret'),
    ).toEqual([
      {
        email: 'jane@example.com',
        display_name: 'Jane',
        domain_id: 'example.com',
        password: 'secret',
      },
    ]);
  });

  it('rejects rows with the wrong column count loudly', () => {
    expect(() => parseUsersCsv('jane@example.com,Jane,example.com')).toThrow(UserCsvParseError);
    try {
      parseUsersCsv('jane@example.com,Jane,example.com');
    } catch (e) {
      expect(e).toBeInstanceOf(UserCsvParseError);
      expect((e as UserCsvParseError).rows).toEqual([1]);
    }
  });

  it('rejects rows with an empty email', () => {
    expect(() => parseUsersCsv(',Jane,example.com,secret')).toThrow(UserCsvParseError);
  });
});
