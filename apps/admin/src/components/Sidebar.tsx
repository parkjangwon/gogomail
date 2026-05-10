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
    {
      type: 'link',
      text: 'Dashboard',
      href: `/companies/${companyId}/dashboard`,
    },
    {
      type: 'link',
      text: 'Audit Logs',
      href: `/companies/${companyId}/audit-logs`,
    },
    {
      type: 'link',
      text: 'Organization',
      href: `/companies/${companyId}/organization`,
    },
    {
      type: 'link',
      text: 'Reports',
      href: `/companies/${companyId}/reports`,
    },
    {
      type: 'link',
      text: 'Roles',
      href: `/companies/${companyId}/roles`,
    },
    {
      type: 'divider',
    },
    {
      type: 'link',
      text: 'API Keys',
      href: `/companies/${companyId}/api-keys`,
    },
    {
      type: 'link',
      text: 'Security Policy',
      href: `/companies/${companyId}/security-policy`,
    },
    {
      type: 'link',
      text: 'SSO',
      href: `/companies/${companyId}/sso-config`,
    },
    {
      type: 'link',
      text: 'Domains',
      href: `/companies/${companyId}/domains`,
    },
    {
      type: 'link',
      text: 'Compliance',
      href: `/companies/${companyId}/compliance`,
    },
    {
      type: 'divider',
    },
    {
      type: 'link',
      text: 'Alerts',
      href: `/companies/${companyId}/alerts`,
    },
    {
      type: 'link',
      text: 'Settings',
      href: `/companies/${companyId}/settings`,
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
