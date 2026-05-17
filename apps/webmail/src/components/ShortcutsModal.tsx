'use client';

import { XMarkIcon } from '@heroicons/react/24/outline';

interface ShortcutsModalProps {
  onClose: () => void;
}

const SHORTCUTS: { category: string; items: { key: string; desc: string }[] }[] = [
  {
    category: '전역',
    items: [
      { key: '?', desc: '단축키 도움말' },
      { key: 'Ctrl + k', desc: '검색창 포커스' },
      { key: '[', desc: '사이드바 접기/펼치기' },
    ],
  },
  {
    category: '앱 전환',
    items: [
      { key: 'g  m', desc: '메일' },
      { key: 'g  c', desc: '캘린더' },
      { key: 'g  a', desc: '연락처' },
      { key: 'g  d', desc: '드라이브' },
    ],
  },
  {
    category: '메일 목록',
    items: [
      { key: 'c', desc: '새 메일 작성' },
      { key: 'j / k', desc: '다음 / 이전 메일' },
      { key: '↑ / ↓', desc: '목록 이동' },
      { key: 'Enter', desc: '선택 메일 열기' },
      { key: 'u', desc: '목록으로 / 메일 닫기' },
      { key: '/', desc: '검색창 포커스' },
      { key: 'Esc', desc: '검색 닫기 / 뒤로' },
      { key: 'Space', desc: '체크박스 토글' },
      { key: 'Home / End', desc: '첫/마지막 메일' },
      { key: '* a', desc: '전체 선택' },
      { key: '* n', desc: '전체 해제' },
    ],
  },
  {
    category: '메일 동작',
    items: [
      { key: 'r', desc: '회신' },
      { key: 'a', desc: '전체 회신' },
      { key: 'f', desc: '전달' },
      { key: 'e', desc: '보관' },
      { key: '#', desc: '삭제' },
      { key: '!', desc: '스팸으로 이동' },
      { key: 'm', desc: '읽음 표시' },
      { key: 'Shift + m', desc: '안읽음 표시' },
    ],
  },
  {
    category: '메일 작성',
    items: [
      { key: 'Ctrl + Enter', desc: '전송' },
      { key: 'Ctrl + s', desc: '임시저장' },
      { key: 'Esc', desc: '작성창 닫기' },
    ],
  },
];

function Kbd({ children }: { children: string }) {
  return (
    <span style={{ display: 'inline-flex', alignItems: 'center', gap: '2px' }}>
      {children.split(' / ').map((part, i) => (
        <span key={i} style={{ display: 'inline-flex', alignItems: 'center', gap: '2px' }}>
          {i > 0 && <span style={{ color: 'var(--color-text-tertiary)', margin: '0 2px' }}>/</span>}
          {part.trim().split('+').map((k, j) => (
            <span key={j} style={{ display: 'inline-flex', alignItems: 'center', gap: '2px' }}>
              {j > 0 && <span style={{ color: 'var(--color-text-tertiary)', fontSize: '10px' }}>+</span>}
              <kbd style={{
                display: 'inline-block',
                padding: '2px 7px',
                fontSize: '11px',
                fontFamily: 'monospace',
                fontWeight: 600,
                lineHeight: '18px',
                color: 'var(--color-text-primary)',
                background: 'var(--color-bg-secondary)',
                border: '1px solid var(--color-border-default)',
                borderRadius: '4px',
                whiteSpace: 'nowrap',
              }}>{k.trim()}</kbd>
            </span>
          ))}
        </span>
      ))}
    </span>
  );
}

export function ShortcutsModal({ onClose }: ShortcutsModalProps) {
  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label="키보드 단축키"
      style={{
        position: 'fixed',
        inset: 0,
        background: 'rgba(0,0,0,0.45)',
        zIndex: 600,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        padding: '24px',
      }}
      onClick={(e) => { if (e.target === e.currentTarget) onClose(); }}
    >
      <div
        style={{
          width: '640px',
          maxHeight: '80vh',
          borderRadius: '12px',
          background: 'var(--color-bg-primary)',
          boxShadow: '0 20px 60px rgba(0,0,0,0.25)',
          display: 'flex',
          flexDirection: 'column',
          overflow: 'hidden',
        }}
      >
        {/* Header */}
        <div style={{
          display: 'flex', alignItems: 'center', justifyContent: 'space-between',
          padding: '16px 20px', borderBottom: '1px solid var(--color-border-subtle)', flexShrink: 0,
        }}>
          <div>
            <span style={{ fontSize: '16px', fontWeight: 600, color: 'var(--color-text-primary)' }}>키보드 단축키</span>
            <span style={{ marginLeft: '10px', fontSize: '12px', color: 'var(--color-text-tertiary)' }}>? 키로 열고 닫기</span>
          </div>
          <button
            aria-label="닫기"
            onClick={onClose}
            style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--color-text-tertiary)', display: 'flex', padding: '4px', borderRadius: '6px' }}
            onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; (e.currentTarget).style.color = 'var(--color-text-primary)'; }}
            onMouseLeave={(e) => { (e.currentTarget).style.background = 'none'; (e.currentTarget).style.color = 'var(--color-text-tertiary)'; }}
          >
            <XMarkIcon style={{ width: '18px', height: '18px' }} />
          </button>
        </div>

        {/* Content — two-column grid */}
        <div style={{ overflowY: 'auto', padding: '20px', display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '20px' }}>
          {SHORTCUTS.map((section) => (
            <div key={section.category}>
              <div style={{ fontSize: '11px', fontWeight: 600, letterSpacing: '0.06em', textTransform: 'uppercase', color: 'var(--color-text-tertiary)', marginBottom: '8px' }}>
                {section.category}
              </div>
              <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
                {section.items.map((item) => (
                  <div key={item.key} style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '5px 8px', borderRadius: '5px', gap: '12px' }}
                    onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-secondary)'; }}
                    onMouseLeave={(e) => { (e.currentTarget).style.background = 'transparent'; }}
                  >
                    <span style={{ fontSize: '13px', color: 'var(--color-text-secondary)', flex: 1 }}>{item.desc}</span>
                    <Kbd>{item.key}</Kbd>
                  </div>
                ))}
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
