'use client';

import { useState, useEffect, useCallback, useRef } from 'react';
import { MagnifyingGlassIcon, EnvelopeIcon, UserIcon, XMarkIcon } from '@heroicons/react/24/outline';
import { listDirectoryUsers, DirectoryUser } from '@/lib/api';

interface OrgChartViewProps {
  onCompose?: (email: string) => void;
}

function UserAvatar({ user, size = 40 }: { user: DirectoryUser; size?: number }) {
  const initials = user.display_name
    .split(' ')
    .map((w) => w[0])
    .join('')
    .slice(0, 2)
    .toUpperCase();
  const hue = user.id.split('').reduce((acc, c) => acc + c.charCodeAt(0), 0) % 360;
  return (
    <div style={{
      width: size, height: size, borderRadius: '50%',
      background: `hsl(${hue}, 55%, 52%)`,
      display: 'flex', alignItems: 'center', justifyContent: 'center',
      fontSize: size * 0.35, fontWeight: 700, color: '#fff', flexShrink: 0,
      userSelect: 'none',
    }}>
      {initials || <UserIcon style={{ width: size * 0.5, height: size * 0.5 }} />}
    </div>
  );
}

function UserCard({ user, selected, onClick }: { user: DirectoryUser; selected: boolean; onClick: () => void }) {
  return (
    <div
      role="button"
      tabIndex={0}
      onClick={onClick}
      onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') onClick(); }}
      style={{
        display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '8px',
        padding: '16px 12px', borderRadius: '10px', cursor: 'pointer',
        background: selected ? 'var(--color-accent-subtle)' : 'var(--color-bg-secondary)',
        border: `1px solid ${selected ? 'var(--color-accent)' : 'var(--color-border-subtle)'}`,
        transition: 'all 0.15s',
        textAlign: 'center',
      }}
      onMouseEnter={(e) => {
        if (!selected) (e.currentTarget as HTMLDivElement).style.background = 'var(--color-bg-tertiary)';
      }}
      onMouseLeave={(e) => {
        if (!selected) (e.currentTarget as HTMLDivElement).style.background = 'var(--color-bg-secondary)';
      }}
    >
      <UserAvatar user={user} size={48} />
      <div style={{ width: '100%' }}>
        <div style={{
          fontSize: '13px', fontWeight: 600, color: 'var(--color-text-primary)',
          whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
        }}>
          {user.display_name}
        </div>
        <div style={{
          fontSize: '11px', color: 'var(--color-text-tertiary)',
          whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis', marginTop: '2px',
        }}>
          {user.email}
        </div>
      </div>
    </div>
  );
}

function DetailPanel({ user, onCompose, onClose }: { user: DirectoryUser; onCompose?: (email: string) => void; onClose: () => void }) {
  return (
    <div style={{
      width: '280px', flexShrink: 0, borderLeft: '1px solid var(--color-border-subtle)',
      display: 'flex', flexDirection: 'column', overflow: 'hidden',
    }}>
      <div style={{
        display: 'flex', alignItems: 'center', justifyContent: 'space-between',
        padding: '12px 16px', borderBottom: '1px solid var(--color-border-subtle)',
      }}>
        <span style={{ fontSize: '13px', fontWeight: 600, color: 'var(--color-text-primary)' }}>상세 정보</span>
        <button onClick={onClose} style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--color-text-tertiary)', padding: '2px', display: 'flex', borderRadius: '4px' }}
          onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }}
          onMouseLeave={(e) => { (e.currentTarget).style.background = 'none'; }}>
          <XMarkIcon style={{ width: '16px', height: '16px' }} />
        </button>
      </div>
      <div style={{ flex: 1, overflowY: 'auto', padding: '24px 20px', display: 'flex', flexDirection: 'column', gap: '20px', alignItems: 'center' }}>
        <UserAvatar user={user} size={80} />
        <div style={{ textAlign: 'center', width: '100%' }}>
          <div style={{ fontSize: '16px', fontWeight: 700, color: 'var(--color-text-primary)', marginBottom: '4px' }}>
            {user.display_name}
          </div>
          <div style={{ fontSize: '12px', color: 'var(--color-text-tertiary)' }}>
            {user.email}
          </div>
        </div>

        {onCompose && user.email && (
          <button
            onClick={() => onCompose(user.email)}
            style={{
              width: '100%', padding: '8px 16px',
              background: 'var(--color-accent)', color: '#fff',
              border: 'none', borderRadius: '6px', cursor: 'pointer',
              fontSize: '13px', fontWeight: 500, display: 'flex', alignItems: 'center', justifyContent: 'center', gap: '6px',
            }}
            onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.opacity = '0.85'; }}
            onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.opacity = '1'; }}
          >
            <EnvelopeIcon style={{ width: '15px', height: '15px' }} />
            메일 쓰기
          </button>
        )}

        <div style={{ width: '100%', display: 'flex', flexDirection: 'column', gap: '12px' }}>
          <InfoRow label="이메일" value={user.email} />
          <InfoRow label="ID" value={user.id} />
        </div>
      </div>
    </div>
  );
}

