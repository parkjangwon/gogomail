'use client';

import { SideNavigation, SideNavigationProps } from '@cloudscape-design/components';
import { useRouter, usePathname } from 'next/navigation';
import { useI18n } from '@/app/i18n-provider';
import { useCompany } from '@/contexts/CompanyContext';

export function Sidebar() {
  const router = useRouter();
  const pathname = usePathname();
  const { t } = useI18n();
  const { currentCompany } = useCompany();
  const cid = currentCompany?.id ?? 'default';

  const p = (path: string) => `/companies/${cid}${path}`;

  const navigationItems: SideNavigationProps.Item[] = [
    {
      type: 'link',
      text: t('nav.dashboard'),
      href: p('/dashboard'),
    },
    { type: 'divider' },

    // RESOURCES
    {
      type: 'section',
      text: 'Resources',
      defaultExpanded: true,
      items: [
        { type: 'link', text: t('nav.companies'), href: p('/tenancy/companies') },
        { type: 'link', text: t('nav.domains'), href: p('/tenancy/domains') },
        { type: 'link', text: t('nav.users'), href: p('/users') },
        { type: 'link', text: t('nav.admin_users'), href: p('/admin-users') },
      ],
    },

    // OPERATIONS
    {
      type: 'section',
      text: 'Operations',
      defaultExpanded: false,
      items: [
        { type: 'link', text: t('nav.mail_flow_logs'), href: p('/mail/flow-logs') },
        { type: 'link', text: t('nav.outbox_events'), href: p('/mail/outbox') },
        { type: 'link', text: t('nav.delivery_attempts'), href: p('/mail/delivery-attempts') },
        { type: 'link', text: t('nav.delivery_routes'), href: p('/delivery/routes') },
        { type: 'link', text: t('nav.trusted_relays'), href: p('/delivery/relays') },
        { type: 'link', text: t('nav.queue_stats'), href: p('/system/queue') },
        { type: 'link', text: t('nav.backpressure'), href: p('/system/backpressure') },
        { type: 'link', text: t('nav.api_health'), href: p('/system/health') },
      ],
    },

    // ACCESS CONTROL
    {
      type: 'section',
      text: 'Access Control',
      defaultExpanded: false,
      items: [
        { type: 'link', text: t('nav.directory'), href: p('/access/directory') },
        { type: 'link', text: t('nav.aliases'), href: p('/access/aliases') },
        { type: 'link', text: t('nav.delegations'), href: p('/access/delegations') },
        { type: 'link', text: t('nav.group_memberships'), href: p('/access/groups') },
        { type: 'link', text: t('nav.roles'), href: p('/roles') },
      ],
    },

    // GOVERNANCE
    {
      type: 'section',
      text: 'Governance',
      defaultExpanded: false,
      items: [
        { type: 'link', text: t('nav.audit_logs'), href: p('/audit-logs') },
        { type: 'link', text: t('nav.alert_rules'), href: p('/security/alerts') },
        { type: 'link', text: t('nav.suppression_list'), href: p('/security/suppression') },
        { type: 'link', text: t('nav.dkim_keys'), href: p('/security/dkim-keys') },
        { type: 'link', text: t('nav.api_keys'), href: p('/security/api-keys') },
        { type: 'link', text: t('nav.compliance'), href: p('/compliance') },
      ],
    },

    // ANALYTICS & STORAGE
    {
      type: 'section',
      text: 'Analytics & Storage',
      defaultExpanded: false,
      items: [
        { type: 'link', text: t('nav.quota_usage'), href: p('/storage/quota-usage') },
        { type: 'link', text: t('nav.quota_alerts'), href: p('/storage/quota-alerts') },
        { type: 'link', text: t('nav.attachments'), href: p('/storage/attachments') },
        { type: 'link', text: t('nav.drive'), href: p('/storage/drive') },
        { type: 'link', text: t('nav.api_usage'), href: p('/analytics/api-usage') },
        { type: 'link', text: t('nav.push_notifications'), href: p('/analytics/push') },
        { type: 'link', text: t('nav.reports'), href: p('/reports') },
      ],
    },
  ];

  return (
    <SideNavigation
      items={navigationItems}
      activeHref={pathname ?? ''}
      header={{
        href: p('/dashboard'),
        text: currentCompany?.name ?? 'GoGoMail',
      }}
      onFollow={(e) => {
        if (e.detail.external) return;
        e.preventDefault();
        router.push(e.detail.href);
      }}
    />
  );
}
