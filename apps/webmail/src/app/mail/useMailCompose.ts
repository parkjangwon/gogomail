import { useState, useCallback } from 'react';
import { type UIComposeIntent, type MessageDetail } from '@/lib/api';

export type ComposeContext = {
  intent: UIComposeIntent;
  source?: MessageDetail;
  draft?: MessageDetail;
  to?: string;
  initialSubject?: string;
  initialBody?: string;
  focusSubjectOnOpen?: boolean;
};

interface UseMailComposeReturn {
  composeContext: ComposeContext | null;
  openCompose: (ctx: ComposeContext) => void;
  closeCompose: () => void;
  pendingCompose: { intent: 'reply' | 'forward'; messageId: string } | null;
  setPendingCompose: React.Dispatch<React.SetStateAction<{ intent: 'reply' | 'forward'; messageId: string } | null>>;
}

export function useMailCompose(): UseMailComposeReturn {
  const [composeContext, setComposeContext] = useState<ComposeContext | null>(null);
  const [pendingCompose, setPendingCompose] = useState<{ intent: 'reply' | 'forward'; messageId: string } | null>(null);

  const openCompose = useCallback((ctx: ComposeContext) => setComposeContext(ctx), []);
  const closeCompose = useCallback(() => setComposeContext(null), []);

  return {
    composeContext,
    openCompose,
    closeCompose,
    pendingCompose,
    setPendingCompose,
  };
}
