'use client';

import {
  Container,
  Header,
  SpaceBetween,
  Table,
  Button,
  Badge,
} from '@cloudscape-design/components';
import { useEffect, useState } from 'react';

interface Session {
  id: string;
  user_id: string;
  ip_address: string;
  user_agent: string;
  created_at: string;
  expires_at: string;
  status: 'active' | 'expired';
}

export default function SessionsPage() {
  const [sessions, setSessions] = useState<Session[]>([]);
  const [selectedItems, setSelectedItems] = useState<Session[]>([]);

  useEffect(() => {
    // Mock data
    setSessions([
      {
        id: 'session-1',
        user_id: 'admin-1',
        ip_address: '127.0.0.1',
        user_agent: 'Mozilla/5.0...',
        created_at: '2026-05-10T13:00:00Z',
        expires_at: '2026-05-10T13:15:00Z',
        status: 'active',
      },
    ]);
  }, []);

  const handleTerminate = async (sessionId: string) => {
    setSessions(sessions.filter(s => s.id !== sessionId));
  };

  return (
    <Container header={<Header>세션 관리</Header>}>
      <SpaceBetween size="l">
        <Table
          columnDefinitions={[
            { header: '세션 ID', cell: (s) => s.id },
            { header: '사용자 ID', cell: (s) => s.user_id },
            { header: 'IP 주소', cell: (s) => s.ip_address },
            { header: '생성 시간', cell: (s) => new Date(s.created_at).toLocaleString('ko-KR') },
            { header: '만료 시간', cell: (s) => new Date(s.expires_at).toLocaleString('ko-KR') },
            {
              header: '상태',
              cell: (s) => (
                <Badge color={s.status === 'active' ? 'green' : 'gray'}>
                  {s.status === 'active' ? '활성' : '만료'}
                </Badge>
              ),
            },
            {
              header: '작업',
              cell: (s) => (
                <Button
                  variant="inline-link"
                  onClick={() => handleTerminate(s.id)}
                >
                  종료
                </Button>
              ),
            },
          ]}
          items={sessions}
          selectionType="multi"
          selectedItems={selectedItems}
          onSelectionChange={(e) => setSelectedItems(e.detail.selectedItems)}
        />
      </SpaceBetween>
    </Container>
  );
}
