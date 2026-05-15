'use client';
import { DataTable } from '@/components/DataTable';
import {
  ContentLayout,
  Header,
  SpaceBetween,
  Box,
  Spinner,
  TextFilter,
  ProgressBar,
  Badge,
} from '@cloudscape-design/components';
import { useMemo, useState } from 'react';
import { useParams } from 'next/navigation';
import { useI18n } from '@/app/i18n-provider';
import { useQuotaUsage, type QuotaUsage } from '@/hooks';

export default function QuotaUsagePage() {
  const { t } = useI18n();
  const params = useParams();
  const companyId = params?.id as string;
  const { data: quotas = [], isLoading } = useQuotaUsage(companyId);
  const [filter, setFilter] = useState('');

  const filteredQuotas = useMemo(
    () =>
      quotas.filter((quota) =>
        [quota.name, quota.scope, quota.domain_id, quota.id]
          .filter((value): value is string => Boolean(value))
          .some((value) => value.toLowerCase().includes(filter.toLowerCase()))
      ),
    [quotas, filter]
  );

  const scopeColor = (scope: QuotaUsage['scope']) => {
    switch (scope) {
      case 'company':
        return 'blue';
      case 'domain':
        return 'green';
      case 'user':
        return 'grey';
      default:
        return 'grey';
    }
  };

  if (isLoading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.quota_usage.title')}</Header>}>
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
          description={t('pages.quota_usage_page.description')}
        >
          {t('pages.quota_usage_page.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        <DataTable
          columnDefinitions={[
            {
              header: 'Scope',
              cell: (item: QuotaUsage) => <Badge color={scopeColor(item.scope)}>{item.scope}</Badge>,
              width: '25%',
            },
            {
              header: t('pages.quota_usage_page.name'),
              cell: (item: QuotaUsage) => item.name,
              width: '22%',
            },
            {
              header: t('pages.quota_usage_page.used'),
              cell: (item: QuotaUsage) => item.quota_used.toLocaleString(),
              width: '12%',
            },
            {
              header: t('pages.quota_usage_page.quota'),
              cell: (item: QuotaUsage) => item.quota_limit.toLocaleString(),
              width: '12%',
            },
            {
              header: t('pages.quota_usage_page.usage'),
              cell: (item: QuotaUsage) => (
                <Box>
                  <ProgressBar value={item.usage_ratio * 100} />
                  <Box color="text-body-secondary" fontSize="body-s">
                    {(item.usage_ratio * 100).toFixed(1)}%
                  </Box>
                </Box>
              ),
              width: '18%',
            },
            {
              header: t('pages.quota_usage_page.last_updated'),
              cell: (item: QuotaUsage) => new Date(item.updated_at).toLocaleString(),
              width: '13%',
            },
          ]}
          items={filteredQuotas}
          header={<Header variant="h2" counter={`(${filteredQuotas.length})`}>{t('pages.quota_usage_page.quotas')}</Header>}
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
