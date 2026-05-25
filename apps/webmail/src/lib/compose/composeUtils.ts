import type { UIComposeIntent, ComposeIntent, MessageAddress, MessageDetail } from '@/lib/api';

export interface EmailTemplate {
  id: string;
  name: string;
  subject: string;
  body: string;
}

export function escapeHtml(text: string): string {
  return text.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

export function parseAddr(raw: string): { address: string; name?: string } {
  const m = raw.match(/^(.+?)\s*<([^>]+)>$/);
  if (m) return { name: m[1].trim() || undefined, address: m[2].trim() };
  return { address: raw.trim() };
}

export function parseAddrs(raw: string): { address: string; name?: string }[] {
  const parts: string[] = [];
  let depth = 0, start = 0;
  for (let i = 0; i < raw.length; i++) {
    if (raw[i] === '<') depth++;
    else if (raw[i] === '>') depth--;
    else if (raw[i] === ',' && depth === 0) {
      parts.push(raw.slice(start, i));
      start = i + 1;
    }
  }
  parts.push(raw.slice(start));
  return parts.map((p) => parseAddr(p.trim())).filter((a) => a.address);
}

export function isRecipientGroupToken(address: string): boolean {
  return /^org:[0-9a-fA-F-]{36}(?::children)?$/.test(address) || /^addressbook:[0-9a-fA-F-]{36}$/.test(address);
}

export function isValidEmailAddress(address: string): boolean {
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
    .flatMap((value) => parseAddrs(value))
    .map((addr) => addr.address)
    .filter((address) => !isValidEmailAddress(address));
}

export function backendComposeIntent(intent: UIComposeIntent): ComposeIntent {
  return intent === 'reply_all' ? 'reply' : intent;
}

export function emailOf(addr: MessageAddress): string {
  return addr.email || addr.address || '';
}

type TFn = (key: string, values?: Record<string, unknown>) => string;

export function buildQuoteHTML(intent: string, source: MessageDetail, t?: TFn): string {
  const from = source.from_name
    ? `${escapeHtml(source.from_name)} &lt;${escapeHtml(source.from_addr)}&gt;`
    : escapeHtml(source.from_addr);
  const date = new Intl.DateTimeFormat('ko-KR', {
    year: 'numeric', month: 'long', day: 'numeric', hour: '2-digit', minute: '2-digit', hour12: false,
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
