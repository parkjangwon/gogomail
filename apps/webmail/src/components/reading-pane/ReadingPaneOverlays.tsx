'use client';

import type { Attachment } from '@/lib/api';

interface ReadingPaneOverlaysProps {
  driveToast: string;
  pdfPreview: { url: string; filename: string } | null;
  setPdfPreview: (value: { url: string; filename: string } | null) => void;
  attachments: Attachment[];
  onDownloadAttachment: (attachment: Attachment) => void | Promise<void>;
  lightbox: { url: string; filename: string; attId: string } | null;
  setLightbox: (value: { url: string; filename: string; attId: string } | null) => void;
}

export function ReadingPaneOverlays({
  driveToast,
  pdfPreview,
  setPdfPreview,
  attachments,
  onDownloadAttachment,
  lightbox,
  setLightbox,
}: ReadingPaneOverlaysProps) {
  return (
    <>
      {driveToast && (
        <div
          style={{
            position: 'fixed',
            bottom: '24px',
            left: '50%',
            transform: 'translateX(-50%)',
            background: 'var(--color-text-primary)',
            color: 'var(--color-bg-primary)',
            fontSize: '13px',
            padding: '8px 18px',
            borderRadius: '20px',
            zIndex: 600,
            boxShadow: '0 4px 12px rgba(0,0,0,0.2)',
            whiteSpace: 'nowrap',
            pointerEvents: 'none',
          }}
        >
          {driveToast}
        </div>
      )}

      {pdfPreview && (
        <>
          <div
            aria-hidden="true"
            onClick={() => setPdfPreview(null)}
            style={{ position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.7)', zIndex: 500, cursor: 'pointer' }}
          />
          <div
            role="dialog"
            aria-label={pdfPreview.filename}
            aria-modal="true"
            style={{
              position: 'fixed',
              inset: '32px',
              zIndex: 501,
              display: 'flex',
              flexDirection: 'column',
              borderRadius: '10px',
              overflow: 'hidden',
              boxShadow: '0 16px 48px rgba(0,0,0,0.5)',
            }}
          >
            <div
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: '12px',
                padding: '10px 16px',
                background: 'var(--color-bg-secondary)',
                borderBottom: '1px solid var(--color-border-default)',
              }}
            >
              <span style={{ flex: 1, fontSize: '13px', fontWeight: 500, color: 'var(--color-text-primary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                {pdfPreview.filename}
              </span>
              <button
                onClick={() => {
                  const target = attachments.find((item) => item.filename === pdfPreview.filename);
                  if (target) void onDownloadAttachment(target);
                }}
                style={{ padding: '5px 14px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-primary)', fontSize: '12px', cursor: 'pointer' }}
              >
                다운로드
              </button>
              <button
                onClick={() => setPdfPreview(null)}
                aria-label="닫기"
                style={{ padding: '5px 14px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-primary)', fontSize: '12px', cursor: 'pointer' }}
              >
                닫기
              </button>
            </div>
            <iframe src={pdfPreview.url} title={pdfPreview.filename} style={{ flex: 1, border: 'none', background: '#fff' }} />
          </div>
        </>
      )}

      {lightbox && (
        <>
          <div
            aria-hidden="true"
            onClick={() => setLightbox(null)}
            style={{
              position: 'fixed',
              inset: 0,
              background: 'rgba(0,0,0,0.85)',
              zIndex: 500,
              cursor: 'zoom-out',
            }}
          />
          <div
            role="dialog"
            aria-label={lightbox.filename}
            aria-modal="true"
            style={{
              position: 'fixed',
              inset: 0,
              zIndex: 501,
              display: 'flex',
              flexDirection: 'column',
              alignItems: 'center',
              justifyContent: 'center',
              gap: '12px',
              padding: '24px',
              pointerEvents: 'none',
            }}
          >
            {/* eslint-disable-next-line @next/next/no-img-element */}
            <img
              src={lightbox.url}
              alt={lightbox.filename}
              style={{
                maxWidth: '90vw',
                maxHeight: '80vh',
                objectFit: 'contain',
                borderRadius: '6px',
                boxShadow: '0 8px 32px rgba(0,0,0,0.4)',
                pointerEvents: 'auto',
              }}
            />
            <div style={{ display: 'flex', alignItems: 'center', gap: '12px', pointerEvents: 'auto' }}>
              <span style={{ color: 'rgba(255,255,255,0.8)', fontSize: '13px', maxWidth: '300px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                {lightbox.filename}
              </span>
              <button
                onClick={() => {
                  const target = attachments.find((item) => item.id === lightbox.attId);
                  if (target) void onDownloadAttachment(target);
                }}
                style={{ padding: '6px 16px', borderRadius: '6px', border: '1px solid rgba(255,255,255,0.3)', background: 'rgba(255,255,255,0.1)', color: '#fff', fontSize: '13px', cursor: 'pointer' }}
                onMouseEnter={(event) => {
                  (event.currentTarget as HTMLButtonElement).style.background = 'rgba(255,255,255,0.2)';
                }}
                onMouseLeave={(event) => {
                  (event.currentTarget as HTMLButtonElement).style.background = 'rgba(255,255,255,0.1)';
                }}
              >
                다운로드
              </button>
            </div>
          </div>
        </>
      )}
    </>
  );
}
