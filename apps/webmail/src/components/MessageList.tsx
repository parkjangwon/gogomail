'use client';

import { MessageSummary } from '@/lib/api';

function formatDate(receivedAt: string): string {
  const date = new Date(receivedAt);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24));

  if (diffDays === 0) {
    return new Intl.DateTimeFormat('ko-KR', {
      hour: '2-digit',
      minute: '2-digit',
      hour12: false,
    }).format(date);
  }
  if (diffDays < 7) {
    return new Intl.DateTimeFormat('ko-KR', { weekday: 'short' }).format(date);
  }
  return new Intl.DateTimeFormat('ko-KR', {
    month: 'numeric',
    day: 'numeric',
  }).format(date);
}

function getDateGroup(receivedAt: string): string {
  const date = new Date(receivedAt);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24));

  if (diffDays === 0) return '오늘';
  if (diffDays === 1) return '어제';
  if (diffDays < 7) return '지난 7일';
  return '이번 달';
}

interface MessageListProps {
  messages: MessageSummary[];
  selectedId: string | null;
  onSelect: (id: string) => void;
  loading?: boolean;
  emptyLabel?: string;
}

export function MessageList({ messages, selectedId, onSelect, loading, emptyLabel }: MessageListProps) {
  if (loading) {
    return (
      <div
        style={{
          width: '380px',
          minWidth: '380px',
          height: '100%',
          borderRight: '1px solid var(--color-border-subtle)',
          overflowY: 'auto',
          padding: '12px 0',
        }}
      >
        {Array.from({ length: 8 }).map((_, i) => (
          <div
            key={i}
            style={{
              padding: '12px 16px',
              borderBottom: '1px solid var(--color-border-subtle)',
            }}
          >
            <div
              style={{
                height: '14px',
                background: 'var(--color-bg-tertiary)',
                borderRadius: '4px',
                marginBottom: '8px',
                width: `${60 + (i % 3) * 15}%`,
                animation: 'pulse 1.5s ease-in-out infinite',
              }}
            />
            <div
              style={{
                height: '12px',
                background: 'var(--color-bg-tertiary)',
                borderRadius: '4px',
                width: '80%',
                animation: 'pulse 1.5s ease-in-out infinite',
              }}
            />
          </div>
        ))}
      </div>
    );
  }

  if (messages.length === 0) {
    return (
      <div
        style={{
          width: '380px',
          minWidth: '380px',
          height: '100%',
          borderRight: '1px solid var(--color-border-subtle)',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          color: 'var(--color-text-tertiary)',
          fontSize: '14px',
        }}
      >
        {emptyLabel ?? '메일이 없습니다'}
      </div>
    );
  }

  // Group messages by date
  const groups: { label: string; messages: MessageSummary[] }[] = [];
  const groupOrder = ['오늘', '어제', '지난 7일', '이번 달'];
  const groupMap = new Map<string, MessageSummary[]>();

  for (const msg of messages) {
    const group = getDateGroup(msg.received_at);
    if (!groupMap.has(group)) groupMap.set(group, []);
    groupMap.get(group)!.push(msg);
  }

  for (const label of groupOrder) {
    if (groupMap.has(label)) {
      groups.push({ label, messages: groupMap.get(label)! });
    }
  }

  // Any remaining groups not in order
  for (const [label, msgs] of groupMap) {
    if (!groupOrder.includes(label)) {
      groups.push({ label, messages: msgs });
    }
  }

  return (
    <div
      role="list"
      aria-label="메일 목록"
      style={{
        width: '380px',
        minWidth: '380px',
        height: '100%',
        borderRight: '1px solid var(--color-border-subtle)',
        overflowY: 'auto',
        overflowX: 'hidden',
      }}
    >
      {groups.map((group) => (
        <div key={group.label} role="group" aria-label={group.label}>
          <div
            aria-hidden="true"
            style={{
              padding: '12px 16px 4px',
              fontSize: '12px',
              color: 'var(--color-text-tertiary)',
              fontWeight: 500,
            }}
          >
            {group.label}
          </div>
          {group.messages.map((msg) => (
            <MessageRow
              key={msg.id}
              message={msg}
              isSelected={selectedId === msg.id}
              onSelect={onSelect}
            />
          ))}
        </div>
      ))}
    </div>
  );
}

interface MessageRowProps {
  message: MessageSummary;
  isSelected: boolean;
  onSelect: (id: string) => void;
}

function MessageRow({ message, isSelected, onSelect }: MessageRowProps) {
  const isUnread = !message.read;

  return (
    <div
      role="listitem"
      onClick={() => onSelect(message.id)}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault();
          onSelect(message.id);
        }
      }}
      tabIndex={0}
      aria-selected={isSelected}
      style={{
        display: 'flex',
        alignItems: 'flex-start',
        gap: '8px',
        padding: '12px 16px',
        borderBottom: '1px solid var(--color-border-subtle)',
        background: isSelected
          ? 'var(--color-accent-subtle)'
          : 'var(--color-bg-primary)',
        cursor: 'pointer',
        transition: 'background 100ms ease',
        position: 'relative',
      }}
      onMouseEnter={(e) => {
        if (!isSelected) {
          (e.currentTarget as HTMLDivElement).style.background = 'var(--color-bg-secondary)';
        }
      }}
      onMouseLeave={(e) => {
        if (!isSelected) {
          (e.currentTarget as HTMLDivElement).style.background = 'var(--color-bg-primary)';
        }
      }}
    >
      {/* Unread dot */}
      <div
        aria-hidden="true"
        style={{
          width: '6px',
          height: '6px',
          borderRadius: '50%',
          background: isUnread ? 'var(--color-accent)' : 'transparent',
          marginTop: '6px',
          flexShrink: 0,
          transition: 'opacity 200ms ease',
        }}
      />

      {/* Content */}
      <div style={{ flex: 1, minWidth: 0 }}>
        {/* Row 1: sender + date */}
        <div
          style={{
            display: 'flex',
            alignItems: 'baseline',
            justifyContent: 'space-between',
            gap: '8px',
            marginBottom: '3px',
          }}
        >
          <span
            style={{
              fontSize: '14px',
              fontWeight: isUnread ? 500 : 400,
              color: 'var(--color-text-primary)',
              overflow: 'hidden',
              textOverflow: 'ellipsis',
              whiteSpace: 'nowrap',
              flex: 1,
              minWidth: 0,
            }}
          >
            {message.from_name || message.from_addr}
          </span>
          <span
            style={{
              fontSize: '13px',
              color: 'var(--color-text-secondary)',
              whiteSpace: 'nowrap',
              flexShrink: 0,
            }}
          >
            {formatDate(message.received_at)}
          </span>
        </div>

        {/* Row 2: subject + preview */}
        <div
          style={{
            fontSize: '14px',
            color: 'var(--color-text-primary)',
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            whiteSpace: 'nowrap',
          }}
        >
          <span style={{ fontWeight: isUnread ? 600 : 400 }}>
            {message.subject || '(제목 없음)'}
          </span>
          {message.preview && (
            <span
              style={{
                color: 'var(--color-text-secondary)',
                fontWeight: 400,
              }}
            >
              {' · '}
              {message.preview}
            </span>
          )}
        </div>
      </div>
    </div>
  );
}
