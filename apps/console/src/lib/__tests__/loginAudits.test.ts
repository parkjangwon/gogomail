import { describe, expect, it } from 'vitest';

import { buildLoginAuditsQuery, exportLoginAuditsCsv } from '../loginAudits';

describe('loginAudits', () => {
  it('builds a company-scoped login audit query', () => {
    expect(
      buildLoginAuditsQuery({
        companyId: ' company-1 ',
        userId: ' user-7 ',
        success: false,
        fromDate: '2026-05-01T00:00:00Z',
        toDate: '2026-05-31T23:59:59Z',
        limit: 100,
        offset: 25,
      })
    ).toBe(
      'company_id=company-1&user_id=user-7&success=false&from_date=2026-05-01T00%3A00%3A00Z&to_date=2026-05-31T23%3A59%3A59Z&limit=100&offset=25'
    );
  });

  it('exports login audit rows as escaped csv', () => {
    expect(
      exportLoginAuditsCsv([
        {
          id: 'login-1',
          user_id: 'user-1',
          company_id: 'company-1',
          ip_address: '127.0.0.1',
          user_agent: 'Mozilla/5.0',
          success: false,
          failure_reason: 'bad password',
          timestamp: '2026-05-15T10:00:00Z',
        },
        {
          id: 'login-2',
          user_id: 'user-2',
          company_id: 'company-1',
          ip_address: '',
          user_agent: 'curl/8.0',
          success: true,
          failure_reason: '',
          timestamp: '2026-05-15T10:30:00Z',
        },
      ])
    ).toBe(
      [
        'id,user_id,company_id,ip_address,user_agent,success,failure_reason,timestamp',
        '"login-1","user-1","company-1","127.0.0.1","Mozilla/5.0","false","bad password","2026-05-15T10:00:00Z"',
        '"login-2","user-2","company-1","","curl/8.0","true","","2026-05-15T10:30:00Z"',
      ].join('\n')
    );
  });
});
