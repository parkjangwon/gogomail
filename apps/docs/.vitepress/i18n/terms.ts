import consoleEn from '../../../console/src/messages/en.json';
import consoleKo from '../../../console/src/messages/ko.json';
import consoleJa from '../../../console/src/messages/ja.json';
import consoleZhCN from '../../../console/src/messages/zh-CN.json';
import webmailEn from '../../../webmail/messages/en.json';
import webmailKo from '../../../webmail/messages/ko.json';
import webmailJa from '../../../webmail/messages/ja.json';
import webmailZhCN from '../../../webmail/messages/zh-CN.json';

export const localeCodes = ['en', 'ko', 'ja', 'zh-CN'] as const;
export type DocsLocale = (typeof localeCodes)[number];

type ConsoleMessages = {
  common: Record<string, string>;
  nav: Record<string, string>;
  login: Record<string, string>;
  layout: Record<string, string>;
  pages: Record<string, Record<string, string | Record<string, string>>>;
};

type WebmailMessages = typeof webmailEn;

const consoleMessages: Record<DocsLocale, ConsoleMessages> = {
  en: consoleEn as unknown as ConsoleMessages,
  ko: consoleKo as unknown as ConsoleMessages,
  ja: consoleJa as unknown as ConsoleMessages,
  'zh-CN': consoleZhCN as unknown as ConsoleMessages,
};

const webmailMessages: Record<DocsLocale, WebmailMessages> = {
  en: webmailEn,
  ko: webmailKo,
  ja: webmailJa,
  'zh-CN': webmailZhCN,
};

export function consoleTerms(locale: DocsLocale) {
  return consoleMessages[locale];
}

export function webmailTerms(locale: DocsLocale) {
  return webmailMessages[locale];
}

export function localeFromLang(lang: string): DocsLocale {
  if (lang.startsWith('ko')) return 'ko';
  if (lang.startsWith('ja')) return 'ja';
  if (lang.startsWith('zh')) return 'zh-CN';
  return 'en';
}
