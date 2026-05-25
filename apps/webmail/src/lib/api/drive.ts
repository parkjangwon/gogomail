import { responseErrorMessage } from './http';

export interface DriveNode {
  id: string;
  node_type: 'file' | 'folder';
  name: string;
  mime_type?: string;
  size: number;
  status: string;
  parent_id?: string;
  created_at: string;
  updated_at: string;
}

export interface DriveUsage {
  quota_used: number;
  quota_limit: number;
  active_bytes: number;
  trashed_bytes: number;
  folder_count: number;
  file_count: number;
}

export interface DriveShareLink {
  id: string;
  node_id: string;
  token?: string;
  token_suffix: string;
  permission?: string;
  password_protected?: boolean;
  expires_at: string;
}

export interface DriveUploadSession {
  id: string;
  user_id: string;
  parent_id?: string;
  upload_id: string;
  name: string;
  declared_size: number;
  received_size: number;
  mime_type: string;
  status: 'pending' | 'uploading' | 'finalized' | 'canceled' | 'expired' | 'failed';
  storage_backend: string;
  storage_path?: string;
  checksum_sha256?: string;
  expires_at: string;
  created_at: string;
  updated_at: string;
  finalized_at?: string;
  canceled_at?: string;
}

export interface DriveUploadProgress {
  phase: 'creating_session' | 'uploading' | 'finalizing';
  uploadedBytes: number;
  totalBytes: number;
  sessionId?: string;
  storageBackend?: string;
}

export interface DriveUploadOptions {
  parentId?: string;
  resumable?: boolean;
  resumeSessionId?: string;
  chunkSizeBytes?: number;
  signal?: AbortSignal;
  onProgress?: (progress: DriveUploadProgress) => void;
}

export interface DriveUploadCapabilities {
  upload_sessions: boolean;
  list_upload_sessions: boolean;
  upload_session_body: boolean;
  upload_session_checksum: boolean;
  finalize_upload_sessions: boolean;
  cancel_upload_sessions: boolean;
  resumable_chunked_uploads: boolean;
  max_upload_session_bytes: number;
  max_session_ttl_seconds: number;
  default_session_ttl_seconds: number;
}

async function getDriveUploadSession(sessionId: string): Promise<DriveUploadSession | null> {
  try {
    const res = await fetch(`/api/mail/drive/upload-sessions/${encodeURIComponent(sessionId)}`);
    if (!res.ok) return null;
    const data = await res.json() as { drive_upload_session?: DriveUploadSession };
    return data.drive_upload_session ?? null;
  } catch {
    return null;
  }
}

export async function cancelDriveUploadSession(sessionId: string): Promise<boolean> {
  try {
    const res = await fetch(`/api/mail/drive/upload-sessions/${encodeURIComponent(sessionId)}`, {
      method: 'DELETE',
    });
    return res.ok;
  } catch {
    return false;
  }
}

async function createDriveUploadSession(
  file: File,
  parentId: string | undefined,
  storageBackends: string[],
  signal?: AbortSignal,
): Promise<DriveUploadSession> {
  for (const storageBackend of storageBackends) {
    const sessionRes = await fetch('/api/mail/drive/upload-sessions', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        parent_id: parentId ?? '',
        name: file.name,
        declared_size: file.size,
        mime_type: file.type || 'application/octet-stream',
        ...(storageBackend ? { storage_backend: storageBackend } : {}),
      }),
      signal,
    });

    if (sessionRes.ok) {
      const body = await sessionRes.json() as { drive_upload_session?: DriveUploadSession };
      if (body.drive_upload_session) return body.drive_upload_session;
    }

    const shouldRetryBackend = storageBackend !== storageBackends[storageBackends.length - 1];
    if (!shouldRetryBackend) {
      throw new Error(await responseErrorMessage(sessionRes, `Create upload session failed: ${sessionRes.status}`));
    }
  }

  throw new Error('Create upload session failed.');
}

async function storeDriveUploadSessionChunk(
  sessionId: string,
  chunk: Blob,
  start: number,
  end: number,
  total: number,
  signal?: AbortSignal,
): Promise<DriveUploadSession> {
  const res = await fetch(`/api/mail/drive/upload-sessions/${encodeURIComponent(sessionId)}/body`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/octet-stream',
      'Content-Range': `bytes ${start}-${end}/${total}`,
    },
    body: chunk,
    signal,
  });
  if (!res.ok) {
    throw new Error(await responseErrorMessage(res, `Upload body failed: ${res.status}`));
  }
  const data = await res.json() as { drive_upload_session?: DriveUploadSession };
  if (!data.drive_upload_session) {
    throw new Error('Upload body failed: missing session response');
  }
  return data.drive_upload_session;
}

