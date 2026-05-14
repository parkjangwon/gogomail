import type { CSSProperties } from 'react';

export const getSmartReplies = (subject: string, body: string): string[] => {
  const text = ((subject ?? '') + ' ' + (body ?? '')).toLowerCase();
  const replies: string[] = [];

  if (/언제|일정|미팅|회의|가능|schedule|meet|available|when/.test(text)) {
    replies.push('일정 확인 후 연락드리겠습니다.', '해당 시간에 가능합니다.');
  }
  if (/감사|thanks|thank you|appreciate/.test(text)) {
    replies.push('천만에요. 도움이 되었으면 합니다.');
  }
  if (/[?？]|알려|문의|질문|어떻게|어디|누가|무엇|왜/.test(text)) {
    replies.push('확인 후 답변드리겠습니다.', '네, 알겠습니다.');
  }
  if (/검토|확인|리뷰|review|check/.test(text)) {
    replies.push('검토 후 피드백 드리겠습니다.');
  }
  if (replies.length < 2) replies.push('감사합니다, 확인하겠습니다.', '알겠습니다.');
  if (replies.length < 3) replies.push('좀 더 검토 후 연락드리겠습니다.');

  return [...new Set(replies)].slice(0, 3);
};

export const readingTime = (text: string): string => {
  const words = text.trim().split(/\s+/).filter(Boolean).length;
  const mins = Math.ceil(words / 200);
  return mins <= 1 ? '약 1분' : `약 ${mins}분`;
};

export const formatFullDate = (receivedAt: string): string =>
  new Intl.DateTimeFormat('ko-KR', {
    year: 'numeric',
    month: 'long',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).format(new Date(receivedAt));

export const toolbarBtnStyleInline = (active?: boolean): CSSProperties => ({
  width: '28px',
  height: '28px',
  borderRadius: '4px',
  border: 'none',
  background: active ? 'var(--color-bg-tertiary)' : 'transparent',
  color: active ? 'var(--color-text-primary)' : 'var(--color-text-secondary)',
  cursor: 'pointer',
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  fontSize: '13px',
  fontWeight: 600,
  transition: 'background 80ms ease',
  flexShrink: 0,
});

export const escapeHtmlInline = (text: string): string => text
  .replace(/&/g, '&amp;')
  .replace(/</g, '&lt;')
  .replace(/>/g, '&gt;');

export const buildInlineQuoteHTML = (intent: string, sourceText: string): string => {
  const header = intent === 'forward'
    ? '<p><strong>---------- 전달된 메시지 ----------</strong></p>'
    : '<p><strong>--- 원본 메시지 ---</strong></p>';
  const bodyLines = (sourceText || '')
    .split('\n')
    .map((line) => `<p>${escapeHtmlInline(line) || '&nbsp;'}</p>`)
    .join('');

  return `<p></p>${header}<blockquote>${bodyLines}</blockquote>`;
};

export const backendComposeIntent = (intent: 'reply' | 'reply_all' | 'forward' | 'new'): 'reply' | 'forward' | 'new' =>
  intent === 'reply_all' ? 'reply' : intent;
