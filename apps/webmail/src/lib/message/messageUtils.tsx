import { ReactNode } from 'react';
import { MessageAddress } from '@/lib/api';

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

export function emailOf(addr: MessageAddress): string {
  return addr.email || addr.address || '';
}

const URL_RE = /https?:\/\/[^\s<>"']+/g;

export function linkify(text: string): ReactNode[] {
  const parts: ReactNode[] = [];
  let last = 0;
  let match: RegExpExecArray | null;
  URL_RE.lastIndex = 0;
  while ((match = URL_RE.exec(text)) !== null) {
    if (match.index > last) parts.push(text.slice(last, match.index));
    const url = match[0];
    parts.push(
      <a key={match.index} href={url} target="_blank" rel="noopener noreferrer"
        style={{ color: 'var(--color-accent)', wordBreak: 'break-all' }}>
        {url}
      </a>
    );
    last = match.index + url.length;
  }
  if (last < text.length) parts.push(text.slice(last));
  return parts;
}
