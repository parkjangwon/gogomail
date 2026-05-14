'use client';

import { type Dispatch, type MutableRefObject, type RefObject, type SetStateAction } from 'react';
import { ArchiveBoxIcon, CalendarIcon, ChevronUpIcon } from '@heroicons/react/24/outline';
import type { SendMessageRequest } from '@/lib/api';
import { toDateTimeLocalValue } from '@/lib/dateTimeLocal';
import { type ComposeScheduleOption } from './ComposeModalActions';

interface ComposeModalFooterProps {
  sendDropdownRef: RefObject<HTMLDivElement | null>;
  showSendDropdown: boolean;
  setShowSendDropdown: Dispatch<SetStateAction<boolean>>;
  sending: boolean;
  sendButtonDisabled: boolean;
  sendButtonLabel: string;
  sendButtonUploading: boolean;
  sendResultLabel: string;
  error: string;
  sent: boolean;
  saveStatus: 'idle' | 'saving' | 'saved';
  savedAt: string;
  sendCountdown: number | null;
  pendingMsgRef: MutableRefObject<SendMessageRequest | null>;
  pendingDraftSendRef: MutableRefObject<boolean>;
  sendAndArchiveRef: MutableRefObject<boolean>;
  scheduledAt: string;
  setScheduledAt: Dispatch<SetStateAction<string>>;
  setShowSchedule: Dispatch<SetStateAction<boolean>>;
  scheduleOptions: ComposeScheduleOption[];
  handleSend: (e: { preventDefault(): void }) => void;
  closeSendDropdown: () => void;
  onArchiveSource?: () => void;
  setSendCountdown: Dispatch<SetStateAction<number | null>>;
}

