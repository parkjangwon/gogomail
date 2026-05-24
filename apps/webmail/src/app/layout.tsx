import './globals.css';
import { cookies } from 'next/headers';
import { NextIntlClientProvider } from 'next-intl';
import { Providers } from '@/components/Providers';

export const metadata = {
  title: 'GoGoMail Webmail',
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
            __html: `(function(){try{
var el=document.documentElement;
var s=localStorage.getItem('webmail_theme');
var d=window.matchMedia('(prefers-color-scheme: dark)').matches;
el.setAttribute('data-theme',s||(d?'dark':'light'));
if(localStorage.getItem('webmail_high_contrast')==='1')el.classList.add('high-contrast');
if(localStorage.getItem('webmail_reduced_motion')==='1')el.classList.add('reduced-motion');
if(localStorage.getItem('webmail_larger_targets')==='1')el.classList.add('larger-targets');
if(localStorage.getItem('webmail_screen_reader')==='1')el.classList.add('screen-reader-mode');
if(localStorage.getItem('webmail_always_focus_ring')==='1')el.classList.add('always-focus-ring');
if(localStorage.getItem('webmail_underline_links')==='1')el.classList.add('underline-links');
if(localStorage.getItem('webmail_dyslexia')==='1')el.classList.add('dyslexia-mode');
var cb=localStorage.getItem('webmail_colorblind');
if(cb&&cb!=='none')el.classList.add('colorblind-'+cb);
var ufs=localStorage.getItem('webmail_ui_font_size');
if(ufs&&ufs!=='md'){el.classList.add('ui-font-size-'+ufs);}
var ls=localStorage.getItem('webmail_line_spacing');
if(ls&&ls!=='normal')el.classList.add('line-spacing-'+ls);
var lts=localStorage.getItem('webmail_letter_spacing');
if(lts&&lts!=='normal')el.classList.add('letter-spacing-'+lts);
var ff=localStorage.getItem('webmail_font_family');
var fm={serif:'Georgia,serif',mono:'"JetBrains Mono","Fira Code",monospace'};
if(ff&&fm[ff])el.style.fontFamily=fm[ff];
}catch(e){}})();`,
          }}
        />
        <style dangerouslySetInnerHTML={{ __html: `
          @media print {
            body { background: white !important; color: black !important; }
            nav, aside, [role="navigation"], [data-print-hide],
            header, button, .compose-modal, [role="toolbar"] { display: none !important; }
            [data-print-reading-pane] {
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
