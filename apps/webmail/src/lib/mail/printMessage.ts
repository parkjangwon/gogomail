import type { MessageDetail, MessageAddress } from '@/lib/api';

type Translator = (key: string) => string;

function emailOf(a: MessageAddress): string {
  return a.email || a.address || '';
}

/**
 * Opens a new print window with the formatted message content.
 * Falls back to window.print() if a popup is blocked.
 */
export function printMessage(
  msg: MessageDetail,
  t: Translator,
): void {
  const w = window.open('', '_blank', 'width=780,height=900,menubar=yes,toolbar=yes');
  if (!w) { window.print(); return; }

  const date = new Intl.DateTimeFormat('ko-KR', {
    dateStyle: 'full',
    timeStyle: 'short',
    hour12: false,
  }).format(new Date(msg.received_at));

  const body = msg.html_body
    ? `<div>${msg.html_body}</div>`
    : (msg.text_body || '').split('\n').map((l) => `<p style="margin:0 0 4px">${l || '&nbsp;'}</p>`).join('');

  const subjectStr = msg.subject || t('misc.mailPage.noSubject');
  const fromLbl = t('mail.from');
  const toLbl = t('mail.to');
  const dateLbl = t('mail.date');

  w.document.write(
    `<!DOCTYPE html><html><head><meta charset="utf-8"><title>${subjectStr}</title>` +
    `<style>body{font-family:-apple-system,sans-serif;font-size:14px;color:#111;max-width:720px;margin:0 auto;padding:24px}` +
    `h1{font-size:20px;margin:0 0 12px}table{border-collapse:collapse;margin-bottom:16px;font-size:13px}` +
    `td{padding:3px 8px 3px 0;vertical-align:top}td:first-child{color:#555;white-space:nowrap;min-width:80px}` +
    `hr{border:none;border-top:1px solid #ddd;margin:16px 0}@media print{body{padding:0}}</style>` +
    `</head><body><h1>${subjectStr}</h1><table>` +
    `<tr><td>${fromLbl}</td><td><b>${msg.from_name ? `${msg.from_name} &lt;${msg.from_addr}&gt;` : msg.from_addr}</b></td></tr>` +
    `<tr><td>${toLbl}</td><td>${(msg.to_addrs ?? []).map((a) => a.name ? `${a.name} &lt;${emailOf(a)}&gt;` : emailOf(a)).join(', ')}</td></tr>` +
    `<tr><td>${dateLbl}</td><td>${date}</td></tr></table><hr>${body}</body></html>`,
  );
  w.document.close();
  w.onload = () => w.print();
}
