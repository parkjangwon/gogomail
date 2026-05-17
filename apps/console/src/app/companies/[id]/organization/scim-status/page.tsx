'use client';

import {
  ContentLayout,
  Header,
  Container,
  Box,
  Spinner,
  SpaceBetween,
  ColumnLayout,
  StatusIndicator,
  CopyToClipboard,
  Alert,
} from '@cloudscape-design/components';
import { useState, useEffect } from 'react';
import { useParams } from 'next/navigation';
import { useI18n } from '@/app/i18n-provider';
import { useSCIMStatus } from '@/hooks';

interface SCIMStatus {
  endpoint: string;
  supported_resources: string[];
  domain_id: string;
  user_count: number;
  status: string;
}

export default function SCIMStatusPage() {
  const { t } = useI18n();
  const params = useParams();
  const companyId = params?.id as string;
  const scimQuery = useSCIMStatus(companyId);
  const [data, setData] = useState<SCIMStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    setLoading(scimQuery.isLoading);
    if (scimQuery.data) setData(scimQuery.data as SCIMStatus);
    if (scimQuery.isError) setError(t('scim_status.failed_load'));
  }, [scimQuery.data, scimQuery.isError, scimQuery.isLoading, t]);

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('scim_status.title')}</Header>}>
        <Box textAlign="center" padding="xl"><Spinner /></Box>
      </ContentLayout>
    );
  }

  if (error || !data) {
    return (
      <ContentLayout header={<Header variant="h1">{t('scim_status.title')}</Header>}>
        <StatusIndicator type="error">{error || t('scim_status.failed_load')}</StatusIndicator>
      </ContentLayout>
    );
  }

  const backendBase = process.env.NEXT_PUBLIC_GOGOMAIL_PUBLIC_BASE_URL
    || (typeof window !== 'undefined' ? window.location.origin : '');
  const scimEndpointUrl = `${backendBase}${data.endpoint}`;

  return (
    <ContentLayout
      header={
        <Header variant="h1" description={t('scim_status.description')}>
          {t('scim_status.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        <Alert type="info" header={t('scim_status.setup_header')}>
          {t('scim_status.setup_desc')}
        </Alert>

        <Container header={<Header variant="h2">{t('scim_status.connection_details')}</Header>}>
          <SpaceBetween size="m">
            <ColumnLayout columns={2} variant="text-grid">
              <SpaceBetween size="xxs">
                <Box fontWeight="bold" fontSize="body-s" color="text-body-secondary">{t('scim_status.status')}</Box>
                <StatusIndicator type={data.status === 'active' ? 'success' : 'warning'}>
                  {data.status === 'active' ? t('scim_status.status_active') : data.status}
                </StatusIndicator>
              </SpaceBetween>
              <SpaceBetween size="xxs">
                <Box fontWeight="bold" fontSize="body-s" color="text-body-secondary">{t('scim_status.supported_resources')}</Box>
                <Box>{data.supported_resources?.join(', ') || '—'}</Box>
              </SpaceBetween>
              <SpaceBetween size="xxs">
                <Box fontWeight="bold" fontSize="body-s" color="text-body-secondary">{t('scim_status.domain_id')}</Box>
                <Box fontSize="body-s"><code>{data.domain_id || '—'}</code></Box>
              </SpaceBetween>
              <SpaceBetween size="xxs">
                <Box fontWeight="bold" fontSize="body-s" color="text-body-secondary">{t('scim_status.provisioned_users')}</Box>
                <Box fontSize="display-l" fontWeight="bold">{data.user_count}</Box>
              </SpaceBetween>
            </ColumnLayout>

            <SpaceBetween size="xxs">
              <Box fontWeight="bold" fontSize="body-s" color="text-body-secondary">{t('scim_status.endpoint_url')}</Box>
              <CopyToClipboard
                copyButtonText={t('scim_status.copy')}
                copySuccessText={t('scim_status.copy_success')}
                copyErrorText={t('scim_status.copy_error')}
                textToCopy={scimEndpointUrl}
                variant="inline"
              />
              <Box fontSize="body-s" color="text-body-secondary"><code>{scimEndpointUrl}</code></Box>
            </SpaceBetween>
          </SpaceBetween>
        </Container>

        <Container header={<Header variant="h2">{t('scim_status.supported_ops')}</Header>}>
          <ColumnLayout columns={3} variant="text-grid">
            {[
              { op: 'GET /Users', desc: t('scim_status.op_get_users') },
              { op: 'POST /Users', desc: t('scim_status.op_post_users') },
              { op: 'GET /Users/{id}', desc: t('scim_status.op_get_user') },
              { op: 'PUT /Users/{id}', desc: t('scim_status.op_put_user') },
              { op: 'PATCH /Users/{id}', desc: t('scim_status.op_patch_user') },
              { op: 'DELETE /Users/{id}', desc: t('scim_status.op_delete_user') },
            ].map(({ op, desc }) => (
              <SpaceBetween key={op} size="xxs">
                <Box fontSize="body-s" fontWeight="bold"><code>{op}</code></Box>
                <Box color="text-body-secondary" fontSize="body-s">{desc}</Box>
              </SpaceBetween>
            ))}
          </ColumnLayout>
        </Container>
      </SpaceBetween>
    </ContentLayout>
  );
}
