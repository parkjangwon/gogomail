// IndexedDB-backed offline cache for mail reading.
//
// Purpose: when the network is unavailable, recently viewed folders, the
// first page of each folder's message list, and opened message bodies can
// still be rendered. The cache is written through on every successful GET and
// read through only when the network fails, so online users always see live
// data and the cache never masks server-side changes.
//
// Keys are namespaced per key-value store below. Entries carry a timestamp so
// callers can enforce a soft TTL and so we can evict the oldest message
// details when the cache grows large.

const DB_NAME = 'gogomail-offline';
const DB_VERSION = 1;
const STORE_KV = 'kv'; // folders + message lists keyed by string
const STORE_MESSAGES = 'messages'; // full message detail keyed by id

// Keep at most this many message detail bodies cached (LRU by cachedAt).
const MAX_CACHED_MESSAGES = 200;

interface CachedEnvelope<T> {
  key: string;
  value: T;
  cachedAt: number;
}

function isIndexedDBAvailable(): boolean {
  try {
    return typeof indexedDB !== 'undefined';
  } catch {
    return false;
  }
}

let dbPromise: Promise<IDBDatabase | null> | null = null;

function openDB(): Promise<IDBDatabase | null> {
  if (!isIndexedDBAvailable()) return Promise.resolve(null);
  if (dbPromise) return dbPromise;
  dbPromise = new Promise((resolve) => {
    let req: IDBOpenDBRequest;
    try {
      req = indexedDB.open(DB_NAME, DB_VERSION);
    } catch {
      resolve(null);
      return;
    }
    req.onupgradeneeded = () => {
      const db = req.result;
      if (!db.objectStoreNames.contains(STORE_KV)) {
        db.createObjectStore(STORE_KV, { keyPath: 'key' });
      }
      if (!db.objectStoreNames.contains(STORE_MESSAGES)) {
        const store = db.createObjectStore(STORE_MESSAGES, { keyPath: 'key' });
        store.createIndex('cachedAt', 'cachedAt');
      }
    };
    req.onsuccess = () => resolve(req.result);
    req.onerror = () => resolve(null);
  });
  return dbPromise;
}

function tx(db: IDBDatabase, store: string, mode: IDBTransactionMode): IDBObjectStore {
  return db.transaction(store, mode).objectStore(store);
}

async function put<T>(store: string, key: string, value: T): Promise<void> {
  const db = await openDB();
  if (!db) return;
  await new Promise<void>((resolve) => {
    try {
      const os = tx(db, store, 'readwrite');
      const envelope: CachedEnvelope<T> = { key, value, cachedAt: Date.now() };
      const req = os.put(envelope);
      req.onsuccess = () => resolve();
      req.onerror = () => resolve();
    } catch {
      resolve();
    }
  });
}

async function get<T>(store: string, key: string): Promise<CachedEnvelope<T> | null> {
  const db = await openDB();
  if (!db) return null;
  return new Promise((resolve) => {
    try {
      const os = tx(db, store, 'readonly');
      const req = os.get(key);
      req.onsuccess = () => resolve((req.result as CachedEnvelope<T> | undefined) ?? null);
      req.onerror = () => resolve(null);
    } catch {
      resolve(null);
    }
  });
}

async function evictOldMessages(): Promise<void> {
  const db = await openDB();
  if (!db) return;
  await new Promise<void>((resolve) => {
    try {
      const os = tx(db, STORE_MESSAGES, 'readwrite');
      const countReq = os.count();
      countReq.onsuccess = () => {
        const excess = countReq.result - MAX_CACHED_MESSAGES;
        if (excess <= 0) { resolve(); return; }
        const index = os.index('cachedAt');
        let removed = 0;
        const cursorReq = index.openCursor();
        cursorReq.onsuccess = () => {
          const cursor = cursorReq.result;
          if (cursor && removed < excess) {
            cursor.delete();
            removed += 1;
            cursor.continue();
          } else {
            resolve();
          }
        };
        cursorReq.onerror = () => resolve();
      };
      countReq.onerror = () => resolve();
    } catch {
      resolve();
    }
  });
}

// ─── Public API ──────────────────────────────────────────────────────────────

const FOLDERS_KEY = 'folders';
const messageListKey = (folderId: string): string => `messages:${folderId || '__all__'}`;

export const offlineCache = {
  async saveFolders<T>(value: T): Promise<void> {
    await put(STORE_KV, FOLDERS_KEY, value);
  },
  async readFolders<T>(): Promise<T | null> {
    const entry = await get<T>(STORE_KV, FOLDERS_KEY);
    return entry?.value ?? null;
  },
  async saveMessageList<T>(folderId: string, value: T): Promise<void> {
    await put(STORE_KV, messageListKey(folderId), value);
  },
  async readMessageList<T>(folderId: string): Promise<T | null> {
    const entry = await get<T>(STORE_KV, messageListKey(folderId));
    return entry?.value ?? null;
  },
  async saveMessage<T>(id: string, value: T): Promise<void> {
    await put(STORE_MESSAGES, id, value);
    void evictOldMessages();
  },
  async readMessage<T>(id: string): Promise<T | null> {
    const entry = await get<T>(STORE_MESSAGES, id);
    return entry?.value ?? null;
  },
};

// Heuristic: treat network/fetch failures (offline) distinctly from HTTP
// errors so we only fall back to cache when we genuinely could not reach the
// server. The request() helper throws Error('Request failed: <status>') for
// HTTP errors and a generic fetch error (TypeError / AbortError) when offline.
export function isLikelyOfflineError(err: unknown): boolean {
  if (typeof navigator !== 'undefined' && navigator.onLine === false) return true;
  if (err instanceof Error) {
    const msg = err.message.toLowerCase();
    if (msg.includes('failed to fetch') || msg.includes('networkerror') || msg.includes('load failed')) return true;
    if (err.name === 'AbortError' || err.name === 'TypeError') return true;
    // 503 from the proxy means backend unreachable — treat as offline for reads.
    if (msg.includes('request failed: 503')) return true;
  }
  return false;
}
