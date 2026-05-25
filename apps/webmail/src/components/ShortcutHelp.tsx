'use client';

import { useTranslations } from 'next-intl';
import { XMarkIcon } from '@heroicons/react/24/outline';

type SectionDef = { id: string; items: { key: string; descId: string }[] };

const SECTIONS: SectionDef[] = [
  {
    id: 'global',
    items: [
      { key: '?', descId: 'help' },
      { key: 'Cmd+k / Ctrl+k', descId: 'spotlight' },
      { key: '[', descId: 'toggleSidebar' },
      { key: 'b', descId: 'notificationCenter' },
      { key: '`', descId: 'dm' },
    ],
  },
  {
    id: 'appSwitch',
    items: [
      { key: 'g  m', descId: 'mail' },
      { key: 'g  c', descId: 'calendar' },
      { key: 'g  a', descId: 'contacts' },
      { key: 'g  d', descId: 'drive' },
      { key: 'g  ,', descId: 'settings' },
    ],
  },
  {
    id: 'mailList',
    items: [
      { key: 's', descId: 'compose' },
      { key: 'j / k', descId: 'nextPrev' },
      { key: '↑ / ↓', descId: 'listMove' },
      { key: 'n / N', descId: 'nextPrevUnread' },
      { key: 'o', descId: 'openSelected' },
      { key: 'Space', descId: 'checkboxToggle' },
      { key: 'Home / End', descId: 'firstLast' },
      { key: 'Esc', descId: 'escCloseReader' },
    ],
  },
  {
    id: 'mailActions',
    items: [
      { key: 'r', descId: 'reply' },
      { key: 'a', descId: 'replyAll' },
      { key: 'f', descId: 'forward' },
      { key: 'e', descId: 'archive' },
      { key: 'v', descId: 'moveSpotlight' },
      { key: '#  /  Del', descId: 'delete' },
      { key: '!', descId: 'spam' },
      { key: 'm', descId: 'markRead' },
      { key: 'Shift+m', descId: 'markUnread' },
      { key: 'z', descId: 'snooze' },
      { key: 'p', descId: 'pin' },
      { key: 'i', descId: 'important' },
      { key: 'l', descId: 'labelCycle' },
    ],
  },
  {
    id: 'folderNav',
    items: [
      { key: 'g  i', descId: 'inbox' },
      { key: 'g  s', descId: 'sent' },
      { key: 'g  t', descId: 'trash' },
      { key: 'g  p', descId: 'spamFolder' },
      { key: 'g  u', descId: 'firstUnread' },
      { key: 'g  x', descId: 'important2' },
    ],
  },
  {
    id: 'mailCompose',
    items: [
      { key: 'Ctrl+Enter', descId: 'send' },
      { key: 'Ctrl+s', descId: 'saveDraft' },
      { key: 'Esc', descId: 'composeClose' },
    ],
  },
  {
    id: 'calendar',
    items: [
      { key: 'd', descId: 'calendarDay' },
      { key: 'w', descId: 'calendarWeek' },
      { key: 'm', descId: 'calendarMonth' },
      { key: 't', descId: 'calendarToday' },
      { key: '← / →', descId: 'calendarPrevNext' },
    ],
  },
  {
    id: 'contacts',
    items: [
      { key: 'j / k', descId: 'contactNextPrev' },
      { key: 'c', descId: 'contactCompose' },
      { key: 'Del / Backspace', descId: 'contactDelete' },
      { key: 'Ctrl+a / Cmd+a', descId: 'contactSelectAll' },
      { key: 'Esc', descId: 'contactClearSelection' },
    ],
  },
];

