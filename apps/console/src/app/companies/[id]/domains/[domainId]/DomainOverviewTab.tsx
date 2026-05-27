'use client';
import {
  Container,
  Header,
  ColumnLayout,
  SpaceBetween,
  Box,
  ProgressBar,
  StatusIndicator,
  Badge,
  KeyValuePairs,
  Alert,
  Button,
} from '@cloudscape-design/components';
import { formatDateTime } from '@/lib/format';
import { DomainDetail, User } from './domainDetailTypes';

interface Props {
  domain: DomainDetail;
  users: User[];
  verifying: boolean;
  onVerifyDNS: () => void;
  onSetActiveTab: (tab: string) => void;
  t: (key: string, fallback?: string) => string;
}

export function DomainOverviewTab({ domain, users, verifying, onVerifyDNS, onSetActiveTab, t }: Props) {
  const quotaPct = domain.quota_limit > 0 ? Math.round((domain.quota_used / domain.quota_limit) * 100) : 0;
  const dnsColor = domain.last_dns_check_status === 'pass' ? 'green' : domain.last_dns_check_status === 'fail' ? 'red' : 'grey';

  return (
    <SpaceBetween size="l">
      <ColumnLayout columns={3}>
        <Container header={<Header variant="h3">{t('pages.domain_detail.storage')}</Header>}>
          <SpaceBetween size="s">
            <ProgressBar
              value={quotaPct}
              status={domain.over_allocated ? 'error' : quotaPct > 80 ? 'in-progress' : 'success'}
              resultText={`${quotaPct}%`}
              additionalInfo={
                domain.quota_limit > 0
                  ? `${(domain.quota_used / 1073741824).toFixed(2)} / ${(domain.quota_limit / 1073741824).toFixed(2)} GB`
                  : `${(domain.quota_used / 1073741824).toFixed(2)} GB (unlimited)`
              }
            />
            {domain.over_allocated ? (
              <StatusIndicator type="error">{t('pages.domain_detail.over_allocated')}</StatusIndicator>
            ) : null}
          </SpaceBetween>
        </Container>

        <Container header={<Header variant="h3">{t('pages.domain_detail.users')}</Header>}>
          <SpaceBetween size="s">
            <Box fontSize="display-l" fontWeight="bold">{users.length}</Box>
            <Box color="text-body-secondary" fontSize="body-s">
              {users.filter(u => u.status === 'active').length} {t('pages.domain_detail.active')}
            </Box>
            <Button variant="inline-link" onClick={() => onSetActiveTab('users')}>
              {t('pages.domain_detail.view_all')} →
            </Button>
          </SpaceBetween>
        </Container>

        <Container header={<Header variant="h3">{t('pages.domain_detail.domain_info')}</Header>}>
          <KeyValuePairs
            items={[
              { label: t('pages.domain_detail.status'), value: <Badge color={domain.status === 'active' ? 'green' : 'grey'}>{domain.status}</Badge> },
              { label: t('pages.domain_detail.dns'), value: <Badge color={dnsColor as 'green' | 'red' | 'grey'}>{domain.last_dns_check_status || t('pages.domain_detail.unchecked')}</Badge> },
              { label: t('pages.domain_detail.created'), value: new Date(domain.created_at).toLocaleDateString() },
              ...(domain.last_dns_checked_at ? [{ label: t('pages.domain_detail.last_checked'), value: formatDateTime(domain.last_dns_checked_at) }] : []),
            ]}
          />
        </Container>
      </ColumnLayout>

      {domain.last_dns_check_status !== 'pass' ? (
        <Alert
          type={domain.last_dns_check_status === 'fail' ? 'error' : 'warning'}
          header={t('pages.domain_detail.dns_verification_required')}
          action={<Button onClick={onVerifyDNS} loading={verifying}>{t('pages.domain_detail.run_verification')}</Button>}
        >
          {t('pages.domain_detail.dns_verification_desc')}
        </Alert>
      ) : null}
    </SpaceBetween>
  );
}
