#!/usr/bin/env python3
import re
from pathlib import Path

# Detailed pattern mappings for each page
# Key: file path, Value: list of (exact_old_string, new_string) tuples
PAGES_TRANSLATIONS = {
    'src/app/companies/[id]/organization/page.tsx': [
        ('const { t: _unused } = useI18n(); _unused;', 'const { t } = useI18n();'),
        ('>Organization Settings</Header>', '>{t(\'pages.organization.title\')}</Header>'),
        ('description="Configure organization settings"', 'description={t(\'pages.organization.description\')}'),
        ('>Organization Settings', '>{t(\'pages.organization.title\')}'),
    ],
    'src/app/companies/[id]/monitoring/page.tsx': [
        ('const { t: _unused } = useI18n(); _unused;', 'const { t } = useI18n();'),
        ('>Monitoring</Header>', '>{t(\'pages.monitoring.title\')}</Header>'),
        ('description="System monitoring"', 'description={t(\'pages.monitoring.description\')}'),
    ],
    'src/app/companies/[id]/reports/page.tsx': [
        ('const { t: _unused } = useI18n(); _unused;', 'const { t } = useI18n();'),
        ('>Reports</Header>', '>{t(\'pages.reports.title\')}</Header>'),
        ('description="Generate system reports"', 'description={t(\'pages.reports.description\')}'),
        ('+ Create Report', '{t(\'pages.reports.create_report\')}'),
        ('>Reports', '>{t(\'pages.reports.title\')}'),
    ],
    'src/app/companies/[id]/access/delegations/page.tsx': [
        ('const { t: _unused } = useI18n(); _unused;', 'const { t } = useI18n();'),
        ('>Delegations</Header>', '>{t(\'pages.delegations.title\')}</Header>'),
        ('description="Manage delegation permissions"', 'description={t(\'pages.delegations.description\')}'),
        ('+ Create Delegation', '{t(\'pages.delegations.create_delegation\')}'),
        ('header: \'Delegator\',', 'header: t(\'pages.delegations.delegator\'),'),
        ('header: \'Delegate\',', 'header: t(\'pages.delegations.delegate\'),'),
        ('header: \'Status\',', 'header: t(\'pages.delegations.status\'),'),
        ('header: \'Created\',', 'header: t(\'pages.delegations.created\'),'),
        ('>Delegations</Header>', '>{t(\'pages.delegations.title\')}</Header>'),
    ],
    'src/app/companies/[id]/access/groups/page.tsx': [
        ('const { t: _unused } = useI18n(); _unused;', 'const { t } = useI18n();'),
        ('>Groups</Header>', '>{t(\'pages.groups.title\')}</Header>'),
        ('description="Manage user groups"', 'description={t(\'pages.groups.description\')}'),
        ('+ Create Group', '{t(\'pages.groups.create_group\')}'),
        ('header: \'Group Name\',', 'header: t(\'pages.groups.group_name\'),'),
        ('header: \'Members\',', 'header: t(\'pages.groups.members\'),'),
        ('header: \'Created\',', 'header: t(\'pages.groups.created\'),'),
    ],
    'src/app/companies/[id]/security/api-keys/page.tsx': [
        ('const { t: _unused } = useI18n(); _unused;', 'const { t } = useI18n();'),
        ('>API Keys</Header>', '>{t(\'pages.api_keys.title\')}</Header>'),
        ('description="Manage API keys"', 'description={t(\'pages.api_keys.description\')}'),
        ('+ Create Key', '{t(\'pages.api_keys.create_key\')}'),
        ('header: \'Key\',', 'header: t(\'pages.api_keys.key\'),'),
        ('header: \'Secret\',', 'header: t(\'pages.api_keys.secret\'),'),
        ('header: \'Created\',', 'header: t(\'pages.api_keys.created\'),'),
        ('header: \'Last Used\',', 'header: t(\'pages.api_keys.last_used\'),'),
    ],
    'src/app/companies/[id]/security/dkim-keys/page.tsx': [
        ('const { t: _unused } = useI18n(); _unused;', 'const { t } = useI18n();'),
        ('>DKIM Keys</Header>', '>{t(\'pages.dkim_keys.title\')}</Header>'),
        ('description="Manage DKIM keys"', 'description={t(\'pages.dkim_keys.description\')}'),
        ('header: \'Domain\',', 'header: t(\'pages.dkim_keys.domain\'),'),
        ('header: \'Status\',', 'header: t(\'pages.dkim_keys.status\'),'),
    ],
    'src/app/companies/[id]/security/suppression/page.tsx': [
        ('const { t: _unused } = useI18n(); _unused;', 'const { t } = useI18n();'),
        ('>Suppression List</Header>', '>{t(\'pages.suppression.title\')}</Header>'),
        ('description="Manage suppressed emails"', 'description={t(\'pages.suppression.description\')}'),
        ('header: \'Email\',', 'header: t(\'pages.suppression.email\'),'),
        ('header: \'Reason\',', 'header: t(\'pages.suppression.reason\'),'),
    ],
    'src/app/companies/[id]/security/alerts/page.tsx': [
        ('const { t: _unused } = useI18n(); _unused;', 'const { t } = useI18n();'),
        ('>Alert Rules</Header>', '>{t(\'pages.alerts.title\')}</Header>'),
        ('description="Manage alert rules"', 'description={t(\'pages.alerts.description\')}'),
        ('+ Create Rule', '{t(\'pages.alerts.create_alert\')}'),
        ('header: \'Rule Name\',', 'header: t(\'pages.alerts.rule_name\'),'),
        ('header: \'Condition\',', 'header: t(\'pages.alerts.condition\'),'),
    ],
    'src/app/companies/[id]/analytics/api-usage/page.tsx': [
        ('const { t: _unused } = useI18n(); _unused;', 'const { t } = useI18n();'),
        ('>API Usage</Header>', '>{t(\'pages.api_usage.title\')}</Header>'),
        ('description="API usage statistics"', 'description={t(\'pages.api_usage.description\')}'),
        ('header: \'Total Requests\',', 'header: t(\'pages.api_usage.total_requests\'),'),
        ('header: \'Daily Average\',', 'header: t(\'pages.api_usage.daily_average\'),'),
    ],
    'src/app/companies/[id]/analytics/push/page.tsx': [
        ('const { t: _unused } = useI18n(); _unused;', 'const { t } = useI18n();'),
        ('>Push Notifications</Header>', '>{t(\'pages.push.title\')}</Header>'),
        ('description="Push notification statistics"', 'description={t(\'pages.push.description\')}'),
        ('header: \'Total Sent\',', 'header: t(\'pages.push.total_sent\'),'),
    ],
    'src/app/companies/[id]/config/user/page.tsx': [
        ('const { t: _unused } = useI18n(); _unused;', 'const { t } = useI18n();'),
        ('>User Configuration</Header>', '>{t(\'pages.config_user.title\')}</Header>'),
        ('description="User settings"', 'description={t(\'pages.config_user.description\')}'),
    ],
    'src/app/companies/[id]/config/domain/page.tsx': [
        ('const { t: _unused } = useI18n(); _unused;', 'const { t } = useI18n();'),
        ('>Domain Configuration</Header>', '>{t(\'pages.config_domain.title\')}</Header>'),
        ('description="Domain settings"', 'description={t(\'pages.config_domain.description\')}'),
    ],
    'src/app/companies/[id]/config/company/page.tsx': [
        ('const { t: _unused } = useI18n(); _unused;', 'const { t } = useI18n();'),
        ('>Company Configuration</Header>', '>{t(\'pages.config_company.title\')}</Header>'),
        ('description="Company settings"', 'description={t(\'pages.config_company.description\')}'),
    ],
    'src/app/companies/[id]/mail/flow-logs/page.tsx': [
        ('const { t: _unused } = useI18n(); _unused;', 'const { t } = useI18n();'),
        ('>Mail Flow Logs</Header>', '>{t(\'pages.flow_logs.title\')}</Header>'),
        ('description="Mail flow logs"', 'description={t(\'pages.flow_logs.description\')}'),
        ('header: \'Sender\',', 'header: t(\'pages.flow_logs.sender\'),'),
        ('header: \'Recipient\',', 'header: t(\'pages.flow_logs.recipient\'),'),
        ('header: \'Subject\',', 'header: t(\'pages.flow_logs.subject\'),'),
        ('header: \'Status\',', 'header: t(\'pages.flow_logs.status\'),'),
    ],
    'src/app/companies/[id]/mail/outbox/page.tsx': [
        ('const { t: _unused } = useI18n(); _unused;', 'const { t } = useI18n();'),
        ('>Outbox Events</Header>', '>{t(\'pages.outbox.title\')}</Header>'),
        ('description="Outbox events"', 'description={t(\'pages.outbox.description\')}'),
    ],
    'src/app/companies/[id]/mail/delivery-attempts/page.tsx': [
        ('const { t: _unused } = useI18n(); _unused;', 'const { t } = useI18n();'),
        ('>Delivery Attempts</Header>', '>{t(\'pages.delivery_attempts.title\')}</Header>'),
        ('description="Mail delivery attempts"', 'description={t(\'pages.delivery_attempts.description\')}'),
        ('header: \'Recipient\',', 'header: t(\'pages.delivery_attempts.recipient\'),'),
        ('header: \'Status\',', 'header: t(\'pages.delivery_attempts.status\'),'),
    ],
    'src/app/companies/[id]/storage/quota-usage/page.tsx': [
        ('const { t: _unused } = useI18n(); _unused;', 'const { t } = useI18n();'),
        ('>Quota Usage</Header>', '>{t(\'pages.quota_usage.title\')}</Header>'),
        ('description="Quota usage"', 'description={t(\'pages.quota_usage.description\')}'),
    ],
    'src/app/companies/[id]/storage/quota-alerts/page.tsx': [
        ('const { t: _unused } = useI18n(); _unused;', 'const { t } = useI18n();'),
        ('>Quota Alerts</Header>', '>{t(\'pages.quota_alerts.title\')}</Header>'),
        ('description="Quota alerts"', 'description={t(\'pages.quota_alerts.description\')}'),
    ],
    'src/app/companies/[id]/storage/attachments/page.tsx': [
        ('const { t: _unused } = useI18n(); _unused;', 'const { t } = useI18n();'),
        ('>Attachments</Header>', '>{t(\'pages.attachments.title\')}</Header>'),
        ('description="Manage attachments"', 'description={t(\'pages.attachments.description\')}'),
    ],
    'src/app/companies/[id]/storage/drive/page.tsx': [
        ('const { t: _unused } = useI18n(); _unused;', 'const { t } = useI18n();'),
        ('>Drive</Header>', '>{t(\'pages.drive.title\')}</Header>'),
        ('description="Manage drive"', 'description={t(\'pages.drive.description\')}'),
    ],
    'src/app/companies/[id]/storage/reconciliation/page.tsx': [
        ('const { t: _unused } = useI18n(); _unused;', 'const { t } = useI18n();'),
        ('>Quota Reconciliation</Header>', '>{t(\'pages.reconciliation.title\')}</Header>'),
        ('description="Quota reconciliation"', 'description={t(\'pages.reconciliation.description\')}'),
    ],
    'src/app/companies/[id]/delivery/routes/page.tsx': [
        ('const { t: _unused } = useI18n(); _unused;', 'const { t } = useI18n();'),
        ('>Delivery Routes</Header>', '>{t(\'pages.routes.title\')}</Header>'),
        ('description="Mail delivery routes"', 'description={t(\'pages.routes.description\')}'),
        ('+ Create Route', '{t(\'pages.routes.create_route\')}'),
    ],
    'src/app/companies/[id]/delivery/relays/page.tsx': [
        ('const { t: _unused } = useI18n(); _unused;', 'const { t } = useI18n();'),
        ('>Trusted Relays</Header>', '>{t(\'pages.relays.title\')}</Header>'),
        ('description="Trusted mail relays"', 'description={t(\'pages.relays.description\')}'),
        ('+ Add Relay', '{t(\'pages.relays.create_relay\')}'),
    ],
    'src/app/companies/[id]/tenancy/domains/page.tsx': [
        ('const { t: _unused } = useI18n(); _unused;', 'const { t } = useI18n();'),
        ('>Domains</Header>', '>{t(\'pages.domains.title\')}</Header>'),
        ('description="Manage domains"', 'description={t(\'pages.domains.description\')}'),
        ('+ Add Domain', '{t(\'pages.domains.create_domain\')}'),
        ('header: \'Domain Name\',', 'header: t(\'pages.domains.domain_name\'),'),
        ('header: \'Status\',', 'header: t(\'pages.domains.status\'),'),
        ('header: \'Created\',', 'header: t(\'pages.domains.created\'),'),
    ],
    'src/app/companies/[id]/tenancy/domain-settings/page.tsx': [
        ('const { t: _unused } = useI18n(); _unused;', 'const { t } = useI18n();'),
        ('>Domain Settings</Header>', '>{t(\'pages.domain_settings.title\')}</Header>'),
        ('description="Domain settings"', 'description={t(\'pages.domain_settings.description\')}'),
    ],
    'src/app/companies/[id]/tenancy/companies/page.tsx': [
        ('const { t: _unused } = useI18n(); _unused;', 'const { t } = useI18n();'),
        ('>Companies</Header>', '>{t(\'pages.companies.title\')}</Header>'),
        ('description="Manage tenant companies"', 'description={t(\'pages.companies.description\')}'),
        ('header: \'Company Name\',', 'header: t(\'pages.companies.company_name\'),'),
        ('header: \'Status\',', 'header: t(\'pages.companies.status\'),'),
        ('header: \'Created\',', 'header: t(\'pages.companies.created\'),'),
    ],
    'src/app/companies/[id]/system/health/page.tsx': [
        ('const { t: _unused } = useI18n(); _unused;', 'const { t } = useI18n();'),
        ('>System Health</Header>', '>{t(\'pages.system_health.title\')}</Header>'),
        ('description="System health"', 'description={t(\'pages.system_health.description\')}'),
    ],
    'src/app/companies/[id]/system/backpressure/page.tsx': [
        ('const { t: _unused } = useI18n(); _unused;', 'const { t } = useI18n();'),
        ('>Backpressure</Header>', '>{t(\'pages.system_backpressure.title\')}</Header>'),
        ('description="System load"', 'description={t(\'pages.system_backpressure.description\')}'),
    ],
}

def translate_file(filepath, translations):
    """Translate a single file with exact string replacements."""
    p = Path(filepath)
    if not p.exists():
        return False

    content = p.read_text()
    original = content

    for old_str, new_str in translations:
        content = content.replace(old_str, new_str)

    if content != original:
        p.write_text(content)
        return True
    return False

# Apply all translations
count = 0
for filepath, translations in PAGES_TRANSLATIONS.items():
    if translate_file(filepath, translations):
        print(f"TRANSLATED: {filepath}")
        count += 1
    else:
        print(f"NO CHANGES: {filepath}")

print(f"\n완료: {count}개 페이지 번역됨")
