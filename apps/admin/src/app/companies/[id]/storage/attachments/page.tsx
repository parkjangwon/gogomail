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

  useEffect(() => {
    fetchAttachments();
  }, []);

  const fetchAttachments = async () => {
    setLoading(true);
    try {
      const res = await fetch('/api/admin/attachments?limit=100', {
        credentials: 'include'
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

  const filteredAttachments = attachments.filter(a =>
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
          description="Manage and cleanup attachment storage"
          actions={
            <Button variant="primary" disabled>
              Cleanup Stale
            </Button>
          }
        >
          Attachments
        </Header>
      }
    >
      <SpaceBetween size="l">
        <Table
          columnDefinitions={[
            {
              header: 'Filename',
              cell: (item: Attachment) => item.filename,
              width: '30%',
            },
            {
              header: 'Size',
              cell: (item: Attachment) => `${(item.size_bytes / 1024 / 1024).toFixed(2)} MB`,
              width: '15%',
            },
            {
              header: 'Upload Date',
              cell: (item: Attachment) => new Date(item.upload_date).toLocaleString(),
              width: '25%',
            },
            {
              header: 'Last Accessed',
              cell: (item: Attachment) => item.last_accessed ? new Date(item.last_accessed).toLocaleString() : 'Never',
              width: '30%',
            },
          ]}
          items={filteredAttachments}
          header={<Header variant="h2" counter={`(${filteredAttachments.length})`}>{t('pages.attachments.title')}</Header>}
          filter={
            <TextFilter
              filteringText={filter}
              onChange={(e) => setFilter(e.detail.filteringText)}
            />
          }
        />
      </SpaceBetween>
    </ContentLayout>
  );
}
