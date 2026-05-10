import './globals.css';
import { NextIntlClientProvider } from 'next-intl';
import { Providers } from '@/components/Providers';
import koMessages from '../../messages/ko.json';

export const metadata = {
  title: 'GoGoMail',
  description: 'Webmail',
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="ko">
      <head>
        <meta name="viewport" content="width=device-width, initial-scale=1" />
        <script
          dangerouslySetInnerHTML={{
            __html: `(function(){try{var s=localStorage.getItem('webmail_theme');var d=window.matchMedia('(prefers-color-scheme: dark)').matches;document.documentElement.setAttribute('data-theme',s||(d?'dark':'light'));}catch(e){}})();`,
          }}
        />
      </head>
      <body>
        <NextIntlClientProvider locale="ko" messages={koMessages}>
          <Providers>{children}</Providers>
        </NextIntlClientProvider>
      </body>
    </html>
  );
}
