import { describe, expect, it } from 'vitest';
import { buildAPIUsageQuery, exportAPIUsageCsv, summarizeAPIUsage, type APIUsageDailyRow } from '../apiUsage';

describe('apiUsage', () => {
  it('builds a query string with populated filters only', () => {
    expect(
      buildAPIUsageQuery({
        companyId: 'company-1',
        domainId: 'domain-1',
        principalId: 'principal-9',
        authSource: 'bearer',
        method: 'GET',
        route: '/api/admin/v1/users',
        status: 500,
        from: '2026-05-01T00:00:00Z',
        to: '2026-05-31T23:59:59Z',
        limit: 25,
      })
    ).toBe(
      'company_id=company-1&domain_id=domain-1&principal_id=principal-9&auth_source=bearer&method=GET&route=%2Fapi%2Fadmin%2Fv1%2Fusers&status=500&from=2026-05-01T00%3A00%3A00Z&to=2026-05-31T23%3A59%3A59Z&limit=25'
    );
  });

  it('exports API usage rows as escaped csv', () => {
    const rows: APIUsageDailyRow[] = [
      {
        day: '2026-05-15T00:00:00Z',
        method: 'GET',
        route: '/api/admin/v1/users',
        status: 500,
        tenant_id: 'tenant-1',
        company_id: 'company-1',
        domain_id: 'domain-1',
        user_id: 'user-1',
        api_key_id: 'key-1',
        principal_id: 'principal-1',
        auth_source: 'bearer',
        request_count: 42,
        request_bytes: 2048,
        response_bytes: 1024,
        latency_ms_total: 840,
        latency_ms_max: 120,
        latency_ms_average: 20,
        first_seen_at: '2026-05-15T00:00:00Z',
        last_seen_at: '2026-05-15T01:00:00Z',
      },
    ];

    expect(exportAPIUsageCsv(rows)).toBe(
      [
        'day,method,route,status,company_id,domain_id,user_id,principal_id,auth_source,request_count,request_bytes,response_bytes,latency_ms_total,latency_ms_max,latency_ms_average,first_seen_at,last_seen_at',
        '"2026-05-15T00:00:00Z","GET","/api/admin/v1/users","500","company-1","domain-1","user-1","principal-1","bearer","42","2048","1024","840","120","20","2026-05-15T00:00:00Z","2026-05-15T01:00:00Z"',
      ].join('\n')
    );
  });

  it('summarizes request volume and error rate', () => {
    const summary = summarizeAPIUsage([
      {
        day: '2026-05-14T00:00:00Z',
        method: 'GET',
        route: '/api/admin/v1/users',
        status: 200,
        tenant_id: 'tenant-1',
        company_id: 'company-1',
        domain_id: 'domain-1',
        user_id: 'user-1',
        api_key_id: 'key-1',
        principal_id: 'principal-1',
        auth_source: 'bearer',
        request_count: 90,
        request_bytes: 9000,
        response_bytes: 3000,
        latency_ms_total: 450,
        latency_ms_max: 50,
        latency_ms_average: 5,
        first_seen_at: '2026-05-14T00:00:00Z',
        last_seen_at: '2026-05-14T01:00:00Z',
      },
      {
        day: '2026-05-15T00:00:00Z',
        method: 'POST',
        route: '/api/admin/v1/users',
        status: 500,
        tenant_id: 'tenant-1',
        company_id: 'company-1',
        domain_id: 'domain-1',
        user_id: 'user-1',
        api_key_id: 'key-1',
        principal_id: 'principal-1',
        auth_source: 'bearer',
        request_count: 10,
        request_bytes: 1000,
        response_bytes: 400,
        latency_ms_total: 250,
        latency_ms_max: 80,
        latency_ms_average: 25,
        first_seen_at: '2026-05-15T00:00:00Z',
        last_seen_at: '2026-05-15T01:00:00Z',
      },
    ]);

    expect(summary).toEqual({
      request_count: 100,
      error_rate: 10,
      average_latency_ms: 7,
      total_request_bytes: 10000,
      total_response_bytes: 3400,
    });
  });
});
