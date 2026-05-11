import { getRequestConfig } from 'next-intl/server';
import { cookies } from 'next/headers';

const VALID_LOCALES = ['ko', 'en', 'ja', 'zh-CN'] as const;
type Locale = (typeof VALID_LOCALES)[number];

async function loadMessages(locale: Locale) {
  const loaders: Record<Locale, () => Promise<{ default: Record<string, unknown> }>> = {
    ko: () => import('../../messages/ko.json'),
    en: () => import('../../messages/en.json'),
    ja: () => import('../../messages/ja.json'),
    'zh-CN': () => import('../../messages/zh-CN.json'),
  };
  return (await loaders[locale]()).default;
}

export default getRequestConfig(async () => {
  const cookieStore = await cookies();
  const raw = cookieStore.get('webmail_locale')?.value ?? 'ko';
  const locale: Locale = (VALID_LOCALES as readonly string[]).includes(raw)
    ? (raw as Locale)
    : 'ko';
  return {
    locale,
    messages: await loadMessages(locale),
  };
});
