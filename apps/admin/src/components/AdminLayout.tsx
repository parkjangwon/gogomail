'use client';

import {
  AppLayout,
  Flashbar,
  TopNavigation,
  Select,
  SelectProps,
  Box,
} from '@cloudscape-design/components';
import { Sidebar } from './Sidebar';
import { useState } from 'react';
import { useRouter } from 'next/navigation';
import { useI18n } from '@/app/i18n-provider';
import { useCompany } from '@/contexts/CompanyContext';
import { locales, type Locale } from '@/i18n/config';

const languageOptions: SelectProps.Option[] = [
  { label: '한국어', value: 'ko' },
  { label: 'English', value: 'en' },
  { label: '日本語', value: 'ja' },
  { label: '简体中文', value: 'zh-CN' },
];

export function AdminLayout({ children }: { children: React.ReactNode }) {
  const [notifications] = useState<any[]>([]);
  const { locale, setLocale } = useI18n();
  const { companies, currentCompany, switchCompany } = useCompany();
  const router = useRouter();

  const cid = currentCompany?.id ?? 'default';

  return (
    <>
      <div id="top-nav">
        <TopNavigation
          identity={{
            href: `/companies/${cid}/dashboard`,
            title: 'GoGoMail Admin',
          }}
          utilities={[
            {
              type: 'menu-dropdown',
              text: currentCompany?.name ?? 'Select Company',
              description: currentCompany ? `${currentCompany.status}` : '',
              iconName: 'settings',
              items: [
                ...companies.map(c => ({
                  id: c.id,
                  text: c.name,
                  description: c.status,
                })),
                {
                  id: 'manage',
                  text: '+ Manage Companies',
                },
              ],
              onItemClick: (e) => {
                if (e.detail.id === 'manage') {
                  router.push(`/companies/${cid}/tenancy/companies`);
                } else {
                  switchCompany(e.detail.id);
                  router.push(`/companies/${e.detail.id}/dashboard`);
                }
              },
            },
            {
              type: 'menu-dropdown',
              iconName: 'user-profile',
              text: 'Admin',
              items: [
                { id: 'settings', text: 'Settings' },
                { id: 'signout', text: 'Sign out' },
              ],
              onItemClick: (e) => {
                if (e.detail.id === 'signout') router.push('/login');
                if (e.detail.id === 'settings') router.push(`/companies/${cid}/organization`);
              },
            },
          ]}
        />
      </div>
      <AppLayout
        navigation={<Sidebar />}
        content={children}
        toolsHide
        maxContentWidth={1600}
        headerSelector="#top-nav"
        notifications={
          notifications.length > 0 ? <Flashbar items={notifications} /> : undefined
        }
        breadcrumbs={
          <div style={{ display: 'flex', justifyContent: 'flex-end', alignItems: 'center', gap: '8px' }}>
            {currentCompany && (
              <Box color="text-body-secondary" fontSize="body-s">
                Company: <strong>{currentCompany.name}</strong>
              </Box>
            )}
            <div style={{ width: '130px' }}>
              <Select
                selectedOption={languageOptions.find(o => o.value === locale) ?? languageOptions[0]}
                onChange={(e) => {
                  if (locales.includes(e.detail.selectedOption.value as Locale)) {
                    setLocale(e.detail.selectedOption.value as Locale);
                  }
                }}
                options={languageOptions}
                expandToViewport
              />
            </div>
          </div>
        }
      />
    </>
  );
}
