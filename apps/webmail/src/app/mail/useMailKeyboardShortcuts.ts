'use client';

import { useEffect, type MutableRefObject } from 'react';
import { type MessageSummary, type MessageDetail, type Folder } from '@/lib/api';
import { type AppId } from '@/components/AppIconBar';
import { type ComposeContext } from './useMailCompose';
import { type ToastItem } from '@/components/Toast';
import { VIRTUAL_IMPORTANT } from '@/components/Sidebar';
import { moveMailPanelFocus } from './mailPageHelpers';

export interface UseMailKeyboardShortcutsParams {
  messages: MessageSummary[];
  searchResults: MessageSummary[] | null;
  selectedMessageId: string | null;
  selectedMessage: MessageDetail | null;
  composeContext: ComposeContext | null;
  showShortcuts: boolean;
  showSpotlight: boolean;
  activeApp: AppId;
  activeFolderSystemType: string | undefined;
  folders: Folder[];
  isMobile: boolean;
  messageLabels: Record<string, string>;
  importantIds: Set<string>;
  gPrefixRef: MutableRefObject<boolean>;
  handleDelete: () => void;
  handleArchive: () => void;
  handleSpam: () => void;
  handleMarkRead: () => void;
  handleMarkUnread: () => void;
  handleStar: (id: string, starred: boolean) => void;
  handleMove: (folderId: string) => void;
  handlePin: (id: string) => void;
  handleImportant: (id: string) => void;
  handleSnooze: (id: string, until: Date) => void;
  setLabel: (id: string, color: string | null) => void;
  openCompose: (opts: ComposeContext) => void;
  setSelectedMessageId: (id: string | null) => void;
  setActiveApp: (app: AppId) => void;
  setShowSpotlight: (v: boolean) => void;
  setSpotlightMoveId: (id: string | null) => void;
  setShowDMModal: (v: boolean | ((p: boolean) => boolean)) => void;
  setShowShortcuts: (v: boolean | ((p: boolean) => boolean)) => void;
  setSidebarCollapsed: (v: boolean | ((p: boolean) => boolean)) => void;
  handleSelectFolder: (id: string) => void;
  handleGlobalEscape: () => boolean;
  addToast: (msg: string, type?: ToastItem['type']) => void;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  t: (key: string, values?: Record<string, any>) => string;
}

