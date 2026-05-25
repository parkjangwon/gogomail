'use client';

import { useTranslations } from 'next-intl';
import {
  ArrowPathIcon,
  ExclamationTriangleIcon,
  PaperClipIcon,
  XMarkIcon,
} from '@heroicons/react/24/outline';

export type UploadedAttachment = {
  id: string;
  filename: string;
  size: number;
  uploading?: boolean;
  error?: string;
  file?: File;
};

interface ComposeAttachmentPanelProps {
  attachments: UploadedAttachment[];
  onRemove: (id: string) => void;
  onRetry: (id: string) => void;
}

export function ComposeAttachmentPanel({ attachments, onRemove, onRetry }: ComposeAttachmentPanelProps) {
  const t = useTranslations('composeFull');

  if (attachments.length === 0) return null;

  return (
    <div style={{ display: 'flex', flexWrap: 'wrap', gap: '6px', padding: '6px 16px', borderTop: '1px solid var(--color-border-subtle)' }}>
      {attachments.map((att) => {
        const kb = att.size < 1024 * 1024 ? `${Math.round(att.size / 1024)} KB` : `${(att.size / 1024 / 1024).toFixed(1)} MB`;
        return (
          <div
            key={att.id}
            style={{
              display: 'inline-flex',
              alignItems: 'center',
              gap: '4px',
              padding: '3px 8px',
              borderRadius: '12px',
              border: `1px solid ${att.error ? 'rgba(217,79,61,0.4)' : 'var(--color-border-default)'}`,
              background: 'var(--color-bg-secondary)',
              fontSize: '12px',
              color: att.error ? 'var(--color-destructive)' : 'var(--color-text-primary)',
            }}
          >
            <span style={{ display: 'inline-flex', alignItems: 'center' }}>
              {att.uploading
                ? <ArrowPathIcon style={{ width: '12px', height: '12px', animation: 'spin 1s linear infinite' }} />
                : att.error
                ? <ExclamationTriangleIcon style={{ width: '12px', height: '12px' }} />
                : <PaperClipIcon style={{ width: '12px', height: '12px' }} />}
            </span>
            <span style={{ maxWidth: '160px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
              {att.filename}
            </span>
            {!att.uploading && (
              <span style={{ color: 'var(--color-text-tertiary)' }}>{kb}</span>
            )}
            {att.error && att.file && (
              <button
                type="button"
                onClick={() => onRetry(att.id)}
                style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--color-accent)', lineHeight: 1, padding: '0 2px', fontSize: '11px', fontWeight: 600 }}
              >
                {t('retry')}
              </button>
            )}
            <button
              type="button"
              onClick={() => onRemove(att.id)}
              style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--color-text-tertiary)', lineHeight: 1, padding: '0 2px', display: 'inline-flex' }}
            >
              <XMarkIcon style={{ width: '12px', height: '12px' }} />
            </button>
          </div>
        );
      })}
    </div>
  );
}
