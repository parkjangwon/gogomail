'use client';

import { SideNavigation, SideNavigationProps } from '@cloudscape-design/components';
import { useRouter, usePathname } from 'next/navigation';
import { useI18n } from '@/app/i18n-provider';
import { useCompany } from '@/contexts/CompanyContext';
import { useMemo } from 'react';

export function Sidebar() {
  const router = useRouter();
  const pathname = usePathname();
  const { t } = useI18n();
  const { currentCompany } = useCompany();
  const cid = currentCompany?.id ?? 'default';

  const p = (path: string) => `/companies/${cid}${path}`;

  const activeSectionKey = useMemo(() => {
    if (!pathname) return 'none';
    if (pathname.includes('/mail/') || pathname.includes('/delivery/') || pathname.includes('/system/')) return 'ops';
    if (pathname.includes('/access/') || pathname.includes('/roles')) return 'access';
    if (pathname.includes('/audit-logs') || pathname.includes('/admin-activity') || pathname.includes('/security/') || pathname.includes('/compliance')) return 'gov';
    if (pathname.includes('/storage/') || pathname.includes('/analytics/') || pathname.includes('/reports')) return 'analytics';
    if (pathname.includes('/config/') || pathname.includes('/tenancy/domain-settings') || pathname.includes('/organization/')) return 'config';
    if (pathname.includes('/tenancy/') || pathname.includes('/users') || pathname.includes('/admin-users')) return 'resources';
    return 'none';
  }, [pathname]);

  const e = (key: string) => activeSectionKey === key ? {} : { defaultExpanded: false };

  // Sections without defaultExpanded auto-expand when they contain activeHref.
  // Sections with defaultExpanded: false stay collapsed unless the user opens them.
  // key={activeSectionKey} remounts the nav when moving between sections so
  // inactive sections reset to collapsed.
  const navigationItems: SideNavigationProps.Item[] = useMemo(() => [
    { type: 'link', text: t('nav.dashboard'), href: p('/dashboard') },
    { type: 'divider' },
    {
      type: 'section',
      text: t('nav.section_resources'),
      ...e('resources'),
      items: [
        { type: 'link', text: t('nav.companies'), href: p('/tenancy/companies') },
        { type: 'link', text: t('nav.domains'), href: p('/tenancy/domains') },
        { type: 'link', text: t('nav.tenant_health'), href: p('/tenancy/health') },
        { type: 'link', text: t('nav.change_history'), href: p('/tenancy/change-history') },
        { type: 'link', text: t('nav.users'), href: p('/users') },
        { type: 'link', text: t('nav.admin_users'), href: p('/admin-users') },
        { type: 'link', text: t('nav.onboarding'), href: p('/tenancy/onboarding') },
      ],
    },
    {
      type: 'section',
      text: t('nav.configuration'),
      ...e('config'),
      items: [
        { type: 'link', text: t('nav.company_config'), href: p('/config/company') },
        { type: 'link', text: t('nav.domain_settings'), href: p('/tenancy/domain-settings') },
        { type: 'link', text: t('nav.sso_config'), href: p('/organization/sso') },
        { type: 'link', text: t('nav.webhooks'), href: p('/organization/webhooks') },
        { type: 'link', text: t('nav.notif_templates'), href: p('/organization/notification-templates') },
        { type: 'link', text: 'Global Signature', href: p('/organization/signature') },
        { type: 'link', text: 'SCIM Provisioning', href: p('/organization/scim-status') },
        { type: 'link', text: t('nav.user_config'), href: p('/config/user') },
      ],
    },
    {
      type: 'section',
      text: t('nav.section_operations'),
      ...e('ops'),
      items: [
        { type: 'link', text: 'Message Trace', href: p('/mail/message-trace') },
        { type: 'link', text: t('nav.mail_flow_logs'), href: p('/mail/flow-logs') },
        { type: 'link', text: t('nav.outbox_events'), href: p('/mail/outbox') },
        { type: 'link', text: t('nav.delivery_attempts'), href: p('/mail/delivery-attempts') },
        { type: 'link', text: t('nav.routing_rules'), href: p('/mail/routing-rules') },
        { type: 'link', text: t('nav.delivery_routes'), href: p('/delivery/routes') },
        { type: 'link', text: t('nav.trusted_relays'), href: p('/delivery/relays') },
        { type: 'link', text: t('nav.queue_stats'), href: p('/system/queue') },
        { type: 'link', text: t('nav.backpressure'), href: p('/system/backpressure') },
        { type: 'link', text: t('nav.api_health'), href: p('/system/health') },
      ],
    },
    {
      type: 'section',
      text: t('nav.section_access_control'),
      ...e('access'),
      items: [
        { type: 'link', text: t('nav.directory'), href: p('/access/directory') },
        { type: 'link', text: t('nav.aliases'), href: p('/access/aliases') },
        { type: 'link', text: t('nav.delegations'), href: p('/access/delegations') },
        { type: 'link', text: t('nav.group_memberships'), href: p('/access/groups') },
        { type: 'link', text: t('nav.roles'), href: p('/roles') },
      ],
    },
    {
      type: 'section',
      text: t('nav.section_governance'),
      ...e('gov'),
      items: [
        { type: 'link', text: t('nav.audit_logs'), href: p('/audit-logs') },
        { type: 'link', text: 'Admin Activity', href: p('/admin-activity') },
        { type: 'link', text: t('nav.alert_rules'), href: p('/security/alerts') },
        { type: 'link', text: t('nav.suppression_list'), href: p('/security/suppression') },
        { type: 'link', text: t('nav.dkim_keys'), href: p('/security/dkim-keys') },
        { type: 'link', text: t('nav.api_keys'), href: p('/security/api-keys') },
        { type: 'link', text: t('nav.mfa_management'), href: p('/security/mfa') },
        { type: 'link', text: t('nav.ip_access'), href: p('/security/ip-access') },
        { type: 'link', text: t('nav.auth_policy'), href: p('/security/auth-policy') },
        { type: 'link', text: t('nav.retention_policy'), href: p('/security/retention') },
        { type: 'link', text: t('nav.session_mgmt'), href: p('/security/sessions') },
        { type: 'link', text: t('nav.rate_limits'), href: p('/security/rate-limits') },
        { type: 'link', text: t('nav.dmarc_spf'), href: p('/security/dmarc') },
        { type: 'link', text: t('nav.spam_filter'), href: p('/security/spam-filter') },
        { type: 'link', text: t('nav.smtp_policy'), href: p('/security/smtp-policy') },
        { type: 'link', text: 'Security Posture', href: p('/security/posture') },
        { type: 'link', text: t('nav.compliance'), href: p('/compliance') },
        { type: 'link', text: 'Legal Holds', href: p('/compliance/legal-holds') },
      ],
    },
    {
      type: 'section',
      text: t('nav.section_analytics_storage'),
      ...e('analytics'),
      items: [
        { type: 'link', text: t('nav.quota_dashboard'), href: p('/storage/quota-dashboard') },
        { type: 'link', text: t('nav.quota_usage'), href: p('/storage/quota-usage') },
        { type: 'link', text: t('nav.quota_alerts'), href: p('/storage/quota-alerts') },
        { type: 'link', text: t('nav.attachments'), href: p('/storage/attachments') },
        { type: 'link', text: t('nav.drive'), href: p('/storage/drive') },
        { type: 'link', text: 'Seat Usage', href: p('/storage/seat-usage') },
        { type: 'link', text: t('nav.api_usage'), href: p('/analytics/api-usage') },
        { type: 'link', text: t('nav.push_notifications'), href: p('/analytics/push') },
        { type: 'link', text: t('nav.reports'), href: p('/reports') },
      ],
    },
  // eslint-disable-next-line react-hooks/exhaustive-deps
  ], [cid, t, activeSectionKey]);

  return (
    <SideNavigation
      key={activeSectionKey}
      items={navigationItems}
      activeHref={pathname ?? ''}
      header={{ href: p('/dashboard'), text: currentCompany?.name ?? 'GoGoMail' }}
      onFollow={(e) => {
        if (e.detail.external) return;
        e.preventDefault();
        router.push(e.detail.href);
      }}
    />
  );
}
