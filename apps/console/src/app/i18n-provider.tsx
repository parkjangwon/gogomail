'use client';

import { ReactNode, createContext, useContext, useState, useEffect } from 'react';
import { locales, defaultLocale, type Locale } from '@/i18n/config';
import koMessages from '@/messages/ko.json';
import enMessages from '@/messages/en.json';
import jaMessages from '@/messages/ja.json';
import zhMessages from '@/messages/zh-CN.json';

type Messages = Record<string, any>;

interface I18nContextType {
  locale: Locale;
  setLocale: (locale: Locale) => void;
  messages: Messages;
  t: (key: string, defaultValue?: string) => string;
}

const I18nContext = createContext<I18nContextType | undefined>(undefined);

const messageMap: Record<Locale, Messages> = {
  ko: koMessages,
  en: enMessages,
  ja: jaMessages,
  'zh-CN': zhMessages,
};

function getNestedValue(obj: any, path: string): any {
  return path.split('.').reduce((acc, part) => acc?.[part], obj);
}

export function I18nProvider({ children }: { children: ReactNode }) {
  const [locale, setLocaleState] = useState<Locale>(defaultLocale);

  useEffect(() => {
    const savedLocale = localStorage.getItem('locale') as Locale | null;
    const browserLocale = navigator.language.split('-')[0] as Locale;
    const initialLocale = savedLocale || (locales.includes(browserLocale) ? browserLocale : defaultLocale);
    setLocaleState(initialLocale);
  }, []);

  const setLocale = (newLocale: Locale) => {
    setLocaleState(newLocale);
    localStorage.setItem('locale', newLocale);
  };

  const messages = messageMap[locale];

  const t = (key: string, defaultValue = key) => {
    const value = getNestedValue(messages, key);
    return value || defaultValue;
  };

  return (
    <I18nContext.Provider value={{ locale, setLocale, messages, t }}>
      {children}
    </I18nContext.Provider>
  );
}

export function useI18n() {
  const context = useContext(I18nContext);
  if (context === undefined) {
    throw new Error('useI18n must be used within I18nProvider');
  }
  return context;
}
