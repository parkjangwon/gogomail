'use client';

import { type Dispatch, type MutableRefObject, type SetStateAction } from 'react';
import { useTranslations } from 'next-intl';
import type { Editor } from '@tiptap/react';
import type { UserAddressEntry } from '@/lib/api';
import { RecipientChips } from '../RecipientChips';
import { UsersIcon, XMarkIcon } from '@heroicons/react/24/outline';

interface ComposeFormFieldsProps {
  to: string;
  setTo: (v: string) => void;
  cc: string;
  setCc: (v: string) => void;
  bcc: string;
  setBcc: (v: string) => void;
  showCc: boolean;
  setShowCc: Dispatch<SetStateAction<boolean>>;
  showBcc: boolean;
  setShowBcc: Dispatch<SetStateAction<boolean>>;
  fromAddress: string;
  setFromAddress: (v: string) => void;
  availableAddresses: UserAddressEntry[];
  userEmail?: string;
  recentRecipients: string[];
  toRef: MutableRefObject<string>;
  ccRef: MutableRefObject<string>;
  bccRef: MutableRefObject<string>;
  subjectRef: MutableRefObject<string>;
  subject: string;
  setSubject: (v: string) => void;
  subjectInputRef: MutableRefObject<HTMLInputElement | null>;
  error: string;
  setError: (v: string) => void;
  triggerAutoSave: (to: string, cc: string, bcc: string, subject: string, bodyText: string, bodyHtml: string) => void;
  editor: Editor | null;
  setShowOrgPicker: (v: boolean) => void;
}

