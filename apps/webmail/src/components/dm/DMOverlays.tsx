'use client';

import type { DMMessage } from '@/lib/api';
import { ArrowDownTrayIcon, ClipboardDocumentIcon, XMarkIcon } from '@heroicons/react/24/outline';

function formatBytes(size?: number): string {
  if (!size || size <= 0) return '';
  if (size < 1024) return `${size} B`;
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`;
  return `${(size / 1024 / 1024).toFixed(1)} MB`;
}

function downloadFromURL(url: string, filename: string) {
  const anchor = document.createElement('a');
  anchor.href = url;
  anchor.download = filename || 'download';
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
}

type DMOverlaysProps = {
  previewImage: DMMessage | null;
  imageMenu: { message: DMMessage; x: number; y: number } | null;
  pendingPasteFile: File | null;
  pendingPastePreview: string;
  notice: string;
  onClosePreview: () => void;
  onCopyImage: (message: DMMessage) => void;
  onSetImageMenu: (menu: { message: DMMessage; x: number; y: number } | null) => void;
  onCancelPaste: () => void;
  onConfirmPaste: () => void;
  t: (key: string, params?: Record<string, string | number>) => string;
};

export function DMOverlays({
  previewImage,
  imageMenu,
  pendingPasteFile,
  pendingPastePreview,
  notice,
  onClosePreview,
  onCopyImage,
  onSetImageMenu,
  onCancelPaste,
  onConfirmPaste,
  t,
}: DMOverlaysProps) {
  return (
    <>
      {previewImage?.attachment_download_url && (
        <div role="dialog" aria-modal="true" aria-label={previewImage.attachment_name || t('imageAttachment')} onClick={onClosePreview} style={{ position: 'absolute', inset: 0, zIndex: 140, background: 'rgba(15,23,42,0.72)', display: 'grid', placeItems: 'center', padding: 24 }}>
          <button type="button" onClick={(e) => { e.stopPropagation(); onClosePreview(); }} aria-label={t('closeImage')} style={{ position: 'absolute', top: 14, right: 14, width: 34, height: 34, border: '1px solid rgba(255,255,255,0.36)', borderRadius: 6, background: 'rgba(15,23,42,0.42)', color: '#fff', display: 'grid', placeItems: 'center', cursor: 'pointer' }}>
            <XMarkIcon style={{ width: 19, height: 19 }} />
          </button>
          <div onClick={(e) => e.stopPropagation()} style={{ position: 'absolute', left: 14, top: 14, display: 'flex', gap: 8 }}>
            <button type="button" onClick={() => downloadFromURL(previewImage.attachment_download_url!, previewImage.attachment_name || previewImage.body || 'image')} aria-label={t('downloadFile')} style={{ width: 34, height: 34, border: '1px solid rgba(255,255,255,0.36)', borderRadius: 6, background: 'rgba(15,23,42,0.42)', color: '#fff', display: 'grid', placeItems: 'center', cursor: 'pointer' }}>
              <ArrowDownTrayIcon style={{ width: 18, height: 18 }} />
            </button>
            <button type="button" onClick={() => onCopyImage(previewImage)} aria-label={t('copyImage')} style={{ width: 34, height: 34, border: '1px solid rgba(255,255,255,0.36)', borderRadius: 6, background: 'rgba(15,23,42,0.42)', color: '#fff', display: 'grid', placeItems: 'center', cursor: 'pointer' }}>
              <ClipboardDocumentIcon style={{ width: 18, height: 18 }} />
            </button>
          </div>
          <img onClick={(e) => e.stopPropagation()} src={previewImage.attachment_download_url} alt={previewImage.attachment_name || previewImage.body || t('imageAttachment')} style={{ maxWidth: 'min(92vw, 920px)', maxHeight: 'min(82vh, 760px)', objectFit: 'contain', borderRadius: 8, boxShadow: '0 24px 80px rgba(0,0,0,0.34)', background: '#fff' }} />
        </div>
      )}
      {imageMenu && (
        <div role="menu" style={{ position: 'fixed', left: imageMenu.x, top: imageMenu.y, zIndex: 180, minWidth: 138, border: '1px solid var(--color-border-default)', borderRadius: 7, background: 'var(--color-bg-primary)', boxShadow: '0 12px 30px rgba(0,0,0,0.18)', padding: 4 }}>
          <button type="button" role="menuitem" onMouseDown={(event) => event.stopPropagation()} onClick={() => onCopyImage(imageMenu.message)} style={{ width: '100%', border: 'none', borderRadius: 5, background: 'transparent', color: 'var(--color-text-primary)', padding: '7px 9px', textAlign: 'left', cursor: 'pointer', fontSize: 13 }}>{t('copyImage')}</button>
          <button type="button" role="menuitem" onMouseDown={(event) => event.stopPropagation()} onClick={() => { downloadFromURL(imageMenu.message.attachment_download_url!, imageMenu.message.attachment_name || imageMenu.message.body || 'image'); onSetImageMenu(null); }} style={{ width: '100%', border: 'none', borderRadius: 5, background: 'transparent', color: 'var(--color-text-primary)', padding: '7px 9px', textAlign: 'left', cursor: 'pointer', fontSize: 13 }}>{t('downloadFile')}</button>
        </div>
      )}
      {pendingPasteFile && (
        <div role="dialog" aria-modal="true" aria-label={t('confirmImageAttach')} onClick={onCancelPaste} style={{ position: 'absolute', inset: 0, zIndex: 150, background: 'rgba(15,23,42,0.46)', display: 'grid', placeItems: 'center', padding: 24 }}>
          <div onClick={(e) => e.stopPropagation()} style={{ width: 'min(360px, 100%)', border: '1px solid var(--color-border-default)', borderRadius: 8, background: 'var(--color-bg-primary)', boxShadow: '0 20px 50px rgba(0,0,0,0.22)', padding: 16 }}>
            <div style={{ fontSize: 15, fontWeight: 700, color: 'var(--color-text-primary)', marginBottom: 10 }}>{t('confirmImageAttach')}</div>
            {pendingPastePreview && <img src={pendingPastePreview} alt={pendingPasteFile.name || t('imageAttachment')} style={{ display: 'block', width: '100%', maxHeight: 240, objectFit: 'contain', border: '1px solid var(--color-border-subtle)', borderRadius: 7, background: 'var(--color-bg-secondary)', marginBottom: 10 }} />}
            <div style={{ fontSize: 12, color: 'var(--color-text-secondary)', marginBottom: 14 }}>{pendingPasteFile.name || t('imageAttachment')} {formatBytes(pendingPasteFile.size)}</div>
            <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 8 }}>
              <button type="button" onClick={onCancelPaste} style={{ border: '1px solid var(--color-border-default)', borderRadius: 6, background: 'transparent', color: 'var(--color-text-secondary)', padding: '7px 11px', fontSize: 13, cursor: 'pointer' }}>{t('cancel')}</button>
              <button type="button" onClick={onConfirmPaste} style={{ border: 'none', borderRadius: 6, background: 'var(--color-accent)', color: '#fff', padding: '7px 11px', fontSize: 13, fontWeight: 700, cursor: 'pointer' }}>{t('attachImage')}</button>
            </div>
          </div>
        </div>
      )}
      {notice && (
        <div role="status" style={{ position: 'absolute', left: '50%', bottom: 18, transform: 'translateX(-50%)', zIndex: 170, borderRadius: 999, background: 'rgba(15,23,42,0.88)', color: '#fff', padding: '6px 12px', fontSize: 12 }}>
          {notice}
        </div>
      )}
    </>
  );
}