export function ComposeModalFooter({
  sendDropdownRef,
  showSendDropdown,
  setShowSendDropdown,
  sending,
  sendButtonDisabled,
  sendButtonLabel,
  sendButtonUploading,
  sendResultLabel,
  error,
  sent,
  saveStatus,
  savedAt,
  sendCountdown,
  pendingMsgRef,
  pendingDraftSendRef,
  sendAndArchiveRef,
  scheduledAt,
  setScheduledAt,
  setShowSchedule,
  scheduleOptions,
  handleSend,
  closeSendDropdown,
  onArchiveSource,
  setSendCountdown,
}: ComposeModalFooterProps) {
  return (
    <>
      <div style={{
        display: 'flex',
        alignItems: 'center',
        gap: '8px',
        padding: '8px 12px',
        borderTop: '1px solid var(--color-border-subtle)',
        flexShrink: 0,
      }}>
        <div ref={sendDropdownRef} style={{ position: 'relative', display: 'flex', borderRadius: '20px', overflow: 'visible', flexShrink: 0 }}>
          <button
            type="submit"
            disabled={sendButtonDisabled}
            aria-busy={sending || sendButtonUploading}
            aria-label={sendButtonLabel}
            aria-describedby={sent && sendResultLabel ? 'compose-send-status' : undefined}
            style={{
              padding: '7px 16px',
              borderRadius: '20px 0 0 20px',
              border: 'none',
              background: sendButtonDisabled ? 'var(--color-border-default)' : 'var(--color-accent)',
              color: '#fff',
              fontSize: '13px',
              fontWeight: 500,
              cursor: sendButtonDisabled ? 'not-allowed' : 'pointer',
            }}
          >
            {sendButtonLabel}
          </button>
          <button
            type="button"
            onClick={() => setShowSendDropdown((value) => !value)}
            aria-haspopup="menu"
            aria-expanded={showSendDropdown}
            aria-controls={showSendDropdown ? 'compose-send-options-menu' : undefined}
            aria-label="전송 옵션 열기"
            style={{
              padding: '7px 10px',
              borderRadius: '0 20px 20px 0',
              border: 'none',
              borderLeft: '1px solid rgba(255,255,255,0.25)',
              background: 'var(--color-accent)',
              color: '#fff',
              cursor: 'pointer',
              display: 'flex',
              alignItems: 'center',
            }}
          >
            <ChevronUpIcon style={{ width: '14px', height: '14px' }} />
          </button>
          {showSendDropdown && (
            <div
              id="compose-send-options-menu"
              role="menu"
              onKeyDown={(e) => {
                if (e.key === 'Escape') {
                  e.stopPropagation();
                  closeSendDropdown();
                }
              }}
              style={{
                position: 'absolute',
                bottom: 'calc(100% + 8px)',
                left: 0,
                background: 'var(--color-bg-primary)',
                border: '1px solid var(--color-border-default)',
                borderRadius: '12px',
                boxShadow: '0 8px 24px rgba(0,0,0,0.16)',
                minWidth: '260px',
                overflow: 'hidden',
                zIndex: 200,
              }}
            >
              <div style={{ padding: '12px 16px 8px', fontSize: '13px', fontWeight: 600, color: 'var(--color-text-primary)' }}>예약 전송</div>
              {scheduleOptions.map((opt) => (
                <button
                  key={opt.label}
                  type="button"
                  role="menuitem"
                  aria-label={`${opt.label}, ${opt.sub}`}
                  onClick={() => {
                    setScheduledAt(toDateTimeLocalValue(opt.date));
                    closeSendDropdown();
                  }}
                  style={{ display: 'flex', alignItems: 'center', gap: '10px', width: '100%', padding: '6px 14px', border: 'none', borderBottom: '1px solid var(--color-border-subtle)', background: 'transparent', cursor: 'pointer', textAlign: 'left' }}
                  onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
                  onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
                >
                  <div style={{ width: '32px', height: '32px', borderRadius: '6px', border: '1px solid var(--color-border-default)', display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', flexShrink: 0 }}>
                    <span style={{ fontSize: '9px', color: 'var(--color-destructive)', fontWeight: 600, lineHeight: 1 }}>{new Intl.DateTimeFormat('ko-KR', { weekday: 'short' }).format(opt.date)}</span>
                    <span style={{ fontSize: '12px', fontWeight: 700, color: 'var(--color-text-primary)', lineHeight: 1.2 }}>{opt.date.getDate()}</span>
                  </div>
                  <div>
                    <div style={{ fontSize: '13px', fontWeight: 500, color: 'var(--color-text-primary)' }}>{opt.label}</div>
                    <div style={{ fontSize: '11px', color: 'var(--color-text-tertiary)' }}>{opt.sub}</div>
                  </div>
                </button>
              ))}
              {onArchiveSource && (
                <button
                  type="button"
                  role="menuitem"
                  aria-label="보내고 보관, 전송 후 원본 메일을 보관함으로 이동"
                  onClick={() => { closeSendDropdown(); sendAndArchiveRef.current = true; handleSend({ preventDefault: () => {} }); }}
                  style={{ display: 'flex', alignItems: 'center', gap: '10px', width: '100%', padding: '6px 14px', border: 'none', background: 'transparent', cursor: 'pointer', textAlign: 'left' }}
                  onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
                  onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
                >
                  <div style={{ width: '32px', height: '32px', borderRadius: '6px', border: '1px solid var(--color-border-default)', display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0 }}>
                    <ArchiveBoxIcon style={{ width: '16px', height: '16px', color: 'var(--color-accent)' }} />
                  </div>
                  <div>
                    <div style={{ fontSize: '13px', fontWeight: 500, color: 'var(--color-text-primary)' }}>보내고 보관</div>
                    <div style={{ fontSize: '11px', color: 'var(--color-text-tertiary)' }}>전송 후 원본 메일을 보관함으로 이동</div>
                  </div>
                </button>
              )}
              <button
                type="button"
                role="menuitem"
                aria-label="사용자 지정 날짜로 예약 전송"
                onClick={() => {
                  closeSendDropdown();
                  setShowSchedule(true);
                  if (!scheduledAt) {
                    setScheduledAt(toDateTimeLocalValue(new Date(Date.now() + 10 * 60000)));
                  }
                }}
                style={{ display: 'flex', alignItems: 'center', gap: '10px', width: '100%', padding: '6px 14px', border: 'none', background: 'transparent', cursor: 'pointer', textAlign: 'left' }}
                onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
                onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
              >
                <div style={{ width: '32px', height: '32px', borderRadius: '6px', border: '1px solid var(--color-border-default)', display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0 }}>
                  <CalendarIcon style={{ width: '16px', height: '16px', color: 'var(--color-accent)' }} />
                </div>
                <div style={{ fontSize: '13px', fontWeight: 500, color: 'var(--color-text-primary)' }}>사용자 지정 날짜</div>
              </button>
            </div>
          )}
        </div>

        {error && <span role="alert" style={{ fontSize: '12px', color: 'var(--color-destructive)', flex: 1 }}>{error}</span>}
        {!error && sent && sendResultLabel && <span id="compose-send-status" role="status" aria-live="polite" style={{ fontSize: '11px', color: 'var(--color-text-tertiary)' }}>{sendResultLabel}</span>}
        {!error && !sent && saveStatus === 'saving' && <span role="status" aria-live="polite" style={{ fontSize: '11px', color: 'var(--color-text-tertiary)' }}>저장 중...</span>}
        {!error && !sent && saveStatus === 'saved' && <span role="status" aria-live="polite" style={{ fontSize: '11px', color: 'var(--color-text-tertiary)' }}>임시저장 {savedAt}</span>}
        <div style={{ flex: 1 }} />
      </div>

      {sendCountdown !== null && sendCountdown > 0 && (
        <div style={{
          display: 'flex', alignItems: 'center', justifyContent: 'space-between',
          padding: '10px 16px', background: 'var(--color-accent-subtle)',
          borderTop: '1px solid var(--color-border-default)',
          fontSize: '13px', color: 'var(--color-text-primary)',
        }}>
          <span>{sendCountdown}초 후 전송됩니다...</span>
          <button
            onClick={() => { setSendCountdown(null); pendingMsgRef.current = null; pendingDraftSendRef.current = false; }}
            style={{ padding: '4px 12px', borderRadius: '5px', border: '1px solid var(--color-border-default)', background: 'transparent', cursor: 'pointer', fontSize: '13px', color: 'var(--color-text-primary)' }}
          >취소</button>
        </div>
      )}
    </>
  );
}
