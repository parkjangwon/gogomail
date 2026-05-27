import { useEffect } from 'react';
import { type MessageDetail, type UIComposeIntent } from '@/lib/api';
import { type ComposeContext } from './useMailCompose';

interface UseMailComposeGateParams {
  selectedMessage: MessageDetail | null;
  activeFolderSystemType: string | undefined;
  pendingCompose: { messageId: string; intent: 'reply' | 'forward' } | null;
  openCompose: (ctx: ComposeContext) => void;
  setSelectedMessageId: (id: string | null) => void;
  setPendingCompose: (v: null) => void;
}

export function useMailComposeGate({
  selectedMessage,
  activeFolderSystemType,
  pendingCompose,
  openCompose,
  setSelectedMessageId,
  setPendingCompose,
}: UseMailComposeGateParams): void {
  // When a draft message loads, open it in compose instead of ReadingPane
  useEffect(() => {
    if (!selectedMessage || activeFolderSystemType !== 'drafts') return;
    openCompose({ intent: 'new', draft: selectedMessage });
    setSelectedMessageId(null);
  }, [selectedMessage, activeFolderSystemType]);

  useEffect(() => {
    if (!pendingCompose || !selectedMessage || selectedMessage.id !== pendingCompose.messageId) return;
    openCompose({ intent: pendingCompose.intent, source: selectedMessage });
    setPendingCompose(null);
  }, [pendingCompose, selectedMessage]);
}
