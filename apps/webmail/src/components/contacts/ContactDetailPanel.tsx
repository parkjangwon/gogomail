'use client';

import type { Dispatch, ReactNode, SetStateAction } from 'react';
import {
  PencilIcon,
  TrashIcon,
  EnvelopeIcon,
  PhoneIcon,
  BuildingOfficeIcon,
  XMarkIcon,
  CheckIcon,
} from '@heroicons/react/24/outline';
import type { ContactObject } from '@/lib/api';
import { avatarColor, initials, type ParsedContact } from './contactsViewHelpers';

interface ContactDetailPanelProps {
  selectedContact: ContactObject | null;
  selectedParsed: ParsedContact | null;
  editMode: boolean;
  editFields: ParsedContact;
  setEditFields: Dispatch<SetStateAction<ParsedContact>>;
  saving: boolean;
  saveError: string;
  onCompose?: (email: string) => void;
  onEditStart: () => void;
  onEditCancel: () => void;
  onSave: () => void;
  onDelete: () => void;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  t: (key: string, values?: Record<string, any>) => string;
}

const EDIT_FIELDS: { fieldKey: string; field: keyof ParsedContact }[] = [
  { fieldKey: 'name', field: 'fn' },
  { fieldKey: 'email', field: 'email' },
  { fieldKey: 'tel', field: 'tel' },
  { fieldKey: 'org', field: 'org' },
  { fieldKey: 'title', field: 'title' },
  { fieldKey: 'note', field: 'note' },
];

const READ_FIELDS: { fieldKey: string; field: keyof ParsedContact; icon: ReactNode }[] = [
  { fieldKey: 'email', field: 'email', icon: <EnvelopeIcon style={{ width: '14px', height: '14px' }} /> },
  { fieldKey: 'tel', field: 'tel', icon: <PhoneIcon style={{ width: '14px', height: '14px' }} /> },
  { fieldKey: 'org', field: 'org', icon: <BuildingOfficeIcon style={{ width: '14px', height: '14px' }} /> },
  { fieldKey: 'title', field: 'title', icon: null },
  { fieldKey: 'note', field: 'note', icon: null },
];

