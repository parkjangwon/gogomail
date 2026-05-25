'use client';
import { EnvelopeIcon, CalendarDaysIcon, UserGroupIcon, Cog6ToothIcon, CloudIcon, BookOpenIcon, ChatBubbleLeftRightIcon } from '@heroicons/react/24/outline';
import { EnvelopeIcon as EnvelopeIconSolid, CalendarDaysIcon as CalendarSolid, UserGroupIcon as UserGroupSolid, Cog6ToothIcon as Cog6ToothSolid, CloudIcon as CloudSolid, ChatBubbleLeftRightIcon as ChatBubbleSolid } from '@heroicons/react/24/solid';
import { useTranslations } from 'next-intl';
import { NotificationBell } from './notifications/NotificationBell';

export type AppId = 'mail' | 'dm' | 'calendar' | 'contacts' | 'drive' | 'settings';

interface AppIconBarProps {
  activeApp: AppId;
  onChangeApp: (app: AppId) => void;
  mailUnread?: number;
  dmUnread?: number;
}

const GUIDE_URL = process.env.NEXT_PUBLIC_WEBMAIL_GUIDE_URL?.trim() ?? '';

const DEV_TOOL_SAFE_BOTTOM = process.env.NODE_ENV === 'development' ? 56 : 8;

interface AppItem { id: AppId; label: string; icon: React.ReactNode; activeIcon: React.ReactNode }

function AppBtn({ app, isActive, onChangeApp, badge, unreadLabel }: { app: AppItem; isActive: boolean; onChangeApp: (id: AppId) => void; badge?: number; unreadLabel: string }) {
  const badgeLabel = badge && badge > 0 ? (badge > 99 ? '99+' : String(badge)) : '';
  return (
    <button
      aria-label={`${app.label}${badgeLabel ? ` (${unreadLabel} ${badgeLabel})` : ''}`}
      aria-pressed={isActive}
      title={app.label}
      onClick={() => onChangeApp(app.id)}
      onMouseDown={(e) => { e.currentTarget.blur(); }}
      style={{
        width: '36px',
        height: '36px',
        borderRadius: '8px',
        border: 'none',
        background: isActive ? 'var(--color-accent-subtle)' : 'transparent',
        color: isActive ? 'var(--color-accent)' : 'var(--color-text-tertiary)',
        cursor: 'pointer',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        transition: 'background 100ms ease, color 100ms ease',
        position: 'relative',
      }}
      onMouseEnter={(e) => {
        if (!isActive) {
          (e.currentTarget).style.background = 'var(--color-bg-tertiary)';
          (e.currentTarget).style.color = 'var(--color-text-secondary)';
        }
      }}
      onMouseLeave={(e) => {
        if (!isActive) {
          (e.currentTarget).style.background = 'transparent';
          (e.currentTarget).style.color = 'var(--color-text-tertiary)';
        }
      }}
    >
      {isActive ? app.activeIcon : app.icon}
      {badgeLabel && (
        <span style={{
          position: 'absolute',
          top: '2px',
          right: '2px',
          minWidth: '14px',
          height: '14px',
          borderRadius: '7px',
          background: 'var(--color-destructive, #dc2626)',
          color: '#fff',
          fontSize: '9px',
          fontWeight: 700,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          padding: '0 3px',
          lineHeight: 1,
          pointerEvents: 'none',
        }}>
          {badgeLabel}
        </span>
      )}
    </button>
  );
}

function AppActionBtn({ label, icon, onClick }: { label: string; icon: React.ReactNode; onClick: () => void }) {
  return (
    <button
      aria-label={label}
      title={label}
      type="button"
      onClick={onClick}
      onMouseDown={(e) => { e.currentTarget.blur(); }}
      style={{
        width: '36px',
        height: '36px',
        borderRadius: '8px',
        border: 'none',
        background: 'transparent',
        color: 'var(--color-text-tertiary)',
        cursor: 'pointer',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        transition: 'background 100ms ease, color 100ms ease',
        position: 'relative',
      }}
      onMouseEnter={(e) => {
        (e.currentTarget).style.background = 'var(--color-bg-tertiary)';
        (e.currentTarget).style.color = 'var(--color-text-secondary)';
      }}
      onMouseLeave={(e) => {
        (e.currentTarget).style.background = 'transparent';
        (e.currentTarget).style.color = 'var(--color-text-tertiary)';
      }}
    >
      {icon}
    </button>
  );
}

export function AppIconBar({ activeApp, onChangeApp, mailUnread, dmUnread }: AppIconBarProps) {
  const t = useTranslations('nav');
  const MAIN_APPS: AppItem[] = [
    { id: 'mail', label: t('mail'), icon: <EnvelopeIcon style={{ width: '20px', height: '20px' }} />, activeIcon: <EnvelopeIconSolid style={{ width: '20px', height: '20px' }} /> },
    { id: 'dm', label: t('dm'), icon: <ChatBubbleLeftRightIcon style={{ width: '20px', height: '20px' }} />, activeIcon: <ChatBubbleSolid style={{ width: '20px', height: '20px' }} /> },
    { id: 'calendar', label: t('calendar'), icon: <CalendarDaysIcon style={{ width: '20px', height: '20px' }} />, activeIcon: <CalendarSolid style={{ width: '20px', height: '20px' }} /> },
    { id: 'contacts', label: t('contacts'), icon: <UserGroupIcon style={{ width: '20px', height: '20px' }} />, activeIcon: <UserGroupSolid style={{ width: '20px', height: '20px' }} /> },
    { id: 'drive', label: t('drive'), icon: <CloudIcon style={{ width: '20px', height: '20px' }} />, activeIcon: <CloudSolid style={{ width: '20px', height: '20px' }} /> },
  ];
  const BOTTOM_APPS: AppItem[] = [
    { id: 'settings', label: t('settings'), icon: <Cog6ToothIcon style={{ width: '20px', height: '20px' }} />, activeIcon: <Cog6ToothSolid style={{ width: '20px', height: '20px' }} /> },
  ];
  const unreadLabel = t('unread');
  return (
    <div
      role="navigation"
      aria-label={t('appSwitcher')}
      style={{
        width: '44px',
        flexShrink: 0,
        height: '100%',
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        padding: `8px 0 ${DEV_TOOL_SAFE_BOTTOM}px`,
        gap: '2px',
        background: 'var(--color-bg-secondary)',
        borderRight: '1px solid var(--color-border-subtle)',
      }}
    >
      <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '2px' }}>
        {MAIN_APPS.map((app) => (
          <AppBtn
            key={app.id}
            app={app}
            isActive={activeApp === app.id}
            onChangeApp={onChangeApp}
            badge={app.id === 'mail' ? mailUnread : app.id === 'dm' ? dmUnread : undefined}
            unreadLabel={unreadLabel}
          />
        ))}
      </div>
      <div style={{ flex: 1 }} />
      <div style={{
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        gap: '2px',
        width: '100%',
        paddingTop: '8px',
        borderTop: '1px solid var(--color-border-subtle)',
      }}>
        <NotificationBell />
        {GUIDE_URL && (
          <AppActionBtn
            label={t('guide')}
            icon={<BookOpenIcon style={{ width: '20px', height: '20px' }} />}
            onClick={() => {
              if (typeof window === 'undefined') return;
              window.open(GUIDE_URL, '_blank', 'noopener,noreferrer');
            }}
          />
        )}
        {BOTTOM_APPS.map((app) => (
          <AppBtn key={app.id} app={app} isActive={activeApp === app.id} onChangeApp={onChangeApp} unreadLabel={unreadLabel} />
        ))}
      </div>
    </div>
  );
}
