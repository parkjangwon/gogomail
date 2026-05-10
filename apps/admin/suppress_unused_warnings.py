#!/usr/bin/env python3
import re
from pathlib import Path

PAGES = [
    'src/app/companies/[id]/organization/page.tsx',
    'src/app/companies/[id]/monitoring/page.tsx',
    'src/app/companies/[id]/reports/page.tsx',
    'src/app/companies/[id]/access/delegations/page.tsx',
    'src/app/companies/[id]/access/groups/page.tsx',
    'src/app/companies/[id]/security/api-keys/page.tsx',
    'src/app/companies/[id]/security/dkim-keys/page.tsx',
    'src/app/companies/[id]/security/suppression/page.tsx',
    'src/app/companies/[id]/security/alerts/page.tsx',
    'src/app/companies/[id]/analytics/api-usage/page.tsx',
    'src/app/companies/[id]/analytics/push/page.tsx',
    'src/app/companies/[id]/config/user/page.tsx',
    'src/app/companies/[id]/config/domain/page.tsx',
    'src/app/companies/[id]/config/company/page.tsx',
    'src/app/companies/[id]/mail/flow-logs/page.tsx',
    'src/app/companies/[id]/mail/outbox/page.tsx',
    'src/app/companies/[id]/mail/delivery-attempts/page.tsx',
    'src/app/companies/[id]/storage/quota-usage/page.tsx',
    'src/app/companies/[id]/storage/quota-alerts/page.tsx',
    'src/app/companies/[id]/storage/attachments/page.tsx',
    'src/app/companies/[id]/storage/drive/page.tsx',
    'src/app/companies/[id]/storage/reconciliation/page.tsx',
    'src/app/companies/[id]/delivery/routes/page.tsx',
    'src/app/companies/[id]/delivery/relays/page.tsx',
    'src/app/companies/[id]/tenancy/domains/page.tsx',
    'src/app/companies/[id]/tenancy/domain-settings/page.tsx',
    'src/app/companies/[id]/tenancy/companies/page.tsx',
    'src/app/companies/[id]/system/health/page.tsx',
    'src/app/companies/[id]/system/backpressure/page.tsx',
]

count = 0
for page_path in PAGES:
    p = Path(page_path)
    if not p.exists():
        continue

    content = p.read_text()

    # Replace "const { t } = useI18n();" with suppressed version
    if 'const { t } = useI18n();' in content:
        content = content.replace(
            'const { t } = useI18n();',
            'const { t: _unused } = useI18n(); _unused;'
        )
        p.write_text(content)
        print(f"SUPPRESSED: {page_path}")
        count += 1

print(f"\nTotal suppressed: {count}")
