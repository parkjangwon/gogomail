'use client';
import {
  Container,
  Header,
  Box,
  SpaceBetween,
  Button,
  StatusIndicator,
} from '@cloudscape-design/components';
import { useRouter } from 'next/navigation';
import { formatDateTime } from '@/lib/format';
import { DomainDetail } from './domainDetailTypes';

interface Props {
  domain: DomainDetail;
  companyId: string;
  verifying: boolean;
  onVerifyDNS: () => void;
  t: (key: string, fallback?: string) => string;
}

export function DomainDNSTab({ domain, companyId, verifying, onVerifyDNS, t }: Props) {
  const router = useRouter();

  return (
    <SpaceBetween size="l">
      <Container
        header={
          <Header variant="h2" actions={<Button variant="primary" onClick={onVerifyDNS} loading={verifying}>{t('pages.domain_detail.run_full_verification')}</Button>}>
            {t('pages.domain_detail.dns_health_check')}
          </Header>
        }
      >
        <SpaceBetween size="m">
          <StatusIndicator type={domain.last_dns_check_status === 'pass' ? 'success' : domain.last_dns_check_status === 'fail' ? 'error' : 'pending'}>
            {t('pages.domain_detail.overall')} {domain.last_dns_check_status || t('pages.domain_detail.not_checked')}
          </StatusIndicator>
          <Box color="text-body-secondary">
            {t('pages.domain_detail.dns_setup_desc')} <strong>{domain.name}</strong>.
          </Box>
          {domain.last_dns_checked_at ? (
            <Box color="text-body-secondary" fontSize="body-s">
              {t('pages.domain_detail.last_checked')}: {formatDateTime(domain.last_dns_checked_at)}
            </Box>
          ) : null}
        </SpaceBetween>
      </Container>
      <Container header={<Header variant="h3">{t('pages.domain_detail.dkim_keys')}</Header>}>
        <SpaceBetween size="s">
          <Box color="text-body-secondary">{t('pages.domain_detail.manage_dkim_desc')}</Box>
          <Button onClick={() => router.push(`/companies/${companyId}/security/dkim-keys`)}>
            {t('pages.domain_detail.manage_dkim_btn')} →
          </Button>
        </SpaceBetween>
      </Container>
    </SpaceBetween>
  );
}
