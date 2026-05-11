'use client';

import { useState, useEffect, useCallback } from 'react';
import { useRouter } from 'next/navigation';
import { deleteMessage, starMessage, markRead, searchMessages, ComposeIntent, MessageDetail, MessageSummary } from '@/lib/api';
import { useMailList } from '@/hooks/useMailList';
import { useMessage } from '@/hooks/useMessage';
import { Sidebar } from '@/components/Sidebar';
import { MessageList } from '@/components/MessageList';
import { ReadingPane } from '@/components/ReadingPane';
import { ComposeModal } from '@/components/ComposeModal';
import { ThemeToggle } from '@/components/ThemeToggle';
import { LocaleSelector } from '@/components/common/LocaleSelector';

export default function MailPage() {
  const router = useRouter();

  const [activeFolderId, setActiveFolderId] = useState('');
  const [selectedMessageId, setSelectedMessageId] = useState<string | null>(null);
  const [composeContext, setComposeContext] = useState<{
    intent: ComposeIntent;
    source?: MessageDetail;
  } | null>(null);
  const [searchQuery, setSearchQuery] = useState('');
  const [searchResults, setSearchResults] = useState<MessageSummary[] | null>(null);
  const [searchLoading, setSearchLoading] = useState(false);

  const { folders, messages, setMessages, foldersLoading, messagesLoading, hasMore, loadingMore, loadMore, adjustUnread } =
    useMailList(activeFolderId);

  // Set default folder to inbox UUID once folders are loaded
  useEffect(() => {
    if (activeFolderId || folders.length === 0) return;
    const inbox = folders.find((f) => f.system_type === 'inbox') ?? folders[0];
    if (inbox) setActiveFolderId(inbox.id);
  }, [folders, activeFolderId]);

  const { message: selectedMessage, loading: messageLoading } =
    useMessage(selectedMessageId);

  // Check auth on mount
  useEffect(() => {
    const token = localStorage.getItem('webmail_token');
    if (!token) router.push('/login');
  }, [router]);

  // Mark selected message as read locally + update sidebar badge
  useEffect(() => {
    if (!selectedMessageId) return;
    setMessages((prev) => {
      const msg = prev.find((m) => m.id === selectedMessageId);
      if (msg && !msg.read) adjustUnread(activeFolderId, -1);
      return prev.map((m) => (m.id === selectedMessageId ? { ...m, read: true } : m));
    });
  }, [selectedMessageId, setMessages, adjustUnread, activeFolderId]);

  const handleMarkUnread = useCallback(async () => {
    if (!selectedMessageId) return;
    setMessages((prev) =>
      prev.map((m) => (m.id === selectedMessageId ? { ...m, read: false } : m))
    );
    adjustUnread(activeFolderId, 1);
    markRead(selectedMessageId, false).catch(() => {
      setMessages((prev) =>
        prev.map((m) => (m.id === selectedMessageId ? { ...m, read: true } : m))
      );
      adjustUnread(activeFolderId, -1);
    });
  }, [selectedMessageId, setMessages, adjustUnread, activeFolderId]);

  const handleSearch = useCallback(async (q: string) => {
    setSearchQuery(q);
    if (!q.trim()) { setSearchResults(null); return; }
    setSearchLoading(true);
    try {
      const res = await searchMessages({ q: q.trim(), limit: 50 });
      setSearchResults(res.messages ?? []);
    } catch {
      setSearchResults([]);
    } finally {
      setSearchLoading(false);
    }
  }, []);

  const handleSelectFolder = useCallback((id: string) => {
    setActiveFolderId(id);
    setSelectedMessageId(null);
    setSearchResults(null);
    setSearchQuery('');
  }, []);

  const handleSelectMessage = useCallback((id: string) => {
    setSelectedMessageId(id);
  }, []);

  const handleDelete = useCallback(async () => {
    if (!selectedMessageId) return;
    try {
      await deleteMessage(selectedMessageId);
      setMessages((prev) => prev.filter((m) => m.id !== selectedMessageId));
      setSelectedMessageId(null);
    } catch {
      // ignore
    }
  }, [selectedMessageId, setMessages]);

  const handleStar = useCallback(async (id: string, starred: boolean) => {
    setMessages((prev) => prev.map((m) => (m.id === id ? { ...m, starred } : m)));
    starMessage(id, starred).catch(() => {
      setMessages((prev) => prev.map((m) => (m.id === id ? { ...m, starred: !starred } : m)));
    });
  }, [setMessages]);

  // Keyboard shortcuts (skip when typing in input/textarea/contenteditable)
  useEffect(() => {
    function onKeyDown(e: KeyboardEvent) {
      const tag = (e.target as HTMLElement).tagName;
      const editable = (e.target as HTMLElement).isContentEditable;
      if (tag === 'INPUT' || tag === 'TEXTAREA' || editable) return;

      const list = searchResults ?? messages;
      const currentIdx = list.findIndex((m) => m.id === selectedMessageId);

      switch (e.key) {
        case 'j': {
          const next = list[currentIdx + 1];
          if (next) setSelectedMessageId(next.id);
          break;
        }
        case 'k': {
          const prev = list[currentIdx - 1];
          if (prev) setSelectedMessageId(prev.id);
          break;
        }
        case 'r':
          if (selectedMessage && !composeContext) {
            e.preventDefault();
            setComposeContext({ intent: 'reply', source: selectedMessage });
          }
          break;
        case 'f':
          if (selectedMessage && !composeContext) {
            e.preventDefault();
            setComposeContext({ intent: 'forward', source: selectedMessage });
          }
          break;
        case '#':
        case 'Delete':
          if (selectedMessageId && !composeContext) handleDelete();
          break;
        case 'Escape':
          if (composeContext) setComposeContext(null);
          else setSelectedMessageId(null);
          break;
      }
    }
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [messages, searchResults, selectedMessageId, selectedMessage, composeContext, handleDelete]);

  if (foldersLoading) {
    return (
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          height: '100vh',
          background: 'var(--color-bg-primary)',
          color: 'var(--color-text-tertiary)',
          fontSize: '14px',
          gap: '10px',
        }}
      >
        <svg
          width="20"
          height="20"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
          aria-hidden="true"
          style={{ animation: 'spin 1s linear infinite' }}
        >
          <path d="M21 12a9 9 0 1 1-6.219-8.56" />
        </svg>
        <style>{`@keyframes spin { to { transform: rotate(360deg); } }`}</style>
        로딩 중...
      </div>
    );
  }

  return (
    <div
      style={{
        display: 'flex',
        height: '100vh',
        overflow: 'hidden',
        background: 'var(--color-bg-primary)',
      }}
    >
      <Sidebar
        folders={folders}
        activeFolderId={activeFolderId}
        onSelectFolder={handleSelectFolder}
        onCompose={() => setComposeContext({ intent: 'new' })}
        onSearch={handleSearch}
        searchQuery={searchQuery}
      />

      <MessageList
        messages={searchResults ?? messages}
        selectedId={selectedMessageId}
        onSelect={handleSelectMessage}
        loading={searchResults !== null ? searchLoading : messagesLoading}
        emptyLabel={searchQuery ? `"${searchQuery}" 검색 결과가 없습니다` : undefined}
        hasMore={searchResults === null ? hasMore : false}
        loadingMore={loadingMore}
        onLoadMore={loadMore}
        onStar={handleStar}
      />

      <ReadingPane
        message={selectedMessage}
        onDelete={handleDelete}
        onReply={() => selectedMessage && setComposeContext({ intent: 'reply', source: selectedMessage })}
        onReplyAll={() => selectedMessage && setComposeContext({ intent: 'reply_all', source: selectedMessage })}
        onForward={() => selectedMessage && setComposeContext({ intent: 'forward', source: selectedMessage })}
        onMarkUnread={handleMarkUnread}
        loading={messageLoading}
      />

      {composeContext && (
        <ComposeModal
          intent={composeContext.intent}
          sourceMessage={composeContext.source}
          onClose={() => setComposeContext(null)}
        />
      )}

      {/* Controls: locale + theme, top-right */}
      <div
        style={{
          position: 'fixed',
          top: '14px',
          right: '16px',
          zIndex: 50,
          display: 'flex',
          alignItems: 'center',
          gap: '8px',
        }}
      >
        <LocaleSelector />
        <ThemeToggle inline />
      </div>
    </div>
  );
}
