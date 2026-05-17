'use client';

import { useEffect, useState } from 'react';
import { XMarkIcon } from '@heroicons/react/24/outline';
import type { DriveNode, DriveShareLink } from '@/lib/api';
import { createDriveShareLink, listDriveShareLinks, revokeDriveShareLink } from '@/lib/api';
import { formatDate } from '@/lib/drive/driveUtils';

interface ShareModalProps {
  node: DriveNode;
  onClose: () => void;
}

export function DriveShareModal({ node, onClose }: ShareModalProps) {
  const [links, setLinks] = useState<DriveShareLink[]>([]);
  const [loading, setLoading] = useState(true);
  const [creating, setCreating] = useState(false);
  const [expiryDays, setExpiryDays] = useState(7);
  const [copied, setCopied] = useState('');

  useEffect(() => {
    listDriveShareLinks(node.id).then(setLinks).finally(() => setLoading(false));
  }, [node.id]);

  async function handleCreate() {
    setCreating(true);
    const expiresAt = new Date(Date.now() + expiryDays * 86400000).toISOString();
    const link = await createDriveShareLink(node.id, expiresAt);
    if (link) setLinks((prev) => [...prev, link]);
    setCreating(false);
  }

  async function handleRevoke(id: string) {
    await revokeDriveShareLink(id);
    setLinks((prev) => prev.filter((l) => l.id !== id));
  }

  function copyLink(link: DriveShareLink) {
    if (!link.token) return;
    const url = `${window.location.origin}/api/mail/drive/share-links/${encodeURIComponent(link.token)}/download`;
    navigator.clipboard.writeText(url).catch(() => {});
    setCopied(link.token_suffix);
    setTimeout(() => setCopied(''), 2000);
  }

  return (
    <div style={{ position: 'fixed', inset: 0, zIndex: 500, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
      <div aria-hidden="true" onClick={onClose} style={{ position: 'absolute', inset: 0, background: 'rgba(0,0,0,0.4)' }} />
      <div style={{ position: 'relative', background: 'var(--color-bg-primary)', border: '1px solid var(--color-border-default)', borderRadius: '12px', padding: '24px', width: '480px', maxWidth: '90vw', boxShadow: '0 8px 32px rgba(0,0,0,0.2)' }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '16px' }}>
          <h2 style={{ fontSize: '16px', fontWeight: 600, color: 'var(--color-text-primary)', margin: 0 }}>공유 링크 — {node.name}</h2>
          <button onClick={onClose} style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--color-text-tertiary)', display: 'flex' }}><XMarkIcon style={{ width: '20px', height: '20px' }} /></button>
        </div>
        <div style={{ display: 'flex', gap: '8px', marginBottom: '16px', alignItems: 'center' }}>
          <label style={{ fontSize: '13px', color: 'var(--color-text-secondary)' }}>유효 기간:</label>
          <select value={expiryDays} onChange={(e) => setExpiryDays(Number(e.target.value))}
            style={{ padding: '4px 8px', borderRadius: '5px', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)', fontSize: '13px' }}>
            <option value={1}>1일</option>
            <option value={7}>7일</option>
            <option value={30}>30일</option>
            <option value={90}>90일</option>
          </select>
          <button onClick={handleCreate} disabled={creating}
            style={{ padding: '5px 14px', borderRadius: '6px', border: 'none', background: 'var(--color-accent)', color: '#fff', fontSize: '13px', cursor: creating ? 'wait' : 'pointer' }}>
            {creating ? '생성 중...' : '링크 만들기'}
          </button>
        </div>
        {loading ? (
          <div style={{ fontSize: '13px', color: 'var(--color-text-tertiary)' }}>로딩 중...</div>
        ) : links.length === 0 ? (
          <div style={{ fontSize: '13px', color: 'var(--color-text-tertiary)' }}>공유 링크가 없습니다.</div>
        ) : (
          <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
            {links.map((link) => (
              <div key={link.id} style={{ display: 'flex', alignItems: 'center', gap: '8px', padding: '8px 12px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-secondary)' }}>
                <div style={{ flex: 1, minWidth: 0 }}>
                  <div style={{ fontSize: '12px', color: 'var(--color-text-secondary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                    .../{link.token_suffix}
                  </div>
                  <div style={{ fontSize: '11px', color: 'var(--color-text-tertiary)', marginTop: '2px' }}>
                    만료: {formatDate(link.expires_at)}
                  </div>
                </div>
                <button onClick={() => copyLink(link)} disabled={!link.token}
                  title={link.token ? '공유 URL 복사' : '보안을 위해 전체 URL은 생성 직후에만 표시됩니다. 새 링크를 만들어 복사하세요.'}
                  style={{ padding: '4px 10px', borderRadius: '5px', border: '1px solid var(--color-border-default)', background: 'transparent', color: link.token ? 'var(--color-accent)' : 'var(--color-text-tertiary)', fontSize: '12px', cursor: link.token ? 'pointer' : 'not-allowed' }}>
                  {copied === link.token_suffix ? '복사됨 ✓' : link.token ? '복사' : '재생성 필요'}
                </button>
                <button onClick={() => handleRevoke(link.id)}
                  style={{ padding: '4px 8px', borderRadius: '5px', border: 'none', background: 'transparent', color: 'var(--color-destructive)', cursor: 'pointer', display: 'flex' }}>
                  <XMarkIcon style={{ width: '14px', height: '14px' }} />
                </button>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