async function finalizeDriveUploadSession(sessionId: string): Promise<DriveNode | null> {
  const res = await fetch(`/api/mail/drive/upload-sessions/${encodeURIComponent(sessionId)}/finalize`, {
    method: 'POST',
  });
  if (!res.ok) {
    throw new Error(await responseErrorMessage(res, `Finalize upload session failed: ${res.status}`));
  }
  const data = await res.json() as { drive_node?: DriveNode };
  return data.drive_node ?? null;
}

export async function uploadDriveFileWithOptions(file: File, options: DriveUploadOptions = {}): Promise<DriveNode | null> {
  const resumable = options.resumable ?? true;
  const chunkSize = Math.max(1 << 20, options.chunkSizeBytes ?? (8 << 20));
  const storageBackends = ['', 'minio', 's3', 'local'];
  const totalBytes = file.size;
  const signal = options.signal;
  const emitProgress = (phase: DriveUploadProgress['phase'], uploadedBytes: number, sessionId?: string, storageBackend?: string) => {
    options.onProgress?.({ phase, uploadedBytes, totalBytes, sessionId, storageBackend });
  };

  const isAborted = () => signal?.aborted ?? false;

  if (!resumable || totalBytes <= 0) {
    const session = await createDriveUploadSession(file, options.parentId, storageBackends, signal);
    emitProgress('creating_session', 0, session.id, session.storage_backend);
    if (isAborted()) throw new DOMException('Upload aborted', 'AbortError');
    const bodyRes = await fetch(`/api/mail/drive/upload-sessions/${encodeURIComponent(session.id)}/body`, {
      method: 'PUT',
      headers: {
        'Content-Type': file.type || 'application/octet-stream',
      },
      body: file,
      signal,
    });
    if (!bodyRes.ok) {
      throw new Error(await responseErrorMessage(bodyRes, `Upload body failed: ${bodyRes.status}`));
    }
    emitProgress('uploading', totalBytes, session.id, session.storage_backend);
    emitProgress('finalizing', totalBytes, session.id, session.storage_backend);
    return finalizeDriveUploadSession(session.id);
  }

  let session: DriveUploadSession | null = null;
  let uploadedBytes = 0;
  let storageBackend = '';

  if (options.resumeSessionId) {
    const resumed = await getDriveUploadSession(options.resumeSessionId);
    if (resumed && (resumed.status === 'pending' || resumed.status === 'uploading' || resumed.status === 'failed')) {
      session = resumed;
      uploadedBytes = Math.max(0, Math.min(resumed.received_size, totalBytes));
      storageBackend = resumed.storage_backend;
      emitProgress('uploading', uploadedBytes, session.id, storageBackend);
    }
  }

  if (!session) {
    session = await createDriveUploadSession(file, options.parentId, storageBackends, signal);
    uploadedBytes = 0;
    storageBackend = session.storage_backend;
    emitProgress('creating_session', uploadedBytes, session.id, storageBackend);
  }

  if (isAborted()) throw new DOMException('Upload aborted', 'AbortError');

  while (uploadedBytes < totalBytes) {
    if (isAborted()) throw new DOMException('Upload aborted', 'AbortError');
    const chunkEnd = Math.min(totalBytes, uploadedBytes + chunkSize);
    const chunk = file.slice(uploadedBytes, chunkEnd);
    const sentSession = await storeDriveUploadSessionChunk(session.id, chunk, uploadedBytes, chunkEnd - 1, totalBytes, signal);
    session = sentSession;
    if (sentSession.received_size !== chunkEnd) {
      throw new Error(
        `Upload session progress mismatch: server recorded ${sentSession.received_size} bytes after chunk ending at ${chunkEnd}`,
      );
    }
    uploadedBytes = sentSession.received_size;
    storageBackend = sentSession.storage_backend;
    emitProgress('uploading', uploadedBytes, session.id, storageBackend);
  }

  if (isAborted()) throw new DOMException('Upload aborted', 'AbortError');
  emitProgress('finalizing', uploadedBytes, session.id, storageBackend);
  return finalizeDriveUploadSession(session.id);
}

export async function uploadDriveFile(file: File, parentId?: string): Promise<DriveNode | null> {
  return uploadDriveFileWithOptions(file, { parentId, resumable: false });
}

