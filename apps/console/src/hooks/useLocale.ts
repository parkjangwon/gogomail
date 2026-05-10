'use client';

import { useState, useEffect } from 'react';
import { locales, defaultLocale, type Locale } from '@/i18n/config';

export function useLocale() {
  const [locale, setLocaleState] = useState<Locale>(defaultLocale);
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    // Get locale from localStorage or navigator
    const savedLocale = localStorage.getItem('locale') as Locale | null;
    const browserLocale = navigator.language.split('-')[0] as Locale;
    const initialLocale = savedLocale || (locales.includes(browserLocale) ? browserLocale : defaultLocale);
    setLocaleState(initialLocale);
    setMounted(true);
  }, []);

  const setLocale = (newLocale: Locale) => {
    setLocaleState(newLocale);
    localStorage.setItem('locale', newLocale);
  };

  return { locale, setLocale, mounted };
}
