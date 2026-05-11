'use client';

import {
  AppLayout,
  Flashbar,
  FlashbarProps,
  TopNavigation,
  Select,
  SelectProps,
  Box,
} from '@cloudscape-design/components';
import { Sidebar } from './Sidebar';
import { useState, useEffect, useRef } from 'react';
import { useRouter, usePathname } from 'next/navigation';
import { useI18n } from '@/app/i18n-provider';
import { useCompany } from '@/contexts/CompanyContext';
import { locales, type Locale } from '@/i18n/config';

const VISIT_KEY = 'ggm_recent_visits';
const MAX_VISITS = 30;

export function recordVisit(path: string) {
  if (typeof window === 'undefined') return;
  try {
    const raw = localStorage.getItem(VISIT_KEY);
    const visits: Array<{ path: string; ts: number }> = raw ? JSON.parse(raw) : [];
    const filtered = visits.filter(v => v.path !== path);
    filtered.unshift({ path, ts: Date.now() });
    localStorage.setItem(VISIT_KEY, JSON.stringify(filtered.slice(0, MAX_VISITS)));
  } catch {
    localStorage.removeItem(VISIT_KEY);
  }
}

export function getRecentVisits(): Array<{ path: string; ts: number }> {
  if (typeof window === 'undefined') return [];
  try {
    const raw = localStorage.getItem(VISIT_KEY);
    return raw ? JSON.parse(raw) : [];
  } catch {
    localStorage.removeItem(VISIT_KEY);
    return [];
  }
}

const languageOptions: SelectProps.Option[] = [
  { label: '한국어', value: 'ko' },
  { label: 'English', value: 'en' },
  { label: '日本語', value: 'ja' },
  { label: '简体中文', value: 'zh-CN' },
];

function useIsMobile(breakpoint = 688) {
  const [isMobile, setIsMobile] = useState(false);
  useEffect(() => {
    const mq = window.matchMedia(`(max-width: ${breakpoint}px)`);
    setIsMobile(mq.matches);
    const handler = (e: MediaQueryListEvent) => setIsMobile(e.matches);
    mq.addEventListener('change', handler);
    return () => mq.removeEventListener('change', handler);
  }, [breakpoint]);
  return isMobile;
}

export function AdminLayout({ children }: { children: React.ReactNode }) {
  const [notifications] = useState<FlashbarProps.MessageDefinition[]>([]);
  const [alertCount, setAlertCount] = useState(0);
  const { locale, setLocale } = useI18n();
  const { companies, currentCompany, switchCompany } = useCompany();
  const router = useRouter();
  const isMobile = useIsMobile();

  const cid = currentCompany?.id ?? 'default';
  const pathname = usePathname();

  useEffect(() => {
    if (pathname) recordVisit(pathname);
  }, [pathname]);

  const alertTimerRef = useRef<ReturnType<typeof setInterval> | null>(null);
  useEffect(() => {
    if (!currentCompany?.id) return;
    const fetchAlerts = () => {
      fetch(`/api/admin/companies/${currentCompany.id}/alert-events`, { credentials: 'include' })
        .then(r => r.ok ? r.json() : null)
        .then(data => { if (data) setAlertCount((data.events ?? []).length); })
        .catch(() => {});
    };
    fetchAlerts();
    alertTimerRef.current = setInterval(fetchAlerts, 60_000);
    return () => { if (alertTimerRef.current) clearInterval(alertTimerRef.current); };
  }, [currentCompany?.id]);

  return (
    <>
      <div id="top-nav">
        <TopNavigation
          identity={{
            href: `/companies/${cid}/dashboard`,
            title: isMobile ? 'GGM' : 'GoGoMail Admin',
          }}
          utilities={[
            {
              type: 'button',
              iconName: 'notification',
              badge: alertCount > 0,
              title: alertCount > 0 ? `${alertCount} active alert${alertCount > 1 ? 's' : ''}` : 'No active alerts',
              onClick: () => router.push(`/companies/${cid}/security/alerts`),
            },
            {
              type: 'menu-dropdown',
              text: isMobile
                ? (currentCompany?.name?.slice(0, 8) ?? '…')
                : (currentCompany?.name ?? 'Select Company'),
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
              text: isMobile ? '' : 'Admin',
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
        content={<div className="admin-content">{children}</div>}
        toolsHide
        maxContentWidth={1600}
        minContentWidth={0}
        headerSelector="#top-nav"
        notifications={
          notifications.length > 0 ? <Flashbar items={notifications} /> : undefined
        }
        breadcrumbs={
          <div className="admin-toolbar">
            {currentCompany && (
              <span className="admin-toolbar-company">
                <Box color="text-body-secondary" fontSize="body-s">
                  Company: <strong>{currentCompany.name}</strong>
                </Box>
              </span>
            )}
            <div style={{ width: isMobile ? '110px' : '130px' }}>
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
