'use client';

import {
  Container,
  Header,
  SpaceBetween,
  Table,
  DateRangePicker,
  Button,
} from '@cloudscape-design/components';
import { useEffect, useState } from 'react';

interface AdminLog {
  id: string;
  user_id: string;
  action: string;
  resource_type: string;
  timestamp: string;
  ip_address: string;
}

export default function LogsPage() {
  const [logs, setLogs] = useState<AdminLog[]>([]);

  useEffect(() => {
    // Mock data
    setLogs([
      {
        id: 'log-1',
        user_id: 'admin-1',
        action: 'LOGIN',
        resource_type: 'auth',
        timestamp: '2026-05-10T13:00:00Z',
        ip_address: '127.0.0.1',
      },
      {
        id: 'log-2',
        user_id: 'admin-1',
        action: 'CREATE',
        resource_type: 'user',
        timestamp: '2026-05-10T13:05:00Z',
        ip_address: '127.0.0.1',
      },
    ]);
  }, []);

  return (
    <Container header={<Header>시스템 로그</Header>}>
      <SpaceBetween size="l">
        <div>
          <DateRangePicker
            value={null}
            onChange={() => {}}
            placeholder="날짜 범위 선택"
          />
        </div>

        <Table
          columnDefinitions={[
            { header: '시간', cell: (log) => new Date(log.timestamp).toLocaleString('ko-KR') },
            { header: '사용자 ID', cell: (log) => log.user_id },
            { header: '작업', cell: (log) => log.action },
            { header: '리소스 타입', cell: (log) => log.resource_type },
            { header: 'IP 주소', cell: (log) => log.ip_address },
          ]}
          items={logs}
        />

        <div>
          <Button>CSV 내보내기</Button>
          <Button style={{ marginLeft: '8px' }}>JSON 내보내기</Button>
        </div>
      </SpaceBetween>
    </Container>
  );
}
