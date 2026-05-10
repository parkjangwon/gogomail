'use client';

import {
  ContentLayout,
  Header,
  Box,
  Spinner,
  SpaceBetween,
  Container,
  ColumnLayout,
  ProgressBar,
  KeyValuePairs,
  Button,
  StatusIndicator,
} from '@cloudscape-design/components';
import { useParams, useRouter } from 'next/navigation';
import { useDashboard } from '@/hooks/useDashboard';
import { useCompany } from '@/contexts/CompanyContext';
import { useI18n } from '@/app/i18n-provider';

export default function DashboardPage() {
  const { t } = useI18n();
  const params = useParams();
  const router = useRouter();
  const companyId = params.id as string;
  const { currentCompany } = useCompany();
  const { data, isLoading } = useDashboard(companyId);

  if (isLoading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.dashboard_page.title')}</Header>}>
        <Box textAlign="center" padding="xl"><Spinner /></Box>
      </ContentLayout>
    );
  }

  const stats = data?.stats ?? {
    total_users: 0,
    active_domains: 0,
    total_storage_used: 0,
    total_storage_limit: 0,
    over_allocated: false,
  };

  const storageUsed = stats.total_storage_used;
  const storageLimit = stats.total_storage_limit;
  const storageUsedGb = (storageUsed / 1073741824).toFixed(1);
  const storageLimitGb = storageLimit > 0 ? (storageLimit / 1073741824).toFixed(1) : null;
  const storagePct = storageLimit > 0 ? Math.round((storageUsed / storageLimit) * 100) : 0;

  const company = currentCompany;

  const quickLinks = [
    { labelKey: 'manage_users', path: '/users' },
    { labelKey: 'manage_domains', path: '/tenancy/domains' },
    { labelKey: 'audit_logs', path: '/audit-logs' },
    { labelKey: 'dkim_keys', path: '/security/dkim-keys' },
    { labelKey: 'quota_usage', path: '/storage/quota-usage' },
    { labelKey: 'api_health', path: '/system/health' },
  ];

  return (
    <ContentLayout
      header={
        <Header
          variant="h1"
          description={company ? `${company.name} — ${company.status}` : t('pages.dashboard_page.system_overview')}
          actions={
            <SpaceBetween direction="horizontal" size="xs">
              <Button onClick={() => router.push(`/companies/${companyId}/tenancy/domains`)}>
                {t('pages.dashboard_page.domains_btn')}
              </Button>
              <Button onClick={() => router.push(`/companies/${companyId}/users`)}>
                {t('pages.dashboard_page.users_btn')}
              </Button>
              <Button variant="primary" onClick={() => router.push(`/companies/${companyId}`)}>
                {t('pages.dashboard_page.company_overview_btn')}
              </Button>
            </SpaceBetween>
          }
        >
          {t('pages.dashboard_page.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        {/* KPI row */}
        <ColumnLayout columns={3} variant="text-grid" minColumnWidth={170}>
          <Container>
            <SpaceBetween size="xs">
              <Box fontSize="display-l" fontWeight="bold">{stats.total_users}</Box>
              <Box color="text-body-secondary" fontSize="body-s">{t('pages.dashboard_page.total_users')}</Box>
            </SpaceBetween>
          </Container>
          <Container>
            <SpaceBetween size="xs">
              <Box fontSize="display-l" fontWeight="bold">{stats.active_domains}</Box>
              <Box color="text-body-secondary" fontSize="body-s">{t('pages.dashboard_page.active_domains')}</Box>
            </SpaceBetween>
          </Container>
          <Container>
            <SpaceBetween size="xs">
              <Box fontSize="display-l" fontWeight="bold">
                {`${storageUsedGb} GB`}
              </Box>
              <Box color="text-body-secondary" fontSize="body-s">
                {storageLimitGb
                  ? `${t('pages.dashboard_page.used_label')} / ${storageLimitGb} GB`
                  : t('pages.dashboard_page.unlimited_storage')}
              </Box>
            </SpaceBetween>
          </Container>
        </ColumnLayout>

        <ColumnLayout columns={2}>
          {/* Storage */}
          <Container header={<Header variant="h3">{t('pages.dashboard_page.storage_quota')}</Header>}>
            <SpaceBetween size="m">
              {storageLimit > 0 ? (
                <>
                  <ProgressBar
                    value={storagePct}
                    status={stats.over_allocated ? 'error' : storagePct > 80 ? 'in-progress' : 'success'}
                    resultText={`${storagePct}%`}
                    additionalInfo={`${storageUsedGb} / ${storageLimitGb} GB`}
                  />
                  {stats.over_allocated && (
                    <StatusIndicator type="error">{t('pages.dashboard_page.over_allocated')}</StatusIndicator>
                  )}
                </>
              ) : (
                <SpaceBetween size="s">
                  <StatusIndicator type="success">{t('pages.dashboard_page.unlimited_storage')}</StatusIndicator>
                  <Box color="text-body-secondary" fontSize="body-s">{storageUsedGb} GB {t('pages.dashboard_page.used_label')}</Box>
                </SpaceBetween>
              )}
              <KeyValuePairs
                items={[
                  { label: t('pages.dashboard_page.used_label'), value: `${storageUsedGb} GB` },
                  { label: t('pages.dashboard_page.limit_label'), value: storageLimitGb ? `${storageLimitGb} GB` : t('pages.dashboard_page.unlimited_label') },
                ]}
              />
            </SpaceBetween>
          </Container>

          {/* Quick Links */}
          <Container header={<Header variant="h3">{t('pages.dashboard_page.quick_actions')}</Header>}>
            <SpaceBetween size="s">
              {quickLinks.map(({ labelKey, path }) => (
                <Button
                  key={labelKey}
                  variant="inline-link"
                  onClick={() => router.push(`/companies/${companyId}${path}`)}
                >
                  {t(`pages.dashboard_page.${labelKey}`)} →
                </Button>
              ))}
            </SpaceBetween>
          </Container>
        </ColumnLayout>
      </SpaceBetween>
    </ContentLayout>
  );
}
