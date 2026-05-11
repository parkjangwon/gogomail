'use client';
import { EnvelopeIcon, CalendarDaysIcon, UserGroupIcon, BuildingOffice2Icon, Cog6ToothIcon, CloudIcon } from '@heroicons/react/24/outline';
import { EnvelopeIcon as EnvelopeIconSolid, CalendarDaysIcon as CalendarSolid, UserGroupIcon as UserGroupSolid, BuildingOffice2Icon as BuildingOffice2Solid, Cog6ToothIcon as Cog6ToothSolid, CloudIcon as CloudSolid } from '@heroicons/react/24/solid';

export type AppId = 'mail' | 'calendar' | 'contacts' | 'orgchart' | 'drive' | 'settings';

interface AppIconBarProps {
  activeApp: AppId;
  onChangeApp: (app: AppId) => void;
}

const MAIN_APPS: { id: AppId; label: string; icon: React.ReactNode; activeIcon: React.ReactNode }[] = [
  { id: 'mail', label: '메일', icon: <EnvelopeIcon style={{ width: '20px', height: '20px' }} />, activeIcon: <EnvelopeIconSolid style={{ width: '20px', height: '20px' }} /> },
  { id: 'calendar', label: '캘린더', icon: <CalendarDaysIcon style={{ width: '20px', height: '20px' }} />, activeIcon: <CalendarSolid style={{ width: '20px', height: '20px' }} /> },
  { id: 'contacts', label: '연락처', icon: <UserGroupIcon style={{ width: '20px', height: '20px' }} />, activeIcon: <UserGroupSolid style={{ width: '20px', height: '20px' }} /> },
  { id: 'orgchart', label: '조직도', icon: <BuildingOffice2Icon style={{ width: '20px', height: '20px' }} />, activeIcon: <BuildingOffice2Solid style={{ width: '20px', height: '20px' }} /> },
  { id: 'drive', label: '드라이브', icon: <CloudIcon style={{ width: '20px', height: '20px' }} />, activeIcon: <CloudSolid style={{ width: '20px', height: '20px' }} /> },
];

const BOTTOM_APPS: { id: AppId; label: string; icon: React.ReactNode; activeIcon: React.ReactNode }[] = [
  { id: 'settings', label: '설정', icon: <Cog6ToothIcon style={{ width: '20px', height: '20px' }} />, activeIcon: <Cog6ToothSolid style={{ width: '20px', height: '20px' }} /> },
];

function AppBtn({ app, isActive, onChangeApp }: { app: typeof MAIN_APPS[0]; isActive: boolean; onChangeApp: (id: AppId) => void }) {
  return (
    <button
      aria-label={app.label}
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
    </button>
  );
}

export function AppIconBar({ activeApp, onChangeApp }: AppIconBarProps) {
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
        padding: '8px 0',
        gap: '2px',
        background: 'var(--color-bg-secondary)',
        borderRight: '1px solid var(--color-border-subtle)',
      }}
    >
      {MAIN_APPS.map((app) => (
        <AppBtn key={app.id} app={app} isActive={activeApp === app.id} onChangeApp={onChangeApp} />
      ))}
      <div style={{ flex: 1 }} />
      {BOTTOM_APPS.map((app) => (
        <AppBtn key={app.id} app={app} isActive={activeApp === app.id} onChangeApp={onChangeApp} />
      ))}
    </div>
  );
}
