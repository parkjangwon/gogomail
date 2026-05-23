import { useEffect } from 'react';
import type { CSSProperties } from 'react';
import { useTranslations } from 'next-intl';
import { EditorContent, useEditor } from '@tiptap/react';
import Placeholder from '@tiptap/extension-placeholder';
import StarterKit from '@tiptap/starter-kit';
import Underline from '@tiptap/extension-underline';
import LinkExt from '@tiptap/extension-link';

export function loadWmSettings(): Record<string, unknown> {
  try {
    return JSON.parse(localStorage.getItem('webmail_settings') ?? '{}') as Record<string, unknown>;
  } catch {
    return {};
  }
}

export function saveWmSetting(key: string, value: unknown) {
  try {
    const s = loadWmSettings();
    s[key] = value;
    localStorage.setItem('webmail_settings', JSON.stringify(s));
    window.dispatchEvent(new StorageEvent('storage', { key: 'webmail_settings', newValue: JSON.stringify(s) }));
  } catch {
    /* ignore */
  }
}

export function Toggle({ value, onChange }: { value: boolean; onChange: (v: boolean) => void }) {
  return (
    <button
      role="switch"
      aria-checked={value}
      onClick={() => onChange(!value)}
      style={{ width: '36px', height: '20px', borderRadius: '10px', background: value ? 'var(--color-accent)' : 'var(--color-bg-tertiary)', border: 'none', cursor: 'pointer', position: 'relative', flexShrink: 0, transition: 'background 150ms ease' }}
    >
      <span
        style={{
          position: 'absolute',
          top: '2px',
          left: value ? '18px' : '2px',
          width: '16px',
          height: '16px',
          borderRadius: '50%',
          background: '#fff',
          boxShadow: '0 1px 3px rgba(0,0,0,0.2)',
          transition: 'left 150ms ease',
        }}
      />
    </button>
  );
}

export function Segment<T extends string | number>({ options, value, onChange }: { options: { value: T; label: string }[]; value: T; onChange: (v: T) => void }) {
  return (
    <div style={{ display: 'inline-flex', borderRadius: '8px', border: '1px solid var(--color-border-default)', overflow: 'hidden', background: 'var(--color-bg-secondary)' }}>
      {options.map((opt, i) => (
        <button
          key={String(opt.value)}
          onClick={() => onChange(opt.value)}
          style={{
            padding: '5px 14px',
            border: 'none',
            borderLeft: i > 0 ? '1px solid var(--color-border-default)' : 'none',
            background: value === opt.value ? 'var(--color-bg-primary)' : 'transparent',
            color: value === opt.value ? 'var(--color-text-primary)' : 'var(--color-text-tertiary)',
            fontSize: '12px',
            fontWeight: value === opt.value ? 600 : 400,
            cursor: 'pointer',
            transition: 'background 100ms ease, color 100ms ease',
            boxShadow: value === opt.value ? '0 1px 3px rgba(0,0,0,0.08)' : 'none',
          }}
        >
          {opt.label}
        </button>
      ))}
    </div>
  );
}

export function SectionCard({ children }: { children: React.ReactNode }) {
  return (
    <div style={{ border: '1px solid var(--color-border-subtle)', borderRadius: '10px', overflow: 'hidden', marginBottom: '20px' }}>
      {children}
    </div>
  );
}

export function Row({ label, description, children, last }: { label: string; description?: string; children: React.ReactNode; last?: boolean }) {
  return (
    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '24px', padding: '14px 20px', borderBottom: last ? 'none' : '1px solid var(--color-border-subtle)', background: 'var(--color-bg-primary)' }}>
      <div style={{ flex: 1, minWidth: 0 }}>
        <div style={{ fontSize: '13px', fontWeight: 500, color: 'var(--color-text-primary)' }}>{label}</div>
        {description && <div style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', marginTop: '2px', lineHeight: 1.5 }}>{description}</div>}
      </div>
      <div style={{ flexShrink: 0 }}>{children}</div>
    </div>
  );
}

export function SectionHeader({ children }: { children: React.ReactNode }) {
  return (
    <div style={{ padding: '10px 20px 6px', background: 'var(--color-bg-secondary)', borderBottom: '1px solid var(--color-border-subtle)' }}>
      <span style={{ fontSize: '10px', fontWeight: 700, letterSpacing: '0.09em', textTransform: 'uppercase', color: 'var(--color-text-tertiary)' }}>{children}</span>
    </div>
  );
}