export function ContactDetailPanel({
  selectedContact,
  selectedParsed,
  editMode,
  editFields,
  setEditFields,
  saving,
  saveError,
  onCompose,
  onEditStart,
  onEditCancel,
  onSave,
  onDelete,
  t,
}: ContactDetailPanelProps) {
  if (!selectedContact || !selectedParsed) {
    return (
      <div
        style={{
          flex: 1,
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          justifyContent: 'center',
          gap: '12px',
          color: 'var(--color-text-tertiary)',
        }}
      >
        <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" style={{ opacity: 0.3 }} aria-hidden="true">
          <path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2" />
          <circle cx="9" cy="7" r="4" />
          <path d="M23 21v-2a4 4 0 0 0-3-3.87" />
          <path d="M16 3.13a4 4 0 0 1 0 7.75" />
        </svg>
        <span style={{ fontSize: '14px' }}>{t('selectAddressBook')}</span>
      </div>
    );
  }

  const displayFn = selectedParsed.fn || selectedContact.ObjectName || '';
  const avatarBg = avatarColor(displayFn);

  return (
    <div style={{ flex: 1, display: 'flex', flexDirection: 'column', overflow: 'hidden', minWidth: 0 }}>
      {/* Header */}
      <div
        style={{
          padding: '20px 24px 16px',
          borderBottom: '1px solid var(--color-border-subtle)',
          display: 'flex',
          alignItems: 'center',
          gap: '16px',
          flexShrink: 0,
        }}
      >
        <div
          style={{
            width: '56px',
            height: '56px',
            borderRadius: '50%',
            background: avatarBg,
            color: '#fff',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            fontSize: '20px',
            fontWeight: 700,
            flexShrink: 0,
          }}
        >
          {initials(displayFn || '?')}
        </div>
        <div style={{ flex: 1, minWidth: 0 }}>
          <div style={{ fontSize: '18px', fontWeight: 600, color: 'var(--color-text-primary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
            {displayFn || t('noName')}
          </div>
          {selectedParsed.org && (
            <div style={{ fontSize: '13px', color: 'var(--color-text-tertiary)', marginTop: '2px' }}>
              {selectedParsed.org}
            </div>
          )}
        </div>
        <div style={{ display: 'flex', gap: '8px', flexShrink: 0 }}>
          {selectedParsed.email && onCompose && (
            <button
              onClick={() => onCompose(selectedParsed!.email)}
              style={{ display: 'flex', alignItems: 'center', gap: '6px', padding: '7px 14px', background: 'var(--color-accent)', color: '#fff', border: 'none', borderRadius: '6px', fontSize: '13px', fontWeight: 500, cursor: 'pointer' }}
              onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.opacity = '0.85'; }}
              onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.opacity = '1'; }}
            >
              <EnvelopeIcon style={{ width: '14px', height: '14px' }} />
              {t('composeButton')}
            </button>
          )}
          {!editMode && (
            <button
              onClick={onEditStart}
              title={t('editTitle')}
              style={{ display: 'flex', alignItems: 'center', padding: '7px', background: 'var(--color-bg-secondary)', border: '1px solid var(--color-border-subtle)', borderRadius: '6px', cursor: 'pointer', color: 'var(--color-text-secondary)' }}
              onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-tertiary)'; }}
              onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
            >
              <PencilIcon style={{ width: '14px', height: '14px' }} />
            </button>
          )}
          <button
            onClick={onDelete}
            title={t('deleteTitle')}
            style={{ display: 'flex', alignItems: 'center', padding: '7px', background: 'var(--color-bg-secondary)', border: '1px solid var(--color-border-subtle)', borderRadius: '6px', cursor: 'pointer', color: '#ef4444' }}
            onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-tertiary)'; }}
            onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
          >
            <TrashIcon style={{ width: '14px', height: '14px' }} />
          </button>
        </div>
      </div>

      {/* Body */}
      <div style={{ flex: 1, overflowY: 'auto', padding: '20px 24px' }}>
        {editMode ? (
          <div style={{ display: 'flex', flexDirection: 'column', gap: '14px', maxWidth: '480px' }}>
            {EDIT_FIELDS.map(({ fieldKey, field }) => (
              <div key={field} style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
                <label style={{ fontSize: '11px', fontWeight: 600, color: 'var(--color-text-tertiary)', textTransform: 'uppercase', letterSpacing: '0.05em' }}>
                  {t(`fields.${fieldKey}`)}
                </label>
                {field === 'note' ? (
                  <textarea
                    value={editFields[field]}
                    onChange={(e) => setEditFields((prev) => ({ ...prev, [field]: e.target.value }))}
                    rows={3}
                    style={{ padding: '8px 10px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-secondary)', color: 'var(--color-text-primary)', fontSize: '13px', resize: 'vertical', outline: 'none', fontFamily: 'inherit' }}
                  />
                ) : (
                  <input
                    type="text"
                    value={editFields[field]}
                    onChange={(e) => setEditFields((prev) => ({ ...prev, [field]: e.target.value }))}
                    style={{ padding: '8px 10px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-secondary)', color: 'var(--color-text-primary)', fontSize: '13px', outline: 'none' }}
                  />
                )}
              </div>
            ))}
            {saveError && <div style={{ fontSize: '12px', color: '#e53e3e', marginTop: '4px' }}>{saveError}</div>}
            <div style={{ display: 'flex', gap: '8px', marginTop: '4px' }}>
              <button
                onClick={onEditCancel}
                style={{ display: 'flex', alignItems: 'center', gap: '4px', padding: '7px 14px', background: 'var(--color-bg-secondary)', border: '1px solid var(--color-border-default)', borderRadius: '6px', fontSize: '13px', cursor: 'pointer', color: 'var(--color-text-secondary)' }}
              >
                <XMarkIcon style={{ width: '14px', height: '14px' }} />
                {t('cancel')}
              </button>
              <button
                disabled={saving}
                onClick={onSave}
                style={{ display: 'flex', alignItems: 'center', gap: '4px', padding: '7px 14px', background: 'var(--color-accent)', border: 'none', borderRadius: '6px', fontSize: '13px', fontWeight: 500, cursor: saving ? 'wait' : 'pointer', color: '#fff', opacity: saving ? 0.7 : 1 }}
              >
                <CheckIcon style={{ width: '14px', height: '14px' }} />
                {saving ? t('saving') : t('save')}
              </button>
            </div>
          </div>
        ) : (
          <div style={{ display: 'flex', flexDirection: 'column', gap: '0' }}>
            {READ_FIELDS
              .filter((row) => selectedParsed[row.field])
              .map((row) => (
                <div
                  key={row.fieldKey}
                  style={{ display: 'flex', alignItems: 'flex-start', gap: '12px', padding: '12px 0', borderBottom: '1px solid var(--color-border-subtle)' }}
                >
                  <div style={{ width: '80px', flexShrink: 0, fontSize: '12px', fontWeight: 600, color: 'var(--color-text-tertiary)', textTransform: 'uppercase', letterSpacing: '0.04em', paddingTop: '1px', display: 'flex', alignItems: 'center', gap: '5px' }}>
                    {row.icon}
                    {t(`fields.${row.fieldKey}`)}
                  </div>
                  <div style={{ flex: 1, fontSize: '14px', color: 'var(--color-text-primary)', wordBreak: 'break-all' }}>
                    {row.fieldKey === 'email' && onCompose ? (
                      <button
                        onClick={() => onCompose(selectedParsed[row.field])}
                        style={{ background: 'none', border: 'none', padding: 0, color: 'var(--color-accent)', cursor: 'pointer', fontSize: '14px', textDecoration: 'underline' }}
                      >
                        {selectedParsed[row.field]}
                      </button>
                    ) : (
                      selectedParsed[row.field]
                    )}
                  </div>
                </div>
              ))}
          </div>
        )}
      </div>
    </div>
  );
}
