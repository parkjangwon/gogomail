'use client';

import { ArrowDownTrayIcon } from '@heroicons/react/24/outline';
import { useTranslations } from 'next-intl';
import type { FolderStats } from '@/lib/api';
import { SectionCard, SectionHeader } from '@/components/settings-view/settingsViewPrimitives';

export type BackupState = {
  status: 'idle' | 'running' | 'done' | 'error';
  fetched: number;
  total: number;
  error?: string;
};

interface SettingsStorageSectionProps {
  folderStats: FolderStats[];
  statsLoading: boolean;
  backupStates: Record<string, BackupState>;
  onLoadStats: () => void;
  onStartBackup: (folderId: string, folderName: string, format: 'eml' | 'zip') => void;
}

export function SettingsStorageSection({
  folderStats,
  statsLoading,
  backupStates,
  onLoadStats,
  onStartBackup,
}: SettingsStorageSectionProps) {
  const t = useTranslations();
  const QUOTA_BYTES = 10 * 1024 * 1024 * 1024;
  const totalUsed = folderStats.reduce((s, f) => s + f.size_bytes, 0);
  const usedPct = Math.min((totalUsed / QUOTA_BYTES) * 100, 100);
  const fmt = (bytes: number) => {
    if (bytes >= 1024 * 1024 * 1024) return `${(bytes / 1024 / 1024 / 1024).toFixed(1)} GB`;
    if (bytes >= 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
    if (bytes >= 1024) return `${(bytes / 1024).toFixed(0)} KB`;
    return `${bytes} B`;
  };
  const barColor = usedPct > 85 ? '#ef4444' : usedPct > 60 ? '#f97316' : 'var(--color-accent)';

  return (
    <>
      <SectionCard>
        <SectionHeader>{t('misc.settingsStorage.quotaSection')}</SectionHeader>
        <div style={{ padding: '0 20px 20px' }}>
          {statsLoading && <div style={{ fontSize: '13px', color: 'var(--color-text-tertiary)', padding: '12px 0' }}>{t('misc.settingsStorage.loading')}</div>}
          {!statsLoading && folderStats.length === 0 && (
            <button onClick={onLoadStats} style={{ marginTop: '8px', padding: '6px 16px', borderRadius: '6px', border: 'none', background: 'var(--color-accent)', color: '#fff', fontSize: '13px', cursor: 'pointer', fontWeight: 600 }}>
              {t('misc.settingsStorage.loadStats')}
            </button>
          )}
          {folderStats.length > 0 && (
            <>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', marginBottom: '6px' }}>
                <span style={{ fontSize: '13px', color: 'var(--color-text-primary)', fontWeight: 500 }}>{t('misc.settingsStorage.usedSuffix', { size: fmt(totalUsed) })}</span>
                <span style={{ fontSize: '12px', color: 'var(--color-text-tertiary)' }}>{t('misc.settingsStorage.totalOf', { size: fmt(QUOTA_BYTES) })}</span>
              </div>
              <div style={{ height: '8px', borderRadius: '4px', background: 'var(--color-bg-tertiary)', overflow: 'hidden', marginBottom: '4px' }}>
                <div style={{ height: '100%', width: `${usedPct}%`, background: barColor, borderRadius: '4px', transition: 'width 0.5s ease' }} />
              </div>
              <div style={{ fontSize: '11px', color: usedPct > 85 ? '#ef4444' : 'var(--color-text-tertiary)', textAlign: 'right' }}>{t('misc.settingsStorage.usedPercent', { pct: usedPct.toFixed(1) })}</div>
            </>
          )}
        </div>
      </SectionCard>

      {folderStats.length > 0 && (
        <SectionCard>
          <SectionHeader>{t('misc.settingsStorage.perFolder')}</SectionHeader>
          <div style={{ overflowX: 'auto' }}>
            <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '12px' }}>
              <thead>
                <tr style={{ borderBottom: '1px solid var(--color-border-subtle)' }}>
                  {[t('misc.settingsStorage.columnFolder'), t('misc.settingsStorage.columnTotal'), t('misc.settingsStorage.columnUnread'), t('misc.settingsStorage.columnStarred'), t('misc.settingsStorage.columnSize'), t('misc.settingsStorage.columnEml'), t('misc.settingsStorage.columnZip')].map((h) => (
                    <th key={h} style={{ padding: '8px 16px', textAlign: 'left', fontSize: '11px', fontWeight: 600, color: 'var(--color-text-tertiary)', textTransform: 'uppercase', letterSpacing: '0.05em', whiteSpace: 'nowrap' }}>{h}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {folderStats.map((f) => {
                  const emlKey = `${f.id}-eml`;
                  const zipKey = `${f.id}-zip`;
                  const emlState = backupStates[emlKey] ?? { status: 'idle' };
                  const zipState = backupStates[zipKey] ?? { status: 'idle' };
                  const BtnLabel = ({ state, format }: { state: BackupState; format: 'EML' | 'ZIP' }) => {
                    if (state.status === 'running') return <>{state.total > 0 ? `${state.fetched}/${state.total}` : '...'}</>;
                    if (state.status === 'done') return <>{t('misc.settingsStorage.done')}</>;
                    if (state.status === 'error') return <>{t('misc.settingsStorage.errorShort')}</>;
                    return <>{format}</>;
                  };
                  return (
                    <tr key={f.id} style={{ borderBottom: '1px solid var(--color-border-subtle)' }}>
                      <td style={{ padding: '10px 16px', color: 'var(--color-text-primary)', fontWeight: 500 }}>{f.name}</td>
                      <td style={{ padding: '10px 16px', color: 'var(--color-text-secondary)', textAlign: 'right' }}>{f.total.toLocaleString()}</td>
                      <td style={{ padding: '10px 16px', color: f.unread > 0 ? 'var(--color-accent)' : 'var(--color-text-tertiary)', textAlign: 'right', fontWeight: f.unread > 0 ? 600 : 400 }}>{f.unread.toLocaleString()}</td>
                      <td style={{ padding: '10px 16px', color: 'var(--color-text-tertiary)', textAlign: 'right' }}>{f.starred.toLocaleString()}</td>
                      <td style={{ padding: '10px 16px', color: 'var(--color-text-secondary)', whiteSpace: 'nowrap' }}>{fmt(f.size_bytes)}</td>
                      <td style={{ padding: '10px 16px' }}>
                        <button
                          onClick={() => onStartBackup(f.id, f.name, 'eml')}
                          disabled={emlState.status === 'running'}
                          style={{ padding: '4px 10px', borderRadius: '5px', border: '1px solid var(--color-border-default)', background: emlState.status === 'done' ? 'var(--color-success-subtle, #dcfce7)' : 'transparent', color: emlState.status === 'error' ? 'var(--color-destructive)' : 'var(--color-text-secondary)', fontSize: '11px', cursor: emlState.status === 'running' ? 'default' : 'pointer', display: 'inline-flex', alignItems: 'center', gap: '4px', whiteSpace: 'nowrap' }}
                        >
                          <ArrowDownTrayIcon style={{ width: 12, height: 12 }} />
                          <BtnLabel state={emlState} format="EML" />
                        </button>
                      </td>
                      <td style={{ padding: '10px 16px' }}>
                        <button
                          onClick={() => onStartBackup(f.id, f.name, 'zip')}
                          disabled={zipState.status === 'running'}
                          style={{ padding: '4px 10px', borderRadius: '5px', border: '1px solid var(--color-border-default)', background: zipState.status === 'done' ? 'var(--color-success-subtle, #dcfce7)' : 'transparent', color: zipState.status === 'error' ? 'var(--color-destructive)' : 'var(--color-text-secondary)', fontSize: '11px', cursor: zipState.status === 'running' ? 'default' : 'pointer', display: 'inline-flex', alignItems: 'center', gap: '4px', whiteSpace: 'nowrap' }}
                        >
                          <ArrowDownTrayIcon style={{ width: 12, height: 12 }} />
                          <BtnLabel state={zipState} format="ZIP" />
                        </button>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
              <tfoot>
                <tr style={{ borderTop: '2px solid var(--color-border-default)' }}>
                  <td style={{ padding: '8px 16px', fontWeight: 600, fontSize: '12px', color: 'var(--color-text-primary)' }}>{t('misc.settingsStorage.sum')}</td>
                  <td style={{ padding: '8px 16px', textAlign: 'right', fontWeight: 600 }}>{folderStats.reduce((s, f) => s + f.total, 0).toLocaleString()}</td>
                  <td style={{ padding: '8px 16px', textAlign: 'right', fontWeight: 600, color: 'var(--color-accent)' }}>{folderStats.reduce((s, f) => s + f.unread, 0).toLocaleString()}</td>
                  <td style={{ padding: '8px 16px', textAlign: 'right', fontWeight: 600 }}>{folderStats.reduce((s, f) => s + f.starred, 0).toLocaleString()}</td>
                  <td style={{ padding: '8px 16px', fontWeight: 600 }}>{fmt(totalUsed)}</td>
                  <td colSpan={2} />
                </tr>
              </tfoot>
            </table>
          </div>
        </SectionCard>
      )}
    </>
  );
}
