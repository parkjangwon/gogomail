'use client';

import { type CSSProperties, type Dispatch, type RefObject, type SetStateAction } from 'react';
import { type Editor } from '@tiptap/react';
import type { DriveNode } from '@/lib/api';
import { DriveNodeIcon } from '@/lib/driveNodeIcon';
import type { EmailTemplate } from '@/lib/compose/composeUtils';
import {
  ChevronRightIcon,
  CloudIcon,
  DocumentTextIcon,
  FaceSmileIcon,
  LinkIcon,
  ListBulletIcon,
  NumberedListIcon,
  PaperClipIcon,
  PencilSquareIcon as PencilSquareIconHero,
  PhotoIcon,
  XMarkIcon,
} from '@heroicons/react/24/outline';

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

const SCHEDULE_INPUT_HELP = '예약 전송은 현재 시각 이후만 선택할 수 있습니다.';

const toolbarBtnStyle = (active?: boolean): CSSProperties => ({
  width: '28px',
  height: '28px',
  borderRadius: '4px',
  border: 'none',
  background: active ? 'var(--color-bg-tertiary)' : 'transparent',
  color: active ? 'var(--color-text-primary)' : 'var(--color-text-secondary)',
  cursor: 'pointer',
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  fontSize: '13px',
  fontWeight: 600,
  transition: 'background 80ms ease',
});

