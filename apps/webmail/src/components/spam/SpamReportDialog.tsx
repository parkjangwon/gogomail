'use client';

import { useState } from 'react';
import { useTranslations } from 'next-intl';
import { XMarkIcon, ShieldExclamationIcon } from '@heroicons/react/24/outline';

interface SpamReportDialogProps {
  fromAddr: string;
  fromName?: string;
  onConfirm: (opts: { blockSender: boolean; blockDomain: boolean }) => void;
  onCancel: () => void;
}

export function SpamReportDialog({ fromAddr, fromName, onConfirm, onCancel }: SpamReportDialogProps) {
  const t = useTranslations('spamDialog');
  const [blockSender, setBlockSender] = useState(true);
  const [blockDomain, setBlockDomain] = useState(false);

  const domain = fromAddr.includes('@') ? fromAddr.split('@')[1] : '';
  const displaySender = fromName ? `${fromName} <${fromAddr}>` : fromAddr;

  function handleConfirm() {
    onConfirm({ blockSender, blockDomain: blockSender && blockDomain });
  }

  return (
    <>
      {/* Backdrop */}
      <div
        aria-hidden="true"
        onClick={onCancel}
        style={{ position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.45)', zIndex: 600 }}
      />

      {/* Dialog */}
      <div
        role="dialog"
        aria-modal="true"
        aria-label={t('ariaLabel')}
        style={{
          position: 'fixed', top: '50%', left: '50%',
          transform: 'translate(-50%, -50%)',
          zIndex: 601, width: '420px',
          background: 'var(--color-bg-primary)',
          border: '1px solid var(--color-border-subtle)',
          borderRadius: '12px',
          boxShadow: '0 20px 60px rgba(0,0,0,0.22)',
          overflow: 'hidden',
        }}
      >
        {/* Header */}
        <div style={{
          display: 'flex', alignItems: 'center', gap: '10px',
          padding: '16px 20px', borderBottom: '1px solid var(--color-border-subtle)',
        }}>
          <ShieldExclamationIcon style={{ width: 20, height: 20, color: 'var(--color-warning)', flexShrink: 0 }} />
          <span style={{ fontSize: '15px', fontWeight: 600, color: 'var(--color-text-primary)', flex: 1 }}>
            {t('title')}
          </span>
          <button
            onClick={onCancel}
            aria-label={t('cancelAria')}
            style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--color-text-tertiary)', display: 'flex', padding: '4px', borderRadius: '6px' }}
            onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
            onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
          >
            <XMarkIcon style={{ width: 18, height: 18 }} />
          </button>
        </div>

        {/* Body */}
        <div style={{ padding: '20px' }}>
          {/* Sender info */}
          <div style={{
            padding: '10px 12px', borderRadius: '8px',
            background: 'var(--color-bg-secondary)',
            border: '1px solid var(--color-border-subtle)',
            marginBottom: '16px',
          }}>
            <div style={{ fontSize: '11px', fontWeight: 600, letterSpacing: '0.06em', textTransform: 'uppercase', color: 'var(--color-text-tertiary)', marginBottom: '4px' }}>
              {t('sender')}
            </div>
            <div style={{ fontSize: '13px', color: 'var(--color-text-primary)', fontFamily: 'monospace', wordBreak: 'break-all' }}>
              {displaySender}
            </div>
          </div>

          {/* Description */}
          <p style={{ fontSize: '13px', color: 'var(--color-text-secondary)', margin: '0 0 16px', lineHeight: 1.5 }}>
            {t('description')}
          </p>

          {/* Block sender checkbox */}
          <label
            style={{ display: 'flex', alignItems: 'flex-start', gap: '10px', cursor: 'pointer', marginBottom: '10px' }}
          >
            <input
              type="checkbox"
              checked={blockSender}
              onChange={(e) => setBlockSender(e.target.checked)}
              style={{ marginTop: '2px', width: 15, height: 15, accentColor: 'var(--color-accent)', cursor: 'pointer', flexShrink: 0 }}
            />
            <div>
              <div style={{ fontSize: '13px', fontWeight: 500, color: 'var(--color-text-primary)', lineHeight: 1.4 }}>
                {t('blockSenderLabel')}
              </div>
              <div style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', marginTop: '1px' }}>
                {t('blockSenderDesc', { addr: fromAddr })}
              </div>
            </div>
          </label>

          {/* Block domain sub-option (shown when blockSender is checked) */}
          {blockSender && domain && (
            <div style={{ marginLeft: '25px', marginBottom: '4px' }}>
              <label style={{ display: 'flex', alignItems: 'flex-start', gap: '10px', cursor: 'pointer' }}>
                <input
                  type="checkbox"
                  checked={blockDomain}
                  onChange={(e) => setBlockDomain(e.target.checked)}
                  style={{ marginTop: '2px', width: 15, height: 15, accentColor: 'var(--color-accent)', cursor: 'pointer', flexShrink: 0 }}
                />
                <div>
                  <div style={{ fontSize: '13px', fontWeight: 500, color: 'var(--color-text-primary)', lineHeight: 1.4 }}>
                    {t('blockDomainLabel')}
                  </div>
                  <div style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', marginTop: '1px' }}>
                    {t('blockDomainDesc', { domain })}
                  </div>
                </div>
              </label>
            </div>
          )}
        </div>

        {/* Footer */}
        <div style={{
          display: 'flex', justifyContent: 'flex-end', gap: '8px',
          padding: '14px 20px', borderTop: '1px solid var(--color-border-subtle)',
          background: 'var(--color-bg-secondary)',
        }}>
          <button
            onClick={onCancel}
            style={{
              padding: '8px 18px', borderRadius: '7px',
              border: '1px solid var(--color-border-default)',
              background: 'transparent', color: 'var(--color-text-secondary)',
              fontSize: '13px', fontWeight: 500, cursor: 'pointer',
            }}
            onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-tertiary)'; }}
            onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
          >
            {t('cancel')}
          </button>
          <button
            onClick={handleConfirm}
            style={{
              padding: '8px 18px', borderRadius: '7px',
              border: 'none',
              background: 'var(--color-destructive)', color: '#fff',
              fontSize: '13px', fontWeight: 600, cursor: 'pointer',
            }}
            onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.opacity = '0.88'; }}
            onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.opacity = '1'; }}
          >
            {t('confirm')}
          </button>
        </div>
      </div>
    </>
  );
}
