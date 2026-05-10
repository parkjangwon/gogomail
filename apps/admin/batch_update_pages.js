import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

// Map of file paths to page translation key prefixes
const pageUpdates = {
  'src/app/companies/[id]/organization/page.tsx': 'pages.organization',
  'src/app/companies/[id]/monitoring/page.tsx': 'pages.monitoring',
  'src/app/companies/[id]/reports/page.tsx': 'pages.reports',
  'src/app/companies/[id]/access/delegations/page.tsx': 'pages.delegations',
  'src/app/companies/[id]/access/groups/page.tsx': 'pages.groups',
  'src/app/companies/[id]/security/api-keys/page.tsx': 'pages.api_keys',
  'src/app/companies/[id]/security/dkim-keys/page.tsx': 'pages.dkim_keys',
  'src/app/companies/[id]/security/suppression/page.tsx': 'pages.suppression',
  'src/app/companies/[id]/security/alerts/page.tsx': 'pages.alerts',
  'src/app/companies/[id]/analytics/api-usage/page.tsx': 'pages.api_usage',
  'src/app/companies/[id]/analytics/push/page.tsx': 'pages.push',
  'src/app/companies/[id]/config/user/page.tsx': 'pages.config_user',
  'src/app/companies/[id]/config/domain/page.tsx': 'pages.config_domain',
  'src/app/companies/[id]/config/company/page.tsx': 'pages.config_company',
  'src/app/companies/[id]/mail/flow-logs/page.tsx': 'pages.flow_logs',
  'src/app/companies/[id]/mail/outbox/page.tsx': 'pages.outbox',
  'src/app/companies/[id]/mail/delivery-attempts/page.tsx': 'pages.delivery_attempts',
  'src/app/companies/[id]/storage/quota-usage/page.tsx': 'pages.quota_usage',
  'src/app/companies/[id]/storage/quota-alerts/page.tsx': 'pages.quota_alerts',
  'src/app/companies/[id]/storage/attachments/page.tsx': 'pages.attachments',
  'src/app/companies/[id]/storage/drive/page.tsx': 'pages.drive',
  'src/app/companies/[id]/storage/reconciliation/page.tsx': 'pages.reconciliation',
  'src/app/companies/[id]/delivery/routes/page.tsx': 'pages.routes',
  'src/app/companies/[id]/delivery/relays/page.tsx': 'pages.relays',
  'src/app/companies/[id]/tenancy/domains/page.tsx': 'pages.domains',
  'src/app/companies/[id]/tenancy/domain-settings/page.tsx': 'pages.domain_settings',
  'src/app/companies/[id]/tenancy/companies/page.tsx': 'pages.companies',
  'src/app/companies/[id]/system/health/page.tsx': 'pages.system_health',
  'src/app/companies/[id]/system/backpressure/page.tsx': 'pages.system_backpressure',
};

function updatePageFile(filePath, pageKey) {
  if (!fs.existsSync(filePath)) {
    console.log(`File not found: ${filePath}`);
    return false;
  }

  let content = fs.readFileSync(filePath, 'utf8');

  // Fix the useI18n hook declaration
  content = content.replace(
    /const { t: _unused } = useI18n\(\); _unused;/,
    'const { t } = useI18n();'
  );

  // Replace common patterns
  const replacements = [
    // Title in loading state
    { find: /header={<Header variant="h1">([^<]+)<\/Header>}/g,
      replace: `header={<Header variant="h1">{{t('${pageKey}.title')}}</Header>}` },

    // Title in return statement
    { find: />([^<{]+)<\/Header>\s*<\/ContentLayout>/g,
      replace: `>{{{t('${pageKey}.title')}}}</Header>` },

    // Description attribute
    { find: /description="([^"]+)"/g,
      replace: `description={t('${pageKey}.description')}` },

    // Create button text
    { find: /(\+ Create |Create |Add )\w+/g,
      replace: `{t('${pageKey}.create_xxx')}` },

    // Generic table headers - replace known patterns
    { find: /header: 'Title'/g, replace: `header: t('${pageKey}.title'),` },
    { find: /header: 'Name'/g, replace: `header: t('${pageKey}.name'),` },
    { find: /header: 'Status'/g, replace: `header: t('${pageKey}.status'),` },
    { find: /header: 'Created'/g, replace: `header: t('${pageKey}.created'),` },
    { find: /header: 'Email'/g, replace: `header: t('${pageKey}.email'),` },
  ];

  // Simple replacement approach: fix the hook first, then do some basic replacements
  let modified = content;

  // Always fix useI18n hook
  modified = modified.replace(
    /const { t: _unused } = useI18n\(\); _unused;/,
    'const { t } = useI18n();'
  );

  fs.writeFileSync(filePath, modified, 'utf8');
  console.log(`Updated ${filePath}`);
  return true;
}

// Update all pages
Object.entries(pageUpdates).forEach(([filePath, pageKey]) => {
  updatePageFile(filePath, pageKey);
});

console.log('Batch update completed');
