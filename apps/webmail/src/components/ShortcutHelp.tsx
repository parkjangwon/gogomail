'use client';

const SHORTCUTS = [
  { key: 'j / k', desc: '다음 / 이전 메일' },
  { key: 'c / n', desc: '새 메일 작성' },
  { key: 'r', desc: '답장' },
  { key: 'f', desc: '전달' },
  { key: 'u', desc: '읽지 않음으로 표시' },
  { key: 's', desc: '별표 토글' },
  { key: '# / Delete', desc: '삭제' },
  { key: 'g i', desc: '받은 편지함으로' },
  { key: 'g s', desc: '보낸 편지함으로' },
  { key: 'g d', desc: '임시 보관함으로' },
  { key: '/', desc: '검색창 포커스' },
  { key: 'Escape', desc: '닫기 / 선택 해제' },
  { key: '?', desc: '단축키 도움말' },
];

interface ShortcutHelpProps {
  onClose: () => void;
}

export function ShortcutHelp({ onClose }: ShortcutHelpProps) {
  return (
    <>
      <div
        aria-hidden="true"
        onClick={onClose}
        style={{ position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.4)', zIndex: 300 }}
      />
      <div
        role="dialog"
        aria-label="키보드 단축키"
        aria-modal="true"
        style={{
          position: 'fixed',
          top: '50%',
          left: '50%',
          transform: 'translate(-50%, -50%)',
          zIndex: 301,
          background: 'var(--color-bg-primary)',
          border: '1px solid var(--color-border-default)',
          borderRadius: '8px',
          boxShadow: '0 8px 32px rgba(0,0,0,0.2)',
          padding: '24px',
          minWidth: '320px',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '16px' }}>
          <span style={{ fontSize: '15px', fontWeight: 600, color: 'var(--color-text-primary)' }}>키보드 단축키</span>
          <button
            onClick={onClose}
            aria-label="닫기"
            style={{ background: 'none', border: 'none', cursor: 'pointer', fontSize: '18px', color: 'var(--color-text-secondary)', lineHeight: 1 }}
          >×</button>
        </div>
        <table style={{ width: '100%', borderCollapse: 'collapse' }}>
          <tbody>
            {SHORTCUTS.map(({ key, desc }) => (
              <tr key={key} style={{ borderBottom: '1px solid var(--color-border-subtle)' }}>
                <td style={{ padding: '8px 12px 8px 0', width: '40%' }}>
                  <kbd style={{
                    fontFamily: 'monospace',
                    fontSize: '12px',
                    background: 'var(--color-bg-secondary)',
                    border: '1px solid var(--color-border-default)',
                    borderRadius: '4px',
                    padding: '2px 6px',
                    color: 'var(--color-text-primary)',
                  }}>{key}</kbd>
                </td>
                <td style={{ padding: '8px 0', fontSize: '13px', color: 'var(--color-text-secondary)' }}>{desc}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </>
  );
}
