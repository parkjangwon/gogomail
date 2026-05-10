#!/usr/bin/env python3
import re
from pathlib import Path

# Comprehensive mapping of all pages with their common patterns
PAGE_PATTERNS = {
    'organization': {
        'file': 'src/app/companies/[id]/organization/page.tsx',
        'title': 'Organization Settings',
        'patterns': [
            ('Organization Settings', "t('pages.organization.title')"),
            ('Configure organization settings', "t('pages.organization.description')"),
            ('Edit Settings', "'Edit Settings'"),
            ('Cancel', "'Cancel'"),
        ]
    },
    'monitoring': {
        'file': 'src/app/companies/[id]/monitoring/page.tsx',
        'title': 'Monitoring',
        'patterns': [
            ('Monitoring', "t('pages.monitoring.title')"),
            ('System monitoring', "t('pages.monitoring.description')"),
        ]
    },
    'reports': {
        'file': 'src/app/companies/[id]/reports/page.tsx',
        'title': 'Reports',
        'patterns': [
            ('Reports', "t('pages.reports.title')"),
            ('Generate system reports', "t('pages.reports.description')"),
            ('Create Report', "t('pages.reports.create_report')"),
        ]
    },
    'delegations': {
        'file': 'src/app/companies/[id]/access/delegations/page.tsx',
        'patterns': [
            ('Delegations', "t('pages.delegations.title')"),
            ('Manage delegation permissions', "t('pages.delegations.description')"),
            ('+ Create Delegation', "t('pages.delegations.create_delegation')"),
            ('Delegator', "t('pages.delegations.delegator')"),
            ('Delegate', "t('pages.delegations.delegate')"),
            ('Status', "t('pages.delegations.status')"),
            ('Created', "t('pages.delegations.created')"),
        ]
    },
    'groups': {
        'file': 'src/app/companies/[id]/access/groups/page.tsx',
        'patterns': [
            ('Groups', "t('pages.groups.title')"),
            ('Manage user groups', "t('pages.groups.description')"),
            ('+ Create Group', "t('pages.groups.create_group')"),
            ('Group Name', "t('pages.groups.group_name')"),
            ('Members', "t('pages.groups.members')"),
            ('Created', "t('pages.groups.created')"),
        ]
    },
    'api-keys': {
        'file': 'src/app/companies/[id]/security/api-keys/page.tsx',
        'patterns': [
            ('API Keys', "t('pages.api_keys.title')"),
            ('Manage API keys', "t('pages.api_keys.description')"),
            ('+ Create Key', "t('pages.api_keys.create_key')"),
            ('Key', "t('pages.api_keys.key')"),
            ('Secret', "t('pages.api_keys.secret')"),
            ('Created', "t('pages.api_keys.created')"),
            ('Last Used', "t('pages.api_keys.last_used')"),
        ]
    },
    'dkim-keys': {
        'file': 'src/app/companies/[id]/security/dkim-keys/page.tsx',
        'patterns': [
            ('DKIM Keys', "t('pages.dkim_keys.title')"),
            ('Manage DKIM keys', "t('pages.dkim_keys.description')"),
            ('Domain', "t('pages.dkim_keys.domain')"),
            ('Status', "t('pages.dkim_keys.status')"),
            ('Verified', "t('pages.dkim_keys.verified')"),
        ]
    },
    'suppression': {
        'file': 'src/app/companies/[id]/security/suppression/page.tsx',
        'patterns': [
            ('Suppression List', "t('pages.suppression.title')"),
            ('Manage suppressed emails', "t('pages.suppression.description')"),
            ('Email', "t('pages.suppression.email')"),
            ('Reason', "t('pages.suppression.reason')"),
        ]
    },
    'alerts': {
        'file': 'src/app/companies/[id]/security/alerts/page.tsx',
        'patterns': [
            ('Alert Rules', "t('pages.alerts.title')"),
            ('Manage alert rules', "t('pages.alerts.description')"),
            ('+ Create Rule', "t('pages.alerts.create_alert')"),
            ('Rule Name', "t('pages.alerts.rule_name')"),
            ('Condition', "t('pages.alerts.condition')"),
        ]
    },
    'api-usage': {
        'file': 'src/app/companies/[id]/analytics/api-usage/page.tsx',
        'patterns': [
            ('API Usage', "t('pages.api_usage.title')"),
            ('API usage statistics', "t('pages.api_usage.description')"),
            ('Total Requests', "t('pages.api_usage.total_requests')"),
            ('Daily Average', "t('pages.api_usage.daily_average')"),
        ]
    },
    'push': {
        'file': 'src/app/companies/[id]/analytics/push/page.tsx',
        'patterns': [
            ('Push Notifications', "t('pages.push.title')"),
            ('Push notification statistics', "t('pages.push.description')"),
            ('Total Sent', "t('pages.push.total_sent')"),
        ]
    },
    'config-user': {
        'file': 'src/app/companies/[id]/config/user/page.tsx',
        'patterns': [
            ('User Configuration', "t('pages.config_user.title')"),
            ('User settings', "t('pages.config_user.description')"),
        ]
    },
    'config-domain': {
        'file': 'src/app/companies/[id]/config/domain/page.tsx',
        'patterns': [
            ('Domain Configuration', "t('pages.config_domain.title')"),
            ('Domain settings', "t('pages.config_domain.description')"),
        ]
    },
    'config-company': {
        'file': 'src/app/companies/[id]/config/company/page.tsx',
        'patterns': [
            ('Company Configuration', "t('pages.config_company.title')"),
            ('Company settings', "t('pages.config_company.description')"),
        ]
    },
    'flow-logs': {
        'file': 'src/app/companies/[id]/mail/flow-logs/page.tsx',
        'patterns': [
            ('Mail Flow Logs', "t('pages.flow_logs.title')"),
            ('Mail flow logs', "t('pages.flow_logs.description')"),
            ('Sender', "t('pages.flow_logs.sender')"),
            ('Recipient', "t('pages.flow_logs.recipient')"),
            ('Subject', "t('pages.flow_logs.subject')"),
            ('Status', "t('pages.flow_logs.status')"),
        ]
    },
    'outbox': {
        'file': 'src/app/companies/[id]/mail/outbox/page.tsx',
        'patterns': [
            ('Outbox Events', "t('pages.outbox.title')"),
            ('Outbox events', "t('pages.outbox.description')"),
        ]
    },
    'delivery-attempts': {
        'file': 'src/app/companies/[id]/mail/delivery-attempts/page.tsx',
        'patterns': [
            ('Delivery Attempts', "t('pages.delivery_attempts.title')"),
            ('Mail delivery attempts', "t('pages.delivery_attempts.description')"),
            ('Recipient', "t('pages.delivery_attempts.recipient')"),
            ('Status', "t('pages.delivery_attempts.status')"),
        ]
    },
    'quota-usage': {
        'file': 'src/app/companies/[id]/storage/quota-usage/page.tsx',
        'patterns': [
            ('Quota Usage', "t('pages.quota_usage.title')"),
            ('Quota usage', "t('pages.quota_usage.description')"),
        ]
    },
    'quota-alerts': {
        'file': 'src/app/companies/[id]/storage/quota-alerts/page.tsx',
        'patterns': [
            ('Quota Alerts', "t('pages.quota_alerts.title')"),
            ('Quota alerts', "t('pages.quota_alerts.description')"),
        ]
    },
    'attachments': {
        'file': 'src/app/companies/[id]/storage/attachments/page.tsx',
        'patterns': [
            ('Attachments', "t('pages.attachments.title')"),
            ('Manage attachments', "t('pages.attachments.description')"),
        ]
    },
    'drive': {
        'file': 'src/app/companies/[id]/storage/drive/page.tsx',
        'patterns': [
            ('Drive', "t('pages.drive.title')"),
            ('Manage drive', "t('pages.drive.description')"),
        ]
    },
    'reconciliation': {
        'file': 'src/app/companies/[id]/storage/reconciliation/page.tsx',
        'patterns': [
            ('Quota Reconciliation', "t('pages.reconciliation.title')"),
            ('Quota reconciliation', "t('pages.reconciliation.description')"),
        ]
    },
    'routes': {
        'file': 'src/app/companies/[id]/delivery/routes/page.tsx',
        'patterns': [
            ('Delivery Routes', "t('pages.routes.title')"),
            ('Mail delivery routes', "t('pages.routes.description')"),
            ('+ Create Route', "t('pages.routes.create_route')"),
        ]
    },
    'relays': {
        'file': 'src/app/companies/[id]/delivery/relays/page.tsx',
        'patterns': [
            ('Trusted Relays', "t('pages.relays.title')"),
            ('Trusted mail relays', "t('pages.relays.description')"),
            ('+ Add Relay', "t('pages.relays.create_relay')"),
        ]
    },
    'domains': {
        'file': 'src/app/companies/[id]/tenancy/domains/page.tsx',
        'patterns': [
            ('Domains', "t('pages.domains.title')"),
            ('Manage domains', "t('pages.domains.description')"),
            ('+ Add Domain', "t('pages.domains.create_domain')"),
            ('Domain Name', "t('pages.domains.domain_name')"),
            ('Status', "t('pages.domains.status')"),
            ('Created', "t('pages.domains.created')"),
        ]
    },
    'domain-settings': {
        'file': 'src/app/companies/[id]/tenancy/domain-settings/page.tsx',
        'patterns': [
            ('Domain Settings', "t('pages.domain_settings.title')"),
            ('Domain settings', "t('pages.domain_settings.description')"),
        ]
    },
    'companies': {
        'file': 'src/app/companies/[id]/tenancy/companies/page.tsx',
        'patterns': [
            ('Companies', "t('pages.companies.title')"),
            ('Manage tenant companies', "t('pages.companies.description')"),
        ]
    },
    'health': {
        'file': 'src/app/companies/[id]/system/health/page.tsx',
        'patterns': [
            ('System Health', "t('pages.system_health.title')"),
            ('System health', "t('pages.system_health.description')"),
        ]
    },
    'backpressure': {
        'file': 'src/app/companies/[id]/system/backpressure/page.tsx',
        'patterns': [
            ('Backpressure', "t('pages.system_backpressure.title')"),
            ('System load', "t('pages.system_backpressure.description')"),
        ]
    },
}

def translate_page(page_config):
    """Translate a single page file."""
    filepath = Path(page_config['file'])
    if not filepath.exists():
        print(f"SKIP: {page_config['file']} (not found)")
        return False

    content = filepath.read_text()
    original = content

    # Replace patterns
    for english_str, trans_key in page_config['patterns']:
        # Replace in various JSX contexts
        # 1. In headers/titles: >text</Header>
        content = re.sub(
            f">{re.escape(english_str)}</",
            f">{{{{{trans_key}}}}}</",
            content
        )
        # 2. In attributes
        content = content.replace(f'"{english_str}"', f'{{{{{trans_key}}}}}')
        # 3. Direct strings
        content = content.replace(f"'{english_str}'", trans_key)

    if content != original:
        filepath.write_text(content)
        print(f"TRANSLATED: {page_config['file']}")
        return True
    else:
        print(f"NO CHANGES: {page_config['file']}")
        return False

# Translate all pages
count = 0
for page_name, page_config in PAGE_PATTERNS.items():
    if translate_page(page_config):
        count += 1

print(f"\nTotal translated: {count}")
