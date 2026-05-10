#!/usr/bin/env python3
import os
import re
from pathlib import Path

# Map of files to their page translation key prefix
PAGES = {
    'src/app/companies/[id]/organization/page.tsx': ('pages.organization', 'Organization'),
    'src/app/companies/[id]/monitoring/page.tsx': ('pages.monitoring', 'Monitoring'),
    'src/app/companies/[id]/reports/page.tsx': ('pages.reports', 'Reports'),
    'src/app/companies/[id]/access/delegations/page.tsx': ('pages.delegations', 'Delegations'),
    'src/app/companies/[id]/access/groups/page.tsx': ('pages.groups', 'Groups'),
    'src/app/companies/[id]/security/api-keys/page.tsx': ('pages.api_keys', 'API Keys'),
    'src/app/companies/[id]/security/dkim-keys/page.tsx': ('pages.dkim_keys', 'DKIM Keys'),
    'src/app/companies/[id]/security/suppression/page.tsx': ('pages.suppression', 'Suppression List'),
    'src/app/companies/[id]/security/alerts/page.tsx': ('pages.alerts', 'Alert Rules'),
    'src/app/companies/[id]/analytics/api-usage/page.tsx': ('pages.api_usage', 'API Usage'),
    'src/app/companies/[id]/analytics/push/page.tsx': ('pages.push', 'Push Notifications'),
    'src/app/companies/[id]/config/user/page.tsx': ('pages.config_user', 'User Configuration'),
    'src/app/companies/[id]/config/domain/page.tsx': ('pages.config_domain', 'Domain Configuration'),
    'src/app/companies/[id]/config/company/page.tsx': ('pages.config_company', 'Company Configuration'),
    'src/app/companies/[id]/mail/flow-logs/page.tsx': ('pages.flow_logs', 'Mail Flow Logs'),
    'src/app/companies/[id]/mail/outbox/page.tsx': ('pages.outbox', 'Outbox Events'),
    'src/app/companies/[id]/mail/delivery-attempts/page.tsx': ('pages.delivery_attempts', 'Delivery Attempts'),
    'src/app/companies/[id]/storage/quota-usage/page.tsx': ('pages.quota_usage', 'Quota Usage'),
    'src/app/companies/[id]/storage/quota-alerts/page.tsx': ('pages.quota_alerts', 'Quota Alerts'),
    'src/app/companies/[id]/storage/attachments/page.tsx': ('pages.attachments', 'Attachments'),
    'src/app/companies/[id]/storage/drive/page.tsx': ('pages.drive', 'Drive'),
    'src/app/companies/[id]/storage/reconciliation/page.tsx': ('pages.reconciliation', 'Quota Reconciliation'),
    'src/app/companies/[id]/delivery/routes/page.tsx': ('pages.routes', 'Delivery Routes'),
    'src/app/companies/[id]/delivery/relays/page.tsx': ('pages.relays', 'Trusted Relays'),
    'src/app/companies/[id]/tenancy/domains/page.tsx': ('pages.domains', 'Domains'),
    'src/app/companies/[id]/tenancy/domain-settings/page.tsx': ('pages.domain_settings', 'Domain Settings'),
    'src/app/companies/[id]/tenancy/companies/page.tsx': ('pages.companies', 'Companies'),
    'src/app/companies/[id]/system/health/page.tsx': ('pages.system_health', 'System Health'),
    'src/app/companies/[id]/system/backpressure/page.tsx': ('pages.system_backpressure', 'Backpressure'),
}

def update_page(filepath, page_key, page_title):
    """Update a single page file with i18n translations."""
    if not os.path.exists(filepath):
        print(f"SKIP: {filepath} (not found)")
        return False

    with open(filepath, 'r') as f:
        content = f.read()

    original_content = content

    # Fix useI18n hook declaration
    content = re.sub(
        r"const { t: _unused } = useI18n\(\); _unused;",
        "const { t } = useI18n();",
        content
    )

    # Replace page title in loading state
    title_replacement = "{t('" + page_key + ".title')}"
    content = re.sub(
        '<Header variant="h1">' + page_title + '</Header>',
        '<Header variant="h1">' + '{' + title_replacement + '}' + '</Header>',
        content
    )

    # Replace description attribute
    desc_replacement = "{t('" + page_key + ".description')}"
    if 'description="' in content:
        content = re.sub(
            r'description="[^"]*"',
            'description=' + '{' + desc_replacement + '}',
            content
        )

    # Replace page title in main header
    content = re.sub(
        '>\s*' + page_title + '\s*</Header>',
        '>' + '{' + title_replacement + '}' + '</Header>',
        content
    )

    if content != original_content:
        with open(filepath, 'w') as f:
            f.write(content)
        print(f"UPDATED: {filepath}")
        return True
    else:
        print(f"NO CHANGES: {filepath}")
        return False

# Update all pages
updated = 0
skipped = 0

for filepath, (page_key, page_title) in PAGES.items():
    if update_page(filepath, page_key, page_title):
        updated += 1
    else:
        skipped += 1

print(f"\nSummary: {updated} updated, {skipped} skipped")
