'use client';

import {
  Container,
  Header,
  SpaceBetween,
  Box,
  Grid,
  LineChart,
  AreaChart,
} from '@cloudscape-design/components';

export default function MonitoringPage() {
  return (
    <Container header={<Header>시스템 모니터링</Header>}>
      <SpaceBetween size="l">
        <Grid gridDefinition={[{ colspan: 6 }, { colspan: 6 }]}>
          <Box>
            <h3>CPU 사용률</h3>
            <p style={{ marginTop: '20px', color: '#666', textAlign: 'center' }}>
              실시간 CPU 사용률: 45%
            </p>
          </Box>
          <Box>
            <h3>메모리 사용률</h3>
            <p style={{ marginTop: '20px', color: '#666', textAlign: 'center' }}>
              실시간 메모리 사용률: 62%
            </p>
          </Box>
        </Grid>

        <Box>
          <h3>디스크 사용률</h3>
          <p style={{ marginTop: '20px', color: '#666' }}>
            - 루트 파티션: 75% (750GB / 1TB)
            <br />- 데이터 파티션: 42% (420GB / 1TB)
          </p>
        </Box>

        <Box>
          <h3>네트워크 트래픽</h3>
          <p style={{ marginTop: '20px', color: '#666' }}>
            - 인바운드: 125 Mbps
            <br />- 아웃바운드: 89 Mbps
          </p>
        </Box>

        <Box>
          <h3>활성 연결 수</h3>
          <p style={{ marginTop: '20px', color: '#666' }}>
            - HTTP/HTTPS: 342
            <br />- SMTP: 28
            <br />- IMAP: 156
            <br />- 총합: 526
          </p>
        </Box>

        <Box>
          <h3>데이터베이스 상태</h3>
          <p style={{ marginTop: '20px', color: '#666' }}>
            - 상태: 정상 ✓
            <br />- 연결 풀: 45/50
            <br />- 응답 시간: 12ms
          </p>
        </Box>
      </SpaceBetween>
    </Container>
  );
}
