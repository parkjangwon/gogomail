'use client';

import { ArrowPathIcon, DocumentIcon, PaperClipIcon, PhotoIcon } from '@heroicons/react/24/outline';
import { useTranslations } from 'next-intl';
import type { Attachment } from '@/lib/api';

interface AttachmentPanelProps {
  attachments: Attachment[];
  hasAttachment: boolean;
  attachmentsLoading: boolean;
  downloadingId: string | null;
  pdfPreviewLoadingId: string | null;
  savingToDriveId: string | null;
  imagePreviews: Record<string, string>;
  onDownload: (att: Attachment) => void;
  onPdfPreview: (att: Attachment) => void;
  onSaveToDrive: (att: Attachment) => void;
  onOpenImage: (url: string, filename: string, attId: string) => void;
}

export function AttachmentPanel({
  attachments,
  hasAttachment,
  attachmentsLoading,
  downloadingId,
  pdfPreviewLoadingId,
  savingToDriveId,
  imagePreviews,
  onDownload,
  onPdfPreview,
  onSaveToDrive,
  onOpenImage,
}: AttachmentPanelProps) {
  const t = useTranslations();
  if (!hasAttachment) return null;

  return (
    <div style={{ marginBottom: '16px', maxWidth: '680px' }}>
      <div style={{ fontSize: '12px', fontWeight: 600, color: 'var(--color-text-tertiary)', letterSpacing: '0.05em', textTransform: 'uppercase', marginBottom: '8px' }}>
        {t('misc.attachment.title')} {attachments.length > 0 ? `(${attachments.length})` : ''}
      </div>
      {attachmentsLoading ? (
        <div style={{ fontSize: '13px', color: 'var(--color-text-tertiary)' }}>{t('misc.attachment.loading')}</div>
      ) : attachments.length === 0 ? (
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', padding: '8px 12px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-secondary)', fontSize: '13px', color: 'var(--color-text-secondary)' }}>
          <PaperClipIcon aria-hidden="true" style={{ width: '14px', height: '14px', flexShrink: 0 }} />
          <span>{t('misc.attachment.loadFailed')}</span>
        </div>
      ) : (
        <div style={{ display: 'flex', flexWrap: 'wrap', gap: '8px' }}>
          {attachments.map((attachment) => {
            const isImage = attachment.mime_type.startsWith('image/');
            const isPdf = attachment.mime_type === 'application/pdf';
            const icon = isImage
              ? <PhotoIcon style={{ width: '14px', height: '14px' }} />
              : isPdf
              ? <DocumentIcon style={{ width: '14px', height: '14px' }} />
              : <PaperClipIcon style={{ width: '14px', height: '14px' }} />;
            const fileSize = attachment.size < 1024 * 1024
              ? `${Math.round(attachment.size / 1024)} KB`
              : `${(attachment.size / 1024 / 1024).toFixed(1)} MB`;
            const previewUrl = isImage ? imagePreviews[attachment.id] : undefined;

            return (
              <div key={attachment.id} style={{ display: 'inline-flex', flexDirection: 'column', gap: '4px', maxWidth: '220px' }}>
                {previewUrl && (
                  <button
                    onClick={() => onOpenImage(previewUrl, attachment.filename, attachment.id)}
                    title={t('misc.attachment.previewTooltip', { filename: attachment.filename })}
                    style={{ border: '1px solid var(--color-border-default)', borderRadius: '6px', overflow: 'hidden', background: 'none', cursor: 'zoom-in', padding: 0, display: 'block' }}
                  >
                    {/* eslint-disable-next-line @next/next/no-img-element */}
                    <img src={previewUrl} alt={attachment.filename} style={{ width: '100%', height: '120px', objectFit: 'cover', display: 'block' }} />
                  </button>
                )}
                <button
                  onClick={() => onDownload(attachment)}
                  disabled={downloadingId === attachment.id}
                  title={t('misc.attachment.downloadTooltip', { filename: attachment.filename, size: fileSize })}
                  style={{
                    display: 'inline-flex',
                    alignItems: 'center',
                    gap: '6px',
                    padding: '6px 12px',
                    borderRadius: '6px',
                    border: '1px solid var(--color-border-default)',
                    background: downloadingId === attachment.id ? 'var(--color-bg-tertiary)' : 'var(--color-bg-secondary)',
                    color: 'var(--color-text-primary)',
                    fontSize: '13px',
                    cursor: downloadingId === attachment.id ? 'wait' : 'pointer',
                    maxWidth: '100%',
                    textAlign: 'left',
                  }}
                  onMouseEnter={(e) => {
                    if (downloadingId !== attachment.id) (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-tertiary)';
                  }}
                  onMouseLeave={(e) => {
                    if (downloadingId !== attachment.id) (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)';
                  }}
                >
                  <span aria-hidden="true" style={{ display: 'inline-flex', alignItems: 'center' }}>
                    {downloadingId === attachment.id ? <ArrowPathIcon style={{ width: '14px', height: '14px', animation: 'spin 1s linear infinite' }} /> : icon}
                  </span>
                  <span style={{ flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', minWidth: 0 }}>{attachment.filename}</span>
                  <span style={{ fontSize: '11px', color: 'var(--color-text-tertiary)', flexShrink: 0 }}>{fileSize}</span>
                </button>
                {isPdf && (
                  <button
                    onClick={() => onPdfPreview(attachment)}
                    disabled={pdfPreviewLoadingId === attachment.id}
                    title={t('misc.attachment.pdfPreview')}
                    style={{ display: 'inline-flex', alignItems: 'center', justifyContent: 'center', gap: '4px', padding: '4px 8px', borderRadius: '5px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', fontSize: '11px', cursor: pdfPreviewLoadingId === attachment.id ? 'wait' : 'pointer', width: '100%' }}
                    onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
                    onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
                  >
                    {pdfPreviewLoadingId === attachment.id ? <ArrowPathIcon style={{ width: '11px', height: '11px', animation: 'spin 1s linear infinite' }} /> : '👁'}
                    {pdfPreviewLoadingId === attachment.id ? t('misc.attachment.pdfLoading') : t('misc.attachment.pdfPreview')}
                  </button>
                )}
                <button
                  onClick={() => onSaveToDrive(attachment)}
                  disabled={savingToDriveId === attachment.id}
                  title={t('misc.attachment.saveToDrive')}
                  style={{ display: 'inline-flex', alignItems: 'center', justifyContent: 'center', gap: '4px', padding: '4px 8px', borderRadius: '5px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', fontSize: '11px', cursor: savingToDriveId === attachment.id ? 'wait' : 'pointer', width: '100%' }}
                  onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
                  onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
                >
                  {savingToDriveId === attachment.id ? <ArrowPathIcon style={{ width: '11px', height: '11px', animation: 'spin 1s linear infinite' }} /> : '☁'}
                  {savingToDriveId === attachment.id ? t('misc.attachment.saving') : t('misc.attachment.saveToDrive')}
                </button>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
