import { describe, expect, it } from 'vitest';
import { CSV_BOM, escapeCsvCell, sanitizeCsvCell } from '../export';

describe('sanitizeCsvCell', () => {
  it('prefixes formula-injection triggers with a single quote', () => {
    expect(sanitizeCsvCell('=1+1')).toBe("'=1+1");
    expect(sanitizeCsvCell('+1')).toBe("'+1");
    expect(sanitizeCsvCell('-1')).toBe("'-1");
    expect(sanitizeCsvCell('@SUM(A1)')).toBe("'@SUM(A1)");
    expect(sanitizeCsvCell("=cmd|'/c calc'!A0")).toBe("'=cmd|'/c calc'!A0");
    expect(sanitizeCsvCell('\tinject')).toBe("'\tinject");
    expect(sanitizeCsvCell('\rinject')).toBe("'\rinject");
  });

  it('leaves benign values unchanged', () => {
    expect(sanitizeCsvCell('hello')).toBe('hello');
    expect(sanitizeCsvCell('user@example.com')).toBe('user@example.com');
    expect(sanitizeCsvCell('')).toBe('');
    expect(sanitizeCsvCell('2026-05-15T00:00:00Z')).toBe('2026-05-15T00:00:00Z');
  });
});

describe('escapeCsvCell', () => {
  it('sanitizes then quotes when structural characters are present', () => {
    expect(escapeCsvCell('=1+1')).toBe("'=1+1");
    expect(escapeCsvCell('a"b')).toBe('"a""b"');
    expect(escapeCsvCell('a,b')).toBe('"a,b"');
    expect(escapeCsvCell('line1\nline2')).toBe('"line1\nline2"');
    expect(escapeCsvCell('line1\r\nline2')).toBe('"line1\r\nline2"');
  });

  it('quotes a sanitized value that then contains a comma', () => {
    // "=a,b" -> "'=a,b" -> contains comma -> wrapped
    expect(escapeCsvCell('=a,b')).toBe('"\'=a,b"');
  });

  it('returns empty string for null/undefined', () => {
    expect(escapeCsvCell(null)).toBe('');
    expect(escapeCsvCell(undefined)).toBe('');
  });

  it('handles numbers and booleans', () => {
    expect(escapeCsvCell(42)).toBe('42');
    expect(escapeCsvCell(true)).toBe('true');
  });
});

describe('CSV_BOM', () => {
  it('is the UTF-8 byte-order mark', () => {
    expect(CSV_BOM).toBe('\uFEFF');
  });
});
