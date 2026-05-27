'use client';

import { type Dispatch, type RefObject, type SetStateAction } from 'react';
import { useTranslations } from 'next-intl';
import { type Editor } from '@tiptap/react';
import type { DriveNode } from '@/lib/api';
import type { EmailTemplate } from '@/lib/compose/composeUtils';
import {
  CloudIcon,
  DocumentTextIcon,
  FaceSmileIcon,
  PaperClipIcon,
  PencilSquareIcon as PencilSquareIconHero,
  PhotoIcon,
} from '@heroicons/react/24/outline';
import { ComposeEditorToolbar } from './compose/ComposeEditorToolbar';
import { toolbarBtnStyle } from './compose/toolbarBtnStyle';
import { ComposeDrivePickerPanel } from './compose/ComposeDrivePickerPanel';
import { ComposeTemplatesPanel } from './compose/ComposeTemplatesPanel';
import { ComposeSchedulePanel } from './compose/ComposeSchedulePanel';

export interface ComposeScheduleOption {
  label: string;
  sub: string;
  date: Date;
}

interface ComposeModalActionsProps {
  editor: Editor | null;
  fileInputRef: RefObject<HTMLInputElement | null>;
  imageInputRef: RefObject<HTMLInputElement | null>;
  handleFileSelect: (files: FileList) => void;
  handleImageFileSelect: (file: File) => void | Promise<void>;
  showEmojiPicker: boolean;
  setShowEmojiPicker: Dispatch<SetStateAction<boolean>>;
  showDrivePicker: boolean;
  setShowDrivePicker: Dispatch<SetStateAction<boolean>>;
  drivePickerNodes: DriveNode[];
  drivePickerLoading: boolean;
  drivePickerCrumbs: Array<{ id: string | undefined; name: string }>;
  attachingDriveId: string | null;
  openDrivePicker: (parentId?: string, crumbs?: Array<{ id: string | undefined; name: string }>) => void | Promise<void>;
  handleAttachFromDrive: (node: DriveNode) => void | Promise<void>;
  showTemplates: boolean;
  setShowTemplates: Dispatch<SetStateAction<boolean>>;
  showTemplateSave: boolean;
  setShowTemplateSave: Dispatch<SetStateAction<boolean>>;
  templates: EmailTemplate[];
  templateSaveName: string;
  setTemplateSaveName: Dispatch<SetStateAction<string>>;
  saveTemplate: () => void;
  deleteTemplate: (id: string) => void;
  subject: string;
  setSubject: Dispatch<SetStateAction<string>>;
  showSigEditor: boolean;
  setShowSigEditor: Dispatch<SetStateAction<boolean>>;
  trackOpens: boolean;
  setTrackOpens: Dispatch<SetStateAction<boolean>>;
  showSchedule: boolean;
  setShowSchedule: Dispatch<SetStateAction<boolean>>;
  scheduledAt: string;
  setScheduledAt: Dispatch<SetStateAction<string>>;
  scheduleMinDateTime: string;
  scheduleOptions: ComposeScheduleOption[];
  imageResizeToolbar: { top: number; left: number } | null;
}


const EMOJI_GROUPS = [
  { labelKey: 'emojiFrequent', emojis: ['😀', '😂', '🥰', '😍', '🤔', '😮', '😢', '😎', '🙏', '👍', '👎', '❤️', '🎉', '✨', '🔥', '💯', '😁', '🤣', '😇', '🥳'] },
  { labelKey: 'emojiNature', emojis: ['🐶', '🐱', '🐭', '🐹', '🐰', '🦊', '🐻', '🐼', '🐨', '🐯', '🦁', '🐮', '🌸', '🌺', '🍀', '🌈', '⭐', '🌙', '☀️', '❄️'] },
  { labelKey: 'emojiFood', emojis: ['🍕', '🍔', '🌮', '🍜', '🍣', '🍰', '☕', '🍺', '🎂', '🍎', '🥑', '🍓', '🍦', '🧁', '🍩', '🧇', '🥐', '🍿', '🍫', '🥤'] },
  { labelKey: 'emojiTravel', emojis: ['✈️', '🚀', '🚗', '🚂', '⛵', '🏖️', '🏔️', '🌏', '🗺️', '🗼', '🎡', '🏰', '🎠', '🚁', '🛸', '🚢', '🛶', '🚌', '🚲', '🏄'] },
  { labelKey: 'emojiActivity', emojis: ['⚽', '🏀', '🎾', '🎯', '🎮', '🎵', '🎸', '📚', '💻', '📱', '🎨', '🎭', '🏋️', '🤸', '🧘', '🎲', '♟️', '🎻', '🎺', '🥁'] },
  { labelKey: 'emojiSymbols', emojis: ['✅', '❌', '⚠️', '💡', '🔑', '📌', '📍', '🔒', '🔓', '💰', '📧', '📞', '🔔', '💬', '📊', '📈', '📉', '🏆', '🎁', '🎗️'] },
] as const;

