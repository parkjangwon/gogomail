'use client';

import { useTranslations } from 'next-intl';

export function ShortcutsSection() {
  const t = useTranslations('settingsModal');

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
      {[
        ['c', t('shortcuts.compose')],
        ['r', t('shortcuts.reply')],
        ['a', t('shortcuts.replyAll')],
        ['f', t('shortcuts.forward')],
        ['/', t('shortcuts.searchFocus')],
        ['[', t('shortcuts.toggleSidebar')],
        ['j / k', t('shortcuts.nextPrev')],
        ['e', t('shortcuts.archive')],
        ['Delete', t('shortcuts.delete')],
        ['m', t('shortcuts.markRead')],
        ['u', t('shortcuts.markUnread')],
        ['s', t('shortcuts.toggleStar')],
        ['Esc', t('shortcuts.close')],
      ].map(([key, desc]) => (
        <div key={desc} style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '8px 12px', borderRadius: '6px', background: 'var(--color-bg-secondary)', marginBottom: '2px' }}>
          <span style={{ fontSize: '13px', color: 'var(--color-text-secondary)' }}>{desc}</span>
          <kbd style={{ fontSize: '11px', fontFamily: 'monospace', padding: '2px 8px', background: 'var(--color-bg-primary)', border: '1px solid var(--color-border-default)', borderRadius: '4px', color: 'var(--color-text-primary)', fontWeight: 600 }}>{key}</kbd>
        </div>
      ))}
    </div>
  );
}
