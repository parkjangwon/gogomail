import { defineConfig, type DefaultTheme } from 'vitepress';
import { docsMessages, type DocsMessages } from './i18n/messages';
import type { DocsLocale } from './i18n/terms';

const localePaths: Record<DocsLocale, string> = {
  en: '/',
  ko: '/ko/',
  ja: '/ja/',
  'zh-CN': '/zh-CN/',
};

function localizedPath(locale: DocsLocale, path: string) {
  return locale === 'en' ? path : `/${locale}${path}`;
}

function nav(locale: DocsLocale, messages: DocsMessages): DefaultTheme.NavItem[] {
  return [
    { text: messages.nav.home, link: localizedPath(locale, '/') },
    { text: messages.nav.adminConsole, link: localizedPath(locale, '/admin-console/') },
    { text: messages.nav.webmail, link: localizedPath(locale, '/webmail/') },
    { text: messages.nav.mcp, link: localizedPath(locale, '/mcp/') },
    { text: messages.pages.glossary.title, link: localizedPath(locale, '/glossary') },
  ];
}

function sidebar(locale: DocsLocale, messages: DocsMessages): DefaultTheme.Sidebar {
  const page = (key: string, path: string) => ({ text: messages.pages[key].title, link: localizedPath(locale, path) });

  return [
    {
      text: messages.nav.home,
      collapsed: false,
      items: [
        page('home', '/'),
        page('glossary', '/glossary'),
      ],
    },
    {
      text: messages.nav.adminConsole,
      collapsed: false,
      items: [
        page('adminOverview', '/admin-console/'),
        page('adminGettingStarted', '/admin-console/getting-started'),
        page('adminRoles', '/admin-console/roles'),
        page('adminResources', '/admin-console/resources'),
        page('adminSettings', '/admin-console/settings'),
        page('externalIntegration', '/admin-console/external-integration'),
        page('adminOperations', '/admin-console/operations'),
        page('adminAccess', '/admin-console/access-control'),
        page('adminGovernance', '/admin-console/governance'),
        page('adminAnalyticsStorage', '/admin-console/analytics-storage'),
      ],
    },
    {
      text: messages.nav.webmail,
      collapsed: false,
      items: [
        page('webmailOverview', '/webmail/'),
        page('webmailGettingStarted', '/webmail/getting-started'),
        page('webmailMail', '/webmail/mail'),
        page('webmailCompose', '/webmail/compose'),
        page('webmailSettings', '/webmail/settings'),
        page('webmailApps', '/webmail/apps'),
        page('webmailShortcuts', '/webmail/shortcuts'),
      ],
    },
    {
      text: messages.nav.mcp,
      collapsed: false,
      items: [
        page('mcpOverview', '/mcp/'),
        page('manageMcp', '/mcp/manage'),
        page('userMcp', '/mcp/user'),
      ],
    },
  ];
}

function themeConfig(locale: DocsLocale): DefaultTheme.Config {
  const messages = docsMessages[locale];
  return {
    logo: '/logo.svg',
    siteTitle: messages.site.title,
    nav: nav(locale, messages),
    sidebar: sidebar(locale, messages),
    search: {
      provider: 'local',
      options: {
        translations: {
          button: {
            buttonText: messages.site.search,
            buttonAriaLabel: messages.site.search,
          },
        },
      },
    },
    outline: {
      level: [2, 3],
    },
    footer: {
      message: messages.site.openSource,
      copyright: `${messages.site.lastUpdated}: 2026-05-16`,
    },
  };
}

export default defineConfig({
  title: docsMessages.en.site.title,
  description: docsMessages.en.site.description,
  appearance: true,
  cleanUrls: true,
  lastUpdated: true,
  head: [
    ['meta', { name: 'theme-color', content: '#f5f5f7' }],
  ],
  themeConfig: themeConfig('en'),
  locales: {
    root: {
      label: docsMessages.en.site.label,
      lang: 'en-US',
      title: docsMessages.en.site.title,
      description: docsMessages.en.site.description,
      themeConfig: themeConfig('en'),
    },
    ko: {
      label: docsMessages.ko.site.label,
      lang: 'ko-KR',
      link: localePaths.ko,
      title: docsMessages.ko.site.title,
      description: docsMessages.ko.site.description,
      themeConfig: themeConfig('ko'),
    },
    ja: {
      label: docsMessages.ja.site.label,
      lang: 'ja-JP',
      link: localePaths.ja,
      title: docsMessages.ja.site.title,
      description: docsMessages.ja.site.description,
      themeConfig: themeConfig('ja'),
    },
    'zh-CN': {
      label: docsMessages['zh-CN'].site.label,
      lang: 'zh-CN',
      link: localePaths['zh-CN'],
      title: docsMessages['zh-CN'].site.title,
      description: docsMessages['zh-CN'].site.description,
      themeConfig: themeConfig('zh-CN'),
    },
  },
});
