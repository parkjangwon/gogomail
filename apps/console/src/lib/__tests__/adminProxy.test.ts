import { describe, expect, it } from 'vitest';
import { assertSameOriginRequest, encodeProxyPath, requestHeadersForBackend, responseHeadersFromBackend } from '../server/adminProxy';

describe('admin proxy security helpers', () => {
  it('encodes path segments and rejects unsafe segments', () => {
    expect(encodeProxyPath(['companies', 'company 1'])).toBe('companies/company%201');
    expect(() => encodeProxyPath(['companies', '../secret'])).toThrow();
    expect(() => encodeProxyPath(['companies', 'bad\\path'])).toThrow();
  });

  it('rejects cross-origin mutating requests', () => {
    const req = new Request('https://console.example.test/api/admin/users', {
      method: 'POST',
      headers: { origin: 'https://evil.example.test' },
    });
    expect(() => assertSameOriginRequest(req)).toThrow();
  });

  it('strips client credentials before proxying', () => {
    const req = new Request('https://console.example.test/api/admin/users', {
      headers: {
        authorization: 'Bearer attacker',
        cookie: 'admin_access_token=attacker',
        'x-forwarded-host': 'evil.example.test',
        accept: 'application/json',
      },
    });
    const headers = requestHeadersForBackend(req, 'trusted');
    expect(headers.get('authorization')).toBe('Bearer trusted');
    expect(headers.get('cookie')).toBeNull();
    expect(headers.get('x-forwarded-host')).toBeNull();
    expect(headers.get('accept')).toBe('application/json');
  });

  it('limits download response headers', () => {
    const response = new Response('a,b\n', {
      headers: {
        'content-type': 'text/csv',
        'content-disposition': 'attachment; filename="x.csv"',
      },
    });
    const headers = responseHeadersFromBackend(response);
    expect(headers['content-disposition']).toBe('attachment; filename="x.csv"');
    expect(headers['cache-control']).toBe('no-store');
    expect(headers['x-content-type-options']).toBe('nosniff');
  });
});
