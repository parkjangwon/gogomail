'use client';
import {
  Container,
  Header,
  Box,
  SpaceBetween,
  Spinner,
  Button,
} from '@cloudscape-design/components';
import { DailyCount } from './domainDetailTypes';

interface Props {
  mailStats: DailyCount[];
  statsLoading: boolean;
  statsFetched: boolean;
  onFetchStats: () => void;
  t: (key: string, fallback?: string) => string;
}

export function DomainStatsTab({ mailStats, statsLoading, onFetchStats, t }: Props) {
  return (
    <Container header={
      <Header
        variant="h2"
        description={t('pages.domain_detail.mail_stats_desc')}
        actions={
          <Button iconName="refresh" loading={statsLoading} onClick={onFetchStats}>
            {t('common.refresh')}
          </Button>
        }
      >
        {t('pages.domain_detail.daily_message_volume')}
      </Header>
    }>
      {statsLoading ? (
        <Box textAlign="center" padding="xl"><Spinner /></Box>
      ) : (
        <SpaceBetween size="l">
          {mailStats.length === 0 ? (
            <Box color="text-body-secondary" textAlign="center" padding="l">{t('pages.domain_detail.no_mail_data_7d')}</Box>
          ) : (() => {
            const maxCount = Math.max(...mailStats.map(d => d.total), 1);
            return (
              <SpaceBetween size="m">
                <div style={{ display: 'flex', alignItems: 'flex-end', gap: '12px', height: '140px', padding: '0 8px' }}>
                  {mailStats.map(day => (
                    <div key={day.date} style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', flex: 1, height: '100%', justifyContent: 'flex-end' }}>
                      {day.total > 0 && (
                        <Box fontSize="body-s" color="text-body-secondary">{day.total}</Box>
                      )}
                      <div style={{ width: '100%', height: `${(day.total / maxCount) * 110}px`, minHeight: day.total > 0 ? '4px' : '0', display: 'flex', flexDirection: 'column', borderRadius: '3px', overflow: 'hidden' }}>
                        <div style={{ flex: day.success, backgroundColor: '#1d8348' }} />
                        <div style={{ flex: day.failed, backgroundColor: '#e74c3c' }} />
                      </div>
                    </div>
                  ))}
                </div>
                <div style={{ display: 'flex', gap: '12px', padding: '0 8px' }}>
                  {mailStats.map(day => (
                    <div key={day.date} style={{ flex: 1, textAlign: 'center' }}>
                      <Box fontSize="body-s" color="text-body-secondary">{day.label}</Box>
                    </div>
                  ))}
                </div>
                <SpaceBetween direction="horizontal" size="l">
                  <Box fontSize="body-s"><span style={{ display: 'inline-block', width: 12, height: 12, backgroundColor: '#1d8348', borderRadius: 2, marginRight: 6 }} />{t('pages.domain_detail.delivered')}</Box>
                  <Box fontSize="body-s"><span style={{ display: 'inline-block', width: 12, height: 12, backgroundColor: '#e74c3c', borderRadius: 2, marginRight: 6 }} />{t('pages.domain_detail.failed')}</Box>
                  <Box fontSize="body-s" color="text-body-secondary">
                    {t('pages.domain_detail.total_last_7d')}: {mailStats.reduce((s, d) => s + d.total, 0)} {t('pages.domain_detail.messages')}
                  </Box>
                </SpaceBetween>
              </SpaceBetween>
            );
          })()}
        </SpaceBetween>
      )}
    </Container>
  );
}