export async function listDriveNodes(parentId?: string): Promise<DriveNode[]> {
  try {
    const p = new URLSearchParams({ status: 'active' });
    if (parentId) p.set('parent_id', parentId);
    const res = await fetch(`/api/mail/drive/nodes?${p}`);
    if (!res.ok) return [];
    const data = await res.json() as { drive_nodes?: DriveNode[] };
    return data.drive_nodes ?? [];
  } catch { return []; }
}

export async function listTrashedDriveNodes(): Promise<DriveNode[]> {
  try {
    const p = new URLSearchParams({ status: 'trashed' });
    const res = await fetch(`/api/mail/drive/nodes?${p}`);
    if (!res.ok) return [];
    const data = await res.json() as { drive_nodes?: DriveNode[] };
    return data.drive_nodes ?? [];
  } catch { return []; }
}

export async function deleteDriveNodePermanently(nodeId: string): Promise<boolean> {
  try {
    const res = await fetch(`/api/mail/drive/nodes/${encodeURIComponent(nodeId)}`, {
      method: 'DELETE',
    });
    return res.ok;
  } catch { return false; }
}

export async function getDriveUsage(): Promise<DriveUsage | null> {
  try {
    const res = await fetch('/api/mail/drive/usage');
    if (!res.ok) return null;
    const data = await res.json() as { usage?: DriveUsage };
    return data.usage ?? null;
  } catch { return null; }
}

export async function createDriveFolder(name: string, parentId?: string): Promise<DriveNode | null> {
  try {
    const res = await fetch('/api/mail/drive/folders', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name, parent_id: parentId ?? '' }),
    });
    if (!res.ok) return null;
    const data = await res.json() as { drive_node?: DriveNode };
    return data.drive_node ?? null;
  } catch { return null; }
}

export async function renameDriveNode(nodeId: string, name: string): Promise<boolean> {
  try {
    const res = await fetch(`/api/mail/drive/nodes/${encodeURIComponent(nodeId)}/name`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name }),
    });
    return res.ok;
  } catch { return false; }
}

export async function moveDriveNode(nodeId: string, parentId: string): Promise<boolean> {
  try {
    const res = await fetch(`/api/mail/drive/nodes/${encodeURIComponent(nodeId)}/parent`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ parent_id: parentId }),
    });
    return res.ok;
  } catch { return false; }
}

export async function trashDriveNode(nodeId: string): Promise<boolean> {
  try {
    const res = await fetch(`/api/mail/drive/nodes/${encodeURIComponent(nodeId)}/trash`, {
      method: 'POST',
    });
    return res.ok;
  } catch { return false; }
}

export async function restoreDriveNode(nodeId: string): Promise<boolean> {
  try {
    const res = await fetch(`/api/mail/drive/nodes/${encodeURIComponent(nodeId)}/restore`, {
      method: 'POST',
    });
    return res.ok;
  } catch { return false; }
}

export async function downloadDriveNode(nodeId: string, filename: string): Promise<void> {
  const res = await fetch(`/api/mail/drive/nodes/${encodeURIComponent(nodeId)}/download`);
  if (!res.ok) throw new Error('Download failed');
  const blob = await res.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url; a.download = filename; a.click();
  URL.revokeObjectURL(url);
}

export async function createDriveShareLink(nodeId: string, expiresAt: string, password = ''): Promise<DriveShareLink | null> {
  try {
    const res = await fetch(`/api/mail/drive/nodes/${encodeURIComponent(nodeId)}/share-links`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ permission: 'download', expires_at: expiresAt, ...(password.trim() ? { password } : {}) }),
    });
    if (!res.ok) return null;
    const data = await res.json() as { drive_share_link?: DriveShareLink };
    return data.drive_share_link ?? null;
  } catch { return null; }
}

export async function listDriveShareLinks(nodeId: string): Promise<DriveShareLink[]> {
  try {
    const res = await fetch(`/api/mail/drive/share-links?node_id=${encodeURIComponent(nodeId)}`);
    if (!res.ok) return [];
    const data = await res.json() as { drive_share_links?: DriveShareLink[] };
    return data.drive_share_links ?? [];
  } catch { return []; }
}

export async function revokeDriveShareLink(linkId: string): Promise<boolean> {
  try {
    const res = await fetch(`/api/mail/drive/share-links/${encodeURIComponent(linkId)}`, {
      method: 'DELETE',
    });
    return res.ok;
  } catch { return false; }
}