const EMOJI_GROUPS = [
  { label: '😊 자주 쓰는', emojis: ['😀', '😂', '🥰', '😍', '🤔', '😮', '😢', '😎', '🙏', '👍', '👎', '❤️', '🎉', '✨', '🔥', '💯', '😁', '🤣', '😇', '🥳'] },
  { label: '🐾 자연', emojis: ['🐶', '🐱', '🐭', '🐹', '🐰', '🦊', '🐻', '🐼', '🐨', '🐯', '🦁', '🐮', '🌸', '🌺', '🍀', '🌈', '⭐', '🌙', '☀️', '❄️'] },
  { label: '🍕 음식', emojis: ['🍕', '🍔', '🌮', '🍜', '🍣', '🍰', '☕', '🍺', '🎂', '🍎', '🥑', '🍓', '🍦', '🧁', '🍩', '🧇', '🥐', '🍿', '🍫', '🥤'] },
  { label: '✈️ 여행', emojis: ['✈️', '🚀', '🚗', '🚂', '⛵', '🏖️', '🏔️', '🌏', '🗺️', '🗼', '🎡', '🏰', '🎠', '🚁', '🛸', '🚢', '🛶', '🚌', '🚲', '🏄'] },
  { label: '🎵 활동', emojis: ['⚽', '🏀', '🎾', '🎯', '🎮', '🎵', '🎸', '📚', '💻', '📱', '🎨', '🎭', '🏋️', '🤸', '🧘', '🎲', '♟️', '🎻', '🎺', '🥁'] },
  { label: '💡 기호', emojis: ['✅', '❌', '⚠️', '💡', '🔑', '📌', '📍', '🔒', '🔓', '💰', '📧', '📞', '🔔', '💬', '📊', '📈', '📉', '🏆', '🎁', '🎗️'] },
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
      <button type="button" aria-label="굵게" title="굵게" style={toolbarBtnStyle(editor?.isActive('bold'))} onClick={() => editor?.chain().focus().toggleBold().run()} onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }} onMouseLeave={(e) => { (e.currentTarget).style.background = editor?.isActive('bold') ? 'var(--color-bg-tertiary)' : 'transparent'; }}><b>B</b></button>
      <button type="button" aria-label="기울임" title="기울임" style={toolbarBtnStyle(editor?.isActive('italic'))} onClick={() => editor?.chain().focus().toggleItalic().run()} onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }} onMouseLeave={(e) => { (e.currentTarget).style.background = editor?.isActive('italic') ? 'var(--color-bg-tertiary)' : 'transparent'; }}><i>I</i></button>
      <button type="button" aria-label="밑줄" title="밑줄" style={toolbarBtnStyle(editor?.isActive('underline'))} onClick={() => editor?.chain().focus().toggleUnderline().run()} onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }} onMouseLeave={(e) => { (e.currentTarget).style.background = editor?.isActive('underline') ? 'var(--color-bg-tertiary)' : 'transparent'; }}><u>U</u></button>
      <button type="button" aria-label="글머리 목록" title="글머리 목록" style={toolbarBtnStyle(editor?.isActive('bulletList'))} onClick={() => editor?.chain().focus().toggleBulletList().run()} onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }} onMouseLeave={(e) => { (e.currentTarget).style.background = editor?.isActive('bulletList') ? 'var(--color-bg-tertiary)' : 'transparent'; }}><ListBulletIcon style={{ width: '14px', height: '14px' }} /></button>
      <button type="button" aria-label="번호 목록" title="번호 목록" style={toolbarBtnStyle(editor?.isActive('orderedList'))} onClick={() => editor?.chain().focus().toggleOrderedList().run()} onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }} onMouseLeave={(e) => { (e.currentTarget).style.background = editor?.isActive('orderedList') ? 'var(--color-bg-tertiary)' : 'transparent'; }}><NumberedListIcon style={{ width: '14px', height: '14px' }} /></button>
      <button type="button" aria-label="링크" title="링크" style={toolbarBtnStyle(editor?.isActive('link'))} onClick={() => {
        const url = window.prompt('링크 URL을 입력하세요:');
        if (url && editor) editor.chain().focus().setLink({ href: url }).run();
      }} onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }} onMouseLeave={(e) => { (e.currentTarget).style.background = editor?.isActive('link') ? 'var(--color-bg-tertiary)' : 'transparent'; }}><LinkIcon style={{ width: '14px', height: '14px' }} /></button>

      <div style={{ position: 'relative' }}>
        <button type="button" onClick={() => setShowEmojiPicker((value) => !value)} title="이모지" style={toolbarBtnStyle(showEmojiPicker)} onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }} onMouseLeave={(e) => { (e.currentTarget).style.background = showEmojiPicker ? 'var(--color-bg-tertiary)' : 'transparent'; }}><FaceSmileIcon style={{ width: '14px', height: '14px' }} /></button>
        {showEmojiPicker && (
          <div style={{ position: 'absolute', bottom: '100%', left: 0, marginBottom: '4px', background: 'var(--color-bg-primary)', border: '1px solid var(--color-border-default)', borderRadius: '8px', boxShadow: '0 8px 24px rgba(0,0,0,0.16)', zIndex: 400, width: '260px', padding: '8px' }}>
            {EMOJI_GROUPS.map((cat) => (
              <div key={cat.label} style={{ marginBottom: '6px' }}>
                <div style={{ fontSize: '10px', color: 'var(--color-text-tertiary)', fontWeight: 600, marginBottom: '4px', letterSpacing: '0.05em' }}>{cat.label}</div>
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

      <button type="button" onClick={() => imageInputRef.current?.click()} title="이미지 삽입" style={toolbarBtnStyle()} onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }} onMouseLeave={(e) => { (e.currentTarget).style.background = 'transparent'; }}><PhotoIcon style={{ width: '14px', height: '14px' }} /></button>

      <div style={{ width: '1px', height: '16px', background: 'var(--color-border-subtle)' }} />

      <button type="button" onClick={() => fileInputRef.current?.click()} title="파일 첨부" style={toolbarBtnStyle()} onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }} onMouseLeave={(e) => { (e.currentTarget).style.background = 'transparent'; }}><PaperClipIcon style={{ width: '14px', height: '14px' }} /></button>
      <div style={{ position: 'relative' }}>
        <button type="button" onClick={() => { if (!showDrivePicker) { openDrivePicker(undefined, [{ id: undefined, name: '드라이브' }]); } else { setShowDrivePicker(false); } }} title="드라이브에서 첨부" style={toolbarBtnStyle(showDrivePicker)} onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }} onMouseLeave={(e) => { (e.currentTarget).style.background = showDrivePicker ? 'var(--color-bg-tertiary)' : 'transparent'; }}><CloudIcon style={{ width: '14px', height: '14px' }} /></button>
        {showDrivePicker && (
          <div style={{ position: 'absolute', bottom: '100%', left: 0, marginBottom: '4px', background: 'var(--color-bg-primary)', border: '1px solid var(--color-border-default)', borderRadius: '8px', boxShadow: '0 8px 24px rgba(0,0,0,0.16)', zIndex: 400, width: '280px', overflow: 'hidden' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '2px', padding: '8px 10px', borderBottom: '1px solid var(--color-border-subtle)', flexWrap: 'wrap' }}>
              {drivePickerCrumbs.map((crumb, i) => (
                <span key={i} style={{ display: 'inline-flex', alignItems: 'center', gap: '2px' }}>
                  {i > 0 && <ChevronRightIcon style={{ width: '10px', height: '10px', color: 'var(--color-text-tertiary)', flexShrink: 0 }} />}
                  <button type="button" onClick={() => { const newCrumbs = drivePickerCrumbs.slice(0, i + 1); openDrivePicker(crumb.id, newCrumbs); }} style={{ background: 'none', border: 'none', cursor: 'pointer', fontSize: '12px', color: i === drivePickerCrumbs.length - 1 ? 'var(--color-text-primary)' : 'var(--color-accent)', padding: '1px 3px', borderRadius: '3px', fontWeight: i === drivePickerCrumbs.length - 1 ? 600 : 400 }}>
                    {crumb.name}
                  </button>
                </span>
              ))}
            </div>
            <div style={{ maxHeight: '240px', overflowY: 'auto' }}>
              {drivePickerLoading ? (
                <div style={{ padding: '20px', textAlign: 'center', fontSize: '12px', color: 'var(--color-text-tertiary)' }}>불러오는 중...</div>
              ) : drivePickerNodes.length === 0 ? (
                <div style={{ padding: '20px', textAlign: 'center', fontSize: '12px', color: 'var(--color-text-tertiary)' }}>파일 없음</div>
              ) : drivePickerNodes.map((node) => (
                <button
                  key={node.id}
                  type="button"
                  onClick={() => handleAttachFromDrive(node)}
                  disabled={attachingDriveId === node.id}
                  style={{ width: '100%', display: 'flex', alignItems: 'center', gap: '8px', padding: '8px 12px', border: 'none', background: 'transparent', cursor: attachingDriveId === node.id ? 'wait' : 'pointer', textAlign: 'left' }}
                  onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-secondary)'; }}
                  onMouseLeave={(e) => { (e.currentTarget).style.background = 'transparent'; }}
                >
                  <DriveNodeIcon node={node} size={14} />
                  <span style={{ fontSize: '13px', color: 'var(--color-text-primary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', flex: 1 }}>{node.name}</span>
                  {node.node_type === 'folder' && <ChevronRightIcon style={{ width: '12px', height: '12px', color: 'var(--color-text-tertiary)', flexShrink: 0 }} />}
                  {attachingDriveId === node.id && <span style={{ fontSize: '11px', color: 'var(--color-text-tertiary)' }}>첨부 중...</span>}
                </button>
              ))}
            </div>
          </div>
        )}
      </div>
      <button type="button" onClick={() => setShowSigEditor((value) => !value)} title="서명" style={toolbarBtnStyle(showSigEditor)} onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }} onMouseLeave={(e) => { (e.currentTarget).style.background = showSigEditor ? 'var(--color-bg-tertiary)' : 'transparent'; }}><PencilSquareIconHero style={{ width: '14px', height: '14px' }} /></button>
      <div style={{ position: 'relative' }}>
        <button
          type="button"
          onClick={() => { setShowTemplates((value) => !value); setShowTemplateSave(false); }}
          title="템플릿"
          style={toolbarBtnStyle(showTemplates)}
          onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }}
          onMouseLeave={(e) => { (e.currentTarget).style.background = showTemplates ? 'var(--color-bg-tertiary)' : 'transparent'; }}
        >
          <DocumentTextIcon style={{ width: '14px', height: '14px' }} />
        </button>
        {showTemplates && (
          <div style={{ position: 'absolute', bottom: '100%', left: 0, marginBottom: '4px', background: 'var(--color-bg-primary)', border: '1px solid var(--color-border-default)', borderRadius: '8px', boxShadow: '0 8px 24px rgba(0,0,0,0.16)', zIndex: 400, width: '220px', overflow: 'hidden' }}>
            {templates.length === 0 && !showTemplateSave && (
              <div style={{ padding: '12px 14px', fontSize: '12px', color: 'var(--color-text-tertiary)' }}>저장된 템플릿이 없습니다</div>
            )}
            {templates.map((template) => (
              <div key={template.id} style={{ position: 'relative', display: 'flex', alignItems: 'center' }} onMouseEnter={(e) => { (e.currentTarget as HTMLDivElement).style.background = 'var(--color-bg-secondary)'; }} onMouseLeave={(e) => { (e.currentTarget as HTMLDivElement).style.background = 'transparent'; }}>
                <button
                  type="button"
                  onClick={() => { editor?.chain().focus().setContent(template.body).run(); if (!subject.trim()) setSubject(template.subject); setShowTemplates(false); }}
                  style={{ display: 'flex', flexDirection: 'column', alignItems: 'flex-start', flex: 1, padding: '8px 14px', border: 'none', background: 'transparent', cursor: 'pointer', textAlign: 'left', minWidth: 0 }}
                >
                  <span style={{ fontSize: '13px', fontWeight: 500, color: 'var(--color-text-primary)' }}>{template.name}</span>
                  {template.subject && <span style={{ fontSize: '11px', color: 'var(--color-text-tertiary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: '160px' }}>{template.subject}</span>}
                </button>
                <button type="button" onClick={(e) => { e.stopPropagation(); deleteTemplate(template.id); }} title="삭제" style={{ background: 'none', border: 'none', cursor: 'pointer', padding: '4px 8px', color: 'var(--color-text-tertiary)', display: 'inline-flex', flexShrink: 0 }} onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.color = 'var(--color-destructive)'; }} onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.color = 'var(--color-text-tertiary)'; }}><XMarkIcon style={{ width: '12px', height: '12px' }} /></button>
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
                + 현재 내용을 템플릿으로 저장
              </button>
            ) : (
              <div style={{ padding: '8px 14px', display: 'flex', gap: '6px' }}>
                <input
                  autoFocus
                  value={templateSaveName}
                  onChange={(e) => setTemplateSaveName(e.target.value)}
                  onKeyDown={(e) => { if (e.key === 'Enter') saveTemplate(); if (e.key === 'Escape') { setShowTemplateSave(false); setTemplateSaveName(''); } }}
                  placeholder="템플릿 이름"
                  style={{ flex: 1, padding: '4px 8px', borderRadius: '4px', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)', fontSize: '12px', outline: 'none' }}
                />
                <button type="button" onClick={saveTemplate} style={{ padding: '4px 10px', borderRadius: '4px', border: 'none', background: 'var(--color-accent)', color: '#fff', fontSize: '12px', cursor: 'pointer' }}>저장</button>
              </div>
            )}
          </div>
        )}
      </div>
      <label style={{ display: 'flex', alignItems: 'center', gap: '4px', cursor: 'pointer', fontSize: '12px', color: 'var(--color-text-secondary)', userSelect: 'none', whiteSpace: 'nowrap' }}>
        <input
          type="checkbox"
          checked={trackOpens}
          onChange={(e) => setTrackOpens(e.target.checked)}
          style={{ width: '12px', height: '12px', cursor: 'pointer', accentColor: 'var(--color-accent)' }}
        />
        수신확인
      </label>
      {showSchedule && (
        <>
          <input type="datetime-local" value={scheduledAt} onChange={(e) => setScheduledAt(e.target.value)} min={scheduleMinDateTime} aria-label="예약 전송 시간" aria-describedby="compose-schedule-help" style={{ fontSize: '12px', padding: '3px 6px', borderRadius: '4px', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-secondary)', color: 'var(--color-text-primary)', outline: 'none' }} />
          <span id="compose-schedule-help" style={{ fontSize: '11px', color: 'var(--color-text-tertiary)', whiteSpace: 'nowrap' }}>{SCHEDULE_INPUT_HELP}</span>
          <button
            type="button"
            onClick={() => { setScheduledAt(''); setShowSchedule(false); }}
            style={{ fontSize: '12px', padding: '3px 6px', borderRadius: '4px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', cursor: 'pointer' }}
          >예약 해제</button>
        </>
      )}
      {!showSchedule && scheduledAt && (
        <button
          type="button"
          onClick={() => setScheduledAt('')}
          style={{ fontSize: '12px', padding: '3px 6px', borderRadius: '4px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', cursor: 'pointer' }}
        >예약 해제</button>
      )}
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
          {([['소', '25%'], ['중', '50%'], ['대', '75%'], ['원본', '100%']] as const).map(([label, pct]) => (
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