function InfoRow({ label, value }: { label: string; value: string }) {
  if (!value) return null;
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '2px' }}>
      <span style={{ fontSize: '10px', fontWeight: 600, textTransform: 'uppercase', letterSpacing: '0.06em', color: 'var(--color-text-tertiary)' }}>{label}</span>
      <span style={{ fontSize: '13px', color: 'var(--color-text-secondary)', wordBreak: 'break-all' }}>{value}</span>
    </div>
  );
}

export function OrgChartView({ onCompose }: OrgChartViewProps) {
  const [users, setUsers] = useState<DirectoryUser[]>([]);
  const [query, setQuery] = useState('');
  const [selected, setSelected] = useState<DirectoryUser | null>(null);
  const [loading, setLoading] = useState(false);
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const searchRef = useRef<HTMLInputElement | null>(null);

  const load = useCallback(async (q: string) => {
    setLoading(true);
    const results = await listDirectoryUsers(q || undefined, 100);
    setUsers(results);
    setLoading(false);
  }, []);

  useEffect(() => {
    load('');
  }, [load]);

  const handleQueryChange = (q: string) => {
    setQuery(q);
    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => load(q), 250);
  };

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const tag = (e.target as HTMLElement).tagName;
      if (tag === 'INPUT' || tag === 'TEXTAREA') return;
      if (e.key === '/' || e.key === 'f') {
        e.preventDefault();
        searchRef.current?.focus();
      }
      if (e.key === 'Escape') {
        if (selected) setSelected(null);
        else if (query) { setQuery(''); load(''); }
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [selected, query, load]);

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%', background: 'var(--color-bg-primary)', overflow: 'hidden' }}>
      {/* Toolbar */}
      <div style={{
        display: 'flex', alignItems: 'center', gap: '12px',
        padding: '12px 20px', borderBottom: '1px solid var(--color-border-subtle)', flexShrink: 0,
      }}>
        <span style={{ fontSize: '15px', fontWeight: 600, color: 'var(--color-text-primary)' }}>조직도</span>
        <div style={{ flex: 1, maxWidth: '340px', position: 'relative' }}>
          <MagnifyingGlassIcon style={{
            width: '15px', height: '15px', position: 'absolute', left: '10px', top: '50%',
            transform: 'translateY(-50%)', color: 'var(--color-text-tertiary)', pointerEvents: 'none',
          }} />
          <input
            ref={searchRef}
            type="text"
            placeholder="이름 또는 이메일 검색... (/)"
            value={query}
            onChange={(e) => handleQueryChange(e.target.value)}
            style={{
              width: '100%', padding: '7px 10px 7px 32px', fontSize: '13px',
              background: 'var(--color-bg-secondary)', border: '1px solid var(--color-border-subtle)',
              borderRadius: '6px', color: 'var(--color-text-primary)', outline: 'none', boxSizing: 'border-box',
            }}
            onFocus={(e) => { (e.target as HTMLInputElement).style.borderColor = 'var(--color-accent)'; }}
            onBlur={(e) => { (e.target as HTMLInputElement).style.borderColor = 'var(--color-border-subtle)'; }}
          />
        </div>
        <span style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', marginLeft: 'auto' }}>
          {loading ? '검색 중...' : `${users.length}명`}
        </span>
      </div>

      {/* Body */}
      <div style={{ display: 'flex', flex: 1, overflow: 'hidden' }}>
        {/* Grid */}
        <div style={{ flex: 1, overflowY: 'auto', padding: '16px 20px' }}>
          {users.length === 0 && !loading && (
            <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', height: '200px', gap: '8px', color: 'var(--color-text-tertiary)' }}>
              <UserIcon style={{ width: '36px', height: '36px', opacity: 0.4 }} />
              <span style={{ fontSize: '14px' }}>{query ? '검색 결과가 없습니다' : '구성원이 없습니다'}</span>
            </div>
          )}
          <div style={{
            display: 'grid',
            gridTemplateColumns: 'repeat(auto-fill, minmax(140px, 1fr))',
            gap: '12px',
          }}>
            {users.map((user) => (
              <UserCard
                key={user.id}
                user={user}
                selected={selected?.id === user.id}
                onClick={() => setSelected((prev) => prev?.id === user.id ? null : user)}
              />
            ))}
          </div>
        </div>

        {/* Detail Panel */}
        {selected && (
          <DetailPanel
            user={selected}
            onCompose={onCompose}
            onClose={() => setSelected(null)}
          />
        )}
      </div>
    </div>
  );
}