export function Kbd({ k }: { k: string }) {
  return (
    <span style={{ display: 'inline-flex', alignItems: 'center', gap: '2px', flexWrap: 'wrap' }}>
      {k.split('/').map((part, pi) => (
        <span key={pi} style={{ display: 'inline-flex', alignItems: 'center', gap: '2px' }}>
          {pi > 0 && <span style={{ color: 'var(--color-text-tertiary)', fontSize: '10px', margin: '0 2px' }}>/</span>}
          {part.trim().split('+').map((seg, si) => (
            <kbd key={si} style={{ display: 'inline-block', padding: '1px 6px', fontSize: '10px', fontFamily: 'monospace', fontWeight: 700, color: 'var(--color-text-primary)', background: 'var(--color-bg-secondary)', border: '1px solid var(--color-border-default)', borderRadius: '4px', whiteSpace: 'nowrap' }}>
              {seg.trim()}
            </kbd>
          ))}
        </span>
      ))}
    </span>
  );
}

export function MiniEditor({ value, onChange, placeholder }: { value: string; onChange: (html: string) => void; placeholder?: string }) {
  const t = useTranslations('settingsPrimitives');
  const editor = useEditor({
    extensions: [
      StarterKit,
      Underline,
      LinkExt.configure({ openOnClick: false }),
      Placeholder.configure({ placeholder: placeholder ?? '' }),
    ],
    content: value,
    immediatelyRender: false,
    onUpdate: ({ editor }) => onChange(editor.getHTML()),
  });

  useEffect(() => {
    if (editor && value !== editor.getHTML()) {
      editor.commands.setContent(value, false);
    }
  }, [value, editor]);

  const btnStyle = (active?: boolean): CSSProperties => ({
    background: active ? 'var(--color-bg-tertiary)' : 'transparent',
    border: 'none',
    cursor: 'pointer',
    padding: '3px 6px',
    borderRadius: '4px',
    fontSize: '12px',
    color: 'var(--color-text-secondary)',
    display: 'inline-flex',
    alignItems: 'center',
  });

  return (
    <div style={{ border: '1px solid var(--color-border-default)', borderRadius: '6px', overflow: 'hidden', background: 'var(--color-bg-primary)' }}>
      <style>{`
        .mini-editor .tiptap { outline: none; }
        .mini-editor .tiptap p.is-editor-empty:first-child::before {
          content: attr(data-placeholder);
          float: left;
          color: var(--color-text-tertiary);
          pointer-events: none;
          height: 0;
        }
        .mini-editor .tiptap a { color: var(--color-accent); text-decoration: underline; }
        .mini-editor .tiptap p { margin: 0 0 4px; }
        .mini-editor .tiptap ul, .mini-editor .tiptap ol { padding-left: 20px; margin: 0; }
      `}</style>
      {/* Minimal toolbar */}
      <div style={{ display: 'flex', gap: '2px', padding: '4px 6px', borderBottom: '1px solid var(--color-border-subtle)', flexWrap: 'wrap' }}>
        <button type="button" style={btnStyle(editor?.isActive('bold'))} onClick={() => editor?.chain().focus().toggleBold().run()}><b>B</b></button>
        <button type="button" style={btnStyle(editor?.isActive('italic'))} onClick={() => editor?.chain().focus().toggleItalic().run()}><i>I</i></button>
        <button type="button" style={btnStyle(editor?.isActive('underline'))} onClick={() => editor?.chain().focus().toggleUnderline().run()}><u>U</u></button>
        <span style={{ width: '1px', background: 'var(--color-border-subtle)', margin: '0 2px' }} />
        <button type="button" style={btnStyle(editor?.isActive('bulletList'))} onClick={() => editor?.chain().focus().toggleBulletList().run()}>{t('bulletList')}</button>
        <button type="button" style={btnStyle(editor?.isActive('orderedList'))} onClick={() => editor?.chain().focus().toggleOrderedList().run()}>{t('orderedList')}</button>
      </div>
      <div className="mini-editor" style={{ minHeight: '80px', padding: '6px 10px', fontSize: '13px', color: 'var(--color-text-primary)' }}>
        <EditorContent editor={editor} />
      </div>
    </div>
  );
}
