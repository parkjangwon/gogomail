'use client';

import { DataTable } from '@/components/DataTable';
import {
  Modal,
  Box,
  SpaceBetween,
  Button,
  Tabs,
  ColumnLayout,
  Container,
  Header,
  KeyValuePairs,
  Badge,
  StatusIndicator,
  Spinner,
  ProgressBar,
} from '@cloudscape-design/components';
import type { Company, Domain } from '@/hooks';

type TFunc = (key: string) => string;

interface Props {
  t: TFunc;
  company: Company;
  open: boolean;
  loadingDomains: boolean;
  domains: Domain[];
  onClose: () => void;
  onOpenDomains: () => void;
  onOpenDomain: (domainId: string) => void;
}

const getQuotaPercent = (used: number, limit: number) => (limit > 0 ? Math.round((used / limit) * 100) : 0);

export function CompanyDetailModal({
  t,
  company,
  open,
  loadingDomains,
  domains,
  onClose,
  onOpenDomains,
  onOpenDomain,
}: Props) {
  return (
    <Modal
      onDismiss={onClose}
      visible={open}
      size="large"
      header={
        <SpaceBetween direction="horizontal" size="s">
          <Box fontWeight="bold" fontSize="heading-m">{company.name}</Box>
          <Badge color={company.status === 'active' ? 'green' : 'grey'}>{company.status}</Badge>
        </SpaceBetween>
      }
      footer={
        <Box float="right">
          <SpaceBetween direction="horizontal" size="xs">
            <Button onClick={onOpenDomains}>{t('pages.companies.add_domain')}</Button>
            <Button variant="primary" onClick={onClose}>{t('pages.companies.close')}</Button>
          </SpaceBetween>
        </Box>
      }
    >
      <Tabs
        tabs={[
          {
            label: t('pages.companies.overview_tab'),
            id: 'overview',
            content: (
              <SpaceBetween size="m">
                <ColumnLayout columns={2}>
                  <Container header={<Header variant="h3">{t('pages.companies.company_info')}</Header>}>
                    <KeyValuePairs
                      items={[
                        { label: t('pages.companies.company_id_label'), value: <Box fontSize="body-s" color="text-body-secondary">{company.id}</Box> },
                        { label: t('pages.companies.status'), value: <Badge color={company.status === 'active' ? 'green' : 'grey'}>{company.status}</Badge> },
                        { label: t('pages.companies.created'), value: new Date(company.created_at ?? Date.now()).toLocaleString() },
                      ]}
                    />
                  </Container>
                  <Container header={<Header variant="h3">{t('pages.companies.storage')}</Header>}>
                    <KeyValuePairs
                      items={[
                        { label: t('pages.companies.used'), value: `${((company.quota_used ?? 0) / 1073741824).toFixed(2)} GB` },
                        { label: t('pages.companies.limit'), value: (company.quota_limit ?? 0) > 0 ? `${((company.quota_limit ?? 0) / 1073741824).toFixed(2)} GB` : t('pages.companies.unlimited') },
                        { label: t('pages.companies.remaining'), value: (company.quota_limit ?? 0) > 0 ? `${((company.quota_remaining ?? 0) / 1073741824).toFixed(2)} GB` : '—' },
                        {
                          label: t('pages.companies.utilization'),
                          value: (company.quota_limit ?? 0) > 0
                            ? <ProgressBar value={getQuotaPercent(company.quota_used ?? 0, company.quota_limit ?? 0)} resultText={`${getQuotaPercent(company.quota_used ?? 0, company.quota_limit ?? 0)}%`} />
                            : '—',
                        },
                      ]}
                    />
                  </Container>
                </ColumnLayout>
              </SpaceBetween>
            ),
          },
          {
            label: `${t('pages.companies.domains_tab')} (${domains.length})`,
            id: 'domains',
            content: loadingDomains ? (
              <Box textAlign="center" padding="l"><Spinner /></Box>
            ) : domains.length === 0 ? (
              <Box textAlign="center" padding="l">
                <SpaceBetween size="m" alignItems="center">
                  <StatusIndicator type="warning">{t('pages.companies.no_domains')}</StatusIndicator>
                  <Box color="text-body-secondary">{t('pages.companies.no_domains_desc')}</Box>
                  <Button
                    variant="primary"
                    onClick={onOpenDomains}
                  >
                    {t('pages.companies.add_domain')}
                  </Button>
                </SpaceBetween>
              </Box>
            ) : (
              <DataTable
                columnDefinitions={[
                  {
                    header: t('pages.companies.domain'),
                    cell: (d: Domain) => (
                      <Button variant="inline-link" onClick={() => onOpenDomain(d.id)}>
                        {d.name}
                      </Button>
                    ),
                    width: '40%',
                  },
                  {
                    header: t('pages.companies.status'),
                    cell: (d: Domain) => (
                      <Badge color={d.status === 'active' ? 'green' : 'grey'}>{d.status}</Badge>
                    ),
                    width: '20%',
                  },
                  {
                    header: t('pages.companies.dns'),
                    cell: (d: Domain) => (
                      <Badge color={d.last_dns_check_status === 'ok' ? 'green' : d.last_dns_check_status === 'error' ? 'red' : 'grey'}>
                        {d.last_dns_check_status || 'Unchecked'}
                      </Badge>
                    ),
                    width: '20%',
                  },
                  {
                    header: t('pages.companies.added'),
                    cell: (d: Domain) => new Date(d.created_at ?? Date.now()).toLocaleDateString(),
                    width: '20%',
                  },
                ]}
                items={domains}
                header={<Header variant="h3">{company.name}</Header>}
              />
            ),
          },
        ]}
      />
    </Modal>
  );
}