export function useMailKeyboardShortcuts(params: UseMailKeyboardShortcutsParams) {
  const {
    messages,
    searchResults,
    selectedMessageId,
    selectedMessage,
    composeContext,
    showShortcuts,
    showSpotlight,
    activeApp,
    activeFolderSystemType,
    folders,
    isMobile,
    messageLabels,
    importantIds,
    gPrefixRef,
    handleDelete,
    handleArchive,
    handleSpam,
    handleMarkRead,
    handleMarkUnread,
    handleStar,
    handleMove,
    handlePin,
    handleImportant,
    handleSnooze,
    setLabel,
    openCompose,
    setSelectedMessageId,
    setActiveApp,
    setShowSpotlight,
    setSpotlightMoveId,
    setShowDMModal,
    setShowShortcuts,
    setSidebarCollapsed,
    handleSelectFolder,
    handleGlobalEscape,
    addToast,
    t,
  } = params;

  useEffect(() => {
    // Korean QWERTY → Latin normalization (allows shortcuts to work in Korean IME mode)
    const KO: Record<string, string> = {
      'ㄷ':'e','ㄱ':'r','ㅅ':'t','ㅛ':'y','ㅕ':'u','ㅑ':'i','ㅐ':'o','ㅔ':'p',
      'ㅁ':'a','ㄴ':'s','ㅇ':'d','ㄹ':'f','ㅎ':'g','ㅗ':'h','ㅓ':'j','ㅏ':'k','ㅣ':'l',
      'ㅋ':'z','ㅌ':'x','ㅊ':'c','ㅍ':'v','ㅠ':'b','ㅜ':'n','ㅡ':'m',
      'ㅂ':'q','ㅈ':'w',
    };
    function onKeyDown(e: KeyboardEvent) {
      if (e.key === 'Escape') {
        if (handleGlobalEscape()) {
          e.preventDefault();
          e.stopPropagation();
          e.stopImmediatePropagation();
        }
        return;
      }

      const tag = (e.target as HTMLElement).tagName;
      const editable = (e.target as HTMLElement).isContentEditable;
      if (tag === 'INPUT' || tag === 'TEXTAREA' || editable) return;

      const key = KO[e.key] ?? e.key;
      const list = searchResults ?? messages;
      const currentIdx = list.findIndex((m) => m.id === selectedMessageId);
      const isMailApp = activeApp === 'mail';

      // g+key two-key folder/app navigation
      if (gPrefixRef.current) {
        gPrefixRef.current = false;
        if (key === 'u') {
          e.preventDefault();
          const firstUnread = list.find((m) => !m.read);
          if (firstUnread) setSelectedMessageId(firstUnread.id);
          return;
        }
        const virtualFolderMap: Record<string, string> = { x: VIRTUAL_IMPORTANT };
        if (virtualFolderMap[key]) { e.preventDefault(); handleSelectFolder(virtualFolderMap[key]); return; }
        const systemTypeMap: Record<string, string> = { i: 'inbox', s: 'sent', t: 'trash', a: 'archive', p: 'spam' };
        const target = systemTypeMap[key];
        if (target) {
          const folder = folders.find((f) => f.system_type === target);
          if (folder) { e.preventDefault(); handleSelectFolder(folder.id); return; }
        }
        if (key === 'h') {
          e.preventDefault();
          setShowDMModal(true);
          return;
        }
        const appSwitchMap: Record<string, AppId> = { m: 'mail', c: 'calendar', a: 'contacts', k: 'contacts', d: 'drive', v: 'drive', ',': 'settings' };
        const appTarget = appSwitchMap[key];
        if (appTarget) { e.preventDefault(); setActiveApp(appTarget); return; }
      }

      if ((key === '`' || e.code === 'Backquote') && !e.ctrlKey && !e.metaKey && !e.altKey) {
        e.preventDefault();
        setShowDMModal(true);
        return;
      }

      if (key === 'g' && !e.ctrlKey && !e.metaKey && !e.altKey) {
        e.preventDefault();
        gPrefixRef.current = true;
        setTimeout(() => { gPrefixRef.current = false; }, 1000);
        return;
      }

      if (key === 's' && !e.ctrlKey && !e.metaKey && !e.altKey && !composeContext) {
        e.preventDefault();
        openCompose({ intent: 'new' });
        return;
      }

      if (!isMailApp) {
        switch (key) {
          case '?':
            setShowShortcuts((v) => !v);
            return;
          case '[':
            setSidebarCollapsed((v) => !v);
            return;
          case 'k':
            if (e.ctrlKey || e.metaKey) {
              e.preventDefault();
              setShowSpotlight(true);
            }
            return;
          case 'b':
            e.preventDefault();
            window.dispatchEvent(new CustomEvent('toggleNotificationCenter'));
            return;
          default:
            return;
        }
      }

      switch (key) {
        case 'ArrowRight':
          if (moveMailPanelFocus('next', !!selectedMessageId && !isMobile)) e.preventDefault();
          return;
        case 'ArrowLeft':
          if (moveMailPanelFocus('prev', !!selectedMessageId && !isMobile)) e.preventDefault();
          return;
        case 'j': {
          const next = list[currentIdx + 1];
          if (next) setSelectedMessageId(next.id);
          break;
        }
        case 'k': {
          if (e.ctrlKey || e.metaKey) {
            e.preventDefault();
            setShowSpotlight(true);
          } else {
            const prev = list[currentIdx - 1];
            if (prev) setSelectedMessageId(prev.id);
          }
          break;
        }
        case 'n': {
          // Next unread message
          const nextUnread = list.slice(currentIdx + 1).find((m) => !m.read);
          if (nextUnread) setSelectedMessageId(nextUnread.id);
          break;
        }
        case 'N': {
          // Prev unread message (Shift+n)
          const prevUnread = list.slice(0, currentIdx).reverse().find((m) => !m.read);
          if (prevUnread) setSelectedMessageId(prevUnread.id);
          break;
        }
        case 'u':
          if (selectedMessageId && !composeContext) handleMarkUnread();
          break;
        case 'm':
          if (selectedMessageId && !composeContext) void handleMarkRead();
          break;
        case 'M': // Shift+m
          if (selectedMessageId && !composeContext) void handleMarkUnread();
          break;
        case '!':
          if (selectedMessageId && !composeContext) handleSpam();
          break;
        case 'r':
          if (selectedMessage && !composeContext) {
            e.preventDefault();
            openCompose({ intent: 'reply', source: selectedMessage });
          }
          break;
        case 'a':
          if (selectedMessage && !composeContext) {
            e.preventDefault();
            openCompose({ intent: 'reply_all', source: selectedMessage });
          }
          break;
        case 'f':
          if (selectedMessage && !composeContext) {
            e.preventDefault();
            openCompose({ intent: 'forward', source: selectedMessage });
          }
          break;
        case 'e': {
          if (selectedMessageId && !composeContext) handleArchive();
          break;
        }
        case 'l': {
          if (selectedMessageId && !composeContext) {
            const colors = ['#ef4444','#f97316','#eab308','#22c55e','#3b82f6','#a855f7'];
            const current = messageLabels[selectedMessageId];
            const currentIdx = current ? colors.indexOf(current) : -1;
            if (currentIdx === colors.length - 1) setLabel(selectedMessageId, null);
            else setLabel(selectedMessageId, colors[currentIdx + 1]);
          }
          break;
        }
        case 'z': {
          if (selectedMessageId && !composeContext && activeFolderSystemType !== 'trash') {
            handleSnooze(selectedMessageId, new Date(Date.now() + 60 * 60 * 1000));
          }
          break;
        }
        case 'p': {
          if (selectedMessageId && !composeContext) handlePin(selectedMessageId);
          break;
        }
        case 'i': {
          if (selectedMessageId && !composeContext) {
            handleImportant(selectedMessageId);
            addToast(importantIds.has(selectedMessageId) ? t('misc.mailPage.importantUnmarked') : t('misc.mailPage.importantMarked'), 'info');
          }
          break;
        }
        case '#':
        case 'Delete':
          if (selectedMessageId && !composeContext) handleDelete();
          break;
        case 'o': {
          if (selectedMessageId && !composeContext) {
            if (!selectedMessageId) {
              const first = list[0];
              if (first) setSelectedMessageId(first.id);
            }
          } else if (!selectedMessageId && !composeContext) {
            const first = list[0];
            if (first) setSelectedMessageId(first.id);
          }
          break;
        }
        case 'v': {
          if (selectedMessageId && !composeContext) {
            e.preventDefault();
            setShowSpotlight(true);
            setSpotlightMoveId(selectedMessageId);
          }
          break;
        }
        case 'b':
          // Toggle Notification Center via custom event (state lives in NotificationBell)
          e.preventDefault();
          window.dispatchEvent(new CustomEvent('toggleNotificationCenter'));
          break;
        case '?':
          setShowShortcuts((v) => !v);
          break;
        case '[':
          if (!composeContext) setSidebarCollapsed((v) => !v);
          break;
      }
    }
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [messages, searchResults, selectedMessageId, selectedMessage, composeContext, openCompose, showShortcuts, handleDelete, handleArchive, handleSpam, handleMarkRead, handleMarkUnread, handleStar, folders, messageLabels, setLabel, activeFolderSystemType, setActiveApp, showSpotlight, handleMove, handlePin, activeApp, isMobile, handleGlobalEscape]);
}
