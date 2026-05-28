'use client';
import {
  Container,
  Header,
  SpaceBetween,
  Button,
  StatusIndicator,
  Alert,
  Box,
} from '@cloudscape-design/components';
import { CopyToClipboard } from '@cloudscape-design/components';
import { DataTable } from '@/components/DataTable';
import { useI18n } from '@/app/i18n-provider';
import type { DnsCheckResult, CreatedDomain, Step4Data } from './types';

interface DnsRecord {
  type: string;
  host: string;
  value: string;
  checked: 'ok' | 'fail' | 'pending';
}

interface Props {
  createdDomain: CreatedDomain | null;
  dnsCheck: DnsCheckResult;
  dnsChecking: boolean;
  dnsCheckError: string;
  handleCheckDns: () => Promise<void>;
  domainName: string;
  step4Selector: Step4Data['selector'];
  step4PublicKeyDns: Step4Data['public_key_dns'];
}

export function OnboardingStep3({
  createdDomain,
  dnsCheck,
  dnsChecking,
  dnsCheckError,
  handleCheckDns,
  domainName,
  step4Selector,
  step4PublicKeyDns,
}: Props) {
  const { t } = useI18n();

  const dnsRecords: DnsRecord[] = [
    {
      type: 'MX',
      host: domainName,
      value: `10 mail.${domainName}`,
      checked: dnsCheck.checked ? (dnsCheck.mx ? 'ok' : 'fail') : 'pending',
    },
    {
      type: 'TXT (SPF)',
      host: domainName,
      value: `v=spf1 include:_spf.${domainName} ~all`,
      checked: dnsCheck.checked ? (dnsCheck.spf ? 'ok' : 'fail') : 'pending',
    },
    {
      type: 'TXT (DKIM)',
      host: `${step4Selector || 'default'}._domainkey.${domainName}`,
      value: step4PublicKeyDns || '(configure DKIM in Step 4)',
      checked: dnsCheck.checked ? (dnsCheck.dkim ? 'ok' : 'fail') : 'pending',
    },
  ];

  return (
    <Container header={<Header variant="h2">{t('onboarding.step3_title')}</Header>}>
      <SpaceBetween size="l">
        <Alert type="info">{t('onboarding.step3_info')}</Alert>
        <DataTable
          header={<Header variant="h3">{t('onboarding.dns_records')}</Header>}
          columnDefinitions={[
            { id: 'type', header: t('onboarding.dns_type'), cell: (item: DnsRecord) => item.type },
            {
              id: 'host',
              header: t('onboarding.dns_host'),
              cell: (item: DnsRecord) => (
                <CopyToClipboard
                  copyButtonAriaLabel={t('onboarding.copy')}
                  copyErrorText={t('onboarding.copy_error')}
                  copySuccessText={t('onboarding.copy_success')}
                  textToCopy={item.host}
                  variant="inline"
                />
              ),
            },
            {
              id: 'value',
              header: t('onboarding.dns_value'),
              cell: (item: DnsRecord) => (
                <CopyToClipboard
                  copyButtonAriaLabel={t('onboarding.copy')}
                  copyErrorText={t('onboarding.copy_error')}
                  copySuccessText={t('onboarding.copy_success')}
                  textToCopy={item.value}
                  variant="inline"
                />
              ),
            },
            {
              id: 'status',
              header: t('onboarding.dns_status'),
              cell: (item: DnsRecord) => {
                if (item.checked === 'pending') return <StatusIndicator type="pending">{t('onboarding.dns_pending')}</StatusIndicator>;
                if (item.checked === 'ok') return <StatusIndicator type="success">{t('onboarding.dns_verified')}</StatusIndicator>;
                return <StatusIndicator type="error">{t('onboarding.dns_not_found')}</StatusIndicator>;
              },
            },
          ]}
          items={dnsRecords}
          variant="container"
        />
        <SpaceBetween direction="horizontal" size="xs" alignItems="center">
          <Button onClick={handleCheckDns} loading={dnsChecking} disabled={!createdDomain}>
            {t('onboarding.check_dns')}
          </Button>
          {dnsCheckError && <Box color="text-status-error">{dnsCheckError}</Box>}
        </SpaceBetween>
      </SpaceBetween>
    </Container>
  );
}
