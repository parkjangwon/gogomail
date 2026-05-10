'use client';

import { useState, useEffect, useCallback } from 'react';
import { useRouter } from 'next/navigation';
import {
  Folder,
  MessageSummary,
  MessageDetail,
  getFolders,
  getMessages,
  getMessage,
  markRead,
  deleteMessage,
} from '@/lib/api';
import { Sidebar } from '@/components/Sidebar';
import { MessageList } from '@/components/MessageList';
import { ReadingPane } from '@/components/ReadingPane';
import { ComposeModal } from '@/components/ComposeModal';
import { ThemeToggle } from '@/components/ThemeToggle';

export default function MailPage() {
  const router = useRouter();

  const [folders, setFolders] = useState<Folder[]>([]);
  const [activeFolderId, setActiveFolderId] = useState('inbox');
  const [messages, setMessages] = useState<MessageSummary[]>([]);
  const [selectedMessageId, setSelectedMessageId] = useState<string | null>(null);
  const [selectedMessage, setSelectedMessage] = useState<MessageDetail | null>(null);
  const [showCompose, setShowCompose] = useState(false);

  const [foldersLoading, setFoldersLoading] = useState(true);
  const [messagesLoading, setMessagesLoading] = useState(false);
  const [messageLoading, setMessageLoading] = useState(false);

  // Check auth on mount
  useEffect(() => {
    const token = localStorage.getItem('webmail_token');
    if (!token) {
      router.push('/login');
    }
  }, [router]);

  // Load folders on mount
  useEffect(() => {
    let cancelled = false;
    setFoldersLoading(true);
    getFolders()
      .then((data) => {
        if (!cancelled) setFolders(data.folders);
      })
      .catch(() => {
        // folders optional — continue with empty list
      })
      .finally(() => {
        if (!cancelled) setFoldersLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  // Load messages when folder changes
  useEffect(() => {
    let cancelled = false;
    setMessages([]);
    setSelectedMessageId(null);
    setSelectedMessage(null);
    setMessagesLoading(true);
    getMessages(activeFolderId)
      .then((data) => {
        if (!cancelled) setMessages(data.messages);
      })
      .catch(() => {
        if (!cancelled) setMessages([]);
      })
      .finally(() => {
        if (!cancelled) setMessagesLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [activeFolderId]);

  // Load full message when selection changes
  useEffect(() => {
    if (!selectedMessageId) {
      setSelectedMessage(null);
      return;
    }
    let cancelled = false;
    setMessageLoading(true);
    getMessage(selectedMessageId)
      .then((data) => {
        if (!cancelled) {
          setSelectedMessage(data);
          // Mark as read locally
          setMessages((prev) =>
            prev.map((m) =>
              m.id === selectedMessageId ? { ...m, is_read: true } : m
            )
          );
        }
      })
      .catch(() => {
        if (!cancelled) setSelectedMessage(null);
      })
      .finally(() => {
        if (!cancelled) setMessageLoading(false);
      });
    // Also mark read on server (fire and forget)
    markRead(selectedMessageId, true).catch(() => undefined);
    return () => {
      cancelled = true;
    };
  }, [selectedMessageId]);

  const handleSelectFolder = useCallback((id: string) => {
    setActiveFolderId(id);
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
      setSelectedMessage(null);
    } catch {
      // ignore
    }
  }, [selectedMessageId]);

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
        onCompose={() => setShowCompose(true)}
      />

      <MessageList
        messages={messages}
        selectedId={selectedMessageId}
        onSelect={handleSelectMessage}
        loading={messagesLoading}
      />

      <ReadingPane
        message={selectedMessage}
        onDelete={handleDelete}
        loading={messageLoading}
      />

      {showCompose && (
        <ComposeModal onClose={() => setShowCompose(false)} />
      )}
      <ThemeToggle />
    </div>
  );
}
