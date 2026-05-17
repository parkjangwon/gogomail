'use client';

import { XMarkIcon } from '@heroicons/react/24/outline';

const SECTIONS = [
  {
    title: '전역',
    items: [
      { key: '?', desc: '단축키 도움말' },
      { key: 'Cmd+k / Ctrl+k', desc: '스팟라이트 통합 검색' },
      { key: '/', desc: '스팟라이트 통합 검색' },
      { key: '[', desc: '사이드바 접기/펼치기' },
    ],
  },
  {
    title: '앱 전환',
    items: [
      { key: 'g  m', desc: '메일' },
      { key: 'g  c', desc: '캘린더' },
      { key: 'g  a', desc: '연락처' },
      { key: 'g  o', desc: '조직도' },
      { key: 'g  d', desc: '드라이브' },
      { key: 'g  ,', desc: '설정' },
    ],
  },
  {
    title: '메일 목록',
    items: [
      { key: 's', desc: '새 메일 작성' },
      { key: 'j / k', desc: '다음 / 이전 메일' },
      { key: '↑ / ↓', desc: '목록 이동' },
      { key: 'n / N', desc: '다음 / 이전 읽지 않은 메일' },
      { key: 'Enter / o', desc: '선택 메일 열기' },
      { key: 'Space / x', desc: '체크박스 토글' },
      { key: 'Home / End', desc: '첫 / 마지막 메일' },
      { key: 'Esc', desc: '읽기창 닫기 / 선택 해제' },
    ],
  },
  {
    title: '메일 동작',
    items: [
      { key: 'r', desc: '회신' },
      { key: 'a', desc: '전체 회신' },
      { key: 'f', desc: '전달' },
      { key: 's', desc: '별표 토글' },
      { key: 'e', desc: '보관' },
      { key: 'v', desc: '편지함으로 이동 (스팟라이트)' },
      { key: '#  /  Del', desc: '삭제' },
      { key: '!', desc: '스팸으로 이동' },
      { key: 'm', desc: '읽음 표시' },
      { key: 'Shift+m', desc: '안읽음 표시' },
      { key: 'z', desc: '1시간 스누즈' },
      { key: 'p', desc: '핀 고정 / 해제' },
      { key: 'i', desc: '중요 표시 / 해제' },
      { key: 't', desc: '이 메일을 할 일로 추가' },
      { key: 'l', desc: '라벨 색상 순환' },
    ],
  },
  {
    title: '폴더 이동',
    items: [
      { key: 'g  i', desc: '받은 편지함' },
      { key: 'g  s', desc: '보낸 편지함' },
      { key: 'g  t', desc: '휴지통' },
      { key: 'g  p', desc: '스팸 편지함' },
      { key: 'g  u', desc: '첫 읽지 않은 메일' },
      { key: 'g  x', desc: '중요 메일함' },
      { key: 'g  w', desc: '할 일 목록' },
    ],
  },
  {
    title: '메일 작성',
    items: [
      { key: 'Ctrl+Enter', desc: '전송' },
      { key: 'Ctrl+s', desc: '임시저장' },
      { key: 'Esc', desc: '작성창 닫기' },
    ],
  },
];

function KbdItem({ value }: { value: string }) {
  const parts = value.split('/').map((p) => p.trim());
  return (
    <span style={{ display: 'inline-flex', alignItems: 'center', gap: '4px', flexWrap: 'wrap' }}>
      {parts.map((part, i) => (
        <span key={i} style={{ display: 'inline-flex', alignItems: 'center', gap: '2px' }}>
          {i > 0 && <span style={{ color: 'var(--color-text-tertiary)', fontSize: '11px', margin: '0 1px' }}>/</span>}
          {part.split('+').map((k, j) => (
            <span key={j} style={{ display: 'inline-flex', alignItems: 'center', gap: '1px' }}>
              {j > 0 && <span style={{ color: 'var(--color-text-tertiary)', fontSize: '10px' }}>+</span>}
              <kbd style={{
                display: 'inline-block', padding: '1px 6px',
                fontSize: '11px', fontFamily: 'monospace', fontWeight: 600,
                color: 'var(--color-text-primary)',
                background: 'var(--color-bg-secondary)',
                border: '1px solid var(--color-border-default)',
                borderRadius: '4px', whiteSpace: 'nowrap',
              }}>{k}</kbd>
            </span>
          ))}
        </span>
      ))}
    </span>
  );
}

interface ShortcutHelpProps {
  onClose: () => void;
}

export function ShortcutHelp({ onClose }: ShortcutHelpProps) {
  return (
    <>
      <div aria-hidden="true" onClick={onClose}
        style={{ position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.45)', zIndex: 590 }} />
      <div
        role="dialog" aria-modal="true" aria-label="키보드 단축키"
        style={{
          position: 'fixed', top: '50%', left: '50%',
          transform: 'translate(-50%, -50%)',
          zIndex: 591, width: '660px', maxHeight: '82vh',
          background: 'var(--color-bg-primary)',
          border: '1px solid var(--color-border-subtle)',
          borderRadius: '12px',
          boxShadow: '0 20px 60px rgba(0,0,0,0.22)',
          display: 'flex', flexDirection: 'column', overflow: 'hidden',
        }}
      >
        {/* Header */}
        <div style={{
          display: 'flex', alignItems: 'center', justifyContent: 'space-between',
          padding: '16px 20px', borderBottom: '1px solid var(--color-border-subtle)', flexShrink: 0,
        }}>
          <div>
            <span style={{ fontSize: '15px', fontWeight: 600, color: 'var(--color-text-primary)' }}>키보드 단축키</span>
            <span style={{ marginLeft: '10px', fontSize: '12px', color: 'var(--color-text-tertiary)' }}>? 키로 열고 닫기</span>
          </div>
          <button aria-label="닫기" onClick={onClose}
            style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--color-text-tertiary)', display: 'flex', padding: '4px', borderRadius: '6px' }}
            onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; (e.currentTarget).style.color = 'var(--color-text-primary)'; }}
            onMouseLeave={(e) => { (e.currentTarget).style.background = 'none'; (e.currentTarget).style.color = 'var(--color-text-tertiary)'; }}
          >
            <XMarkIcon style={{ width: '18px', height: '18px' }} />
          </button>
        </div>

        {/* Body: 2-column grid */}
        <div style={{ overflowY: 'auto', padding: '20px 24px', display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '24px 32px' }}>
          {SECTIONS.map((section) => (
            <div key={section.title}>
              <div style={{
                fontSize: '10px', fontWeight: 700, letterSpacing: '0.08em',
                textTransform: 'uppercase', color: 'var(--color-text-tertiary)',
                marginBottom: '8px', paddingBottom: '4px',
                borderBottom: '1px solid var(--color-border-subtle)',
              }}>
                {section.title}
              </div>
              <div style={{ display: 'flex', flexDirection: 'column', gap: '2px' }}>
                {section.items.map((item) => (
                  <div key={item.key} style={{
                    display: 'flex', alignItems: 'center', justifyContent: 'space-between',
                    padding: '4px 6px', borderRadius: '4px', gap: '8px',
                  }}
                    onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-secondary)'; }}
                    onMouseLeave={(e) => { (e.currentTarget).style.background = 'transparent'; }}
                  >
                    <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)', flex: 1, minWidth: 0 }}>{item.desc}</span>
                    <KbdItem value={item.key} />
                  </div>
                ))}
              </div>
            </div>
          ))}
        </div>
      </div>
    </>
  );
}
