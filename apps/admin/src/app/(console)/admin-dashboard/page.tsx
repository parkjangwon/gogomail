'use client';

import {
  Container,
  Header,
  SpaceBetween,
  Box,
  Button,
  Grid,
  Card,
} from '@cloudscape-design/components';
import { useRouter } from 'next/navigation';

export default function AdminDashboard() {
  const router = useRouter();

  const adminSections = [
    {
      title: '관리자 사용자 관리',
      description: '시스템 관리자 계정 추가/삭제/편집',
      href: '/admin-users',
    },
    {
      title: '세션 관리',
      description: '활성 세션 모니터링 및 관리',
      href: '/sessions',
    },
    {
      title: '시스템 로그',
      description: '관리자 활동 로그 및 시스템 이벤트',
      href: '/logs',
    },
    {
      title: '시스템 모니터링',
      description: '성능 지표 및 시스템 상태',
      href: '/monitoring',
    },
  ];

  return (
    <Container header={<Header>시스템 관리</Header>}>
      <SpaceBetween size="l">
        <Grid gridDefinition={[{ colspan: 6 }, { colspan: 6 }]}>
          {adminSections.map((section) => (
            <Card key={section.title}>
              <SpaceBetween size="m">
                <div>
                  <h3>{section.title}</h3>
                  <p style={{ marginTop: '8px', color: '#666' }}>
                    {section.description}
                  </p>
                </div>
                <Box float="right">
                  <Button
                    onClick={() => router.push(section.href)}
                    variant="primary"
                  >
                    이동
                  </Button>
                </Box>
              </SpaceBetween>
            </Card>
          ))}
        </Grid>
      </SpaceBetween>
    </Container>
  );
}
