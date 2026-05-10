'use client';

import {
  ContentLayout,
  Header,
  Table,
  Button,
  SpaceBetween,
  Box,
  Spinner,
  TextFilter,
  Alert,
} from '@cloudscape-design/components';
import { useState, useEffect } from 'react';
import { useI18n } from '@/app/i18n-provider';

interface Attachment {
  id: string;
  filename: string;
  size_bytes: number;
  upload_date: string;
  last_accessed: string;
}

export default function AttachmentsPage() {
  const { t } = useI18n();
  const [attachments, setAttachments] = useState<Attachment[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('');
  const [cleanupLoading, setCleanupLoading] = useState(false);
  const [cleanupSuccess, setCleanupSuccess] = useState(false);

  useEffect(() => {
    fetchAttachments();
  }, []);

  const fetchAttachments = async () => {
    setLoading(true);
    try {
      const res = await fetch('/api/admin/attachments?limit=100', {
        credentials: 'include',
      });
      if (res.ok) {
        const data = await res.json();
        setAttachments(data.attachments || []);
      }
    } catch (error) {
      console.error('Failed to fetch attachments:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleCleanup = async () => {
    setCleanupLoading(true);
    try {
      await fetch('/api/admin/attachment-cleanup/runs', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({}),
        credentials: 'include',
      });
      setCleanupSuccess(true);
      setTimeout(() => setCleanupSuccess(false), 3000);
    } catch (error) {
      console.error('Failed to run cleanup:', error);
    } finally {
      setCleanupLoading(false);
    }
  };

  const filteredAttachments = attachments.filter((a) =>
    a.filename.toLowerCase().includes(filter.toLowerCase())
  );

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.attachments.title')}</Header>}>
        <Box textAlign="center" padding="xl">
          <Spinner />
        </Box>
      </ContentLayout>
    );
  }

  return (
    <ContentLayout
      header={
        <Header
          variant="h1"
          description={t('pages.attachments_page.description')}
          actions={
            <Button
              variant="primary"
              onClick={handleCleanup}
              loading={cleanupLoading}
              disabled={cleanupLoading}
            >
              {t('pages.attachments_page.run_cleanup')}
            </Button>
          }
        >
          {t('pages.attachments.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        {cleanupSuccess && (
          <Alert type="success">{t('pages.attachments_page.cleanup_success')}</Alert>
        )}

        <Table
          columnDefinitions={[
            {
              header: t('pages.attachments_page.filename'),
              cell: (item: Attachment) => item.filename,
              width: '30%',
            },
            {
              header: t('pages.attachments_page.size'),
              cell: (item: Attachment) =>
                `${(item.size_bytes / 1024 / 1024).toFixed(2)} MB`,
              width: '15%',
            },
            {
              header: t('pages.attachments_page.upload_date'),
              cell: (item: Attachment) => new Date(item.upload_date).toLocaleString(),
              width: '25%',
            },
            {
              header: t('pages.attachments_page.last_accessed'),
              cell: (item: Attachment) =>
                item.last_accessed
                  ? new Date(item.last_accessed).toLocaleString()
                  : t('pages.attachments_page.never'),
              width: '30%',
            },
          ]}
          items={filteredAttachments}
          header={
            <Header variant="h2" counter={`(${filteredAttachments.length})`}>
              {t('pages.attachments.title')}
            </Header>
          }
          filter={
            <TextFilter
              filteringText={filter}
              filteringPlaceholder={t('common.search')}
              onChange={(e) => setFilter(e.detail.filteringText)}
            />
          }
        />
      </SpaceBetween>
    </ContentLayout>
  );
}
