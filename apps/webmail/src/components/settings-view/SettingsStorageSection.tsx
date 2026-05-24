'use client';

import { ArrowDownTrayIcon, CheckIcon, ExclamationCircleIcon } from '@heroicons/react/24/outline';
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

const SYSTEM_TYPE_ORDER = ['inbox', 'sent', 'drafts', 'spam', 'junk', 'trash', 'archive'];

function fmt(bytes: number): string {
  if (bytes >= 1024 * 1024 * 1024) return `${(bytes / 1024 / 1024 / 1024).toFixed(1)} GB`;
  if (bytes >= 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
  if (bytes >= 1024) return `${(bytes / 1024).toFixed(0)} KB`;
  return `${bytes} B`;
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
  const barColor = usedPct > 85 ? 'var(--color-destructive)' : usedPct > 60 ? '#f97316' : 'var(--color-accent)';

  // Sort: system folders first (by defined order), then custom folders alphabetically
  const sortedStats = [...folderStats].sort((a, b) => {
    const ai = SYSTEM_TYPE_ORDER.indexOf(a.system_type ?? '');
    const bi = SYSTEM_TYPE_ORDER.indexOf(b.system_type ?? '');
    if (ai !== -1 && bi !== -1) return ai - bi;
    if (ai !== -1) return -1;
    if (bi !== -1) return 1;
    return a.name.localeCompare(b.name);
  });

  // Translate system folder names
  const folderDisplayName = (f: FolderStats): string => {
    const st = f.system_type === 'junk' ? 'spam' : f.system_type;
    if (st && ['inbox', 'sent', 'drafts', 'spam', 'trash', 'archive'].includes(st)) {
      return t(`sidebar.system.${st}` as Parameters<typeof t>[0]);
    }
    return f.name;
  };

  const BackupBtn = ({
    folderId, folderName, format,
  }: { folderId: string; folderName: string; format: 'eml' | 'zip' }) => {
    const key = `${folderId}-${format}`;
    const state = backupStates[key] ?? { status: 'idle' };
    const label = format.toUpperCase();

    let content: React.ReactNode;
    let extraStyle: React.CSSProperties = {};

    if (state.status === 'running') {
      const pct = state.total > 0 ? Math.round((state.fetched / state.total) * 100) : null;
      content = (
        <span style={{ display: 'inline-flex', alignItems: 'center', gap: 4 }}>
          <span style={{ width: 10, height: 10, border: '2px solid currentColor', borderTopColor: 'transparent', borderRadius: '50%', display: 'inline-block', animation: 'spin 0.7s linear infinite' }} />
          {pct !== null ? `${pct}%` : '...'}
        </span>
      );
      extraStyle = { opacity: 0.7, cursor: 'default' };
    } else if (state.status === 'done') {
      content = <><CheckIcon style={{ width: 11, height: 11 }} />{t('misc.settingsStorage.done')}</>;
      extraStyle = { background: 'color-mix(in srgb, var(--color-success, #16a34a) 12%, transparent)', borderColor: 'color-mix(in srgb, var(--color-success, #16a34a) 30%, transparent)', color: 'var(--color-success, #16a34a)' };
    } else if (state.status === 'error') {
      content = <><ExclamationCircleIcon style={{ width: 11, height: 11 }} />{t('misc.settingsStorage.errorShort')}</>;
      extraStyle = { borderColor: 'var(--color-destructive)', color: 'var(--color-destructive)' };
    } else {
      content = <><ArrowDownTrayIcon style={{ width: 11, height: 11 }} />{label}</>;
    }

    return (
      <button
        onClick={() => state.status === 'idle' && onStartBackup(folderId, folderName, format)}
        disabled={state.status === 'running'}
        title={format === 'eml' ? 'EML 형식으로 백업' : 'ZIP 형식으로 백업'}
        style={{
          display: 'inline-flex', alignItems: 'center', gap: 4,
          padding: '4px 9px', borderRadius: 5,
          border: '1px solid var(--color-border-default)',
          background: 'transparent',
          color: 'var(--color-text-secondary)',
          fontSize: 11, fontWeight: 500,
          cursor: state.status === 'running' ? 'default' : 'pointer',
          whiteSpace: 'nowrap', lineHeight: 1,
          transition: 'background 0.15s, border-color 0.15s',
          ...extraStyle,
        }}
        onMouseEnter={(e) => {
          if (state.status === 'idle') (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-tertiary)';
        }}
        onMouseLeave={(e) => {
          if (state.status === 'idle') (e.currentTarget as HTMLButtonElement).style.background = 'transparent';
        }}
      >
        {content}
      </button>
    );
  };

  const COL = {
    folder: { width: '30%', padding: '9px 16px' },
    num:    { width: '10%', padding: '9px 12px', textAlign: 'right' as const },
    size:   { width: '12%', padding: '9px 12px', textAlign: 'right' as const },
    btn:    { width: '14%', padding: '9px 12px', textAlign: 'center' as const },
  };

  return (
    <>
      {/* ── Quota card ── */}
      <SectionCard>
        <SectionHeader>{t('misc.settingsStorage.quotaSection')}</SectionHeader>
        <div style={{ padding: '0 20px 20px' }}>
          {statsLoading && (
            <div style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 13, color: 'var(--color-text-tertiary)', padding: '12px 0' }}>
              <span style={{ width: 14, height: 14, border: '2px solid currentColor', borderTopColor: 'transparent', borderRadius: '50%', display: 'inline-block', animation: 'spin 0.7s linear infinite', flexShrink: 0 }} />
              {t('misc.settingsStorage.loading')}
            </div>
          )}
          {!statsLoading && folderStats.length === 0 && (
            <div style={{ padding: '8px 0' }}>
              <button
                onClick={onLoadStats}
                style={{ padding: '6px 16px', borderRadius: 6, border: 'none', background: 'var(--color-accent)', color: '#fff', fontSize: 13, cursor: 'pointer', fontWeight: 600 }}
              >
                {t('misc.settingsStorage.loadStats')}
              </button>
            </div>
          )}
          {folderStats.length > 0 && (
            <>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', marginBottom: 8 }}>
                <span style={{ fontSize: 15, fontWeight: 600, color: 'var(--color-text-primary)' }}>
                  {t('misc.settingsStorage.usedSuffix', { size: fmt(totalUsed) })}
                </span>
                <span style={{ fontSize: 12, color: 'var(--color-text-tertiary)' }}>
                  {t('misc.settingsStorage.totalOf', { size: fmt(QUOTA_BYTES) })}
                </span>
              </div>
              <div style={{ height: 8, borderRadius: 4, background: 'var(--color-bg-tertiary)', overflow: 'hidden', marginBottom: 6 }}>
                <div style={{ height: '100%', width: `${usedPct}%`, background: barColor, borderRadius: 4, transition: 'width 0.5s ease' }} />
              </div>
              <div style={{ fontSize: 11, color: usedPct > 85 ? 'var(--color-destructive)' : 'var(--color-text-tertiary)', textAlign: 'right' }}>
                {t('misc.settingsStorage.usedPercent', { pct: usedPct.toFixed(1) })}
              </div>
            </>
          )}
        </div>
      </SectionCard>

      {/* ── Per-folder table ── */}
      {(folderStats.length > 0 || statsLoading) && (
        <SectionCard>
          <SectionHeader>{t('misc.settingsStorage.perFolder')}</SectionHeader>
          {statsLoading ? (
            <div style={{ padding: '16px 20px', fontSize: 13, color: 'var(--color-text-tertiary)' }}>{t('misc.settingsStorage.loading')}</div>
          ) : (
            <div style={{ overflowX: 'auto' }}>
              <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
                <colgroup>
                  <col style={{ width: '28%' }} />
                  <col style={{ width: '10%' }} />
                  <col style={{ width: '10%' }} />
                  <col style={{ width: '10%' }} />
                  <col style={{ width: '13%' }} />
                  <col style={{ width: '14%' }} />
                  <col style={{ width: '15%' }} />
                </colgroup>
                <thead>
                  <tr style={{ borderBottom: '1px solid var(--color-border-subtle)', background: 'var(--color-bg-secondary)' }}>
                    {[
                      { label: t('misc.settingsStorage.columnFolder'), align: 'left' as const },
                      { label: t('misc.settingsStorage.columnTotal'), align: 'right' as const },
                      { label: t('misc.settingsStorage.columnUnread'), align: 'right' as const },
                      { label: t('misc.settingsStorage.columnStarred'), align: 'right' as const },
                      { label: t('misc.settingsStorage.columnSize'), align: 'right' as const },
                      { label: 'EML', align: 'center' as const },
                      { label: 'ZIP', align: 'center' as const },
                    ].map(({ label, align }) => (
                      <th key={label} style={{
                        padding: '8px 12px',
                        textAlign: align,
                        fontSize: 11,
                        fontWeight: 600,
                        color: 'var(--color-text-tertiary)',
                        textTransform: 'uppercase',
                        letterSpacing: '0.04em',
                        whiteSpace: 'nowrap',
                      }}>{label}</th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {sortedStats.map((f, i) => {
                    const displayName = folderDisplayName(f);
                    const rowBg = i % 2 === 1 ? 'var(--color-bg-secondary)' : 'transparent';
                    return (
                      <tr
                        key={f.id}
                        style={{ borderBottom: '1px solid var(--color-border-subtle)', background: rowBg }}
                        onMouseEnter={(e) => { (e.currentTarget as HTMLTableRowElement).style.background = 'color-mix(in srgb, var(--color-accent) 5%, transparent)'; }}
                        onMouseLeave={(e) => { (e.currentTarget as HTMLTableRowElement).style.background = rowBg; }}
                      >
                        <td style={{ padding: '10px 12px', color: 'var(--color-text-primary)', fontWeight: 500, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis', maxWidth: 0 }}>
                          {displayName}
                        </td>
                        <td style={{ padding: '10px 12px', textAlign: 'right', color: 'var(--color-text-secondary)', fontVariantNumeric: 'tabular-nums' }}>
                          {f.total.toLocaleString()}
                        </td>
                        <td style={{ padding: '10px 12px', textAlign: 'right', fontVariantNumeric: 'tabular-nums', color: f.unread > 0 ? 'var(--color-accent)' : 'var(--color-text-tertiary)', fontWeight: f.unread > 0 ? 600 : 400 }}>
                          {f.unread > 0 ? f.unread.toLocaleString() : '—'}
                        </td>
                        <td style={{ padding: '10px 12px', textAlign: 'right', color: f.starred > 0 ? '#f59e0b' : 'var(--color-text-tertiary)', fontVariantNumeric: 'tabular-nums' }}>
                          {f.starred > 0 ? f.starred.toLocaleString() : '—'}
                        </td>
                        <td style={{ padding: '10px 12px', textAlign: 'right', color: 'var(--color-text-secondary)', whiteSpace: 'nowrap', fontVariantNumeric: 'tabular-nums' }}>
                          {f.size_bytes > 0 ? fmt(f.size_bytes) : '—'}
                        </td>
                        <td style={{ padding: '10px 12px', textAlign: 'center' }}>
                          <BackupBtn folderId={f.id} folderName={displayName} format="eml" />
                        </td>
                        <td style={{ padding: '10px 12px', textAlign: 'center' }}>
                          <BackupBtn folderId={f.id} folderName={displayName} format="zip" />
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
                <tfoot>
                  <tr style={{ borderTop: '2px solid var(--color-border-default)', background: 'var(--color-bg-secondary)' }}>
                    <td style={{ padding: '9px 12px', fontWeight: 700, fontSize: 12, color: 'var(--color-text-primary)' }}>
                      {t('misc.settingsStorage.sum')}
                    </td>
                    <td style={{ padding: '9px 12px', textAlign: 'right', fontWeight: 700, fontVariantNumeric: 'tabular-nums' }}>
                      {folderStats.reduce((s, f) => s + f.total, 0).toLocaleString()}
                    </td>
                    <td style={{ padding: '9px 12px', textAlign: 'right', fontWeight: 700, color: 'var(--color-accent)', fontVariantNumeric: 'tabular-nums' }}>
                      {folderStats.reduce((s, f) => s + f.unread, 0) > 0 ? folderStats.reduce((s, f) => s + f.unread, 0).toLocaleString() : '—'}
                    </td>
                    <td style={{ padding: '9px 12px', textAlign: 'right', fontWeight: 700, fontVariantNumeric: 'tabular-nums' }}>
                      {folderStats.reduce((s, f) => s + f.starred, 0) > 0 ? folderStats.reduce((s, f) => s + f.starred, 0).toLocaleString() : '—'}
                    </td>
                    <td style={{ padding: '9px 12px', textAlign: 'right', fontWeight: 700, fontVariantNumeric: 'tabular-nums', whiteSpace: 'nowrap' }}>
                      {fmt(totalUsed)}
                    </td>
                    <td colSpan={2} />
                  </tr>
                </tfoot>
              </table>
            </div>
          )}
        </SectionCard>
      )}

      <style>{`@keyframes spin { to { transform: rotate(360deg); } }`}</style>
    </>
  );
}
