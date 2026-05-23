import type { MessageDetail, MessageAddress } from '@/lib/api';

export interface ParsedAddress {
  address: string;
  name?: string;
}

export interface PickerItem {
  id: string;
  display_name: string;
  email: string;
  kind?: 'user' | 'org' | 'addressbook';
  include_children?: boolean;
  count?: number;
}

function escapeHtml(text: string): string {
  return text.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

export function splitAddressList(raw: string): string[] {
  const parts: string[] = [];
  let depth = 0;
  let start = 0;
  for (let i = 0; i < raw.length; i++) {
    if (raw[i] === '<') depth++;
    else if (raw[i] === '>') depth--;
    else if ((raw[i] === ',' || raw[i] === ';') && depth === 0) {
      const part = raw.slice(start, i).trim();
      if (part) parts.push(part);
      start = i + 1;
    }
  }
  const last = raw.slice(start).trim();
  if (last) parts.push(last);
  return parts;
}

export function parseAddress(raw: string): ParsedAddress {
  const m = raw.match(/^(.+?)\s*<([^>]+)>$/);
  if (m) return { name: m[1].trim() || undefined, address: m[2].trim() };
  return { address: raw.trim() };
}

export function parseAddressList(raw: string): ParsedAddress[] {
  return splitAddressList(raw).map((part) => parseAddress(part)).filter((addr) => addr.address);
}

export function isRecipientGroupToken(address: string): boolean {
  return /^org:[0-9a-fA-F-]{36}(?::children)?$/.test(address) || /^addressbook:[0-9a-fA-F-]{36}$/.test(address);
}

export function isValidRecipientAddress(address: string): boolean {
  if (isRecipientGroupToken(address)) return true;
  if (!address || /\s|<|>/.test(address)) return false;
  const at = address.indexOf('@');
  if (at <= 0 || at !== address.lastIndexOf('@') || at === address.length - 1) return false;
  const domain = address.slice(at + 1);
  if (domain.startsWith('.') || domain.endsWith('.') || domain.includes('..')) return false;
  return true;
}

export function invalidRecipientAddresses(...values: string[]): string[] {
  return values
    .flatMap((value) => parseAddressList(value))
    .map((addr) => addr.address)
    .filter((address) => !isValidRecipientAddress(address));
}

export function emailOf(addr: MessageAddress): string {
  return addr.email || addr.address || '';
}

function pickerItemKindFromEmail(email: string): PickerItem['kind'] {
  if (email.startsWith('org:')) return 'org';
  if (email.startsWith('addressbook:')) return 'addressbook';
  return 'user';
}

export function parseToPickerItems(str: string): PickerItem[] {
  if (!str.trim()) return [];
  return splitAddressList(str).map((part) => {
    const parsed = parseAddress(part);
    const kind = pickerItemKindFromEmail(parsed.address);
    return {
      id: parsed.address,
      display_name: parsed.name || parsed.address,
      email: parsed.address,
      kind,
    };
  });
}

export function pickerItemsToString(items: PickerItem[]): string {
  return items
    .map((item) => (item.display_name && item.display_name !== item.email ? `${item.display_name} <${item.email}>` : item.email))
    .join(', ');
}

type TFn = (key: string, values?: Record<string, unknown>) => string;

export function buildQuoteHTML(intent: string, source: MessageDetail, t?: TFn): string {
  const from = source.from_name
    ? `${escapeHtml(source.from_name)} &lt;${escapeHtml(source.from_addr)}&gt;`
    : escapeHtml(source.from_addr);
  const date = new Intl.DateTimeFormat('ko-KR', {
    year: 'numeric',
    month: 'long',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).format(new Date(source.received_at));
  const bodyLines = (source.text_body || '')
    .split('\n')
    .map((line) => `<p>${escapeHtml(line) || '&nbsp;'}</p>`)
    .join('');
  const forwardedHeader = t ? t('misc.compose.forwardedHeader') : '---------- 전달된 메시지 ----------';
  const originalHeader = t ? t('misc.compose.originalHeader') : '--- 원본 메시지 ---';
  const header = intent === 'forward'
    ? `<p><strong>${escapeHtml(forwardedHeader)}</strong></p>`
    : `<p><strong>${escapeHtml(originalHeader)}</strong></p>`;
  const fromLabel = t ? t('misc.compose.fromLabel') : '보낸 사람:';
  const dateLabel = t ? t('misc.compose.dateLabel') : '날짜:';
  const subjectLabel = t ? t('misc.compose.subjectLabel') : '제목:';
  const noSubject = t ? t('misc.compose.noSubject') : '(제목 없음)';
  return `<p></p>${header}<blockquote><p><strong>${escapeHtml(fromLabel)}</strong> ${from}</p><p><strong>${escapeHtml(dateLabel)}</strong> ${escapeHtml(date)}</p><p><strong>${escapeHtml(subjectLabel)}</strong> ${escapeHtml(source.subject || noSubject)}</p><p>&nbsp;</p>${bodyLines}</blockquote>`;
}
