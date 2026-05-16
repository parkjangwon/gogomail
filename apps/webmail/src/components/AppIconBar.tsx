'use client';
import { EnvelopeIcon, CalendarDaysIcon, UserGroupIcon, Cog6ToothIcon, CloudIcon } from '@heroicons/react/24/outline';
import { EnvelopeIcon as EnvelopeIconSolid, CalendarDaysIcon as CalendarSolid, UserGroupIcon as UserGroupSolid, Cog6ToothIcon as Cog6ToothSolid, CloudIcon as CloudSolid } from '@heroicons/react/24/solid';

export type AppId = 'mail' | 'calendar' | 'contacts' | 'drive' | 'settings';

interface AppIconBarProps {
  activeApp: AppId;
  onChangeApp: (app: AppId) => void;
  mailUnread?: number;
}

const MAIN_APPS: { id: AppId; label: string; icon: React.ReactNode; activeIcon: React.ReactNode }[] = [
  { id: 'mail', label: '메일', icon: <EnvelopeIcon style={{ width: '20px', height: '20px' }} />, activeIcon: <EnvelopeIconSolid style={{ width: '20px', height: '20px' }} /> },
  { id: 'calendar', label: '캘린더', icon: <CalendarDaysIcon style={{ width: '20px', height: '20px' }} />, activeIcon: <CalendarSolid style={{ width: '20px', height: '20px' }} /> },
  { id: 'contacts', label: '연락처', icon: <UserGroupIcon style={{ width: '20px', height: '20px' }} />, activeIcon: <UserGroupSolid style={{ width: '20px', height: '20px' }} /> },
  { id: 'drive', label: '드라이브', icon: <CloudIcon style={{ width: '20px', height: '20px' }} />, activeIcon: <CloudSolid style={{ width: '20px', height: '20px' }} /> },
];

const BOTTOM_APPS: { id: AppId; label: string; icon: React.ReactNode; activeIcon: React.ReactNode }[] = [
  { id: 'settings', label: '설정', icon: <Cog6ToothIcon style={{ width: '20px', height: '20px' }} />, activeIcon: <Cog6ToothSolid style={{ width: '20px', height: '20px' }} /> },
];

const DEV_TOOL_SAFE_BOTTOM = process.env.NODE_ENV === 'development' ? 56 : 8;

function AppBtn({ app, isActive, onChangeApp, badge }: { app: typeof MAIN_APPS[0]; isActive: boolean; onChangeApp: (id: AppId) => void; badge?: number }) {
  const badgeLabel = badge && badge > 0 ? (badge > 99 ? '99+' : String(badge)) : '';
  return (
    <button
      aria-label={`${app.label}${badgeLabel ? ` (읽지 않음 ${badgeLabel})` : ''}`}
      aria-pressed={isActive}
      title={app.label}
      onClick={() => onChangeApp(app.id)}
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

export function AppIconBar({ activeApp, onChangeApp, mailUnread }: AppIconBarProps) {
  return (
    <div
      role="navigation"
      aria-label="앱 전환"
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
      {MAIN_APPS.map((app) => (
        <AppBtn
          key={app.id}
          app={app}
          isActive={activeApp === app.id}
          onChangeApp={onChangeApp}
          badge={app.id === 'mail' ? mailUnread : undefined}
        />
      ))}
      <div style={{ flex: 1 }} />
      {BOTTOM_APPS.map((app) => (
        <AppBtn key={app.id} app={app} isActive={activeApp === app.id} onChangeApp={onChangeApp} />
      ))}
    </div>
  );
}
