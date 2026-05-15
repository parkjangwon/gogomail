import { describe, expect, it } from 'vitest';
import {
  buildMailFlowLogsQuery,
  exportMailFlowLogsCsv,
  type MailFlowLogRow,
} from '../mailFlowLogs';

describe('mailFlowLogs', () => {
  it('builds a query string with only populated filters', () => {
    expect(
      buildMailFlowLogsQuery({
        companyId: 'company-1',
        domainId: 'domain-9',
        status: 'delivered',
        search: 'invoice',
        since: '2026-05-01T00:00:00Z',
        until: '2026-05-31T23:59:59Z',
        limit: 25,
      })
    ).toBe(
      'company_id=company-1&domain_id=domain-9&flow_status=delivered&from_addr=invoice&to_addr=invoice&subject=invoice&rfc_message_id=invoice&since=2026-05-01T00%3A00%3A00Z&until=2026-05-31T23%3A59%3A59Z&limit=25'
    );
  });

  it('exports rows as CSV with escaped values', () => {
    const rows: MailFlowLogRow[] = [
      {
        id: 'log-1',
        from: 'Sender, Inc. <sender@example.com>',
        to: 'recipient@example.com',
        subject: 'Quarterly "Update"',
        status: 'delivered',
        created_at: '2026-05-15T00:00:00Z',
        timestamp: '2026-05-15T00:00:00Z',
        message_size: 2048,
      },
    ];

    expect(exportMailFlowLogsCsv(rows)).toBe(
      [
        'id,from,to,subject,status,created_at',
        'log-1,"Sender, Inc. <sender@example.com>","recipient@example.com","Quarterly ""Update""",delivered,2026-05-15T00:00:00Z',
      ].join('\n')
    );
  });

  it('builds a query string from explicit message trace filters', () => {
    expect(
      buildMailFlowLogsQuery({
        companyId: 'company-1',
        fromAddr: 'alice@example.com',
        toAddr: 'bob@example.com',
        subject: 'Quarterly update',
        rfcMessageId: '<msg@example.com>',
        direction: 'outbound',
        limit: 100,
      })
    ).toBe(
      'company_id=company-1&direction=outbound&from_addr=alice%40example.com&to_addr=bob%40example.com&subject=Quarterly+update&rfc_message_id=%3Cmsg%40example.com%3E&limit=100'
    );
  });
});
