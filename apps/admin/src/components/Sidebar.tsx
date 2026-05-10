'use client';

import { SideNavigation, SideNavigationProps } from '@cloudscape-design/components';
import { useRouter, usePathname, useParams } from 'next/navigation';

export function Sidebar() {
  const router = useRouter();
  const pathname = usePathname();
  const params = useParams();
  const companyId = params?.id as string;

  if (!companyId) return null;

  const navigationItems: SideNavigationProps.Item[] = [
    // MAIN
    {
      type: 'link',
      text: 'Dashboard',
      href: `/companies/${companyId}/dashboard`,
    },

    // SYSTEM
    {
      type: 'section',
      text: 'System',
      items: [
        {
          type: 'link',
          text: 'Queue Stats',
          href: `/companies/${companyId}/system/queue`,
        },
        {
          type: 'link',
          text: 'Backpressure',
          href: `/companies/${companyId}/system/backpressure`,
        },
        {
          type: 'link',
          text: 'API Health',
          href: `/companies/${companyId}/system/health`,
        },
      ],
    },

    // TENANCY
    {
      type: 'section',
      text: 'Tenancy',
      items: [
        {
          type: 'link',
          text: 'Companies',
          href: `/companies/${companyId}/tenancy/companies`,
        },
        {
          type: 'link',
          text: 'Domains',
          href: `/companies/${companyId}/tenancy/domains`,
        },
        {
          type: 'link',
          text: 'Domain Settings',
          href: `/companies/${companyId}/tenancy/domain-settings`,
        },
      ],
    },

    // USERS & ACCESS
    {
      type: 'section',
      text: 'Users & Access',
      items: [
        {
          type: 'link',
          text: 'Users',
          href: `/companies/${companyId}/users`,
        },
        {
          type: 'link',
          text: 'Admin Users',
          href: `/companies/${companyId}/admin-users`,
        },
        {
          type: 'link',
          text: 'Directory',
          href: `/companies/${companyId}/access/directory`,
        },
        {
          type: 'link',
          text: 'Aliases',
          href: `/companies/${companyId}/access/aliases`,
        },
        {
          type: 'link',
          text: 'Delegations',
          href: `/companies/${companyId}/access/delegations`,
        },
        {
          type: 'link',
          text: 'Group Memberships',
          href: `/companies/${companyId}/access/groups`,
        },
      ],
    },

    // DELIVERY & MAIL
    {
      type: 'section',
      text: 'Delivery & Mail',
      items: [
        {
          type: 'link',
          text: 'Delivery Routes',
          href: `/companies/${companyId}/delivery/routes`,
        },
        {
          type: 'link',
          text: 'Trusted Relays',
          href: `/companies/${companyId}/delivery/relays`,
        },
        {
          type: 'link',
          text: 'Mail Flow Logs',
          href: `/companies/${companyId}/mail/flow-logs`,
        },
        {
          type: 'link',
          text: 'Outbox Events',
          href: `/companies/${companyId}/mail/outbox`,
        },
        {
          type: 'link',
          text: 'Delivery Attempts',
          href: `/companies/${companyId}/mail/delivery-attempts`,
        },
      ],
    },

    // SECURITY
    {
      type: 'section',
      text: 'Security',
      items: [
        {
          type: 'link',
          text: 'API Keys',
          href: `/companies/${companyId}/security/api-keys`,
        },
        {
          type: 'link',
          text: 'DKIM Keys',
          href: `/companies/${companyId}/security/dkim-keys`,
        },
        {
          type: 'link',
          text: 'Audit Logs',
          href: `/companies/${companyId}/audit-logs`,
        },
        {
          type: 'link',
          text: 'Suppression List',
          href: `/companies/${companyId}/security/suppression`,
        },
        {
          type: 'link',
          text: 'Alert Rules',
          href: `/companies/${companyId}/security/alerts`,
        },
      ],
    },

    // STORAGE & QUOTAS
    {
      type: 'section',
      text: 'Storage & Quotas',
      items: [
        {
          type: 'link',
          text: 'Quota Usage',
          href: `/companies/${companyId}/storage/quota-usage`,
        },
        {
          type: 'link',
          text: 'Quota Alerts',
          href: `/companies/${companyId}/storage/quota-alerts`,
        },
        {
          type: 'link',
          text: 'Attachments',
          href: `/companies/${companyId}/storage/attachments`,
        },
        {
          type: 'link',
          text: 'Drive',
          href: `/companies/${companyId}/storage/drive`,
        },
        {
          type: 'link',
          text: 'Quota Reconciliation',
          href: `/companies/${companyId}/storage/reconciliation`,
        },
      ],
    },

    // ANALYTICS
    {
      type: 'section',
      text: 'Analytics',
      items: [
        {
          type: 'link',
          text: 'API Usage',
          href: `/companies/${companyId}/analytics/api-usage`,
        },
        {
          type: 'link',
          text: 'Push Notifications',
          href: `/companies/${companyId}/analytics/push`,
        },
        {
          type: 'link',
          text: 'Reports',
          href: `/companies/${companyId}/reports`,
        },
      ],
    },

    // CONFIG
    {
      type: 'section',
      text: 'Configuration',
      items: [
        {
          type: 'link',
          text: 'Company Config',
          href: `/companies/${companyId}/config/company`,
        },
        {
          type: 'link',
          text: 'Domain Config',
          href: `/companies/${companyId}/config/domain`,
        },
        {
          type: 'link',
          text: 'User Config',
          href: `/companies/${companyId}/config/user`,
        },
      ],
    },

    // ORGANIZATION (legacy, keep for now)
    {
      type: 'section',
      text: 'Organization',
      items: [
        {
          type: 'link',
          text: 'Settings',
          href: `/companies/${companyId}/organization`,
        },
        {
          type: 'link',
          text: 'Roles',
          href: `/companies/${companyId}/roles`,
        },
        {
          type: 'link',
          text: 'Compliance',
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
