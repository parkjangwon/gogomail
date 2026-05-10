'use client';

import { SideNavigation, SideNavigationProps } from '@cloudscape-design/components';
import { useRouter, usePathname, useParams } from 'next/navigation';
import { useI18n } from '@/app/i18n-provider';

export function Sidebar() {
  const router = useRouter();
  const pathname = usePathname();
  const params = useParams();
  const companyId = params?.id as string;
  const { t } = useI18n();

  if (!companyId) return null;

  const navigationItems: SideNavigationProps.Item[] = [
    // MAIN
    {
      type: 'link',
      text: t('nav.dashboard'),
      href: `/companies/${companyId}/dashboard`,
    },

    // SYSTEM
    {
      type: 'section',
      text: t('nav.system'),
      items: [
        {
          type: 'link',
          text: t('nav.queue_stats'),
          href: `/companies/${companyId}/system/queue`,
        },
        {
          type: 'link',
          text: t('nav.backpressure'),
          href: `/companies/${companyId}/system/backpressure`,
        },
        {
          type: 'link',
          text: t('nav.api_health'),
          href: `/companies/${companyId}/system/health`,
        },
      ],
    },

    // TENANCY
    {
      type: 'section',
      text: t('nav.tenancy'),
      items: [
        {
          type: 'link',
          text: t('nav.companies'),
          href: `/companies/${companyId}/tenancy/companies`,
        },
        {
          type: 'link',
          text: t('nav.domains'),
          href: `/companies/${companyId}/tenancy/domains`,
        },
        {
          type: 'link',
          text: t('nav.domain_settings'),
          href: `/companies/${companyId}/tenancy/domain-settings`,
        },
      ],
    },

    // USERS & ACCESS
    {
      type: 'section',
      text: t('nav.users_access'),
      items: [
        {
          type: 'link',
          text: t('nav.users'),
          href: `/companies/${companyId}/users`,
        },
        {
          type: 'link',
          text: t('nav.admin_users'),
          href: `/companies/${companyId}/admin-users`,
        },
        {
          type: 'link',
          text: t('nav.directory'),
          href: `/companies/${companyId}/access/directory`,
        },
        {
          type: 'link',
          text: t('nav.aliases'),
          href: `/companies/${companyId}/access/aliases`,
        },
        {
          type: 'link',
          text: t('nav.delegations'),
          href: `/companies/${companyId}/access/delegations`,
        },
        {
          type: 'link',
          text: t('nav.group_memberships'),
          href: `/companies/${companyId}/access/groups`,
        },
      ],
    },

    // DELIVERY & MAIL
    {
      type: 'section',
      text: t('nav.delivery_mail'),
      items: [
        {
          type: 'link',
          text: t('nav.delivery_routes'),
          href: `/companies/${companyId}/delivery/routes`,
        },
        {
          type: 'link',
          text: t('nav.trusted_relays'),
          href: `/companies/${companyId}/delivery/relays`,
        },
        {
          type: 'link',
          text: t('nav.mail_flow_logs'),
          href: `/companies/${companyId}/mail/flow-logs`,
        },
        {
          type: 'link',
          text: t('nav.outbox_events'),
          href: `/companies/${companyId}/mail/outbox`,
        },
        {
          type: 'link',
          text: t('nav.delivery_attempts'),
          href: `/companies/${companyId}/mail/delivery-attempts`,
        },
      ],
    },

    // SECURITY
    {
      type: 'section',
      text: t('nav.security'),
      items: [
        {
          type: 'link',
          text: t('nav.api_keys'),
          href: `/companies/${companyId}/security/api-keys`,
        },
        {
          type: 'link',
          text: t('nav.dkim_keys'),
          href: `/companies/${companyId}/security/dkim-keys`,
        },
        {
          type: 'link',
          text: t('nav.audit_logs'),
          href: `/companies/${companyId}/audit-logs`,
        },
        {
          type: 'link',
          text: t('nav.suppression_list'),
          href: `/companies/${companyId}/security/suppression`,
        },
        {
          type: 'link',
          text: t('nav.alert_rules'),
          href: `/companies/${companyId}/security/alerts`,
        },
      ],
    },

    // STORAGE & QUOTAS
    {
      type: 'section',
      text: t('nav.storage_quotas'),
      items: [
        {
          type: 'link',
          text: t('nav.quota_usage'),
          href: `/companies/${companyId}/storage/quota-usage`,
        },
        {
          type: 'link',
          text: t('nav.quota_alerts'),
          href: `/companies/${companyId}/storage/quota-alerts`,
        },
        {
          type: 'link',
          text: t('nav.attachments'),
          href: `/companies/${companyId}/storage/attachments`,
        },
        {
          type: 'link',
          text: t('nav.drive'),
          href: `/companies/${companyId}/storage/drive`,
        },
        {
          type: 'link',
          text: t('nav.quota_reconciliation'),
          href: `/companies/${companyId}/storage/reconciliation`,
        },
      ],
    },

    // ANALYTICS
    {
      type: 'section',
      text: t('nav.analytics'),
      items: [
        {
          type: 'link',
          text: t('nav.api_usage'),
          href: `/companies/${companyId}/analytics/api-usage`,
        },
        {
          type: 'link',
          text: t('nav.push_notifications'),
          href: `/companies/${companyId}/analytics/push`,
        },
        {
          type: 'link',
          text: t('nav.reports'),
          href: `/companies/${companyId}/reports`,
        },
      ],
    },

    // CONFIG
    {
      type: 'section',
      text: t('nav.configuration'),
      items: [
        {
          type: 'link',
          text: t('nav.company_config'),
          href: `/companies/${companyId}/config/company`,
        },
        {
          type: 'link',
          text: t('nav.domain_config'),
          href: `/companies/${companyId}/config/domain`,
        },
        {
          type: 'link',
          text: t('nav.user_config'),
          href: `/companies/${companyId}/config/user`,
        },
      ],
    },

    // ORGANIZATION
    {
      type: 'section',
      text: t('nav.organization'),
      items: [
        {
          type: 'link',
          text: t('nav.settings'),
          href: `/companies/${companyId}/organization`,
        },
        {
          type: 'link',
          text: t('nav.roles'),
          href: `/companies/${companyId}/roles`,
        },
        {
          type: 'link',
          text: t('nav.compliance'),
          href: `/companies/${companyId}/compliance`,
        },
      ],
    },
  ];

  const activeHref = navigationItems
    .filter((item) => item.type === 'link')
    .find((item) => pathname?.includes((item.href as string).split('/').pop() || ''))?.href;

  return (
    <SideNavigation
      items={navigationItems}
      activeHref={activeHref}
      header={{
        href: `/companies/${companyId}`,
        text: 'Admin Console',
      }}
      onFollow={(e) => {
        if (e.detail.external) return;
        e.preventDefault();
        router.push(e.detail.href);
      }}
    />
  );
}
