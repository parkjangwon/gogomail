'use client';

import {
  Container,
  Header,
  SpaceBetween,
  Button,
  FormField,
  Input,
  Modal,
  RadioGroup,
  Box,
  ColumnLayout,
  Textarea,
} from '@cloudscape-design/components';
import { useState, useMemo } from 'react';
import { DataTable } from '@/components/DataTable';
import { SpamFilterEvent, EventFilter } from './spamFilterTypes';

interface SpamFilterEventsTableProps {
  events: SpamFilterEvent[];
  lastUpdated: Date | null;
  refreshing: boolean;
  onRefresh: () => void;
  locale: string;
  t: (key: string, defaultValue?: string) => string;
}

export function SpamFilterEventsTable({ events, lastUpdated, refreshing, onRefresh, t }: SpamFilterEventsTableProps) {
  const [eventFilter, setEventFilter] = useState<EventFilter>('all');
  const [eventFrom, setEventFrom] = useState('');
  const [eventTo, setEventTo] = useState('');
  const [eventMinScore, setEventMinScore] = useState('');
  const [detailEvent, setDetailEvent] = useState<SpamFilterEvent | null>(null);

  const filteredEvents = useMemo(() => {
    const minScore = eventMinScore.trim() === '' ? null : Number(eventMinScore);
    const fromTime = eventFrom ? Date.parse(eventFrom) : null;
    const toTime = eventTo ? Date.parse(eventTo) : null;
    return events.filter(event => {
      const created = event.created_at ? Date.parse(event.created_at) : 0;
      if (fromTime && created && created < fromTime) return false;
      if (toTime && created && created > toTime) return false;
      if (minScore !== null && !Number.isNaN(minScore) && (event.spam_score ?? 0) < minScore) return false;
      if (eventFilter === 'all') return true;
      const status = `${event.flow_status ?? ''} ${event.enhanced_status ?? ''} ${event.error_message ?? ''}`.toLowerCase();
      if (eventFilter === 'rejected') return status.includes('reject') || status.includes('blocked');
      if (eventFilter === 'delivered') return status.includes('deliver') || status.includes('accept');
      return status.includes('filter') || status.includes('spam') || status.includes('quarantine');
    });
  }, [eventFilter, eventFrom, eventMinScore, eventTo, events]);

  return (
    <>
      <Container
        header={
          <Header
            variant="h2"
            counter={`(${filteredEvents.length})`}
            actions={
              <SpaceBetween direction="horizontal" size="xs">
                {lastUpdated && <Box color="text-body-secondary">{t('pages.spam_filter_page.last_updated')}: {lastUpdated.toLocaleTimeString()}</Box>}
                <Button onClick={onRefresh} loading={refreshing}>{t('pages.spam_filter_page.refresh')}</Button>
              </SpaceBetween>
            }
          >
            {t('pages.spam_filter_page.events_section')}
          </Header>
        }
      >
        <SpaceBetween size="m">
          <RadioGroup
            value={eventFilter}
            onChange={e => setEventFilter(e.detail.value as EventFilter)}
            items={[
              { value: 'all', label: t('pages.spam_filter_page.event_filter_all') },
              { value: 'filtered', label: t('pages.spam_filter_page.event_filter_filtered') },
              { value: 'rejected', label: t('pages.spam_filter_page.event_filter_rejected') },
              { value: 'delivered', label: t('pages.spam_filter_page.event_filter_delivered') },
            ]}
          />
          <ColumnLayout columns={3}>
            <FormField label={t('pages.spam_filter_page.event_from_label')}>
              <Input value={eventFrom} onChange={event => setEventFrom(event.detail.value)} placeholder="2026-05-17T00:00:00Z" />
            </FormField>
            <FormField label={t('pages.spam_filter_page.event_to_label')}>
              <Input value={eventTo} onChange={event => setEventTo(event.detail.value)} placeholder="2026-05-17T23:59:59Z" />
            </FormField>
            <FormField label={t('pages.spam_filter_page.event_min_score_label')}>
              <Input type="number" value={eventMinScore} onChange={event => setEventMinScore(event.detail.value)} placeholder="5" />
            </FormField>
          </ColumnLayout>
          <DataTable
            searchPlaceholder={t('pages.spam_filter_page.events_search')}
            columnDefinitions={[
              {
                header: t('pages.spam_filter_page.col_time'),
                cell: (item: SpamFilterEvent) => item.created_at ? new Date(item.created_at).toLocaleString() : '—',
                width: '16%',
              },
              {
                header: t('pages.spam_filter_page.col_from'),
                cell: (item: SpamFilterEvent) => item.from_addr || item.mail_from || '—',
                width: '18%',
              },
              {
                header: t('pages.spam_filter_page.col_subject'),
                cell: (item: SpamFilterEvent) => item.subject || '—',
                width: '24%',
              },
              {
                header: t('pages.spam_filter_page.col_action'),
                cell: (item: SpamFilterEvent) => item.enhanced_status || item.flow_status,
                width: '10%',
              },
              {
                header: t('pages.spam_filter_page.col_score'),
                cell: (item: SpamFilterEvent) => item.spam_score?.toFixed(1) ?? '—',
                width: '8%',
              },
              {
                header: t('pages.spam_filter_page.col_reason'),
                cell: (item: SpamFilterEvent) => item.error_message || '—',
                width: '24%',
              },
              {
                header: t('pages.spam_filter_page.col_manage'),
                cell: (item: SpamFilterEvent) => (
                  <Button variant="inline-link" onClick={() => setDetailEvent(item)}>
                    {t('pages.spam_filter_page.view_details')}
                  </Button>
                ),
                width: '10%',
              },
            ]}
            items={filteredEvents}
            header={<Header variant="h3">{t('pages.spam_filter_page.events_table_title')}</Header>}
          />
        </SpaceBetween>
      </Container>

      <Modal
        visible={detailEvent !== null}
        onDismiss={() => setDetailEvent(null)}
        header={t('pages.spam_filter_page.event_detail_title')}
        footer={<Button onClick={() => setDetailEvent(null)}>{t('common.close', 'Close')}</Button>}
      >
        {detailEvent && (
          <SpaceBetween size="s">
            <ColumnLayout columns={2}>
              <Box><Box variant="awsui-key-label">{t('pages.spam_filter_page.col_time')}</Box>{detailEvent.created_at ? new Date(detailEvent.created_at).toLocaleString() : '—'}</Box>
              <Box><Box variant="awsui-key-label">{t('pages.spam_filter_page.col_score')}</Box>{detailEvent.spam_score?.toFixed(1) ?? '—'}</Box>
              <Box><Box variant="awsui-key-label">{t('pages.spam_filter_page.col_from')}</Box>{detailEvent.from_addr || detailEvent.mail_from || '—'}</Box>
              <Box><Box variant="awsui-key-label">{t('pages.spam_filter_page.col_action')}</Box>{detailEvent.enhanced_status || detailEvent.flow_status}</Box>
              <Box><Box variant="awsui-key-label">SPF</Box>{detailEvent.spf_result || '—'}</Box>
              <Box><Box variant="awsui-key-label">DKIM</Box>{detailEvent.dkim_result || '—'}</Box>
              <Box><Box variant="awsui-key-label">DMARC</Box>{detailEvent.dmarc_result || '—'}</Box>
              <Box><Box variant="awsui-key-label">{t('pages.spam_filter_page.col_subject')}</Box>{detailEvent.subject || '—'}</Box>
            </ColumnLayout>
            <FormField label={t('pages.spam_filter_page.col_reason')}>
              <Textarea value={detailEvent.error_message || '—'} readOnly rows={5} />
            </FormField>
          </SpaceBetween>
        )}
      </Modal>
    </>
  );
}
