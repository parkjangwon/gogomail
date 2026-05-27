'use client';

import { type Dispatch, type SetStateAction } from 'react';
import { useTranslations } from 'next-intl';
import { type Editor } from '@tiptap/react';
import type { EmailTemplate } from '@/lib/compose/composeUtils';
import { XMarkIcon } from '@heroicons/react/24/outline';

interface ComposeTemplatesPanelProps {
  open: boolean;
  editor: Editor | null;
  templates: EmailTemplate[];
  templateSaveName: string;
  setTemplateSaveName: Dispatch<SetStateAction<string>>;
  showTemplateSave: boolean;
  setShowTemplateSave: Dispatch<SetStateAction<boolean>>;
  setShowTemplates: Dispatch<SetStateAction<boolean>>;
  saveTemplate: () => void;
  deleteTemplate: (id: string) => void;
  subject: string;
  setSubject: Dispatch<SetStateAction<string>>;
}

export function ComposeTemplatesPanel({
  open,
  editor,
  templates,
  templateSaveName,
  setTemplateSaveName,
  showTemplateSave,
  setShowTemplateSave,
  setShowTemplates,
  saveTemplate,
  deleteTemplate,
  subject,
  setSubject,
}: ComposeTemplatesPanelProps) {
  const t = useTranslations('composeFull');

  if (!open) return null;

  return (
    <div style={{ position: 'absolute', bottom: '100%', left: 0, marginBottom: '4px', background: 'var(--color-bg-primary)', border: '1px solid var(--color-border-default)', borderRadius: '8px', boxShadow: '0 8px 24px rgba(0,0,0,0.16)', zIndex: 400, width: '220px', overflow: 'hidden' }}>
      {templates.length === 0 && !showTemplateSave && (
        <div style={{ padding: '12px 14px', fontSize: '12px', color: 'var(--color-text-tertiary)' }}>{t('noSavedTemplates')}</div>
      )}
      {templates.map((template) => (
        <div
          key={template.id}
          style={{ position: 'relative', display: 'flex', alignItems: 'center' }}
          onMouseEnter={(e) => { (e.currentTarget as HTMLDivElement).style.background = 'var(--color-bg-secondary)'; }}
          onMouseLeave={(e) => { (e.currentTarget as HTMLDivElement).style.background = 'transparent'; }}
        >
          <button
            type="button"
            onClick={() => { editor?.chain().focus().setContent(template.body).run(); if (!subject.trim()) setSubject(template.subject); setShowTemplates(false); }}
            style={{ display: 'flex', flexDirection: 'column', alignItems: 'flex-start', flex: 1, padding: '8px 14px', border: 'none', background: 'transparent', cursor: 'pointer', textAlign: 'left', minWidth: 0 }}
          >
            <span style={{ fontSize: '13px', fontWeight: 500, color: 'var(--color-text-primary)' }}>{template.name}</span>
            {template.subject && <span style={{ fontSize: '11px', color: 'var(--color-text-tertiary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: '160px' }}>{template.subject}</span>}
          </button>
          <button
            type="button"
            onClick={(e) => { e.stopPropagation(); deleteTemplate(template.id); }}
            title={t('templateDelete')}
            style={{ background: 'none', border: 'none', cursor: 'pointer', padding: '4px 8px', color: 'var(--color-text-tertiary)', display: 'inline-flex', flexShrink: 0 }}
            onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.color = 'var(--color-destructive)'; }}
            onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.color = 'var(--color-text-tertiary)'; }}
          >
            <XMarkIcon style={{ width: '12px', height: '12px' }} />
          </button>
        </div>
      ))}
      {templates.length > 0 && <div style={{ height: '1px', background: 'var(--color-border-subtle)', margin: '2px 0' }} />}
      {!showTemplateSave ? (
        <button
          type="button"
          onClick={() => setShowTemplateSave(true)}
          style={{ display: 'flex', alignItems: 'center', gap: '6px', width: '100%', padding: '8px 14px', border: 'none', background: 'transparent', cursor: 'pointer', textAlign: 'left', fontSize: '12px', color: 'var(--color-accent)' }}
          onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
          onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
        >
          {t('saveCurrentAsTemplate')}
        </button>
      ) : (
        <div style={{ padding: '8px 14px', display: 'flex', gap: '6px' }}>
          <input
            autoFocus
            value={templateSaveName}
            onChange={(e) => setTemplateSaveName(e.target.value)}
            onKeyDown={(e) => { if (e.key === 'Enter') saveTemplate(); if (e.key === 'Escape') { setShowTemplateSave(false); setTemplateSaveName(''); } }}
            placeholder={t('templateNamePlaceholder')}
            style={{ flex: 1, padding: '4px 8px', borderRadius: '4px', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)', fontSize: '12px', outline: 'none' }}
          />
          <button
            type="button"
            onClick={saveTemplate}
            style={{ padding: '4px 10px', borderRadius: '4px', border: 'none', background: 'var(--color-accent)', color: '#fff', fontSize: '12px', cursor: 'pointer' }}
          >{t('save')}</button>
        </div>
      )}
    </div>
  );
}
