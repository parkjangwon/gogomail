import { describe, expect, it } from 'vitest';

import {
  buildAuditLogsQuery,
  exportAuditLogsCsv,
} from '../auditLogs';

describe('auditLogs', () => {
  it('builds a server-side audit log query from the active filters', () => {
    expect(
      buildAuditLogsQuery({
        companyId: ' company-1 ',
        domainId: ' domain-9 ',
        userId: ' user-7 ',
        category: ' security ',
        action: ' login.failed ',
        targetType: ' session ',
        fromDate: '2026-05-01T00:00:00Z',
        toDate: '2026-05-31T23:59:59Z',
        limit: 250,
        offset: 50,
      })
    ).toBe(
      'company_id=company-1&domain_id=domain-9&user_id=user-7&category=security&action=login.failed&target_type=session&from_date=2026-05-01T00%3A00%3A00Z&to_date=2026-05-31T23%3A59%3A59Z&limit=250&offset=50'
    );
  });

  it('exports audit log rows as escaped csv', () => {
    expect(
      exportAuditLogsCsv([
        {
          id: 'audit-1',
          actor_id: 'admin-1',
          category: 'security',
          action: 'login.failed',
          target_type: 'session',
          target_id: 'sess-1',
          result: 'success',
          ip_address: '127.0.0.1',
          created_at: '2026-05-15T10:00:00Z',
        },
        {
          id: 'audit-2',
          actor_id: 'admin-2',
          category: 'auth',
          action: 'password.reset',
          target_type: 'user',
          target_id: 'user-9',
          result: 'error',
          ip_address: '',
          created_at: '2026-05-15T10:30:00Z',
        },
      ])
    ).toBe(
      [
        'id,actor_id,category,action,target_type,target_id,result,ip_address,created_at',
        '"audit-1","admin-1","security","login.failed","session","sess-1","success","127.0.0.1","2026-05-15T10:00:00Z"',
        '"audit-2","admin-2","auth","password.reset","user","user-9","error","","2026-05-15T10:30:00Z"',
      ].join('\n')
    );
  });
});
