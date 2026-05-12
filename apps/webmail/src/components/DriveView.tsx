'use client';

import { useState, useEffect, useRef, useCallback } from 'react';
import {
  DriveNode, DriveUsage, DriveShareLink,
  listDriveNodes, listTrashedDriveNodes, getDriveUsage, createDriveFolder,
  renameDriveNode, trashDriveNode, restoreDriveNode, deleteDriveNodePermanently,
  downloadDriveNode, uploadDriveFile, createDriveShareLink, listDriveShareLinks, revokeDriveShareLink,
} from '@/lib/api';
import { DriveNodeIcon } from '@/lib/driveNodeIcon';
import {
  FolderIcon, ArrowUpTrayIcon, FolderPlusIcon,
  EllipsisVerticalIcon, ArrowDownTrayIcon, LinkIcon, PencilIcon,
  TrashIcon, XMarkIcon, ArrowPathIcon, ChevronRightIcon, ArrowUturnLeftIcon,
} from '@heroicons/react/24/outline';
import { FolderIcon as FolderSolid, TrashIcon as TrashSolid } from '@heroicons/react/24/solid';

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
  return `${(bytes / 1024 / 1024 / 1024).toFixed(2)} GB`;
}

function formatDate(iso: string): string {
  return new Intl.DateTimeFormat('ko-KR', { year: 'numeric', month: 'short', day: 'numeric' }).format(new Date(iso));
}

interface BreadcrumbItem { id: string; name: string; }

interface NodeMenuProps {
  node: DriveNode;
  onDownload: () => void;
  onRename: () => void;
  onShare: () => void;
  onTrash: () => void;
  onClose: () => void;
}

function NodeMenu({ node, onDownload, onRename, onShare, onTrash, onClose }: NodeMenuProps) {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    function onDown(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) onClose();
    }
    document.addEventListener('mousedown', onDown);
    return () => document.removeEventListener('mousedown', onDown);
  }, [onClose]);
  const item = (label: string, icon: React.ReactNode, onClick: () => void, danger?: boolean): React.ReactNode => (
    <button onClick={() => { onClick(); onClose(); }}
      style={{ display: 'flex', alignItems: 'center', gap: '8px', width: '100%', padding: '7px 14px', border: 'none', background: 'transparent', color: danger ? 'var(--color-destructive)' : 'var(--color-text-primary)', fontSize: '13px', cursor: 'pointer', textAlign: 'left' }}
      onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
      onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
    >{icon}{label}</button>
  );
  return (
    <div ref={ref} style={{ position: 'absolute', top: '100%', right: 0, marginTop: '2px', background: 'var(--color-bg-primary)', border: '1px solid var(--color-border-default)', borderRadius: '8px', boxShadow: '0 4px 20px rgba(0,0,0,0.14)', zIndex: 200, minWidth: '180px', overflow: 'hidden', padding: '4px 0' }}>
      {node.node_type === 'file' && item('다운로드', <ArrowDownTrayIcon style={{ width: '14px', height: '14px' }} />, onDownload)}
      {item('이름 변경', <PencilIcon style={{ width: '14px', height: '14px' }} />, onRename)}
      {item('공유 링크', <LinkIcon style={{ width: '14px', height: '14px' }} />, onShare)}
      <div style={{ height: '1px', background: 'var(--color-border-subtle)', margin: '4px 0' }} />
      {item('휴지통', <TrashIcon style={{ width: '14px', height: '14px' }} />, onTrash, true)}
    </div>
  );
}

interface ShareModalProps {
  node: DriveNode;
  onClose: () => void;
}

