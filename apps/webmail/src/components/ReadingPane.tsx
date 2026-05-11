'use client';

import { MessageDetail } from '@/lib/api';

interface ReadingPaneProps {
  message: MessageDetail | null;
  onDelete?: () => void;
  onReply?: () => void;
  onForward?: () => void;
  loading?: boolean;
}

function formatFullDate(receivedAt: string): string {
  return new Intl.DateTimeFormat('ko-KR', {
    year: 'numeric',
    month: 'long',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).format(new Date(receivedAt));
}

const iconStyle: React.CSSProperties = {
  display: 'inline-flex',
  alignItems: 'center',
  justifyContent: 'center',
  padding: '5px 10px',
  borderRadius: '5px',
  border: '1px solid var(--color-border-default)',
  background: 'transparent',
  color: 'var(--color-text-secondary)',
  fontSize: '13px',
  cursor: 'pointer',
  transition: 'background 100ms ease, color 100ms ease',
};

function ActionButton({
  label,
  onClick,
  danger,
}: {
  label: string;
  onClick?: () => void;
  danger?: boolean;
}) {
  return (
    <button
      onClick={onClick}
      title={label}
      aria-label={label}
      style={{
        ...iconStyle,
        color: danger ? 'var(--color-destructive)' : 'var(--color-text-secondary)',
        borderColor: danger ? 'rgba(217,79,61,0.3)' : 'var(--color-border-default)',
      }}
      onMouseEnter={(e) => {
        (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)';
      }}
      onMouseLeave={(e) => {
        (e.currentTarget as HTMLButtonElement).style.background = 'transparent';
      }}
    >
      {label}
    </button>
  );
}

export function ReadingPane({
  message,
  onDelete,
  onReply,
  onForward,
  loading,
}: ReadingPaneProps) {
  if (loading) {
    return (
      <main
        aria-label="메일 읽기"
        style={{
          flex: 1,
          minWidth: 0,
          height: '100%',
          overflowY: 'auto',
          padding: '20px 24px',
          background: 'var(--color-bg-primary)',
          display: 'flex',
          flexDirection: 'column',
          gap: '16px',
        }}
      >
        {[100, 60, 80, 40, 70, 90].map((w, i) => (
          <div
            key={i}
            style={{
              height: i === 0 ? '24px' : '14px',
              background: 'var(--color-bg-tertiary)',
              borderRadius: '4px',
              width: `${w}%`,
            }}
          />
        ))}
      </main>
    );
  }

  if (!message) {
    return (
      <main
        aria-label="메일 읽기"
        style={{
          flex: 1,
          minWidth: 0,
          height: '100%',
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          justifyContent: 'center',
          gap: '8px',
          background: 'var(--color-bg-primary)',
          color: 'var(--color-text-tertiary)',
        }}
      >
        <svg
          width="40"
          height="40"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="1.5"
          strokeLinecap="round"
          strokeLinejoin="round"
          aria-hidden="true"
        >
          <rect x="2" y="4" width="20" height="16" rx="2" />
          <path d="m22 7-8.97 5.7a1.94 1.94 0 0 1-2.06 0L2 7" />
        </svg>
        <p style={{ fontSize: '14px' }}>메시지를 선택하세요</p>
      </main>
    );
  }

  const toList = (message.to_addrs ?? [])
    .map((t) => (t.name ? `${t.name} <${t.address}>` : t.address))
    .join(', ');

  return (
    <main
      aria-label="메일 읽기"
      style={{
        flex: 1,
        minWidth: 0,
        height: '100%',
        overflowY: 'auto',
        background: 'var(--color-bg-primary)',
        display: 'flex',
        flexDirection: 'column',
      }}
    >
      {/* Toolbar */}
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'flex-end',
          gap: '8px',
          padding: '12px 24px',
          borderBottom: '1px solid var(--color-border-subtle)',
          flexShrink: 0,
        }}
      >
        <ActionButton label="답장" onClick={onReply} />
        <ActionButton label="전달" onClick={onForward} />
        <ActionButton label="삭제" onClick={onDelete} danger />
      </div>

      {/* Message content */}
      <div style={{ padding: '20px 24px', flex: 1 }}>
        {/* Subject */}
        <h1
          style={{
            fontSize: '18px',
            fontWeight: 600,
            color: 'var(--color-text-primary)',
            lineHeight: 1.4,
            marginBottom: '16px',
          }}
        >
          {message.subject || '(제목 없음)'}
        </h1>

        {/* Sender row */}
        <div
          style={{
            display: 'flex',
            alignItems: 'flex-start',
            justifyContent: 'space-between',
            gap: '16px',
            marginBottom: '8px',
          }}
        >
          <div>
            <div
              style={{
                fontSize: '14px',
                fontWeight: 500,
                color: 'var(--color-text-primary)',
              }}
            >
              {message.from_name || message.from_addr}
              {message.from_name && (
                <span
                  style={{
                    fontSize: '13px',
                    fontWeight: 400,
                    color: 'var(--color-text-secondary)',
                    marginInlineStart: '6px',
                  }}
                >
                  &lt;{message.from_addr}&gt;
                </span>
              )}
            </div>
            {toList && (
              <div
                style={{
                  fontSize: '13px',
                  color: 'var(--color-text-secondary)',
                  marginTop: '2px',
                }}
              >
                받는 사람: {toList}
              </div>
            )}
          </div>
          <span
            style={{
              fontSize: '13px',
              color: 'var(--color-text-secondary)',
              flexShrink: 0,
            }}
          >
            {formatFullDate(message.received_at)}
          </span>
        </div>

        {/* Divider */}
        <hr
          style={{
            border: 'none',
            borderTop: '1px solid var(--color-border-subtle)',
            margin: '16px 0',
          }}
        />

        {/* Body */}
        <div
          style={{
            maxWidth: '680px',
            fontSize: '14px',
            lineHeight: 1.6,
            color: 'var(--color-text-primary)',
          }}
        >
          <pre
            style={{
              fontFamily: 'inherit',
              whiteSpace: 'pre-wrap',
              wordBreak: 'break-word',
              margin: 0,
            }}
          >
            {message.text_body || '(내용 없음)'}
          </pre>
        </div>
      </div>
    </main>
  );
}
