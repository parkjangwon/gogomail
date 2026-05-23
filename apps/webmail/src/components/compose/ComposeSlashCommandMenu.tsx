'use client';

import { useTranslations } from 'next-intl';
import type { SlashCommand } from '@/lib/compose/slashCommands';

interface ComposeSlashCommandMenuProps {
  menu: { query: string; top: number; cursorTop: number; left: number } | null;
  commands: readonly SlashCommand[];
  selectedIndex: number;
  onSelect: (command: SlashCommand) => void;
  onHover: (index: number) => void;
}

export function ComposeSlashCommandMenu({
  menu,
  commands,
  selectedIndex,
  onSelect,
  onHover,
}: ComposeSlashCommandMenuProps) {
  const t = useTranslations('composeSlash');
  if (!menu || commands.length === 0) return null;

  return (
    <div
      style={{
        position: 'fixed',
        ...(menu.top + 320 > window.innerHeight
          ? { bottom: window.innerHeight - menu.cursorTop + 4 }
          : { top: menu.top }),
        left: Math.min(menu.left, window.innerWidth - 240),
        zIndex: 600,
        width: '232px',
        background: 'var(--color-bg-primary)',
        border: '1px solid var(--color-border-default)',
        borderRadius: '10px',
        boxShadow: '0 8px 32px rgba(0,0,0,0.18)',
        overflow: 'hidden',
      }}
    >
      <div style={{ padding: '4px 10px 2px', fontSize: '10px', fontWeight: 700, letterSpacing: '0.06em', textTransform: 'uppercase', color: 'var(--color-text-tertiary)' }}>{t('formatting')}</div>
      {commands.map((cmd, i) => (
        <div
          key={cmd.id}
          onMouseDown={(e) => { e.preventDefault(); onSelect(cmd); }}
          onMouseEnter={() => onHover(i)}
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: '10px',
            padding: '6px 10px',
            cursor: 'pointer',
            background: i === selectedIndex ? 'var(--color-bg-secondary)' : 'transparent',
          }}
        >
          <div
            style={{
              width: '28px',
              height: '28px',
              borderRadius: '6px',
              border: '1px solid var(--color-border-default)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              fontSize: '11px',
              fontWeight: 700,
              flexShrink: 0,
              fontFamily: 'monospace',
            }}
          >
            {cmd.icon}
          </div>
          <div>
            <div style={{ fontSize: '13px', fontWeight: 500, color: 'var(--color-text-primary)' }}>{cmd.label}</div>
            <div style={{ fontSize: '11px', color: 'var(--color-text-tertiary)' }}>{cmd.desc}</div>
          </div>
        </div>
      ))}
    </div>
  );
}