export function ComposeModalActions({
  editor,
  fileInputRef,
  imageInputRef,
  handleFileSelect,
  handleImageFileSelect,
  showEmojiPicker,
  setShowEmojiPicker,
  showDrivePicker,
  setShowDrivePicker,
  drivePickerNodes,
  drivePickerLoading,
  drivePickerCrumbs,
  attachingDriveId,
  openDrivePicker,
  handleAttachFromDrive,
  showTemplates,
  setShowTemplates,
  showTemplateSave,
  setShowTemplateSave,
  templates,
  templateSaveName,
  setTemplateSaveName,
  saveTemplate,
  deleteTemplate,
  subject,
  setSubject,
  showSigEditor,
  setShowSigEditor,
  trackOpens,
  setTrackOpens,
  showSchedule,
  setShowSchedule,
  scheduledAt,
  setScheduledAt,
  scheduleMinDateTime,
  scheduleOptions,
  imageResizeToolbar,
}: ComposeModalActionsProps) {
  const t = useTranslations('composeFull');
  return (
    <>
      <input
        ref={fileInputRef}
        type="file"
        multiple
        style={{ display: 'none' }}
        onChange={(e) => { if (e.target.files?.length) { handleFileSelect(e.target.files); e.target.value = ''; } }}
      />
      <input
        ref={imageInputRef}
        type="file"
        accept="image/*"
        style={{ display: 'none' }}
        onChange={(e) => { if (e.target.files?.[0]) { void handleImageFileSelect(e.target.files[0]); e.target.value = ''; } }}
      />
      <ComposeEditorToolbar editor={editor} />

      <div style={{ position: 'relative' }}>
        <button type="button" onClick={() => setShowEmojiPicker((value) => !value)} title={t('emoji')} style={toolbarBtnStyle(showEmojiPicker)} onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }} onMouseLeave={(e) => { (e.currentTarget).style.background = showEmojiPicker ? 'var(--color-bg-tertiary)' : 'transparent'; }}><FaceSmileIcon style={{ width: '14px', height: '14px' }} /></button>
        {showEmojiPicker && (
          <div style={{ position: 'absolute', bottom: '100%', left: 0, marginBottom: '4px', background: 'var(--color-bg-primary)', border: '1px solid var(--color-border-default)', borderRadius: '8px', boxShadow: '0 8px 24px rgba(0,0,0,0.16)', zIndex: 400, width: '260px', padding: '8px' }}>
            {EMOJI_GROUPS.map((cat) => (
              <div key={cat.labelKey} style={{ marginBottom: '6px' }}>
                <div style={{ fontSize: '10px', color: 'var(--color-text-tertiary)', fontWeight: 600, marginBottom: '4px', letterSpacing: '0.05em' }}>{t(cat.labelKey)}</div>
                <div style={{ display: 'flex', flexWrap: 'wrap', gap: '2px' }}>
                  {cat.emojis.map((em) => (
                    <button
                      key={em}
                      type="button"
                      onClick={() => { editor?.chain().focus().insertContent(em).run(); setShowEmojiPicker(false); }}
                      style={{ background: 'none', border: 'none', cursor: 'pointer', fontSize: '18px', padding: '2px', borderRadius: '4px', lineHeight: 1 }}
                      onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-secondary)'; }}
                      onMouseLeave={(e) => { (e.currentTarget).style.background = 'none'; }}
                    >
                      {em}
                    </button>
                  ))}
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      <button type="button" onClick={() => imageInputRef.current?.click()} title={t('insertImage')} style={toolbarBtnStyle()} onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }} onMouseLeave={(e) => { (e.currentTarget).style.background = 'transparent'; }}><PhotoIcon style={{ width: '14px', height: '14px' }} /></button>

      <div style={{ width: '1px', height: '16px', background: 'var(--color-border-subtle)' }} />

      <button type="button" onClick={() => fileInputRef.current?.click()} title={t('attachFile')} style={toolbarBtnStyle()} onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }} onMouseLeave={(e) => { (e.currentTarget).style.background = 'transparent'; }}><PaperClipIcon style={{ width: '14px', height: '14px' }} /></button>
      <div style={{ position: 'relative' }}>
        <button type="button" onClick={() => { if (!showDrivePicker) { openDrivePicker(undefined, [{ id: undefined, name: t('drive') }]); } else { setShowDrivePicker(false); } }} title={t('attachFromDrive')} style={toolbarBtnStyle(showDrivePicker)} onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }} onMouseLeave={(e) => { (e.currentTarget).style.background = showDrivePicker ? 'var(--color-bg-tertiary)' : 'transparent'; }}><CloudIcon style={{ width: '14px', height: '14px' }} /></button>
        <ComposeDrivePickerPanel
          open={showDrivePicker}
          drivePickerNodes={drivePickerNodes}
          drivePickerLoading={drivePickerLoading}
          drivePickerCrumbs={drivePickerCrumbs}
          attachingDriveId={attachingDriveId}
          openDrivePicker={openDrivePicker}
          handleAttachFromDrive={handleAttachFromDrive}
        />
      </div>
      <button type="button" onClick={() => setShowSigEditor((value) => !value)} title={t('signatureToggle')} style={toolbarBtnStyle(showSigEditor)} onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }} onMouseLeave={(e) => { (e.currentTarget).style.background = showSigEditor ? 'var(--color-bg-tertiary)' : 'transparent'; }}><PencilSquareIconHero style={{ width: '14px', height: '14px' }} /></button>
      <div style={{ position: 'relative' }}>
        <button
          type="button"
          onClick={() => { setShowTemplates((value) => !value); setShowTemplateSave(false); }}
          title={t('template')}
          style={toolbarBtnStyle(showTemplates)}
          onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }}
          onMouseLeave={(e) => { (e.currentTarget).style.background = showTemplates ? 'var(--color-bg-tertiary)' : 'transparent'; }}
        >
          <DocumentTextIcon style={{ width: '14px', height: '14px' }} />
        </button>
        <ComposeTemplatesPanel
          open={showTemplates}
          editor={editor}
          templates={templates}
          templateSaveName={templateSaveName}
          setTemplateSaveName={setTemplateSaveName}
          showTemplateSave={showTemplateSave}
          setShowTemplateSave={setShowTemplateSave}
          setShowTemplates={setShowTemplates}
          saveTemplate={saveTemplate}
          deleteTemplate={deleteTemplate}
          subject={subject}
          setSubject={setSubject}
        />
      </div>
      <label style={{ display: 'flex', alignItems: 'center', gap: '4px', cursor: 'pointer', fontSize: '12px', color: 'var(--color-text-secondary)', userSelect: 'none', whiteSpace: 'nowrap' }}>
        <input
          type="checkbox"
          checked={trackOpens}
          onChange={(e) => setTrackOpens(e.target.checked)}
          style={{ width: '12px', height: '12px', cursor: 'pointer', accentColor: 'var(--color-accent)' }}
        />
        {t('trackOpens')}
      </label>
      <ComposeSchedulePanel
        open={showSchedule}
        scheduledAt={scheduledAt}
        setScheduledAt={setScheduledAt}
        setShowSchedule={setShowSchedule}
        scheduleMinDateTime={scheduleMinDateTime}
      />
      {imageResizeToolbar && editor?.isActive('image') && (
        <div
          style={{
            position: 'fixed',
            top: imageResizeToolbar.top,
            left: imageResizeToolbar.left,
            zIndex: 500,
            display: 'flex',
            gap: '2px',
            background: 'var(--color-bg-primary)',
            border: '1px solid var(--color-border-default)',
            borderRadius: '6px',
            boxShadow: '0 4px 16px rgba(0,0,0,0.16)',
            padding: '3px',
          }}
        >
          {([[t('imgSmall'), '25%'], [t('imgMedium'), '50%'], [t('imgLarge'), '75%'], [t('imgOriginal'), '100%']] as const).map(([label, pct]) => (
            <button
              key={label}
              type="button"
              onMouseDown={(e) => {
                e.preventDefault();
                editor.chain().focus().updateAttributes('image', { style: `width: ${pct}` }).run();
              }}
              style={{
                padding: '2px 8px',
                fontSize: '11px',
                fontWeight: 500,
                borderRadius: '4px',
                border: 'none',
                background: 'transparent',
                color: 'var(--color-text-secondary)',
                cursor: 'pointer',
              }}
              onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }}
              onMouseLeave={(e) => { (e.currentTarget).style.background = 'transparent'; }}
            >
              {label}
            </button>
          ))}
        </div>
      )}
    </>
  );
}