export function ComposeFormFields({
  to, setTo,
  cc, setCc,
  bcc, setBcc,
  showCc, setShowCc,
  showBcc, setShowBcc,
  fromAddress, setFromAddress,
  availableAddresses,
  userEmail,
  recentRecipients,
  toRef, ccRef, bccRef, subjectRef,
  subject, setSubject, subjectInputRef,
  error, setError,
  triggerAutoSave,
  editor,
  setShowOrgPicker,
}: ComposeFormFieldsProps) {
  const t = useTranslations('composeFull');

  return (
    <>
      {/* From */}
      {(userEmail || availableAddresses.length > 0) && (
        <div style={{ display: 'flex', alignItems: 'center', borderBottom: '1px solid var(--color-border-subtle)', padding: '6px 16px', gap: '8px', flexShrink: 0, background: 'var(--color-bg-secondary)' }}>
          <span style={{ fontSize: '11px', color: 'var(--color-text-tertiary)', flexShrink: 0 }}>{t('from')}</span>
          {availableAddresses.length > 1 ? (
            <select
              value={fromAddress}
              onChange={(e) => setFromAddress(e.target.value)}
              style={{ fontSize: '13px', color: 'var(--color-text-secondary)', background: 'transparent', border: 'none', outline: 'none', cursor: 'pointer', flex: 1 }}
            >
              {availableAddresses.map((a) => (
                <option key={a.id} value={a.address}>{a.address}</option>
              ))}
            </select>
          ) : (
            <span style={{ fontSize: '13px', color: 'var(--color-text-secondary)' }}>{fromAddress || userEmail}</span>
          )}
        </div>
      )}

      {/* To */}
      <div style={{ display: 'flex', alignItems: 'center', borderBottom: `1px solid ${error === t('errToRequired') ? 'var(--color-destructive)' : 'var(--color-border-subtle)'}`, padding: '0 16px', flexShrink: 0 }}>
        <label htmlFor="compose-to" style={{ fontSize: '13px', color: error === t('errToRequired') ? 'var(--color-destructive)' : 'var(--color-text-secondary)', flexShrink: 0, paddingRight: '8px' }}>{t('to')}</label>
        <RecipientChips
          id="compose-to"
          value={to}
          onChange={(v) => { setTo(v); toRef.current = v; if (error) setError(''); triggerAutoSave(v, ccRef.current, bccRef.current, subjectRef.current, editor?.getText() ?? '', editor?.getHTML() ?? ''); }}
          placeholder="example@domain.com"
          autoFocus
          hasError={error === t('errToRequired')}
          suggestions={recentRecipients}
        />
        <div style={{ display: 'flex', gap: '4px', flexShrink: 0, marginLeft: '4px' }}>
          <button type="button" onClick={() => setShowOrgPicker(true)} title={t('orgPickerTitle')}
            style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--color-text-tertiary)', padding: '2px 4px', display: 'inline-flex', flexShrink: 0 }}>
            <UsersIcon style={{ width: '15px', height: '15px' }} />
          </button>
          <button type="button"
            onClick={() => { setShowCc(v => !v); if (showCc) { setCc(''); ccRef.current = ''; } }}
            style={{ fontSize: '12px', color: showCc ? 'var(--color-text-primary)' : 'var(--color-text-tertiary)', background: 'none', border: 'none', cursor: 'pointer', padding: '2px 6px', borderRadius: '4px', fontWeight: 500 }}
            onMouseEnter={(e) => { (e.currentTarget).style.color = 'var(--color-text-primary)'; }}
            onMouseLeave={(e) => { (e.currentTarget).style.color = showCc ? 'var(--color-text-primary)' : 'var(--color-text-tertiary)'; }}
          >Cc</button>
          <button type="button"
            onClick={() => { setShowBcc(v => !v); if (showBcc) { setBcc(''); bccRef.current = ''; } }}
            style={{ fontSize: '12px', color: showBcc ? 'var(--color-text-primary)' : 'var(--color-text-tertiary)', background: 'none', border: 'none', cursor: 'pointer', padding: '2px 6px', borderRadius: '4px', fontWeight: 500 }}
            onMouseEnter={(e) => { (e.currentTarget).style.color = 'var(--color-text-primary)'; }}
            onMouseLeave={(e) => { (e.currentTarget).style.color = showBcc ? 'var(--color-text-primary)' : 'var(--color-text-tertiary)'; }}
          >Bcc</button>
        </div>
      </div>

      {/* CC — only when toggled */}
      {showCc && (
        <div style={{ display: 'flex', alignItems: 'center', borderBottom: '1px solid var(--color-border-subtle)', padding: '0 16px', flexShrink: 0 }}>
          <label htmlFor="compose-cc" style={{ fontSize: '13px', color: 'var(--color-text-secondary)', flexShrink: 0, paddingRight: '8px' }}>Cc</label>
          <RecipientChips
            id="compose-cc"
            value={cc}
            onChange={(v) => { setCc(v); ccRef.current = v; triggerAutoSave(toRef.current, v, bccRef.current, subjectRef.current, editor?.getText() ?? '', editor?.getHTML() ?? ''); }}
            placeholder="example@domain.com, ..."
            suggestions={recentRecipients}
          />
          <button type="button" onClick={() => setShowOrgPicker(true)} title={t('orgPickerTitle')}
            style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--color-text-tertiary)', padding: '2px 4px', display: 'inline-flex', flexShrink: 0 }}>
            <UsersIcon style={{ width: '15px', height: '15px' }} />
          </button>
          <button type="button" onClick={() => { setShowCc(false); setCc(''); ccRef.current = ''; }} style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--color-text-tertiary)', padding: '2px 4px', display: 'inline-flex', flexShrink: 0 }}><XMarkIcon style={{ width: '13px', height: '13px' }} /></button>
        </div>
      )}

      {/* BCC — only when toggled */}
      {showBcc && (
        <div style={{ display: 'flex', alignItems: 'center', borderBottom: '1px solid var(--color-border-subtle)', padding: '0 16px', flexShrink: 0 }}>
          <label htmlFor="compose-bcc" style={{ fontSize: '13px', color: 'var(--color-text-secondary)', flexShrink: 0, paddingRight: '8px' }}>Bcc</label>
          <RecipientChips
            id="compose-bcc"
            value={bcc}
            onChange={(v) => { setBcc(v); bccRef.current = v; triggerAutoSave(toRef.current, ccRef.current, v, subjectRef.current, editor?.getText() ?? '', editor?.getHTML() ?? ''); }}
            placeholder="example@domain.com, ..."
            suggestions={recentRecipients}
          />
          <button type="button" onClick={() => setShowOrgPicker(true)} title={t('orgPickerTitle')}
            style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--color-text-tertiary)', padding: '2px 4px', display: 'inline-flex', flexShrink: 0 }}>
            <UsersIcon style={{ width: '15px', height: '15px' }} />
          </button>
          <button type="button" onClick={() => { setShowBcc(false); setBcc(''); bccRef.current = ''; }} style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--color-text-tertiary)', padding: '2px 4px', display: 'inline-flex', flexShrink: 0 }}><XMarkIcon style={{ width: '13px', height: '13px' }} /></button>
        </div>
      )}

      {/* Subject */}
      <div style={{ display: 'flex', alignItems: 'center', borderBottom: '1px solid var(--color-border-subtle)', padding: '0 16px', flexShrink: 0 }}>
        <input
          ref={subjectInputRef}
          id="compose-subject"
          type="text"
          value={subject}
          onChange={(e) => { setSubject(e.target.value); subjectRef.current = e.target.value; triggerAutoSave(toRef.current, ccRef.current, bccRef.current, e.target.value, editor?.getText() ?? '', editor?.getHTML() ?? ''); }}
          placeholder={t('subjectPlaceholder')}
          style={{ flex: 1, padding: '10px 0', border: 'none', outline: 'none', fontSize: '14px', background: 'transparent', color: 'var(--color-text-primary)', fontWeight: 500 }}
        />
      </div>
    </>
  );
}
