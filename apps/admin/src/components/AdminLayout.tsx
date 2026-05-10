'use client';

import { AppLayout, Flashbar, Select, SelectProps } from '@cloudscape-design/components';
import { Sidebar } from './Sidebar';
import { useState } from 'react';
import { useI18n } from '@/app/i18n-provider';
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

  const handleLanguageChange = (option: any) => {
    if (locales.includes(option.value as Locale)) {
      setLocale(option.value as Locale);
    }
  };

  return (
    <AppLayout
      navigation={<Sidebar />}
      content={children}
      toolsHide
      notifications={
        notifications.length > 0 ? (
          <Flashbar items={notifications} />
        ) : undefined
      }
      breadcrumbs={
        <div style={{ display: 'flex', justifyContent: 'flex-end', width: '100%' }}>
          <div style={{ width: '120px' }}>
            <Select
              selectedOption={languageOptions.find(opt => opt.value === locale) || languageOptions[0]}
              onChange={(e) => handleLanguageChange(e.detail.selectedOption)}
              options={languageOptions}
              expandToViewport
            />
          </div>
        </div>
      }
    />
  );
}
