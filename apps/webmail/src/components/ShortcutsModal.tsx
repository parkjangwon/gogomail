'use client';

import { useTranslations } from 'next-intl';
import { XMarkIcon } from '@heroicons/react/24/outline';

interface ShortcutsModalProps {
  onClose: () => void;
}

type SectionDef = { id: string; items: { key: string; descId: string }[] };

const SHORTCUTS: SectionDef[] = [
  {
    id: 'global',
    items: [
      { key: '?', descId: 'help' },
      { key: 'Ctrl + k', descId: 'searchFocus' },
      { key: '[', descId: 'toggleSidebar' },
    ],
  },
  {
    id: 'appSwitch',
    items: [
      { key: 'g  m', descId: 'mail' },
      { key: 'g  c', descId: 'calendar' },
      { key: 'g  a', descId: 'contacts' },
      { key: 'g  d', descId: 'drive' },
    ],
  },
  {
    id: 'mailList',
    items: [
      { key: 'c', descId: 'composeC' },
      { key: 'j / k', descId: 'nextPrev' },
      { key: '↑ / ↓', descId: 'listMove' },
      { key: 'o', descId: 'openSelected' },
      { key: 'u', descId: 'backOrClose' },
      { key: '/', descId: 'searchFocus' },
      { key: 'Esc', descId: 'escClose' },
      { key: 'Space', descId: 'checkboxToggle' },
      { key: 'Home / End', descId: 'firstLast' },
      { key: '* a', descId: 'selectAll' },
      { key: '* n', descId: 'deselectAll' },
    ],
  },
  {
    id: 'mailActions',
    items: [
      { key: 'r', descId: 'reply' },
      { key: 'a', descId: 'replyAll' },
      { key: 'f', descId: 'forward' },
      { key: 'e', descId: 'archive' },
      { key: '#', descId: 'delete' },
      { key: '!', descId: 'spam' },
      { key: 'm', descId: 'markRead' },
      { key: 'Shift + m', descId: 'markUnread' },
    ],
  },
  {
    id: 'mailCompose',
    items: [
      { key: 'Ctrl + Enter', descId: 'send' },
      { key: 'Ctrl + s', descId: 'saveDraft' },
      { key: 'Esc', descId: 'composeClose' },
    ],
  },
];

function Kbd({ children }: { children: string }) {
  return (
    <span style={{ display: 'inline-flex', alignItems: 'center', gap: '2px' }}>
      {children.split(' / ').map((part, i) => (
        <span key={i} style={{ display: 'inline-flex', alignItems: 'center', gap: '2px' }}>
          {i > 0 && <span style={{ color: 'var(--color-text-tertiary)', margin: '0 2px' }}>/</span>}
          {part.trim().split('+').map((k, j) => (
            <span key={j} style={{ display: 'inline-flex', alignItems: 'center', gap: '2px' }}>
              {j > 0 && <span style={{ color: 'var(--color-text-tertiary)', fontSize: '10px' }}>+</span>}
              <kbd style={{
                display: 'inline-block',
                padding: '2px 7px',
                fontSize: '11px',
                fontFamily: 'monospace',
                fontWeight: 600,
                lineHeight: '18px',
                color: 'var(--color-text-primary)',
                background: 'var(--color-bg-secondary)',
                border: '1px solid var(--color-border-default)',
                borderRadius: '4px',
                whiteSpace: 'nowrap',
              }}>{k.trim()}</kbd>
            </span>
          ))}
        </span>
      ))}
    </span>
  );
}

export function ShortcutsModal({ onClose }: ShortcutsModalProps) {
  const t = useTranslations('modals.shortcuts');
  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label={t('ariaLabel')}
      style={{
        position: 'fixed',
        inset: 0,
        background: 'rgba(0,0,0,0.45)',
        zIndex: 600,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        padding: '24px',
      }}
      onClick={(e) => { if (e.target === e.currentTarget) onClose(); }}
    >
      <div
        style={{
          width: '640px',
          maxHeight: '80vh',
          borderRadius: '12px',
          background: 'var(--color-bg-primary)',
          boxShadow: '0 20px 60px rgba(0,0,0,0.25)',
          display: 'flex',
          flexDirection: 'column',
          overflow: 'hidden',
        }}
      >
        {/* Header */}
        <div style={{
          display: 'flex', alignItems: 'center', justifyContent: 'space-between',
          padding: '16px 20px', borderBottom: '1px solid var(--color-border-subtle)', flexShrink: 0,
        }}>
          <div>
            <span style={{ fontSize: '16px', fontWeight: 600, color: 'var(--color-text-primary)' }}>{t('title')}</span>
            <span style={{ marginLeft: '10px', fontSize: '12px', color: 'var(--color-text-tertiary)' }}>{t('hint')}</span>
          </div>
          <button
            aria-label={t('closeAria')}
            onClick={onClose}
            style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--color-text-tertiary)', display: 'flex', padding: '4px', borderRadius: '6px' }}
            onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; (e.currentTarget).style.color = 'var(--color-text-primary)'; }}
            onMouseLeave={(e) => { (e.currentTarget).style.background = 'none'; (e.currentTarget).style.color = 'var(--color-text-tertiary)'; }}
          >
            <XMarkIcon style={{ width: '18px', height: '18px' }} />
          </button>
        </div>

        {/* Content — two-column grid */}
        <div style={{ overflowY: 'auto', padding: '20px', display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '20px' }}>
          {SHORTCUTS.map((section) => (
            <div key={section.id}>
              <div style={{ fontSize: '11px', fontWeight: 600, letterSpacing: '0.06em', textTransform: 'uppercase', color: 'var(--color-text-tertiary)', marginBottom: '8px' }}>
                {t(`sections.${section.id}`)}
              </div>
              <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
                {section.items.map((item) => (
                  <div key={item.key} style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '5px 8px', borderRadius: '5px', gap: '12px' }}
                    onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-secondary)'; }}
                    onMouseLeave={(e) => { (e.currentTarget).style.background = 'transparent'; }}
                  >
                    <span style={{ fontSize: '13px', color: 'var(--color-text-secondary)', flex: 1 }}>{t(`items.${item.descId}`)}</span>
                    <Kbd>{item.key}</Kbd>
                  </div>
                ))}
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
