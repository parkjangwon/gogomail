'use client';

import {
  ContentLayout,
  Header,
  Container,
  SpaceBetween,
  Button,
  Box,
  ColumnLayout,
  ButtonDropdown,
  Flashbar,
  FlashbarProps,
  Badge,
} from '@cloudscape-design/components';
import { useState } from 'react';
import { useI18n } from '@/app/i18n-provider';
import { useCompany } from '@/contexts/CompanyContext';

interface ReportDef {
  id: string;
  name: string;
  description: string;
  category: string;
  exportEndpoint?: string; // CSV backend endpoint (relative)
}

const REPORT_DEFS: ReportDef[] = [
  {
    id: 'audit_logs',
    name: 'Audit Log Report',
    description: 'Full audit trail of admin actions for compliance review',
    category: 'Compliance',
    exportEndpoint: 'audit-logs/export',
  },
  {
    id: 'users_export',
    name: 'User Directory Export',
    description: 'All users with status, quota, and domain assignments',
    category: 'Users',
    exportEndpoint: 'users/bulk-export',
  },
  {
    id: 'domain_health',
    name: 'Domain Health Summary',
    description: 'Domain status, DNS check results, and quota usage per domain',
    category: 'Domains',
  },
  {
    id: 'quota_summary',
    name: 'Storage Quota Summary',
    description: 'Storage allocation and usage breakdown across domains',
    category: 'Storage',
  },
];

const CATEGORY_COLORS: Record<string, 'blue' | 'green' | 'red' | 'grey'> = {
  Compliance: 'red',
  Users: 'blue',
  Domains: 'green',
  Storage: 'grey',
};

export default function ReportsPage() {
  const { t } = useI18n();
  const { currentCompany } = useCompany();
  const cid = currentCompany?.id;
  const [flash, setFlash] = useState<FlashbarProps.MessageDefinition[]>([]);
  const [exporting, setExporting] = useState<string | null>(null);

  const err = (msg: string) => setFlash([{ type: 'error', content: msg, dismissible: true, onDismiss: () => setFlash([]) }]);
  const ok = (msg: string) => setFlash([{ type: 'success', content: msg, dismissible: true, onDismiss: () => setFlash([]) }]);

  const handleCSVExport = async (report: ReportDef) => {
    if (!report.exportEndpoint || !cid) {
      err('CSV export not available for this report');
      return;
    }
    setExporting(report.id);
    try {
      const res = await fetch(`/admin/v1/companies/${cid}/${report.exportEndpoint}`);
      if (!res.ok) throw new Error(await res.text());
      const blob = await res.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `${report.id}-${cid}.csv`;
      a.click();
      URL.revokeObjectURL(url);
      ok(`${report.name} exported`);
    } catch (e: unknown) {
      err(String(e));
    } finally {
      setExporting(null);
    }
  };

  const handlePrint = () => {
    window.print();
  };

  return (
    <ContentLayout
      header={
        <Header
          variant="h1"
          description="Generate and export compliance and operational reports"
          actions={
            <Button iconName="file" onClick={handlePrint}>Print / Save as PDF</Button>
          }
        >
          {t('nav.reports')}
        </Header>
      }
    >
      <SpaceBetween size="m">
        {flash.length > 0 && <Flashbar items={flash} />}

        <ColumnLayout columns={2}>
          {REPORT_DEFS.map(report => (
            <Container
              key={report.id}
              header={
                <Header
                  variant="h3"
                  actions={
                    <ButtonDropdown
                      loading={exporting === report.id}
                      items={[
                        { id: 'csv', text: 'Export as CSV', disabled: !report.exportEndpoint },
                        { id: 'print', text: 'Print / Save as PDF' },
                      ]}
                      onItemClick={({ detail }) => {
                        if (detail.id === 'csv') handleCSVExport(report);
                        else handlePrint();
                      }}
                    >
                      Export
                    </ButtonDropdown>
                  }
                >
                  <SpaceBetween size="xs" direction="horizontal">
                    <Badge color={CATEGORY_COLORS[report.category] ?? 'grey'}>{report.category}</Badge>
                    <span>{report.name}</span>
                  </SpaceBetween>
                </Header>
              }
            >
              <Box color="text-body-secondary">{report.description}</Box>
              {!report.exportEndpoint && (
                <Box color="text-status-inactive" fontSize="body-s" padding={{ top: 'xs' }}>
                  CSV export: available via API
                </Box>
              )}
            </Container>
          ))}
        </ColumnLayout>

        <Container header={<Header variant="h3">Custom Export</Header>}>
          <SpaceBetween size="m">
            <Box color="text-body-secondary">
              Use the Change History page for filtered audit exports, or the Tenant Health page for a real-time health snapshot.
            </Box>
            <SpaceBetween size="xs" direction="horizontal">
              <Button variant="inline-link" href={`/companies/${cid}/tenancy/change-history`}>
                Change History & Audit Export →
              </Button>
              <Button variant="inline-link" href={`/companies/${cid}/tenancy/health`}>
                Tenant Health Report →
              </Button>
            </SpaceBetween>
          </SpaceBetween>
        </Container>
      </SpaceBetween>
    </ContentLayout>
  );
}
