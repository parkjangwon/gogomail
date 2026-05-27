'use client';

import type { MutableRefObject, RefObject } from 'react';
import {
  ArrowUpTrayIcon, FolderPlusIcon, XMarkIcon, PauseIcon, PlayIcon,
} from '@heroicons/react/24/outline';
import { useTranslations } from 'next-intl';
import { formatBytes } from '@/lib/drive/driveUtils';
import {
  DRIVE_UPLOAD_CONCURRENCY,
  getDriveUploadSourceLabel,
  collectDroppedFiles,
  type DriveUploadBatch,
  type DriveUploadItem,
  type DriveUploadStatus,
} from './driveViewHelpers';

interface DriveUploadModalProps {
  driveUploadBatch: DriveUploadBatch | null;
  driveUploads: DriveUploadItem[];
  driveUploadResumable: boolean | null;
  driveUploadModalDismissedRef: MutableRefObject<boolean>;
  DRIVE_UPLOAD_STATUS_LABELS: Record<DriveUploadStatus, string>;
  currentParentId: string;
  fileInputRef: RefObject<HTMLInputElement | null>;
  folderInputRef: RefObject<HTMLInputElement | null>;
  onClose: () => void;
  onPause: (id: string) => void;
  onResume: (id: string) => Promise<void>;
  onCancel: (id: string) => Promise<void>;
  onDropFiles: (files: Awaited<ReturnType<typeof collectDroppedFiles>>) => Promise<void>;
}