function ShareModal({ node, onClose }: ShareModalProps) {
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

  function copyLink(suffix: string) {
    const url = `${window.location.origin}/api/mail/drive/share-links/${suffix}/download`;
    navigator.clipboard.writeText(url).catch(() => {});
    setCopied(suffix);
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
                <button onClick={() => copyLink(link.token_suffix)}
                  style={{ padding: '4px 10px', borderRadius: '5px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-accent)', fontSize: '12px', cursor: 'pointer' }}>
                  {copied === link.token_suffix ? '복사됨 ✓' : '복사'}
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

export function DriveView() {
  const [activeSection, setActiveSection] = useState<'drive' | 'trash'>('drive');
  const [breadcrumb, setBreadcrumb] = useState<BreadcrumbItem[]>([{ id: '', name: '내 드라이브' }]);
  const [nodes, setNodes] = useState<DriveNode[]>([]);
  const [trashNodes, setTrashNodes] = useState<DriveNode[]>([]);
  const [usage, setUsage] = useState<DriveUsage | null>(null);
  const [loading, setLoading] = useState(true);
  const [trashLoading, setTrashLoading] = useState(false);
  const [menuNodeId, setMenuNodeId] = useState<string | null>(null);
  const [renameNodeId, setRenameNodeId] = useState<string | null>(null);
  const [renameName, setRenameName] = useState('');
  const [shareNode, setShareNode] = useState<DriveNode | null>(null);
  const [newFolderMode, setNewFolderMode] = useState(false);
  const [newFolderName, setNewFolderName] = useState('');
  const [uploading, setUploading] = useState(false);
  const [dragOver, setDragOver] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const newFolderRef = useRef<HTMLInputElement>(null);
  const renameRef = useRef<HTMLInputElement>(null);

  const currentParentId = breadcrumb[breadcrumb.length - 1]?.id ?? '';

  const loadNodes = useCallback(async (parentId: string) => {
    setLoading(true);
    const data = await listDriveNodes(parentId || undefined);
    setNodes(data.sort((a, b) => {
      if (a.node_type !== b.node_type) return a.node_type === 'folder' ? -1 : 1;
      return a.name.localeCompare(b.name, 'ko');
    }));
    setLoading(false);
  }, []);

  const loadTrashNodes = useCallback(async () => {
    setTrashLoading(true);
    const data = await listTrashedDriveNodes();
    setTrashNodes(data.sort((a, b) => a.name.localeCompare(b.name, 'ko')));
    setTrashLoading(false);
  }, []);

  useEffect(() => {
    loadNodes(currentParentId);
    getDriveUsage().then(setUsage);
  }, [currentParentId, loadNodes]);

  useEffect(() => {
    if (activeSection === 'trash') loadTrashNodes();
  }, [activeSection, loadTrashNodes]);

  useEffect(() => { if (newFolderMode) setTimeout(() => newFolderRef.current?.focus(), 50); }, [newFolderMode]);
  useEffect(() => { if (renameNodeId) setTimeout(() => renameRef.current?.select(), 50); }, [renameNodeId]);

  function navigateTo(item: BreadcrumbItem) {
    const idx = breadcrumb.findIndex((b) => b.id === item.id);
    if (idx !== -1) setBreadcrumb(breadcrumb.slice(0, idx + 1));
  }

  function openFolder(node: DriveNode) {
    if (node.node_type !== 'folder') return;
    setBreadcrumb((prev) => [...prev, { id: node.id, name: node.name }]);
  }

  async function handleCreateFolder() {
    if (!newFolderName.trim()) { setNewFolderMode(false); return; }
    const created = await createDriveFolder(newFolderName.trim(), currentParentId || undefined);
    if (created) setNodes((prev) => [created, ...prev]);
    setNewFolderName('');
    setNewFolderMode(false);
  }

  async function handleRename() {
    if (!renameNodeId || !renameName.trim()) { setRenameNodeId(null); return; }
    const ok = await renameDriveNode(renameNodeId, renameName.trim());
    if (ok) setNodes((prev) => prev.map((n) => n.id === renameNodeId ? { ...n, name: renameName.trim() } : n));
    setRenameNodeId(null);
  }

  async function handleTrash(nodeId: string) {
    const ok = await trashDriveNode(nodeId);
    if (ok) setNodes((prev) => prev.filter((n) => n.id !== nodeId));
    getDriveUsage().then(setUsage);
  }

  async function handleRestore(nodeId: string) {
    const ok = await restoreDriveNode(nodeId);
    if (ok) {
      setTrashNodes((prev) => prev.filter((n) => n.id !== nodeId));
      getDriveUsage().then(setUsage);
    }
  }

  async function handlePermanentDelete(nodeId: string) {
    if (!confirm('영구 삭제하면 복원할 수 없습니다. 계속하시겠습니까?')) return;
    const ok = await deleteDriveNodePermanently(nodeId);
    if (ok) {
      setTrashNodes((prev) => prev.filter((n) => n.id !== nodeId));
      getDriveUsage().then(setUsage);
    }
  }

  async function handleEmptyTrash() {
    if (!confirm(`휴지통을 비우면 ${trashNodes.length}개 항목이 영구 삭제됩니다. 계속하시겠습니까?`)) return;
    await Promise.all(trashNodes.map((n) => deleteDriveNodePermanently(n.id)));
    setTrashNodes([]);
    getDriveUsage().then(setUsage);
  }

  async function handleUpload(files: FileList) {
    setUploading(true);
    for (const file of Array.from(files)) {
      const node = await uploadDriveFile(file, currentParentId || undefined);
      if (node) setNodes((prev) => [...prev, node]);
    }
    setUploading(false);
    getDriveUsage().then(setUsage);
  }

  const usedPct = usage && usage.quota_limit > 0 ? Math.min(100, (usage.quota_used / usage.quota_limit) * 100) : 0;
  const barColor = usedPct >= 90 ? '#ef4444' : usedPct >= 70 ? '#f59e0b' : '#22c55e';

  return (
    <div style={{ flex: 1, minWidth: 0, height: '100%', display: 'flex', background: 'var(--color-bg-primary)', position: 'relative' }}>

      {/* ── Sidebar ── */}
      <div style={{ width: '200px', flexShrink: 0, borderRight: '1px solid var(--color-border-subtle)', display: 'flex', flexDirection: 'column', padding: '12px 0', overflowY: 'auto' }}>
        {/* Nav items */}
        <div style={{ padding: '0 8px', marginBottom: '4px' }}>
          <button
            onClick={() => setActiveSection('drive')}
            style={{ display: 'flex', alignItems: 'center', gap: '8px', width: '100%', padding: '7px 10px', borderRadius: '6px', border: 'none', background: activeSection === 'drive' ? 'var(--color-accent-subtle)' : 'transparent', color: activeSection === 'drive' ? 'var(--color-accent)' : 'var(--color-text-secondary)', fontSize: '13px', fontWeight: activeSection === 'drive' ? 600 : 400, cursor: 'pointer', textAlign: 'left' }}
            onMouseEnter={(e) => { if (activeSection !== 'drive') (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
            onMouseLeave={(e) => { if (activeSection !== 'drive') (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
          >
            <FolderSolid style={{ width: '16px', height: '16px', flexShrink: 0 }} />
            내 드라이브
          </button>
        </div>
        <div style={{ padding: '0 8px', marginBottom: '16px' }}>
          <button
            onClick={() => setActiveSection('trash')}
            style={{ display: 'flex', alignItems: 'center', gap: '8px', width: '100%', padding: '7px 10px', borderRadius: '6px', border: 'none', background: activeSection === 'trash' ? 'var(--color-accent-subtle)' : 'transparent', color: activeSection === 'trash' ? 'var(--color-accent)' : 'var(--color-text-secondary)', fontSize: '13px', fontWeight: activeSection === 'trash' ? 600 : 400, cursor: 'pointer', textAlign: 'left' }}
            onMouseEnter={(e) => { if (activeSection !== 'trash') (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
            onMouseLeave={(e) => { if (activeSection !== 'trash') (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
          >
            <TrashSolid style={{ width: '16px', height: '16px', flexShrink: 0 }} />
            휴지통
            {trashNodes.length > 0 && (
              <span style={{ marginLeft: 'auto', fontSize: '11px', background: 'var(--color-bg-tertiary)', color: 'var(--color-text-tertiary)', borderRadius: '10px', padding: '1px 6px' }}>{trashNodes.length}</span>
            )}
          </button>
        </div>

        {/* Spacer */}
        <div style={{ flex: 1 }} />

        {/* Storage bar */}
        {usage && (
          <div style={{ padding: '12px 14px', borderTop: '1px solid var(--color-border-subtle)' }}>
            <div style={{ fontSize: '11px', color: 'var(--color-text-tertiary)', marginBottom: '6px', fontWeight: 500 }}>저장공간</div>
            <div style={{ height: '6px', borderRadius: '3px', background: 'var(--color-bg-tertiary)', overflow: 'hidden', marginBottom: '6px' }}>
              <div style={{ height: '100%', borderRadius: '3px', width: `${usedPct}%`, background: barColor, transition: 'width 400ms ease' }} />
            </div>
            <div style={{ fontSize: '11px', color: 'var(--color-text-secondary)', lineHeight: 1.4 }}>
              <span style={{ fontWeight: 500, color: barColor }}>{formatBytes(usage.quota_used)}</span>
              <span style={{ color: 'var(--color-text-tertiary)' }}> / {formatBytes(usage.quota_limit)} 사용 중</span>
            </div>
            {usedPct >= 70 && (
              <div style={{ marginTop: '4px', fontSize: '10px', color: barColor, fontWeight: 500 }}>
                {usedPct >= 90 ? '저장공간이 거의 가득 찼습니다' : '저장공간이 70% 이상 사용됨'}
              </div>
            )}
          </div>
        )}
      </div>

      {/* ── Main content ── */}
      {activeSection === 'trash' ? (
        /* Trash view */
        <div style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column' }}>
          {/* Trash toolbar */}
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px', padding: '12px 20px', borderBottom: '1px solid var(--color-border-subtle)', flexShrink: 0 }}>
            <TrashSolid style={{ width: '18px', height: '18px', color: 'var(--color-text-tertiary)' }} />
            <span style={{ fontSize: '15px', fontWeight: 600, color: 'var(--color-text-primary)', flex: 1 }}>휴지통</span>
            <button onClick={loadTrashNodes} title="새로고침"
              style={{ padding: '5px 8px', borderRadius: '5px', border: '1px solid var(--color-border-default)', background: 'transparent', cursor: 'pointer', color: 'var(--color-text-secondary)', display: 'flex', alignItems: 'center' }}>
              <ArrowPathIcon style={{ width: '15px', height: '15px' }} />
            </button>
            {trashNodes.length > 0 && (
              <button onClick={handleEmptyTrash}
                style={{ display: 'inline-flex', alignItems: 'center', gap: '6px', padding: '5px 14px', borderRadius: '6px', border: '1px solid var(--color-destructive)', background: 'transparent', color: 'var(--color-destructive)', fontSize: '13px', fontWeight: 500, cursor: 'pointer' }}>
                <TrashIcon style={{ width: '14px', height: '14px' }} />
                휴지통 비우기
              </button>
            )}
          </div>

          {/* Trash file list */}
          <div style={{ flex: 1, overflowY: 'auto', padding: '16px 20px' }}>
            {trashLoading ? (
              <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
                {Array.from({ length: 4 }).map((_, i) => (
                  <div key={i} style={{ height: '56px', borderRadius: '8px', background: 'var(--color-bg-secondary)', animation: 'pulse 1.5s ease-in-out infinite' }} />
                ))}
              </div>
            ) : trashNodes.length === 0 ? (
              <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', height: '300px', gap: '12px', color: 'var(--color-text-tertiary)' }}>
                <TrashIcon style={{ width: '48px', height: '48px', opacity: 0.3 }} />
                <div style={{ fontSize: '14px' }}>휴지통이 비어 있습니다</div>
              </div>
            ) : (
              <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
                {trashNodes.map((node) => (
                  <div key={node.id} style={{ display: 'flex', alignItems: 'center', gap: '12px', padding: '10px 14px', borderRadius: '8px', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-primary)' }}
                    onMouseEnter={(e) => { (e.currentTarget as HTMLDivElement).style.background = 'var(--color-bg-secondary)'; }}
                    onMouseLeave={(e) => { (e.currentTarget as HTMLDivElement).style.background = 'var(--color-bg-primary)'; }}
                  >
                    <div style={{ flexShrink: 0 }}><DriveNodeIcon node={node} /></div>
                    <div style={{ flex: 1, minWidth: 0 }}>
                      <div style={{ fontSize: '13px', fontWeight: 500, color: 'var(--color-text-primary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{node.name}</div>
                      <div style={{ fontSize: '11px', color: 'var(--color-text-tertiary)', marginTop: '2px' }}>
                        {node.node_type === 'file' ? formatBytes(node.size) : '폴더'} · {formatDate(node.updated_at)}
                      </div>
                    </div>
                    <button onClick={() => handleRestore(node.id)}
                      style={{ display: 'inline-flex', alignItems: 'center', gap: '5px', padding: '5px 12px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', fontSize: '12px', cursor: 'pointer', flexShrink: 0 }}
                      onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
                      onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
                    >
                      <ArrowUturnLeftIcon style={{ width: '13px', height: '13px' }} />
                      복원
                    </button>
                    <button onClick={() => handlePermanentDelete(node.id)}
                      style={{ display: 'inline-flex', alignItems: 'center', gap: '5px', padding: '5px 12px', borderRadius: '6px', border: '1px solid var(--color-destructive)', background: 'transparent', color: 'var(--color-destructive)', fontSize: '12px', cursor: 'pointer', flexShrink: 0 }}
                      onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = '#fef2f2'; }}
                      onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
                    >
                      <TrashIcon style={{ width: '13px', height: '13px' }} />
                      영구 삭제
                    </button>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>
      ) : (
        /* Drive view */
        <div
          style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', position: 'relative' }}
          onDragOver={(e) => { e.preventDefault(); setDragOver(true); }}
          onDragLeave={(e) => { if (!e.currentTarget.contains(e.relatedTarget as Node)) setDragOver(false); }}
          onDrop={(e) => { e.preventDefault(); setDragOver(false); if (e.dataTransfer.files.length) handleUpload(e.dataTransfer.files); }}
        >
          {dragOver && (
            <div aria-hidden="true" style={{ position: 'absolute', inset: 0, background: 'var(--color-accent-subtle)', border: '2px dashed var(--color-accent)', borderRadius: '4px', zIndex: 100, display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: '16px', fontWeight: 600, color: 'var(--color-accent)', pointerEvents: 'none' }}>
              파일을 여기에 놓으세요
            </div>
          )}

          {/* Toolbar */}
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px', padding: '12px 20px', borderBottom: '1px solid var(--color-border-subtle)', flexShrink: 0 }}>
            {/* Breadcrumb */}
            <div style={{ display: 'flex', alignItems: 'center', gap: '4px', flex: 1, minWidth: 0, overflow: 'hidden' }}>
              {breadcrumb.map((item, i) => (
                <span key={item.id} style={{ display: 'flex', alignItems: 'center', gap: '4px', minWidth: 0 }}>
                  {i > 0 && <ChevronRightIcon style={{ width: '14px', height: '14px', color: 'var(--color-text-tertiary)', flexShrink: 0 }} />}
                  <button
                    onClick={() => navigateTo(item)}
                    style={{
                      background: 'none', border: 'none', cursor: i === breadcrumb.length - 1 ? 'default' : 'pointer',
                      fontSize: '14px', fontWeight: i === breadcrumb.length - 1 ? 600 : 400,
                      color: i === breadcrumb.length - 1 ? 'var(--color-text-primary)' : 'var(--color-text-secondary)',
                      padding: '2px 4px', borderRadius: '4px', maxWidth: '180px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
                    }}
                    onMouseEnter={(e) => { if (i < breadcrumb.length - 1) (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
                    onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'none'; }}
                  >{item.name}</button>
                </span>
              ))}
            </div>

            {/* Actions */}
            <button onClick={() => loadNodes(currentParentId)} title="새로고침"
              style={{ padding: '5px 8px', borderRadius: '5px', border: '1px solid var(--color-border-default)', background: 'transparent', cursor: 'pointer', color: 'var(--color-text-secondary)', display: 'flex', alignItems: 'center' }}>
              <ArrowPathIcon style={{ width: '15px', height: '15px' }} />
            </button>
            <button onClick={() => setNewFolderMode(true)} title="새 폴더"
              style={{ display: 'inline-flex', alignItems: 'center', gap: '6px', padding: '5px 12px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', fontSize: '13px', cursor: 'pointer' }}>
              <FolderPlusIcon style={{ width: '15px', height: '15px' }} /> 새 폴더
            </button>
            <button onClick={() => fileInputRef.current?.click()} disabled={uploading}
              style={{ display: 'inline-flex', alignItems: 'center', gap: '6px', padding: '5px 14px', borderRadius: '6px', border: 'none', background: 'var(--color-accent)', color: '#fff', fontSize: '13px', fontWeight: 500, cursor: uploading ? 'wait' : 'pointer' }}>
              <ArrowUpTrayIcon style={{ width: '15px', height: '15px' }} /> {uploading ? '업로드 중...' : '업로드'}
            </button>
            <input ref={fileInputRef} type="file" multiple style={{ display: 'none' }} onChange={(e) => { if (e.target.files) { handleUpload(e.target.files); e.target.value = ''; } }} />
          </div>

          {/* File grid */}
          <div style={{ flex: 1, overflowY: 'auto', padding: '16px 20px' }}>
            {/* New folder input */}
            {newFolderMode && (
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '12px', padding: '8px 12px', borderRadius: '8px', border: '1px solid var(--color-accent)', background: 'var(--color-accent-subtle)' }}>
                <FolderIcon style={{ width: '20px', height: '20px', color: '#f59e0b', flexShrink: 0 }} />
                <input
                  ref={newFolderRef}
                  value={newFolderName}
                  onChange={(e) => setNewFolderName(e.target.value)}
                  onKeyDown={(e) => { if (e.key === 'Enter') handleCreateFolder(); if (e.key === 'Escape') { setNewFolderMode(false); setNewFolderName(''); } }}
                  placeholder="폴더 이름"
                  style={{ flex: 1, border: 'none', background: 'transparent', outline: 'none', fontSize: '13px', color: 'var(--color-text-primary)' }}
                />
                <button onClick={handleCreateFolder} style={{ padding: '3px 10px', borderRadius: '5px', border: 'none', background: 'var(--color-accent)', color: '#fff', fontSize: '12px', cursor: 'pointer' }}>만들기</button>
                <button onClick={() => { setNewFolderMode(false); setNewFolderName(''); }} style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--color-text-tertiary)', display: 'flex' }}><XMarkIcon style={{ width: '16px', height: '16px' }} /></button>
              </div>
            )}

            {loading ? (
              <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(160px, 1fr))', gap: '12px' }}>
                {Array.from({ length: 8 }).map((_, i) => (
                  <div key={i} style={{ height: '120px', borderRadius: '8px', background: 'var(--color-bg-secondary)', animation: 'pulse 1.5s ease-in-out infinite' }} />
                ))}
              </div>
            ) : nodes.length === 0 && !newFolderMode ? (
              <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', height: '300px', gap: '12px', color: 'var(--color-text-tertiary)' }}>
                <FolderIcon style={{ width: '48px', height: '48px', opacity: 0.4 }} />
                <div style={{ fontSize: '14px' }}>파일이 없습니다</div>
                <div style={{ fontSize: '12px', opacity: 0.8 }}>파일을 드래그하거나 업로드 버튼을 클릭하세요</div>
              </div>
            ) : (
              <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(160px, 1fr))', gap: '12px' }}>
                {nodes.map((node) => {
                  const isRenaming = renameNodeId === node.id;
                  return (
                    <div
                      key={node.id}
                      onDoubleClick={() => openFolder(node)}
                      style={{ position: 'relative', borderRadius: '8px', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-primary)', padding: '14px 12px 10px', cursor: node.node_type === 'folder' ? 'pointer' : 'default', transition: 'background 100ms ease, border-color 100ms ease' }}
                      onMouseEnter={(e) => { (e.currentTarget as HTMLDivElement).style.background = 'var(--color-bg-secondary)'; (e.currentTarget as HTMLDivElement).style.borderColor = 'var(--color-border-default)'; }}
                      onMouseLeave={(e) => { (e.currentTarget as HTMLDivElement).style.background = 'var(--color-bg-primary)'; }}
                    >
                      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: '10px' }}>
                        <DriveNodeIcon node={node} />
                        <div style={{ position: 'relative' }}>
                          <button
                            onClick={(e) => { e.stopPropagation(); setMenuNodeId(menuNodeId === node.id ? null : node.id); }}
                            style={{ background: 'none', border: 'none', cursor: 'pointer', padding: '2px', color: 'var(--color-text-tertiary)', display: 'flex', borderRadius: '4px' }}
                            onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-tertiary)'; }}
                            onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'none'; }}
                          ><EllipsisVerticalIcon style={{ width: '16px', height: '16px' }} /></button>
                          {menuNodeId === node.id && (
                            <NodeMenu
                              node={node}
                              onDownload={() => downloadDriveNode(node.id, node.name).catch(() => {})}
                              onRename={() => { setRenameNodeId(node.id); setRenameName(node.name); }}
                              onShare={() => setShareNode(node)}
                              onTrash={() => handleTrash(node.id)}
                              onClose={() => setMenuNodeId(null)}
                            />
                          )}
                        </div>
                      </div>
                      {isRenaming ? (
                        <input
                          ref={renameRef}
                          value={renameName}
                          onChange={(e) => setRenameName(e.target.value)}
                          onBlur={handleRename}
                          onKeyDown={(e) => { if (e.key === 'Enter') handleRename(); if (e.key === 'Escape') setRenameNodeId(null); }}
                          onClick={(e) => e.stopPropagation()}
                          style={{ width: '100%', border: '1px solid var(--color-accent)', borderRadius: '4px', padding: '2px 6px', fontSize: '12px', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)', outline: 'none', boxSizing: 'border-box' }}
                        />
                      ) : (
                        <div style={{ fontSize: '12px', fontWeight: 500, color: 'var(--color-text-primary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', marginBottom: '4px' }}>{node.name}</div>
                      )}
                      <div style={{ fontSize: '11px', color: 'var(--color-text-tertiary)' }}>
                        {node.node_type === 'file' ? formatBytes(node.size) : '폴더'} · {formatDate(node.updated_at)}
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
          </div>
        </div>
      )}

      {shareNode && <ShareModal node={shareNode} onClose={() => setShareNode(null)} />}
    </div>
  );
}
