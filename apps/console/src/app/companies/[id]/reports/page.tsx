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
  category_key: string;
  exportEndpoint?: string;
}

const REPORT_DEFS: ReportDef[] = [
  { id: 'audit_logs', category_key: 'compliance', exportEndpoint: 'audit-logs/export' },
  { id: 'users_export', category_key: 'users', exportEndpoint: 'users/bulk-export' },
  { id: 'domain_health', category_key: 'domains' },
  { id: 'quota_summary', category_key: 'storage' },
];

const CATEGORY_COLORS: Record<string, 'blue' | 'green' | 'red' | 'grey'> = {
  compliance: 'red',
  users: 'blue',
  domains: 'green',
  storage: 'grey',
};

export default function ReportsPage() {
  const { t } = useI18n();
  const { currentCompany } = useCompany();
  const cid = currentCompany?.id;
  const [flash, setFlash] = useState<FlashbarProps.MessageDefinition[]>([]);
  const [exporting, setExporting] = useState<string | null>(null);

  const err = (msg: string) => setFlash([{ type: 'error', content: msg, dismissible: true, onDismiss: () => setFlash([]) }]);
  const ok = (msg: string) => setFlash([{ type: 'success', content: msg, dismissible: true, onDismiss: () => setFlash([]) }]);

  const getLabel = (id: string, field: 'name' | 'desc' | 'cat') =>
    t(`pages.reports_page.${id}_${field}` as Parameters<typeof t>[0]);

  const handleCSVExport = async (report: ReportDef) => {
    if (!report.exportEndpoint || !cid) {
      err(t('pages.reports_page.export_unavailable'));
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
      ok(getLabel(report.id, 'name'));
    } catch (e: unknown) {
      err(String(e));
    } finally {
      setExporting(null);
    }
  };

  const handlePrint = () => window.print();

  return (
    <ContentLayout
      header={
        <Header
          variant="h1"
          description={t('pages.reports_page.page_description')}
          actions={
            <Button iconName="file" onClick={handlePrint}>
              {t('pages.reports_page.print_pdf')}
            </Button>
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
                        { id: 'csv', text: t('pages.reports_page.export_csv'), disabled: !report.exportEndpoint },
                        { id: 'print', text: t('pages.reports_page.print_pdf') },
                      ]}
                      onItemClick={({ detail }) => {
                        if (detail.id === 'csv') handleCSVExport(report);
                        else handlePrint();
                      }}
                    >
                      {t('pages.reports_page.export_btn')}
                    </ButtonDropdown>
                  }
                >
                  <SpaceBetween size="xs" direction="horizontal">
                    <Badge color={CATEGORY_COLORS[report.category_key] ?? 'grey'}>
                      {getLabel(report.id, 'cat')}
                    </Badge>
                    <span>{getLabel(report.id, 'name')}</span>
                  </SpaceBetween>
                </Header>
              }
            >
              <Box color="text-body-secondary">{getLabel(report.id, 'desc')}</Box>
              {!report.exportEndpoint && (
                <Box color="text-status-inactive" fontSize="body-s" padding={{ top: 'xs' }}>
                  {t('pages.reports_page.csv_api')}
                </Box>
              )}
            </Container>
          ))}
        </ColumnLayout>

        <Container header={<Header variant="h3">{t('pages.reports_page.custom_export')}</Header>}>
          <SpaceBetween size="m">
            <Box color="text-body-secondary">
              {t('pages.reports_page.custom_desc')}
            </Box>
            <SpaceBetween size="xs" direction="horizontal">
              <Button variant="inline-link" href={`/companies/${cid}/tenancy/change-history`}>
                {t('pages.reports_page.change_history_link')}
              </Button>
              <Button variant="inline-link" href={`/companies/${cid}/tenancy/health`}>
                {t('pages.reports_page.tenant_health_link')}
              </Button>
            </SpaceBetween>
          </SpaceBetween>
        </Container>
      </SpaceBetween>
    </ContentLayout>
  );
}