export function DriveUploadModal({
  driveUploadBatch,
  driveUploads,
  driveUploadResumable,
  driveUploadModalDismissedRef,
  DRIVE_UPLOAD_STATUS_LABELS,
  fileInputRef,
  folderInputRef,
  onClose,
  onPause,
  onResume,
  onCancel,
  onDropFiles,
}: DriveUploadModalProps) {
  const t = useTranslations('drive');

  const activeDriveUploads = driveUploads.filter((i) => i.status === 'creating_session' || i.status === 'uploading' || i.status === 'finalizing');
  const queuedDriveUploads = driveUploads.filter((i) => i.status === 'queued');
  const completedDriveUploads = driveUploads.filter((i) => i.status === 'done' || i.status === 'canceled');
  const erroredDriveUploads = driveUploads.filter((i) => i.status === 'error');
  const totalBytes = driveUploads.reduce((s, i) => s + i.totalBytes, 0);
  const progressBytes = driveUploads.reduce((s, i) => s + Math.min(i.uploadedBytes, i.totalBytes), 0);
  const progressPct = totalBytes > 0 ? Math.min(100, Math.round((progressBytes / totalBytes) * 100)) : 0;
  const uploadBatchNames = driveUploadBatch?.files ?? [];

  return (
    <div
      data-testid="drive-upload-modal"
      role="dialog"
      aria-modal="true"
      aria-label={t('uploadModal.title')}
      onDragOver={(e) => {
        if (Array.from(e.dataTransfer.types).includes('Files')) {
          e.preventDefault();
          e.stopPropagation();
        }
      }}
      onDrop={async (e) => {
        e.preventDefault();
        e.stopPropagation();
        const files = await collectDroppedFiles(e.dataTransfer);
        if (files.length) await onDropFiles(files);
      }}
      style={{
        position: 'fixed',
        inset: 0,
        zIndex: 60,
        background: 'rgba(15, 23, 42, 0.58)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        padding: '24px',
      }}
      onMouseDown={(e) => {
        if (e.target === e.currentTarget && activeDriveUploads.length === 0) {
          driveUploadModalDismissedRef.current = true;
          onClose();
        }
      }}
    >
      <div style={{ width: 'min(1120px, 100%)', maxHeight: 'min(84vh, 920px)', display: 'flex', flexDirection: 'column', borderRadius: '10px', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-primary)', boxShadow: '0 24px 72px rgba(15, 23, 42, 0.42)', overflow: 'hidden' }}>
        {/* Header */}
        <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: '16px', padding: '18px 20px', borderBottom: '1px solid var(--color-border-subtle)' }}>
          <div style={{ minWidth: 0, flex: 1 }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '10px', flexWrap: 'wrap' }}>
              <div style={{ fontSize: '15px', fontWeight: 700, color: 'var(--color-text-primary)' }}>{t('uploadModal.title')}</div>
              <span style={{ display: 'inline-flex', alignItems: 'center', gap: '6px', padding: '3px 8px', borderRadius: '999px', background: 'var(--color-bg-secondary)', color: 'var(--color-text-secondary)', fontSize: '11px', fontWeight: 500 }}>
                {t('uploadModal.selectedCount', { count: driveUploadBatch?.fileCount ?? driveUploads.length })}
              </span>
              <span style={{ display: 'inline-flex', alignItems: 'center', gap: '6px', padding: '3px 8px', borderRadius: '999px', background: 'var(--color-accent-subtle)', color: 'var(--color-accent)', fontSize: '11px', fontWeight: 500 }}>
                {t('uploadModal.concurrent', { active: activeDriveUploads.length, max: DRIVE_UPLOAD_CONCURRENCY })}
              </span>
              <span style={{ display: 'inline-flex', alignItems: 'center', gap: '6px', padding: '3px 8px', borderRadius: '999px', background: 'rgba(34, 197, 94, 0.10)', color: '#15803d', fontSize: '11px', fontWeight: 500 }}>
                {driveUploadResumable ? t('uploadModal.resumableOn') : t('uploadModal.resumableOff')}
              </span>
            </div>
            <div style={{ marginTop: '6px', fontSize: '12px', lineHeight: 1.5, color: 'var(--color-text-tertiary)' }}>
              {driveUploadResumable ? t('uploadModal.descResumable') : t('uploadModal.descNonResumable')}
            </div>
          </div>
          <button
            type="button"
            onClick={() => { driveUploadModalDismissedRef.current = true; onClose(); }}
            title={t('uploadModal.closeWindow')}
            style={{ display: 'inline-flex', alignItems: 'center', justifyContent: 'center', width: '32px', height: '32px', borderRadius: '8px', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-primary)', color: 'var(--color-text-secondary)', flexShrink: 0 }}
          >
            <XMarkIcon style={{ width: '14px', height: '14px' }} />
          </button>
        </div>

        {/* Batch info */}
        {driveUploadBatch && (
          <div style={{ padding: '14px 20px', borderBottom: '1px solid var(--color-border-subtle)', background: 'linear-gradient(180deg, rgba(148, 163, 184, 0.06), rgba(148, 163, 184, 0.02))' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '8px', flexWrap: 'wrap', fontSize: '11px', color: 'var(--color-text-tertiary)' }}>
              <span>{getDriveUploadSourceLabel(driveUploadBatch.source, t)}</span>
              <span>{formatBytes(driveUploadBatch.totalBytes)}</span>
              <span>{t('uploadModal.filesCount', { count: driveUploadBatch.fileCount })}</span>
            </div>
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: '8px' }}>
              {uploadBatchNames.map((item, index) => (
                <span
                  key={`${driveUploadBatch.id}-${index}-${item.relativePath}-${item.size}`}
                  style={{ display: 'inline-flex', alignItems: 'center', gap: '6px', padding: '6px 10px', borderRadius: '999px', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-primary)', color: 'var(--color-text-secondary)', fontSize: '11px', maxWidth: '100%', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}
                  title={`${item.relativePath} · ${formatBytes(item.size)}`}
                >
                  <strong style={{ fontWeight: 600 }}>{item.name}</strong>
                  <span style={{ color: 'var(--color-text-tertiary)' }}>{formatBytes(item.size)}</span>
                </span>
              ))}
              {driveUploadBatch.fileCount > uploadBatchNames.length && (
                <span style={{ display: 'inline-flex', alignItems: 'center', padding: '6px 10px', borderRadius: '999px', border: '1px dashed var(--color-border-default)', color: 'var(--color-text-tertiary)', fontSize: '11px' }}>
                  {t('uploadModal.moreCount', { count: driveUploadBatch.fileCount - uploadBatchNames.length })}
                </span>
              )}
            </div>
          </div>
        )}

        {/* Progress summary */}
        <div style={{ padding: '12px 20px', borderBottom: '1px solid var(--color-border-subtle)' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '12px', marginBottom: '10px', flexWrap: 'wrap', fontSize: '11px', color: 'var(--color-text-tertiary)' }}>
            <span>{t('uploadModal.inProgress', { count: activeDriveUploads.length })}</span>
            <span>{t('uploadModal.queued', { count: queuedDriveUploads.length })}</span>
            <span>{t('uploadModal.completed', { count: completedDriveUploads.length })}</span>
            <span>{t('uploadModal.failed', { count: erroredDriveUploads.length })}</span>
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
            <div style={{ flex: 1, minWidth: 0 }}>
              <div style={{ height: '7px', borderRadius: '999px', background: 'var(--color-bg-tertiary)', overflow: 'hidden' }}>
                <div style={{ width: `${progressPct}%`, height: '100%', borderRadius: '999px', background: 'var(--color-accent)', transition: 'width 180ms ease' }} />
              </div>
            </div>
            <div style={{ fontSize: '11px', color: 'var(--color-text-tertiary)', whiteSpace: 'nowrap' }}>
              {formatBytes(progressBytes)} / {formatBytes(totalBytes)}
            </div>
          </div>
        </div>

        {/* Upload items */}
        <div style={{ overflowY: 'auto', padding: '10px 0' }}>
          {driveUploads.map((item) => {
            const progress = item.totalBytes > 0 ? Math.min(100, Math.round((item.uploadedBytes / item.totalBytes) * 100)) : 0;
            const label = DRIVE_UPLOAD_STATUS_LABELS[item.status];
            const canPause = item.status === 'creating_session' || item.status === 'uploading' || item.status === 'finalizing';
            const canResume = item.status === 'paused' || item.status === 'error';
            const canCancel = item.status !== 'done' && item.status !== 'canceled';
            return (
              <div key={item.id} style={{ padding: '10px 20px', borderBottom: '1px solid var(--color-border-subtle)' }}>
                <div style={{ display: 'flex', alignItems: 'flex-start', gap: '14px' }}>
                  <div style={{ flex: 1, minWidth: 0 }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '8px', minWidth: 0, flexWrap: 'wrap' }}>
                      <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--color-text-primary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }} title={item.relativePath}>
                        {item.relativePath}
                      </div>
                      <span style={{ display: 'inline-flex', alignItems: 'center', padding: '3px 8px', borderRadius: '999px', background: item.status === 'error' ? 'rgba(239, 68, 68, 0.12)' : item.status === 'done' ? 'rgba(34, 197, 94, 0.12)' : 'var(--color-bg-secondary)', color: item.status === 'error' ? 'var(--color-destructive)' : item.status === 'done' ? '#15803d' : 'var(--color-text-secondary)', fontSize: '11px', fontWeight: 500, flexShrink: 0 }}>
                        {label}
                      </span>
                    </div>
                    <div style={{ marginTop: '8px', height: '8px', borderRadius: '999px', background: 'var(--color-bg-tertiary)', overflow: 'hidden' }}>
                      <div style={{ width: `${progress}%`, height: '100%', borderRadius: '999px', background: item.status === 'error' ? 'var(--color-destructive)' : item.status === 'done' ? '#22c55e' : 'var(--color-accent)', transition: 'width 160ms ease' }} />
                    </div>
                    <div style={{ display: 'flex', justifyContent: 'space-between', gap: '8px', marginTop: '8px', fontSize: '11px', color: 'var(--color-text-tertiary)' }}>
                      <span>{formatBytes(item.uploadedBytes)} / {formatBytes(item.totalBytes)}</span>
                      <span>{item.resumable ? t('uploadModal.itemResumable') : t('uploadModal.itemNonResumable')}</span>
                    </div>
                    {item.error && (
                      <div style={{ marginTop: '8px', fontSize: '11px', color: 'var(--color-destructive)', lineHeight: 1.45 }}>
                        {item.error}
                      </div>
                    )}
                  </div>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '6px', flexShrink: 0 }}>
                    {canPause && (
                      <button type="button" onClick={() => onPause(item.id)} title={t('uploadModal.pause')}
                        style={{ display: 'inline-flex', alignItems: 'center', justifyContent: 'center', width: '30px', height: '30px', borderRadius: '8px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)' }}>
                        <PauseIcon style={{ width: '14px', height: '14px' }} />
                      </button>
                    )}
                    {canResume && (
                      <button type="button" onClick={() => { void onResume(item.id); }} title={t('uploadModal.resume')}
                        style={{ display: 'inline-flex', alignItems: 'center', justifyContent: 'center', width: '30px', height: '30px', borderRadius: '8px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)' }}>
                        <PlayIcon style={{ width: '14px', height: '14px' }} />
                      </button>
                    )}
                    {canCancel && (
                      <button type="button" onClick={() => { void onCancel(item.id); }} title={t('uploadModal.cancel')}
                        style={{ display: 'inline-flex', alignItems: 'center', justifyContent: 'center', width: '30px', height: '30px', borderRadius: '8px', border: '1px solid var(--color-destructive)', background: 'transparent', color: 'var(--color-destructive)' }}>
                        <XMarkIcon style={{ width: '14px', height: '14px' }} />
                      </button>
                    )}
                  </div>
                </div>
              </div>
            );
          })}
        </div>

        {/* Footer */}
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '12px', padding: '14px 20px', borderTop: '1px solid var(--color-border-subtle)', background: 'var(--color-bg-secondary)' }}>
          <div style={{ fontSize: '11px', color: 'var(--color-text-tertiary)', lineHeight: 1.5 }}>
            {t('uploadModal.footerHint')}
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px', flexShrink: 0 }}>
            <button type="button" onClick={() => fileInputRef.current?.click()}
              style={{ display: 'inline-flex', alignItems: 'center', gap: '6px', padding: '7px 12px', borderRadius: '8px', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-primary)', color: 'var(--color-text-secondary)', fontSize: '12px', fontWeight: 500 }}>
              <ArrowUpTrayIcon style={{ width: '14px', height: '14px' }} />
              {t('uploadModal.addFiles')}
            </button>
            <button type="button" onClick={() => folderInputRef.current?.click()}
              style={{ display: 'inline-flex', alignItems: 'center', gap: '6px', padding: '7px 12px', borderRadius: '8px', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-primary)', color: 'var(--color-text-secondary)', fontSize: '12px', fontWeight: 500 }}>
              <FolderPlusIcon style={{ width: '14px', height: '14px' }} />
              {t('uploadModal.addFolder')}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