function KbdItem({ value }: { value: string }) {
  const parts = value.split('/').map((p) => p.trim());
  return (
    <span style={{ display: 'inline-flex', alignItems: 'center', gap: '4px', flexWrap: 'wrap' }}>
      {parts.map((part, i) => (
        <span key={i} style={{ display: 'inline-flex', alignItems: 'center', gap: '2px' }}>
          {i > 0 && <span style={{ color: 'var(--color-text-tertiary)', fontSize: '11px', margin: '0 1px' }}>/</span>}
          {part.split('+').map((k, j) => (
            <span key={j} style={{ display: 'inline-flex', alignItems: 'center', gap: '1px' }}>
              {j > 0 && <span style={{ color: 'var(--color-text-tertiary)', fontSize: '10px' }}>+</span>}
              <kbd style={{
                display: 'inline-block', padding: '1px 6px',
                fontSize: '11px', fontFamily: 'monospace', fontWeight: 600,
                color: 'var(--color-text-primary)',
                background: 'var(--color-bg-secondary)',
                border: '1px solid var(--color-border-default)',
                borderRadius: '4px', whiteSpace: 'nowrap',
              }}>{k}</kbd>
            </span>
          ))}
        </span>
      ))}
    </span>
  );
}

interface ShortcutHelpProps {
  onClose: () => void;
}

export function ShortcutHelp({ onClose }: ShortcutHelpProps) {
  const t = useTranslations('modals.shortcuts');
  return (
    <>
      <div aria-hidden="true" onClick={onClose}
        style={{ position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.45)', zIndex: 590 }} />
      <div
        role="dialog" aria-modal="true" aria-label={t('ariaLabel')}
        style={{
          position: 'fixed', top: '50%', left: '50%',
          transform: 'translate(-50%, -50%)',
          zIndex: 591, width: '660px', maxHeight: '82vh',
          background: 'var(--color-bg-primary)',
          border: '1px solid var(--color-border-subtle)',
          borderRadius: '12px',
          boxShadow: '0 20px 60px rgba(0,0,0,0.22)',
          display: 'flex', flexDirection: 'column', overflow: 'hidden',
        }}
      >
        {/* Header */}
        <div style={{
          display: 'flex', alignItems: 'center', justifyContent: 'space-between',
          padding: '16px 20px', borderBottom: '1px solid var(--color-border-subtle)', flexShrink: 0,
        }}>
          <div>
            <span style={{ fontSize: '15px', fontWeight: 600, color: 'var(--color-text-primary)' }}>{t('title')}</span>
            <span style={{ marginLeft: '10px', fontSize: '12px', color: 'var(--color-text-tertiary)' }}>{t('hint')}</span>
          </div>
          <button aria-label={t('closeAria')} onClick={onClose}
            style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--color-text-tertiary)', display: 'flex', padding: '4px', borderRadius: '6px' }}
            onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; (e.currentTarget).style.color = 'var(--color-text-primary)'; }}
            onMouseLeave={(e) => { (e.currentTarget).style.background = 'none'; (e.currentTarget).style.color = 'var(--color-text-tertiary)'; }}
          >
            <XMarkIcon style={{ width: '18px', height: '18px' }} />
          </button>
        </div>

        {/* Body: 2-column grid */}
        <div style={{ overflowY: 'auto', padding: '20px 24px', display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '24px 32px' }}>
          {SECTIONS.map((section) => (
            <div key={section.id}>
              <div style={{
                fontSize: '10px', fontWeight: 700, letterSpacing: '0.08em',
                textTransform: 'uppercase', color: 'var(--color-text-tertiary)',
                marginBottom: '8px', paddingBottom: '4px',
                borderBottom: '1px solid var(--color-border-subtle)',
              }}>
                {t(`sections.${section.id}`)}
              </div>
              <div style={{ display: 'flex', flexDirection: 'column', gap: '2px' }}>
                {section.items.map((item) => (
                  <div key={item.key} style={{
                    display: 'flex', alignItems: 'center', justifyContent: 'space-between',
                    padding: '4px 6px', borderRadius: '4px', gap: '8px',
                  }}
                    onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-secondary)'; }}
                    onMouseLeave={(e) => { (e.currentTarget).style.background = 'transparent'; }}
                  >
                    <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)', flex: 1, minWidth: 0 }}>{t(`items.${item.descId}`)}</span>
                    <KbdItem value={item.key} />
                  </div>
                ))}
              </div>
            </div>
          ))}
        </div>
      </div>
    </>
  );
}
