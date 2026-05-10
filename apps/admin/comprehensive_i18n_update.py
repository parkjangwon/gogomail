#!/usr/bin/env python3
import os
import re
from pathlib import Path

# Define pages with their key prefixes
PAGES_CONFIG = {
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
}

def add_i18n_import(content):
    """Add useI18n import if not present."""
    if 'useI18n' in content:
        return content  # Already imported

    # Find the React import line and add after it
    import_pattern = r"import { .*? } from 'react';"
    if re.search(import_pattern, content):
        # Add useI18n import after the CloudscapeDesign import
        import_add = "import { useI18n } from '@/app/i18n-provider';"
        # Find where to insert it - after the last import statement
        last_import = max(
            (m.end() for m in re.finditer(r"import .* from .*;", content)),
            default=None
        )
        if last_import:
            content = content[:last_import] + '\n' + import_add + content[last_import:]

    return content

def add_use_i18n_hook(content):
    """Add const { t } = useI18n(); in the component."""
    # Find the component function
    func_pattern = r'(export default function \w+\(\) \{)'
    if not re.search(func_pattern, content):
        return content

    # Check if t is already declared
    if re.search(r"const { t }", content):
        return content

    # Add after the function declaration and any existing const/useState declarations
    lines = content.split('\n')
    insert_idx = None

    for i, line in enumerate(lines):
        if 'export default function' in line:
            # Find the next line that's not a const declaration
            j = i + 1
            while j < len(lines) and (lines[j].strip().startswith('const {') or lines[j].strip() == '' or lines[j].strip() == '{'):
                j += 1
            insert_idx = j
            break

    if insert_idx:
        lines.insert(insert_idx, '  const { t } = useI18n();')
        content = '\n'.join(lines)

    return content

def update_page(filepath, page_key):
    """Update a single page file."""
    if not os.path.exists(filepath):
        print(f"SKIP: {filepath} (not found)")
        return False

    with open(filepath, 'r') as f:
        content = f.read()

    original_content = content

    # Add import
    content = add_i18n_import(content)

    # Add hook
    content = add_use_i18n_hook(content)

    # If content changed, save it
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
for filepath, page_key in PAGES_CONFIG.items():
    if update_page(filepath, page_key):
        updated += 1

print(f"\nTotal updated: {updated}")
