import './globals.css';
import { cookies } from 'next/headers';
import { NextIntlClientProvider } from 'next-intl';
import { Providers } from '@/components/Providers';

export const metadata = {
  title: 'GoGoMail 웹메일',
  description: 'Webmail',
};

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

export default async function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const cookieStore = await cookies();
  const raw = cookieStore.get('webmail_locale')?.value ?? 'ko';
  const locale: Locale = (VALID_LOCALES as readonly string[]).includes(raw)
    ? (raw as Locale)
    : 'ko';
  const messages = await loadMessages(locale);

  return (
    <html lang={locale} suppressHydrationWarning>
      <head>
        <meta name="viewport" content="width=device-width, initial-scale=1" />
        <script
          dangerouslySetInnerHTML={{
            __html: `(function(){try{var s=localStorage.getItem('webmail_theme');var d=window.matchMedia('(prefers-color-scheme: dark)').matches;document.documentElement.setAttribute('data-theme',s||(d?'dark':'light'));}catch(e){}})();`,
          }}
        />
        <style dangerouslySetInnerHTML={{ __html: `
          @media print {
            body { background: white !important; color: black !important; }
            nav, aside, [role="navigation"], [aria-label="폴더 목록"],
            [aria-label="메일 목록"], [aria-label="사이드바"], header,
            button, .compose-modal, [role="toolbar"] { display: none !important; }
            [aria-label="메일 읽기"] {
              position: static !important;
              width: 100% !important;
              height: auto !important;
              overflow: visible !important;
              box-shadow: none !important;
              border: none !important;
              font-size: 13pt !important;
            }
            a[href]:after { content: " (" attr(href) ")"; font-size: 10pt; color: #666; }
          }
        `}} />
      </head>
      <body>
        <NextIntlClientProvider locale={locale} messages={messages}>
          <Providers>{children}</Providers>
        </NextIntlClientProvider>
      </body>
    </html>
  );
}
