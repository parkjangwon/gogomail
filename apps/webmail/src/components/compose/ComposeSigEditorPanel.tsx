'use client';

import { type Dispatch, type SetStateAction } from 'react';
import { useTranslations } from 'next-intl';

interface ComposeSigEditorPanelProps {
  open: boolean;
  signature: string;
  setSignature: Dispatch<SetStateAction<string>>;
}

export function ComposeSigEditorPanel({ open, signature, setSignature }: ComposeSigEditorPanelProps) {
  const t = useTranslations('composeFull');

  if (!open) return null;

  return (
    <div style={{ padding: '8px 16px', borderTop: '1px solid var(--color-border-subtle)', background: 'var(--color-bg-secondary)' }}>
      <div style={{ fontSize: '11px', color: 'var(--color-text-tertiary)', marginBottom: '4px', fontWeight: 600, textTransform: 'uppercase', letterSpacing: '0.05em' }}>{t('signatureLabel')}</div>
      <textarea
        value={signature}
        onChange={(e) => setSignature(e.target.value)}
        onBlur={() => { try { localStorage.setItem('webmail_signature', signature); } catch { /* ignore */ } }}
        placeholder={t('signaturePlaceholder')}
        rows={3}
        style={{ width: '100%', padding: '6px 8px', borderRadius: '4px', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)', fontSize: '13px', resize: 'vertical', outline: 'none', boxSizing: 'border-box', fontFamily: 'inherit' }}
      />
      <div style={{ fontSize: '11px', color: 'var(--color-text-tertiary)', marginTop: '2px' }}>{t('signatureHint')}</div>
    </div>
  );
}
